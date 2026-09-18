# Unused-code checks

Run from the repository root after `pnpm install --frozen-lockfile`:

```sh
go install golang.org/x/tools/cmd/deadcode@v0.48.0
python3 scripts/unused-code/check.py --report-dir /tmp/helpin-unused-reports
```

Ensure `$(go env GOPATH)/bin` is on `PATH`. `KNIP` and `DEADCODE` may specify
alternate executable paths. Knip is pinned in the root package and lockfile.
The `Unused code` PR workflow runs the same check for Community and Enterprise.

The check rejects new findings relative to `baseline.json`, using file and symbol
names rather than line numbers or counts. Existing findings are an investigation
queue, not an approved deletion list. To intentionally refresh the baseline, run
with `--update-baseline` and review the diff. Analyzer errors cannot update it.

Knip follows frontend, desktop, mobile, admin, help-center, website and package
entrypoints. Edition aliases select the corresponding implementation. Published
package entrypoint exports remain public API even without in-repository callers.
SDK browser loader and Playwright test entrypoints are explicit. The SDK's
Playwright plugin is disabled to avoid executing its configs during analysis.

Go analysis starts at every executable under `server/cmd` in each edition,
without test roots, so test-only helpers remain visible. Generated and marker
methods are excluded. Reflection, platform-specific builds, external consumers,
and dynamically constructed paths still require manual review. Rust and static
assets are outside these analyzers' coverage. Do not delete API routes, historical
migrations, configuration keys or public assets solely because a local reference
search is empty.

Analyzer references: [Knip configuration](https://knip.dev/reference/configuration)
and [Go deadcode](https://pkg.go.dev/golang.org/x/tools/cmd/deadcode).
