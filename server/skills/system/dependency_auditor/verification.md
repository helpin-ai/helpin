# Dependency Verification

## Version Sources
- Verify latest versions from the primary ecosystem source first: Go proxy or `go list`, crates.io API, PyPI JSON API, npm registry metadata, and Maven Central search/metadata.
- Use official repositories, releases, and changelogs only to add context or when a registry cannot provide changelog detail.
- Cache package/version lookups within a run. Do not fetch the same package metadata repeatedly for duplicate declarations.
- Prefer batch-friendly commands when available and safe: `go list -m -u -json all`, `cargo metadata --no-deps`, `npm outdated --json`, `pnpm outdated --format json`, `yarn npm info`, Maven/Gradle dependency report tasks, and manifest-level parsing before per-package HTTP calls.

## Advisory Sources
- Use OSV.dev as the first unified advisory surface.
- Query OSV when package ecosystem, package name, and current version are known.
- Map ecosystems as:
  - Go: `Go`
  - Rust crates.io: `crates.io`
  - Python PyPI: `PyPI`
  - Node npm: `npm`
  - Java Maven Central: `Maven`
- Treat OSV aliases and related IDs as evidence for labels and priority: CVE, GHSA, RustSec, Go vulndb, PyPA, and OSV IDs.
- If OSV is unavailable, use official advisory sources only when directly cited by registry or release notes. Do not infer security impact from vague wording.

## Failure And Rate-Limit Policy
- On network failures, rate limits, ambiguous package identity, private source, or incompatible metadata, mark the dependency unresolvable with the explicit reason.
- Do not retry indefinitely. One retry is acceptable for transient HTTP failures; after that, report the limitation.
- If the repository has more outdated dependencies than `max_tasks`, prioritize security fixes first, then major/runtime-risk updates, then patch/minor updates.
- Every created task must cite the latest-version source URL and any changelog, release note, repository, or advisory URLs used.
