# Jev support integration

This guide is for operators configuring Jev support decisions and contributors
tracing their behavior. The checked-in API enables routing, tagging, handoff, and
follow-up decisions by default when a valid `JEV_API_KEY` configuration is supplied.
Source presence and manifests do not establish that the deployed secret contains
the key or that production is running this code.

The original implementation used `feat/semantic-decision-engine` and a
`release/jev-production` branch based on `b746ff2d9`; those are historical release
records, not instructions to switch to or deploy those branches.

## Behavior

- **Routing:** existing human ownership, channel/settings, entitlement, explicit
  rules and daily triage budget checks retain precedence. The dynamic mailbox
  descriptions go to Jev. An accepted result uses the normal triage suggestion/
  auto-move path, so existing `TriageAutoMoveEnabled` still controls actual moves.
  Shared/unknown keeps the existing shared behavior. Low probability, malformed
  response, version mismatch, timeout, provider failure, admission cap or usage
  persistence failure falls through to the existing LLM. If Jev routing is enabled,
  the optional local-model path is skipped to avoid a three-stage cascade.
- **Tags:** on eligible customer replies, including human-owned conversations,
  evaluate each existing workspace tag independently. The input is up to 12 recent
  public reply turns ending at the triggering message, with a 15,000-byte content
  budget. Internal notes and sender identity fields are excluded. Oversized latest
  turns skip tagging; input is never silently partially truncated within a turn.
  At most 50 tag questions share a request; larger taxonomies use additional
  batches and the same combined daily cap. Routing uses its own request so its
  input/decision contract remains unchanged.
- **Tag writes:** add matching existing tags only; no taxonomy creation, no
  removal of human tags. Manual removals carry `tag_id` metadata and suppress
  re-application on later replies; legacy name-only removal messages are also
  recognized. Suppression is rechecked after the provider call before adding. AI additions have a Jev system message and
  publish the normal conversation-update event. Uncertain tags remain unchanged.
  This reduces routine tagging; it does not eliminate every need for review.
- **Handoff detection:** eligible customer replies are assessed before the support
  agent starts or resumes. Accepted human requests, repeated failed troubleshooting,
  and outstanding human actions use the existing handoff flow, briefing, assignment,
  notifications and analytics. Explicit human-request phrases and turn caps retain
  precedence. Ownership, latest public message and AI control version are rechecked
  after the provider call and fenced again when committing the handoff.
- **Follow-up classification:** due, eligible inactivity episodes are assessed before
  launching Runtime. `team_owes_work` and `needs_human` hand off with the existing
  internal briefing; `confirmed_resolved` and `no_follow_up` skip reminders without
  closing the conversation. `waiting_customer` launches the restricted Runtime
  writer for the contextual question and separate closure notice. Runtime still
  reviews evidence and may choose a safer handoff or skip. Its persisted run input
  contains the accepted classification. Existing sequence timing, delivery checks,
  reply/edit cancellation, citations, ownership and final outcome guards remain.
- **Lifecycle context:** complete public reply text, message IDs and sender roles,
  bounded to 15,000 encoded bytes. Identity metadata, internal notes and deleted
  messages are excluded. Missing source, a newer public reply, empty-text turns or
  oversized history fall back to Runtime instead of truncating away obligations.
  Uncertain decisions, provider/audit/usage failures, admission limits and duplicate
  evaluations also preserve the original Runtime path. No key means no Jev calls.
- **Independent controls:** routing, tags, handoff and follow-up each support `primary`, `shadow`,
  and `off`. Shadow calls record results without applying Jev decisions; the existing
  Runtime behavior still runs. Initial calls
  are synchronous with bounded provider deadlines; shadow is not a background
  zero-latency mechanism. More than 50 eligible tags may add multiple deadlines.
- **No key:** no Jev client is constructed; existing product behavior remains.

The active widget and inbound-email triage paths already call `EvaluateAndRoute`.
The internal customer-reply trigger now permits tag evaluation for human-owned
conversations while the routing ownership guard still prevents a move. Existing
routing rules run before the routing provider; independent tag classification may
still run when a rule handles routing.

## Configuration

All values are server-side; credentials are never exposed to the browser.

