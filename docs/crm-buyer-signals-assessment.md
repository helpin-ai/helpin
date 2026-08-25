# CRM Buyer Signals — Assessment and Implementation Roadmap

> **Historical design record.** This assessment preserves the decisions and
> implementation plan used to build the system; phase language and open-item
> lists below are not current status. Use
> [`crm-buyer-signals.md`](crm-buyer-signals.md) as the canonical implemented
> architecture and operations reference.

Assessed: 2026-08-23. Updated after the CRM signal reliability fixes and review of Helpin's vendored events pipeline and the upstream Usermaven implementation.

This document describes the current buyer-signal system and defines the implementation sequence for turning it into a trustworthy cross-product signal system. Phase 0 is a decision-complete implementation specification. Later phases define product direction; their detector thresholds and score weights must be calibrated with production evidence.

---

## 0. Decisions and resolved findings

### Resolved on 2026-08-23

The following findings were valid when the assessment was first written and are now fixed:

- `buying_signal_to_task` uses the canonical Go taxonomy. The invalid `authority_signal` and `need_signal` defaults were removed.
- `crm_deal_health_scores` has a deterministic producer. Scores refresh after signal changes, on reads as a backstop, and in a periodic workspace sweep.
- Calendar events enqueue versioned signal analysis on create, update, synchronization, cancellation, and deletion handling.
- Email HTML is converted to stable plain text before both prompting and literal evidence verification.

These items explain the current architecture but are not roadmap work.

### Architecture decisions

- Do not keep extending the seven-value signal taxonomy. Add independent `signal_domain` and `polarity` dimensions with stable rule identity and versioning.
- Identity provenance and trust are Phase 0 requirements. A signal written without provenance cannot be made trustworthy afterward.
- Keep Helpin's current opaque widget and server credentials. Do not encode tenancy into the API key or infer credential kind from its punctuation.
- Derive a stable public `workspace_identifier` from the immutable workspace UUID. Pixel and S2S payloads may include it, but it is never authorization.
- Rust capture obtains the authoritative workspace from the matched credential, rejects a supplied identifier that differs, and writes the validated value to ClickHouse `project_id`.
- Use two evaluator cadences: a frequent global micro-batch for perishable intent and a daily sweep for absence, decay, and lifecycle conditions.
- Product-usage signals require customer instrumentation. Installing the widget provides website and support behavior, not automatic product telemetry.
- `docs.view_count` is an aggregate and cannot support contact-level intent. That requires a dedicated identified article-view event.
- Multiple active stakeholders may be account context in v1, but cannot activate work until each contributing identity is independently trusted.

### Usermaven reference and the Helpin adaptation

Usermaven stores a stable public `Workspace.identifier` separately from its Postgres UUID. Browser events use that identifier; server credentials use `identifier.server_token`; Rust capture takes the first segment as ClickHouse `project_id`. Rotating the suffix therefore preserves tenancy.

That invariant is not true in Helpin. Helpin creates unrelated random `WidgetKey` and `SecretKey` values and rotates both. A bare Helpin server secret has no dot, so syntax-based project derivation and server classification are both wrong for Helpin's model.

Helpin will make the credential-to-workspace mapping explicit:

```text
opaque browser or server credential
  -> token registry match
  -> authorized credential { workspace_identifier, credential_kind, origins }
  -> optional payload workspace_identifier equality check
  -> ClickHouse project_id = authorized workspace_identifier
```

This preserves current installations, makes key rotation independent of event tenancy, and removes security decisions based on secret formatting.

---

## 1. What exists today

### Conversation signal taxonomy

Seven types are defined in `server/internal/model/crm_signal.go`:

| Signal type | Current meaning |
|---|---|
| `buying_intent` | Interest in purchasing, pricing, demos, or evaluation |
| `objection` | Feature, price, timing, or implementation concerns |
| `competitor_mention` | A competing product or alternative is being evaluated |
| `budget_signal` | Budget availability, approval, or funding timing |
| `timeline_signal` | Deadlines, urgency, or implementation timing |
| `champion_signal` | An internal advocate is pushing adoption |
| `risk_signal` | Deprioritization, silence, organizational change, or deal risk |

