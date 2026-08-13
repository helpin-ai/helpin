---
name: security_triage
description: Triage security scanner output from repository scans, suppress false positives, and create actionable remediation tasks for applicable medium-or-higher findings.
metadata:
  title: Security Triage
  supported_runtimes:
    - native_sdk
---

# Security Triage

Use this skill when an agent runs security scanners against a repository and must decide which findings are real enough to become Helpin tasks.

## Core Rules

- Keep the repository read-only. Do not edit files, manifests, lockfiles, generated output, or scanner configuration.
- Treat the template prompt configuration as authoritative: scanners, severity threshold, destination team/stage, max task count, and cadence are already resolved.
- Scanner output is evidence, not truth. Inspect the affected code/configuration before creating a task.
- Create tasks only for findings classified as `applicable` and at or above the configured severity threshold.
- Suppress false positives, non-reachable issues, duplicate findings, and low/info findings unless the prompt explicitly allows them.
- Group related findings by fix unit before creating tasks.
- Every created task must carry the shared `security` label.
- Repeated runs must be idempotent: do not create a new task when an open matching `security` task already exists.

## Plan

Start with `update_plan` using these phases:

1. Confirm configured scanners and read-only repository scope.
2. Ensure the shared `security` label exists and list open tasks with that label.
3. Run the configured dedicated scanner tools and collect normalized findings.
4. Review normalized findings by scanner, rule, severity, file, line, and evidence.
5. Inspect affected code/configuration to classify applicability.
6. Match applicable findings against existing open security tasks.
7. Create new remediation tasks or comment on existing matching tasks.
8. Summarize created tasks, updated existing tasks, suppressed findings, and limitations.

## Scanner Tools

Run only configured scanners with the dedicated scanner tools. Do not run scanner CLIs through `run_command`, do not save scanner JSON files, and do not parse scanner output with Python, jq, shell pipelines, or ad-hoc scripts.

Use these tools according to the configured `scanners` list:

- `scan_semgrep`
- `scan_trivy`
- `scan_gitleaks`

Use scanner pagination and filtering instead of rerunning a scanner when output is compacted. Start with `summary_only: true` for counts/groups only, then request `detail_level: "index"` for compact finding rows. Use `detail_level: "full"` only for narrow follow-up inspection.

Summary-first examples:

```json
{
  "scan_paths": ["."],
  "severity_threshold": "high",
  "summary_only": true
}
```

```json
{
  "scan_paths": ["."],
  "category": "dependency",
  "severity_threshold": "critical",
  "detail_level": "index",
  "page": 1,
  "page_size": 50
}
```

```json
{
  "scan_paths": ["."],
  "category": "secret",
  "severity_threshold": "high",
  "detail_level": "index",
  "page": 1,
  "page_size": 100
}
```

Scanner responses include `total_findings`, `returned_findings`, `page`, `page_size`, `has_more`, and optionally `bounded`. If `has_more` or `bounded` is true, request the next page or narrow filters such as `category`, `rule_ids`, `package_names`, `vulnerability_ids`, or `paths`.

## Security Label And Existing Tasks

Before scanner task creation, ensure the reusable security label exists:

```json
{
  "name": "security",
  "description": "Security findings and remediation work",
  "color": "#dc2626"
}
```

Then list open security tasks:

```json
{
  "label_id": "<security_label_id>",
  "open_only": true,
  "detail_level": "compact",
  "limit": 100
}
```

Use these tools:

- `ensure_task_label`
- `list_tasks`
- `add_task_comment`

Do not filter this existing-task lookup by the configured destination state. Duplicate detection must consider all non-completed, non-archived security tasks, including tasks already moved to backlog, in progress, review, or another open workflow state.

Parse Sentinel markers such as `<!-- sentinel:root_cause=... finding_ids=[...] -->` from compact task excerpts yourself. Do not expect structured marker fields from `list_tasks`.

If an open matching task exists, do not create a duplicate. Add a comment only when the current scan adds materially new evidence, such as:

- New CVE, GHSA, OSV, CWE, or scanner rule ID.
- New affected manifest, source file, path, sink, line, package, image, or resource.
- New fixed version, mitigation, severity, CVSS score, or advisory URL.
- Corroborating evidence from another scanner.

If the finding is unchanged from the open task and prior comments, do nothing.

Pass the configured severity threshold, `include_low_info`, and scan scope if available. If no scan scope is configured, scan the entire selected repository with `scan_paths: ["."]`.

Important path rule:

- Repository tools such as `list_directory`, `read_files`, and `repository_search` are scoped to the checked-out repository. Do not use them to inspect `/app/security-rules`, `/app/.cache`, or any other absolute runtime path.
- Treat `/app/security-rules/semgrep`, `/app/.cache`, and `/tmp/helpin-security-cache` as paths owned by the scanner tools only.
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

The worker image prewarms Trivy vulnerability databases under the read-only image cache and ships local Semgrep rules under `/app/security-rules/semgrep`. The scanner tools seed writable runtime caches under `/tmp/helpin-security-cache` so Kubernetes workers with a read-only root filesystem can still run scanners safely.

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

## Grouping And Dedupe Identity

Create at most one task per fix unit. Group findings when they share:

- Same vulnerable package, manifest, and remediation version.
- Same secret type and leak location pattern.
- Same scanner rule and remediation pattern.
- Same source/sink pair or data-flow issue.
- Same infrastructure control gap.

For dependency vulnerabilities, group multiple CVEs in one task only when one package/manifest update fixes them together. Do not group unrelated packages just because they were reported by the same scanner.

Build deterministic finding IDs before creating or commenting:

- Dependency: `sentinel:v1:dependency:<ecosystem>:<manifest>:<package>:<cve-or-advisory>`
- Secret: `sentinel:v1:secret:<scanner-rule>:<path>:<line-or-stable-fingerprint>`
- SAST: `sentinel:v1:sast:<scanner-rule>:<path>:<sink-or-line>`
- Misconfig: `sentinel:v1:misconfig:<scanner-rule>:<path>:<resource>`

Include a single-line hidden marker at the top of new task descriptions, capped to roughly 300 characters:

```markdown
<!-- sentinel:root_cause=<stable-root-cause-key> finding_ids=<json-array> -->
```

Include this hidden marker at the start of scan-update comments:

```markdown
<!-- sentinel:scan_update finding_ids=<json-array> -->
```

Do not create duplicate tasks for the same issue across Semgrep, Trivy, and Gitleaks. Mention corroborating scanners in the task description or scan-update comment.

## Task Creation

Use `create_task` once per applicable fix-unit group that does not already have a matching open security task, up to `max_tasks`.

Task name:
`Fix <severity> security issue: <short root cause>`

Task type: `chore`

Always pass `label_ids: ["<security_label_id>"]`.

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

Source refs should include scanner/advisory/release URLs where verified.

## Existing Task Comments

When an open matching `security` task exists and new evidence is present, use `add_task_comment` with the matched `task_id`.

Comment body must include:

- Scan timestamp or run context if available.
- Finding IDs from this scan.
- New evidence added since the task was created or last commented.
- Current severity, affected files/manifests/packages, and fixed versions.
- Recommended verification command.

Do not rewrite the existing task description during routine reruns.

## Final Response

Summarize:

- Scanners run and any scanner failures.
- Findings by severity before triage.
- Tasks created with task IDs/refs.
- Existing tasks commented with task IDs/refs.
- Findings suppressed as false positive/not actionable.
- Limitations and what should be checked manually.
