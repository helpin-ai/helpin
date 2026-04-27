# Coverage Gaps Redesign — Design Spec

**Date:** 2026-04-27
**Status:** Proposed
**Module:** Support → Coverage Gaps

---

## 1. Problem

The Coverage Gaps page exists to surface support failures and turn them into knowledge-base improvements. In production we observe two failures of the current implementation:

1. **The inbox is unreadable.** Most gaps land in the `needs_review` bucket because the rule-based classifier (`server/internal/service/support_coverage_rules.go`) falls back to it whenever an event lacks an `issue_key`. On a real workspace, this means the most common signal — `human_reply_after_ai` — almost always produces a `needs_review` gap with a generic title like "Needs review: human resolved after AI failure."
2. **Each ticket becomes its own row.** Dedupe is computed via `hashExcerpt(summary)` — a SHA-256 of the lowercased, trimmed summary text. Slight wording variations of the same underlying problem ("How do I reset my password?" vs. "how to reset password") produce separate gaps. The inbox accumulates dozens of duplicates per real issue.

The combined result: writers and managers stop opening the page because there is no clear next action and the noise drowns the signal. The feature does not currently produce its intended outcome (published articles that prevent future tickets).

## 2. Goals

The redesign optimizes for one outcome: **a published or updated KB article that demonstrably reduces future support failures on the same topic.** Success metrics:

- Number of articles shipped from gaps per week.
- Median time from gap surfaced → article published.
- Reduction in evidence on a topic in the 30 days after an article ships.

Non-goals for v1 (kept in scope for future phases — see Section 9):

