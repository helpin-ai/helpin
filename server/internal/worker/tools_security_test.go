package worker

import (
	"encoding/json"
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
		"XDG_CACHE_HOME=/tmp/helpin-security-cache",
		"SEMGREP_SETTINGS_FILE=/tmp/helpin-security-cache/semgrep/settings.yml",
		"TRIVY_CACHE_DIR=/tmp/helpin-security-cache/trivy",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected env to contain %q, got %v", expected, env)
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
