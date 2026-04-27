package worker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
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

	securityRuntimeCacheDir = "/tmp/helpin-security-cache"
	securityImageCacheDir   = "/app/.cache"
	securitySemgrepRulesDir = "/app/security-rules/semgrep"
)

type securityScannerRequest struct {
	ScanPaths            []string `json:"scan_paths"`
	ExcludePaths         []string `json:"exclude_paths"`
	SeverityThreshold    string   `json:"severity_threshold"`
	MaxFindings          int      `json:"max_findings"`
	Page                 int      `json:"page"`
	PageSize             int      `json:"page_size"`
	SummaryOnly          bool     `json:"summary_only"`
	Category             string   `json:"category"`
	RuleIDs              []string `json:"rule_ids"`
	PackageNames         []string `json:"package_names"`
	VulnerabilityIDs     []string `json:"vulnerability_ids"`
	Paths                []string `json:"paths"`
	IncludeLowInfo       bool     `json:"include_low_info"`
	Config               string   `json:"config"`
	FallbackToAutoConfig bool     `json:"fallback_to_auto_config"`
	Scanners             []string `json:"scanners"`
	ScanGitHistory       bool     `json:"scan_git_history"`
	DetailLevel          string   `json:"detail_level"`

	legacyMaxFindings bool
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
	Scanner                 string                 `json:"scanner"`
	Summary                 SecurityScanSummary    `json:"summary"`
	SummaryBeforePagination SecurityScanSummary    `json:"summary_before_pagination"`
	TotalFindings           int                    `json:"total_findings"`
	ReturnedFindings        int                    `json:"returned_findings"`
	Page                    int                    `json:"page"`
	PageSize                int                    `json:"page_size"`
	HasMore                 bool                   `json:"has_more"`
	ScanID                  string                 `json:"scan_id,omitempty"`
	CacheHit                bool                   `json:"cache_hit"`
	SummaryOnly             bool                   `json:"summary_only,omitempty"`
	DetailLevel             string                 `json:"detail_level,omitempty"`
	Bounded                 bool                   `json:"bounded,omitempty"`
	Compaction              *helpinCompactionHint  `json:"_helpin_compaction,omitempty"`
	Groups                  []SecurityFindingGroup `json:"groups,omitempty"`
	Findings                []SecurityScanFinding  `json:"findings"`
	Warnings                []string               `json:"warnings,omitempty"`
}

type SecurityFindingGroup struct {
	Kind            string              `json:"kind"`
	Key             string              `json:"key"`
	Category        string              `json:"category,omitempty"`
	Severity        string              `json:"severity,omitempty"`
	RuleID          string              `json:"rule_id,omitempty"`
	PackageName     string              `json:"package_name,omitempty"`
	VulnerabilityID string              `json:"vulnerability_id,omitempty"`
	Path            string              `json:"path,omitempty"`
	Count           int                 `json:"count"`
	Summary         SecurityScanSummary `json:"summary"`
}

type securityScannerCacheEntry struct {
	ScanID    string
	Scanner   string
	Findings  []SecurityScanFinding
	Warnings  []string
	CreatedAt time.Time
}

