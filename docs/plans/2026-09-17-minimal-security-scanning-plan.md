# Minimal security scanning for Helpin Community

Date: 2026-09-17
Status: Planned; no CI or repository settings changed by this document.

## Outcome and decisions

Make security checks and remediation visible to open-source users while keeping additional PR wait time small. Extend the existing release security checks rather than introducing another image build pipeline.

- Dependency review is the only new required PR check initially.
- CodeQL runs independently on relevant PRs, without becoming a required merge check during the measurement period.
- Trivy stays in release validation; add nightly scans of the exact published images. Do not add image builds or full image scans to ordinary PRs.
- Keep the existing CycloneDX release evidence, private reporting policy, and release promotion process.
- Publish a short explanation of coverage, thresholds, and exceptions. A custom dashboard and additional report formats are deferred.

This plan complements [the Community release plan](2026-09-15-community-release-plan.md). It does not replace its application security and release acceptance requirements.

## What already exists

Observed in the local checkout on 2026-09-17; these are source configuration findings, not proof of successful production runs or enabled GitHub settings.

| Existing implementation | Consequence for this plan |
| --- | --- |
| `SECURITY.md` defines private reporting and support for the latest published 0.1 patch. | Verify the reporting flow and assign an owner; do not create a second policy. |
| `.github/dependabot.yml` updates GitHub Actions weekly. | Preserve it; separately verify dependency alerts and security updates for application ecosystems. |
| `.github/workflows/ci.yml` selects checks by changed paths and aggregates them into `CI required`. Internal PR jobs commonly use `arc-runners-helpin-ai`; fork jobs use hosted runners. | Avoid sharing scarce ARC capacity with new scans. Keep advisory CodeQL out of the required aggregate. |
| `community-release.yml` calls `community-test.yml` with `export-images: true` for amd64 and arm64. | Release scans are already wired before image publication. |
| `community/check-images.sh` scans Compose images, generates vulnerability-inclusive CycloneDX files, checks fixable High/Critical findings, and performs a secret scan without uploading raw secret matches. | Reuse the scanner and retain its separation of publishable evidence from sensitive findings. |
| `community/verify-image-findings.py` accepts only matching, unexpired exceptions for digest-pinned upstream images. | Preserve narrowly scoped exceptions and make their rationale visible. |
| `community/package-release.py` copies CycloneDX files into the bundle's `release-evidence/`. | SBOM/report evidence is already part of the release design; verify it survives packaging and promotion. |
| `community-bundle.yml` runs scheduled acceptance, but does not request image export. Trivy steps require `export-images`. | Scheduled acceptance currently does not provide nightly vulnerability rescanning. Add a scan-only workflow rather than enabling export/build work just for security. |

No CodeQL or dependency-review workflow was found in `.github/workflows/`. Live branch protection, scanner settings, runner capacity, and actual job durations still need inspection during implementation.

## 1. Establish ownership and baseline

- [ ] Assign one security triage owner and a backup. Record them in maintainer documentation.
- [ ] Verify GitHub private vulnerability reporting, dependency graph, Dependabot alerts/security updates, secret scanning, and push protection where supported. Check feature availability while the repository is private and again at public launch.
- [ ] Confirm application dependency coverage: `server/go.mod` and `go.sum`, root `pnpm-lock.yaml` and workspace manifests, and separately maintained Cargo/Python/Node manifests. Record unsupported or incomplete coverage instead of claiming all dependencies are checked.
- [ ] Capture 10–20 representative existing CI runs: queue time, required-check completion time, and runner usage. Separate docs-only, application, dependency, and Community packaging changes.
- [ ] Run initial scans and triage the baseline before enabling new merge requirements.

## 2. Add lightweight PR dependency review

Proposed file: `.github/workflows/dependency-review.yml`.

