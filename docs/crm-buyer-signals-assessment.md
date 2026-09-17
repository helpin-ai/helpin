# CRM Buyer Signals — Assessment and Implementation Roadmap

> **Historical design record.** This assessment preserves the decisions and
> implementation plan used to build the system; phase language and open-item
> lists below are not current status. Use
> [`crm-signals.md`](crm-signals.md) as the canonical implemented
> architecture and operations reference.

Assessed: 2026-08-23. Updated after the CRM signal reliability fixes and review of Helpin's vendored events pipeline and the upstream Usermaven implementation.

This document describes the current buyer-signal system and defines the implementation sequence for turning it into a trustworthy cross-product signal system. Phase 0 is a decision-complete implementation specification. Later phases define product direction; their detector thresholds and score weights must be calibrated with production evidence.

Two tracks start immediately and in parallel: Phase 0 (event tenancy and identity) and Phase 1a (cross-module Postgres signals). Phase 1a is not gated on Phase 0 — see §4.

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
- Use the immutable workspace UUID directly as the ClickHouse `project_id`. Do not derive, hash, or publish a separate public identifier: the value never reaches a browser, so it needs no public form.
- Tenancy is never carried in the event payload. Rust capture obtains the authoritative workspace from the matched credential in the token registry and writes it to `project_id`. A payload-supplied workspace field would be unauthoritative by construction and is not added.
- Use two evaluator cadences: a frequent global micro-batch for perishable intent and a daily sweep for absence, decay, and lifecycle conditions.
- Product-usage signals require customer instrumentation. Installing the widget provides website and support behavior, not automatic product telemetry.
- `docs.view_count` is an aggregate and cannot support contact-level intent. That requires a dedicated identified article-view event.
- Multiple active stakeholders may be account context in v1, but cannot activate work until each contributing identity is independently trusted.
- Phase 0's trust gate applies to pixel-captured and browser-claimed evidence only. Cross-module Postgres signals (Phase 1a) are already identity-trusted and run in parallel with Phase 0 rather than behind it.

### Usermaven reference and the Helpin adaptation

Usermaven stores a stable public `Workspace.identifier` separately from its Postgres UUID. Browser events carry that identifier; server credentials use `identifier.server_token`; Rust capture takes the first dot-segment as ClickHouse `project_id`. Rotating the suffix therefore preserves tenancy, and the identifier must be public because the browser sends it.

Neither half of that holds in Helpin:

- Helpin creates unrelated random `WidgetKey` and `SecretKey` values and rotates both, so no segment of a Helpin credential encodes tenancy. A bare Helpin server secret has no dot, so syntax-based project derivation and server classification are both wrong here.
- **Helpin's browser never sends a project identifier at all.** `project_id` does not appear anywhere in `packages/sdk-js/src`; it is produced server-side in Rust and consumed only by Kafka, KStreams session keying, ClickHouse, and the retroactive worker.

The second point removes the reason Usermaven needs a derived public identifier. Helpin resolves the workspace entirely from the credential:

```text
opaque browser or server credential
  -> token registry match (existing GET /api/internal/widget-tokens)
  -> authorized credential { workspace_id, credential_kind, origins }
  -> ClickHouse project_id = authorized workspace_id
```

Consequences, all simplifications relative to the Usermaven shape:

- No hashing helper, no canonicalization rules, no cross-language fixed test vectors, no 50-byte partition key. `project_id` is the workspace UUID.
- No SDK or bootstrap change is required for tenancy, so the SDK release leaves the Phase 0 critical path. The SDK still changes for signed identity (§3.5), which is independent.
- Go evaluator queries join directly on the workspace UUID instead of derive-then-resolve.
- Key rotation independence comes from the registry indirection, not from the string's shape.

One open item before committing: confirm that the separately deployed Usermaven analytics UI does not expose `project_id` in a customer-visible URL or export. If it does, the UUID becomes semi-public and a derived opaque identifier should be reconsidered. This is the same unknown recorded in §3.2.1 and should be answered at the same time.

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
       -> ClickHouse helpin.events
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

Phase 0 makes every new behavioral event attributable to exactly one authorized workspace and records how strongly its person/company identity is known.

