# CRM Signals

This is the canonical engineering and operations reference for Helpin CRM
signals. It describes the implemented system. Historical design decisions and
the original phase plan remain in
[`crm-buyer-signals-assessment.md`](crm-buyer-signals-assessment.md), but that
assessment is not a statement of current implementation status.

## Purpose

CRM signals turn evidence already owned by Helpin into explainable CRM
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
for all event-enabled workspaces regardless of rollout mode, so history is warm
whenever a workspace is looked at. Workspace rollout and per-rule
shadow/activation policy gate only user-visible feeds and downstream actions.

Workspaces are **live by default**. The legacy signal corpus was deleted at the
motion-spine migration and the legacy feed was not in production use, so there
is no baseline to shadow against. `shadow` remains available as an explicit
per-workspace opt-out.

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
ClickHouse evidence and write durable `crm_signals` rows to Postgres.

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

See [`crm-signal-ingestion.md`](crm-signal-ingestion.md) for the
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
creates one `crm_signals` row per applicable interpretation. No mapping
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

Promotion also requires at least one enabled interpretation for that exact
`(rule_key, rule_version)`, visible to the workspace. Interpretation lookup
matches the version exactly, so promoting an unmapped version would leave the
rule producing observations and no signals. Activation is refused instead.
Publish the interpretation rows for a new rule version before activating it.

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

### Release readiness gate

A workspace is considered release-ready when it has at least 100 observations,
an unmapped-observation rate below 5%, a duplicate fingerprint-and-motion rate
below 1%, and zero immutable-meaning violations. The last condition is a hard
test invariant: two rescores may change priority, never stored motion, type,
polarity, or meaning fingerprint. `GET /signals/shadow-gate` exposes the gate.

The gate is diagnostic rather than blocking. Workspaces start live, so
`POST /signals/rollout/activate` is the recovery path out of an explicit shadow
opt-out; it records a warning when the thresholds are not met instead of
refusing, so activating below the bar stays visible in logs.

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
| GET | `/signals/shadow-gate` | objective release-readiness gate (diagnostic) |
| GET | `/signals/rollout` | current workspace shadow/live state; live by default |
| POST | `/signals/rollout/activate` | return a workspace to live after a shadow opt-out |
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

### Customer-work foundation

