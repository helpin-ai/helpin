# Test Suite, CI, and Email E2E Plan

Status date: 2026-04-30

## Decisions Made (resolved from earlier Open Questions)

These were open when the plan was first drafted; they're now committed
defaults so phases can be executed without further blocking discussion.

- **Backend test DB**: default to Postgres via GitHub Actions `services` so CI
  matches production. SQLite is allowed only for tests that explicitly need
  cross-DB coverage. See `server/internal/service/testdb_test.go` for the
  established harness pattern; new tests follow it.
- **MailSlurp scope**: real email goes through Postmark (staging server). No
  local SMTP/IMAP path until/unless we adopt a different provider. Keep the
  surface narrow.
- **Initial required branch-protection checks on `develop`**: `Server CI`,
  `Frontend CI`, `Help Center CI`, `Migration Validate`. Browser smoke and
  email-e2e land later, after stabilization.
- **PR CI runtime budget**: ≤ 10 min wall-clock with parallel jobs. Per-job
  ceilings: server ≤ 6 min, frontend ≤ 4 min, packages ≤ 3 min, help-center
  ≤ 3 min. If any job sustains over budget for a week, split or move to nightly.
- **Real email cadence**: nightly + before every staging deploy + manual
  workflow_dispatch. Not a release gate for prod (too narrow a signal vs cost).

## Goal

Turn the current collection of local tests into a dependable CI contract:

- every PR gets fast deterministic regression coverage
- builds and migrations stay gated
- support inbox and email behavior get layered coverage from unit tests through real-provider checks
- external email checks use MailSlurp only where real mailbox behavior matters

The immediate gap is not that tests do not exist. The gap is that most tests are not enforced by CI.

## Current State

### CI

Current workflow: `.github/workflows/ci.yml`

Status:

- [ ] PR-triggered CI is enabled
- [x] manual `workflow_dispatch` CI exists
- [x] server `go vet ./...` runs manually
- [x] server `go build ./cmd/api` and `go build ./cmd/temporal-worker` run manually
- [x] frontend `pnpm build` runs manually
- [x] help-center build runs manually
- [ ] `go test ./...` runs in CI
- [ ] migration validation runs in CI
- [ ] frontend Vitest runs in CI
- [ ] package Vitest runs in CI
- [ ] Playwright suites run in CI
- [ ] real email provider tests run in CI

### Existing Test Inventory

Backend:

- 274 Go test files exist.
- Heaviest coverage areas: `internal/service`, `internal/repository`, `internal/worker`, `internal/handler`, `internal/temporalapp`, `internal/websocket`.
- Email/support-related coverage already includes Postmark webhook processing, email fallback queueing/sending, inbound email threading, bounces, complaints, unread/read behavior, and support inbox hydration.

Frontend/packages/help-center:

- About 176 local test/spec files outside `node_modules`.
- Frontend has broad Vitest coverage across PM, docs, support, stores, hooks, and lib helpers.
- Package tests exist for `widget-core`, `sdk-js`, `react`, `nextjs`, and `help-center`.
- Playwright exists for support inbox harness, SDK widget behavior, and help-center search.

### Known Problems

- CI is manual-only; PR triggers are commented out.
- CI builds but does not test.
- There is no single root `test` script that runs workspace tests consistently.
- Frontend has Vitest dependency and tests, but no `frontend/package.json` `test` script.
- Browser tests are wired but intentionally not gated yet.
- There is no formal split between PR, nightly, and release-gate suites.
- Real email behavior is mostly mocked or provider-webhook simulated; no live mailbox assertion exists yet.

## Test Strategy

Use layered tests. Do not put every test in every PR.

### Layer 1: PR Fast Suite

Purpose: block obvious regressions quickly.

Target runtime: under 10 minutes.

Runs on every pull request to `develop` and `main`.

Required checks:

- server vet
- server build
- server tests
- migration validation
- frontend build
- frontend Vitest
- core package Vitest
- help-center Vitest

Not included:

- real Postmark sends
- MailSlurp
- full browser e2e
- slow race suite
- external provider flows

### Layer 2: PR Browser Smoke

Purpose: catch support/widget browser regressions with deterministic mocked harnesses.

Target runtime: under 10 minutes, parallelizable.

Runs after Layer 1 is stable. Can start as non-required, then become required.