Scope of the gate: no detector whose evidence originates in **browser-claimed or pixel-captured behavior** may activate a task, notification, or outreach before Phase 0 exits. Detectors whose evidence originates in already-trusted Postgres records — connected mailboxes, agent-verified support conversations, PM tasks, calendar events, deal state — are not gated by Phase 0 and ship in Phase 1a. Their identity is established by existing CRM foreign keys and connected-channel ownership, not by widget claims, so Phase 0's signing and tenancy work does not change their trust level.

Phase 1a still depends on §3.7 (signal schema fields), which is small and independent of the Rust, SDK, and ClickHouse work.

Phase 0 is complete when:

1. API-key rotation cannot change the ClickHouse project used by new events.
2. A credential cannot write under another workspace.
3. Capture knows browser versus server credentials without examining token syntax.
4. Signed widget identity is verifiable, unsigned identity cannot mutate CRM in enforced mode, and provenance is persisted.
5. Existing SDKs continue ingesting during migration.
6. Existing rows remain readable through explicit legacy project aliases.
7. Signals can distinguish detector kind, domain, polarity, rule version, evaluation window, and identity trust.

### 3.1 Workspace identity on events

`project_id` is the canonical lowercase workspace UUID. There is no derived identifier, no new column on `workspaces`, and no shared hashing function.

- The token registry is the only place the credential-to-workspace mapping is resolved (§3.3).
- Rust never computes a workspace value; it copies the one attached to the matched credential.
- Nothing outside the server ever sends or receives it, so it needs no public, rotatable, or opaque form.
- Canonicalize to lowercase at the single point where the token response is built, so Rust, Kafka keys, and ClickHouse all see one byte-identical string.

An earlier revision of this document specified a `hlp_v1_<sha256>` derived identifier, copied from Usermaven's model. That was dropped once it was confirmed the Helpin SDK never sends a project identifier: the derivation existed only to make a public value safe, and Helpin has no public value. See the Usermaven comparison in §0.

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

Backfill scope — there are **two** historical project shapes, not one. `enrichment/handler.rs:152` sets `project_id = api_key.split('.').next()`, and Helpin credentials contain no dot, so the first segment is the whole credential:

| Credential presented | Historical `project_id` |
|---|---|
| Widget/browser key | the `widget_key` value |
| Server secret (S2S) | **the `server_secret` value itself** |

Consequences that the migration must handle:

- A single workspace's browser and server events are already split across **two unrelated ClickHouse projects**. Alias backfill must insert a row for the current `widget_key` *and* a row for the current `server_secret` of every active installation, or S2S history becomes unreachable after cutover.
- The server secret is currently written into ClickHouse as a `project_id` value, and `handler.rs:159` additionally copies the full credential into the `api_key` column. A live secret is therefore stored in the analytics store in plaintext, replicated to every downstream consumer of those tables.
- Alias rows sourced from a server secret must be marked `source = 'legacy_server_secret'` and treated as read-only compatibility keys. They must never be returned by any API, log line, or metric label, and the admin UI must not display them.
- Treat this as a credential exposure. Phase 0 includes: stop writing the raw credential to the `api_key` column (write the installation ID or nothing), and offer affected workspaces the audited secret rotation from §3.5. Rotation does not invalidate the alias, because the alias maps a historical string to a workspace and is never an authorization claim.

Remaining rules:

- On future key rotation, retain the retired key as an alias.
- Do not invent mappings for keys already lost before this migration. Recover those only from verifiable logs, backups, or historical configuration.
- New rows use only the workspace UUID. Aliases are a read-compatibility mechanism, never accepted as a workspace authorization claim.

### 3.2.1 Downstream query compatibility

Cutover changes `project_id` for new rows only. The Go reader resolves canonical plus alias projects (§3.8), but the separately deployed Usermaven analytics UI and any saved dashboards query a single project string and will silently show a series that stops at the cutover date. Before enabling the new value in production, confirm which of these applies and record the answer here:

- the analytics UI can be pointed at a project set rather than a single project; or
- a one-time ClickHouse restatement rewrites historical `project_id` to the workspace UUID (feasible with the ReplacingMergeTree `ver` mechanism the retroactive worker already uses); or
- the split is accepted and dashboards are rebuilt.

Do not cut over until one of these is chosen. This is the only Phase 0 step with user-visible analytics impact.

Answer one further question in the same pass: **does that UI expose `project_id` in a customer-visible URL, filter, or export?** If yes, the workspace UUID becomes semi-public and §3.1's decision to use it directly must be revisited in favour of an opaque derived value. If no — the expected answer, since `project_id` is server-produced and absent from the SDK — the UUID stands.

