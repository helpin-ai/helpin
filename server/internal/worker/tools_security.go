package worker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	ToolScanSemgrep  = "scan_semgrep"
	ToolScanTrivy    = "scan_trivy"
	ToolScanGitleaks = "scan_gitleaks"
)

type securityScannerRequest struct {
	ScanPaths            []string `json:"scan_paths"`
	ExcludePaths         []string `json:"exclude_paths"`
	SeverityThreshold    string   `json:"severity_threshold"`
	MaxFindings          int      `json:"max_findings"`
	IncludeLowInfo       bool     `json:"include_low_info"`
	Config               string   `json:"config"`
	FallbackToAutoConfig bool     `json:"fallback_to_auto_config"`
	Scanners             []string `json:"scanners"`
	ScanGitHistory       bool     `json:"scan_git_history"`
}

type SecurityScanSummary struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Info     int `json:"info"`
}

type SecurityScanFinding struct {
	Scanner          string                 `json:"scanner"`
	Category         string                 `json:"category"`
	Severity         string                 `json:"severity"`
	RuleID           string                 `json:"rule_id"`
	Title            string                 `json:"title"`
	Message          string                 `json:"message"`
	Path             string                 `json:"path"`
	StartLine        *int                   `json:"start_line,omitempty"`
	EndLine          *int                   `json:"end_line,omitempty"`
	PackageName      *string                `json:"package_name,omitempty"`
	InstalledVersion *string                `json:"installed_version,omitempty"`
	FixedVersion     *string                `json:"fixed_version,omitempty"`
	VulnerabilityID  *string                `json:"vulnerability_id,omitempty"`
	CWEIDs           []string               `json:"cwe_ids,omitempty"`
	CVSSScore        *float64               `json:"cvss_score,omitempty"`
	References       []string               `json:"references,omitempty"`
	Fingerprint      string                 `json:"fingerprint"`
	Raw              map[string]interface{} `json:"raw,omitempty"`
}

type SecurityScannerResult struct {
	Scanner  string                `json:"scanner"`
	Summary  SecurityScanSummary   `json:"summary"`
	Findings []SecurityScanFinding `json:"findings"`
	Warnings []string              `json:"warnings,omitempty"`
}

func securityScannerToolSchema(scanner string) map[string]interface{} {
	props := map[string]interface{}{
		"scan_paths": map[string]interface{}{
			"type":        "array",
			"description": "Repository-relative paths to scan. Defaults to the whole repository.",
			"items":       map[string]interface{}{"type": "string"},
		},
		"exclude_paths": map[string]interface{}{
			"type":        "array",
			"description": "Repository-relative path prefixes to exclude from returned findings.",
			"items":       map[string]interface{}{"type": "string"},
		},
		"severity_threshold": map[string]interface{}{
			"type":        "string",
			"description": "Minimum normalized severity to return.",
			"enum":        []string{"critical", "high", "medium", "low", "info"},
		},
		"max_findings": map[string]interface{}{
			"type":        "integer",
			"description": "Maximum findings to return after filtering. Defaults to 200, max 1000.",
		},
		"include_low_info": map[string]interface{}{
			"type":        "boolean",
			"description": "Whether low and informational findings may be returned.",
		},
	}
	switch scanner {
	case ToolScanSemgrep:
		props["config"] = map[string]interface{}{"type": "string", "description": "Semgrep config path or registry config. Defaults to bundled Helpin rules."}
		props["fallback_to_auto_config"] = map[string]interface{}{"type": "boolean", "description": "Fallback to Semgrep auto config when bundled rules are unavailable. Defaults to true."}
	case ToolScanTrivy:
		props["scanners"] = map[string]interface{}{
			"type":        "array",
			"description": "Trivy scanner kinds to run.",
			"items":       map[string]interface{}{"type": "string", "enum": []string{"vuln", "misconfig", "secret"}},
		}
	case ToolScanGitleaks:
		props["scan_git_history"] = map[string]interface{}{"type": "boolean", "description": "Run a full git-history scan. Defaults to false for scheduled scans."}
	}
	return map[string]interface{}{
		"type":                 "object",
		"properties":           props,
		"required":             []string{},
		"additionalProperties": false,
	}
}