Required checks:

- frontend support Playwright mocked harness
- SDK widget mocked Playwright suite

### Layer 3: Nightly Integration

Purpose: catch cross-service failures that are too slow or flaky for every PR.

Runs nightly and manually.

Required checks:

- `go test -race` on stable backend packages or full backend if runtime is acceptable
- backend tests with real Postgres and Redis services
- support inbox API integration smoke
- Temporal workflow smoke for these specific workflows:
  - `EmailSyncWorkflow` (Gmail backfill + incremental sync)
  - `SignalDetectionWorkflow` (CRM signal extraction)
  - `DealManagementCronWorkflow` (hourly deal progression cron)
  - command-run plan workflows used by the Ask Agents dock
- Playwright support app flow against local API stack
- MailSlurp email delivery and inbound reply tests

### Layer 4: Release Gate

Purpose: validate deployability before staging or production promotion.

Runs on staging deployment and before production promotion.

Required checks:

- migration status/validate against target environment
- app health checks
- support inbox smoke
- widget smoke
- Postmark webhook reachability check
- MailSlurp real email smoke

## CI Implementation Plan

### Phase 0: Stabilize the Existing Manual Workflow

Status: in progress

- [x] Inventory current CI behavior.
- [x] Inventory backend/frontend test count.
- [ ] Add clear comments in `ci.yml` explaining PR/nightly/release test boundaries.
- [ ] Add `go test ./...` to the manual server job.
- [ ] Add migration validation to the manual server job:
  - command: `cd server && go run ./cmd/migrate validate`
- [ ] Add frontend unit test command once script exists.
- [ ] Add package unit test commands once scripts are normalized.

Acceptance:

- Manual CI runs build + tests without relying on local machine state.
- Failures are actionable and not dominated by missing infrastructure.

### Phase 1: Add Test Scripts

Status: not started

Frontend:

- [ ] Add `frontend/package.json` script:
  - `test`: `vitest run`
  - optional `test:watch`: `vitest`
- [ ] Confirm `pnpm --dir frontend run test` passes locally.
- [ ] Split any flaky or browser-dependent tests out of unit test path if needed.

Root/workspace:

- [ ] Add root `test` script or CI-specific scripts.
- [ ] Prefer explicit CI commands over an opaque all-in-one script at first.

Packages:

- [ ] Confirm these pass:
  - `pnpm --dir packages/widget-core run test`
  - `pnpm --dir packages/sdk-js run test`
  - `pnpm --dir packages/react run test`
  - `pnpm --dir packages/nextjs run test`
  - `pnpm --dir help-center run test`
- [ ] Add missing `test` scripts where packages have tests but no script.

Acceptance:

- All local unit test suites have named scripts.
- CI can call scripts without knowing internal file globs.

### Phase 2: Enable PR CI

Status: not started

Phases 2 and 3 run in parallel: when `pull_request` is re-enabled, the
backend `go test` job lands on the *same* PR run but as **non-required**
until the stabilization criteria below are met. This avoids a window where
PRs feel "gated" but backend regressions slip through because no test job
runs yet.

- [ ] Re-enable `pull_request` trigger for `main` and `develop`.
- [ ] Keep `workflow_dispatch`.
- [ ] Add concurrency cancellation for repeated pushes to the same PR branch.
- [ ] Land the Phase 3 backend test job at the same time, as non-required.
- [ ] Keep CI required checks initially limited to deterministic suites.

Initial required PR checks (matches branch-protection list above):

- [ ] Server CI (vet + build)
- [ ] Frontend CI (build + Vitest)
- [ ] Help Center CI (build + Vitest)
- [ ] Migration Validate (`go run ./cmd/migrate validate`)

Promotion-to-required criteria (per job):

- 5 consecutive green nightlies on `develop`, AND
- flake rate < 2% over a rolling 7-day window (failed-then-retried-green
  on the same SHA counts as a flake), AND
- p95 runtime within the per-job budget defined above.

Acceptance:

- Every PR runs the fast suite automatically.
- A job is only required after it meets the promotion criteria; there is no
  vague "stabilize for one week" anymore.

### Phase 3: Backend Test Hardening

Status: not started

