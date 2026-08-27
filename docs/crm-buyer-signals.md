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

- **signal type** — the stable seven-value CRM taxonomy;
- **detector** — LLM extraction or a deterministic versioned rule;
- **domain and polarity** — where evidence came from and which direction it
  moves the account;
- **confidence** — whether extracted evidence is supported;
- **business priority** — how important the evidence is now;
- **activation** — whether this exact rule version may create downstream work.

## Architecture

```text
connected email, calendar, support, PM, and CRM records
  -> LLM extraction or daily deterministic rules
                                                    \
browser and authenticated product events             -> durable Postgres signal
  -> authenticated event pipeline                     -> scoring and composition
  -> ClickHouse usermaven.events                       -> CRM feeds and briefs
  -> ten-minute deterministic behavioral rules        -> controlled activation

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
| `detector_kind` | `llm_extracted` or `rule_derived` |
| `signal_domain` | conversation, web behavior, product usage, support, delivery, relationship, or market |
| `polarity` | positive, negative, or neutral |
| `rule_key`, `rule_version` | immutable detector identity and semantics |
| evidence window | bounded period evaluated by a rule |
| identity method and trust | provenance used by activation gates |
| evidence fingerprint | idempotency and repeat-dismissal suppression |
| feedback state | reviewed, dismissed with reason, or acted |

The model lives in `server/internal/model/crm_signal*.go`. Schema changes are in
`server/internal/dbmigrate/sql/20260824*_crm_signal_*.sql`.

## Signal producers

### Conversation extraction

Stored email, calendar, note, call, and supported conversation sources can
enqueue the Temporal `SignalDetectionWorkflow`. The LLM returns structured
signal candidates, but persistence verifies that quoted evidence literally
exists in normalized source text. Deterministic workflow IDs, source-level
uniqueness, thread suppression, and evidence fingerprints prevent duplicate
signals.

See [`crm-buyer-signal-ingestion.md`](crm-buyer-signal-ingestion.md) for the
email ingestion details.

### Daily first-party rules

The daily evaluator derives signals from trusted Postgres relationships:

| Rule key | Evidence |
|---|---|
| `support_volume_spike` | recent support volume versus account baseline |
| `urgent_issue_open_deal` | high-priority support issue on an active deal |
| `support_ai_escalation` | support conversation escalated to a human |
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
| `session_depth_spike` | context only |
| `new_account_stakeholder` | context only |
| `anonymous_account_traffic` | context only |
| `campaign_attributed_return` | context only |
| `pre_identification_history` | context only |
| `identified_article_view` | context only |
| `versioned_interaction` | disabled and context only by default |

An event reaching ClickHouse does not guarantee a CRM signal. Before a
behavioral candidate is stored, the evaluator resolves its external identity to
a CRM contact, company, or deal. Candidates with no CRM entity are discarded;
the procurement rule additionally requires an open deal.

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
rows make routing idempotent across replicas and restarts. The
`buying_signal_to_task` automation template requests only activation-approved
signals and remains approval-gated.

Users can mark a signal reviewed, acted, or dismissed with one of the supported
reasons. Precision reports retain rule version, trust, source, domain, and
detection-to-feedback timing so promotion decisions can be evidence-based. A
signal contributes once using its latest feedback outcome, so review followed
by action does not inflate the precision denominator.

## Product and API surfaces

Signals appear in CRM Insights and on contact, company, and deal pages. The
workspace feed uses the shared query builder for owner, account, domain,
polarity, signal type, detection date, and identity trust, with additional
computed severity, age, and status controls. Account and meeting briefs group
corroborating evidence and explain what changed.

Primary routes under `/api/crm`:

| Method | Route | Purpose |
|---|---|---|
| GET | `/signals/feed` | ranked, grouped workspace feed |
| GET | `/signals` | paginated signal list |
| GET | `/signals/brief` | current account/deal signal brief |
| GET | `/meetings/{id}/signal-brief` | meeting-specific brief |
| GET | `/signals/precision` | rule feedback and precision report |
| GET/POST | `/signals/routing-policy` | inspect or create a policy version |
| POST | `/signals/rules/{ruleKey}/versions/{version}/activate` | activate a rule version |
| POST | `/signals/{id}/review` | record review |
| POST | `/signals/{id}/dismiss` | dismiss with a reason |
| POST | `/signals/{id}/acted` | record action |
| POST | `/signals/external-evidence` | ingest normalized provider evidence |

All routes are workspace-scoped and protected by CRM RBAC. Routing policy,
rule promotion, and external-evidence ingestion routes require CRM admin
permission.

## Operations and migrations

Postgres signal models participate in API startup migration, with additional
idempotent schema work in `MigrateCRMSignalSchema`. Versioned Postgres SQL lives
under `server/internal/dbmigrate/sql`.

ClickHouse schema is versioned under `server/internal/chmigrate/sql`. Local
event-stack startup runs the ClickHouse migration runner; stage and production
run the same migrations before event consumers are deployed. The backend needs
`CLICKHOUSE_DSN` to evaluate behavioral rules. Without it, conversation and
daily Postgres rules continue to work, while the behavioral cadence is skipped.

The signal evaluator runs immediately on API startup and then on its normal
cadence. Routing runs at startup and every ten minutes. Deal-health snapshots
run at startup and every six hours.

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