The [customer-work blueprint](crm-customer-work-blueprint.md) owns the agreed
complete target experience; the [automation connection plan](crm-playbook-automation-change-proposal.md)
owns the planned Flow/Beacon/skill extension. This reference describes implemented
behavior in the inspected branch, not production deployment. The additive foundation stores independent
customer situations in `crm_situations`, with source links in
`crm_situation_references`. Source adapters now reconcile eligible signals and
unresolved Review suggestions, using `crm_situation_source_links` for stable
source-to-work mappings. The branch now has the unified Signals inbox and a
compatible Review redirect, described under [Review consolidation](#review-consolidation).
This does not enable Playbook execution or new outbound automation.

| Method | Route under `/api/crm` | Permission and behavior |
|---|---|---|
| GET | `/situations` | `crm.read`; filtered, ranked situations with full category counts |
| GET | `/situations/{id}` | `crm.read`; situation, current identity labels, and available source links |
| POST | `/situations` | `crm.edit`; manual creation only, no execution or approval |
| POST | `/situations/{id}/commands` | `crm.edit`; revision-checked update, pause, resume, or explicit close |
| GET | `/situations/{id}/history` | `crm.read`; newest-first lifecycle history and actor attribution |
| POST | `/situations/{id}/actions/{action_id}/{decision}` | `crm.edit`; accept/dismiss one linked suggestion revision |

All routes also require active workspace membership and CRM module access.
Manual creation requires `creation_key`, `title`, `objective`, an original
`commercial_motion`, and at least one permitted `company_id`, `contact_id`, or
`deal_id`. A stable creation key identifies a request within its workspace:
identical normalized retries return the existing record (200 instead of 201);
different intent under the same key returns 409. Work is never merged merely
because it shares a customer, motion, title, or evidence.

`owner_mode` is `routing` (default), `member`, or `unassigned`. Routing snapshots
the existing motion-aware owner policy at creation; later reads and retries do
not reassign stored work. Explicit owner and next-action owner IDs must identify
active members of the same workspace. Inactive or deleted owners remain visible
as responsibility gaps. Manual creation begins `open` / `needs_context`; clients
cannot write lifecycle, approval, execution, or outcome state through this API.
Optional source references accept only existing workspace-local `signal` and
`suggestion` IDs. Source payloads and action state remain in their owning systems.
Reading a situation does not mark evidence reviewed; linking a suggestion does
not accept or execute it. Deleted/foreign sources are omitted from disclosure
without deleting the customer situation.

The list accepts `scope` (`mine`, `my_teams`, `unassigned`, `all`), `state`
(`needs_attention`, `waiting`, `open`, `paused`, `closed`, `all`), `category`, `q`,
the shared JSON `filter`, `page`, and `page_size` (maximum 100). Defaults are
Mine + Needs attention, all categories, page 1, size 25. An owned situation whose
next action belongs solely to an active colleague is Waiting in Mine. Team scope
uses current workspace memberships, not client-supplied team IDs.
Open actions assigned to a different member also participate in Mine/My teams;
they are not hidden behind the primary next-action owner. Detail responses include
all linked canonical actions, their resolved assignees, and revision tokens.

Categories are Sales (`prospecting`, `conversion`), Onboarding & adoption
(`onboarding`, `adoption`), Expansion (`expansion`), and Retention (`renewal`,
`retention`); the original motions remain independently filterable. Category
keys are `sales`, `onboarding_adoption`, `expansion`, and `retention`, with `all`
as the unfiltered view. `category_counts` honors every filter except category;
`total` is the selected category's count before pagination. Counts and rows use
one read snapshot. Ranking preserves fractional business priority, then uses
creation time and ID for stable ties.

Standalone suggestions without enough commercial context remain in All with
`commercial_motion=needs_context`, reported in `uncategorized_count`; the adapter
does not invent a customer, human creator, or fifth category tab. Explicit deal
motion and unambiguous supporting signal motion take precedence over type-based
fallbacks. `pending_action_total` counts distinct pending suggestions across the
entire filtered result, including the selected category, before pagination.

`effective_attention` and per-row action counts are read projections from current
suggestions and stored checkpoints, not copied decision state. Pending proposals,
manual follow-through, confirmed failures, and unconfirmed execution remain
distinct. An overdue checkpoint becomes follow-up due without a page-opening
mutation. Accepted `pending` executions and stale `in_progress` executions appear
as uncertain work requiring review; the adapter never blindly retries them.
Historical accepted records with missing execution status are also uncertain,
not assumed successful or omitted from reconciliation.

### Customer-work controls and history

Situation `revision` starts at 1 and advances for each committed lifecycle
command. A command requires a stable `command_key`, the displayed
`expected_revision`, and one `operation`. Its state change and append-only
`crm_situation_changes` receipt commit atomically. Conflicting concurrent edits
return 409; a failed history write rolls back the state change. Identical retries
by the same member return the original receipt with `replayed=true`, even after
newer edits. Different intent or a different member under that key returns 409.
Refetch detail for current state: a replay receipt describes the original change,
not a new update. Source priority rescoring does not overwrite controlled fields
or advance this work revision. Canonical proposals retain their own revisions.

Operations:

- `update` accepts partial `changes`: `owner`, `next_action_owner`, `next_step`,
  `attention`, and `checkpoint`. Omitted fields retain their values. Owners use
  `{member_id: "workspace-member-uuid"}`; `{member_id: null}` explicitly unassigns.
  Checkpoints use `{at: "RFC3339 timestamp"}` or `{at: null}` to clear. Assignment
  changes never change the account owner or silently move an existing commitment
  or proposal to the new situation owner. New assignees must be active members of
  this workspace; old inactive owners do not prevent someone from repairing work.
- Manual attention accepts `needs_context`, `follow_up_due`, `waiting_customer`,
  or `waiting_work`. Approval/failure attention remains derived from canonical
  actions and cannot be fabricated by a work update. Open waits require a next
  step, a checkpoint, and an active responsible member (next-action owner, falling
  back to situation owner only when no next-action owner is assigned). A past
  checkpoint is valid and surfaces as due; there is no read-time mutation.
- `pause` requires a reason and an open situation. It retains the commitment and
  deadline, allowing repair while paused. `resume` requires a paused situation and
  revalidates any waiting commitment; it does not send, retry or launch anything.
- `close` requires an outcome `{kind, summary}`: `achieved`, `not_pursued`,
  `invalid`, or `duplicate`. The server records `outcome_basis=human_assessment`,
  the authenticated member and closure time, and clears the active checkpoint.
  A duplicate additionally requires `duplicate_of_situation_id`: a different,
  workspace-local situation that is not itself closed as a duplicate. Neither
  closure nor duplication changes another situation, evidence feedback, pending
  proposals, or executor results. Closed work is read-only; source replay cannot
  reopen it or revise its outcome.

Example ownership and commitment update:

```json
{
  "command_key": "commitment-change-unique-request-id",
  "expected_revision": 3,
  "operation": "update",
  "changes": {
    "next_step": "Confirm the kickoff date with the customer",
    "attention": "waiting_customer",
    "checkpoint": {"at": "2026-09-10T09:00:00Z"}
  }
}
```

Lifecycle commands, source enrollment and legacy suggestion execution admission
share a short database lock. A pause/close committed first prevents a new claim;
an action admitted first remains visible in the pause receipt's
`in_flight_action_count`. No external execution runs under this lock. Pause does
not pretend to cancel an already-admitted or irreversible action. Closing is
refused while a linked accepted action is running or unconfirmed, including
historical missing execution status. Its actual result must be reconciled first;
human assessment cannot stand in for execution confirmation.

History accepts `limit` (1–100, default 50) and `before_revision` for keyset
pagination. Each change records operation, actor, reason, revision, before/after
controlled facts, and timestamp. New manual creation is attributed to its member;
new source creation is attributed to `signal` or `suggestion`, without inventing a
human creator. Retries do not duplicate history. Existing pre-migration records
gain no fabricated historical events; their next command starts history with its
observed before-state. No public history edit/delete operation exists.

Migration `202609060002_crm_situation_lifecycle.sql` adds the revision, outcome
provenance, duplicate link, and history table. These remain versioned-SQL-owned,
not AutoMigrate-owned. Checkpoint delivery now uses the shared Automation outbox
described below. Flow/Agent adapters, cancellation of actual executions, and
version-bound continuation remain required for the complete release; cancelling
a checkpoint does not claim that a running Agent or outbound action was cancelled.

### Durable checkpoint delivery

Migration `202609060003_automation_scheduled_events.sql` adds a shared Automation
event outbox, not a CRM-specific runtime. Creating or changing a Signal commits
its lifecycle history and checkpoint schedule in one transaction. The durable
identity is workspace + Signal ID + lifecycle revision. Replaying a command does
not schedule another check. Changing the commitment revokes pending/claimed checks
for older revisions; pause, close, or clearing the checkpoint leaves no active
check. Resume schedules against the new revision, preserving the agreed due time.
The migration recovers only open records with an explicit checkpoint and does not
reset existing delivery receipts or replay historical evidence.

The namespace-wide `automation-scheduled-events` Temporal workflow runs on the
existing Automation queue once per minute. API and worker startup both ensure its
stable identity without changing user Flow schedules. Each activity drains up to
100 events within 40 seconds. PostgreSQL workers claim with `SKIP LOCKED`, a
two-minute lease, and a fresh claim token. Expired claims can be recovered, but
old workers cannot acknowledge or retry a replacement claim. Delivery gets at
most five attempts, with persisted exponential backoff for reported failures;
unknown event kinds fail closed. Tick timing is approximate and can lag under
backlog or outages; persisted due times remain authoritative. Workers require
Temporal and the versioned migration to be available.

The `crm.checkpoint_due` consumer takes the same workspace lock as lifecycle and
approval admission, then rechecks the canonical event, open lifecycle, revision,
due time, active owner, and current linked actions. Stale events are acknowledged
without evaluating changed work. Successful checks record a receipt such as
`needs_owner`, `needs_approval`, `waiting_work`, or `follow_up_due`; they do not
create an action, send a message, launch/resume an Agent, create a PM task, or
close the customer objective. Receipt and reevaluation commit atomically.

List/detail contracts expose the current revision's `checkpoint_status`,
`checkpoint_result`, `checkpoint_attempts`, and `checkpoint_completed_at` when
present. These are diagnostic fields, not four additional main-table columns.
A delivery failure appears in the existing Needs attention projection. An
unavailable commitment owner also surfaces there and in Unassigned, using live
membership rather than a potentially stale receipt. Assignment or checkpoint
changes establish a new revision; old delivery failures remain stored but do not
override the new commitment. Reading the page does not run or reschedule checks.

This consumer remains attention-only. Separate Playbook entry/check consumers on
the same shared worker provide guarded normal Agent Runtime dispatch, pinned
configuration, exact action authorization and recovery as documented below.

### Source reconciliation and decision safety

The existing routing sweep also reconciles customer work on startup and every
ten minutes, with bounded workspace contexts and keyset pagination. Suggestions
created through `CRMSuggestionService` project immediately; the sweep repairs
transient projection failures and covers other producers. Each source import is
transactional and replica-safe. Only an explicit, workspace-local open
`context.situation_id` on a suggestion consolidates work; shared evidence or a
shared customer alone does not. Replays preserve assignment and customer outcomes.

Signal qualification reuses current workspace rollout, exact rule version,
trust, priority, and routing-policy gates. Recording a situation is independent
of notification delivery and unrelated existing PM tasks. This separation grants
no execution authority and does not weaken existing delivery/automation gates.
Dismissed, superseded, already-acted, and context-only signals are not enrolled.
Historical successful/dismissed suggestions are not reopened by reconciliation.

The action endpoint requires `{revision, edits?}` for `accept`, or
`{revision, reason}` for `dismiss`. It delegates to the existing suggestion service,
checks the exact linked proposal, rejects stale revisions, and does not decide
other pending actions. Legacy approval routes also honor linked situation pauses
and closure. Missing/dismissed/superseded supporting evidence blocks acceptance.

Deal creation and stage changes retain their real executors. Follow-up,
enrichment, risk, and other manual recommendations are accepted as
`execution_status=manual_required`, with no fabricated `executed_at`. Executor
claims persist `in_progress` before attempting work; confirmed failure/success
remain separate from approval. Legacy status-only updates remain decisions, not
executions, and cannot reset accepted work to pending or overwrite a concurrent
decision. No action result automatically closes a customer situation.

Shared Flow/Agent process orchestration, durable continuations and recovery
controls, and the unified Signals/Playbooks UI remain required before
the complete blueprint can ship. These backend integration tests are not a claim
that the complete automated customer journeys have shipped.

### Playbook configuration and manual participation

CRM owns typed policy, draft/publication history, read-only eligibility preview,
explicit participation and human-assessed progress. These business-policy APIs do
not launch work. Their `execution_enabled: false` fields are publication/preview
contracts, not the live automation gate.

The connected execution path is implemented separately: Playbooks define the
customer process, a managed Flow wakes the existing CRM Agent Beacon with captured
job skills, and canonical CRM actions require exact human approval. Guided setup,
activation, durable dispatch, live guards and recovery are described below and in
the [connection plan](crm-playbook-automation-change-proposal.md). Ordinary saved
Flows/Agents are preserved; there is no ordered-step builder or new saved Agent per
customer/Playbook.

Migration `202609070001_crm_playbooks.sql` owns `crm_playbooks`, immutable
`crm_playbook_versions`, and append-only `crm_playbook_changes`. It adds nullable
Playbook/version/progress fields directly to `crm_situations`; there is no second
customer process or enrollment lifecycle table. No existing Signal is enrolled,
backfilled, reassigned, or closed by this migration. Composite foreign keys keep
each published/participating version in the correct workspace and Playbook.
These tables/columns are versioned-SQL-owned, not AutoMigrate-owned.

Configuration uses existing permissions: `crm.read` for discovery, lists, detail,
preview and history; `crm.admin` for draft changes, publication and the admission
gate; `crm.edit` for explicitly applying a published policy and recording progress.
The current built-in role matrix grants `crm.admin` to workspace admins/owners;
this increment does not introduce a new manager role or broaden CRM permissions.
Handler and service boundaries enforce the same tenant and permission checks.

| Endpoint under `/api/crm` | Contract |
| --- | --- |
| `GET /playbooks/templates` | Three independent draft defaults; no installation or writes. |
| `GET /playbooks` | Server-paginated definitions and canonical open/paused/closed Signal counts. |
| `POST /playbooks` | Create a draft using a stable `creation_key`; enrollment starts disabled. |
| `GET /playbooks/{id}` | Current draft, published definition, admission gate, counts and execution availability. |
| `POST /playbooks/{id}/commands` | `update_draft`, `publish`, or `set_enrollment`, bound to `expected_revision` and a stable `command_key`. |
| `GET /playbooks/{id}/history` | Immutable command receipts, newest first; `before` revision cursor and `page_size`. |
| `GET /playbooks/{id}/versions` | Immutable published definitions, newest first; `before` version cursor and `page_size`. |
| `GET /playbooks/{id}/preview` | Exact `revision` required; optional `version_id` selects the current published policy instead of the draft. |
| `GET /playbooks/{id}/automation/preview` | Read-only Beacon skill selection for the exact `revision` and optional current published `version_id`; never a runnable connection. |
| `GET /playbooks/{id}/participants` | Canonical Signal list, including existing scope/state/category/query-builder filters and complete counts. |
| `POST /playbooks/{id}/apply` | Explicitly confirm a published version on one eligible, open Signal. |
| `POST /playbooks/{id}/participants/{situation_id}/milestones` | Record a human milestone assessment with an observed Signal revision and command key. |

Playbook list filters are `q`, `state` (`all`, `draft`, `accepting`, `stopped`),
`page`, and `page_size`. Page size defaults to 25 and is capped at 100. A published
Playbook with later draft edits remains published; `draft` here means never
published, not a separate lifecycle for participating customers. Participant lists
default to all responsibilities and all lifecycle states within that Playbook.
Counts aggregate before pagination; independent objectives for one customer
remain separate rows and are not mislabeled as distinct customer counts.

**Definition.** Policies contain name, description, journey, objective, eligible
commercial motions, optional shared query-builder rules, business responsibilities,
observable milestones, approval requirements, follow-up/escalation timing, and
stop conditions. The three defaults cover buying intent, sales-to-success handoff,
and renewal recovery; a `custom` journey can represent other configured CRM work.
Defaults are configuration starters, not installed/working automation templates.
Selecting a journey does not invent a won deal, qualified source event, or trigger.

Eligibility supports `company_id`, `contact_id`, `deal_id`, `pipeline_id`,
`stage_id`, `company_domain`, `owner_member_id`, and `created_at` through the shared
query builder. Definitions are capped at 30 rules and 100 total values; referenced
records/stages must belong to the workspace and explicit members must be active.
Existing shared CRM pipeline and ownership settings are referenced, not copied.

Incomplete drafts can omit an objective, milestones, success criteria or escalation
owner. Publishing requires those details and an active workspace escalation member.
Milestone keys are stable and unique within a version. Default outbound/CRM policy
is `approval_required`; PM tasks default to `not_allowed`. Both modes express
requirements for eventual Playbook-driven execution, not changes to a user's
ordinary manual CRM permissions. Unsupported automatic-action settings are rejected
until the shared execution contract is implemented; no confidence-based bypass is
introduced. Follow-up and escalation hours are stored business policy, not new
timers created by publishing or attaching a definition.

**Publication and admission.** Every command uses optimistic revision checks and
an actor-bound request fingerprint. Identical retries return their original receipt
even after newer changes; changed intent under the same key is rejected. Publication
creates a frozen version and updates only the Playbook's current-version pointer.
It does not enable enrollment. `set_enrollment` with `accepting_customers: true`
allows new explicit applications of the current published version; it installs no
event subscriber or automatic enrollment sweep. Setting it false affects new
applications only, not active customers or existing checkpoints. Publication,
version creation and audit receipt commit together or all roll back.

**Preview.** Preview is read-only and explicitly scoped as `existing_signals`:
open, not-already-bound Signals with a currently available linked CRM record,
matching motions and optional filters. It is not a full-workspace customer scan,
trigger qualification check, or simulation of outgoing actions. Its `version_id`
is null for a draft preview or identifies the current published policy selected by
the caller. A changed draft/publication revision rejects stale preview requests.
Future automatic enrollment must independently preserve source trust, activation,
rollout and authorization gates rather than treating this manual preview as authority.

**Participation.** Apply requires `confirmed: true`, `situation_id`, `version_id`,
`expected_playbook_revision`, `expected_situation_revision`, and `command_key`.
The transaction rechecks admission, publication identity, current eligibility,
open lifecycle and referenced members under the same workspace lock as other
Signal commands. It pins the published version and initializes pending milestones
on that Signal. Existing owner, next-action owner, objective, next step, checkpoint,
evidence and decisions remain intact; role defaults do not silently reassign
already-existing commitments. One Signal has one pinned Playbook; another customer
objective can independently use the same or a different Playbook. There is no
silent replacement or active-version migration endpoint.

Milestone assessments accept `pending`, `achieved`, or `not_applicable`, require a
summary, and record the authenticated member, timestamp and `human_assessment`
basis. The milestone must exist in the participating version, not a later draft.
Progress changes are blocked on paused/closed Signals. They do not approve an
action or close the customer objective. Pause/resume/reassignment/closure remain
the existing Signal commands, with their original safeguards and outcome semantics.
Applying a Playbook and changing progress append `apply_playbook` / `update_milestone`
to the same Signal history, increment its revision, and atomically replace any
older pending checkpoint. Publishing a new policy never rewrites those snapshots.

### Beacon skill-selection preview

`GET /crm/playbooks/{id}/automation/preview?revision=:revision` requires `crm.read`
and active workspace membership/module access, like other Playbook reads. Omit
`version_id` for the draft, or supply the current published version ID. Stale
policy revisions or mismatched published versions return 409. Draft changes never
silently replace the published journey used by a version-specific preview.

The response includes policy identity/scope, journey, Beacon preset key, selected
skill titles/keys/roles/content versions, specialization version, explanatory
status and `execution_enabled: false`. Supported journeys report `not_connected`;
custom journeys report `unsupported_journey` and remain usable manually. No
matching workspace Agent/Flow is inferred or created, and no package contents,
prompts or saved Agent configuration are returned.

The [specialization compiler](../server/internal/agentcontract/crm_playbook_skills.go)
selects the existing core skill and exactly one of the three Playbook-only jobs.
It captures complete skill packages, including reference files, separately from
the ordinary Agent catalogue. Digests identify captured content; they are not
authorization or evidence that the full Flow/Agent execution connection has been
published. Neither this preview nor normal Playbook publication persists that
connection, assigns these skills to saved Agents, or starts a run.

The [context preparer](../server/internal/agentcontract/crm_playbook_context.go)
validates already-authorized policy/Signal/skill references and copies minimal CRM
context into the optional shared run-input contract. It rejects stale or foreign
work, mismatched CRM targets, paused/closed Signals and unavailable owners, without
changing progress or rescheduling checks. The bound launcher consumes this path
after a durable wake-up claim and live authorization. The catalogue-only preview
does not inspect live connections; use the automation overview for current status.

### Approved automation settings

These endpoints require active membership, `crm.admin`, `pm.admin.automations`,
and access to both CRM and Automation. They are separate from the catalogue-only
skill preview above; its `not_connected` status describes that preview's lack of
execution binding, not publication history.

| Endpoint | Contract |
| --- | --- |
| `POST /crm/playbooks/{id}/automation/connection/preview` | Read-only review of `playbook_version_id`, `expected_revision`, `flow_id`, `agent_id`. Returns selected names/skills, `connection_version`, a content fingerprint and execution-disabled explanation. |
| `POST /crm/playbooks/{id}/automation/connections` | Adds `command_key`, `expected_connection_version` and `review_fingerprint` to the selection. Returns 201 for an immutable publication or 200 for its identical retry. No client-supplied prompt, snapshot or enable flag is accepted. |
| `GET /crm/playbooks/{id}/automation/connections` | Newest-first receipt metadata, paginated with `before` and `page_size`; `next_before_version` continues history. |
| `GET /crm/playbooks/{id}/automation/connections/{connection_id}` | Exact tenant-local historical receipt, not a replacement with today's settings. |

Migration `202609070002` creates `crm_playbook_connections` only. It freezes the
immutable policy reference/hash, selected Flow configuration, effective saved
Beacon prompt/runtime projection, host limits/access selectors and full skill
packages. Database constraints prohibit execution enablement. No existing policy,
Flow/Agent configuration, enrollment or customer work is updated. Active Signals
do not adopt a new connection implicitly.

Publication rechecks source settings under row locks and validates both expected
versions and the reviewed fingerprint. Command receipts are actor-bound and checked
before freshness checks, so a retry returns its original publication even after
later edits. Mismatched/reused intent and stale reviews return 409; missing or
foreign selected records return 404 without exposing their settings. Raw prompts,
packages, runtime configuration and internal receipt fingerprints stay server-side.

Legacy fixed-target connection review remains non-executing and does not turn an
ordinary Flow into Playbook authority. Guided setup creates/reuses a dedicated,
disabled `crm.playbook.work_due` Flow with a dynamic CRM target resolved from the
canonical Signal. Schema-2 snapshots are required for activation. Repository/document
output branches and uncaptured optional runtime skills are rejected. Handoff
automation requires its receiving-owner acceptance milestone; custom journeys
remain manually usable.

Frozen runtime profiles have a deterministic private identity per publication;
the existing Helpin Beacon ID remains authoritative for access, billing and
Activity. The bound launcher registers that profile through the existing Agent
Runtime and serves exact captured package bytes through guarded host routes.
Ordinary run/approval/continuation paths cannot populate or reuse this authority.
Host/app/profile/target identity, current workspace permissions/modules/billing,
policy, generation and Signal revision are rechecked; metadata is never authority.

### Live automation and exact action APIs

All routes are workspace-scoped. CRM reads require active membership, CRM module
access and `crm.read`. Setup/configuration requires `crm.admin`,
`pm.admin.automations`, both modules and applicable Automation billing features.
Signal start/pause and action review require `crm.edit`, not Automation-builder
access. Every run also rechecks the live configuration authorizer's authority.

| Endpoint | Contract |
| --- | --- |
| `GET /crm/playbooks/{id}/automation` | Live settings, latest reviewed connection metadata, safe Agent/Flow names and runtime availability. Settings retain the active connection ID; merely reading a newer publication does not adopt it. |
| `POST /crm/playbooks/{id}/automation/setup` | Exact published policy ID/revision. Prepares a disabled dedicated Flow using saved Beacon, then returns review metadata. Does not publish, enable, enroll or launch. |
| `POST /crm/playbooks/{id}/automation/settings` | Actor-bound command key, expected settings revision, reviewed connection, explicit confirmation, manual/automatic entry and bounded usage limits. Activation never starts historical/paused Signals. |
| `GET /crm/situations/{id}/automation` | Nullable per-Signal binding, safe blocker, generation, last check and escalation time. |
| `POST /crm/situations/{id}/automation` | Exact Signal revision, binding generation, connection ID and confirmed start/pause. Existing bindings resume their pinned configuration. |
| `GET /crm/playbook-actions/{action_id}` | Canonical intent, designated approver, expiry and confirmed destination identity; no runtime prompts or credentials. |
| `POST /crm/situations/{id}/actions/{action_id}/accept` | Existing revision-aware decision with optional exact `edits.playbook_action`. Wrong approver, changed facts, stale revision, expiry or revoked authority cannot execute. |
| `POST /crm/playbook-actions/{action_id}/reconcile` | Read-only provider/local receipt inspection by the original approver. Never sends/creates/retries; absence is inconclusive. |
| `POST /crm/playbook-actions/{action_id}/inspect` | After five minutes, the original active approver records exact action revision, confirmed completed/not-completed finding and evidence. Human attribution is distinct from provider verification. |
| `GET /crm/playbooks/{id}/automation/activity?page=1` | Paginated safe projection of connection publications, gate/adoption receipts and normal Agent runs. |
| `GET /crm/playbooks/agent-usage/{agentID}` | CRM-readable reviewed Playbook dependencies for Beacon; no prompt/package exposure. |

Migrations `202609080001`–`202609080003` own live gates, normal-run bindings,
action intents and confirmed-won provenance. Customer progress stays exclusively
on canonical situations/milestones; approval/execution status stays exclusively
on existing suggestions. New typed actions are `email`, `task`, `handoff`,
`milestone` and `deal_stage`. Milestones/outcomes are human assessments, never
inferred from sends, tasks or completed runs.

Fresh qualified evidence and a real newly won-deal transition enter through shared
eligibility. Both detection and creation must follow the automatic activation
boundary; no startup/enabling backfill occurs. Multiple matches and missing role
owners stay for review. Native won-deal creation and wake-up are committed with
the stage change; duplicate updates cannot create another open handoff. Sales
remains accountable until the account's designated Success owner accepts.

Maintenance detects material facts, decisions and linked PM task state; it fences
stale proposals/runs and schedules bounded follow-through. Pending approval,
uncertain results and incomplete linked tasks use cheap checks, not repeated AI.
No-progress/cost caps, explicit stop conditions and escalation routing remain
visible in Signals. Temporary authority-read failure denies action but is not
misrepresented as confirmed permission revocation.

Gmail sends use the approving user's connected mailbox and one exact linked
recipient, subject and body, with a stable RFC Message-ID. Unknown sends are
inspected against that identity, never retried automatically. Typed Playbook
outreach shares unresolved-intent and 24-hour recent-outreach checks, including
CRM-recorded manual messages. Independently configured sequences, ordinary Flows
and external senders are not silently placed under this coordination. PM tasks use
actual authorized teams/assignees and native CRM links; PM/Support/Docs are optional.

### Playbooks UI

`/w/:slug/crm/playbooks` and `/w/:slug/crm/playbooks/:playbookId` expose the
configuration, participation and explicit automation APIs. The index uses a searchable table
with server-paginated status filtering and open/paused/closed Signal counts;
those counts are objectives, not distinct customers. Detail separates Signals,
Setup, and Activity. Setup reuses the shared query builder, member picker, CRM
record picker, and Quiet page/form primitives. Additional eligibility rules
support all/any matching and preserve the original seven commercial motions.

Draft saving, publishing, allowing enrollment, applying a Playbook, recording
milestones, and changing individual Signal lifecycle remain separate commands.
The UI retains unsaved drafts on failed requests, protects navigation, keeps
retry keys stable for an unchanged intent, and refetches after command receipts.
The progress drawer resolves the Signal's pinned version through paginated
version history rather than displaying criteria from the latest draft. Ownership
and next-step edits use canonical Signal commands, preserving version/checkpoints.

Setup includes reviewed connection, separate activation, entry choice and usage
limits. Signal drawers expose start/resume/pause and focused execution status.
Exact action review reuses the existing composer and real PM/CRM pickers; result
inspection never retries the action. Activity shows recorded configuration and
execution events without presenting them as customer success. This is branch
implementation, not a deployment or migration of application data.

Browser verification renders the production pages with an isolated mocked API:
`cd frontend && npx playwright test --config=playwright.crm.config.ts`.
The harness does not require the API server or a Temporal worker.

### Signals daily workspace

The existing `/w/:slug/crm/insights` route now renders one Signal inbox containing
canonical situations, unresolved standalone recommendations, and the existing
commercially relevant evidence groups that are not already represented by work.
Evidence visibility uses the same grouping, scoring, and minimum lane priority
as the previous feed; it does not require an automation-routing policy or
activation-ready rules. The explicit workspace shadow opt-out still applies.
It retains the
approved Signal/Customer/Category/Owner/Priority columns, with the next step and
work status beneath the title. Priority uses the existing signal-scoring bands:
High >= 15, Medium >= 8, Low < 8, with the actual score in a tooltip. Standalone
recommendations without business scoring say **Not scored**, not zero or AI
confidence. Category is the only tab strip; assignment, work state, attention,
priority, evidence review, recommendation type, and sorting remain visible in
the main toolbar. The server owns filtering, category facets,
ranking, pagination, and uncategorized counts. Defaults are Everyone + Needs attention;
an explicitly selected personal/team filter remains unchanged.
Unclassified work remains visible in All; category selection never rewrites motion.

`?signal=:id` opens a shareable drawer using the same canonical record, query key,
and `SignalDrawer` component as Playbook participation. Linked actions use the
existing revision-bound situation approval/dismissal endpoints. Confirmation
resolves actual deal/pipeline/stage/customer targets before CRM mutations;
follow-up/enrichment/risk approvals explicitly record decisions only. Failed,
running, manual-required, and unknown results are not presented as completion.
Decision requests never automatically retry, including after a timeout. A stale
or failed decision must be closed and the refreshed action reviewed before retry.
All six existing dismissal reasons remain available; dismissing an action does
not close its situation or dismiss the evidence.

The shared next-step editor supports active-member assignment, a follow-up state,
and a local-time next check using the existing member/date controls. Waiting work
requires an active responsible member, a next step, and a checkpoint. Omitted
fields are preserved. Pausing, resuming, milestones, and outcomes use existing
canonical commands; none starts a Flow, Agent, outbound message, or PM task.
Pinned Playbook progress, evidence, and paginated activity are accessible on
demand. Opening a drawer or evidence section does not mark evidence reviewed.
Situation detail includes a workspace-scoped, deduplicated evidence projection
from direct references and linked actions, without internal scoring metadata.

### Review consolidation

`GET /crm/signal-inbox` composes canonical situations, unlinked unresolved
suggestions, and untracked evidence groups in one repeatable-read snapshot with
server-side filtering, counts, and pagination across the complete union. It never projects
sources, claims actions, routes owners, or creates tasks during a read. A
workspace-scoped `NOT EXISTS` excludes a standalone row once its suggestion is
linked to a situation. Independent customer objectives are not merged merely
because they share a customer. Classification follows explicit motion, the
target deal/pipeline motion, unambiguous evidence, then recommendation type.

Evidence rows have `kind=evidence` and use the previous feed's stable group ID,
not a situation ID. `GET /crm/signal-inbox/evidence/:id` and the `?group=:id`
drawer inspect current sources without importing situations or enqueueing
playbook-entry events. Evidence already referenced by tracked work (including
paused/closed work) or unresolved recommendations is excluded before grouping.
Dismissed, superseded, acted-on, context-only, and below-floor evidence does not
resurface as new work. Opening the drawer is read-only; marking evidence reviewed
remains an explicit existing CRM feedback action. Automatic situation imports
retain their independent policy gates; signal visibility cannot enable them.

`/crm/review` now redirects to Signals with Everyone + Needs approval and
recommendation-confidence sorting. The duplicate Review sidebar entry is removed.
Needs approval checks pending actions independently of the effective attention
status, so a failure, pause, or closed lifecycle cannot conceal another pending
recommendation. Lifecycle restrictions still prevent approving blocked work.
All five pending recommendation types are filterable. Search and ordering now
apply across the entire result set, not only the current legacy Review page.

Filters, pagination, `?signal=:id`, and `?recommendation=:id` are URL-backed and
survive drawer navigation. Recommendation details resolve the original suggestion
and revision, including after a decision or later projection. A linked
recommendation offers an explicit link to its canonical Signal; it does not
silently replace a drawer during an in-flight decision. The new revision-required
decision adapters delegate to the original suggestion services and executors.
No second approval lifecycle, execution ledger, or Flow/Agent runtime is added.

Exact CRM target links retain explicit-object precedence over context fallbacks.
Source links, all six dismissal reasons, and recommendation context/confidence
remain available. Context recorded at detection is collapsed and labeled as
historical; deal approval confirmations resolve current target records. Older
non-deal approvals with a legacy `succeeded` status are shown as requiring
follow-through, never as proof that a message was sent or a record changed.
Evidence review filters use current, non-dismissed/non-superseded sources and
remain separate from approval. The drawer's explicit **Mark evidence reviewed**
action uses the existing CRM endpoint; reads never mark evidence reviewed.

The prior intelligence page remains reachable at `/crm/insights?view=evidence`,
preserving raw motion/evidence filters, source navigation, deal health, CRM search,
and setup links. Review consolidation and guarded Playbook execution are connected
without rewriting ordinary saved Flows/Agents. Further changes to existing
Flow/Agent behavior still require the user's confirmation.

The isolated browser suite covers the compatibility redirect, standalone and
linked decisions, preserved filters/deep links, mixed failure/approval states,
evidence review, permissions, stale requests, and existing Playbook interactions.
Repository/HTTP tests also exercise the union, priority thresholds, motion
precedence, tenant isolation, pagination, and revision guards against SQLite and
a disposable PostgreSQL database. No application data migration is part of this
UI cutover's verification.

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
daily evaluator thereafter. Baseline leases are workspace-scoped and run
regardless of rollout mode, so the eight-week eligibility history matures even
for a workspace that has opted into shadow.
A workspace anomaly guard suppresses
a decline batch only with at least 100 eligible accounts and a 35% trip rate;
smaller workspaces rely on the five-of-seven rule. Suppressions are persisted
with their reason for auditability.

## Operations and migrations

Postgres signal models participate in API startup migration, with additional
idempotent schema work in `MigrateCRMSignalSchema`. Versioned Postgres SQL lives
under `server/internal/dbmigrate/sql`.

Customer work is owned by versioned migrations
`202609050002_crm_customer_situations.sql` and
`202609060001_crm_situation_sources.sql`, not GORM AutoMigrate. The source migration
allows provenance-backed work without a fabricated CRM target or human creator;
manual creation retains its stronger requirements. Neither migration rewrites
signals/decisions or replays external actions. Apply them through the normal
deployment migration process before enabling this backend. Reconciliation then
records eligible unresolved work without sending, creating tasks, or starting agents.

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
| CRM primary | `openrouter` / `z-ai/glm-5.3-flash:exacto` |
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

Customer-work foundation tests run without an application database:

```sh
cd server
go test -race ./internal/model ./internal/repository ./internal/service ./internal/handler ./internal/dbmigrate -run TestCRMSituation -count=1
```

Playbook policy, captured skills, live-gate enforcement, durable dispatch, exact
approvals and recovery are verified with isolated fixtures and fake providers:

```sh
cd server
go test -race ./internal/model ./internal/agentcontract ./internal/service ./internal/handler -run TestCRMPlaybook -count=1
```

Frontend API contracts are covered in
`frontend/src/lib/services/__tests__/crmPlaybookService.test.ts`. Browser acceptance
in `frontend/e2e/crm` covers Signals/Review parity, manual Playbooks, guided
automation setup, explicit activation, reviewed actions and result inspection.
Run these against local mocked endpoints with
`pnpm exec playwright test --config=playwright.crm.config.ts` from `frontend`.
The tests do not enable saved customer automations or send real messages.

To exercise the same fixtures against PostgreSQL, set
`CRM_SITUATION_TEST_POSTGRES_DSN` to a disposable database named
`crm_situation_test`. Each fixture creates a unique schema, applies the actual
migrations twice, and removes only that schema afterward. Never use an application
database for this override; ordinary test runs use isolated SQLite databases.

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

Then inspect `crm_signal_evaluation_runs` and `crm_signals` in Postgres or
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
| entity signal panels | `frontend/src/components/crm/EntitySignals.tsx` |
| SDK capture | `packages/sdk-js/src/core` |
| event pipeline | `events-pipeline/` |

## Related documents

- [`crm-signal-ingestion.md`](crm-signal-ingestion.md) — email and
  conversation ingestion details.
- [`crm-entity-summaries.md`](crm-entity-summaries.md) — downstream summary
  refresh and provenance.
- [`crm-email-sync.md`](crm-email-sync.md) — normalized CRM email source.
- [`CRM_MODULE.md`](CRM_MODULE.md) — broader CRM architecture.
- [`crm-buyer-signals-assessment.md`](crm-buyer-signals-assessment.md) —
  historical assessment and design rationale; not current status.