var securityScannerCommandRunner = runSecurityScannerCommand

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
			"description": "Legacy maximum findings to return after filtering. Prefer page/page_size.",
		},
		"page": map[string]interface{}{
			"type":        "integer",
			"description": "1-based result page after filtering and sorting. Defaults to 1.",
		},
		"page_size": map[string]interface{}{
			"type":        "integer",
			"description": "Maximum findings to return on this page. Defaults to 100, max 200.",
		},
		"summary_only": map[string]interface{}{
			"type":        "boolean",
			"description": "When true, return counts and groups without full findings.",
		},
		"detail_level": map[string]interface{}{
			"type":        "string",
			"description": "Finding detail shape. Use index for compact triage rows, full for verbose scanner details.",
			"enum":        []string{"full", "index"},
		},
		"category": map[string]interface{}{
			"type":        "string",
			"description": "Optional normalized category filter.",
			"enum":        []string{"dependency", "sast", "secret", "misconfig"},
		},
		"rule_ids": map[string]interface{}{
			"type":        "array",
			"description": "Optional scanner rule IDs to include.",
			"items":       map[string]interface{}{"type": "string"},
		},
		"package_names": map[string]interface{}{
			"type":        "array",
			"description": "Optional dependency package names to include.",
			"items":       map[string]interface{}{"type": "string"},
		},
		"vulnerability_ids": map[string]interface{}{
			"type":        "array",
			"description": "Optional CVE/GHSA/OSV vulnerability IDs to include.",
			"items":       map[string]interface{}{"type": "string"},
		},
		"paths": map[string]interface{}{
			"type":        "array",
			"description": "Optional exact or prefix repository paths to include after scanning.",
			"items":       map[string]interface{}{"type": "string"},
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
		req.Config = securitySemgrepRulesDir
	}
	if !jsonFieldPresent(input, "fallback_to_auto_config") {
		req.FallbackToAutoConfig = true
	}
	env, warnings := securityScannerEnv()
	if req.Config != "auto" && !semgrepConfigAvailable(req.Config) {
		warnings = append(warnings, fmt.Sprintf("semgrep config %q is unavailable or empty; using --config auto", req.Config))
		req.Config = "auto"
	}
	cacheKey, scanID, cacheErr := securityScannerCacheKey(ctx, "semgrep", req)
	if cacheErr == nil {
		if entry, ok := getSecurityScannerCacheEntry(ctx, cacheKey); ok {
			return marshalSecurityScannerCachedResult("semgrep", entry.ScanID, true, entry.Findings, req, append(warnings, entry.Warnings...))
		}
	} else {
		warnings = append(warnings, fmt.Sprintf("scanner cache disabled: %v", cacheErr))
	}
	args := []string{"scan", "--config", req.Config, "--json"}
	for _, exclude := range req.ExcludePaths {
		args = append(args, "--exclude", exclude)
	}
	args = append(args, req.ScanPaths...)

	stdout, stderr, commandErr := securityScannerCommandRunner(ctx, "semgrep", args, env)
	warnings = append(warnings, scannerCommandWarnings("semgrep", stderr, commandErr)...)
	if commandErr != nil && req.FallbackToAutoConfig && req.Config != "auto" {
		fallbackArgs := []string{"scan", "--config", "auto", "--json"}
		for _, exclude := range req.ExcludePaths {
			fallbackArgs = append(fallbackArgs, "--exclude", exclude)
		}
		fallbackArgs = append(fallbackArgs, req.ScanPaths...)
		stdout, stderr, commandErr = securityScannerCommandRunner(ctx, "semgrep", fallbackArgs, env)
		warnings = append(warnings, "semgrep bundled config failed; retried with --config auto")
		warnings = append(warnings, scannerCommandWarnings("semgrep fallback", stderr, commandErr)...)
	}
	if commandErr != nil && strings.TrimSpace(stdout) == "" {
		warnings = append(warnings, fmt.Sprintf("semgrep produced no JSON output; returning zero findings"))
		return marshalSecurityScannerResult("semgrep", nil, req, warnings)
	}
	findings, parseWarnings, err := parseSemgrepFindings([]byte(stdout))
	if err != nil {
		if commandErr != nil {
			warnings = append(warnings, scannerParseFailureWarnings("semgrep", stdout, err)...)
			return marshalSecurityScannerResult("semgrep", nil, req, warnings)
		}
		return "", err
	}
	warnings = append(warnings, parseWarnings...)
	if cacheErr == nil {
		putSecurityScannerCacheEntry(ctx, cacheKey, securityScannerCacheEntry{
			ScanID:    scanID,
			Scanner:   "semgrep",
			Findings:  findings,
			Warnings:  append([]string(nil), warnings...),
			CreatedAt: time.Now().UTC(),
		})
	}
	return marshalSecurityScannerCachedResult("semgrep", scanID, false, findings, req, warnings)
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
	cacheKey, scanID, cacheErr := securityScannerCacheKey(ctx, "trivy", req)
	if cacheErr == nil {
		if entry, ok := getSecurityScannerCacheEntry(ctx, cacheKey); ok {
			return marshalSecurityScannerCachedResult("trivy", entry.ScanID, true, entry.Findings, req, entry.Warnings)
		}
	}
	args := []string{
		"fs",
		"--cache-dir", filepath.Join(securityRuntimeCacheDir, "trivy"),
		"--format", "json",
		"--skip-version-check",
		"--scanners", strings.Join(req.Scanners, ","),
		"--severity", "LOW,MEDIUM,HIGH,CRITICAL",
	}
	warnings := prepareTrivyRuntimeCache()
	if cacheErr != nil {
		warnings = append(warnings, fmt.Sprintf("scanner cache disabled: %v", cacheErr))
	}
	env, envWarnings := securityScannerEnv()
	warnings = append(warnings, envWarnings...)
	args = append(args, req.ScanPaths...)
	stdout, stderr, commandErr := securityScannerCommandRunner(ctx, "trivy", args, env)
	warnings = append(warnings, scannerCommandWarnings("trivy", stderr, commandErr)...)
	if commandErr != nil && strings.TrimSpace(stdout) == "" {
		warnings = append(warnings, "trivy produced no JSON output; returning zero findings")
		return marshalSecurityScannerResult("trivy", nil, req, warnings)
	}
	findings, parseWarnings, err := parseTrivyFindings([]byte(stdout))
	if err != nil {
		if commandErr != nil {
			warnings = append(warnings, scannerParseFailureWarnings("trivy", stdout, err)...)
			return marshalSecurityScannerResult("trivy", nil, req, warnings)
		}
		return "", err
	}
	warnings = append(warnings, parseWarnings...)
	if cacheErr == nil {
		putSecurityScannerCacheEntry(ctx, cacheKey, securityScannerCacheEntry{
			ScanID:    scanID,
			Scanner:   "trivy",
			Findings:  findings,
			Warnings:  append([]string(nil), warnings...),
			CreatedAt: time.Now().UTC(),
		})
	}
	return marshalSecurityScannerCachedResult("trivy", scanID, false, findings, req, warnings)
}

