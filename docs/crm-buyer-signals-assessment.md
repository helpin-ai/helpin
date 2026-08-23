# CRM Buyer Signals — Competitive Assessment

Assessed: 2026-08-23. Scope: the buyer-signal feature (`crm_buyer_signals`) and, secondarily, CRM entity summaries (`crm_entity_summaries`).

This is an **assessment**, not a specification. It describes what is implemented today, judges it against the current AI-CRM market, and proposes directions. Proposed signal names, weights, and half-lives in this document are illustrative starting points, not decisions.

Two existing docs describe these systems and both have drifted from the code:

- `docs/crm-buyer-signal-ingestion.md` states the confidence floor is `0.3` and the ingestion version is `phase1a`. The code uses `0.6` and `verified-v2`. It also states that support- and meeting-triggered ingestion do not exist; both ship today.
- `docs/crm-entity-summaries.md` states company summaries do not exist. They are fully implemented — service, handler, route, hooks, UI, daily reconciliation, and automation-catalog entry.

Where this document and those two disagree, this one reflects the code as read on the date above. Correcting those docs is separate work and is not done here.

---

## 1. What exists today

### Taxonomy

Seven signal types and six source types, defined in `server/internal/model/crm_signal.go`:

| Signal type | Meaning per the detection prompt |
|---|---|
| `buying_intent` | Interest in purchasing, pricing questions, demo requests, active evaluation |
| `objection` | Concerns, pushback on features/price/timeline, stated barriers |
| `competitor_mention` | Competing products named or alternatives alluded to |
| `budget_signal` | Budget availability, approval process, funding timeline |
| `timeline_signal` | Deadlines, implementation timelines, urgency |
| `champion_signal` | Internal advocate emerges, pushes for adoption, refers colleagues |
| `risk_signal` | Going silent, org changes, deprioritization |

Source types: `email`, `meeting`, `call`, `note`, `manual`, `support`.

### Pipeline

```
stored CRM record
  → eligibility gate            server/internal/crmsignal/ingestion.go
  → Temporal workflow           server/internal/temporalapp/signal_detection_workflow.go
  → LLM extraction              server/internal/service/crm_signal_detection.go
  → evidence verification       verifiedSignalEvidence
  → confidence floor (0.6)
  → dedupe (3 layers)           server/internal/repository/crm_signal.go
  → persist + WebSocket notify
```

Detection runs at temperature 0.1 in JSON mode, metered under `BillingFeatureCRMSignalDetection`, on the `automation-default` queue with deterministic workflow IDs (`crm-signal-email-{message_id}`) and `REJECT_DUPLICATE` reuse policy. Health telemetry reports to the automation catalog under `crm.buyer_signal_ingestion`.

### Live sources and their trigger sites

| Source | Where ingestion is triggered |
|---|---|
| Email (Gmail sync + manual) | `service/crm_email.go` (`manual_send`, `thread_reply`, `manual_create`); `temporalapp/email_sync_activities.go` (`storeMessage`) |
| Meetings / transcripts | `service/crm_meeting_processing.go` — `projectSignals`, transcript truncated to 12000 chars |
| Support conversations | `service/support_events.go` — `enqueueSupportSignalDetection` |
| Calendar | `model.PayloadFromCalendarEvent` exists but **has no caller** — declared, never ingested |
| Note / manual | Only via `POST /api/crm/signals`; no automated detector |

### Storage and surfaces

`crm_buyer_signals` carries `signal_type`, `source_type`, `source_id`, `source_thread_id`, `summary`, `evidence_excerpt`, `metadata` (JSONB), `confidence`, `detected_at`, `evidence_fingerprint`, `dismissed_at`, plus `contact_id` / `deal_id` / `company_id`.

Surfaces: `frontend/src/components/crm/BuyerSignals.tsx` (three presentations), mounted on contact, company, and deal detail pages, plus `frontend/src/pages/crm/Insights.tsx` (latest 12 signals, stat tiles). Ten routes under `/api/crm/signals`, `/api/crm/{entity}/{id}/signals`, and `/api/crm/health-scores` in `server/internal/router/router.go`.

---

## 2. Verdict: what is genuinely strong

These are not table stakes. Several are ahead of what the market ships.