func toolScanSemgrep(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	req, err := parseSecurityScannerRequest(input)
	if err != nil {
		return "", err
	}
	if req.Config == "" {
		req.Config = "/app/security-rules/semgrep"
	}
	if !jsonFieldPresent(input, "fallback_to_auto_config") {
		req.FallbackToAutoConfig = true
	}
	args := []string{"scan", "--config", req.Config, "--json"}
	for _, exclude := range req.ExcludePaths {
		args = append(args, "--exclude", exclude)
	}
	args = append(args, req.ScanPaths...)

	stdout, stderr, err := runSecurityScannerCommand(ctx, "semgrep", args)
	warnings := scannerCommandWarnings("semgrep", stderr, err)
	if err != nil && req.FallbackToAutoConfig && req.Config != "auto" {
		fallbackArgs := []string{"scan", "--config", "auto", "--json"}
		for _, exclude := range req.ExcludePaths {
			fallbackArgs = append(fallbackArgs, "--exclude", exclude)
		}
		fallbackArgs = append(fallbackArgs, req.ScanPaths...)
		stdout, stderr, err = runSecurityScannerCommand(ctx, "semgrep", fallbackArgs)
		warnings = append(warnings, "semgrep bundled config failed; retried with --config auto")
		warnings = append(warnings, scannerCommandWarnings("semgrep fallback", stderr, err)...)
	}
	if err != nil && strings.TrimSpace(stdout) == "" {
		return "", fmt.Errorf("semgrep scan failed: %w", err)
	}
	findings, parseWarnings, err := parseSemgrepFindings([]byte(stdout))
	if err != nil {
		return "", err
	}
	warnings = append(warnings, parseWarnings...)
	return marshalSecurityScannerResult("semgrep", findings, req, warnings)
}

func toolScanTrivy(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	req, err := parseSecurityScannerRequest(input)
	if err != nil {
		return "", err
	}
	if len(req.Scanners) == 0 {
		req.Scanners = []string{"vuln", "misconfig", "secret"}
	}
	for _, scanner := range req.Scanners {
		switch scanner {
		case "vuln", "misconfig", "secret":
		default:
			return "", fmt.Errorf("unsupported trivy scanner %q", scanner)
		}
	}
	args := []string{
		"fs",
		"--cache-dir", "/app/.cache/trivy",
		"--format", "json",
		"--skip-version-check",
		"--scanners", strings.Join(req.Scanners, ","),
		"--severity", "LOW,MEDIUM,HIGH,CRITICAL",
	}
	args = append(args, req.ScanPaths...)
	stdout, stderr, err := runSecurityScannerCommand(ctx, "trivy", args)
	warnings := scannerCommandWarnings("trivy", stderr, err)
	if err != nil && strings.TrimSpace(stdout) == "" {
		return "", fmt.Errorf("trivy scan failed: %w", err)
	}
	findings, parseWarnings, err := parseTrivyFindings([]byte(stdout))
	if err != nil {
		return "", err
	}
	warnings = append(warnings, parseWarnings...)
	return marshalSecurityScannerResult("trivy", findings, req, warnings)
}

func toolScanGitleaks(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	req, err := parseSecurityScannerRequest(input)
	if err != nil {
		return "", err
	}
	if !jsonFieldPresent(input, "severity_threshold") {
		req.SeverityThreshold = "high"
	}
	args := gitleaksCommandArgs(req)
	stdout, stderr, err := runSecurityScannerCommand(ctx, "gitleaks", args)
	warnings := scannerCommandWarnings("gitleaks", stderr, nil)
	if !req.ScanGitHistory {
		warnings = append(warnings, "gitleaks ran against the working tree only; git history was not scanned")
	}
	if err != nil && strings.TrimSpace(stdout) == "" {
		return "", fmt.Errorf("gitleaks scan failed: %w", err)
	}
	findings, parseWarnings, err := parseGitleaksFindings([]byte(stdout))
	if err != nil {
		return "", err
	}
	warnings = append(warnings, parseWarnings...)
	return marshalSecurityScannerResult("gitleaks", findings, req, warnings)
}

func gitleaksCommandArgs(req securityScannerRequest) []string {
	args := []string{"detect", "--source", ".", "--report-format", "json", "--report-path", "-", "--redact", "--exit-code", "0"}
	if !req.ScanGitHistory {
		args = append(args, "--no-git")
	}
	return args
}