func toolScanGitleaks(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	req, err := parseSecurityScannerRequest(input)
	if err != nil {
		return "", err
	}
	if !jsonFieldPresent(input, "severity_threshold") {
		req.SeverityThreshold = "high"
	}
	cacheKey, scanID, cacheErr := securityScannerCacheKey(ctx, "gitleaks", req)
	if cacheErr == nil {
		if entry, ok := getSecurityScannerCacheEntry(ctx, cacheKey); ok {
			return marshalSecurityScannerCachedResult("gitleaks", entry.ScanID, true, entry.Findings, req, entry.Warnings)
		}
	}
	args := gitleaksCommandArgs(req)
	env, warnings := securityScannerEnv()
	if cacheErr != nil {
		warnings = append(warnings, fmt.Sprintf("scanner cache disabled: %v", cacheErr))
	}
	stdout, stderr, commandErr := securityScannerCommandRunner(ctx, "gitleaks", args, env)
	warnings = append(warnings, scannerCommandWarnings("gitleaks", stderr, commandErr)...)
	if !req.ScanGitHistory {
		warnings = append(warnings, "gitleaks ran against the working tree only; git history was not scanned")
	}
	if commandErr != nil && strings.TrimSpace(stdout) == "" {
		warnings = append(warnings, "gitleaks produced no JSON output; returning zero findings")
		return marshalSecurityScannerResult("gitleaks", nil, req, warnings)
	}
	findings, parseWarnings, err := parseGitleaksFindings([]byte(stdout))
	if err != nil {
		if commandErr != nil {
			warnings = append(warnings, scannerParseFailureWarnings("gitleaks", stdout, err)...)
			return marshalSecurityScannerResult("gitleaks", nil, req, warnings)
		}
		return "", err
	}
	warnings = append(warnings, parseWarnings...)
	if cacheErr == nil {
		putSecurityScannerCacheEntry(ctx, cacheKey, securityScannerCacheEntry{
			ScanID:    scanID,
			Scanner:   "gitleaks",
			Findings:  findings,
			Warnings:  append([]string(nil), warnings...),
			CreatedAt: time.Now().UTC(),
		})
	}
	return marshalSecurityScannerCachedResult("gitleaks", scanID, false, findings, req, warnings)
}