**Evidence verification is the standout.** `verifiedSignalEvidence` (`service/crm_signal_detection.go`) strips HTML, unescapes entities, normalizes whitespace, then requires the model's `raw_evidence` to be a **literal substring** of `subject + body + thread_context`. Detections that fail are dropped outright. Most competitors ship LLM-extracted signals with no such gate, and pay for it in credibility the first time a rep is burned by a fabricated quote. This is the single best decision in the feature.

**Idempotency is layered properly.** Deterministic workflow IDs prevent duplicate work at the queue layer; a unique index on `(workspace_id, source_type, source_id, signal_type)` prevents duplicate rows; a 24-hour same-thread/same-type suppression window prevents noisy active threads from producing seven near-identical `timeline_signal` rows. Three independent mechanisms at three different layers, each doing a distinct job.

**Dismissal is fingerprint-aware.** `evidence_fingerprint` is a hash over `source_type|signal_type|summary|source_id|source_thread_id|evidence_excerpt`. A dismissed signal stays hidden until its evidence *materially changes*, then re-surfaces. This avoids both the "I dismissed it and it came back tomorrow" failure and the "I dismissed it once and never saw the escalated version" failure. Very few CRMs get this right.

**Provenance is complete and clickable.** Every signal stores its source type, source ID, thread ID, evidence excerpt, direction, participant count, content hash, and detector version. `BuyerSignals.tsx` deep-links each one to the originating email thread, meeting, support conversation, or note. A rep can always answer "why does the CRM think this?"

**Overviews degrade safely.** `crm_entity_summaries` distinguishes `stale` (refresh failed, prior good content preserved) from `error` (refresh failed, nothing usable). A failed regeneration never destroys a working summary. Combined with the readiness gate (three qualifying activities in 90 days before auto-generation) and the `ready`/`computed_at` skip check that avoids paying for redundant LLM calls, this is disciplined engineering.

**Summaries are grounded by construction.** The prompt forbids inventing CRM state and forbids verbatim quoting; the payload contains only stored CRM evidence; output is normalized post-hoc (markdown capped at 2200 chars, highlights capped at 5, non-allowlisted highlight kinds dropped); `source_refs` metadata backs a clickable "Based on" list in the UI.

---

## 3. Verdict: where it stands against the market