Sources are `email`, `meeting`, `call`, `note`, `manual`, and `support`. Calendar events become meeting payloads. These values remain the conversational classification axis; they are not the universal taxonomy for web behavior, product usage, support health, delivery, renewals, or market events.

### Detection pipeline

```text
stored CRM record
  -> eligibility gate
  -> deterministic Temporal workflow ID
  -> low-temperature JSON LLM extraction
  -> literal evidence verification
  -> LLM confidence floor
  -> layered deduplication
  -> Postgres persistence, summary/health refresh, WebSocket notification
```

The strongest existing properties are:

- Literal evidence verification drops unsupported model output.
- Workflow, database, and same-thread suppression provide distinct idempotency layers.
- Evidence fingerprints keep dismissed evidence hidden until it materially changes.
- Signals retain clickable provenance to their source.
- Contact, company, and deal summaries preserve their last good result when regeneration fails.
- Company summaries already combine CRM, email, meeting, support, task, deal, and related-contact context.

### Current deal health

Deal health now has a deterministic `v1` producer combining stage probability, inactivity, overdue close dates, and weighted/decayed conversation signals. It fixes the empty-panel defect but remains an initial heuristic:

- weights are global rather than calibrated;
- confidence still means extraction certainty, not business importance;
- support, product, behavioral, and absence evidence are not inputs;
- there is no outcome-based calibration or factor-level precision reporting.

### Events pipeline asset

Helpin already operates the vendored Usermaven pipeline:

```text
JS SDK -> Rust capture -> Kafka raw
       -> geo/proxy/bot/UA/privacy enrichment -> Kafka enriched
       -> KStreams sessionization -> Kafka sessionized
       -> ClickHouse usermaven.events
```

Rows contain anonymous and identified user fields, company fields, URL and campaign context, session IDs, bot/proxy classification, timestamps, custom attributes, and acquisition identifiers. The retroactive worker can restate prior anonymous activity after identification.

The missing pieces are trustworthy attribution and a tenant-safe Go read path. The Go server has no ClickHouse repository, and CRM pages correctly do not query ClickHouse synchronously.

---

## 2. Strategic direction

Helpin's conversation extraction is unusually careful. Its defensible advantage is broader: CRM, email, meetings, support, PM tasks, docs, website behavior, and instrumented product behavior can be evaluated inside one workspace. Competitors can buy the same funding and hiring feeds; they cannot easily reproduce this first-party cross-module context.

The intended model is:

```text
trusted evidence from multiple domains
  -> versioned deterministic or LLM detector
  -> durable signal with provenance
  -> decay and composition
  -> account/deal health and prioritized feed
  -> human-approved action
  -> feedback and calibration
```

ClickHouse remains the high-volume behavioral store, not the CRM source of truth. Evaluators write durable derived signals to Postgres; CRM surfaces read Postgres. Market feeds remain later work because they provide parity, while proprietary first-party composition provides differentiation.

---

## 3. Phase 0 — Trustworthy event tenancy and identity

### Goal and exit condition

Phase 0 makes every new behavioral event attributable to exactly one authorized workspace and records how strongly its person/company identity is known. No behavioral detector may activate a task, notification, or outreach before Phase 0 exits.

Phase 0 is complete when:

1. API-key rotation cannot change the ClickHouse project used by new events.
2. A credential cannot write under another workspace identifier.
3. Capture knows browser versus server credentials without examining token syntax.
4. Signed widget identity is verifiable, unsigned identity cannot mutate CRM in enforced mode, and provenance is persisted.
5. Existing SDKs continue ingesting during migration.
6. Existing rows remain readable through explicit legacy project aliases.
7. Signals can distinguish detector kind, domain, polarity, rule version, evaluation window, and identity trust.

### 3.1 Deterministic workspace identifier

Do not add another identifier column to `workspaces`. Define one shared function:

```text
WorkspaceIdentifier(workspaceUUID) =
  "hlp_v1_" + base64url_no_padding(
    SHA-256("helpin:event-project:v1:" + canonical_lowercase_workspace_uuid)
  )
```