- [ ] Run `go test ./...` in CI.
- [ ] Fix packages that fail because of missing env or local-only assumptions.
- [ ] Standardize test DB use:
  - SQLite for pure repository/service tests where supported
  - Postgres service for Postgres-specific behavior
  - miniredis for Redis behavior where possible
- [ ] Add CI Postgres service if full backend suite needs it.
- [ ] Add CI Redis service if full backend suite needs it.
- [ ] Add migration validation in CI.
- [ ] Add `go test -race` as nightly first, not PR-required.

Backend commands:

```bash
cd server
go vet ./...
go build ./cmd/api
go build ./cmd/temporal-worker
go test ./...
go run ./cmd/migrate validate
```

Acceptance:

- Full backend test suite passes in CI.
- Test failures do not require provider credentials.
- Provider tests are isolated behind integration tags or nightly jobs.

### Phase 4: Frontend and Package Test Hardening

Status: not started

- [ ] Run frontend Vitest in CI.
- [ ] Run package Vitest in CI.
- [ ] Run help-center Vitest in CI.
- [ ] Keep Vite builds as separate checks; do not treat build as a substitute for tests.
- [ ] Add coverage reporting later only after pass/fail stability is solved.

Commands:

```bash
pnpm --dir frontend run test
pnpm --dir packages/widget-core run test
pnpm --dir packages/sdk-js run test
pnpm --dir packages/react run test
pnpm --dir packages/nextjs run test
pnpm --dir help-center run test
pnpm build
```

Acceptance:

- Unit/component tests pass without Playwright browser installation.
- Build and test failures are separated in CI output.

### Phase 5: Deterministic Browser Tests

Status: not started

Support inbox:

- [ ] Keep using `frontend/playwright.support.config.ts`.
- [ ] Run default mocked support tests:
  - `pnpm --dir frontend exec playwright install --with-deps chromium`
  - `pnpm --dir frontend run test:e2e:support`
- [ ] Keep `frontend/e2e/support/live/**` ignored from default PR suite.
- [ ] Add missing P0 cases from `docs/prds/support-widget-browser-test-coverage.md`.

SDK widget:

- [ ] Run package widget Playwright suite:
  - `pnpm --dir packages/sdk-js exec playwright test --config=./playwright.widget.config.ts --project=chromium`
- [ ] Keep live/provider cases separate.

Help center:

- [ ] PR CI runs help-center Playwright against a **local preview** spun up
      in the job (`pnpm --dir help-center run dev` on a free port + wait-on).
      External docs URL is too fragile for PR-required gating.
- [ ] Nightly may additionally hit the deployed docs URL as a smoke check.

Acceptance:

- Browser suites are deterministic, mocked, and not dependent on shared external state.
- Browser artifacts are uploaded on failure.

### Phase 6: MailSlurp Email E2E

Status: proposed

Use MailSlurp for real mailbox behavior, not as a replacement for deterministic local tests.

MailSlurp is appropriate for:

- confirming a real email reaches a real inbox
- asserting subject/body/reply-to/thread headers
- testing reply-to support conversation threading
- extracting links and verifying rendered content
- validating unsubscribe/reply flows

MailSlurp is not appropriate for:

- every PR unit test
- replacing Postmark webhook unit tests
- testing internal queue logic
- high-volume test loops

Required secrets:

- [ ] `MAILSLURP_API_KEY`
- [ ] Postmark test server token or staging email token
- [ ] test sender domain verified in Postmark
- [ ] inbound route domain pointed at test/staging API

Recommended job type:

- [ ] nightly
- [ ] manual workflow dispatch
- [ ] release gate
- [ ] not initially PR-required

#### MailSlurp Scenario A: Support Offline Fallback Delivery

Status: not started

Flow:

1. Create a fresh MailSlurp inbox.
2. Create or identify a support conversation with `customer_email = inbox.emailAddress`.
3. Send an agent reply while visitor is offline.
4. Wait for fallback delay or force/process the queue in test mode.
5. Wait for MailSlurp email.
6. Assert:
   - subject includes conversation context
   - body includes agent reply
   - `Reply-To` is `conv-{conversation_id}@{reply_domain}`
   - sender matches expected support sender
   - no sensitive tokens or internal data are leaked
7. Assert app DB:
   - `support_email_logs.status = sent` or `delivered` depending on webhook availability
   - `postmark_message_id` is present
   - `to_email` equals MailSlurp inbox address

