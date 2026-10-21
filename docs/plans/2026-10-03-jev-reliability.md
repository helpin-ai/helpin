# Jev reliability improvements

This implementation plan records the approved, bounded reliability improvements
for contributors working on the existing Jev integrations. All changes are shared
Community behavior. No deployment or live customer/provider evaluation is included.

**Goal:** Reuse valid support decisions, recover from transient failures, expose
accurate decision metrics, and remove retired translation dependencies.

**Architecture:** Retain the support audit table, workspace admission lock, shared
daily cap, existing decision prompts, thresholds, ownership checks and action
guards. Cache keys include the relevant rollout policy. Failed/pending attempts
have a one-minute cooldown; stale completions cannot settle abandoned attempts.

**Stack:** Go, GORM/PostgreSQL, SQLite test fixtures, Prometheus.

## Implementation

- [x] Add service regressions for cached results, cooldown/retry, shadow promotion,
  threshold changes, invalid cached/provider output and unchanged usage/caps in
  `server/internal/service/support_jev_test.go`; observe the failures.
- [x] Extend `server/internal/repository/support_jev.go` admission and settlement;
  update `server/internal/service/support_jev.go` to validate/reuse results and
  preserve metering. Cover stale settlement and PostgreSQL concurrent admission.
- [x] Add metrics regressions and correct support operation labels; instrument
  the shared product decision service and PM triage using bounded labels.
- [x] Remove the unused translation Jev dependency and retired policies from
  configuration, wiring and fixtures while retaining response/schema compatibility.
  Correct `server/.env.example` and the existing Jev operator guides.
- [x] Review tagging dispatch for a safe, bounded change. Defer decoupling if it
  needs durable orchestration beyond this reliability patch.
- [x] Run targeted Community and EE regressions, race checks where warranted,
  PostgreSQL admission checks, API/worker builds, documentation checks and diff review.

## Verification commands

From `server/`, run targeted tests with `TZ=UTC go test ./internal/decision
./internal/config ./internal/service ./internal/repository ./internal/observability
-run 'Test(Jev|SupportJev|PMTriage|Translation|SupportTranslation)' -count=1` and the
equivalent `-tags ee` checks. Run affected concurrency tests with `-race` and the
repository's disposable PostgreSQL integration fixture. Build `./cmd/api` and
`./cmd/temporal-worker` in both editions. From the root, use the documentation
naming/link checks and `git diff --check`.

No changes to classifier meanings, candidate retrieval, automatic approval policy,
or rollout defaults are planned. Those require separate product evaluation.

## Initial reliability results

Support decisions now reuse validated successful results without new provider
calls, usage records or admissions. Failed attempts can retry after a minute
within the existing daily cap. Workspace and event locks coordinate admission
with settlement; abandoned and completed attempts cannot accept late writes.
Mode/threshold changes invalidate the cache. Existing domain action guards remain.

Metrics distinguish routing, tagging, handoff, follow-up, PM and the five product
features. The Temporal worker can expose the same registry through its optional
`METRICS_ADDR`. Translation no longer carries a Jev dependency; retired policies
are ignored. Response and database fields remain compatible.

Tagging scheduling was deferred in the initial patch. `runWidgetPostMessageAutomation` runs triage
before starting the reply flow, and triage tags before routing. Separating these
steps needs explicit durable dispatch and ordering decisions; another detached
goroutine would not provide those guarantees.

Targeted Community service regressions passed with the race detector; equivalent
EE regressions passed. Repository unit/race checks and disposable PostgreSQL 16
admission checks passed, including simultaneous duplicates, shared limits,
abandoned attempts and existing product decision concurrency tests. Local
Redis/PostgreSQL socket tests required sandbox escalation. Documentation naming,
maintained/edited-page links, all 11 documentation-script tests and diff whitespace
checks passed. The final PM metrics regressions passed in both editions, including
the race detector in Community. Full backend builds and `go vet ./...` passed in
both editions; API/worker builds and service vet checks were repeated successfully
after the last PM metrics change. The disposable database container was removed.