func parseSecurityScannerRequest(input json.RawMessage) (securityScannerRequest, error) {
	var req securityScannerRequest
	if len(strings.TrimSpace(string(input))) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return req, fmt.Errorf("parse input: %w", err)
		}
	}
	if len(req.ScanPaths) == 0 {
		req.ScanPaths = []string{"."}
	}
	for idx, path := range req.ScanPaths {
		clean, err := cleanRepoRelativePath(path)
		if err != nil {
			return req, fmt.Errorf("scan_paths[%d]: %w", idx, err)
		}
		req.ScanPaths[idx] = clean
	}
	for idx, path := range req.ExcludePaths {
		clean, err := cleanRepoRelativePath(path)
		if err != nil {
			return req, fmt.Errorf("exclude_paths[%d]: %w", idx, err)
		}
		if clean == "." {
			return req, fmt.Errorf("exclude_paths[%d] cannot exclude the entire repository", idx)
		}
		req.ExcludePaths[idx] = clean
	}
	req.SeverityThreshold = normalizeSecuritySeverity(req.SeverityThreshold)
	if req.SeverityThreshold == "" {
		req.SeverityThreshold = "medium"
	}
	if req.MaxFindings <= 0 {
		req.MaxFindings = 200
	}
	if req.MaxFindings > 1000 {
		req.MaxFindings = 1000
	}
	return req, nil
}

func jsonFieldPresent(input json.RawMessage, field string) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(input, &raw); err != nil {
		return false
	}
	_, ok := raw[field]
	return ok
}

func runSecurityScannerCommand(ctx *ExecutionContext, program string, args []string) (string, string, error) {
	if ctx == nil || strings.TrimSpace(ctx.WorkDir) == "" {
		return "", "", fmt.Errorf("scanner requires a checked-out repository workspace")
	}
	baseCtx := context.Background()
	if ctx.Context != nil {
		baseCtx = ctx.Context
	}
	cmdCtx, cancel := context.WithTimeout(baseCtx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, program, args...)
	cmd.Dir = ctx.WorkDir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func scannerCommandWarnings(label, stderr string, err error) []string {
	var warnings []string
	if trimmed := strings.TrimSpace(stderr); trimmed != "" {
		warnings = append(warnings, fmt.Sprintf("%s stderr: %s", label, truncateString(trimmed, 1000)))
	}
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("%s exited with %v; parser will use JSON output if available", label, err))
	}
	return warnings
}

func marshalSecurityScannerResult(scanner string, findings []SecurityScanFinding, req securityScannerRequest, warnings []string) (string, error) {
	filtered, filterWarnings := filterSecurityFindings(findings, req)
	warnings = append(warnings, filterWarnings...)
	result := SecurityScannerResult{
		Scanner:  scanner,
		Summary:  summarizeSecurityFindings(filtered),
		Findings: filtered,
		Warnings: dedupeStrings(warnings),
	}
	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal scanner result: %w", err)
	}
	return string(out), nil
}

func filterSecurityFindings(findings []SecurityScanFinding, req securityScannerRequest) ([]SecurityScanFinding, []string) {
	var warnings []string
	excludes := req.ExcludePaths
	deduped := make([]SecurityScanFinding, 0, len(findings))
	seen := make(map[string]bool, len(findings))
	for _, finding := range findings {
		finding.Severity = normalizeSecuritySeverity(finding.Severity)
		if finding.Severity == "" {
			finding.Severity = "info"
		}
		finding.Path = normalizeRepoOutputPath(finding.Path)
		if !pathIncluded(finding.Path, req.ScanPaths) {
			continue
		}
		if pathExcluded(finding.Path, excludes) {
			continue
		}
		if !req.IncludeLowInfo && (finding.Severity == "low" || finding.Severity == "info") {
			continue
		}
		if severityRank(finding.Severity) < severityRank(req.SeverityThreshold) {
			continue
		}
		key := strings.Join([]string{finding.Scanner, finding.Category, finding.RuleID, finding.Path, intPtrString(finding.StartLine), finding.Fingerprint}, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, finding)
	}
	sort.SliceStable(deduped, func(i, j int) bool {
		a, b := deduped[i], deduped[j]
		if severityRank(a.Severity) != severityRank(b.Severity) {
			return severityRank(a.Severity) > severityRank(b.Severity)
		}
		for _, cmp := range [][2]string{{a.Scanner, b.Scanner}, {a.Path, b.Path}, {a.RuleID, b.RuleID}, {a.Fingerprint, b.Fingerprint}} {
			if cmp[0] != cmp[1] {
				return cmp[0] < cmp[1]
			}
		}
		return intPtrValue(a.StartLine) < intPtrValue(b.StartLine)
	})
	if len(deduped) > req.MaxFindings {
		warnings = append(warnings, fmt.Sprintf("findings truncated from %d to max_findings=%d", len(deduped), req.MaxFindings))
		deduped = deduped[:req.MaxFindings]
	}
	return deduped, warnings
}

