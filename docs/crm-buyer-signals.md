# CRM Buyer Signals

This is the canonical engineering and operations reference for Helpin buyer
signals. It describes the implemented system. Historical design decisions and
the original phase plan remain in
[`crm-buyer-signals-assessment.md`](crm-buyer-signals-assessment.md), but that
assessment is not a statement of current implementation status.

## Purpose

Buyer signals turn evidence already owned by Helpin into explainable CRM
intelligence. Evidence can come from conversations, support, delivery work,
deal state, website behavior, instrumented product activity, or normalized
external providers.

The system deliberately separates:

- **observation** — immutable, motion-agnostic evidence emitted by a detector;
- **interpretation** — versioned commercial meaning for one observation and
  one concurrent motion;
- **signal type** — the stable seven-value CRM taxonomy;
- **detector** — LLM extraction or a deterministic versioned rule;
- **domain and polarity** — where evidence came from and which direction it
  moves the account;
- **confidence** — whether extracted evidence is supported;
- **business priority** — how important the evidence is now;
- **activation** — whether this exact rule version may create downstream work.

## Delivered phases

| Phase | Releasable scope |
|---|---|
| 1 — motion-aware spine | observations, account motion resolver, pipeline defaults and deal overrides, versioned interpretation maps, immutable meaning snapshots, motion-exit supersession, `(entity, motion)` composition, evidence-gated lanes, routing ownership, and the objective shadow-to-live gate |
| 2 — customer behavior | shared server-only event catalog, commercial-state materialization and health, weekday baselines, six customer rules with level-rule re-arm and anomaly suppression, delayed subscription-outcome calibration, recommendations, and operator UI |

Phase 1 can be released and calibrated using existing evidence producers.
Phase 2 collection, state materialization, baselines, and rule evaluation run
for event-enabled workspaces during shadow mode so history is warm before
cutover. Workspace rollout and per-rule shadow/activation policy gate only
user-visible feeds and downstream actions.

## Architecture

```text
connected email, calendar, support, PM, and CRM records
  -> LLM extraction or daily deterministic rules
                                                    \
browser and authenticated product events             -> immutable observation
  -> authenticated event pipeline                     -> motion resolver
  -> ClickHouse helpin.events                          -> versioned interpretation
  -> ten-minute deterministic behavioral rules        -> durable signal per motion
                                                       -> scoring and lane composition

normalized external evidence API                     /
```

Postgres is the CRM source of truth. ClickHouse stores high-volume behavioral
evidence; CRM pages do not query it synchronously. Evaluators aggregate bounded
ClickHouse evidence and write durable `crm_buyer_signals` rows to Postgres.

## Signal data model

The stable signal types are:

- `buying_intent`
- `objection`
- `competitor_mention`
- `budget_signal`
- `timeline_signal`
- `champion_signal`
- `risk_signal`

Independent dimensions provide the context that should not be encoded in that
taxonomy:

| Dimension | Values or purpose |
|---|---|
| `detector_kind` | `llm_extracted`, `rule_derived`, or `manual` |
| `signal_domain` | conversation, web behavior, product usage, support, delivery, relationship, or market |
| `polarity` | positive, negative, or neutral |
| `rule_key`, `rule_version` | immutable detector identity and semantics |
| `commercial_motion` | prospecting, conversion, onboarding, adoption, expansion, renewal, or retention |
| interpretation snapshot | immutable type, polarity, action, weight, half-life, context, and mapping version |
| evidence window | bounded period evaluated by a rule |
| identity method and trust | provenance used by activation gates |
| evidence fingerprint | idempotency and repeat-dismissal suppression |
| feedback state | reviewed, dismissed with reason, or acted |

The model lives in `server/internal/model/crm_signal*.go`. Schema changes are in
the versioned `server/internal/dbmigrate/sql/*_crm_signal_*.sql` migrations.

## Signal producers

### Conversation extraction

Stored email, calendar, note, call, and supported conversation sources can
enqueue the Temporal `SignalDetectionWorkflow`. The LLM returns structured
signal candidates, but persistence verifies that quoted evidence literally
exists in normalized source text. Deterministic workflow IDs, source-level
uniqueness, thread suppression, and evidence fingerprints prevent duplicate
signals. Verified extraction is recorded as the immutable
`conversation_signal_extraction` detector version. It starts in shadow mode
and becomes routable only after an admin promotes that exact version.

