---
name: dependency_auditor
description: Audits direct package dependencies in repository manifests and creates verified update tasks.
metadata:
  title: Dependency Auditor
  supported_runtimes:
    - native_sdk
---

- Use `update_plan` first and keep it current as you inspect manifests, verify versions, gather context, and create tasks.
- Follow the configured ecosystems, include_indirect flag, task destination, and max_tasks value in the agent's own instructions. Treat those values as already resolved and authoritative.
- Do not plan or perform a step to discover configuration variables, workspace context, teams, stages, cadence, or repository selection. Use the configured values directly.
- Inspect the repository read-only. Do not edit dependency manifests, lockfiles, checksums, or source files.
- Use this plan shape: discover manifests -> parse direct dependencies -> verify latest versions/advisories -> classify every dependency -> prioritize outdated dependencies -> create tasks -> final summary.
- Compare against direct manifest declarations, not lockfile-resolved versions. Use lockfiles only as supporting context for currently resolved versions, features, and risk notes.
- Repositories may contain multiple selected ecosystems. Scan every configured ecosystem in the same run and keep classification counts per ecosystem in the final summary.
- Scan only configured ecosystems. For detailed parsing and verification rules, follow the bundled references:
  - Go: `ecosystems/go.md`
  - Rust: `ecosystems/rust.md`
  - Python: `ecosystems/python.md`
  - Node / JavaScript / TypeScript: `ecosystems/node.md`
  - Java / JVM: `ecosystems/java.md`
  - Cross-ecosystem verification: `verification.md`
- If bundled reference files are unavailable in the current runtime, follow the mandatory rules summarized in this file and do not improvise beyond verified registry/advisory data.
- Prefer pinned verification surfaces: Go proxy or `go list -m -u -json`, crates.io API, PyPI JSON API, npm registry metadata, Maven Central search/metadata, and OSV.dev for advisories. Use official release/changelog sources only as fallback or additional context.
- Never guess versions, dates, release notes, or security findings. If a dependency cannot be verified, mark it unresolvable in the final summary and do not create a task for it.
- Classify every discovered direct dependency as `up_to_date`, `outdated`, or `unresolvable`.
- For outdated dependencies, gather 3 to 5 material changes between the current and latest verified versions. Prefer official release notes, changelogs, registry pages, repository releases, and advisory databases.
- Query OSV.dev for security advisories when package identity and version are known. Flag CVE, GHSA, RustSec, Go vulndb, PyPA, OSV, security advisory, and vulnerability evidence.
- Flag migration risk such as breaking changes, deprecations, removals, changed API behavior, Go toolchain requirements, Rust MSRV changes, or Python version requirement changes.
- Deduplicate within the run. Create at most one task per dependency identity: Go module path, Rust crate plus source identity, normalized Python package plus source/index identity, npm package plus registry identity, or Maven groupId/artifactId plus repository identity.
- Consolidate all affected manifest files, declaration sections, extras, markers, features, replacement directives, and source overrides into the single task description.
- Create tasks only for outdated dependencies with verified latest versions, up to the configured max_tasks limit. If more outdated dependencies exist than max_tasks, create the highest-priority tasks first and report the rest in the final summary.
- Use `create_task` once per created task. Pass the configured destination team directly as `team_id`; include the configured destination state directly as `state_id` only when it is present.
- Use task type `chore`. Set priority `high` for security fixes, `medium` for major bumps, runtime-version increases, or confirmed breaking changes, and `low` for patch and minor bumps.
- Include acceptance criteria and labels in the task description because `create_task` may not support separate fields for them. Labels should include `dependency-update`, the ecosystem, `security` when applicable, and `breaking-change` when applicable.
- Each task description must include verification source URLs and enough context for a human to review and apply the update safely.
- After task creation, respond with a concise summary: ecosystems scanned, manifests scanned, direct dependencies examined, tasks created, up-to-date count, unresolvable count, created task IDs/refs, and important limitations.
