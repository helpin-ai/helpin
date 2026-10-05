# Jev product decisions

This extends the existing direct Jev provider with five independent, operator-funded classifiers. CRM signal classification is excluded. A configured `JEV_API_KEY` enables these classifiers by default; explicit feature modes and the workspace allowlist still take precedence.

## Controls and rollout

Apply SQL migrations through `202609170004_flow_condition_activity` before enabling the code. The first migration owns `jev_decision_attempts`; the second allows Flow condition history without an agent and adds outcome/provenance columns. Both are idempotent. API and Temporal worker need the same settings.

All five modes default to **primary**, thresholds to **0.95**, and daily limits to **1000 attempted calls per workspace per feature**:

| Environment prefix | Purpose |
| --- | --- |
| `JEV_MEETING_ROUTING` | Internal/customer meeting follow-ups |
| `JEV_COVERAGE_CLASSIFICATION` | Conversation and knowledge-gap categories |
| `JEV_COVERAGE_TOPIC_MATCHING` | Same-need versus related/distinct topic candidates |
| `JEV_AUTOMATION_CONDITION` | Optional semantic filters on event-triggered Flows |
| `JEV_ANSWER_EVIDENCE` | Support of a question by each returned knowledge excerpt |

Each prefix accepts `_MODE=off|shadow|primary`, `_THRESHOLD`, and `_DAILY_LIMIT`. `JEV_API_KEY` and `JEV_WORKSPACE_IDS` are shared with the existing support integration. Without a key, no provider is constructed. Use a limited workspace allowlist for an initial pilot. Thresholds are provider probabilities, not locally calibrated accuracy estimates.

Translation and language detection use the configured translation provider. They
have no Jev dependency. Retired `JEV_TRANSLATION_REVIEW_*` and
`JEV_LANGUAGE_DETECTION_*` settings are ignored.

For an evaluation-only rollout, explicitly set the relevant `_MODE=shadow` with reviewed, representative multilingual examples. Compare routing/category/matching outputs against human decisions and review false positives, abstention, fallbacks, latency, and usage. Promote one feature at a time after acceptable results. Synthetic tests verify mechanics and guards; they do not establish classifier quality on real customer traffic.

Returning a mode to `off` stops new calls and restores existing behavior for meetings and Coverage, and removes advisory answer labels. It does not reverse existing assignments. Existing topic membership history supports human correction; canonical topics are never automatically merged or deleted.

**Flow exception:** a configured semantic condition is an explicit gate. Off, shadow, missing key, budget limits, provider/audit failures, oversized input, uncertainty, and stale evidence all skip the action. Turning the classifier off must never bypass that gate. To remove it, clear the condition in the Flow editor. The editor shows configuration availability and disables adding or editing conditions without active Jev. Saved conditions remain visible and can be explicitly removed; unrelated edits remain possible. The API also rejects new or changed conditions while unavailable. Availability describes configuration, not provider health or remaining budget. Conditions are supported on event-triggered Flows; schedule/CRM-playbook conditions are rejected. Existing Flows without conditions keep their behavior.

## Evidence and behavior

- Meetings: keep generated draft text. Assess complete bounded draft/transcript input and select an exact supplied quote supporting the scope. Incomplete/oversized input or ambiguous output keeps the previous routing path. Existing revision, linked-work and pending-draft guards still apply.
- Coverage: constrain the existing narrative generator to accepted classifications. Reject contradictory generated categories before persisting a finding. Jev does not generate recommendations or set business priority.
- Topic matching: compare up to 12 open, workspace-owned candidates selected from the existing top-500 topic list using Unicode token overlap. This candidate set is explicitly non-exhaustive; it is not semantic vector retrieval. Multiple confident matches or unresolved candidate uncertainty go to review. Manual assignments, dismissed/reviewed signals, superseded findings and edited/closed candidate topics are checked again inside the assignment transaction. New topic identity preserves Unicode and word order.
- Flow conditions: use supplied event metadata and the current task title/description when available. No extra document, PR-body, code or run-output fetching is implied. Existing entitlement, trigger, scope, loop, action and target guards remain. Re-read the Flow and task after classification before allowing an action. This revision check narrows the external-call race; it is not a distributed atomic transaction with the downstream action. Activity distinguishes a matching condition from an executed action and shows skipped/unavailable/uncertain/stale outcomes.
- Answer evidence: assess the actual question and the exact returned excerpts with source IDs, including preview searches. `answer_support` is advisory (`supported`, `partial`, `unsupported`, `uncertain`) and explicitly scoped to `returned_excerpt_only`. It does not filter/rerank retrieval, raise grounded confidence, or bypass citation, reply, handoff or publication guards. Full authorized evidence remains available to existing citation validation.

## Operational details

The pinned direct provider receives at most 16,000 encoded bytes per assessment with closed choices and a two-second provider deadline. Inputs are not silently truncated except the already bounded, explicitly labeled excerpts returned by the support knowledge tool. Failed/abandoned calls consume admission budget. Repeated pending/failed inputs have a one-minute cooldown. Identical ready results are cached by feature/version/model/evidence/questions/policy.

`jev_decision_attempts` stores evidence hashes, identity and structured outcomes, not raw evidence. Usage is recorded in `ai_execution_usage` with `jev_<feature>` and attempt-specific idempotency keys, without customer credit debits. Topic memberships, new meeting projections, Flow condition Activity and answer-tool labels carry assessment provenance. Provider/usage/audit errors cannot yield an actionable primary decision.

The API and Temporal worker expose `/metrics` when `METRICS_ADDR` is configured
for that process. Use separate addresses when running both on the same host and
scrape both processes. `helpin_ai_decisions_total` and
`helpin_ai_duration_seconds` label the five features by their lowercase
names above; PM assessments use `pm_triage`. Outcomes include `ready`, `shadow`,
`cached`, `pending`, `failed`, `daily_limit`, `input_limit` and `error` as applicable.
Duration measures decision service time, including admission, persistence and
cache hits. Counters distinguish cached results from new decisions. Labels contain no
source IDs or evidence. These metrics measure execution, not classifier quality.

## Verification

- Race-tested decision, configuration, meeting, Coverage, topic, answer-tool and Flow success/failure/fallback paths, with existing regression suites.
- Disposable PostgreSQL: both migrations applied twice; simultaneous admission deduplication and cap enforcement; nullable-agent condition history; concurrent human topic membership versus semantic assignment.
- Flow component tests: create/edit/remove round trip, scheduled-condition rejection, disabled/error states, Activity outcomes. Chromium harness tests exercise keyboard saving, narrow layout, light/dark themes and condition removal.
- Both API/worker editions and both frontend editions, backend vet, module tidy and clean-diff checks. Full service/repository/handler/model suites pass with `TZ=UTC`; two pre-existing SQLite expiry tests fail under local Europe/Berlin time and reproduce on the earlier PM branch.

Browser harness: `pnpm --dir frontend exec playwright test --config playwright.automation.config.ts`.
PostgreSQL tests: set `PM_TRIAGE_TEST_DATABASE_URL` to a disposable database, then run `go test -race -tags integration ./internal/repository -run TestJevProductPostgres` from `server/`.