See [`crm-buyer-signal-ingestion.md`](crm-buyer-signal-ingestion.md) for the
email ingestion details.

### Daily first-party rules

The daily evaluator derives signals from trusted Postgres relationships:

| Rule key | Evidence |
|---|---|
| `support_volume_spike` | recent support volume versus account baseline |
| `urgent_issue_open_deal` | high-priority support issue on an active deal |
| `support_ai_escalation` | support conversation escalated to a human (neutral operational context in v2) |
| `support_csat_deterioration` | recent CSAT decline versus account baseline |
| `requested_feature_shipped` | support-requested, company-linked PM feature task completed |
| `deal_stage_stalled` | time in stage exceeded the configured threshold |
| `deal_gone_dark` | no inbound activity in the configured period |
| `champion_quiet` | a known champion stopped responding |
| `timeline_followup_lapsed` | promised date passed without follow-up |
| `deal_single_threaded` | material deal depends on one engaged contact |
| `renewal_approaching` | renewal is near without recent engagement |
| `buying_committee_expanded` | new participants joined the account motion |
| `buying_committee_shrank` | previously engaged participants disappeared |

### Behavioral rules

The behavioral evaluator runs at startup and every ten minutes when
`CLICKHOUSE_DSN` is configured:

| Rule key | Default activation class |
|---|---|
| `repeated_pricing_activity` | verified identity may be promoted |
| `procurement_page_activity` | verified identity with an open deal may be promoted |
| `known_contact_returned` | verified identity may be promoted |
| `high_intent_product_event` | authenticated server event with verified identity may be promoted |
| `configured_form_submission` | verified identity may be promoted |
| `session_depth_spike` | context only; shown as a deep browsing session after the configured pageview threshold, with no historical-baseline claim |
| `new_account_stakeholder` | context only |
| `anonymous_account_traffic` | context only |
| `campaign_attributed_return` | context only |
| `pre_identification_history` | context only; emitted once when the visitor's first identified event makes earlier anonymous activity attributable |
| `identified_article_view` | context only |
| `versioned_interaction` | disabled and context only by default |

An event reaching ClickHouse does not guarantee a CRM signal. Before a
behavioral candidate is stored, the evaluator resolves its external identity to
a CRM contact, company, or deal. Candidates with no CRM entity are discarded;
the procurement rule additionally requires an open deal.

SDK identify calls retain both the durable browser `anonymous_id` and the
customer's `external_user_id` on the CRM identity link. Signed widget HMAC
proof determines verified trust. This lets later browser, cross-device, or
server events resolve through either identifier without treating an unverified
browser claim as verified evidence.

Installing the widget provides website and support behavior. Product-usage
signals require explicit customer instrumentation, normally as authenticated
server events.

### External evidence

`POST /api/crm/signals/external-evidence` accepts normalized funding, hiring,
job-change, technology, leadership, and third-party-intent evidence. The
ingestion contract, provenance, and idempotency are implemented. Provider
connectors are not; external evidence remains context-only by default. The
admin-only endpoint requires a workspace-owned CRM target and normalizes
provider identity to probabilistic trust instead of accepting caller-declared
identity provenance.

### Manual context

`POST /api/crm/signals` creates a user assertion, not verified extracted
evidence. The server always records these rows with `source_type=manual`,
`detector_kind=manual`, `identity_method=manual_entry`, and untrusted identity.
Rule identity, source identity, and activation fields are not accepted from the
client. Contact, company, and deal references must belong to the route
workspace. Manual context can inform a seller, but cannot impersonate a
promoted rule version.

## Evaluation and idempotency

`CRMSignalRuleEvaluator` runs two coordinated schedules:

- daily rules use a 48-hour overlap;
- behavioral rules use a 15-minute overlap on a ten-minute cadence.

Global watermarks and leases prevent replicas from evaluating the same cadence
concurrently. Each run records its window, candidate count, inserted count, and
failure. Durable uniqueness is based on workspace, rule/version, resolved
entity, evidence window, and fingerprint.

Rule configuration is immutable by version. Threshold changes create and
activate a new version rather than mutating the meaning of historical signals.

### Observation and interpretation