### 3.3 Token registry and Rust authorization

**The registry already exists.** This section extends a live component; it does not introduce one.

`GET /api/internal/widget-tokens` (`router/router.go:434`) is served behind `middleware.RequireInternalAPISecret` and built by `SupportInboxService.ListWidgetTokens` (`service/support_inbox_widget.go:1426`) from all active widget installations. Rust fetches it from `HTTP_TOKENS_URL` with an `INTERNAL_API_SECRET` bearer, retries with backoff on startup, then refreshes **every 10 seconds** in a background task, keeping the previous set on failure or empty response (`auth/http_tokens.rs`).

#### 3.3.1 Server-side changes

`ListWidgetTokens` currently emits `{id, client_secret: WidgetKey, server_secret: SecretKey, origins: ["*"]}`. Two changes:

- Add `workspace_id` — the canonical lowercase workspace UUID for the installation. This is the entire tenancy fix.
- Return the installation's **configured** origins instead of the hardcoded `[]string{"*"}`. Until this lands, §3.5's origin validation is a no-op regardless of what Rust implements, because every installation authorizes every origin. The `allowed_origins` field must exist on the installation model first.

The response shape becomes:

```json
{
  "id": "installation-id",
  "workspace_id": "0f9c…-uuid",
  "client_secret": "existing-widget-key",
  "server_secret": "existing-server-secret",
  "origins": ["https://app.example.com"]
}
```

Existing credentials are unchanged, so no customer re-embeds a snippet.

#### 3.3.2 Rust authorization context

`validate_token` (`auth/authorization.rs`) already distinguishes which field matched — `found_api_key` (client secret), `found_token` (server secret via `x-auth-token`), and `found_api_key_in_server_secret` (server secret presented as `api_key`). **Credential kind is therefore already known at match time.** The `contains('.')` server classification can be deleted without adding any new signal; only the return type has to change.

Replace authorization's `Result<(), InvalidTokenReason>` with:

```text
Result<AuthorizedCredential, InvalidTokenReason>

AuthorizedCredential {
  workspace_id,
  credential_kind: browser | server,
  installation_id,
  allowed_origins
}
```

Mandatory request flow:

1. Extract the effective credential using existing query/header precedence.
2. Match it and return its authorization context.
3. Parse events only after authorization succeeds.
4. Set transformed `project_id` from the authorization context, never from `api_key`.
5. Stop copying the raw credential into the `api_key` column; write `installation_id` or nothing (§3.2).
6. Decide server-only behavior, including custom timestamps, from `credential_kind`; remove `contains('.')` checks.
7. Use the same authorized project for processed, transformed, failed, and fallback-sink partition keys.

There is deliberately no step that reads tenancy from the request body. See §3.4.

For browser credentials, validate `Origin` against `allowed_origins`. Missing Origin is acceptable only for server credentials and controlled non-browser transports. Introduce report-only origin metrics before enforcing. Never log credentials.

#### 3.3.3 Endpoint hygiene

The endpoint returns every active installation's `client_secret` and `server_secret`, unpaginated, and Rust re-fetches the full set every 10 seconds. This is fine at current scale and becomes a growing plaintext credential transfer on a fixed loop as installations grow. While this section's changes are being made, add either an `ETag` / `If-None-Match` short-circuit or a change cursor, so steady-state refreshes return `304` instead of the whole table. Keep the fail-safe behavior of retaining the prior token set on error.

### 3.4 Pixel and S2S payloads — no change

**Decision: do not add a workspace field to the SDK configuration or the event wire type.**

An earlier revision specified an optional `workspace_identifier` on the payload, validated for equality against the credential's workspace. That is removed. A payload-supplied tenancy value cannot be authoritative, because the credential already determines the workspace unambiguously at match time (§3.3.2). Carrying it anyway would add:

- a new rejection path whenever SDK and server versions skew;
- an SDK release and bootstrap change on the Phase 0 critical path;
- a tenancy-shaped field in payloads that a future reader could mistake for authorization.

Nothing is gained in exchange. `project_id` does not appear anywhere in `packages/sdk-js/src` today, and Phase 0 preserves that: the browser never sends, receives, or learns the workspace value.

Consequences:

- The SDK requires **no** tenancy change. It still changes for signed identity (§3.5), which is independent and can ship on its own schedule.
- Helpin-owned S2S publishers authenticate with the installation server secret and send no workspace field.
- No ClickHouse tenancy-column migration is required; the authorized workspace UUID is written into the existing `project_id` column.
- Old and new SDK versions are indistinguishable to capture, so migration has no client-version dependency.

### 3.5 Secure widget identity

Implement a Secure Mode using customer-server-generated HMAC-SHA256. The browser never receives the signing secret.

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
widget_key
normalized_email
external_user_id
company_external_id
issued_at
expires_at
```

- Normalize email by trimming and lowercasing; do not rewrite customer IDs.
- Empty optional values remain empty lines.
- `widget_key` is the workspace binding. It is unique per installation and already known to the customer's backend, so the message needs no separate workspace field — one more reason no public workspace identifier is required.
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

Add nullable/backfilled fields to `crm_signals`:

- `detector_kind`: `llm_extracted` or `rule_derived`;
- `signal_domain`: `conversation`, `web_behavior`, `product_usage`, `support`, `delivery`, `relationship`, or `market`;
- `polarity`: `positive`, `negative`, or `neutral`;
- `rule_key`, `rule_version`;
- `window_started_at`, `window_ended_at`;
- `evidence_identity_method`, `evidence_identity_trust`.

Backfill existing rows as `llm_extracted`/`conversation` and map polarity from current types. Preserve the seven signal types for API compatibility; do not add renewal, procurement, authority, and similar concepts to that enum.

Rule-derived signals do not fake LLM confidence. Make confidence nullable or apply the `0.6` floor only to `llm_extracted`. Business importance belongs in scoring, not confidence.

### 3.8 Tenant-safe Go boundary

Phase 0 adds the Go ClickHouse interface and a tenancy smoke query; detector queries ship in Phase 1b. Phase 1a touches Postgres only and does not cross this boundary.

- Repository construction requires an internal workspace UUID.
- It resolves the project set as the workspace UUID plus verified legacy aliases from Postgres. No derivation step is involved.
- Public methods never accept an arbitrary project override.
- Every query includes the resolved project set and a bounded time window.
- Empty project sets are errors.
- Query duration and row count are tagged by internal workspace UUID, never secrets.
- ClickHouse remains outside contact/company page render paths.

### 3.9 Deployment sequence

1. Add aliases, `allowed_origins` on the installation model, identity settings/provenance, and signal schema migrations; backfill both the current widget key and the current server secret of every active installation (§3.2).
2. Resolve the §3.2.1 downstream query-compatibility question and confirm `project_id` is not customer-visible.
3. Extend `GET /api/internal/widget-tokens` with `workspace_id` and configured origins. Old capture ignores unknown fields, so this is deployable on its own.
4. Deploy Rust authorization context: `AuthorizedCredential` return type, `project_id` from the matched credential, credential-kind from the match rather than punctuation, stop writing the raw credential to the `api_key` column, report-only origin metrics.
5. Deploy ClickHouse provenance schema/writer changes before emitting new fields.
6. Deploy SDK signed-identity support. No SDK tenancy change is required (§3.4), so this step is independent of steps 3–4 and does not block them.
7. Enable Go verification/provenance in report-only mode and expose admin warnings.
8. Enforce signed CRM identity and origins after production telemetry shows installations are configured.
9. Enable tenant-safe ClickHouse smoke reads and compare canonical/alias counts.

Steps 3–4 are the whole tenancy fix and involve one Go endpoint and one Rust module. The SDK, ClickHouse writer, and enforcement work sit on separate tracks.

Do not reinterpret historical rows without an explicit alias or rewrite historical identity trust upward.

### 3.10 Tests and observability

Required tests:

- the token response carries a canonical lowercase workspace UUID for every active installation, and a missing one fails closed rather than defaulting to empty;
- legacy-alias uniqueness and cross-workspace ownership constraints;
- both widget-key and server-secret aliases resolve to the same workspace, and server-secret aliases never appear in API responses, logs, or metric labels;
- no raw credential is written to the ClickHouse `api_key` column after cutover;
- an installation's client secret and server secret both resolve to the same `project_id`, and rotating either leaves it unchanged;
- a payload field named like a workspace or project is ignored entirely and cannot influence `project_id`;
- batch, failed, fallback, and timestamp paths use authorization context;
- punctuation in credentials cannot grant server privileges, and credential kind is taken from which registry field matched;
- an unmatched or stale credential is rejected rather than producing an empty or partial `project_id`;
- cross-workspace credentials cannot write, read, or alias another project;
- valid, invalid, expired, and tampered email/company/workspace signatures;
- unsigned enforced-mode identity cannot mutate CRM;
- verified identity cannot be downgraded;
- canonical plus legacy reads return expected events without leakage.

Required metrics:

- authorization failures by reason;
- origin mismatches by installation;
- identity verification outcome/method;
- token-registry refresh age, fetch failures, and consecutive stale-set retentions — a silently stale registry rejects newly created installations;
- canonical versus legacy-alias event counts, per workspace, across cutover;
- capture-to-ClickHouse delay and evaluator watermark lag.

Do not log raw credentials, signatures, email addresses, or full identity payloads.

---

## 4. Phase 1 — Proprietary first-party signals

Phase 1 turns already-owned evidence into durable Postgres signals. It does not add a live ClickHouse dependency to CRM pages.

It splits into two tracks with **different dependencies and different start dates**. Phase 1a depends only on §3.7 and can begin immediately, in parallel with Phase 0. Phase 1b depends on all of Phase 0.

| Track | Evidence source | Depends on | Can start |
|---|---|---|---|
| 1a — Cross-module | Postgres: support, PM tasks, calendar, deals, email | §3.7 signal schema only | Immediately, parallel to Phase 0 |
| 1b — Behavioral | ClickHouse `helpin.events` | All of Phase 0 | After Phase 0 exits |

The ordering is deliberate. 1a is the part no competitor can reproduce — they integrate with a support tool; Helpin *is* the support tool — and it requires no Rust deploy, no SDK release, no ClickHouse schema change, and no customer-side backend work. Gating it behind Phase 0 would put the cheapest differentiated work behind the longest pole in the plan.

### Shared evaluator framework

- Use global watermark-based micro-batches with overlap, not an unbounded per-workspace sweep.
- Upsert by workspace, rule key/version, entity, evidence window, and fingerprint.
- Store aggregate evidence and source references, not raw histories, in Postgres.
- Thresholds live in versioned rule configuration, never in signal types.
- Every rule runs in shadow mode with precision measured before it is allowed to activate anything.

Identity boundaries (both tracks):

- connected mailbox, agent-verified support, signed widget, and authenticated server evidence may activate;
- anonymous and browser-claimed behavior is context-only;
- multi-stakeholder activity is display-only in v1;
- `docs.view_count` is an aggregate and is not contact evidence.

### 4.1 Phase 1a — Cross-module signals (Postgres only)

No ClickHouse dependency, no Phase 0 dependency beyond the §3.7 columns. Identity comes from `support_conversations.crm_contact_id` / `crm_company_id`, PM task company rollups, calendar attendee records, and connected-mailbox threads — all already trusted under the `verified_support` and `connected_mailbox` provenance methods.

Most of these rules need no LLM. They are deterministic queries, so there is no token cost, no hallucination surface, and no evidence-verification problem: the evidence *is* the row.

Run daily with a two-day overlap, except where noted:

| Rule | Derived from | Why it matters |
|---|---|---|
| Support volume spike on account | `support_conversations` grouped by `crm_company_id` over a rolling window vs. baseline | Churn precursor or expansion pain — either way sales should know |
| Open urgent issue on a negotiating account | `priority` + open deal stage | A deal blocker sales is currently blind to |
| Escalation to human | `ai_escalated_at` | The AI could not resolve it; frustration is real |
| CSAT deterioration | CSAT in conversation `metadata` vs. account baseline | Leading churn indicator |
| Requested feature shipped | PM task state transition on a task linked to the account | The strongest re-engagement trigger available, and only Helpin can fire it |
| Deal stalled relative to stage | Time in current stage vs. stage threshold | — |
| Gone dark | No inbound activity for a stage-dependent period | Closes the gap between what `risk_signal` promises and what event-driven detection can see |
| Champion went quiet | Contact with a prior `champion_signal` stopped responding | — |
| Promised follow-up lapsed | A `timeline_signal` named a date; the date passed with no activity | — |
| Single-threaded deal | Only one contact has ever engaged on an open deal above a value threshold | One of the most reliable loss predictors in B2B |
| Renewal window approaching | Contract date minus lead time, with no recent engagement | — |
| Buying committee expanded | New attendee domains on calendar events, or new participants on a thread | Deal is broadening — a strong positive |
| Buying committee shrank | A previously engaged contact stopped appearing on threads and invites | — |

Calendar booked, accepted, declined, cancelled, and no-show remains **event-driven** rather than swept; the daily evaluator reconciles missed or changed outcomes.

Absence rules are the structural reason this track matters beyond its cheapness. Every existing trigger site fires on a newly stored record, so the system can only detect what happened, never what stopped happening — while `risk_signal` explicitly promises to catch going silent. Actual silence is invisible to the current architecture by construction.

Phase 1a exits when these rules are idempotent, explainable, and shadow-measured.

### 4.2 Phase 1b — Behavioral signals (ClickHouse)

Gated on Phase 0. Complete the Go ClickHouse reader behind the Phase 0 tenant boundary; use Usermaven query behavior as a reference but re-evaluate its cost for scheduled cross-tenant work and implement the reader in Go.

Run every 10 minutes with a 15-minute overlap, scoped to workspaces with at least one active installation and recent event volume — not a full tenant scan. Initial activation-eligible rules:

| Rule | Derived from |
|---|---|
| Repeated pricing activity by a verified contact | `doc_path` matching + `session_id` count in a bounded window |
| Security / compliance / procurement page activity on an open-deal account | `doc_path` — a strong late-stage procurement indicator |
| Verified known contact returning after dormancy | Gap between `session_id` groups for a `user_anonymous_id` |
| Customer-instrumented high-intent product event | Server-credential events with verified identity |

Context-only in v1 (display, never activation) because identity is browser-claimed or absent:

| Candidate | Derived from |
|---|---|
| Session depth / dwell spike | `pageview` count per `session_id`, `$pageleave` timing |
| New stakeholder at a known account | Unseen `user_anonymous_id` resolving to a known `company_id` |
| Anonymous account traffic — the dark funnel | `company_id` present, `user_id` empty |
| Campaign-attributed return | `utm_*` / `click_id_gclid` on a known contact |
| Pre-identification history | The retroactive backfill, surfaced when a lead converts |

Confirm the ClickHouse TTL and retention policy before assuming any lookback window: the writer that owns the DDL is referenced by `events-pipeline/README.md` but is not vendored in this repo.

Phase 1b exits when behavioral rules are idempotent, explainable, tenant-safe, and shadow-measured.

---

## 5. Phase 2 — Composition, ranking, and account health

- Keep extraction confidence separate from business weight.
- Score using domain weight, polarity, recency/decay, entity value/stage, and identity trust.
- Boost corroboration across independent domains, such as pricing behavior plus budget discussion.
- Preserve factor breakdowns so every score movement is explainable.
- Add company health only after entity-resolution precision is measured.
- Fit weights/half-lives against won, lost, expansion, and churn outcomes when sample sizes allow; label initial values heuristic. Appendix C holds illustrative starting values — placeholders for calibration, not decisions.
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

- `server/internal/model/support_inbox.go:1489` — `WidgetToken` / `WidgetTokensResponse`; add `workspace_id`, and the signed-identity contracts.
- `server/internal/service/support_inbox_widget.go:1426` — `ListWidgetTokens`; source of the token registry, currently hardcoding `Origins: ["*"]`.
- `server/internal/handler/support_inbox_widget.go:39` — `GetWidgetTokens`; add ETag/change-cursor handling here.
- `server/internal/router/router.go:434` — `GET /api/internal/widget-tokens` behind `RequireInternalAPISecret`.
- `server/internal/service/support_inbox_widget.go` — identify enforcement and identity provenance.
- `server/internal/model/crm_signal.go` — detector/domain/polarity/rule/evidence fields.
- `server/internal/service/crm_signal.go` — deterministic health producer and later composition.
- `server/internal/crmsignal/` — normalized event-driven ingestion.
- `server/internal/temporalapp/` — workflows and future evaluator orchestration.

### SDK and pipeline

- `packages/sdk-js/src/core/client.ts` — backend-identify payload; signed identity only, no tenancy change.
- `packages/sdk-js/src/core/types.ts` — signed identity types.
- `events-pipeline/rust-capture/src/auth/http_tokens.rs` — `Token` struct and the 10-second registry refresh; add `workspace_id`.
- `events-pipeline/rust-capture/src/auth/authorization.rs` — `validate_token`; return `AuthorizedCredential` instead of `Result<()>`. The match already distinguishes `found_api_key` / `found_token` / `found_api_key_in_server_secret`, so credential kind needs no new input.
- `events-pipeline/rust-capture/src/enrichment/handler.rs:152` — the `api_key.split('.')` derivation to delete; line 159 copies the raw credential into the `api_key` column.
- `events-pipeline/rust-capture/src/sinks/` — consistent project partitioning.
- `events-pipeline/eventpipeline-retroactive/` — identity-restatement reference.
- Separately deployed ClickHouse writer/schema — provenance columns and confirmed TTL/retention.

### Existing surfaces

- `frontend/src/components/crm/EntitySignals.tsx`
- `frontend/src/components/crm/EntitySummaryCard.tsx`
- `frontend/src/pages/crm/Insights.tsx`
- `server/internal/templates/manifests/buying_signal_to_task/template.yaml`

## Appendix B — References

- [HelpScout — Advanced Beacon Customization](https://docs.helpscout.com/article/1406-advanced-beacon-customization)
- [HelpScout — Beacon Secure Mode](https://developer.helpscout.com/beacon-2/web/secure-mode/)
- [Clarify — Buying Signals](https://www.clarify.ai/signals)
- [Common Room — Signals](https://www.commonroom.io/product/signals/)
- [Unify — How Signal-Based Selling Works](https://www.unifygtm.com/explore/how-signal-based-selling-works)
- [Overloop — Buying Signals: The Complete Playbook for B2B Outbound](https://overloop.com/blog/buying-signals-playbook) — published decay windows
- [Attio — 2026 Changelog](https://attio.com/changelog/2026)
- [Salesmotion — AI Account Research Tools That Cite Their Sources](https://salesmotion.io/blog/ai-account-research-tools-cite-sources)

## Appendix C — Illustrative scoring parameters

**These are placeholders, not decisions.** They exist so Phase 2 starts from a stated hypothesis rather than an empty table. The only defensible way to set them is to fit against closed-won and closed-lost outcomes once there is enough history; until then they must be configurable and treated as guesses.

The market converges on a composite of the form:

```text
Account Score = Σ ( SignalWeight × RecencyFactor × EntityMultiplier )
```

with `RecencyFactor` an exponential decay whose half-life varies by signal type, and `EntityMultiplier` covering deal value, stage, and ICP fit once an ICP definition exists.

| Signal type | Illustrative weight | Illustrative half-life | Reasoning |
|---|---|---|---|
| `buying_intent` | High | ~7 days | Perishable; intent expressed a month ago is a different buying window |
| `budget_signal` | High | ~21 days | Budget cycles move slowly but do move |
| `timeline_signal` | High | ~10 days | Should arguably decay against the *stated* deadline, not the detection date |
| `risk_signal` | High | ~30 days | Risk compounds rather than fading |
| `objection` | Medium | ~30 days | Persists until addressed; should ideally clear on resolution, not on time |
| `competitor_mention` | Medium | ~45 days | Evaluations run for weeks |
| `champion_signal` | Medium | ~180 days | Relationships are durable; the slowest-decaying type |

Rule-derived signals from Phase 1 need their own weights and half-lives per `rule_key`, held in the same versioned rule configuration rather than in this table.

Two further mechanisms belong in the same work:

- **Compound boost.** Co-occurring signals from *independent domains* within a window are worth more than their sum. Pricing-page behavior plus a `budget_signal` on the same account inside two weeks is a materially different situation from either alone. A single signal rarely changes anything; together they tell a story — and composition is only possible once more than one signal family exists, which is what Phase 1 delivers.
- **Separate `confidence` from `weight`.** Keep `confidence` as "did this really happen" and add a distinct dimension for "how much should this move the needle." Conflating them is why the read-side `0.6` floor currently doubles as both a hallucination filter and a relevance filter.

## Related internal docs

- `docs/crm-signal-ingestion.md` — update separately for calendar and verification behavior.
- `docs/crm-entity-summaries.md` — update separately for company summaries.
- `docs/prds/widget-identify-crm-leads.md`
- `docs/prds/widget-sdk-feature-parity.md`
- `docs/prds/support-live-chat.md`
- `docs/prds/autonomous-support-coverage.md`
- `events-pipeline/README.md`