- Data gaps (AI lacked customer/account context).
- Action gaps (AI couldn't perform a requested action).
- Outcome dashboards / 30-day deflection reporting.
- Inline surfacing inside the docs editor ("3 readers found this confusing").

## 3. Target users

The feature serves a **support team as a unit**. Two practical user shapes:

- **Solo / small team (Helpin's typical customer):** one person wears the docs writer, support manager, and AI ops hats. They want a single ranked to-do list of "what to fix this week."
- **Larger orgs (15+ support):** distinct content owner, support manager, automation owner. Filter chips let each role focus on their lane without fragmenting the data model.

Persona D from brainstorming: serve all sizes via one shared model with role-flavored filters, not separate views.

## 4. Three-category model (architectural target)

Based on Intercom Recommendations parity research (March 2026), gaps fall into three categories:

| Category | Cause | Fix path | v1? |
|---|---|---|---|
| **Content gap** | KB content missing, weak, outdated, or contradictory | Write/edit a Doc | ✅ |
| **Data gap** | AI lacked account/customer context | Connect a data source | ❌ (Phase 3) |
| **Action gap** | AI couldn't perform an action the user wanted | Create/extend an agent or automation tool | ❌ (Phase 4) |

**v1 ships content gaps only**, but the data model, classifier interface, and UI components must accommodate all three categories from day one to avoid a rewrite when data and action gaps land.

## 5. Conceptual architecture

```
Support events  ──►  Topic clusterer  ──►  Coverage gap upsert
                                                    │
                                              (live evidence
                                               flows in here)
                                                    │
                            ┌───────────────────────┼───────────────────────┐
                            ▼                       ▼                       ▼
                     Daily batch enricher     Spike trigger          Manual regenerate
                     (03:00 ws-local)         (≥5 events / hour)     (UI button)
                            │                       │                       │
                            └───────────────────────┼───────────────────────┘
                                                    ▼
                                           LLM enrichment
                                  (canonical title, gap subtype,
                                   route decision, draft article)
                                                    │
                                                    ▼
                                          Coverage gaps inbox
                                          (impact-ranked, filterable)
                                                    │
                                                    ▼
                                          User: Reject | Add ▾
                                                    │
                                                    ▼
                                  Doc created / existing Doc updated
                                  Gap status → Done
```

## 6. Components

### 6.1 Signal layer (existing, unchanged in v1)

`SupportEvent` ingestion in `server/internal/service/support_coverage.go` continues unchanged. The seven existing event types still feed gap creation. No new event types required for v1.

### 6.2 Topic clusterer (new)

A new component that assigns each incoming event to a `support_topic`, instead of writing one gap per `dedupeKey` hash.

**v1 implementation (no LLM, no embeddings):**

- Normalize `issue_summary`: lowercase, strip punctuation, remove stopwords, sort tokens.
- Hash the normalized form → use as cluster key.
- Lookup or create a `support_topic` row keyed by the cluster hash + workspace.
- Attach the event as evidence to the gap belonging to that topic.

This is a deliberate v1 shortcut. Section 8 calls out the upgrade path to embedding-based clustering when volume justifies it.

### 6.3 Coverage gap upsert (modified)

`SupportCoverageGap` rows now represent **one gap per topic per workspace**, not one per event hash. The current `dedupe_key` column is repurposed to the topic-cluster hash. Existing fields (`evidence_count`, `first_seen_at`, `last_seen_at`, `confidence`) keep their meaning.

A new column `gap_kind` (enum: `content` | `data` | `action`) is added — defaults to `content` in v1, exists to support Phase 3/4.

### 6.4 Enricher (new — async Temporal workflow)

A new `CoverageGapEnrichmentWorkflow` runs on three triggers:

- **Daily batch** at 03:00 workspace-local time — re-enriches every gap with new evidence since last run.
- **Spike trigger** — if a topic accumulates ≥5 new evidence items in a rolling 1-hour window, enqueues immediate re-enrichment for that single topic.
- **Manual regenerate** — UI button enqueues a single-gap re-enrichment.

The enrichment LLM call (using existing `internal/llm/` provider, same pattern as `crm_signal_detection.go`) takes:

- Top N evidence excerpts for the topic (deduped, max ~3000 tokens).
- Workspace KB context: list of existing article titles + a vector retrieval of the top 5 candidate articles to update.

It returns:

- `canonical_title` — action-verb form: `"Write article: …"` or `"Update article: {existing_title}"`.
- `gap_subtype` — `missing_article` | `weak_article` | `outdated_or_conflicting`.
- `route` — `create_new` | `update_existing` (with target `document_id`).
- `draft_markdown` — the suggested article body or section to add.
- `confidence`.

Output is written to a `SupportGapSuggestion` row (existing model, repurposed). Older suggestions for the same gap are superseded, not deleted (for audit).

### 6.5 Impact ranking (new — read-time)

Each gap gets an impact tier computed at read time:

- **High:** evidence_count ≥ 10 in last 30d
- **Medium:** evidence_count ≥ 3
- **Low:** evidence_count < 3

Thresholds are global v1 constants — tunable per workspace later. Gaps with `evidence_count == 1` are hidden by default (filterable via "Show noise"). Default sort: impact desc, then `last_seen_at` desc.

### 6.6 Lifecycle (modified)

Status state machine:

```
[Open] ──► (Add)    ──► [Done]
       └─► (Reject) ──► [Rejected]
```

- `Open` is the only inbox state. `Done` and `Rejected` are removed from default view.
- **Manual close only** — no auto-close in v1.
- Existing statuses `drafted` and `fixed` collapse into `Done`. `ignored` collapses into `Rejected`. A migration is needed (Section 7).

### 6.7 UI (rewritten)

Three-column layout matching the validated Intercom reference:

- **Left:** workspace nav (no change — existing sidebar).
- **Middle (gap list):**
  - Header: "Most impactful ways to improve coverage" + total count.
  - Filter chips: `CONTENT GAPS` (only one in v1, but rendered as a chip so the pattern lands now) + `RESOLVED` toggle.
  - Sort dropdown: Impact (default) | Recent.
  - Each row: category badge · action-verb title · preview snippet · impact tier badge.
  - Empty state: positive — "Nothing to fix right now."
- **Right (detail pane):**
  - Top: recommendation card — title, evidence count, last-seen, **Reject** + **Add ▾** buttons.
  - Add button label exposes routing: `Add to "Password reset basics" ▾` or `Create new article ▾`.
  - Dropdown options: `Create new article instead` / `Add to a different article…` / `Open in editor` (loads draft into TipTap without persisting a Doc until save).
  - Body: inline Markdown preview of `draft_markdown`.
  - Below: Evidence list (existing) — collapsible, no internal scroll (per current "single page scroll" decision).

Page itself uses single-scroll layout already implemented in `routes/_authenticated/w/$slug/support/coverage.tsx`.

### 6.8 Manual regenerate

A small "Regenerate" button on the detail pane enqueues a single-gap re-enrichment. Disables for 30 seconds after click to prevent spam.

## 7. Data model changes

All via dbmigrate (SQL migrations under `server/internal/dbmigrate/sql/`).

### 7.1 New columns on `support_coverage_gaps`

- `gap_kind text not null default 'content'` — for Phase 3/4.
- `topic_id uuid` — already exists; will be backfilled where currently null.
- `impact_tier text generated always as (...) stored` — optional; can be view-computed instead.

### 7.2 Status migration

```sql
update support_coverage_gaps set status = 'done'     where status in ('drafted','fixed');
update support_coverage_gaps set status = 'rejected' where status = 'ignored';
```

Then drop the old enum values once code no longer references them.

### 7.3 Dedupe rebuild

A one-time backfill: re-cluster all existing `open` gaps using the new normalize-and-token-sort hash. Merge duplicates: pick the gap with highest `evidence_count` as primary; sum counts; reattach evidence rows; mark the others as `rejected` with note `"merged into {primary_id} during cluster rebuild"`.

This is destructive of historical gap IDs but preserves all evidence. Run as a separate dbmigrate migration with explicit logging.

### 7.4 No new tables in v1

`SupportGapSuggestion` is repurposed (existing). `support_topics` is reused (existing). No new tables.

## 8. Deferred decisions (called out so we don't accidentally bake them in)

- **Embedding-based clustering.** The token-sort hash will fail on semantically equivalent but lexically distinct phrasings ("can't log in" vs. "trouble signing in"). When v1 ships and we measure cluster precision on real data, we will either (a) tighten the normalization rules or (b) introduce pgvector embeddings for the topic clusterer. Reserved column `embedding vector(1536)` may be added at that point — not now.
- **Outcome measurement.** "Did the article reduce tickets?" requires linking topics to subsequent conversations and computing deflection rate. Designed but not built in v1.
- **Inline surfacing on the docs editor.** Out of scope.

## 9. Phasing

- **Phase 1 (this spec):** Content gaps only — clusterer, enricher, redesigned UI, manual lifecycle.
- **Phase 2:** Outcome measurement (30-day deflection reports), inline surfacing on KB articles.
- **Phase 3:** Data gaps — new event type emitted by the AI agent when it lacks customer context. New fix path: data connector setup.
- **Phase 4:** Action gaps — new event type emitted when AI rejects an action it cannot perform. New fix path: agent/automation creation.

The architectural commitments in v1 — `gap_kind` column, category chip in UI, three-category enrichment interface — are present so Phases 3 and 4 are additive, not rewrites.

## 10. Error handling and edge cases

- **LLM enrichment failure:** gap remains in current state with last successful suggestion (or no suggestion). Surfaces in UI with "regenerate" affordance. Failure logged with `slog.ErrorContext`.
- **No evidence on a gap:** can't happen in v1 (gap is created from evidence). Defensive: gaps with `evidence_count == 0` are hidden everywhere.
- **Empty workspace KB:** enrichment falls back to `route = create_new` with no `update_existing` candidate.
- **Spike trigger storm:** the per-topic cooldown (1 hour) prevents repeated re-enrichment on a still-active spike. Daily batch is the catch-up.
- **Doc deleted after Add:** gap stays `Done`. We do not re-open. The next batch run on new evidence may create a new gap on the same topic.

## 11. Testing

- **Unit tests** for the topic clusterer normalizer (table-driven: input variants → expected normalized form).
- **Unit tests** for the impact tier computation.
- **Service tests** for the enricher with a mocked LLM provider.
- **Integration tests** for the full ingest → cluster → enrich → list pipeline against in-memory SQLite.
- **Migration test** for the status backfill and dedupe rebuild on a fixture dataset.

## 12. Rollout

- Behind a feature flag `support.coverage_gaps_v2` (per-workspace). Default off.
- Workspaces are migrated to the new model on flag flip — backfill runs at flip time, not at deploy time.
- Old UI remains accessible behind a "Switch back" link for two weeks after enable, then removed.

---

## Open questions to validate before implementation

1. Are workspace-local times available everywhere we need them for the daily batch trigger? If not, default to UTC 03:00.
2. Does the existing `support_topics` table have the shape we need for the new clusterer, or do we need columns added? (To be confirmed in implementation planning.)
3. What is the LLM cost budget per workspace per day? Current estimate (~50 calls/day for an active workspace) needs validation against Helpin's typical workspace size.