`crm_signal_observations` stores evidence once, before commercial meaning is
assigned. At detection time the motion resolver appends its applicable-motion
set and input snapshot to resolver history. Late evidence cannot overwrite a
newer snapshot. Identical resolver snapshots are not rewritten, and the audit
retains 400 days; the signal's own immutable snapshot remains authoritative
after audit retention. A versioned `(rule_key, rule_version, motion)` mapping then
creates one `crm_buyer_signals` row per applicable interpretation. No mapping
means no commercial signal; the observation remains visible to mapping-coverage
metrics. Signal
type, polarity, recommended action, business weight, half-life, and mapping
version are immutable after creation. Read-time rescoring may change magnitude
as evidence ages; it cannot change meaning.

Interpretation contracts are deployment-versioned data rather than mutable
workspace settings. The current migrations seed explicit motion mappings for
every enabled global rule version and fail migration if any enabled version is
left unmapped.

Motions are concurrent. Composition therefore groups by `(entity, motion)`,
not only entity. Lane membership is evidence-gated: subscription or deal state
only determines whether an interpretation applies; an account enters a lane
only when that lane has an active signal above its threshold.

Deal motion resolves from the deal override first and the pipeline's
`default_commercial_motion` second. Existing pipelines are seeded using an
explicit renewal/expansion name match; ambiguous pipeline names remain
`new_business` and are corrected once in pipeline settings. This avoids a
per-deal backfill and makes renewal and expansion usable before customer-state
instrumentation exists.

Supersession ownership follows the most specific entity. Contact lifecycle
refreshes only contact-scoped signals; signals carrying a deal ID remain active
until that deal's pipeline, override, or stage reconciliation exits its motion.

## Scoring and composition

The read side calculates a versioned business score independently of LLM
confidence. Factors include:

- rule weight and domain weight;
- exponential recency decay using a rule half-life;
- evidence identity trust;
- signal polarity;
- deal amount and stage context;
- corroboration across independent domains.

Responses expose `business_priority`, `signed_impact`, `severity`,
`score_version`, and the complete factor breakdown. Initial weights and
half-lives are configurable heuristics and require calibration against won,
lost, expansion, and churn outcomes.

Composition correlates signals that describe the same underlying evidence
source before summing account priority. Support and email signals use their
conversation or thread ID, standalone sources use their source ID, and other
signals fall back to the evidence fingerprint. All signals remain inspectable,
but each source contributes one priority, one directional impact, and one
representative domain to compound scoring.

Priority uses absolute evidence strength within a motion so opposing evidence
cannot net a high-stakes account to zero. When positive and negative strength
are within the configured ambiguity band, the row is marked
`needs_judgment=true` and no recommended action is shown. Motion exit
supersedes, but never rewrites or revives, old interpretations. If supersession
alone changes the row's direction, the API marks that transition so the UI can
explain why a recommendation appeared without new evidence.

Level rules emit only on threshold crossing and require an explicit re-arm
crossing before another observation. Capacity saturation uses 85% to trigger
and 80% to re-arm.

## Activation and feedback

All seeded deterministic rule versions start in `shadow_mode=true`. Shadow
rules evaluate and store signals, but cannot route notifications or create
tasks. This is the expected safe default.

CRM admins promote an activation-eligible global rule version by creating its
workspace policy copy. Promotion enables that version and clears shadow mode
for the workspace; context-only rule versions cannot be promoted.

A signal becomes activation-eligible only when:

1. its exact rule version is enabled, activation-enabled, and no longer in
   shadow mode;
2. an active routing policy exists;
3. business priority meets the policy threshold;
4. identity trust meets the policy requirement;
5. the signal was not dismissed or already delivered; and
6. no matching open task exists.

Routing policies support feed, notification, and digest deliveries. Delivery
rows make routing idempotent across replicas and restarts. Notification and
digest rows move through pending, sending, sent, or failed states; failed and
abandoned sends are reclaimed by a later routing sweep. The
`buying_signal_to_task` automation template requests only activation-approved
signals and remains approval-gated.

Users can mark a signal reviewed, acted, or dismissed with one of the supported
reasons. Precision reports retain rule version, trust, source, domain, and
detection-to-feedback timing so promotion decisions can be evidence-based. A
signal contributes once using its latest feedback outcome, so review followed
by action does not inflate the precision denominator.