- Parse and canonicalize the UUID before hashing; reject invalid values.
- Keep the full 256-bit digest rather than truncating it.
- The result is stable, public routing metadata, not a secret or authentication token.
- Do not use HMAC or a rotatable deployment secret: rotation would fragment tenancy again.
- Put the canonical implementation in a small Go event-tenancy package and publish fixed test vectors for Rust/TypeScript compatibility where needed.
- Rust normally receives the value through the token registry rather than recomputing it.

The internal workspace UUID remains the source of truth. The derived identifier can always be recomputed by token issuance, S2S publishers, evaluator repositories, and migrations.

### 3.2 Legacy project aliases

Add:

```text
workspace_event_project_aliases
  id UUID PK
  workspace_id UUID NOT NULL
  project_id TEXT NOT NULL UNIQUE
  source TEXT NOT NULL
  valid_from TIMESTAMPTZ
  valid_to TIMESTAMPTZ
  created_at TIMESTAMPTZ NOT NULL
```

- Backfill each active installation's current `widget_key`, because historical rows currently use it as `project_id`.
- On future key rotation, retain the retired key as an alias.
- Do not invent mappings for keys already lost before this migration. Recover those only from verifiable logs, backups, or historical configuration.
- New rows use only the derived workspace identifier. Aliases are a read-compatibility mechanism, never accepted as a workspace authorization claim.

### 3.3 Token registry and Rust authorization

Extend each internal token-registry record:

```json
{
  "id": "installation-id",
  "client_secret": "existing-widget-key",
  "server_secret": "existing-server-secret",
  "workspace_identifier": "hlp_v1_…",
  "origins": ["https://app.example.com"]
}
```

Replace authorization's `Result<()>` with:

```text
AuthorizedCredential {
  workspace_identifier,
  credential_kind: browser | server,
  installation_id,
  allowed_origins
}
```

Mandatory request flow:

1. Extract the effective credential using existing query/header precedence.
2. Match it and return its authorization context.
3. Parse events only after authorization succeeds.
4. If an event supplies `workspace_identifier`, require exact equality; reject the request on mismatch.
5. If an old SDK omits it, inject the authorized value.
6. Set transformed `project_id` from the authorization context, never from `api_key` or a payload-only field.
7. Decide server-only behavior, including custom timestamps, from `credential_kind`; remove `contains('.')` checks.
8. Use the same authorized project for processed, transformed, failed, and fallback-sink partition keys.

For browser credentials, validate `Origin` against configured origins. Missing Origin is acceptable only for server credentials and controlled non-browser transports. Introduce report-only origin metrics before enforcing. Never log credentials.

### 3.4 Pixel and S2S payloads

Add optional `workspace_identifier` to the JavaScript SDK configuration and event wire type.

- Widget/bootstrap responses provide it to new clients.
- Pixel, beacon, fetch, XHR, Node HTTPS, batch, and S2S transports preserve it.
- Helpin-owned S2S publishers include it explicitly.
- Older clients may omit it; capture derives it from the credential.
- Global properties and event attributes cannot override it.

The field exists for consistency diagnostics. It never replaces credential validation. No ClickHouse tenancy-column migration is required: the validated identifier continues to be written into existing `project_id`.

### 3.5 Secure widget identity

Implement a HelpScout-style Secure Mode using customer-server-generated HMAC-SHA256. The browser never receives the signing secret.

The SDK accepts:

```json
{
  "identity_verification": {
    "version": "v1",
    "issued_at": 1787500000,
    "expires_at": 1787500900,
    "signature": "lowercase-hex-hmac"
  }
}
```

Sign this newline-delimited, domain-separated message:

```text
helpin-widget-identity:v1
workspace_identifier
widget_key
normalized_email
external_user_id
company_external_id
issued_at
expires_at
```

- Normalize email by trimming and lowercasing; do not rewrite customer IDs.
- Empty optional values remain empty lines.
- Limit validity to 15 minutes and validate when identity is established or changed.
- Compare signatures in constant time.
- Use the installation secret with the domain-separated message.
- Secret rotation invalidates newly presented old signatures; already verified sessions retain their recorded provenance until session expiry.