func gitleaksCommandArgs(req securityScannerRequest) []string {
	args := []string{"detect", "--source", ".", "--report-format", "json", "--report-path", "-", "--redact", "--exit-code", "0"}
	if !req.ScanGitHistory {
		args = append(args, "--no-git")
	}
	return args
}

func securityScannerCacheKey(ctx *ExecutionContext, scanner string, req securityScannerRequest) (string, string, error) {
	if ctx == nil {
		return "", "", fmt.Errorf("execution context is unavailable")
	}
	cacheInput := map[string]interface{}{
		"scanner":                 strings.TrimSpace(scanner),
		"work_dir":                strings.TrimSpace(ctx.WorkDir),
		"scan_paths":              sortedStrings(req.ScanPaths),
		"exclude_paths":           sortedStrings(req.ExcludePaths),
		"config":                  strings.TrimSpace(req.Config),
		"fallback_to_auto_config": req.FallbackToAutoConfig,
		"scanners":                sortedStrings(req.Scanners),
		"scan_git_history":        req.ScanGitHistory,
	}
	payload, err := json.Marshal(cacheInput)
	if err != nil {
		return "", "", fmt.Errorf("marshal scanner cache key: %w", err)
	}
	sum := sha256.Sum256(payload)
	key := hex.EncodeToString(sum[:])
	return key, key[:16], nil
}

func getSecurityScannerCacheEntry(ctx *ExecutionContext, key string) (securityScannerCacheEntry, bool) {
	if ctx == nil || strings.TrimSpace(key) == "" {
		return securityScannerCacheEntry{}, false
	}
	ctx.securityScannerCacheMu.Lock()
	defer ctx.securityScannerCacheMu.Unlock()
	if ctx.securityScannerCache == nil {
		return securityScannerCacheEntry{}, false
	}
	entry, ok := ctx.securityScannerCache[key]
	return entry, ok
}

func putSecurityScannerCacheEntry(ctx *ExecutionContext, key string, entry securityScannerCacheEntry) {
	if ctx == nil || strings.TrimSpace(key) == "" {
		return
	}
	ctx.securityScannerCacheMu.Lock()
	defer ctx.securityScannerCacheMu.Unlock()
	if ctx.securityScannerCache == nil {
		ctx.securityScannerCache = make(map[string]securityScannerCacheEntry)
	}
	ctx.securityScannerCache[key] = entry
}

func sortedStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	sort.Strings(out)
	return out
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
	rawCategory := req.Category
	req.Category = normalizeSecurityCategory(req.Category)
	if req.Category == "" && strings.TrimSpace(rawCategory) != "" {
		return req, fmt.Errorf("category must be one of dependency, sast, secret, or misconfig")
	}
	for idx, path := range req.Paths {
		clean, err := cleanRepoRelativePath(path)
		if err != nil {
			return req, fmt.Errorf("paths[%d]: %w", idx, err)
		}
		req.Paths[idx] = clean
	}
	req.RuleIDs = normalizeStringFilters(req.RuleIDs)
	req.PackageNames = normalizeStringFilters(req.PackageNames)
	req.VulnerabilityIDs = normalizeStringFilters(req.VulnerabilityIDs)
	req.DetailLevel = strings.ToLower(strings.TrimSpace(req.DetailLevel))
	if req.DetailLevel == "" {
		req.DetailLevel = "full"
	}
	if req.DetailLevel != "full" && req.DetailLevel != "index" {
		return req, fmt.Errorf("detail_level must be full or index")
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if jsonFieldPresent(input, "max_findings") && !jsonFieldPresent(input, "page_size") && !jsonFieldPresent(input, "page") {
		req.legacyMaxFindings = true
		if req.MaxFindings <= 0 {
			req.MaxFindings = 200
		}
		if req.MaxFindings > 1000 {
			req.MaxFindings = 1000
		}
		req.PageSize = req.MaxFindings
	} else {
		if req.PageSize <= 0 {
			req.PageSize = 100
		}
		if req.PageSize > 200 {
			req.PageSize = 200
		}
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

func runSecurityScannerCommand(ctx *ExecutionContext, program string, args []string, env []string) (string, string, error) {
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
	cmd.Env = mergeSecurityScannerEnv(os.Environ(), env)
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func securityScannerEnv() ([]string, []string) {
	var warnings []string
	semgrepCache := filepath.Join(securityRuntimeCacheDir, "semgrep")
	if err := os.MkdirAll(semgrepCache, 0o755); err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to create semgrep runtime cache: %v", err))
	}
	homeDir := filepath.Join(securityRuntimeCacheDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to create scanner home directory: %v", err))
	}
	env := []string{
		"HOME=" + homeDir,
		"XDG_CACHE_HOME=" + securityRuntimeCacheDir,
		"SEMGREP_SETTINGS_FILE=" + filepath.Join(semgrepCache, "settings.yml"),
		"TRIVY_CACHE_DIR=" + filepath.Join(securityRuntimeCacheDir, "trivy"),
	}
	return env, warnings
}

func mergeSecurityScannerEnv(base, overrides []string) []string {
	merged := append([]string(nil), base...)
	for _, entry := range overrides {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		merged = upsertEnv(merged, key, value)
	}
	return merged
}

func prepareTrivyRuntimeCache() []string {
	var warnings []string
	runtimeDir := filepath.Join(securityRuntimeCacheDir, "trivy")
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		return []string{fmt.Sprintf("failed to create trivy runtime cache: %v", err)}
	}
	if directoryHasEntries(runtimeDir) {
		return warnings
	}
	seedDir := filepath.Join(securityImageCacheDir, "trivy")
	if !directoryHasEntries(seedDir) {
		return warnings
	}
	if err := copyDirectoryContents(seedDir, runtimeDir); err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to seed trivy runtime cache from image cache: %v", err))
	}
	return warnings
}

func directoryHasEntries(path string) bool {
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) > 0
}

func copyDirectoryContents(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}

func scannerCommandWarnings(label, stderr string, err error) []string {
	var warnings []string
	if trimmed := strings.TrimSpace(stderr); trimmed != "" {
		warnings = append(warnings, fmt.Sprintf("%s stderr: %s", label, truncateSecurityString(trimmed, 1000)))
	}
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("%s exited with %v; parser will use JSON output if available", label, err))
	}
	return warnings
}

func scannerParseFailureWarnings(label, stdout string, err error) []string {
	var warnings []string
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("%s JSON output could not be parsed: %v", label, err))
	}
	if trimmed := strings.TrimSpace(stdout); trimmed != "" {
		warnings = append(warnings, fmt.Sprintf("%s stdout was omitted because scanner parse failures may include sensitive findings", label))
	}
	return warnings
}

func semgrepConfigAvailable(config string) bool {
	config = strings.TrimSpace(config)
	if config == "" || config == "auto" || strings.Contains(config, "://") {
		return true
	}
	if !filepath.IsAbs(config) && !strings.HasPrefix(config, ".") {
		return true
	}
	info, err := os.Stat(config)
	if err != nil {
		return false
	}
	if !info.IsDir() {
		return true
	}
	return directoryHasEntries(config)
}