func summarizeSecurityFindings(findings []SecurityScanFinding) SecurityScanSummary {
	var summary SecurityScanSummary
	for _, finding := range findings {
		summary.Total++
		switch normalizeSecuritySeverity(finding.Severity) {
		case "critical":
			summary.Critical++
		case "high":
			summary.High++
		case "medium":
			summary.Medium++
		case "low":
			summary.Low++
		default:
			summary.Info++
		}
	}
	return summary
}

func cleanRepoRelativePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || path == "." {
		return ".", nil
	}
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("must be relative to the repository root")
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("must stay within the repository")
	}
	return clean, nil
}

func normalizeRepoOutputPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return path
	}
	path = filepath.ToSlash(filepath.Clean(path))
	if path == "." {
		return path
	}
	return strings.TrimPrefix(path, "./")
}

func pathIncluded(path string, includes []string) bool {
	if len(includes) == 0 {
		return true
	}
	path = normalizeRepoOutputPath(path)
	for _, include := range includes {
		include = normalizeRepoOutputPath(include)
		if include == "." || path == include || strings.HasPrefix(path, strings.TrimSuffix(include, "/")+"/") {
			return true
		}
	}
	return false
}

func pathExcluded(path string, excludes []string) bool {
	path = normalizeRepoOutputPath(path)
	for _, exclude := range excludes {
		exclude = normalizeRepoOutputPath(exclude)
		if path == exclude || strings.HasPrefix(path, strings.TrimSuffix(exclude, "/")+"/") {
			return true
		}
	}
	return false
}

func normalizeSecuritySeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "critical", "crit":
		return "critical"
	case "high", "error":
		return "high"
	case "medium", "med", "warning", "warn", "moderate":
		return "medium"
	case "low":
		return "low"
	case "info", "informational", "inventory", "experiment", "unknown", "negligible":
		return "info"
	default:
		return ""
	}
}

func severityRank(value string) int {
	switch normalizeSecuritySeverity(value) {
	case "critical":
		return 5
	case "high":
		return 4
	case "medium":
		return 3
	case "low":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

func securityHash(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:24]
}

func stringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func intPtr(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}

func floatPtr(value float64) *float64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func intPtrString(value *int) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%d", *value)
}

func intPtrValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func compactMessage(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	return truncateString(value, limit)
}