Dismissal is workflow feedback, not churn ground truth. Retention calibration
joins signals to later subscription outcomes and slices provenance using the
persisted `identity_method` (including `server_event`). The clean cutover does
not replay legacy evidence; if replay is added later, replay-created signals
must set `replay_calibration_excluded` because today's motion cannot be treated
as the motion at the original event time.

### Shadow release gate

Phase 1 remains in shadow mode until a workspace has at least 100 observations,
an unmapped-observation rate below 5%, a duplicate fingerprint-and-motion rate
below 1%, and zero immutable-meaning violations. The last condition is a hard
test invariant: two rescores may change priority, never stored motion, type,
polarity, or meaning fingerprint. `GET /signals/shadow-gate` exposes the gate.

## Product and API surfaces

Signals appear in the CRM Signal Inbox and on contact, company, and deal pages.
The inbox leads with why the evidence matters now, a recommended next step,
and navigation to the resolved CRM record; detailed scoring stays behind an
evidence disclosure. The workspace feed uses the shared query builder for owner, account, domain,
polarity, signal type, detection date, and identity trust, with additional
computed severity, age, and status controls. Account and meeting briefs group
corroborating evidence and explain what changed.

The inbox is divided into server-owned per-motion lanes. Each lane has its own
rank, total, page, and page size, so a high-volume expansion queue cannot hide
retention accounts or falsify its count. Each row represents one account and
one motion, shows other active motions as context, and carries at most one next
action. Owner resolution is lane-aware: customer lanes prefer the customer
success owner; deal-led lanes prefer the deal owner; both then fall back to
company owner, the dedicated signal-routing default, the active workspace
owner, and finally visible-but-unassigned. Routing defaults live in
`crm_signal_routing_settings`, not autonomy settings.

Review suggestions retain their supporting signal IDs. Review cards display
that evidence and preview immediate deal creation or stage changes before
approval. Approval atomically claims one pending suggestion before execution,
and dismissal requires a calibration reason. Suggestion reads and mutations,
plus every execution target, are workspace-scoped.

Primary routes under `/api/crm`:

| Method | Route | Purpose |
|---|---|---|
| GET | `/signals/feed` | ranked, grouped workspace feed |
| GET | `/signals/shadow-preview` | admin-only preview excluded from the regular feed |
| GET | `/signals` | paginated signal list |
| GET | `/signals/brief` | current account/deal signal brief |
| GET | `/meetings/{id}/signal-brief` | meeting-specific brief |
| GET | `/signals/precision` | rule feedback and precision report |
| GET | `/signals/outcomes` | delayed subscription-outcome calibration |
| GET | `/signals/shadow-gate` | objective Phase 1 shadow-release gate |
| GET | `/signals/rollout` | current workspace shadow/live cutover state |
| POST | `/signals/rollout/activate` | activate a workspace after its shadow gate passes |
| GET | `/signals/rules` | effective shadow/live rule versions |
| GET/PUT | `/signals/routing-settings` | dedicated default signal ownership |
| GET/POST | `/signals/routing-policy` | inspect or create a policy version |
| POST | `/signals/rules/{ruleKey}/versions/{version}/activate` | activate a rule version |
| POST | `/signals/{id}/review` | record review |
| POST | `/signals/{id}/dismiss` | dismiss with a reason |
| POST | `/signals/{id}/acted` | record action |
| POST | `/signals/external-evidence` | ingest normalized provider evidence |

All routes are workspace-scoped and protected by CRM RBAC. Routing policy,
rule promotion, and external-evidence ingestion routes require CRM admin
permission.

## Server-authenticated customer state

Commercial state is server-credential only in v1. Browser identity proofs are
not widened with billing fields or a second freshness lifetime. Customers with
a backend send state using server credentials; verified browser identity still
supports ordinary feature-use evidence.

The reserved commercial-event and company-field list has one JSON source in
`server/internal/eventcatalog/commercial_events.json` and is generated into
both Rust capture and Go evaluation code. Rust rejects browser submissions at
ingest; Go rechecks before observation creation. Tests fail when either
generated catalog drifts from the shared source.