| Variable | Default | Meaning |
|---|---|---|
| `JEV_API_KEY` | empty | Enables Jev client construction when supplied |
| `JEV_ROUTING_MODE` | primary | off / shadow / primary |
| `JEV_TAGS_MODE` | primary | off / shadow / primary |
| `JEV_HANDOFF_MODE` | primary | off / shadow / primary |
| `JEV_FOLLOW_UP_MODE` | primary | off / shadow / primary |
| `JEV_TIMEOUT_MS` | 1000 | Per-request deadline, at most 2000; no retries |
| `JEV_WORKSPACE_IDS` | empty | All workspaces; specify IDs to restrict |
| `JEV_ROUTING_THRESHOLD` | 0.90 | Minimum selected mailbox probability |
| `JEV_TAG_THRESHOLD` | 0.95 | Minimum yes probability for a tag |
| `JEV_HANDOFF_THRESHOLD` | 0.95 | Minimum selected handoff probability |
| `JEV_FOLLOW_UP_THRESHOLD` | 0.95 | Minimum selected follow-up state probability |
| `JEV_DAILY_LIMIT` | 1000 | Combined API attempts per workspace, per UTC day |

When a key is present, invalid modes, out-of-range thresholds, or a timeout above
2000 ms fail service/client construction and stop API startup. The timeout is
rejected, not clamped. The request-failure fallbacks described above apply after
successful initialization. See [API wiring](../../server/cmd/api/main.go) and
[configuration validation](../../server/internal/service/support_jev.go).

Thresholds are provisional operational settings, **not** measured 90%/95%
correctness guarantees. The provider confidence (distribution concentration) is
stored separately and never substituted for a probability. The model is pinned
to `jev-1.13.0`; any returned version change is rejected. Context and tag names
are customer data sent to `https://api.typesafe.ai/v1/systemone` when enabled.
The previous frozen Helpin export benchmark remains a separate pending action;
this implementation/test run does not send those records.

## Usage, audit and failure behavior

A workspace row lock serializes admission. An event is reserved before each call,
so simultaneous duplicate requests cannot both execute and failed calls consume
the cap. The fingerprint includes model/question contract, input and dynamic
choices. Repeated identical evaluations skip the external call; routing falls
back if no fresh Jev result is available. Failed/pending admissions are not retried
for the same fingerprint, preventing repeated charges; later new messages can
produce a new tagging fingerprint.

The existing `support_conversation_triage_events` table stores `jev_decision`
events containing probabilities, provider confidence, modes, thresholds, version,
usage and status. It does not store request text or the credential. Provider errors
never include response bodies. `ai_execution_usage` records token usage with a
unique call identity in both editions. **This initial Jev path is operator-funded
and does not debit customer AI credits.** The ordinary LLM fallback retains its
existing metering. This explicit pilot cost policy avoids inventing customer
pricing for a new provider; a per-workspace daily call cap bounds request volume.

No migrations or new runtime dependencies are required. The shared
`internal/decision.Provider` / `DecideMany` contract now powers handoff and
follow-up classification. CRM and agent selection remain outside this integration.

## Validation commands

```sh
cd server
go test -race ./internal/decision ./internal/service ./internal/config \
  -run 'Test(SupportInboxTriage|SupportTag|SupportDecision|Jev)' -count=1
go vet ./...
go build ./...
```

Tests use fake HTTP transports and SQLite fixture conversations. They do not use
production credentials or send customer text to Jev. Earlier synthetic API measurements are retained in the semantic-decision-engine research worktree.

## Recorded validation result (2026-09-17)

The following is the implementation author's historical report. This docs audit
compared source but did not rerun these full suites, access a production cluster,
or verify the referenced release branches. Use results from the exact revision
you intend to deploy.

Community and enterprise builds pass, as does `go vet ./...`. Targeted tests pass
with `-race`. Full service, repository, config and decision package tests pass
with `TZ=UTC`. Two existing SQLite expiry tests fail in the machine's local
Europe/Berlin timezone; the same failures were reproduced using pre-Jev merged
sources via a Go overlay. No unrelated expiry code was changed.

The isolated production release also passed targeted race tests, enterprise build,
and `go vet ./...` on 2026-09-17.

## Production activation and rollback

The existing main-branch production workflow builds the server family and updates
production GitOps manifests. Production loads environment values from Kubernetes
secret `helpin/helpin-secrets`. The local `server/.env` is not deployed.
Configure `JEV_API_KEY` in the production secret source; do not commit it.
All four decisions default to primary when the key exists. Check all four mode
overrides when enabling.

After secret synchronization, restart the API deployment through the normal
operations process and verify rollout readiness, `/api/health`, Jev audit-event
status, tag additions, routing outcomes and LLM fallback errors. A healthy HTTP
endpoint alone does not prove Jev is active.