Add `identity_verification_mode` (`off`, `report_only`, `enforced`) and `allowed_origins` to widget installation settings. Existing installs start `report_only`; new installs start `enforced`. `off` is development/test-only and is not a production UI option.

Add an admin-only, audited “rotate server/signing secret” operation that returns the new secret exactly once. Normal installation reads continue to mask it. This operation does not rotate the public widget key, so enabling Secure Mode does not force customers to replace their embed snippet. The same installation secret can authenticate S2S events and sign identity because the HMAC message is domain-separated; document both uses and warn that rotation requires updating customer backends.

Seed suggested origins from the workspace website and configured help-center/widget domains, but require an admin to confirm them. Store normalized scheme-and-host origins only; paths, queries, wildcard subdomains, and `*` are invalid in enforced production configuration.

In enforced mode, unsigned/invalid visitors retain anonymous support but cannot:

- match or create CRM contacts from email;
- create or associate companies from browser fields;
- promote lifecycle state;
- backfill conversations onto CRM identities;
- produce trusted behavioral signals.

Company identity is included in the signature. Signing email alone would leave client-supplied company identity exposed.

### 3.6 Identity provenance

Persist:

- `identity_method`: `anonymous`, `browser_claim`, `signed_widget`, `verified_support`, `connected_mailbox`, or `server_event`;
- `identity_trust`: `untrusted`, `probabilistic`, or `verified`;
- verification time and verifier version.

Store provenance on widget sessions and CRM identity-link operations. Add first-class event provenance through Rust transform, Kafka schemas, and the ClickHouse writer/table. The token context proves transport kind; the signature or connected channel proves person/company identity. Browser `user_id`, email, and company fields remain claims unless verification upgrades them. Never downgrade an existing verified identity from later weaker evidence.

### 3.7 Signal schema foundation

Add nullable/backfilled fields to `crm_buyer_signals`:

- `detector_kind`: `llm_extracted` or `rule_derived`;
- `signal_domain`: `conversation`, `web_behavior`, `product_usage`, `support`, `delivery`, `relationship`, or `market`;
- `polarity`: `positive`, `negative`, or `neutral`;
- `rule_key`, `rule_version`;
- `window_started_at`, `window_ended_at`;
- `evidence_identity_method`, `evidence_identity_trust`.

Backfill existing rows as `llm_extracted`/`conversation` and map polarity from current types. Preserve the seven signal types for API compatibility; do not add renewal, procurement, authority, and similar concepts to that enum.

Rule-derived signals do not fake LLM confidence. Make confidence nullable or apply the `0.6` floor only to `llm_extracted`. Business importance belongs in scoring, not confidence.

### 3.8 Tenant-safe Go boundary

Phase 0 adds the Go ClickHouse interface and a tenancy smoke query; detector queries ship in Phase 1.

- Repository construction requires an internal workspace UUID.
- It computes the canonical identifier and resolves verified legacy aliases in Postgres.
- Public methods never accept an arbitrary project override.
- Every query includes the resolved project set and a bounded time window.
- Empty project sets are errors.
- Query duration and row count are tagged by internal workspace UUID, never secrets.
- ClickHouse remains outside contact/company page render paths.

### 3.9 Deployment sequence

1. Add aliases, identity settings/provenance, and signal schema migrations; backfill current widget keys.
2. Add the deterministic identifier helper and cross-language fixed test vectors.
3. Extend the token endpoint with the derived identifier and configured origins while old capture ignores added fields.
4. Deploy Rust authorization context, validated project assignment, credential-kind handling, and report-only origins.
5. Deploy ClickHouse provenance schema/writer changes before emitting new fields.
6. Deploy SDK/bootstrap identifier and signed-identity support.
7. Enable Go verification/provenance in report-only mode and expose admin warnings.
8. Enforce signed CRM identity and origins after production telemetry shows installations are configured.
9. Enable tenant-safe ClickHouse smoke reads and compare canonical/alias counts.

Do not reinterpret historical rows without an explicit alias or rewrite historical identity trust upward.

### 3.10 Tests and observability

Required tests:

- fixed workspace-identifier vectors and canonical/invalid UUID handling;
- legacy-alias uniqueness and cross-workspace ownership constraints;
- browser/server credentials map to the same identifier and rotation leaves it unchanged;
- matching payload identifiers succeed, mismatches fail, omission supports old SDKs;
- batch, failed, fallback, and timestamp paths use authorization context;
- punctuation in credentials cannot grant server privileges;
- cross-workspace credentials cannot write, read, or alias another project;
- valid, invalid, expired, and tampered email/company/workspace signatures;
- unsigned enforced-mode identity cannot mutate CRM;
- verified identity cannot be downgraded;
- canonical plus legacy reads return expected events without leakage.

Required metrics:

- authorization failures and workspace mismatches;
- origin mismatches by installation;
- identity verification outcome/method;
- accepted events omitting explicit identifier;
- canonical versus legacy-alias counts;
- capture-to-ClickHouse delay and evaluator watermark lag.

Do not log raw credentials, signatures, email addresses, or full identity payloads.

---

## 4. Phase 1 — Proprietary first-party signals

Phase 1 turns already-owned evidence into durable Postgres signals. It does not add a live ClickHouse dependency to CRM pages.

### Evaluator framework

- Complete the Go ClickHouse reader behind the Phase 0 tenant boundary.
- Use global watermark-based micro-batches with overlap, not an unbounded per-workspace sweep.
- Upsert by workspace, rule key/version, entity, evidence window, and fingerprint.
- Store aggregate evidence and source references, not raw browsing histories, in Postgres.
- Use Usermaven query behavior as a reference, but re-evaluate its cost for scheduled cross-tenant work and implement the reader in Go.

### Frequent intent cadence

Run every 10 minutes with a 15-minute overlap. Initial activation-eligible rules:

1. Repeated pricing activity by a verified contact in a bounded window.
2. A verified known contact returning after dormancy.
3. Customer-instrumented high-intent product events sent with a server credential and verified identity.
4. Security/compliance/procurement-page activity by a verified identity on an account with an open deal.

Thresholds live in versioned rule configuration, not signal types.

### Daily lifecycle/absence cadence

Run daily with a two-day overlap. Initial rules:

- deal stalled relative to stage;
- no inbound activity for a stage-dependent period;
- champion or primary contact went quiet;
- promised date passed without follow-up;
- open urgent support issue on an account with an active deal;
- support volume or CSAT deterioration relative to baseline;
- requested feature completed on a PM task linked to the account.

Calendar booked, accepted, declined, cancelled, and no-show remains event-driven; the daily evaluator reconciles missed/changed outcomes.

Identity boundaries:

- connected mailbox, signed widget, and authenticated server evidence may activate;
- anonymous and browser-claimed behavior is context-only;
- multi-stakeholder activity is display-only in v1;
- `docs.view_count` is not contact evidence.

Phase 1 exits when rule signals are idempotent, explainable, tenant-safe, and measured in shadow mode before actions are enabled.

---

## 5. Phase 2 — Composition, ranking, and account health

- Keep extraction confidence separate from business weight.
- Score using domain weight, polarity, recency/decay, entity value/stage, and identity trust.
- Boost corroboration across independent domains, such as pricing behavior plus budget discussion.
- Preserve factor breakdowns so every score movement is explainable.
- Add company health only after entity-resolution precision is measured.
- Fit weights/half-lives against won, lost, expansion, and churn outcomes when sample sizes allow; label initial values heuristic.
- Track rule coverage, precision, dismissals, downstream action, and outcome correlation independently.

Add a workspace signal feed with owner, account, domain, polarity, severity, age, trust, and status filters; group corroborating evidence into account stories and emphasize what changed.

Phase 2 exits when reps can understand why an account is ranked, inspect every source, and distinguish extraction certainty from business priority.

---

## 6. Phase 3 — Activation and feedback

- Route eligible signals to owners, teams, feeds, digests, and notifications.
- Keep external outreach and consequential CRM writes human-approved by default.
- Gate `buying_signal_to_task` on trust, score, dedupe, and an existing-open-task check.
- Measure detection-to-review and detection-to-action time.
- Capture dismissal reasons: incorrect evidence, wrong entity, duplicate, irrelevant, handled, or bad timing.
- Report precision by rule version, domain, identity method, and workspace.
- Preserve historical rule versions for evaluation and rollback.
- Add pre-meeting briefs and “what changed” summaries from the same evidence graph.

