# Jev support integration

Implemented on `feat/semantic-decision-engine`, after merging `origin/develop`
commit `1c7a37de3` (merge `dcf1697b8`). The earlier review's disabled/shadow rollout
recommendation is superseded by the user's explicit request: **primary routing
and automatic tagging are the defaults when `JEV_API_KEY` is configured**.
Production release is prepared on `release/jev-production` from `origin/main` at `b746ff2d9`. Only backend integration files are included; experimental datasets and model artifacts are excluded.

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
- **Independent controls:** routing and tags each support `primary`, `shadow`,
  and `off`. Shadow calls record results without routing/tag writes. Initial calls
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
| `JEV_TIMEOUT_MS` | 1000 | Per-request deadline, at most 2000; no retries |
| `JEV_WORKSPACE_IDS` | empty | All workspaces; specify IDs to restrict |
| `JEV_ROUTING_THRESHOLD` | 0.90 | Minimum selected mailbox probability |
| `JEV_TAG_THRESHOLD` | 0.95 | Minimum yes probability for a tag |
| `JEV_DAILY_LIMIT` | 1000 | Combined API attempts per workspace, per UTC day |

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
`internal/decision.Provider` / `DecideMany` contract is reusable, but CRM, agent
selection, follow-up actions and other review recommendations are not enabled by
this change.

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

## Validation result (2026-09-17)

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
Routing and tagging default to primary when the key exists. Check for existing
`JEV_ROUTING_MODE` or `JEV_TAGS_MODE` overrides when enabling.

After secret synchronization, restart the API deployment through the normal
operations process and verify rollout readiness, `/api/health`, Jev audit-event
status, tag additions, routing outcomes and LLM fallback errors. A healthy HTTP
endpoint alone does not prove Jev is active.

Emergency feature rollback: set both `JEV_ROUTING_MODE=off` and
`JEV_TAGS_MODE=off` in production, then restart the API deployment. Existing LLM
routing remains available. Already-added tags are not automatically removed.
The previous server-family release was `server-v0.95.403`; use GitOps to revert
images if a full binary rollback is needed. No new migrations accompany Jev.
