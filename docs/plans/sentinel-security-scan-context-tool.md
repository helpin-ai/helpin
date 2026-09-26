# Sentinel scanner tools and normalized findings

This undated historical plan explains why Sentinel uses dedicated scanner tools instead of parsing command output. It is useful to contributors maintaining security-triage contracts, but the proposed Go parser and scanner commands below are not an implementation inventory for this repository.

## Source review — 2026-09-18

- [The tool catalog](../../server/internal/agentcontract/tool_catalog.json) declares `scan_semgrep`, `scan_trivy`, and `scan_gitleaks`; [tool constants](../../server/internal/agentcontract/tool_constants.go) also normalize legacy `run_*` aliases. The proposed aggregate `get_security_scan_context` is not present.
- Current schemas add `summary_only`, `detail_level`, `page`, `page_size`, and category/rule/package/vulnerability/path filters. `max_findings` is documented as legacy; page size defaults to 100 with a maximum of 200. `scan_git_history` is already exposed, so the “later add” note below is historical.
- The [Sentinel template](../../server/internal/service/agent_templates.go) and [security-triage skill](../../server/skills/system/security_triage/SKILL.md) instruct summary-first scans, compact pagination, repository inspection, duplicate checks, and creation/commenting of remediation tasks. “Read-only” refers to repository files; Sentinel can mutate Helpin tasks and labels.
- The template command allowlist contains `git`, `rg`, `grep`, `find`, `cat`, `ls`, `head`, `tail`, and `pwd`, with no `python3`. This is the template configuration, not a claim that every custom agent in every deployment shares that allowlist.
- Scanner execution availability depends on the deployed Agent Runtime image and configuration. The Helpin tree contains catalog contracts, prompts, rules, and tests for template policy, but the proposed `SecurityScanFinding` parser implementation was not found here. This review therefore does not prove scanner CLI defaults, cache locations, normalization, redaction, or runtime error handling; verify those against the paired Runtime implementation before changing that contract.
- Each dedicated tool selects one scanner. The original “only fail when every requested scanner fails” wording belongs to an aggregate design; the current skill tells the agent to report one scanner's failure and continue with other configured scanner calls. An execution/parsing warning with zero findings is not proof of a clean scan.
- No security scan, runtime execution, vendor lookup, or parser test was run during this documentation review. JSON examples and cache/Temporal-worker assumptions below preserve the original proposal rather than verified deployment guarantees.

## Original plan

## Summary

Sentinel should not rely on arbitrary `python3` scripts to parse scanner output. Add dedicated backend scanner tools that run approved security scanners, capture full JSON output, normalize findings, redact sensitive values, and return compact triage-ready facts to the agent.

This keeps Sentinel constrained, avoids `run_command` output truncation, and prevents ad-hoc parser logic from drifting across agent runs. Use separate model-facing tools per scanner, with shared parser and normalization code underneath.

## Key Changes

- Add three built-in tools: `scan_semgrep`, `scan_trivy`, and `scan_gitleaks`.
- Use one shared backend parser/normalizer package so all scanner tools return the same finding schema.
- Execute each scanner from the repository workspace and parse JSON output inside the backend tool.
- Return normalized findings across SAST, dependency vulnerabilities, secrets, and misconfigurations.
- Do not add `python3` to the custom agent command allowlist.
- Keep `run_command` available only for lightweight repository inspection commands such as `git`, `rg`, `grep`, `find`, `cat`, `ls`, `head`, `tail`, and `pwd`.
- Expose scanner-specific failures separately so Sentinel can continue triage when one scanner fails.

## Tool Contracts

All scanner tools return the same normalized response shape. Scanner-specific tools keep model-facing contracts small while preserving scanner-specific options.

### `scan_semgrep`

Input:

```json
{
  "scan_paths": ["."],
  "exclude_paths": [],
  "config": "/app/security-rules/semgrep",
  "fallback_to_auto_config": true,
  "severity_threshold": "medium",
  "max_findings": 200,
  "include_low_info": false
}
```

Defaults:

- `scan_paths`: `["."]`
- `exclude_paths`: `[]`
- `config`: `/app/security-rules/semgrep`
- `fallback_to_auto_config`: `true`
- `severity_threshold`: `medium`
- `max_findings`: `200`
- `include_low_info`: `false`

### `scan_trivy`

Input:

```json
{
  "scan_paths": ["."],
  "exclude_paths": [],
  "scanners": ["vuln", "misconfig", "secret"],
  "severity_threshold": "medium",
  "max_findings": 200,
  "include_low_info": false
}
```

Defaults:

- `scan_paths`: `["."]`
- `exclude_paths`: `[]`
- `scanners`: `["vuln", "misconfig", "secret"]`
- `severity_threshold`: `medium`
- `max_findings`: `200`
- `include_low_info`: `false`

### `scan_gitleaks`

Input:

```json
{
  "scan_paths": ["."],
  "exclude_paths": [],
  "scan_git_history": false,
  "severity_threshold": "high",
  "max_findings": 200,
  "include_low_info": false
}
```

Defaults:

- `scan_paths`: `["."]`
- `exclude_paths`: `[]`
- `scan_git_history`: `false`
- `severity_threshold`: `high`
- `max_findings`: `200`
- `include_low_info`: `false`

### Shared Output

Output:

```json
{
  "scanner": "semgrep",
  "summary": {
    "total": 0,
    "critical": 0,
    "high": 0,
    "medium": 0,
    "low": 0,
    "info": 0
  },
  "findings": [
    {
      "scanner": "semgrep",
      "category": "sast",
      "severity": "high",
      "rule_id": "rule-id",
      "title": "Finding title",
      "message": "Finding explanation",
      "path": "relative/file/path.go",
      "start_line": 10,
      "end_line": 12,
      "package_name": null,
      "installed_version": null,
      "fixed_version": null,
      "vulnerability_id": null,
      "cwe_ids": ["CWE-89"],
      "cvss_score": null,
      "references": ["https://example.com/advisory"],
      "fingerprint": "stable-fingerprint",
      "raw": {
        "target": null,
        "dependency_file": null,
        "dependency_section": null
      }
    }
  ],
  "warnings": []
}
```

The implementation should not add an aggregate `get_security_scan_context` tool in v1. Sentinel should call only the scanner tools configured by the template prompt and then perform cross-scanner grouping itself.

## Shared Parser Rules

- Scanner output parsing is strict JSON parsing in Go structs. Do not parse scanner output with string matching except for defensive fallback of optional fields.
- Paths returned to the model must be repository-relative and must never include `/tmp/agent-workspace-*`, `/app`, or host paths.
- Severity is normalized to `critical`, `high`, `medium`, `low`, or `info`.
- If `include_low_info` is false, filter out `low` and `info` before applying `max_findings`.
- `severity_threshold` filters normalized findings before they are returned. Unknown severities map to `info` unless the scanner has an explicit numeric score that maps higher.
- `max_findings` caps returned findings after severity filtering and stable sorting. Include a warning when findings are truncated.
- Stable sort findings by severity rank descending, scanner name, repository path, start line, rule ID, then fingerprint.
- Secret findings must redact raw secret values, matched text, and code snippets before returning data to the agent.
- If one scanner fails, return findings from successful scanners plus a warning. Only fail the tool when every requested scanner fails.
- Dedupe exact duplicates by `(scanner, category, rule_id, path, start_line, fingerprint)` before returning.
- Emit per-scanner summaries before filtering and after filtering so the agent can explain suppressed scanner volume without seeing raw output.

## Semgrep Parser

Run command:

```json
{
  "program": "semgrep",
  "args": ["scan", "--config", "/app/security-rules/semgrep", "--json", "<scan_paths...>"]
}
```

Fallback command when local rules are unavailable:

```json
{
  "program": "semgrep",
  "args": ["scan", "--config", "auto", "--json", "<scan_paths...>"]
}
```

Parse shape:

```json
{
  "results": [
    {
      "check_id": "python.sqlalchemy.security.sqlalchemy-execute-raw-query",
      "path": "eventpipeline-retroactive/main.py",
      "start": {"line": 192, "col": 15},
      "end": {"line": 192, "col": 45},
      "extra": {
        "message": "Detected raw SQL execution...",
        "severity": "WARNING",
        "fingerprint": "...",
        "metadata": {
          "category": "security",
          "cwe": ["CWE-89"],
          "owasp": ["A03:2021"],
          "references": ["https://..."],
          "impact": "HIGH",
          "likelihood": "LOW"
        }
      }
    }
  ],
  "errors": []
}
```

Extraction rules:

- `scanner`: `semgrep`
- `category`: `sast`
- `rule_id`: `check_id`
- `title`: prefer `extra.metadata.shortlink` label or final segment of `check_id`; otherwise `check_id`
- `message`: `extra.message`
- `path`: `path`
- `start_line`: `start.line`
- `end_line`: `end.line`
- `fingerprint`: prefer `extra.fingerprint`; otherwise hash `semgrep:<check_id>:<path>:<start.line>:<end.line>:<message>`
- `references`: combine `extra.metadata.references`, `extra.metadata.source`, and `extra.metadata.shortlink` when they are URLs
- `cwe_ids`: parse strings from `extra.metadata.cwe`; preserve values like `CWE-89`
- `severity`: map `ERROR` to `high`, `WARNING` to `medium`, `INFO`/`INVENTORY`/`EXPERIMENT` to `info`; if metadata contains higher explicit risk such as `impact=HIGH` and `confidence=HIGH`, keep at least `high`
- Include metadata fields `impact`, `likelihood`, `confidence`, `owasp`, and `technology` under `raw` only if compact and non-sensitive.
- Do not return Semgrep code snippets by default. The agent can read affected files separately when triaging applicability.

## Trivy Parser

Run command:

```json
{
  "program": "trivy",
  "args": ["fs", "--cache-dir", "/app/.cache/trivy", "--format", "json", "--skip-version-check", "--scanners", "vuln,misconfig,secret", "--severity", "MEDIUM,HIGH,CRITICAL", "<scan_paths...>"]
}
```

Parse shape:

```json
{
  "Results": [
    {
      "Target": "docs/package-lock.json",
      "Class": "lang-pkgs",
      "Type": "npm",
      "Vulnerabilities": [],
      "Misconfigurations": [],
      "Secrets": []
    }
  ]
}
```

### Trivy Vulnerabilities

Parse from `Results[].Vulnerabilities[]`.

Important fields:

- `VulnerabilityID`
- `PkgName`
- `InstalledVersion`
- `FixedVersion`
- `Severity`
- `Title`
- `Description`
- `PrimaryURL`
- `References`
- `CVSS`
- `CweIDs`
- `PublishedDate`
- `LastModifiedDate`
- `PkgPath`

Extraction rules:

- `scanner`: `trivy`
- `category`: `dependency`
- `rule_id`: `VulnerabilityID`
- `vulnerability_id`: `VulnerabilityID`
- `title`: prefer `Title`; fallback `VulnerabilityID`
- `message`: compact `Description`, capped to 500 characters
- `path`: prefer `PkgPath`; fallback `Results[].Target`; fallback scan path
- `package_name`: `PkgName`
- `installed_version`: `InstalledVersion`
- `fixed_version`: `FixedVersion`
- `severity`: lowercase `Severity`
- `references`: `PrimaryURL` plus `References`, deduped and capped to 10 URLs
- `cwe_ids`: `CweIDs`
- `cvss_score`: highest available numeric score from `CVSS` vendor entries
- `fingerprint`: hash `trivy:vuln:<target>:<pkg_name>:<installed_version>:<vulnerability_id>`
- `raw.target`: `Results[].Target`
- `raw.dependency_file`: `Results[].Target`
- `raw.ecosystem`: `Results[].Type`

Task grouping hint:

- Group vulnerabilities by `path` plus package ecosystem when the same dependency update remediates multiple CVEs, such as `next` and related npm transitive vulnerabilities in `docs/package-lock.json`.

### Trivy Misconfigurations

Parse from `Results[].Misconfigurations[]`.

Important fields:

- `ID`
- `AVDID`
- `Type`
- `Title`
- `Description`
- `Message`
- `Resolution`
- `Severity`
- `PrimaryURL`
- `References`
- `CauseMetadata`

Extraction rules:

- `scanner`: `trivy`
- `category`: `misconfig`
- `rule_id`: prefer `AVDID`; fallback `ID`
- `title`: `Title`
- `message`: prefer `Message`; otherwise compact `Description`
- `path`: `Results[].Target`
- `start_line`: `CauseMetadata.StartLine`
- `end_line`: `CauseMetadata.EndLine`
- `severity`: lowercase `Severity`
- `references`: `PrimaryURL` plus `References`, deduped and capped to 10 URLs
- `fingerprint`: hash `trivy:misconfig:<target>:<rule_id>:<start_line>:<message>`
- `raw.target`: `Results[].Target`
- `raw.resolution`: `Resolution`

Do not return full `CauseMetadata.Code` unless a future UI needs it. It can contain more source context than the model needs.

### Trivy Secrets

Parse from `Results[].Secrets[]`.

Important fields commonly include:

- `RuleID`
- `Category`
- `Severity`
- `Title`
- `StartLine`
- `EndLine`
- `Match`
- `Code`

Extraction rules:

- `scanner`: `trivy`
- `category`: `secret`
- `rule_id`: `RuleID`
- `title`: prefer `Title`; fallback `RuleID`
- `message`: `Secret detected by Trivy rule <RuleID>. Raw secret value redacted.`
- `path`: `Results[].Target`
- `start_line`: `StartLine`
- `end_line`: `EndLine`
- `severity`: lowercase `Severity`; if absent, map to `high`
- `references`: empty unless scanner provides URLs
- `fingerprint`: hash `trivy:secret:<target>:<rule_id>:<start_line>`
- Never return `Match`, raw secret values, or `Code`.
- Include only compact metadata such as secret `Category` under `raw`.

