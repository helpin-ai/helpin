# Go Dependency Audits

## Parse
- Discover every `go.mod`. If `go.work` exists, use it to identify workspace modules, but still parse each module's own `go.mod`.
- Direct dependencies are `require` entries not marked `// indirect`, unless `include_indirect` is true.
- Record `go` and `toolchain` directives separately. They are runtime/toolchain context, not normal module dependencies.
- Record `replace`, `exclude`, and any local module relationships. If a dependency is replaced to a local path or unverifiable fork, mark it unresolvable unless the replacement target has a verified comparable version.
- If `vendor/modules.txt` exists, use it only as supporting context for the vendored resolved version.

## Verify Versions
- Preferred CLI when available: `go list -m -u -json <module>` for each direct dependency, or `go list -m -u -json all` when it can batch safely without excessive output.
- Preferred registry fallback: `https://proxy.golang.org/<module>/@latest`, plus `/@v/list` and `/@v/<version>.info` when latest metadata or version history is needed.
- Treat `retract` directives as exclusion evidence. Do not propose a retracted version.
- Treat pseudo-versions as comparable only when the target latest version is a verified tagged release or a newer verified pseudo-version.
- Treat `+incompatible` versions as higher risk; call out that the module has no matching semantic import version suffix.
- Respect semantic import versioning. Do not propose a major upgrade requiring a new `/v2+` module path unless the current module path already uses that major path or official migration notes explicitly document the path change.

## Classify And Describe
- Bump type is based on the verified module version after stripping build metadata. Major path changes are `major`.
- Include affected `go.mod` paths, current declaration, latest verified version, `go` directive, `toolchain` directive, replacement details, and advisory/changelog URLs in each task.
- If version lookup works but changelog lookup does not, create the task only if the latest version is verified; state that release notes were unavailable.
