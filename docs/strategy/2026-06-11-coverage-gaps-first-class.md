# Support Coverage Gaps — Current State & First-Class Roadmap (2026-06-11)

## What exists today (it's already deep)

The pipeline: support events (AI handoff, failed widget search, article feedback, human reply, human resolution)
→ real-time evidence recording + spike enrichment (5th evidence within cooldown triggers immediate Temporal enrichment)
→ daily Temporal batch (`CoverageDailyAnalysisWorkflow`, max 10 parallel workspaces, cursor-paginated, transcript-hash dedup)
→ Claude analysis per conversation (HasGap, kind/category taxonomy, customer_need / ai_failure / human_resolution / decision_reason, confidence)
→ knowledge refinement via embedding+lexical hybrid search (`CoverageKnowledgeMatcher`)
→ dedup (`DedupeKey`) + topic clustering (`ClusterKey`)
→ gap with evidence, recommendations, AI-drafted article suggestions (create/update/merge)
→ manual review UI (`SupportCoveragePage`: filters, detail pane, dismiss/done/reclassify/merge/link/draft).

Strengths: rich data model (12 entities), idempotent upserts, retrieval traces copied from live inbox AI (`SupportAIRetrievalTrace`), 24 test functions, v4 analyzer with active iteration, cross-module hook that marks docs blocks stale on resolution.

## Why it isn't first-class yet

The detection half is excellent; the **resolution half is manual and the loop never closes**:

1. **No closed loop.** Applying a suggestion publishes an article but the gap stays open until a human flips it. Nothing verifies the published article actually deflects the issue, and nothing reopens the gap on recurrence.
2. **No agent on the other end.** The Documentation Agent doesn't consume gaps. There's no `coverage.gap_detected` automation-rule trigger, despite the automation layer being built exactly for this.
3. **No prioritization.** ImpactTier exists but no scoring algorithm. Users can't answer "which gap costs us the most tickets?"
4. **Invisible.** Buried on one page. No notifications, no weekly digest wiring (the `SupportCoverageDigestDelivery` model exists, unused), no surfacing in the inbox or docs editor where the work actually happens. No trend reporting (`SupportCoverageSnapshot` exists, underused).
5. **Operational rough edges.** Reanalysis is all-or-nothing per workspace; legacy statuses (drafted/fixed/ignored/merged/human_only) linger; topic dedup misses near-synonym variants; daily LLM sweep over every conversation is unmetered cost.

## First-class roadmap (priority order)

### P0 — Close the loop (this turns a report into a system)
- **Auto-close with verification**: when a suggestion is applied, store `ResultDocumentID` (already done), then have the next daily run check whether new conversations on that topic now retrieve/cite the article (retrieval traces already capture `CitedSourceIDs`). If yes for N days → auto-transition `open → done` with `status_changed_by = system`. If the topic recurs after closure → reopen with a `regression` flag.
- **Deflection attribution**: count AI-resolved conversations citing a gap-born article. This single number ("articles created from gaps deflected X conversations this month") is the feature's ROI statement and the demo headline.

### P1 — Wire the agents (this is the product thesis)
- Add automation-rule trigger `coverage.gap_detected` (and `coverage.gap_spike`) following the existing `story.state_entered` pattern, carrying the minimal trigger payload per the agents taxonomy in `docs/AGENTS_AND_AUTOMATION.md`.
- Default rule (autonomy-gated like CRM thresholds): high-confidence `missing_article` gaps → Documentation Agent drafts the article as a docs change proposal → human approves → publish → P0 loop verifies. Confidence below threshold → suggestion stays manual, exactly like CRM's auto_execute/review thresholds.
- This makes coverage the flagship example of "built-in automation + system agent" working together — the self-healing knowledge base.

### P2 — Prioritization & visibility
- **Impact score** = evidence_count × recency decay × failure_mode weight (ai_handoff > weak_retrieval) × unique-customer count. Sort the gap list by it; populate ImpactTier from it.
- **Notifications**: spike → workspace notification; weekly digest (model already exists) with top-5 gaps by impact + gaps closed + deflection stats.
- **Surface in flow**: inbox conversation view shows "this conversation contributed evidence to gap X"; docs editor shows "this article resolves/relates to N open gaps."
- **Trend reporting** from snapshots: gaps opened/closed per week, coverage score, time-to-resolution, deflection rate. This doubles as the first real support analytics surface.

### P3 — Hardening
- Per-topic reanalysis (replace workspace-wide `TriggerReanalysis` as the default path).
- Migrate legacy statuses to the open/done/rejected model; drop dead enum values.
- Embedding-based topic merge suggestions ("these 3 topics look like one issue — merge?").
- Meter analyzer LLM spend per workspace (feeds the billing/AI-metering work).
- Graceful degradation alert when embedding provider fails (today it silently falls back to lexical).

## Positioning note

Intercom Fin and Zendesk AI answer from existing content. None of the named competitors close the loop from *unanswerable conversations* back into *published, verified knowledge* autonomously. With P0+P1 shipped, coverage gaps stops being an internal report page and becomes the marquee story: **"Your knowledge base fixes itself."**