- [ ] Run on every PR targeting `main` or `develop`, independently of build/test jobs, on a GitHub-hosted runner.
- [ ] Use GitHub's dependency review action with a High severity threshold, so newly introduced High/Critical dependency vulnerabilities fail the check.
- [ ] Avoid installing the entire application or rebuilding containers for this check. Validate that GitHub's dependency graph actually sees the supported lockfiles; if dependency submission is needed, measure its cost explicitly.
- [ ] Keep a stable check name and configure it as a separate required check after validation. Avoid workflow-level path filters that could leave required checks pending on docs-only PRs.
- [ ] Support fork PRs with minimum read permissions. Do not use a privileged workflow to execute fork code.
- [ ] Cancel superseded runs for the same PR. Pin the action to a reviewed full commit SHA, consistent with existing workflows.
- [ ] Treat API failures or incomplete required dependency data as check failures requiring investigation, not a clean security result.

Do not block unrelated PRs on the entire pre-existing vulnerability backlog. Existing dependency findings remain tracked through alerts, scheduled scans, and release review.

## 3. Trial CodeQL without extending the merge gate

Proposed file: `.github/workflows/codeql.yml` using advanced setup for explicit triggers and language selection.

- [ ] Start with Go and JavaScript/TypeScript, using the standard query suite and separate language jobs. Record components outside this coverage, including Rust; CodeQL is not a claim of complete application coverage.
- [ ] Analyze relevant PRs, pushes to protected development/release branches, and weekly on the default branch. Allow manual runs. Include build/configuration changes in selection and retain a scheduled full scan as a backstop.
- [ ] Use hosted runners, with no production secrets. Upload results with only the permissions supported for each event, including fork PRs.
- [ ] Keep the workflow outside `CI required` and any ruleset requiring CodeQL while measuring. Leave execution failures visible rather than turning them into success with blanket error suppression.
- [ ] Avoid duplicate default-setup and advanced-setup scans. Do not add another full product build unless extraction requires it.
- [ ] After at least 20 representative PR runs, review runtime, queue contention, findings, and false positives. Make serious new findings blocking only after triage and acceptable latency are demonstrated.

During the trial, a named maintainer must review serious findings before the affected code is released. Advisory status means it does not mechanically block merging; it does not mean findings are ignored.

## 4. Extend existing image scanning to published releases

Proposed file: `.github/workflows/security-nightly.yml`; refactor `community/check-images.sh` only as needed to accept an explicit image inventory and architecture.

- [ ] Keep current release scanning of both supported architectures and exact tested images. Verify scanner failures, database download failures, and missing reports fail release validation.
- [ ] Nightly, resolve the newest supported published Community release, including the documented beta/pre-release channel. Do not rely blindly on GitHub's latest-release endpoint, which can omit pre-releases.
- [ ] Read image references from its digest-pinned bundle. Scan each distinct application, Runtime, and bundled infrastructure image for each supported architecture without rebuilding or starting the application.
- [ ] Use the existing checksum-verified scanner installation, updating its version through a reviewed change when necessary. Reuse downloads/cache where safe while allowing the vulnerability database to refresh.
- [ ] Keep the current release gate explicit: fixable High/Critical findings block unless covered by an approved, exact, unexpired exception. Unfixed High/Critical findings must remain visible for manual assessment; a missing fix is not evidence of safety.
- [ ] Nightly findings alert the assigned maintainer through a verified notification route and do not block unrelated PRs. Verify delivery; a red workflow that nobody follows is insufficient.
- [ ] Record scanner version, database freshness, timestamp, architecture, image digest, severity counts, and exception references. State when a scan failed or was skipped instead of presenting it as passing.
- [ ] Deduplicate findings across scans. Give each accepted exception an owner, rationale, affected digest/package/advisory, and expiry. Escalate serious exploitable findings even if no upstream fix exists.
- [ ] For a candidate held before promotion, rescan its exact digests when the evidence is older than 24 hours; keep promotion bound to the same tested artifacts.

Before the first release exists, the nightly workflow should state that there is no published target. Do not display that state as proof of a clean release scan.

## 5. Expose useful security evidence

Proposed documentation: `docs/community/security.md`, linked from the README and release notes.