The reference set: [Clarify](https://www.clarify.ai/signals), [Common Room](https://www.commonroom.io/product/signals/), [Unify](https://www.unifygtm.com/explore/best-ai-tools-buyer-signals-to-outreach) + Clay, [Attio](https://attio.com/changelog/2026).

| Capability | Helpin | Clarify | Common Room | Unify + Clay | Attio |
|---|---|---|---|---|---|
| Conversation extraction (email/meeting/support) | **Strong** | Partial | Partial | Weak | Partial |
| Evidence grounding / verified quotes | **Strong** | None stated | None stated | None stated | None stated |
| Cross-module data (support + product + PM) | **Unique** | None | Community only | None | None |
| Market signals (funding, hiring, job changes) | **None** | Strong | Strong | Strong | Partial |
| Category / intent data | **None** | Strong | Strong | Strong | None |
| First-party behavior (site, product, opens) | **None** | Strong | Strong | Strong | Partial |
| Absence / time-based detection | **None** | None | None | Partial | Partial |
| Signal scoring, weighting, decay | **None** | Not stated | Not stated | **Strong** | Strong |
| Composite account score | **None** (table exists, no producer) | Not stated | Partial | Strong | Strong |
| Workspace-level signal feed | **None** | Strong | Strong | Strong | Strong |
| Signal → action routing / plays | **One broken template** | Strong | Strong | Strong | Strong |
| Feedback loop on signal quality | **None** | None stated | None stated | Partial | Partial |

The pattern is clear. Helpin has built one signal family — LLM-extracted sales semantics from conversations — and built it more carefully than anyone. It has built none of the other three families the market treats as standard, has no ranking model, and has almost no path from a detected signal to an action.

Clarify's framing is the cleanest articulation of what is missing. It organizes signals into three families: **market signals** (funding, hiring, champion moves), **category engagement** (intent), and **first-party behavior** (website visits, email opens, product usage). Helpin has zero coverage of the first two and near-zero of the third. Clarify's thesis — *"A single signal rarely changes anything. But together, they tell a story"* — is precisely the capability Helpin lacks, because it has only one kind of signal to combine.

One market note worth internalizing: by mid-2026, signal-only outbound is plateauing as the third-party signal stack commoditizes. Everyone can buy funding and hiring data from the same vendors. Differentiation is moving toward **proprietary signal sources** and **composition**. That should shape the priority order below.

---

## 4. Gap 1 — Scoring, decay, and ranking

### Observation

There is no scoring model of any kind.

- Confidence floor is `0.6` at ingest (`minimumDetectedConfidence`, `service/crm_signal_detection.go:21`) and `0.6` again at read (`minimumAutomatedSignalConfidence`, `repository/crm_signal.go:16`).
- Ordering is `ORDER BY detected_at DESC` (`repository/crm_signal.go:119`) and `detected_at DESC, id DESC` for the company rollup (line 180). Nothing else.
- `confidence` is the LLM's self-reported certainty that the signal *is what it says it is*. It is not a measure of how much the signal should matter. These are conflated everywhere they are used.
- `crm_deal_health_scores` exists as a table and a model, with `score` (0–100) and `factors` (JSONB). `CreateHealthScore` has exactly one caller: the HTTP handler at `router.go:1621`. **No job, cron, or workflow ever writes a health score.** The "Deal Health" panel on `Insights.tsx` is permanently empty in any real workspace.
- `Insights.tsx` computes "signal quality" as average confidence — which measures how confident the model was, not how often it was right.

The practical consequence: a `buying_intent` detected 90 days ago sorts identically to one detected this morning, and a `competitor_mention` on a $500 deal sorts above a `budget_signal` on a $500k deal that landed an hour earlier. Reps get a reverse-chronological list and must do the prioritization themselves — which is the work the feature was supposed to do.

### Direction

The market converges on a composite of the form:

```
Account Score = Σ ( SignalWeight × RecencyFactor × ICPMultiplier )
```

with `RecencyFactor` an exponential decay whose half-life varies by signal type. The reasoning is well documented — a pricing-page visit is nearly worthless after ten days, a funding event stays relevant for a month, a champion relationship stays relevant for a year. Unify and Overloop both publish decay windows on roughly this shape.

Applied to the current taxonomy, an illustrative starting table:

| Signal type | Illustrative weight | Illustrative half-life | Reasoning |
|---|---|---|---|
| `buying_intent` | High | ~7 days | Perishable; intent expressed a month ago is a different buying window |
| `budget_signal` | High | ~21 days | Budget cycles move slowly but do move |
| `timeline_signal` | High | ~10 days | Should arguably decay against the *stated* deadline, not detection date |
| `objection` | Medium | ~30 days | Persists until addressed; should ideally clear on resolution, not on time |
| `risk_signal` | High | ~30 days | Risk compounds rather than fading |
| `champion_signal` | Medium | ~180 days | Relationships are durable; this is the slowest-decaying type |
| `competitor_mention` | Medium | ~45 days | Evaluations run for weeks |

**These numbers are placeholders.** The only defensible way to set them is to fit against closed-won and closed-lost outcomes once there is enough history. Until then, they should be configurable and treated as guesses.

Three further pieces belong in the same work:

- **Compound boost.** Co-occurring types within a window are worth more than their sum. `budget_signal` + `timeline_signal` on the same account inside two weeks is a materially different situation from either alone. This is the mechanism behind Clarify's "together, they tell a story."
- **ICP multiplier.** A signal on an in-profile account matters more. Requires an ICP definition, which does not exist yet.
- **Separate `confidence` from `severity`/`weight`.** Keep `confidence` as "did this really happen" and add a distinct dimension for "how much should this move the needle." Conflating them is why the read-side 0.6 floor currently doubles as both a hallucination filter and a relevance filter.

Whatever produces this score is the natural producer for `crm_deal_health_scores`, whose `factors` JSONB column is already shaped to hold the per-signal contributions. That table does not need to be designed; it needs to be filled.

---

## 5. Gap 2 — First-party behavior signals

**This is the highest-leverage gap and the strategic centerpiece of this assessment.**

### Observation

Clarify, Common Room, and Unify all treat first-party behavior as a core signal family: website visits, email opens, product usage. Helpin captures none of it as signals.

But the framing "Helpin is behind on first-party behavior" understates the situation, because Helpin is not a standalone CRM. It is a single product with a single Postgres database containing:

- **Support conversations** (`support_conversations`) already carrying `crm_contact_id` and `crm_company_id` foreign keys, plus `priority`, `ai_escalated_at`, CSAT ratings in `metadata`, and full visitor context (device, location, company linkage status).
- **PM tasks**, joinable to accounts via `CompanyRollupID` — already used as company-summary evidence.
- **Docs and help center**, with `view_count` tracked per document (`model/docs.go:611`).
- **An embeddable widget** with visitor identity resolution (`VisitorContactData`, `VisitorCompanyContextStatus`, and the work described in `docs/PRD-widget-identify-crm-leads.md`).
- **Calendar events** (`model/crm_calendar.go`) with `status`, `attendees[].response_status`, `contact_ids`, and `deal_id`.

Clarify and Attio structurally cannot see any of this. They are CRMs that integrate with a support tool; Helpin *is* the support tool. This is a defensible moat, and it is currently unmined.

### Direction

The signals this unlocks do not exist in any competitor's catalog:

| Candidate signal | Derived from | Why it matters |
|---|---|---|
| Support volume spike on account | `support_conversations` grouped by `crm_company_id` over a rolling window | Churn precursor, or expansion pain — either way, sales should know |
| Open P1/urgent ticket on an account in negotiation | `priority` + deal stage | A deal blocker sales is currently blind to |
| Escalation to human | `ai_escalated_at` | The AI could not resolve it; frustration is real |
| CSAT drop | CSAT in conversation `metadata` | Leading churn indicator |
| Requested feature shipped | PM task state transition on a task linked to the account | The single best re-engagement trigger in existence, and only Helpin can fire it |
| Pricing / security doc viewed repeatedly | `docs` view tracking + widget visitor identity | Classic intent, sourced first-party rather than bought |
| Meeting booked / declined / cancelled / no-show | `crm_calendar_events.status`, `attendees[].response_status` | Among the strongest short-horizon signals in B2B sales |
| Buying committee expanded | New attendee domains on calendar events, or new participants on a thread | Deal is broadening — a strong positive |

Two properties make this cheap relative to its value:

1. **Most of it needs no LLM.** These are deterministic queries over data already in Postgres. No token cost, no hallucination surface, no evidence-verification problem — the evidence *is* the row.
2. **The calendar path is already half-built.** `model.PayloadFromCalendarEvent` exists and is unreferenced. The calendar event model already carries per-attendee `response_status`. Meeting-booked and meeting-declined are close to free, and are the cheapest high-value item in this entire document.

One design consequence: deterministic signals have no meaningful `confidence`. Either the meeting was declined or it was not. This argues for the `detector_kind` distinction described in the next section.

---

## 6. Gap 3 — Time-based and absence detectors

### Observation

Every trigger site in the system fires on **a new stored record**: an email persisted, a meeting transcript processed, a support message received. `crmsignal.IngestionService` is invoked from those paths and nowhere else.

The structural consequence is that Helpin can only detect things that *happened*. It cannot detect things that *stopped happening*.

This matters because the `risk_signal` type explicitly promises to catch "going silent" — but it can only fire if someone writes an email *saying* they are going silent. Actual silence, which is what going dark means, is invisible to the pipeline by construction. The taxonomy makes a promise the architecture cannot keep.

### Direction

A scheduled evaluator that queries for absence rather than reacting to presence. Candidates:

| Candidate signal | Condition |
|---|---|
| Gone dark | No inbound activity on a contact or deal for N days, where N is stage-dependent |
| Deal stalled | Time in current stage exceeds a threshold for that stage |
| Single-threaded | Only one contact has ever engaged on an open deal above a value threshold — one of the most reliable loss predictors in B2B |
| Champion went quiet | A contact with a prior `champion_signal` has stopped responding |
| Promised follow-up lapsed | A `timeline_signal` named a date; the date passed with no activity |
| Renewal window approaching | Contract date minus lead time, with no recent engagement |
| Buying committee shrank | A previously engaged contact stopped appearing on threads and calendar invites |

Two design notes:

- This needs a **scheduled sweep**, not the per-record workflow. `CRMSummaryDailyReconciliationWorkflow` (cron `0 3 * * *`) is the closest existing pattern and a reasonable model to follow.
- Absence signals and deterministic first-party signals share a property that LLM-extracted signals do not: they have no quoted evidence and no meaningful confidence score. A `detector_kind` field distinguishing `llm_extracted` from `rule_derived` would keep confidence semantics coherent, let the read-side 0.6 floor apply only where it means something, and let the UI present "no reply in 21 days" differently from "the model read this quote and inferred budget."

---

## 7. Gap 4 — Market and third-party signals

### Observation

Helpin ingests no external data as signals. Missing relative to the market: funding rounds, hiring spikes, job changes and champion moves, tech-stack changes, leadership changes, M&A, layoffs, news mentions, G2/review-site activity, and third-party intent.

`enrich_crm_company` and `enrich_crm_contact` (`commandtools/metadata.go`) do perform external lookups, but they write enrichment fields — they do not produce buyer signals. They are a partial starting point for the plumbing, not for the capability.

### Direction

This is real and worth having. Job changes in particular are among the highest-converting signals in the market: a champion who moves to a new company is a warm relationship at a cold account, and it is the canonical example both Clarify and Common Room lead with.

But it should be honestly ranked **last of the four**, for two reasons:

1. **It is the commoditized layer.** Every competitor buys funding and hiring data from the same handful of vendors. Building it produces parity, not advantage — and the mid-2026 market signal is that signal-only outbound built on this layer is already plateauing.
2. **It is the only one of the four that is primarily a procurement problem.** The others are engineering work against data Helpin already owns. This one requires a vendor contract, per-record cost, coverage evaluation, and an ongoing data-quality relationship. Building it in-house via scraping is a maintenance liability that competes with vendors who do nothing else.

The defensible sequencing is to mine the proprietary sources first (Gaps 2 and 3), get the ranking model working (Gap 1), and then buy the commodity layer to fill out breadth once there is a scoring system capable of weighting it correctly against the proprietary signals. Adding funding-round noise to a reverse-chronological list would make the product worse, not better.

---

## 8. Cross-cutting gaps

### Taxonomy has drifted three ways — and it has broken an automation

| Source | Types declared |
|---|---|
| `server/internal/model/crm_signal.go` (**the truth**) | `buying_intent`, `objection`, `competitor_mention`, `budget_signal`, `timeline_signal`, `champion_signal`, `risk_signal` |
| `CLAUDE.md` | substitutes `authority_signal`, `need_signal`, `churn_risk` for `objection`, `champion_signal`, `risk_signal` |
| `templates/manifests/buying_signal_to_task/template.yaml` | inherits CLAUDE.md's incorrect list |

This is not cosmetic. The template's default `signal_types` is `[buying_intent, budget_signal, authority_signal, need_signal, timeline_signal]` (`template.yaml:51`). Two of those five values **cannot match any stored row**. The `buying_signal_to_task` automation — the only signal-to-action path that exists — ships with a default filter that is 40% dead, and the taxonomy it exposes to users in its options list does not describe the system.

`CLAUDE.md` also lists source types as `email, meeting, call, support`, omitting `note` and `manual`.

### Missing signal types

The current seven cover early-to-mid funnel discovery. Absent: `authority` / decision-maker identified (BANT's A, and notably the type `CLAUDE.md` hallucinated into existence), `expansion` / upsell intent, `renewal`, `advocacy` / referral, `procurement` or security review started (a real and highly predictive late-stage gate in B2B), and `stakeholder change` (someone joined or left the buying committee). Post-sale coverage in particular is thin, which is odd given that Helpin owns the support surface where post-sale signal is richest.

### No activation layer

Signals are read-only panels on three detail pages plus a twelve-item list on Insights. There is:

- No workspace-level signal feed — no "what changed across my accounts today," which is the primary daily surface in every competitor product.
- No filtering by type, severity, account owner, or recency beyond the built-in list ordering.
- No subscriptions, alerts, or digests. A signal fires, a WebSocket event publishes, and if nobody has the contact page open, nobody learns anything.
- No routing to a rep. Unify's operating standard is under 72 hours from signal detection to first outreach; Helpin has no mechanism that would let a team measure that number, let alone hit it.
- One automation template, running on cron, with a broken default filter.

The detection is good. Nothing happens with it.

### No feedback loop

`dismissed_at` and `dismissed_by_member_id` are recorded and then never read for anything except hiding the row. Dismissal is the strongest available label — a human looked at a signal and said it was wrong or irrelevant — and it is discarded. There is no per-type precision metric, no prompt tuning signal, no per-workspace calibration, and no way to answer "is `competitor_mention` actually useful, or does everyone dismiss it?"

### Smaller items

- **HTML is not sanitized before prompting.** `preferredBody` in `crmsignal/ingestion.go` falls back to raw `body_html`. The evidence verifier *does* strip tags before matching, which creates an asymmetry: a legitimate quote spanning an inline tag boundary in the source can fail substring verification and be silently dropped. This makes the best feature in the system quietly lossy on HTML-heavy mail.
- **Multi-contact, no-deal emails are skipped entirely** (`multi_contact_without_deal`). This is a deliberate precision-first choice and was correct for Phase 1a, but it means Helpin systematically misses the buying-committee threads that matter most in B2B — exactly the deals worth the most.
- **`crm_deal_health_scores` has no producer** (see §4).
- **`PayloadFromCalendarEvent` has no caller** (see §5).

---

## 9. Overviews (entity summaries)

Shorter section — this feature is in better shape than signals.

**Strong.** The readiness gate (three qualifying activities in 90 days) is a genuinely good idea that prevents the most common failure mode of AI summaries: confidently summarizing nothing. The `stale`-vs-`error` distinction preserves the last good summary through failures. `source_refs` plus the "Based on" click-through list gives real provenance. Cost guardrails — the `ready`/`computed_at` skip check plus payload-hash idempotency — prevent redundant paid generations. The company summary already pulls PM tasks and support conversations, making it the one place where Helpin's cross-module advantage is actually realized today. The "Generate anyway" escape hatch with an explicit low-confidence warning is the right way to handle the gate.

**Gaps.**

- **Citations are not inline.** `source_refs` is a flat list beside the summary, not anchored to the claims in the markdown. A reader cannot tell *which* email supports *which* sentence. The market is moving the other way — inline, clickable, per-claim citations are becoming the differentiator in AI account research, and Helpin already stores everything needed to do it.
- **No "what changed since you last looked."** The summary is a snapshot that silently overwrites its predecessor. The most valuable thing a rep returning to an account wants is the diff, and prior versions are discarded.
- **No pre-meeting brief.** Attio ships briefing as a first-class agent action. Helpin has calendar events with linked contacts and deals plus a summary generator, and does not connect them.
- **No portfolio-level overview.** Summaries exist per contact, company, and deal. There is no pipeline-level or book-of-business-level synthesis — no "what should I care about across my 40 accounts this morning."
- **No feedback loop and no write-back.** Same gap as signals: no rating, no eval set, no golden dataset, and summaries never propose or make CRM updates.
- **Multi-contact/no-deal emails skip contact refresh**, mirroring the signal-side limitation.

---

## 10. Prioritized recommendations

Ordered by differentiation per unit of effort, not by size.

| # | Recommendation | Gap | Effort | Impact | Depends on |
|---|---|---|---|---|---|
| 1 | Reconcile the taxonomy across `crm_signal.go`, `CLAUDE.md`, and `buying_signal_to_task/template.yaml`; fix the template's dead default filter | §8 | Trivial | Unbreaks the only signal→action path | — |
| 2 | Refresh the two stale Phase 1 docs against the code | §0 | Trivial | Stops compounding drift | — |
| 3 | Wire calendar ingestion: meeting booked / declined / cancelled / no-show, from the existing unused payload builder | §5 | Small | High — strongest short-horizon signals, near-free | — |
| 4 | Sanitize HTML to text before prompt construction, aligning it with the verifier's normalization | §8 | Small | Recovers signals currently lost to verification asymmetry | — |
| 5 | Add `detector_kind` (`llm_extracted` \| `rule_derived`) and decouple `confidence` from `weight`/`severity` | §4, §6 | Small | Structural prerequisite for 6–8 | — |
| 6 | First-party behavior signals from support, PM tasks, docs, and widget | §5 | Medium | **Highest — the defensible moat** | 5 |
| 7 | Scoring model: per-type weight × exponential decay × ICP, compound boost; make it the producer for `crm_deal_health_scores` | §4 | Medium | High — turns a list into a ranking | 5 |
| 8 | Scheduled absence evaluator: gone dark, stalled, single-threaded, champion quiet, renewal window | §6 | Medium | High — closes the gap between what `risk_signal` promises and what it can detect | 5 |
| 9 | Workspace-level signal feed with filtering, plus routing and alerting | §8 | Medium | High — without it, detection has no consumer | 7 |
| 10 | Feed dismissals back into per-type precision metrics and prompt calibration | §8 | Medium | Medium — the only path to knowing if any of this works | — |
| 11 | Extend the taxonomy: `authority`, `expansion`, `renewal`, `advocacy`, `procurement`, `stakeholder_change` | §8 | Medium | Medium — post-sale coverage, where Helpin's data is richest | 1 |
| 12 | Inline per-claim citations in summaries; add a "what changed" diff | §9 | Medium | Medium | — |
| 13 | Revisit the multi-contact/no-deal skip once scoring can suppress low-value noise | §8 | Medium | Medium — unlocks buying-committee threads | 7 |
| 14 | Market/third-party signals: funding, hiring, job changes, tech-stack | §7 | Large | Medium — parity, not advantage; buy rather than build | 7, 9 |

The shape of this ordering: items 1–5 are cheap corrections and structural groundwork. Items 6–9 are the substance — mine what Helpin uniquely owns, rank it, detect what is absent, and actually put it in front of someone. Item 14 is last on purpose.

---

## Appendix A — Key files

**Backend**
- `server/internal/model/crm_signal.go` — types, sources, `CRMBuyerSignal`, `CRMDealHealthScore`
- `server/internal/model/crm_signal_source.go` — `SignalSourcePayload`, `PayloadFromEmail`, `PayloadFromCalendarEvent` (unused), `PayloadFromSupportMessage`
- `server/internal/crmsignal/ingestion.go` — eligibility gate, payload construction, workflow start
- `server/internal/service/crm_signal_detection.go` — LLM prompt, `verifiedSignalEvidence`, confidence floor
- `server/internal/service/crm_signal.go` — service layer, health scores
- `server/internal/repository/crm_signal.go` — dedupe, read floor, ordering, company rollup
- `server/internal/temporalapp/signal_detection_workflow.go` — workflow and activities
- `server/internal/service/crm_summary.go` — entity summaries (~1600 LOC)
- `server/internal/temporalapp/crm_summary_workflow.go` — debounce + daily reconciliation
- `server/internal/router/router.go` — signal and summary routes
- `server/internal/templates/manifests/buying_signal_to_task/template.yaml` — the one automation

**Frontend**
- `frontend/src/components/crm/BuyerSignals.tsx`
- `frontend/src/components/crm/EntitySummaryCard.tsx`
- `frontend/src/pages/crm/Insights.tsx`
- `frontend/src/hooks/queries/useCRM.ts`
- `frontend/src/lib/crmTypes.ts`

**Migrations**
- `server/migrations/028_crm_intelligence.sql`
- `server/migrations/040_crm_signal_ingestion.sql`
- `server/migrations/041_crm_entity_summaries.sql`
- `server/internal/dbmigrate/sql/202608230001_crm_buyer_signal_dismissal.sql`
- `server/internal/dbmigrate/sql/202608230002_crm_buyer_signal_company_context.sql`

## Appendix B — Market sources

- [Clarify — Buying Signals](https://www.clarify.ai/signals)
- [Common Room — Signals](https://www.commonroom.io/product/signals/)
- [Unify — Best AI Tools to Turn Buyer Signals Into Outreach](https://www.unifygtm.com/explore/best-ai-tools-buyer-signals-to-outreach)
- [Unify — How Signal-Based Selling Works](https://www.unifygtm.com/explore/how-signal-based-selling-works)
- [Overloop — Buying Signals: The Complete Playbook for B2B Outbound](https://overloop.com/blog/buying-signals-playbook)
- [Attio — 2026 Changelog](https://attio.com/changelog/2026)
- [Salesmotion — AI Account Research Tools That Cite Their Sources](https://salesmotion.io/blog/ai-account-research-tools-cite-sources)

## Related internal docs

- `docs/crm-buyer-signal-ingestion.md` (stale — see §0)
- `docs/crm-entity-summaries.md` (stale — see §0)
- `docs/CRM_MODULE.md`
- `docs/AGENTS_AND_AUTOMATION.md`
- `docs/PRD-widget-identify-crm-leads.md`
