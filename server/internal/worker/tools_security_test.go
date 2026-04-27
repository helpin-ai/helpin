package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestParseSemgrepFindingsNormalizesMetadata(t *testing.T) {
	input := []byte(`{
		"results": [{
			"check_id": "python.sqlalchemy.security.sqlalchemy-execute-raw-query",
			"path": "eventpipeline-retroactive/main.py",
			"start": {"line": 192, "col": 15},
			"end": {"line": 192, "col": 45},
			"extra": {
				"message": "Detected raw SQL execution.",
				"severity": "WARNING",
				"fingerprint": "semgrep-fp",
				"metadata": {
					"category": "security",
					"cwe": ["CWE-89"],
					"owasp": ["A03:2021"],
					"references": ["https://semgrep.dev/docs"],
					"impact": "HIGH",
					"likelihood": "LOW"
				}
			}
		}],
		"errors": []
	}`)

	findings, warnings, err := parseSemgrepFindings(input)
	if err != nil {
		t.Fatalf("parseSemgrepFindings returned error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	finding := findings[0]
	if finding.Scanner != "semgrep" || finding.Category != "sast" || finding.Severity != "medium" {
		t.Fatalf("unexpected normalized finding: %+v", finding)
	}
	if finding.RuleID != "python.sqlalchemy.security.sqlalchemy-execute-raw-query" || finding.Path != "eventpipeline-retroactive/main.py" {
		t.Fatalf("unexpected identity fields: %+v", finding)
	}
	if finding.StartLine == nil || *finding.StartLine != 192 || finding.EndLine == nil || *finding.EndLine != 192 {
		t.Fatalf("unexpected location: %+v", finding)
	}
	if len(finding.CWEIDs) != 1 || finding.CWEIDs[0] != "CWE-89" {
		t.Fatalf("expected CWE metadata, got %v", finding.CWEIDs)
	}
	if len(finding.References) != 1 || finding.References[0] != "https://semgrep.dev/docs" {
		t.Fatalf("expected reference URL, got %v", finding.References)
	}
}

func TestParseTrivyFindingsNormalizesVulnerabilityMisconfigAndSecret(t *testing.T) {
	input := []byte(`{
		"Results": [{
			"Target": "docs/package-lock.json",
			"Class": "lang-pkgs",
			"Type": "npm",
			"Vulnerabilities": [{
				"VulnerabilityID": "CVE-2025-29927",
				"PkgName": "next",
				"InstalledVersion": "13.0.6",
				"FixedVersion": "13.5.9",
				"Severity": "CRITICAL",
				"Title": "Next.js authorization bypass",
				"Description": "Middleware authorization bypass.",
				"PrimaryURL": "https://avd.aquasec.com/nvd/cve-2025-29927",
				"References": ["https://github.com/vercel/next.js/security/advisories/GHSA-example"],
				"CweIDs": ["CWE-287"],
				"CVSS": {"nvd": {"V3Score": 9.1}}
			}],
			"Misconfigurations": [{
				"ID": "DS002",
				"AVDID": "AVD-DS-0002",
				"Title": "Root user",
				"Message": "Specify at least 1 USER command.",
				"Severity": "MEDIUM",
				"PrimaryURL": "https://avd.aquasec.com/misconfig/ds002",
				"CauseMetadata": {"StartLine": 1, "EndLine": 12}
			}],
			"Secrets": [{
				"RuleID": "aws-access-key-id",
				"Category": "AWS",
				"Severity": "HIGH",
				"Title": "AWS access key",
				"StartLine": 4,
				"EndLine": 4,
				"Match": "AKIASECRET",
				"Code": {"Lines": [{"Content": "secret"}]}
			}]
		}]
	}`)

	findings, warnings, err := parseTrivyFindings(input)
	if err != nil {
		t.Fatalf("parseTrivyFindings returned error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(findings))
	}

	vuln := findings[0]
	if vuln.Category != "dependency" || vuln.Severity != "critical" || vuln.RuleID != "CVE-2025-29927" {
		t.Fatalf("unexpected vulnerability finding: %+v", vuln)
	}
	if vuln.PackageName == nil || *vuln.PackageName != "next" || vuln.FixedVersion == nil || *vuln.FixedVersion != "13.5.9" {
		t.Fatalf("unexpected vulnerability package fields: %+v", vuln)
	}
	if vuln.CVSSScore == nil || *vuln.CVSSScore != 9.1 {
		t.Fatalf("expected CVSS 9.1, got %+v", vuln.CVSSScore)
	}

	misconfig := findings[1]
	if misconfig.Category != "misconfig" || misconfig.RuleID != "AVD-DS-0002" || misconfig.StartLine == nil || *misconfig.StartLine != 1 {
		t.Fatalf("unexpected misconfig finding: %+v", misconfig)
	}

	secret := findings[2]
	if secret.Category != "secret" || secret.Severity != "high" || secret.RuleID != "aws-access-key-id" {
		t.Fatalf("unexpected secret finding: %+v", secret)
	}
	encoded, _ := json.Marshal(secret)
	if strings.Contains(string(encoded), "AKIASECRET") {
		t.Fatalf("secret finding leaked raw secret data: %s", encoded)
	}
}

func TestParseGitleaksFindingsRedactsSecrets(t *testing.T) {
	input := []byte(`[{
		"RuleID": "generic-api-key",
		"Description": "Detected a Generic API Key",
		"File": "rust-capture/.env.kafka",
		"StartLine": 2,
		"EndLine": 2,
		"StartColumn": 16,
		"EndColumn": 36,
		"Match": "KAFKA_PASSWORD=REAL_SECRET",
		"Secret": "REAL_SECRET",
		"Entropy": 3.5,
		"Tags": ["key", "generic"],
		"Fingerprint": "gitleaks-fp"
	}]`)

	findings, warnings, err := parseGitleaksFindings(input)
	if err != nil {
		t.Fatalf("parseGitleaksFindings returned error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	finding := findings[0]
	if finding.Scanner != "gitleaks" || finding.Category != "secret" || finding.Severity != "high" {
		t.Fatalf("unexpected gitleaks finding: %+v", finding)
	}
	if finding.Path != "rust-capture/.env.kafka" || finding.Fingerprint != "gitleaks-fp" {
		t.Fatalf("unexpected gitleaks identity fields: %+v", finding)
	}
	encoded, _ := json.Marshal(finding)
	if strings.Contains(string(encoded), "REAL_SECRET") || strings.Contains(string(encoded), "KAFKA_PASSWORD=REAL_SECRET") {
		t.Fatalf("gitleaks finding leaked raw secret data: %s", encoded)
	}
}

func TestScanGitleaksCommandWritesJSONToStdoutAndDoesNotFailOnFindings(t *testing.T) {
	req := securityScannerRequest{}
	args := gitleaksCommandArgs(req)
	joined := strings.Join(args, " ")
	for _, expected := range []string{"--report-format json", "--report-path -", "--redact", "--exit-code 0", "--no-git"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected gitleaks args to contain %q, got %v", expected, args)
		}
	}

	req.ScanGitHistory = true
	args = gitleaksCommandArgs(req)
	if strings.Contains(strings.Join(args, " "), "--no-git") {
		t.Fatalf("did not expect --no-git when scan_git_history is true: %v", args)
	}
}

func TestSecurityScannerEnvUsesWritableRuntimeCache(t *testing.T) {
	env, warnings := securityScannerEnv()
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	joined := strings.Join(env, "\n")
	for _, expected := range []string{
		"HOME=/tmp/helpin-security-cache/home",
		"XDG_CACHE_HOME=/tmp/helpin-security-cache",
		"SEMGREP_SETTINGS_FILE=/tmp/helpin-security-cache/semgrep/settings.yml",
		"TRIVY_CACHE_DIR=/tmp/helpin-security-cache/trivy",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected env to contain %q, got %v", expected, env)
		}
	}
}

func TestMergeSecurityScannerEnvOverridesReadOnlyImageDefaults(t *testing.T) {
	base := []string{
		"PATH=/usr/local/bin:/usr/bin",
		"HOME=/home/app",
		"XDG_CACHE_HOME=/app/.cache",
		"SEMGREP_SETTINGS_FILE=/app/.cache/semgrep/settings.yml",
	}
	overrides := []string{
		"HOME=/tmp/helpin-security-cache/home",
		"XDG_CACHE_HOME=/tmp/helpin-security-cache",
		"SEMGREP_SETTINGS_FILE=/tmp/helpin-security-cache/semgrep/settings.yml",
	}

	merged := mergeSecurityScannerEnv(base, overrides)
	lookup := map[string]string{}
	counts := map[string]int{}
	for _, entry := range merged {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		lookup[key] = value
		counts[key]++
	}

	for key, want := range map[string]string{
		"HOME":                  "/tmp/helpin-security-cache/home",
		"XDG_CACHE_HOME":        "/tmp/helpin-security-cache",
		"SEMGREP_SETTINGS_FILE": "/tmp/helpin-security-cache/semgrep/settings.yml",
	} {
		if got := lookup[key]; got != want {
			t.Fatalf("expected %s=%q, got %q in %v", key, want, got, merged)
		}
		if counts[key] != 1 {
			t.Fatalf("expected %s to appear once, got %d in %v", key, counts[key], merged)
		}
	}
}

func TestFilterSecurityFindingsAppliesThresholdExcludesAndCaps(t *testing.T) {
	findings := []SecurityScanFinding{
		{Scanner: "semgrep", Category: "sast", Severity: "medium", RuleID: "m", Path: "src/a.go", Fingerprint: "1"},
		{Scanner: "trivy", Category: "dependency", Severity: "critical", RuleID: "c", Path: "docs/package-lock.json", Fingerprint: "2"},
		{Scanner: "gitleaks", Category: "secret", Severity: "high", RuleID: "s", Path: "fixtures/.env", Fingerprint: "3"},
		{Scanner: "gitleaks", Category: "secret", Severity: "high", RuleID: "s", Path: "fixtures/.env", Fingerprint: "3"},
	}
	filtered, warnings := filterSecurityFindings(findings, securityScannerRequest{
		ExcludePaths:      []string{"fixtures"},
		SeverityThreshold: "medium",
		MaxFindings:       1,
		legacyMaxFindings: true,
	})
	if len(filtered) != 1 {
		t.Fatalf("expected 1 finding after exclude, dedupe, cap; got %+v", filtered)
	}
	if filtered[0].RuleID != "c" {
		t.Fatalf("expected critical finding first, got %+v", filtered[0])
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "truncated") {
		t.Fatalf("expected truncation warning, got %v", warnings)
	}
}

func TestSecurityScannerPaginationAndFilters(t *testing.T) {
	findings := []SecurityScanFinding{
		{Scanner: "trivy", Category: "dependency", Severity: "critical", RuleID: "CVE-1", Path: "app/package-lock.json", PackageName: stringPtr("next"), VulnerabilityID: stringPtr("CVE-1"), Fingerprint: "1"},
		{Scanner: "trivy", Category: "dependency", Severity: "high", RuleID: "CVE-2", Path: "app/package-lock.json", PackageName: stringPtr("next"), VulnerabilityID: stringPtr("CVE-2"), Fingerprint: "2"},
		{Scanner: "trivy", Category: "dependency", Severity: "high", RuleID: "CVE-3", Path: "api/poetry.lock", PackageName: stringPtr("django"), VulnerabilityID: stringPtr("CVE-3"), Fingerprint: "3"},
		{Scanner: "trivy", Category: "secret", Severity: "high", RuleID: "secret", Path: ".env", Fingerprint: "4"},
	}

	out, err := marshalSecurityScannerResult("trivy", findings, securityScannerRequest{
		SeverityThreshold: "high",
		Category:          "dependency",
		PackageNames:      []string{"next"},
		Page:              1,
		PageSize:          1,
	}, nil)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	result := decodeSecurityScannerResult(t, out)
	if result.TotalFindings != 2 || result.ReturnedFindings != 1 || !result.HasMore {
		t.Fatalf("unexpected pagination metadata: %+v", result)
	}
	if len(result.Findings) != 1 || *result.Findings[0].PackageName != "next" {
		t.Fatalf("unexpected findings: %+v", result.Findings)
	}
	if result.Summary.Critical != 1 || result.Summary.High != 1 {
		t.Fatalf("summary should cover all filtered findings before pagination: %+v", result.Summary)
	}
}

func TestSecurityScannerSummaryOnlyOmitsFindingsAndReturnsGroups(t *testing.T) {
	findings := []SecurityScanFinding{
		{Scanner: "trivy", Category: "dependency", Severity: "critical", RuleID: "CVE-1", Path: "app/package-lock.json", PackageName: stringPtr("next"), VulnerabilityID: stringPtr("CVE-1"), Fingerprint: "1"},
		{Scanner: "trivy", Category: "dependency", Severity: "high", RuleID: "CVE-2", Path: "app/package-lock.json", PackageName: stringPtr("next"), VulnerabilityID: stringPtr("CVE-2"), Fingerprint: "2"},
	}

	out, err := marshalSecurityScannerResult("trivy", findings, securityScannerRequest{
		SeverityThreshold: "high",
		SummaryOnly:       true,
		Page:              1,
		PageSize:          1,
	}, nil)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	result := decodeSecurityScannerResult(t, out)
	if !result.SummaryOnly || len(result.Findings) != 0 || result.ReturnedFindings != 0 {
		t.Fatalf("expected summary-only result without findings: %+v", result)
	}
	if result.TotalFindings != 2 || len(result.Groups) == 0 {
		t.Fatalf("expected summary groups, got %+v", result)
	}
}

func TestSecurityScannerIndexOmitsVerboseFieldsAndAddsCompactionHint(t *testing.T) {
	findings := []SecurityScanFinding{{
		Scanner:          "trivy",
		Category:         "dependency",
		Severity:         "critical",
		RuleID:           "CVE-1",
		Title:            strings.Repeat("title", 80),
		Message:          strings.Repeat("verbose advisory ", 100),
		Path:             "app/package-lock.json",
		PackageName:      stringPtr("next"),
		InstalledVersion: stringPtr("13.0.0"),
		FixedVersion:     stringPtr("13.5.9"),
		VulnerabilityID:  stringPtr("CVE-1"),
		CWEIDs:           []string{"CWE-79"},
		References:       []string{"https://example.com/advisory"},
		Fingerprint:      "fp-1",
		Raw:              map[string]interface{}{"target": "raw"},
	}}

	out, err := marshalSecurityScannerResult("trivy", findings, securityScannerRequest{
		SeverityThreshold: "high",
		DetailLevel:       "index",
		Page:              1,
		PageSize:          10,
	}, nil)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if !strings.Contains(out, `"_helpin_compaction"`) {
		t.Fatalf("expected compaction hint in index result: %s", out)
	}
	result := decodeSecurityScannerResult(t, out)
	if result.DetailLevel != "index" || result.Compaction == nil || !result.Compaction.Exempt {
		t.Fatalf("expected index result with compaction hint, got %+v", result)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("expected one finding, got %+v", result.Findings)
	}
	finding := result.Findings[0]
	if finding.Message != "" || len(finding.References) != 0 || len(finding.CWEIDs) != 0 || finding.Raw != nil {
		t.Fatalf("expected verbose fields omitted in index finding, got %+v", finding)
	}
	if len([]rune(finding.Title)) > 240 {
		t.Fatalf("expected title to be truncated, got %d runes", len([]rune(finding.Title)))
	}
}

func TestSecurityScannerIndexBoundsOversizedResponse(t *testing.T) {
	findings := make([]SecurityScanFinding, 0, 80)
	for i := 0; i < 80; i++ {
		findings = append(findings, SecurityScanFinding{
			Scanner:     "trivy",
			Category:    "dependency",
			Severity:    "high",
			RuleID:      fmt.Sprintf("CVE-%d", i),
			Title:       strings.Repeat("large-title", 100),
			Path:        fmt.Sprintf("service/%s/package-lock.json", strings.Repeat("deep-path/", 20)),
			PackageName: stringPtr(fmt.Sprintf("package-%d-%s", i, strings.Repeat("x", 200))),
			Fingerprint: fmt.Sprintf("fp-%d", i),
		})
	}

	out, err := marshalSecurityScannerResult("trivy", findings, securityScannerRequest{
		SeverityThreshold: "high",
		DetailLevel:       "index",
		Page:              1,
		PageSize:          80,
	}, nil)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if len([]rune(out)) > boundedToolOutputCompactionMaxRunes {
		t.Fatalf("expected bounded output <= %d runes, got %d", boundedToolOutputCompactionMaxRunes, len([]rune(out)))
	}
	result := decodeSecurityScannerResult(t, out)
	if !result.Bounded || !result.HasMore {
		t.Fatalf("expected bounded result with has_more, got %+v", result)
	}
	if result.ReturnedFindings >= result.TotalFindings {
		t.Fatalf("expected partial findings after bounding, got %+v", result)
	}
	if !containsWarning(result.Warnings, "response_bounded") {
		t.Fatalf("expected response_bounded warning, got %v", result.Warnings)
	}
}

func TestSecurityScannerToolsAreRegistered(t *testing.T) {
	registry := NewToolRegistry(nil)
	defs := registry.Definitions()
	for _, name := range []string{ToolScanSemgrep, ToolScanTrivy, ToolScanGitleaks} {
		found := false
		for _, def := range defs {
			if def.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected tool %q to be registered", name)
		}
	}
}

func TestSecurityScannerToolsAreHighVolumeForModelCompaction(t *testing.T) {
	for _, name := range []string{ToolScanSemgrep, ToolScanTrivy, ToolScanGitleaks} {
		if !isHighVolumeToolOutput(name) {
			t.Fatalf("expected %s to be treated as high-volume output", name)
		}
	}
}

func TestScanSemgrepMissingBundledConfigFallsBackToAuto(t *testing.T) {
	var capturedArgs []string
	withSecurityScannerCommandRunner(t, func(ctx *ExecutionContext, program string, args []string, env []string) (string, string, error) {
		if program != "semgrep" {
			t.Fatalf("expected semgrep, got %s", program)
		}
		capturedArgs = append([]string(nil), args...)
		return `{"results":[],"errors":[]}`, "", nil
	})

	out, err := toolScanSemgrep(&ExecutionContext{
		Context: context.Background(),
		WorkDir: t.TempDir(),
	}, json.RawMessage(`{"config":"/tmp/helpin-missing-semgrep-rules","scan_paths":["."],"severity_threshold":"high"}`))
	if err != nil {
		t.Fatalf("toolScanSemgrep returned error: %v", err)
	}
	result := decodeSecurityScannerResult(t, out)
	if !strings.Contains(strings.Join(capturedArgs, " "), "--config auto") {
		t.Fatalf("expected semgrep auto fallback args, got %v", capturedArgs)
	}
	if !containsWarning(result.Warnings, "unavailable or empty") {
		t.Fatalf("expected missing config warning, got %v", result.Warnings)
	}
}

func TestScanSemgrepCommandFailureReturnsStructuredResult(t *testing.T) {
	withSecurityScannerCommandRunner(t, func(ctx *ExecutionContext, program string, args []string, env []string) (string, string, error) {
		return "", "semgrep registry unavailable", errors.New("exit status 1")
	})

	out, err := toolScanSemgrep(&ExecutionContext{
		Context: context.Background(),
		WorkDir: t.TempDir(),
	}, json.RawMessage(`{"config":"auto","scan_paths":["."],"severity_threshold":"high"}`))
	if err != nil {
		t.Fatalf("toolScanSemgrep returned error: %v", err)
	}
	result := decodeSecurityScannerResult(t, out)
	if result.Scanner != "semgrep" || result.Summary.Total != 0 || len(result.Findings) != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !containsWarning(result.Warnings, "semgrep registry unavailable") || !containsWarning(result.Warnings, "returning zero findings") {
		t.Fatalf("expected failure warnings, got %v", result.Warnings)
	}
}

func TestScanTrivyCommandFailureWithInvalidOutputReturnsStructuredResult(t *testing.T) {
	withSecurityScannerCommandRunner(t, func(ctx *ExecutionContext, program string, args []string, env []string) (string, string, error) {
		return "Scan path must be a single target", "", errors.New("exit status 1")
	})

	out, err := toolScanTrivy(&ExecutionContext{
		Context: context.Background(),
		WorkDir: t.TempDir(),
	}, json.RawMessage(`{"scan_paths":["poetry.lock","package-lock.json"],"scanners":["vuln"],"severity_threshold":"high"}`))
	if err != nil {
		t.Fatalf("toolScanTrivy returned error: %v", err)
	}
	result := decodeSecurityScannerResult(t, out)
	if result.Scanner != "trivy" || result.Summary.Total != 0 || len(result.Findings) != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !containsWarning(result.Warnings, "JSON output could not be parsed") || !containsWarning(result.Warnings, "stdout was omitted") {
		t.Fatalf("expected parse failure warnings, got %v", result.Warnings)
	}
	if containsWarning(result.Warnings, "Scan path must be a single target") {
		t.Fatalf("expected raw scanner stdout to be omitted, got %v", result.Warnings)
	}
}

func TestScanTrivyReusesCachedFindingsForPaginationAndFilters(t *testing.T) {
	calls := 0
	withSecurityScannerCommandRunner(t, func(ctx *ExecutionContext, program string, args []string, env []string) (string, string, error) {
		calls++
		if program != "trivy" {
			t.Fatalf("expected trivy, got %s", program)
		}
		return `{
			"Results": [{
				"Target": "package-lock.json",
				"Type": "npm",
				"Vulnerabilities": [{
					"VulnerabilityID": "CVE-1",
					"PkgName": "next",
					"InstalledVersion": "13.0.6",
					"FixedVersion": "13.5.9",
					"Severity": "CRITICAL",
					"Title": "Next.js auth bypass"
				}, {
					"VulnerabilityID": "CVE-2",
					"PkgName": "cross-spawn",
					"InstalledVersion": "5.1.0",
					"FixedVersion": "7.0.5",
					"Severity": "HIGH",
					"Title": "ReDoS"
				}]
			}]
		}`, "", nil
	})

	execCtx := &ExecutionContext{
		Context: context.Background(),
		WorkDir: t.TempDir(),
	}
	firstOut, err := toolScanTrivy(execCtx, json.RawMessage(`{"scan_paths":["."],"scanners":["vuln"],"severity_threshold":"high","summary_only":true}`))
	if err != nil {
		t.Fatalf("first toolScanTrivy returned error: %v", err)
	}
	first := decodeSecurityScannerResult(t, firstOut)
	if first.CacheHit {
		t.Fatalf("first scan should not be a cache hit: %+v", first)
	}
	if first.ScanID == "" {
		t.Fatalf("expected first scan_id")
	}
	if first.ReturnedFindings != 0 || len(first.Findings) != 0 || first.TotalFindings != 2 {
		t.Fatalf("unexpected first summary result: %+v", first)
	}

	secondOut, err := toolScanTrivy(execCtx, json.RawMessage(`{"scan_paths":["."],"scanners":["vuln"],"severity_threshold":"high","package_names":["next"],"page":1,"page_size":1}`))
	if err != nil {
		t.Fatalf("second toolScanTrivy returned error: %v", err)
	}
	second := decodeSecurityScannerResult(t, secondOut)
	if !second.CacheHit {
		t.Fatalf("second scan should be a cache hit: %+v", second)
	}
	if second.ScanID != first.ScanID {
		t.Fatalf("expected reused scan_id %q, got %q", first.ScanID, second.ScanID)
	}
	if calls != 1 {
		t.Fatalf("expected scanner command to run once, got %d", calls)
	}
	if second.TotalFindings != 1 || len(second.Findings) != 1 || second.Findings[0].PackageName == nil || *second.Findings[0].PackageName != "next" {
		t.Fatalf("expected cached filtered next finding, got %+v", second)
	}
}

func withSecurityScannerCommandRunner(t *testing.T, runner func(ctx *ExecutionContext, program string, args []string, env []string) (string, string, error)) {
	t.Helper()

	previous := securityScannerCommandRunner
	securityScannerCommandRunner = runner
	t.Cleanup(func() {
		securityScannerCommandRunner = previous
	})
}

func decodeSecurityScannerResult(t *testing.T, out string) SecurityScannerResult {
	t.Helper()

	var result SecurityScannerResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode scanner result: %v\n%s", err, out)
	}
	return result
}

func containsWarning(warnings []string, needle string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, needle) {
			return true
		}
	}
	return false
}