func marshalSecurityScannerResult(scanner string, findings []SecurityScanFinding, req securityScannerRequest, warnings []string) (string, error) {
	return marshalSecurityScannerCachedResult(scanner, "", false, findings, req, warnings)
}

func marshalSecurityScannerCachedResult(scanner, scanID string, cacheHit bool, findings []SecurityScanFinding, req securityScannerRequest, warnings []string) (string, error) {
	filtered, filterWarnings := filterSecurityFindings(findings, req)
	warnings = append(warnings, filterWarnings...)
	pageFindings, page, pageSize, hasMore := paginateSecurityFindings(filtered, req)
	returnedFindings := pageFindings
	if req.SummaryOnly {
		returnedFindings = []SecurityScanFinding{}
	} else if req.DetailLevel == "index" {
		returnedFindings = compactSecurityFindings(returnedFindings)
	}
	summary := summarizeSecurityFindings(filtered)
	result := SecurityScannerResult{
		Scanner:                 scanner,
		Summary:                 summary,
		SummaryBeforePagination: summary,
		TotalFindings:           len(filtered),
		ReturnedFindings:        len(returnedFindings),
		Page:                    page,
		PageSize:                pageSize,
		HasMore:                 hasMore,
		ScanID:                  scanID,
		CacheHit:                cacheHit,
		SummaryOnly:             req.SummaryOnly,
		DetailLevel:             req.DetailLevel,
		Findings:                returnedFindings,
		Warnings:                dedupeStrings(warnings),
	}
	if req.SummaryOnly || req.DetailLevel == "full" {
		result.Groups = summarizeSecurityFindingGroups(filtered)
	}
	if req.DetailLevel == "index" && !req.SummaryOnly {
		return marshalBoundedSecurityScannerIndexResult(result)
	}
	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal scanner result: %w", err)
	}
	return string(out), nil
}

func compactSecurityFindings(findings []SecurityScanFinding) []SecurityScanFinding {
	out := make([]SecurityScanFinding, 0, len(findings))
	for _, finding := range findings {
		finding.Title = truncateSecurityString(strings.TrimSpace(finding.Title), 237)
		finding.Message = ""
		finding.References = nil
		finding.CWEIDs = nil
		finding.Raw = nil
		out = append(out, finding)
	}
	return out
}

func marshalBoundedSecurityScannerIndexResult(result SecurityScannerResult) (string, error) {
	result.Compaction = &helpinCompactionHint{
		Exempt:   true,
		MaxRunes: boundedToolOutputCompactionMaxRunes,
		Mode:     "bounded_index",
	}
	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal scanner result: %w", err)
	}
	if len([]rune(string(out))) <= boundedToolOutputCompactionMaxRunes {
		return string(out), nil
	}

	originalFindings := result.Findings
	result.Bounded = true
	result.HasMore = true
	result.Warnings = dedupeStrings(append(result.Warnings, "response_bounded; narrow by category/package/CVE/rule/path or request next page"))
	low, high, best := 0, len(originalFindings), 0
	for low <= high {
		count := low + (high-low)/2
		result.Findings = originalFindings[:count]
		result.ReturnedFindings = count
		out, err = json.Marshal(result)
		if err != nil {
			return "", fmt.Errorf("marshal scanner result: %w", err)
		}
		if len([]rune(string(out))) <= boundedToolOutputCompactionMaxRunes {
			best = count
			low = count + 1
		} else {
			high = count - 1
		}
	}
	result.Findings = originalFindings[:best]
	result.ReturnedFindings = best
	out, err = json.Marshal(result)
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
		finding.Category = normalizeSecurityCategory(finding.Category)
		finding.Path = normalizeRepoOutputPath(finding.Path)
		if !pathIncluded(finding.Path, req.ScanPaths) {
			continue
		}
		if pathExcluded(finding.Path, excludes) {
			continue
		}
		if req.Category != "" && normalizeSecurityCategory(finding.Category) != req.Category {
			continue
		}
		if len(req.Paths) > 0 && !pathIncluded(finding.Path, req.Paths) {
			continue
		}
		if len(req.RuleIDs) > 0 && !stringInFoldedSet(finding.RuleID, req.RuleIDs) {
			continue
		}
		if len(req.PackageNames) > 0 && !stringPtrInFoldedSet(finding.PackageName, req.PackageNames) {
			continue
		}
		if len(req.VulnerabilityIDs) > 0 && !stringPtrInFoldedSet(finding.VulnerabilityID, req.VulnerabilityIDs) {
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
	if req.legacyMaxFindings && len(deduped) > req.MaxFindings {
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

func paginateSecurityFindings(findings []SecurityScanFinding, req securityScannerRequest) ([]SecurityScanFinding, int, int, bool) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}
	start := (page - 1) * pageSize
	if start >= len(findings) {
		return []SecurityScanFinding{}, page, pageSize, false
	}
	end := start + pageSize
	if end > len(findings) {
		end = len(findings)
	}
	return findings[start:end], page, pageSize, end < len(findings)
}

