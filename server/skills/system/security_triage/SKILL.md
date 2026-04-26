---
name: security_triage
description: Triage security scanner output from repository scans, suppress false positives, and create actionable remediation tasks for applicable medium-or-higher findings.
---

# Security Triage

Use this skill when an agent runs security scanners against a repository and must decide which findings are real enough to become Helpin tasks.

## Core Rules

- Keep the repository read-only. Do not edit files, manifests, lockfiles, generated output, or scanner configuration.
- Treat the template prompt configuration as authoritative: scanners, severity threshold, destination team/stage, max task count, and cadence are already resolved.
- Scanner output is evidence, not truth. Inspect the affected code/configuration before creating a task.
- Create tasks only for findings classified as `applicable` and at or above the configured severity threshold.
- Suppress false positives, non-reachable issues, duplicate findings, and low/info findings unless the prompt explicitly allows them.
- Group related findings by root cause before creating tasks.

## Plan

Start with `update_plan` using these phases:

1. Confirm configured scanners and read-only repository scope.
2. Run the configured dedicated scanner tools and collect normalized findings.
3. Review normalized findings by scanner, rule, severity, file, line, and evidence.
4. Inspect affected code/configuration to classify applicability.
5. Group applicable findings by root cause and create remediation tasks.
6. Summarize created tasks, suppressed findings, and limitations.

## Scanner Tools

Run only configured scanners with the dedicated scanner tools. Do not run scanner CLIs through `run_command`, do not save scanner JSON files, and do not parse scanner output with Python, jq, shell pipelines, or ad-hoc scripts.

Use these tools according to the configured `scanners` list:

- `scan_semgrep`
- `scan_trivy`
- `scan_gitleaks`

Pass the configured severity threshold, `include_low_info`, and scan scope if available. If no scan scope is configured, scan the entire selected repository with `scan_paths: ["."]`.

Important path rule:

- Repository file tools such as `list_directory`, `read_file`, and `search_files` are scoped to the checked-out repository. Do not use them to inspect `/app/security-rules`, `/app/.cache`, or any other absolute runtime path.
- Treat `/app/security-rules/semgrep` and `/app/.cache/trivy` as runtime paths owned by the scanner tools only.
- Do not spend tool calls checking whether those runtime paths exist. Let the scanner tools handle bundled rules, caches, and scanner-native fallbacks.

Semgrep:
```json
{
  "scan_paths": ["."],
  "severity_threshold": "medium",
  "include_low_info": false
}
```

Trivy:
```json
{
  "scan_paths": ["."],
  "scanners": ["vuln", "misconfig", "secret"],
  "severity_threshold": "medium",
  "include_low_info": false
}
```

Gitleaks:
```json
{
  "scan_paths": ["."],
  "scan_git_history": false,
  "severity_threshold": "high",
  "include_low_info": false
}
```

If a scanner is unavailable, fails to execute, or cannot download required data, record the limitation and continue with the remaining scanners. Do not fabricate findings.

The worker image prewarms Trivy vulnerability databases under `/app/.cache/trivy` and ships local Semgrep rules under `/app/security-rules/semgrep`. The scanner tools prefer these paths so scheduled runs do not depend on upstream registry downloads, but they may use scanner-native fallbacks when cache/rules are absent.

## Applicability Triage

Classify every medium-or-higher finding as one of:

- `applicable`: The issue is present in reachable code/configuration, has a plausible exploit or operational risk, and has a clear remediation path.
- `false_positive`: The scanner matched a pattern that is not exploitable in this repository context.
- `not_actionable`: The finding is real but cannot be remediated from this repository, lacks a useful fix, or is intentionally accepted by project configuration.
- `needs_more_context`: Evidence is incomplete; do not create a task unless the risk is clearly high and the task can ask for specific verification.

Use repository context to check:

- Is the affected file used in production, tests, examples, docs, generated code, or fixtures?
- Is tainted input actually attacker-controlled?
- Is a vulnerable dependency used on an affected code path?
- Is a secret real-looking or a placeholder/test token?
- Is a misconfiguration applied to deployable infrastructure or only a sample?
- Is there a safer local mitigation than a broad rewrite?

## Grouping

Create at most one task per root cause. Group findings when they share:

- Same scanner rule and remediation pattern.
- Same vulnerable package/image/config object.
- Same secret type and leak location pattern.
- Same source/sink pair or data-flow issue.
- Same infrastructure control gap.

Do not create duplicate tasks for the same issue across Semgrep, Trivy, and Gitleaks. Mention corroborating scanners in the task description.

## Task Creation

Use `create_task` once per applicable root-cause group, up to `max_tasks`.

Task name:
`Fix <severity> security issue: <short root cause>`

Task type: `chore`

Priority:

- `high` for critical/high findings, confirmed secrets, exploitable injection/authz issues, or security fixes with known CVE/GHSA/OSV IDs.
- `medium` for applicable medium findings.
- Do not create low-priority tasks unless explicitly configured.

Description must include:

- Severity and scanner/rule IDs.
- Affected files and line numbers.
- Why the finding is applicable in this repository.
- False-positive checks performed.
- Recommended remediation steps.
- Verification command to rerun after the fix.
- Source URLs or advisory IDs when available.
- Suppressed duplicate/corroborating findings in the same group.

Acceptance criteria:

- The vulnerable code/configuration/dependency/secret exposure is remediated.
- The relevant scanner no longer reports this finding.
- Tests or deploy validation relevant to the change pass.
- Any migration or operational follow-up is documented.

Source refs should include scanner/advisory/release URLs where verified. If `create_task` does not support labels, include suggested labels in the description: `security`, scanner name, severity, and `false-positive-triaged`.

## Final Response

Summarize:

- Scanners run and any scanner failures.
- Findings by severity before triage.
- Tasks created with task IDs/refs.
- Findings suppressed as false positive/not actionable.
- Limitations and what should be checked manually.