Phase 3 exits when automation quality is measurable and reversible by rule version rather than inferred from aggregate LLM confidence.

---

## 7. Phase 4 — Additional capture and external signals

After proprietary signals are trusted, ranked, and actionable:

- implement explicitly configured `$form` capture for demo/pricing/contact forms;
- add an identified article-view event carrying article ID and identity provenance;
- activate scroll/click capture only when a versioned rule needs it;
- evaluate purchased funding, hiring, job-change, technology, leadership, and third-party intent providers;
- normalize external evidence into the same domain, polarity, provenance, version, decay, and feedback model;
- prefer vendor integration over maintaining broad scraping unless the source is uniquely proprietary.

External signals complement first-party evidence; they should not dominate merely because they are easy to buy.

---

## 8. Summary and overview roadmap

CRM entity summaries are ahead of buyer signals: readiness gating, stale-result preservation, source references, cost guards, and company-wide context already work.

Later improvements should reuse the trusted signal foundation:

- inline per-claim citations rather than a flat source list;
- version history and “what changed since last view”;
- pre-meeting briefs from linked contacts, deals, calendar, tasks, support, and signals;
- portfolio synthesis for an owner's accounts;
- explicit feedback/evaluation sets;
- proposed write-backs with approval, never silent CRM mutation.

Summaries must distinguish context-only untrusted behavior from verified evidence even when both are visible.

---

## Appendix A — Key implementation areas

### Backend

- `server/internal/model/support_inbox.go` — installation/token and signed-identity contracts.
- `server/internal/service/support_inbox_widget.go` — token registry, identify enforcement, and provenance.
- `server/internal/model/crm_signal.go` — detector/domain/polarity/rule/evidence fields.
- `server/internal/service/crm_signal.go` — deterministic health producer and later composition.
- `server/internal/crmsignal/` — normalized event-driven ingestion.
- `server/internal/temporalapp/` — workflows and future evaluator orchestration.

### SDK and pipeline

- `packages/sdk-js/src/core/client.ts` — event and backend-identify payloads.
- `packages/sdk-js/src/core/types.ts` — workspace identifier and signed identity types.
- `events-pipeline/rust-capture/src/auth/` — authorized credential context.
- `events-pipeline/rust-capture/src/events/event.rs` — optional wire identifier.
- `events-pipeline/rust-capture/src/enrichment/handler.rs` — validated project and credential kind.
- `events-pipeline/rust-capture/src/sinks/` — consistent project partitioning.
- `events-pipeline/eventpipeline-retroactive/` — identity-restatement reference.
- Separately deployed ClickHouse writer/schema — provenance columns and confirmed TTL/retention.

### Existing surfaces

- `frontend/src/components/crm/BuyerSignals.tsx`
- `frontend/src/components/crm/EntitySummaryCard.tsx`
- `frontend/src/pages/crm/Insights.tsx`
- `server/internal/templates/manifests/buying_signal_to_task/template.yaml`

## Appendix B — References

- [HelpScout — Advanced Beacon Customization](https://docs.helpscout.com/article/1406-advanced-beacon-customization)
- [HelpScout — Beacon Secure Mode](https://developer.helpscout.com/beacon-2/web/secure-mode/)
- [Clarify — Buying Signals](https://www.clarify.ai/signals)
- [Common Room — Signals](https://www.commonroom.io/product/signals/)
- [Unify — How Signal-Based Selling Works](https://www.unifygtm.com/explore/how-signal-based-selling-works)
- [Attio — 2026 Changelog](https://attio.com/changelog/2026)

## Related internal docs

- `docs/crm-buyer-signal-ingestion.md` — update separately for calendar and verification behavior.
- `docs/crm-entity-summaries.md` — update separately for company summaries.
- `docs/PRD-widget-identify-crm-leads.md`
- `docs/PRD_WIDGET_SDK_FEATURE_PARITY.md`
- `docs/PRD-support-live-chat.md`
- `docs/PRD-autonomous-support-coverage.md`
- `events-pipeline/README.md`