func truncateString(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

func isURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && parsed.Scheme != "" && parsed.Host != ""
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func capStrings(values []string, max int) []string {
	values = dedupeStrings(values)
	if max > 0 && len(values) > max {
		return values[:max]
	}
	return values
}

func parseSemgrepFindings(data []byte) ([]SecurityScanFinding, []string, error) {
	if strings.TrimSpace(string(data)) == "" {
		return nil, nil, fmt.Errorf("semgrep returned empty JSON output")
	}
	var payload struct {
		Results []struct {
			CheckID string `json:"check_id"`
			Path    string `json:"path"`
			Start   struct {
				Line int `json:"line"`
			} `json:"start"`
			End struct {
				Line int `json:"line"`
			} `json:"end"`
			Extra struct {
				Message     string                 `json:"message"`
				Severity    string                 `json:"severity"`
				Fingerprint string                 `json:"fingerprint"`
				Metadata    map[string]interface{} `json:"metadata"`
			} `json:"extra"`
		} `json:"results"`
		Errors []interface{} `json:"errors"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, nil, fmt.Errorf("parse semgrep JSON: %w", err)
	}
	var warnings []string
	if len(payload.Errors) > 0 {
		warnings = append(warnings, fmt.Sprintf("semgrep reported %d parser/runtime errors", len(payload.Errors)))
	}
	findings := make([]SecurityScanFinding, 0, len(payload.Results))
	for _, result := range payload.Results {
		refs := metadataStringSlice(result.Extra.Metadata, "references")
		for _, key := range []string{"source", "shortlink"} {
			if value, ok := result.Extra.Metadata[key].(string); ok && isURL(value) {
				refs = append(refs, value)
			}
		}
		raw := compactMetadata(result.Extra.Metadata, []string{"impact", "likelihood", "confidence", "owasp", "technology", "category"})
		severity := normalizeSecuritySeverity(result.Extra.Severity)
		if severityRank(severity) < severityRank("high") && strings.EqualFold(metadataString(result.Extra.Metadata, "impact"), "high") && strings.EqualFold(metadataString(result.Extra.Metadata, "confidence"), "high") {
			severity = "high"
		}
		fingerprint := strings.TrimSpace(result.Extra.Fingerprint)
		if fingerprint == "" {
			fingerprint = securityHash("semgrep", result.CheckID, result.Path, fmt.Sprintf("%d", result.Start.Line), fmt.Sprintf("%d", result.End.Line), result.Extra.Message)
		}
		findings = append(findings, SecurityScanFinding{
			Scanner:     "semgrep",
			Category:    "sast",
			Severity:    severity,
			RuleID:      result.CheckID,
			Title:       semgrepTitle(result.CheckID, result.Extra.Metadata),
			Message:     compactMessage(result.Extra.Message, 500),
			Path:        result.Path,
			StartLine:   intPtr(result.Start.Line),
			EndLine:     intPtr(result.End.Line),
			CWEIDs:      metadataStringSlice(result.Extra.Metadata, "cwe"),
			References:  capStrings(refs, 10),
			Fingerprint: fingerprint,
			Raw:         raw,
		})
	}
	return findings, warnings, nil
}

func semgrepTitle(checkID string, metadata map[string]interface{}) string {
	for _, key := range []string{"shortlink", "source"} {
		if value, ok := metadata[key].(string); ok && value != "" && !isURL(value) {
			return value
		}
	}
	parts := strings.Split(checkID, ".")
	if len(parts) > 0 && parts[len(parts)-1] != "" {
		return strings.ReplaceAll(parts[len(parts)-1], "-", " ")
	}
	return checkID
}

func parseTrivyFindings(data []byte) ([]SecurityScanFinding, []string, error) {
	if strings.TrimSpace(string(data)) == "" {
		return nil, nil, fmt.Errorf("trivy returned empty JSON output")
	}
	var payload struct {
		Results []trivyResult `json:"Results"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, nil, fmt.Errorf("parse trivy JSON: %w", err)
	}
	var findings []SecurityScanFinding
	for _, result := range payload.Results {
		findings = append(findings, parseTrivyVulnerabilities(result)...)
		findings = append(findings, parseTrivyMisconfigurations(result)...)
		findings = append(findings, parseTrivySecrets(result)...)
	}
	return findings, nil, nil
}

type trivyResult struct {
	Target            string               `json:"Target"`
	Class             string               `json:"Class"`
	Type              string               `json:"Type"`
	Vulnerabilities   []trivyVulnerability `json:"Vulnerabilities"`
	Misconfigurations []trivyMisconfig     `json:"Misconfigurations"`
	Secrets           []trivySecret        `json:"Secrets"`
}

type trivyVulnerability struct {
	VulnerabilityID  string                            `json:"VulnerabilityID"`
	PkgName          string                            `json:"PkgName"`
	InstalledVersion string                            `json:"InstalledVersion"`
	FixedVersion     string                            `json:"FixedVersion"`
	Severity         string                            `json:"Severity"`
	Title            string                            `json:"Title"`
	Description      string                            `json:"Description"`
	PrimaryURL       string                            `json:"PrimaryURL"`
	References       []string                          `json:"References"`
	CVSS             map[string]map[string]interface{} `json:"CVSS"`
	CWEIDs           []string                          `json:"CweIDs"`
	PkgPath          string                            `json:"PkgPath"`
}

type trivyMisconfig struct {
	ID            string   `json:"ID"`
	AVDID         string   `json:"AVDID"`
	Type          string   `json:"Type"`
	Title         string   `json:"Title"`
	Description   string   `json:"Description"`
	Message       string   `json:"Message"`
	Resolution    string   `json:"Resolution"`
	Severity      string   `json:"Severity"`
	PrimaryURL    string   `json:"PrimaryURL"`
	References    []string `json:"References"`
	CauseMetadata struct {
		StartLine int `json:"StartLine"`
		EndLine   int `json:"EndLine"`
	} `json:"CauseMetadata"`
}

type trivySecret struct {
	RuleID    string `json:"RuleID"`
	Category  string `json:"Category"`
	Severity  string `json:"Severity"`
	Title     string `json:"Title"`
	StartLine int    `json:"StartLine"`
	EndLine   int    `json:"EndLine"`
}

func parseTrivyVulnerabilities(result trivyResult) []SecurityScanFinding {
	findings := make([]SecurityScanFinding, 0, len(result.Vulnerabilities))
	for _, vuln := range result.Vulnerabilities {
		path := vuln.PkgPath
		if path == "" {
			path = result.Target
		}
		title := strings.TrimSpace(vuln.Title)
		if title == "" {
			title = vuln.VulnerabilityID
		}
		refs := append([]string{}, vuln.References...)
		if isURL(vuln.PrimaryURL) {
			refs = append([]string{vuln.PrimaryURL}, refs...)
		}
		findings = append(findings, SecurityScanFinding{
			Scanner:          "trivy",
			Category:         "dependency",
			Severity:         normalizeSecuritySeverity(vuln.Severity),
			RuleID:           vuln.VulnerabilityID,
			Title:            title,
			Message:          compactMessage(vuln.Description, 500),
			Path:             path,
			PackageName:      stringPtr(vuln.PkgName),
			InstalledVersion: stringPtr(vuln.InstalledVersion),
			FixedVersion:     stringPtr(vuln.FixedVersion),
			VulnerabilityID:  stringPtr(vuln.VulnerabilityID),
			CWEIDs:           vuln.CWEIDs,
			CVSSScore:        floatPtr(maxTrivyCVSS(vuln.CVSS)),
			References:       capStrings(refs, 10),
			Fingerprint:      securityHash("trivy", "vuln", result.Target, vuln.PkgName, vuln.InstalledVersion, vuln.VulnerabilityID),
			Raw: map[string]interface{}{
				"target":          result.Target,
				"dependency_file": result.Target,
				"ecosystem":       result.Type,
			},
		})
	}
	return findings
}

func parseTrivyMisconfigurations(result trivyResult) []SecurityScanFinding {
	findings := make([]SecurityScanFinding, 0, len(result.Misconfigurations))
	for _, misconfig := range result.Misconfigurations {
		ruleID := misconfig.AVDID
		if ruleID == "" {
			ruleID = misconfig.ID
		}
		message := misconfig.Message
		if message == "" {
			message = misconfig.Description
		}
		refs := append([]string{}, misconfig.References...)
		if isURL(misconfig.PrimaryURL) {
			refs = append([]string{misconfig.PrimaryURL}, refs...)
		}
		raw := map[string]interface{}{"target": result.Target}
		if strings.TrimSpace(misconfig.Resolution) != "" {
			raw["resolution"] = misconfig.Resolution
		}
		findings = append(findings, SecurityScanFinding{
			Scanner:     "trivy",
			Category:    "misconfig",
			Severity:    normalizeSecuritySeverity(misconfig.Severity),
			RuleID:      ruleID,
			Title:       misconfig.Title,
			Message:     compactMessage(message, 500),
			Path:        result.Target,
			StartLine:   intPtr(misconfig.CauseMetadata.StartLine),
			EndLine:     intPtr(misconfig.CauseMetadata.EndLine),
			References:  capStrings(refs, 10),
			Fingerprint: securityHash("trivy", "misconfig", result.Target, ruleID, fmt.Sprintf("%d", misconfig.CauseMetadata.StartLine), message),
			Raw:         raw,
		})
	}
	return findings
}

func parseTrivySecrets(result trivyResult) []SecurityScanFinding {
	findings := make([]SecurityScanFinding, 0, len(result.Secrets))
	for _, secret := range result.Secrets {
		severity := normalizeSecuritySeverity(secret.Severity)
		if severity == "" {
			severity = "high"
		}
		title := strings.TrimSpace(secret.Title)
		if title == "" {
			title = secret.RuleID
		}
		raw := map[string]interface{}{"target": result.Target}
		if strings.TrimSpace(secret.Category) != "" {
			raw["category"] = secret.Category
		}
		findings = append(findings, SecurityScanFinding{
			Scanner:     "trivy",
			Category:    "secret",
			Severity:    severity,
			RuleID:      secret.RuleID,
			Title:       title,
			Message:     fmt.Sprintf("Secret detected by Trivy rule %s. Raw secret value redacted.", secret.RuleID),
			Path:        result.Target,
			StartLine:   intPtr(secret.StartLine),
			EndLine:     intPtr(secret.EndLine),
			Fingerprint: securityHash("trivy", "secret", result.Target, secret.RuleID, fmt.Sprintf("%d", secret.StartLine)),
			Raw:         raw,
		})
	}
	return findings
}

func maxTrivyCVSS(values map[string]map[string]interface{}) float64 {
	var max float64
	for _, vendor := range values {
		for _, key := range []string{"V3Score", "V2Score"} {
			switch value := vendor[key].(type) {
			case float64:
				if value > max {
					max = value
				}
			case int:
				if float64(value) > max {
					max = float64(value)
				}
			}
		}
	}
	return max
}

func parseGitleaksFindings(data []byte) ([]SecurityScanFinding, []string, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, nil, fmt.Errorf("gitleaks returned empty JSON output")
	}
	var payload []struct {
		RuleID      string   `json:"RuleID"`
		Description string   `json:"Description"`
		File        string   `json:"File"`
		StartLine   int      `json:"StartLine"`
		EndLine     int      `json:"EndLine"`
		StartColumn int      `json:"StartColumn"`
		EndColumn   int      `json:"EndColumn"`
		Entropy     float64  `json:"Entropy"`
		Tags        []string `json:"Tags"`
		Fingerprint string   `json:"Fingerprint"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, nil, fmt.Errorf("parse gitleaks JSON: %w", err)
	}
	findings := make([]SecurityScanFinding, 0, len(payload))
	for _, result := range payload {
		fingerprint := strings.TrimSpace(result.Fingerprint)
		if fingerprint == "" {
			fingerprint = securityHash("gitleaks", result.RuleID, result.File, fmt.Sprintf("%d", result.StartLine), fmt.Sprintf("%d", result.StartColumn), fmt.Sprintf("%d", result.EndColumn))
		}
		raw := map[string]interface{}{}
		if result.Entropy > 0 {
			raw["entropy"] = result.Entropy
		}
		if len(result.Tags) > 0 {
			raw["tags"] = result.Tags
		}
		findings = append(findings, SecurityScanFinding{
			Scanner:     "gitleaks",
			Category:    "secret",
			Severity:    "high",
			RuleID:      result.RuleID,
			Title:       result.Description,
			Message:     fmt.Sprintf("Potential secret detected by Gitleaks rule %s. Raw secret value redacted.", result.RuleID),
			Path:        result.File,
			StartLine:   intPtr(result.StartLine),
			EndLine:     intPtr(result.EndLine),
			Fingerprint: fingerprint,
			Raw:         raw,
		})
	}
	return findings, nil, nil
}

func metadataString(metadata map[string]interface{}, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return value
}

func metadataStringSlice(metadata map[string]interface{}, key string) []string {
	if metadata == nil {
		return nil
	}
	value, ok := metadata[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return typed
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if str, ok := item.(string); ok && str != "" {
				out = append(out, str)
			}
		}
		return out
	case string:
		if typed != "" {
			return []string{typed}
		}
	}
	return nil
}

func compactMetadata(metadata map[string]interface{}, keys []string) map[string]interface{} {
	if metadata == nil {
		return nil
	}
	out := make(map[string]interface{})
	for _, key := range keys {
		if value, ok := metadata[key]; ok {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