- [ ] Describe the checks, schedule, scope, release threshold, and private reporting route.
- [ ] Add a clearly labelled nightly release-scan status link/badge after the workflow is active and has a real target.
- [ ] Explain where the existing `release-evidence/*.cdx.json` files are located. These currently include vulnerability data, not just dependency inventory.
- [ ] Verify every required image/architecture has evidence before packaging; do not silently accept an empty or partial glob of report files.
- [ ] Link the exception policy and explain the distinction between scan completion, policy pass, and zero findings.
- [ ] Publish advisories and affected/fixed versions for confirmed vulnerabilities in released Helpin code, with coordinated disclosure.
- [ ] Continue keeping secret matches and undisclosed application vulnerability details out of public logs and artifacts. GitHub code-scanning alerts are maintainer-facing; uploading SARIF alone is not a public report.

Retain the existing release evidence. Defer a custom dashboard, additional downloadable formats, and a broad new scanner suite until there is a demonstrated need.

## PR latency budget and rollout decision

Target: dependency review adds no more than one minute to p95 merge-ready time. This is an acceptance target, not a runtime guarantee.

For jobs starting concurrently with available runners:

```text
additional wait = max(0, security check completion time - existing required CI completion time)
```

Also measure queue contention and total runner minutes: non-required jobs can still slow required jobs if they compete for capacity. Use hosted runners for the initial trial and verify the account's concurrency limits.

- Dependency review: provisionally budget 30 seconds–2 minutes, then replace this estimate with observed values.
- CodeQL: measure separately by language; do not assume the earlier generic estimate applies to this repository.
- Trivy: zero new ordinary PR jobs; its time belongs to release validation and nightly monitoring.
- Record the one-time CI cost of modifying workflows: `scripts/ci/checks.py` currently selects all check groups for workflow changes. This is distinct from recurring security-check overhead.

If dependency review misses the target, inspect queueing and dependency graph waits before changing coverage. If CodeQL is too expensive, keep it advisory or move its PR execution to an explicitly reviewed narrower policy while retaining protected-branch and scheduled coverage. Document that reducing PR scanning shifts detection until after merge.

## Verification and completion

- [ ] Clean/docs-only PR: dependency review completes and required checks do not remain pending.
- [ ] Controlled dependency fixture: a newly introduced known High/Critical finding blocks; an existing unrelated finding does not newly block the PR. Validate graph ingestion without executing vulnerable code.
- [ ] Fork PR: checks run with the intended restricted permissions and no privileged credentials.
- [ ] CodeQL: findings upload correctly, serious findings reach the owner, and the initial workflow is not a merge requirement.
- [ ] Image policy: covered/uncovered findings, expired exceptions, unfixed findings, scanner errors, and missing image evidence behave as documented.
- [ ] Nightly: uses the published release digests, covers both architectures, and a simulated failure reaches the owner.
- [ ] Release bundle: required CycloneDX evidence and metadata survive packaging and promotion; secret reports do not.
- [ ] Run relevant existing CI policy and Community image/packaging tests plus workflow linting when implementation lands. Add focused tests only for changed gating, inventory, and evidence behavior.
- [ ] Publish measured latency and runner-minute results and the decision on requiring CodeQL.

The initial rollout is complete when dependency review is enforced, CodeQL is running under a measured advisory trial, release/nightly image checks have an accountable owner, and users can find the policy and release evidence.

## References

- [GitHub dependency review](https://docs.github.com/en/code-security/concepts/supply-chain-security/dependency-review)
- [Code scanning workflow configuration](https://docs.github.com/en/code-security/reference/code-scanning/workflow-configuration-options)
- [GitHub Actions concurrency](https://docs.github.com/en/actions/concepts/workflows-and-actions/concurrency)
- [CodeQL performance troubleshooting](https://docs.github.com/en/code-security/reference/code-scanning/troubleshoot-analysis-errors/analysis-takes-too-long)
- [Chatwoot security practices](https://www.chatwoot.com/security)
- [Plane nightly Trivy remediation example](https://github.com/makeplane/plane/pull/9806)

The peer examples support visible practices, advisories, and remediation. They do not establish that every open-source project must publish full scanner reports.