No live Jev calls, customer-data evaluations, schema changes, rollout changes or
deployments were performed. Classifier quality still requires representative
human-reviewed evaluation.

## Follow-up: durable background tagging

The user approved moving tagging off the reply path. This remains shared
Community behavior in the existing worktree. Reuse the live-translation queue
pattern: an insert trigger records identifier-only work alongside each new public
customer reply. A cancellable API worker polls independently of triage and reply
startup. This avoids new runtime infrastructure and also covers database-backed
ingestion paths. The existing shared scheduled-event dispatcher was considered,
but its minute cadence and Temporal dependency are unnecessary for this task.

Jobs have a unique message identity, fenced leases, five bounded attempts and
backoff at least as long as Jev's cache cooldown. The worker checks current
configuration, entitlements, source privacy/deletion and newer customer messages.
It revalidates evidence and selected tag names before atomically applying tags,
audit notes and completion. Manual changes use the same conversation lock; no
provider request runs while that lock is held. Existing websocket updates remain.
No historical backfill or classifier/policy changes are included.

- [x] Add a routing regression proving that a slow/unavailable tag classifier is
  not called by `EvaluateAndRoute`; observe failure before removing that call.
- [x] Add `model/support_tag_job.go`, `repository/support_tag_job.go` and migration
  `202610030001_support_tag_jobs.sql`. Verify transactional enqueue, eligibility,
  duplicate claims, retries, lease expiry and privacy against PostgreSQL.
- [x] Separate tag selection from application in `service/support_jev.go` and
  move tag application/audit under conversation locks in `service/support_tag.go`.
  Reuse existing prompts, thresholds, cache, metering and human-removal rules.
- [x] Add `service/support_tag_jobs.go` with independent bounded processing and
  explicit shutdown; wire into `cmd/api/main.go`. Cover disabled/shadow/denied
  modes, retries, stale messages, renamed/deleted tags, manual removals, lost
  leases, exactly-once audit and rollback.
- [x] Update the existing Jev operator guide, verify Community and EE behavior,
  race/concurrency cases, real migration replay, builds, vet, and docs checks.

Validation: targeted Go tests match `Test(SupportTag|SupportJev|JevTag|JevRouting|
SupportInboxTriage|Translation|SupportTranslation)` in both editions, with race
checks for the changed services and repository. PostgreSQL tests use only a
disposable local database. Existing support message and tag regression suites
cover affected persistence. Full backend build/vet and documentation naming/link
checks finish the change.

### Follow-up results

Tagging now runs through the durable queue independently of routing and reply
startup. Jobs are enqueued transactionally for new eligible customer messages.
Retries preserve Jev caching and the shared daily cap. Lease fencing, evidence
rechecks and conversation locks protect tag writes; tag changes, audit notes and
job completion commit together. Privacy erasure removes queued work.

The routing regression failed before the synchronous call was removed and passed
afterward. Targeted Community tests passed with the race detector; equivalent EE
tests passed. PostgreSQL integration tests passed for transactional enqueue,
concurrent claims, lease fencing, rollback, bounded retries and anonymization.
The complete migration ledger replayed twice successfully in the tagging fixture,
including live-translation enqueue and existing contact privacy regressions.
Full backend builds and vet passed in both editions. Documentation naming, links,
all 11 documentation-script tests and diff whitespace checks passed.

The broader fresh-schema parity check failed because the pre-existing
`dock_chats.coverage_gap_id` model index is absent from the migration ledger.
The identical failure was reproduced in a fresh database with the tagging
migration excluded through a temporary Go overlay. No unrelated schema fix was
included.

Apply `202610030001_support_tag_jobs.sql` through the normal migration runner
before deploying the new API; AutoMigrate alone does not install the queue or its
triggers. No live Jev calls, customer-data evaluations or deployments were made.