#### MailSlurp Scenario B: Customer Reply Threads Back

Status: not started

Flow:

1. Reuse Scenario A email.
2. Send a reply from MailSlurp to the email's `Reply-To`.
3. Let Postmark inbound webhook hit `/api/webhooks/postmark/inbound`.
4. Assert:
   - support message created with `sender_type = customer`
   - `via_channel = email`
   - message is attached to the original conversation
   - duplicate inbound webhook does not create duplicate messages
   - websocket/realtime invalidation path emits expected support event

#### MailSlurp Scenario C: Forwarded Mailbox Route Creates Conversation

Status: not started

Flow:

1. Configure a test support email route.
2. Send from MailSlurp to route address.
3. Let inbound webhook process the message.
4. Assert:
   - new support conversation exists
   - contact email/name are parsed
   - mailbox routing is applied
   - inbound HTML/plaintext body is preserved safely

#### MailSlurp Scenario D: Bounce Handling

Status: not started

Flow:

1. Send a fallback email to a MailSlurp address that we configure to
   simulate a hard bounce (or use Postmark's bounce simulator).
2. Wait for the Postmark bounce webhook to hit `/api/webhooks/postmark/delivery`.
3. Assert:
   - `support_email_logs.status = bounced`
   - bounce reason persisted
   - in-app delivery indicator surfaces "Delivery failed" on the message
   - no infinite retry loop (job stops being requeued)

#### MailSlurp Scenario E: Spam Complaint Webhook

Status: not started

Flow:

1. Send a fallback email to a MailSlurp inbox.
2. Trigger a spam-complaint webhook (Postmark sandbox supports this).
3. Assert:
   - `support_email_logs.status = spam_complaint`
   - in-app indicator reads "Marked as spam"
   - subsequent sends to the same address are suppressed per policy

#### MailSlurp Scenario F: Attachment Delivery

Status: not started

Flow:

1. Send a fallback email with one image and one non-image attachment.
2. Wait for MailSlurp delivery.
3. Assert:
   - both attachments are present on the received MIME body
   - sizes match
   - filename encoding survives the round-trip
   - inbound reply with the same attachments threads back correctly
     (tests our parser as well as outbound)

#### MailSlurp Scenario G: HTML Rendering Integrity

Status: not started

Flow:

1. Send a fallback email with rich HTML (links, images, blockquote, list).
2. Wait for MailSlurp delivery.
3. Assert:
   - HTML structure preserved (links resolve, images have src)
   - no broken `<style>` or stripped tags from server-side sanitization
   - plaintext alternative is generated and roughly matches HTML semantics
   - dark-mode prefers-color-scheme block (if we ship one) is present

#### MailSlurp Scenario H: Auth/Product Emails

Status: later

Candidate flows:

- password reset
- invite email
- email verification
- notification digest

Keep separate from support inbox until support email flows are stable.

#### Resiliency: MailSlurp/Postmark Provider Outages

Real-provider tests fail for reasons other than our code: rate limits,
provider 5xx, expired API keys. Without a guard the nightly is permanently
red and the team learns to ignore it.

- [ ] Wrap MailSlurp HTTP calls with a `skipOnProviderError` helper that
      maps known provider failures (HTTP 429/5xx, network timeout, MailSlurp
      "no message after N seconds" when inbound webhook can't be confirmed
      reachable) into `t.Skip` with a clear reason.
- [ ] Emit a structured "skipped, provider error" line so the dashboard can
      separate provider flake from product flake.
- [ ] Page only on consecutive runs that produce *failures*, not on
      provider skips.

#### Forked PRs and Secrets

`MAILSLURP_API_KEY` and Postmark staging tokens are not available to
forked-PR workflows. The `email-e2e` job is restricted to:

- `pull_request` events from same-repo branches (internal contributors), or
- `workflow_dispatch`, or
- `schedule` (nightly).

Forked PRs see the job as `skipped`, not failed. CI status remains green.

Acceptance (overall MailSlurp suite):

- Outbound delivery, inbound threading, bounce, spam, and attachment
  scenarios pass against staging on a green nightly.
- Failures include provider message IDs, MailSlurp inbox IDs, and app
  conversation IDs.
- Provider-error skips are visibly distinct from product failures.
- Tests clean up or use expiring inboxes.

## Support Inbox and Email Coverage Matrix

| Area | Current confidence | PR target | Nightly/release target | Remaining work |
| --- | --- | --- | --- | --- |
| Email fallback queueing | Medium | Go unit/service tests | Redis-backed integration | Ensure full suite runs in CI |
| Postmark send request shape | Medium | Mock HTTP Postmark tests | Real Postmark staging smoke | Keep provider creds out of PR |
| Delivery/open/bounce webhooks | Medium | Handler/service unit tests | Provider webhook replay smoke | Add CI enforcement |
| UI delivery state | Medium | Vitest component/thread tests | Browser support smoke | Keep sent vs delivered semantics tested |
| Inbound reply threading | Medium | Service tests with Postmark payloads | MailSlurp reply-to e2e | Add real inbound route test |
| Forwarded mailbox routes | Medium | Service tests | MailSlurp route e2e | Add E2E fixture setup |
| Widget support chat | Medium | Vitest + mocked Playwright | Real API browser smoke | Gate mocked suite first |
| WebSocket presence/read state | Medium | Unit + mocked Playwright | Real Redis/API smoke | Add nightly only if flaky |
| CRM Gmail sync | Low/medium | Mock Gmail API tests | Optional Gmail sandbox later | Do not mix with MailSlurp |
| Product emails | Low/medium | Mock Postmark tests | MailSlurp smoke | Add after support flows |

## Proposed CI Jobs

### `server`

PR-required after stabilization.

Migration validate runs *before* the build/test steps so a broken migration
fails the job in ~30s instead of after a 5-minute build+test cycle.

Steps:

1. checkout
2. setup Go 1.24
3. start Postgres service (matches prod; SQLite only for tests that opt in)
4. `go run ./cmd/migrate validate` — fail fast on bad migrations
5. `go vet ./...`
6. `go build ./cmd/api`
7. `go build ./cmd/temporal-worker`
8. `go test ./...`

### `frontend`

PR-required after stabilization.

Steps:

1. checkout
2. setup Node 22
3. setup pnpm
4. `pnpm install --frozen-lockfile`
5. `pnpm --dir frontend run test`
6. `pnpm build`

### `packages`

PR-required after stabilization or folded into frontend if runtime is acceptable.

Steps:

1. `pnpm --dir packages/widget-core run test`
2. `pnpm --dir packages/sdk-js run test`
3. `pnpm --dir packages/react run test`
4. `pnpm --dir packages/nextjs run test`

### `help-center`

PR-required after stabilization.

Steps:

1. `pnpm --dir help-center run test`
2. `pnpm --dir help-center run build`

### `browser-smoke`

Start as optional, then required.

Steps:

1. install Chromium
2. run frontend support Playwright mocked suite
3. run SDK widget Playwright mocked suite
4. upload traces/screenshots on failure

### `email-e2e`

Nightly/release/manual only.

Steps:

1. verify required secrets are present
2. create MailSlurp inbox
3. run support fallback delivery scenario
4. run inbound reply threading scenario
5. record provider IDs in test output
6. clean up or rely on expiring inboxes

## Progress Tracker

### Milestone 1: CI Baseline

- [ ] Re-enable PR trigger in `ci.yml`.
- [ ] Add concurrency cancellation.
- [ ] Add backend `go test ./...`.
- [ ] Add migration validation.
- [ ] Add frontend `test` script.
- [ ] Add frontend Vitest to CI.
- [ ] Add help-center Vitest to CI.
- [ ] Add package Vitest jobs.
- [ ] Meet promotion-to-required criteria (see Phase 2): 5 consecutive
      green nightlies + flake rate < 2% + p95 within budget.

Exit criteria:

- Every PR gets build + unit test coverage.
- CI failures are deterministic and actionable.

### Milestone 2: Backend Integration Confidence

- [ ] Identify tests requiring local sockets or services.
- [ ] Add CI Redis where required.
- [ ] Add CI Postgres where required.
- [ ] Mark real-provider tests with integration/nightly boundaries.
- [ ] Add `go test -race` nightly.
- [ ] Add migration validate to PR and staging deploy workflows.

Exit criteria:

- Backend tests run consistently in CI.
- Race suite runs outside PR path.

### Milestone 3: Frontend and Widget Browser Confidence

- [ ] Enable mocked support Playwright as optional CI.
- [ ] Fix flake and artifact reporting.
- [ ] Promote mocked support Playwright to required.
- [ ] Enable SDK widget Playwright as optional CI.
- [ ] Promote SDK widget Playwright to required if runtime is acceptable.
- [ ] Decide local-preview path for help-center e2e.

Exit criteria:

- Support and widget browser regressions are caught without external services.

### Milestone 4: MailSlurp Real Email Confidence

- [ ] Choose MailSlurp account/project and billing limits.
- [ ] Add `MAILSLURP_API_KEY` to CI secrets.
- [ ] Add staging/test Postmark token secrets.
- [ ] Add test sender/domain and inbound route configuration.
- [ ] Build helper for creating expiring MailSlurp inboxes.
- [ ] Implement support fallback delivery test.
- [ ] Implement inbound reply threading test.
- [ ] Add webhook/replay diagnostics to failure output.
- [ ] Run nightly for one week.
- [ ] Add release-gate smoke.

Exit criteria:

- We can prove real outbound support email delivery and inbound reply threading before release.

### Milestone 5: Reporting and Governance

- [ ] Add CI status badges or dashboard notes.
- [ ] Track runtime by job.
- [ ] Track flaky test count.
- [ ] Track failed-provider tests separately from code tests.
- [ ] Define required checks in GitHub branch protection.
- [ ] Add a short test policy to contributor docs.

Exit criteria:

- The team knows which suite to run for which change.
- Branch protection reflects the agreed quality bar.

## Flaky-Test Quarantine Policy

Without a deadline, `t.Skip` and `.skip()` accumulate forever. Policy:

- A test that fails twice on the same SHA in a 24h window is **flaky** and
  may be quarantined (skipped on PR-required, kept in nightly with
  `allow_failure: true`) by anyone — no triage required.
- The flake's owner has **14 calendar days** to either re-stabilize or
  delete it. After 14 days with no fix, the test is deleted; coverage gap
  filed as an issue against the owning team.
- Quarantined tests appear in a weekly report (Milestone 5 dashboard).
- Adding new code to a quarantined area requires either fixing the test or
  documenting why coverage is acceptable.

## Success Metrics

The project is "done" (Milestone 5 exit) when, for two consecutive weeks:

- ≥ 80% of merges to `develop` had a green required-checks run on the
  merging SHA (no failed-then-overridden merges).
- p95 PR CI runtime < 10 min.
- Flake rate (failed-then-retried-green within 24h on the same SHA) < 2%.
- ≤ 5 quarantined tests in the active list.
- Nightly real-email suite green on at least 5 of the last 7 nights
  (provider-error skips don't count against this).

## CI Cost Budget

`arc-runners-helpin-ai` and self-hosted Postgres services have real cost.
Soft monthly budget (revisit quarterly):

- ≤ 8,000 self-hosted runner minutes/month for PR + nightly combined.
- Per-PR average ≤ 10 runner-minutes (i.e., 10 jobs × 1 min, or 2 × 5 min,
  whatever the parallelism gets you).
- If we sustain over budget for 2 consecutive weeks, slow tests get demoted
  from PR to nightly; we don't just buy more runners.

## Test Ownership

Suggested ownership:

- Backend unit/integration: platform/backend owner
- Frontend Vitest: frontend owner
- Support inbox Playwright: support product owner + frontend owner
- Widget Playwright: SDK/widget owner
- MailSlurp email e2e: support/backend owner
- CI workflow and branch protection: platform/devops owner

## Initial Priority Order

1. Make CI run on PRs.
2. Add `go test ./...` and fix failures.
3. Add frontend/package Vitest scripts and CI steps.
4. Add migration validation.
5. Add deterministic Playwright support/widget suites.
6. Add MailSlurp nightly support email flow.
7. Promote stable browser and email smoke checks to release gates.

## Open Questions

The original five open questions are resolved — see "Decisions Made" at
the top of this document. Questions still genuinely open:

- Do we publish the per-job runtime/flake dashboard inside GitHub Actions
  insights, or pipe to a separate observability backend?
- Who owns the on-call rotation for the nightly real-email suite when it
  fails outside business hours?
- Should the help-center Playwright eventually run against a per-PR
  preview deployment instead of an in-job local preview?

