# Follow-up cleanup audit (2026-09-18)

Base: `d4edcc694` on `origin/develop`. Changes are isolated on
`cleanup/followup-dead-code`.

Removed after reference searches across application workspaces:

- Unrendered `WorkflowManager` and its barrel exports.
- Six unused checklist/view mutation hooks; live query hooks and service methods remain.
- Unimported `dataStrategy.ts` and `formatters.ts` modules.
- Legacy coverage finding upsert/draft generation methods, their exclusive helpers
  and DTOs, and the uncalled snapshot refresh implementation.
- Two test-only wrappers. Rewrite tests now call the handler's live
  `RewriteDraftForSurface` entrypoint with the same surface and billing key;
  child-result tests directly assert live evidence extraction.

The active V2 coverage pipeline, clustering/assignment tests, embedding provider,
repositories, routes and historical migrations remain. Four tests specific to
removed legacy coverage APIs were removed; active materializer and analyzer tests
continue to exercise production paths. Expiry fixtures now store UTC timestamps,
matching SQLite comparisons, so Berlin timezone runs test the intended expiry.
Frontend absolute-date and roadmap tests now construct local dates to match
their local-time display/calendar contracts.

## Remaining investigation queue

The initial baseline contains 1,076 Community and 1,142 Enterprise TypeScript
findings, and 153 Community and 150 Enterprise Go function findings. Counts
include unused types/exports and dependency diagnostics, not just removable files.
They overlap across editions. Do not add them together as a count of dead code.
Raw reports can be reproduced using the README command.

- Dependencies: both editions report candidates such as `@hookform/resolvers`,
  `@fontsource-variable/geist` and `@types/dompurify` in the frontend. Before
  removal, check build tooling, CSS imports and published package contracts.
  `dayjs` is used by Enterprise and stays. `react-hook-form` still has an import
  in the UI form module; its reachability needs separate review.
- Dependency ownership: `UnicodeSpinner` imports `unicode-animations`, which is
  declared at the root. Tests also report undeclared `jsdom` imports. These are
  packaging concerns rather than verified dead dependencies.
- Assets: no literal references were found for `frontend/public/vite.svg` or
  `frontend/src/assets/react.svg`. The public path may have external consumers;
  this pass does not infer asset usage from source references alone.
- Configuration: all five frontend example keys have source references. Of 182
  server example keys, eleven lacked exact literal references in the first scan.
  Four `JEV_*_MODE` names are constructed in `config/jev_product.go` and are live.
  The remaining candidates are `CLICKHOUSE_KAFKA_GROUP`,
  `CLICKHOUSE_MIGRATION_REQUIRE_KAFKA_CONFIG`, `CODEX_OPENAI_AUTH_MODE`,
  `CODEX_ENABLE_CHATGPT_OAUTH`, `CODEX_CHATGPT_ACCESS_TOKEN`,
  `CODEX_CHATGPT_ACCOUNT_ID`, and `AGENT_NATIVE_SELECTIVE_PLANNER_ENABLED`.
  Verify external runtime/deployment ownership before removing these examples.
- API routes: router registrations are production roots for backend analysis.
  A missing frontend caller does not establish that an API is unused by SDKs,
  integrations or external clients. No route retirement is justified by this
  source-only audit; that needs usage/deprecation evidence.

The baseline is a starting point for incremental review. It is not proof that
all remaining findings are safe to delete, and it does not audit Rust or runtime
traffic. The PR check prevents new findings without requiring this entire queue
to be resolved in one change.

## Validation

- Frontend: Community and Enterprise production builds; 468 test files passed,
  2,817 tests passed and 3 skipped in Europe/Berlin.
- Frontend date regressions: both UTC and Europe/Berlin passed.
- Backend: `go build ./...`, `go build -tags ee ./...`, and `go vet ./...` passed.
- Affected CLI, preview, rewrite, child-result and coverage service tests passed
  in both editions; race runs passed in UTC (Community) and Berlin (Enterprise).
- Unused-code check passed for both editions. A temporary unused-module canary
  was detected end to end and removed. Five checker tests and ten CI policy
  tests passed; actionlint and frozen-lockfile validation passed.

Local validation used Go 1.26.4, Node 22.22.1 and existing workspace dependencies.
The workflow installs from the frozen lockfile using repository version files;
its hosted run remains to be observed after pushing the branch.