func summarizeSecurityFindingGroups(findings []SecurityScanFinding) []SecurityFindingGroup {
	type groupAccumulator struct {
		group    SecurityFindingGroup
		findings []SecurityScanFinding
	}
	groups := make(map[string]*groupAccumulator)
	add := func(kind, key string, finding SecurityScanFinding) {
		if strings.TrimSpace(key) == "" {
			return
		}
		mapKey := kind + "\x00" + key
		acc := groups[mapKey]
		if acc == nil {
			acc = &groupAccumulator{group: SecurityFindingGroup{
				Kind:            kind,
				Key:             key,
				Category:        normalizeSecurityCategory(finding.Category),
				Severity:        finding.Severity,
				RuleID:          finding.RuleID,
				PackageName:     derefString(finding.PackageName),
				VulnerabilityID: derefString(finding.VulnerabilityID),
				Path:            finding.Path,
			}}
			groups[mapKey] = acc
		}
		acc.findings = append(acc.findings, finding)
		acc.group.Count++
		if severityRank(finding.Severity) > severityRank(acc.group.Severity) {
			acc.group.Severity = finding.Severity
		}
	}
	for _, finding := range findings {
		add("severity", finding.Severity, finding)
		add("category", normalizeSecurityCategory(finding.Category), finding)
		add("rule", finding.RuleID, finding)
		if finding.PackageName != nil {
			add("package", *finding.PackageName, finding)
		}
		if finding.VulnerabilityID != nil {
			add("vulnerability", *finding.VulnerabilityID, finding)
		}
		add("path", finding.Path, finding)
	}
	out := make([]SecurityFindingGroup, 0, len(groups))
	for _, acc := range groups {
		acc.group.Summary = summarizeSecurityFindings(acc.findings)
		out = append(out, acc.group)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Key < out[j].Key
	})
	if len(out) > 200 {
		out = out[:200]
	}
	return out
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

func normalizeSecurityCategory(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "dependency", "vuln", "vulnerability", "package":
		return "dependency"
	case "sast", "code", "static":
		return "sast"
	case "secret", "secrets":
		return "secret"
	case "misconfig", "misconfiguration", "config", "iac":
		return "misconfig"
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

func normalizeStringFilters(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

func stringInFoldedSet(value string, filters []string) bool {
	value = strings.TrimSpace(value)
	for _, filter := range filters {
		if strings.EqualFold(value, strings.TrimSpace(filter)) {
			return true
		}
	}
	return false
}

func stringPtrInFoldedSet(value *string, filters []string) bool {
	if value == nil {
		return false
	}
	return stringInFoldedSet(*value, filters)
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
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
	return truncateSecurityString(value, limit)
}

func truncateSecurityString(value string, limit int) string {
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