## Gitleaks Parser

Run command:

```json
{
  "program": "gitleaks",
  "args": ["detect", "--source", ".", "--no-git", "--report-format", "json", "--redact"]
}
```

Parse shape:

```json
[
  {
    "RuleID": "generic-api-key",
    "Description": "Detected a Generic API Key...",
    "File": "rust-capture/.env.kafka",
    "StartLine": 2,
    "EndLine": 2,
    "StartColumn": 16,
    "EndColumn": 36,
    "Match": "KAFKA_PASSWORD=REDACTED",
    "Secret": "REDACTED",
    "Commit": "",
    "Entropy": 3.5,
    "Author": "",
    "Email": "",
    "Date": "",
    "Message": "",
    "Tags": ["key", "API", "generic"],
    "Fingerprint": "..."
  }
]
```

Extraction rules:

- `scanner`: `gitleaks`
- `category`: `secret`
- `rule_id`: `RuleID`
- `title`: `Description`
- `message`: `Potential secret detected by Gitleaks rule <RuleID>. Raw secret value redacted.`
- `path`: `File`
- `start_line`: `StartLine`
- `end_line`: `EndLine`
- `severity`: default `high`; map to `critical` only when future scanner metadata explicitly marks critical
- `references`: empty by default
- `fingerprint`: prefer `Fingerprint`; otherwise hash `gitleaks:<rule_id>:<file>:<start_line>:<start_column>:<end_column>`
- Never return `Secret`, `Match`, raw line content, author email, commit message, or git commit data unless a future explicit audit mode requires it.
- Include `Entropy` and `Tags` under `raw` because they help triage false positives without leaking secret content.

Gitleaks mode:

- V1 uses `--no-git` for working-tree scans to avoid expensive full-history scanning on every cron.
- Return a warning that historical git scanning was not performed.
- Later add a template variable `scan_git_history` for explicit full-history scans.

## Normalized Finding Type

Use one internal Go struct for all parsers:

```go
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
```

The tool response should also include:

```go
type SecurityScannerResult struct {
	Scanner  string                `json:"scanner"`
	Summary  SecurityScanSummary   `json:"summary"`
	Findings []SecurityScanFinding `json:"findings"`
	Warnings []string              `json:"warnings,omitempty"`
}
```

## Sentinel Skill And Template Updates

- Update the Sentinel skill to call `scan_semgrep`, `scan_trivy`, and/or `scan_gitleaks` according to the configured `scanners` list before triage.
- Instruct Sentinel not to inspect `/app/security-rules` or `/app/.cache` with file tools. Those are runtime paths for scanner execution, not repository files.
- Instruct Sentinel not to parse scanner JSON with `python3`, `jq`, or ad-hoc command pipelines.
- Keep fallback wording: if prewarmed cache or rules are missing, the scanner tools may rely on scanner-native fallback behavior, but the agent should not manage cache files manually.
- Update the Sentinel template allowed tools to include `scan_semgrep`, `scan_trivy`, and `scan_gitleaks`, and exclude `python3`.
- Keep `scan_paths` as an explicit template variable later. For v1, default to `["."]` and make the UI copy say the entire selected repository is scanned.

## Tests

- Add parser unit tests using fixtures for Semgrep, Trivy vulnerability, Trivy misconfiguration, Trivy secret, and Gitleaks output.
- Add redaction tests proving raw secrets and matched secret text are never returned.
- Add tool tests for severity filtering, finding caps, scanner command construction, scanner failure handling, and unsupported option rejection.
- Add template tests confirming Sentinel includes `scan_semgrep`, `scan_trivy`, and `scan_gitleaks`, and does not allow `python3`.
- Run backend tests and builds, plus frontend build if template UI text changes.

## Assumptions

- Sentinel remains constrained and read-only.
- Scanner parsing is backend infrastructure, not agent reasoning.
- The tool runs from the hydrated repository workspace.
- Kubernetes Temporal workers run with a read-only root filesystem, so scanner tools must use writable runtime caches under `/tmp/helpin-security-cache`.
- Trivy may seed that writable runtime cache from the baked read-only image cache under `/app/.cache/trivy`.
- Scanner outputs are not written into tracked repository files by the agent.
- `python3` remains unavailable to custom agents unless a separate sandboxed scripting capability is designed later.