Commercial-state updates use PATCH semantics: omitted values are preserved and
explicit null clears a value. Valid out-of-order updates are retained in
history with `applied_to_current=false`; they do not change current state or
inflate integration-health failures. Conflicting same-timestamp updates,
unsupported patches, and timestamps more than five minutes in the future are
rejected with stable reason codes. Per-company health exposes the last accepted
update and rejected-update count without leaking internal storage errors.

The state materializer uses the shared evaluator lease/watermark table with a
separate lease and watermark per workspace. A workspace's first successful
sweep covers the retained 366-day window; later sweeps use a 24-hour overlap
every ten minutes. Only one replica owns a workspace sweep, event IDs are
idempotent within each workspace, events order by `state_updated_at`, and
company IDs resolve in batches. A broken workspace retains its own watermark
and backlog without blocking healthy workspaces from advancing. Active
event-enabled workspaces are included in shadow mode; rollout mode does not
gate collection.

A server-authenticated state assertion uses the external CRM company ID and a
separate state timestamp:

```json
{
  "event_type": "commercial_state_updated",
  "company": {
    "id": "external-company-id",
    "commercial_state": {
      "subscription_status": "active",
      "plan_key": "growth",
      "seats_purchased": 25,
      "seats_used": 22,
      "renewal_at": "2026-11-01T00:00:00Z"
    }
  },
  "event_attributes": {
    "state_updated_at": "2026-08-28T12:00:00Z"
  }
}
```

The seeded customer rules are:

| Rule | Detection contract |
|---|---|
| `account_usage_decline` | at least five of seven days below 60% of the account's own weekday median |
| `activation_stalled` | onboarding has passed day seven without first value; a new onboarding timestamp creates a new scope |
| `workflow_failure_spike` | at least five of seven days above 2× weekday expectation, with at least three failures on a tripped day |
| `capacity_saturation` | seats cross 85%; re-arms only after utilization falls to 80% |
| `payment_failed` | verified `server_event` edge |
| `downgrade_requested` | verified `server_event` edge |

Usage decline compares each of seven days to that weekday's eight-week median,
requires eight complete workspace-local weeks with at least three active days
for every weekday, and emits only when five of
seven days are below 60% of expectation. Missing days are represented as zero
rather than disappearing from the baseline. Baselines rebuild in a separately
leased daily job, resolve company IDs and write rows in batches, and rules read
the persisted values while scanning only the seven recent local-calendar days.
Superseded baseline windows retain 90 days for diagnosis and are pruned by the
daily evaluator thereafter. Baseline leases are workspace-scoped and begin in
shadow mode, allowing the eight-week eligibility history to mature before live
feed activation.
A workspace anomaly guard suppresses
a decline batch only with at least 100 eligible accounts and a 35% trip rate;
smaller workspaces rely on the five-of-seven rule. Suppressions are persisted
with their reason for auditability.

## Operations and migrations

Postgres signal models participate in API startup migration, with additional
idempotent schema work in `MigrateCRMSignalSchema`. Versioned Postgres SQL lives
under `server/internal/dbmigrate/sql`.

The motion-spine migration intentionally removes legacy signals, feedback, and
deliveries before adding the non-null interpretation contract. Suggestions and
external-evidence links are detached first. There is no replay or archive: the
new corpus starts from evidence detected under the motion-aware contract, as a
deliberate clean cutover.

Run both Postgres migrations before deploying the API. The first migration is
destructive by design for the legacy signal corpus and should be treated as the
cutover boundary; it does not delete CRM contacts, companies, deals, source
events, suggestions, or external evidence.

ClickHouse schema is versioned under `server/internal/chmigrate/sql`. Local
event-stack startup runs the ClickHouse migration runner; stage and production
run the same migrations before event consumers are deployed. The backend needs
`CLICKHOUSE_DSN` to evaluate behavioral rules. Without it, conversation and
daily Postgres rules continue to work, while the behavioral cadence is skipped.

The signal evaluator runs immediately on API startup and then on its normal
cadence. Commercial state runs every ten minutes under a watermark, usage
baselines run daily under a separate lease, routing runs at startup and every
ten minutes, and deal-health snapshots run at startup and every six hours.

### CRM AI model routing

Conversation signal extraction, CRM summaries, and deal-automation inference
use a deployment-owned primary/fallback route. Meeting intelligence shares the
primary route and has its own fallback. With no overrides configured, the
reviewed defaults are:

| Route | Provider and model |
|---|---|
| CRM primary | `openrouter` / `deepseek/deepseek-v4-flash-0731` |
| CRM fallback | `openrouter` / `openai/gpt-5.6-luna` |
| Meeting fallback | `openrouter` / `google/gemini-3.7-flash` |

Optional deployment overrides:

```env
CRM_LLM_PROVIDER=
CRM_LLM_MODEL=
CRM_LLM_OPENROUTER_PROVIDER=

CRM_LLM_FALLBACK_PROVIDER=
CRM_LLM_FALLBACK_MODEL=
CRM_LLM_FALLBACK_OPENROUTER_PROVIDER=

CRM_MEETING_LLM_FALLBACK_PROVIDER=
CRM_MEETING_LLM_FALLBACK_MODEL=
CRM_MEETING_LLM_FALLBACK_OPENROUTER_PROVIDER=
```

Provider/model variables must be set as pairs. Models must have an enabled
pricing-catalog route and the corresponding provider credentials must be
configured. For an OpenRouter route, the `*_OPENROUTER_PROVIDER` value pins one
underlying infrastructure provider by sending `order: [value]` with
`allow_fallbacks: false`; leave it empty to let OpenRouter choose. API and
Temporal worker validate and use the same route registry.

Provider credentials remain global rather than CRM-specific:

```env
OPENROUTER_API_KEY=
OPENROUTER_BASE_URL=https://openrouter.ai/api/v1
ANTHROPIC_API_KEY=
OPENAI_API_KEY=
OPENAI_BASE_URL=
```

## Local verification

Start and verify the event stack from the repository root:

```bash
cp events-pipeline/.env.events.example events-pipeline/.env.events
just events-up
just events-smoke
just events-browser-smoke
```

The smoke test verifies the authorized project at each pipeline stage and the
browser test generates representative page, form, article, interaction, and
product events.

For a behavioral signal test, also ensure:

1. the event's canonical `project_id` belongs to the test workspace;
2. its external user or company ID resolves to a CRM contact or company;
3. the event satisfies the selected rule threshold and trust requirement;
4. the API has `CLICKHOUSE_DSN` configured; and
5. the ten-minute evaluator has run.

Then inspect `crm_signal_evaluation_runs` and `crm_buyer_signals` in Postgres or
open CRM Insights. A stored signal is expected to remain activation-blocked
while its rule version is in shadow mode.

Signal-specific backend tests:

```bash
cd server
go test ./internal/service ./internal/repository ./internal/handler
```

See [`../events-pipeline/README.md`](../events-pipeline/README.md) for event
pipeline operations and migration commands.

## Implementation map

| Area | Primary location |
|---|---|
| models and API DTOs | `server/internal/model/crm_signal*.go` |
| conversation extraction | `server/internal/service/crm_signal_detection.go` |
| deterministic evaluator | `server/internal/service/crm_signal_rule_evaluator.go` |
| Postgres rules | `server/internal/repository/crm_signal_rule_postgres.go` |
| behavioral rules | `server/internal/repository/event_clickhouse_rules.go` |
| scoring and composition | `server/internal/service/crm_signal_score.go` |
| activation and feedback | `server/internal/service/crm_signal_activation.go` |
| API handlers and routes | `server/internal/handler/crm_signal.go`, `server/internal/router/router.go` |
| frontend feed | `frontend/src/components/crm/SignalWorkspaceFeed.tsx` |
| entity signal panels | `frontend/src/components/crm/BuyerSignals.tsx` |
| SDK capture | `packages/sdk-js/src/core` |
| event pipeline | `events-pipeline/` |

## Related documents

- [`crm-buyer-signal-ingestion.md`](crm-buyer-signal-ingestion.md) — email and
  conversation ingestion details.
- [`crm-entity-summaries.md`](crm-entity-summaries.md) — downstream summary
  refresh and provenance.
- [`crm-email-sync.md`](crm-email-sync.md) — normalized CRM email source.
- [`CRM_MODULE.md`](CRM_MODULE.md) — broader CRM architecture.
- [`crm-buyer-signals-assessment.md`](crm-buyer-signals-assessment.md) —
  historical assessment and design rationale; not current status.