Emergency feature rollback: set `JEV_ROUTING_MODE=off`, `JEV_TAGS_MODE=off`,
`JEV_HANDOFF_MODE=off` and `JEV_FOLLOW_UP_MODE=off`, then restart the API deployment.
Existing LLM routing remains available. Already-added tags are not automatically removed.
For binary rollback, select the previously verified release from the current
GitOps history and deployment record. The original rollout named
`server-v0.95.403`; it is not a permanently valid rollback target. The checked-in
[production API manifest](../../k8s/prod/server.yaml) identifies the desired image
and secret reference, but does not prove cluster state. Jev uses existing schemas.

## Lifecycle integration verification

The lifecycle extension was originally developed on `feat/jev-support-lifecycle`. It reuses
existing schemas and public APIs; there is no new UI, migration or dependency.
Existing inbox handoff notes, follow-up status/reason and conversation events
expose the outcomes. Both new controls are independent of routing/tagging.

Capability parity: customer reply entry points, channel settings, explicit human
requests, hard turn caps, ownership, agent permissions, reply billing, episode
eligibility, timing, delivery verification, citation validation, human takeover,
reopening and two-stage reminders retain their existing paths. New decisions
only bypass Runtime when no generated reply is needed. Jev calls share the
operator-funded admission cap and audit/usage accounting; Runtime generation
retains its existing metering.

Test commands (from `server/`):

```sh
TZ=UTC go test -race ./internal/service ./internal/config ./internal/decision \
  -run 'Test(Jev|SupportFollowUp|SupportAIControl|SupportChat|ScheduledSupportFollowUp)' -count=1
# Use a disposable PostgreSQL database, never production.
TZ=UTC SUPPORT_FOLLOWUP_TEST_DSN='host=127.0.0.1 port=5432 user=postgres password=test dbname=test sslmode=disable' \
  go test -race -tags=integration ./internal/service \
  -run 'Test(JevFollowUpPostgres|SupportFollowUpPostgres|SupportAIControlPostgres)' -count=1
go vet ./...
go build ./...
go build -tags=enterprise ./...
```

Tests use synthetic conversations and fake provider/Runtime responses. They verify
wiring and guarded effects, not Jev classification accuracy on customer data.
Thresholds remain provisional; no live customer transcript evaluation or production
deployment is part of these local checks.

The original implementation report records lifecycle validation on 2026-09-17 as passing: full service, repository, config and
provider suites with `TZ=UTC` and `-race`; the targeted PostgreSQL lifecycle and
control integration suites with `-race`; `go vet ./...`; and community/enterprise
builds. PostgreSQL tests used a disposable local container, removed afterward.


## PM triage

The PM integration uses the same configured Jev provider and workspace allowlist,
with independent controls:

| Variable | Default | Meaning |
| --- | --- | --- |
| `JEV_PM_MODE` | primary with a key | off / shadow / primary |
| `JEV_PM_THRESHOLD` | 0.95 | Minimum selected-option probability |
| `JEV_PM_DAILY_LIMIT` | 1000 | Separate attempted calls per workspace per UTC day |

Apply `202609170002_pm_triage.sql` through the normal migration runner before
starting this API version. Its assessment and label-suppression tables are SQL-owned.
Provider usage is recorded as `pm_triage`, under the operator-funded pilot policy.
Failed calls consume admission budget; identical successful inputs reuse an
actor-scoped assessment. Internal support notes are excluded from Jev context.

Primary mode automatically adds qualifying existing task labels on creation and
content edits. Human removals suppress later automatic additions. Task type/team
changes, task relationships and support task creation require review. Priority is
not derived from classifier probability. Shadow mode records decisions without
applying labels or exposing suggestions. Set `JEV_PM_MODE=off` to disable PM
classification independently of support routing and lifecycle decisions.

## Implementation references

- [Provider contract](../../server/internal/decision/jev.go): endpoint, pinned model,
  timeout and response validation.
- [Decision service](../../server/internal/service/support_jev.go) and
  [admission repository](../../server/internal/repository/support_jev.go): modes,
  tag batching, usage records, fingerprints and daily cap.
- [Lifecycle context](../../server/internal/service/support_jev_lifecycle.go),
  [handoff](../../server/internal/service/support_chat_jev.go), and
  [follow-up](../../server/internal/service/support_ai_follow_up_jev.go): bounded
  complete history, guarded outcomes and Runtime fallback.
- [Production workflow](../../.github/workflows/deploy-prod.yml): server builds
  depend on detected server changes or an explicit forced release, not every
  documentation-only push to main.
