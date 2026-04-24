# Rust Dependency Audits

## Parse
- Discover every `Cargo.toml`. If a workspace is present, parse the root workspace and all member manifests.
- Direct dependencies include `[dependencies]`, `[dev-dependencies]`, `[build-dependencies]`, `[workspace.dependencies]`, and target-specific dependency sections.
- Resolve `workspace = true` entries from `[workspace.dependencies]`; the member declaration is direct, but version/source details live in the workspace root.
- Record renamed dependencies via `package = "..."`; dedupe by package name plus source identity, not only the local alias.
- Record features, `default-features`, optional status, target conditions, dependency section, registry, git, branch, tag, rev, and path source.
- Use `Cargo.lock` only as supporting context. Do not create tasks for lockfile-only transitive crates.

## Verify Versions
- Preferred structure command when available: `cargo metadata --format-version 1 --no-deps`.
- Preferred registry source: crates.io API `https://crates.io/api/v1/crates/{crate}` for crates.io dependencies.
- Use the latest stable crate version unless the current declaration is prerelease or explicitly allows prereleases.
- Do not propose updates for path dependencies.
- For git dependencies, propose only when pinned to a tag and a newer verified tag exists for the same repository/source; otherwise mark unresolvable.
- For custom registries, verify only if the registry URL/source is discoverable and accessible; otherwise mark unresolvable.

## Classify And Describe
- Rust semver treats `0.x.y` minor changes as potentially breaking. Classify `0.x -> 0.y` where `x != y` as `major` risk unless official notes prove compatibility.
- Flag MSRV increases, feature behavior changes, removed defaults, renamed crates, and source changes.
- Include affected manifests, dependency sections, local alias, actual package name, source identity, features, optional/default-feature settings, target conditions, current declaration, latest verified version, and advisory/changelog URLs in each task.
