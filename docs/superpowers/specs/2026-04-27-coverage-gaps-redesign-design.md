# Coverage Gaps Redesign — Design Spec

**Date:** 2026-04-27
**Status:** Proposed (v1.2, post-review)
**Module:** Support → Coverage Gaps

**Revision log:**
- 2026-04-27 v1: initial draft.
- 2026-04-27 v1.1: addressed code review — lifecycle precision, async enrichment specifics (workflow IDs, idempotency, cooldowns, Temporal-disabled fallback), expanded `support_coverage_topics` schema, cluster key composition rule, suggestion versioning, 30-day impact computation source, dropped per-workspace feature flag (does not exist as infra), added outcome instrumentation in v1, prior-work crosslinks.
- 2026-04-27 v1.2: addressed second-round review — spike trigger SQL uses `gap_id` (no `topic_id` on evidence); partial unique index on open gaps only (allows future gaps on same topic after rejection/Done); fixed env var to `TEMPORAL_ADDRESS` and dropped the invalid "disabled fallback" — Temporal is now declared a hard dependency matching CRM workflows; §6.6 confirms `DocsDocumentService.Create` defaults to `DocStatusDraft` (open question #1 resolved); switched to existing constants `create_article` / `update_article` (instead of invented `create_new` / `update_existing`); confirmed `support_gap_evidence.created_at` exists (`server/internal/model/support_coverage.go:152`), closing open question #4.

---

## 0. Relationship to prior work

This spec **supersedes the product direction** in:

- `docs/superpowers/plans/2026-04-15-docs-coverage-loop-v1.md` — built the current event/gap/evidence/suggestion foundation. **Kept:** signal capture (`SupportEvent`), gap row model, evidence model.
- `docs/superpowers/specs/2026-04-15-coverage-resolution-flows-design.md` — added draft/update/apply flows. **Kept:** `SupportGapSuggestion` model with `suggestion_type` (`create_article` | `update_article`), the apply-to-doc backend, and `result_document_id` linkage.

**Changed from those:** status model collapses to `Open → Done | Rejected`; classification moves from rule-based to LLM-enriched; clustering moves from per-event hashing to topic-scoped; UI rewritten around action-verb titles and inline drafts; topics gain `cluster_key`/`canonical_title`/enrichment timestamps; suggestions gain explicit versioning.

---

## 1. Problem

The Coverage Gaps page exists to surface support failures and turn them into knowledge-base improvements. In production we observe two failures of the current implementation:

1. **The inbox is unreadable.** Most gaps land in `needs_review` because the rule-based classifier (`server/internal/service/support_coverage_rules.go`) falls back to it whenever an event lacks an `issue_key`. The most common signal — `human_reply_after_ai` — almost always produces a `needs_review` gap with a generic title.
2. **Each ticket becomes its own row.** Dedupe is `hashExcerpt(summary)` — SHA-256 of lowercased+trimmed text. Slight wording variations of the same problem produce separate gaps.

Combined: writers stop opening the page. The feature does not produce its intended outcome (published articles that prevent future tickets).

## 2. Goals

Optimize for one outcome: **a published or updated KB article that demonstrably reduces future support failures on the same topic.** Success metrics:

- Number of articles shipped from gaps per week.
- Median time from gap surfaced → article published.
- Reduction in evidence on a topic in the 30 days after an article ships.

Non-goals for v1 (scoped to Phase 2-4):

- Data gaps and Action gaps (the other two categories — Phase 3, Phase 4).
- Outcome dashboards (Phase 2). v1 still **persists the data needed** for them.
- Inline surfacing inside the docs editor (Phase 2).

## 3. Target users

The feature serves a **support team as a unit**.

- **Solo / small team:** one person wears the docs writer, support manager, and AI ops hats. Wants a single ranked to-do list.
- **Larger orgs (15+):** distinct content owner, support manager, automation owner. Filter chips let each role focus on their lane without fragmenting the data model.

Persona D from brainstorming: serve all sizes via one shared model with role-flavored filters.

## 4. Three-category model (architectural target)

| Category | Cause | Fix path | v1? |
|---|---|---|---|
| **Content gap** | KB content missing, weak, outdated, or contradictory | Write/edit a Doc | ✅ |
| **Data gap** | AI lacked account/customer context | Connect a data source | ❌ Phase 3 |
| **Action gap** | AI couldn't perform an action the user wanted | Create/extend an agent or automation tool | ❌ Phase 4 |

**v1 ships content gaps only**, but the data model (`gap_kind` column), classifier interface, and UI components must accommodate all three from day one.

## 5. Conceptual architecture

```
Support events  ──►  Topic clusterer  ──►  Coverage gap upsert + evidence row
                                                    │
                            ┌───────────────────────┼───────────────────────┐
                            ▼                       ▼                       ▼
                     Daily batch enricher     Spike trigger          Manual regenerate
                     (03:00 UTC)              (≥5 events / hour      (UI button,
                                              per topic)              POST .../regenerate)
                            │                       │                       │
                            └───────────────────────┼───────────────────────┘
                                                    ▼
                                  CoverageGapEnrichmentWorkflow (Temporal)
                                  → LLM call (canonical title, gap subtype,
                                    route decision, draft markdown)
                                  → Writes new SupportGapSuggestion (active),
                                    supersedes the prior active one.
                                                    │
                                                    ▼
                                        Coverage gaps inbox
                                        (impact-ranked, filterable)
                                                    │
                                                    ▼
                                        User: Reject | Add ▾ | Open in editor
```

## 6. Components

### 6.1 Signal layer (existing, unchanged in v1)

`SupportEvent` ingestion in `server/internal/service/support_coverage.go` continues unchanged. Seven existing event types still feed gap creation. No new event types in v1.

### 6.2 Topic clusterer (new)

Replaces per-event `hashExcerpt` dedupe with topic-scoped clustering. Assigns each incoming event to a `support_coverage_topics` row.

**v1 implementation (no LLM, no embeddings):**

- Normalize `issue_summary`: lowercase → strip punctuation → remove stopwords → token-sort.
- Compose `cluster_key` to **prevent false merges across signal types or unrelated articles**:

  ```
  cluster_key = sha256(
    workspace_id              || '|' ||
    signal_type               || '|' ||   // event type — keeps article_feedback separate from human_reply_after_ai
    coalesce(document_id, '') || '|' ||   // article-feedback gaps scope to their article
    coalesce(issue_key, '')   || '|' ||   // when present, anchors to known issue taxonomy
    normalized_token_sort(issue_summary)
  )
  ```

- Lookup or create a `support_coverage_topics` row keyed by `(workspace_id, cluster_key)`. The first-evidence summary becomes a placeholder `title`. The enricher later sets `canonical_title`.
- Lookup-or-create a `support_coverage_gaps` row for that topic (`one gap per topic`). Increment `evidence_count`. Append the new `support_gap_evidence` row.

**Cluster precision is a known v1 risk** — see Section 8 for the embedding-based upgrade path.

### 6.3 Coverage gap upsert (modified)

`SupportCoverageGap` now represents **one gap per topic per workspace**. Existing fields (`evidence_count`, `first_seen_at`, `last_seen_at`, `confidence`) keep their meaning but `evidence_count` is **all-time** (see 6.5 for 30-day variant used in ranking).

A new column `gap_kind text not null default 'content'` is added — exists to support Phase 3/4.

The legacy `dedupe_key` column is retained but deprecated; new rows write the same value as the topic's `cluster_key` for backward compat. A **partial** unique index `(workspace_id, topic_id) WHERE status='open'` enforces one open gap per topic. After a gap is `Done` or `Rejected`, future re-emergence on the same topic creates a new open gap — intentional, see lifecycle §6.6.

### 6.4 Enricher (new — async Temporal workflow)

A new `CoverageGapEnrichmentWorkflow` (registered alongside existing CRM workflows in `internal/temporalapp/`) runs on three triggers:

| Trigger | Workflow ID | Cooldown |
|---|---|---|
| Daily batch | `coverage-gap-batch-{workspace_id}-{YYYYMMDD}` | One per workspace per day |
| Spike | `coverage-gap-enrich-{topic_id}` | Reused; `cooldown_until` field on topic suppresses re-fire within 1 hour |
| Manual regenerate | `coverage-gap-enrich-{topic_id}-{ulid}` | 30 sec UI debounce + reuses topic-level cooldown |

**Idempotency** is enforced by Temporal's workflow-ID dedupe + the topic-level `cooldown_until` and `last_enriched_at` columns. Concurrent triggers for the same topic produce one enrichment.

**Activity:** `EnrichTopicActivity(ctx, topic_id)` — single-purpose, retryable per Temporal defaults (max 3 attempts, exponential backoff). The activity:

1. Loads topic + last 30 days of evidence (capped at top 20 by `last_seen_at`).
2. Loads workspace KB context: titles of all published articles + a token-similarity top-5 of "potential update targets."
3. Calls `internal/llm/` provider (same pattern as `crm_signal_detection.go`) with a structured prompt.
4. Parses the structured response: `{canonical_title, gap_subtype, route, target_document_id, draft_markdown, confidence}`.
5. Writes new `SupportGapSuggestion` with `is_active=true`. Sets prior active suggestions for the same gap to `is_active=false, superseded_at=now()`.
6. Updates topic: `canonical_title`, `last_enriched_at=now()`, `cooldown_until=now() + 1h`.

**Spike trigger source:** the existing `ProcessSupportEvent` flow checks after each evidence insert — `support_gap_evidence` has no `topic_id` column, so the count is taken via the gap (one gap per open topic in v2):

```sql
SELECT COUNT(*) FROM support_gap_evidence
 WHERE gap_id = <gap_id of the row we just inserted into>
   AND created_at > now() - interval '1 hour'
```

If the count `>= 5` and `cooldown_until < now()` on the topic, signal the workflow. The "5 events on a topic in an hour" semantic holds because v2 enforces one open gap per topic (see 7.2 partial unique index).

**Daily batch source:** a Temporal cron schedule registered at startup (UTC 03:00 — workspace-local times deferred to Phase 2). The batch lists topics with `last_enriched_at < now() - interval '24 hours'` and `evidence_count >= 2`, fans out to `EnrichTopicActivity`.

**Manual regenerate endpoint:** `POST /api/support/coverage/gaps/{gap_id}/regenerate` — handler validates permissions, ignores cooldown for manual fires (still respects the 30-sec UI debounce), enqueues workflow.

**Temporal is required.** This matches the existing CRM workflows pattern — `internal/config/config.go` reads `TEMPORAL_ADDRESS` (default `localhost:7233`) and the API server connects at startup. We do not build a synchronous in-process fallback for v1; if Temporal is unreachable, manual regenerate returns 503 and the daily batch logs an error. If we later need a no-Temporal mode for development convenience, that's a separate cross-cutting change, not coverage-gaps-specific.

### 6.5 Impact ranking (read-time)

Each gap gets an impact tier computed at read time from the **last-30-day** evidence count, **not** the all-time `evidence_count` column:

```sql
SELECT g.*,
  (SELECT COUNT(*) FROM support_gap_evidence e
   WHERE e.gap_id = g.id AND e.created_at > now() - interval '30 days') AS evidence_30d
FROM support_coverage_gaps g
WHERE g.workspace_id = ? AND g.status = 'open'
ORDER BY <impact tier desc>, g.last_seen_at desc
LIMIT 50;
```

Tiers (global v1 constants):

- **High:** `evidence_30d ≥ 10`
- **Medium:** `evidence_30d ≥ 3`
- **Low:** `evidence_30d < 3`

Gaps with `evidence_30d == 1` are hidden by default (filterable via "Show all"). The 30-day window matches Intercom's reporting window.

### 6.6 Lifecycle (modified)

State machine:

```
                  ┌── Add ──────────────► [Done] (terminal)
                  │   (commits doc + closes gap)
                  │
       [Open] ────┼── Reject ──────────► [Rejected] (terminal)
                  │
                  └── Open in editor ──► [Open] (no transition; gap closes
                                         via Add when the editor saves)
```

**Manual close only** in v1 — no auto-close. Precise transition rules:

- **Add** is the explicit close action. Behavior depends on the active suggestion's `route`:
  - `create_article`: backend calls `DocsDocumentService.Create` (which defaults to `DocStatusDraft` per `docs_document.go:83`) with `draft_markdown` as the body. The user finishes/publishes from the docs editor. Gap-side: `gap.status='done'`, `gap.closed_at=now()`, `gap.result_document_id=<new>`, snapshots `gap.closed_evidence_count = evidence_30d at close time`.
  - `update_article`: backend appends `draft_markdown` as a new section to the target Doc. Same gap-side bookkeeping with `result_document_id=<existing>`.
- **Reject** sets `gap.status='rejected'`, `gap.closed_at=now()`, `gap.rejection_reason` (optional free text). The gap leaves the inbox; future evidence on the same `cluster_key` re-opens nothing — it accumulates on the rejected gap until 30 days pass and the gap drops out of evidence_30d, OR a future re-enrichment of the topic creates a new gap if the rejected gap is older than 30 days. (Net effect: rejection is "ignore for ~30 days," not "ignore forever." Documented and intentional.)
- **Open in editor** opens the docs editor at a new in-memory route loaded with `draft_markdown` as initial TipTap content + the `gap_id`/`suggestion_id` carried in URL state. The gap stays `Open`. When the editor saves, it calls the same Add backend (which closes the gap). If the user navigates away without saving, nothing changes.

**Migration of existing statuses:**

- `drafted` → `open` (it's not a terminal state in the new model; the suggestion just exists)
- `fixed` → `done`
- `ignored` → `rejected`
- `open` → `open`

This collapses the old four-state model into three states. The `drafted`/`fixed` semantics in `support_coverage_drafts.go:98,213,268` are removed — drafts no longer change gap status. Gap status changes only on user action.

### 6.7 UI (rewritten)

Three-column layout matching the validated Intercom reference:

- **Left:** workspace nav (existing sidebar).
- **Middle (gap list):**
  - Header: "Most impactful ways to improve coverage" + count.
  - Filter chips: `CONTENT GAPS` (only one in v1, rendered so the pattern lands now) + `STATUS: Open / Done / Rejected` segmented control.
  - Sort: Impact (default) | Recent.
  - Row: category badge · action-verb title (`canonical_title`) · preview snippet · impact tier badge.
  - Empty state: "Nothing to fix right now."
- **Right (detail pane):**
  - Top: recommendation card — title, evidence-30d count, last-seen, **Reject** + **Add ▾** buttons.
  - Add button label exposes routing: `Add to "Password reset basics" ▾` or `Create new article ▾`.
  - Dropdown options: `Create new article instead` / `Add to a different article…` / `Open in editor`.
  - Body: inline Markdown preview of active suggestion's `draft_markdown`.
  - Below: Evidence list (existing). No internal scroll (per current single-page-scroll decision).
- **Manual regenerate** button on detail pane; disables for 30 sec after click; calls `POST .../regenerate`.

Page itself uses single-scroll layout already implemented in `routes/_authenticated/w/$slug/support/coverage.tsx`.

## 7. Data model changes

All via dbmigrate (`server/internal/dbmigrate/sql/`).

### 7.1 `support_coverage_topics` — expand

```sql
ALTER TABLE support_coverage_topics
  ADD COLUMN cluster_key text,
  ADD COLUMN canonical_title text,
  ADD COLUMN last_enriched_at timestamptz,
  ADD COLUMN cooldown_until timestamptz;

-- Drop the issue_key uniqueness; add cluster_key uniqueness (cluster_key may
-- be null for legacy rows during migration window).
DROP INDEX IF EXISTS idx_support_coverage_topics_workspace_issue_key;
CREATE INDEX idx_support_coverage_topics_workspace_issue_key
  ON support_coverage_topics(workspace_id, issue_key)
  WHERE issue_key <> '';

CREATE UNIQUE INDEX idx_support_coverage_topics_workspace_cluster_key
  ON support_coverage_topics(workspace_id, cluster_key)
  WHERE cluster_key IS NOT NULL;
```

### 7.2 `support_coverage_gaps` — expand

```sql
ALTER TABLE support_coverage_gaps
  ADD COLUMN gap_kind text NOT NULL DEFAULT 'content',
  ADD COLUMN closed_at timestamptz,
  ADD COLUMN closed_evidence_count int,
  ADD COLUMN result_document_id uuid,
  ADD COLUMN rejection_reason text;

-- Partial unique: one open gap per topic. Done/Rejected gaps don't block
-- a future re-emergence creating a new open gap on the same topic.
CREATE UNIQUE INDEX idx_support_coverage_gaps_workspace_topic_open
  ON support_coverage_gaps(workspace_id, topic_id)
  WHERE status = 'open' AND topic_id IS NOT NULL;
```

`result_document_id` is the canonical link from gap → produced doc, used by Phase 2 deflection reporting.

### 7.3 `support_gap_suggestions` — versioning

```sql
ALTER TABLE support_gap_suggestions
  ADD COLUMN is_active boolean NOT NULL DEFAULT true,
  ADD COLUMN superseded_at timestamptz;

CREATE INDEX idx_support_gap_suggestions_gap_active
  ON support_gap_suggestions(gap_id)
  WHERE is_active;
```

When the enricher writes a new suggestion, prior `is_active=true` rows for the same `gap_id` are flipped to `is_active=false, superseded_at=now()`. UI queries filter on `is_active=true`.

### 7.4 Status migration (one-shot)

```sql
UPDATE support_coverage_gaps SET status = 'done'     WHERE status = 'fixed';
UPDATE support_coverage_gaps SET status = 'rejected' WHERE status = 'ignored';
UPDATE support_coverage_gaps SET status = 'open'     WHERE status = 'drafted';
```

Old enum values are kept in the type definition for one release cycle, then dropped in a follow-up migration.

### 7.5 Cluster rebuild (one-shot, destructive)

For every `open` gap in every workspace:

1. Recompute `cluster_key` for the gap's first evidence row using the new composition rule.
2. Look up or create a topic with that `cluster_key`.
3. If no other open gap is bound to that topic, point the gap at it.
4. If another open gap is already bound, **merge**: the gap with the higher `evidence_count` wins; reattach evidence rows from the loser; `loser.status='rejected'`, `loser.rejection_reason='merged into <winner_id> during cluster rebuild 2026-04-27'`.

Done-and-rejected gaps are not touched. Run as a separate dbmigrate file with explicit per-workspace `slog.InfoContext` logging (`workspace_id`, `gaps_merged`, `topics_created`).

### 7.6 No new tables in v1.

## 8. Deferred decisions (called out so we don't bake them in)

- **Embedding-based clustering.** Token-sort hash will fail on semantically equivalent but lexically distinct phrasings ("can't log in" vs. "trouble signing in"). When v1 ships and we measure cluster precision on real data, choose between (a) tighter normalization rules, or (b) pgvector embeddings. Reserved column `embedding vector(1536)` may be added then — not now.
- **Workspace-local times.** Daily batch runs at UTC 03:00 in v1. Per-workspace timezones deferred to Phase 2.
- **Outcome dashboards.** Data is captured (`closed_evidence_count`, `result_document_id`, `closed_at`); the report UI is Phase 2.
- **Inline KB editor surfacing.** Out of scope.

## 9. Phasing

- **Phase 1 (this spec):** Content gaps — clusterer, enricher, redesigned UI, manual lifecycle, outcome data captured.
- **Phase 2:** Outcome dashboards (30-day deflection per topic), inline surfacing on KB articles, workspace-local batch times.
- **Phase 3:** Data gaps — new event type emitted by AI agent when it lacks customer context. Fix path: data connector setup.
- **Phase 4:** Action gaps — new event type emitted when AI rejects an action. Fix path: agent/automation creation.

The architectural commitments in v1 (`gap_kind` column, three-category UI scaffold, three-category enrichment interface) are present so Phases 3 and 4 are additive.

## 10. Error handling and edge cases

- **LLM enrichment failure:** topic remains with prior active suggestion (or none). Workflow logs failure, retries per Temporal defaults, then surfaces "Regenerate" affordance in UI. `ErrorContext` log includes `workspace_id`, `topic_id`.
- **No suggestion yet (newly created topic before first enrich):** detail pane shows "Generating recommendation…" with a progress indicator and offers immediate manual regenerate.
- **No evidence on a gap:** can't happen in v1 (gap is created from evidence). Defensive: gaps with `evidence_count == 0` are hidden everywhere.
- **Empty workspace KB:** enrichment falls back to `route = create_article` with no `update_article` candidate.
- **Spike trigger storm:** per-topic `cooldown_until` (1 hour) prevents repeated re-enrichment during a sustained spike. Daily batch is the catch-up.
- **Doc deleted after Add:** gap stays `Done`. We do not re-open. Future evidence on the same topic accumulates against the closed gap until 30-day window expires; new evidence after that window may surface as a new gap on the same topic if no open gap exists.
- **Concurrent Add and Reject (two users):** first write wins; second receives 409 with current state. UI shows toast "This gap was already resolved by {user}."
- **Add when no active suggestion exists:** Add button is disabled until enrichment produces one. Tooltip explains.
- **User edits draft in TipTap then closes without saving:** no state change. Suggestion remains the system's stored draft; user's in-flight edits are lost. (Acceptable for v1 — autosave is Phase 2.)

## 11. Testing

- **Unit tests** (table-driven) for the clusterer normalizer: input variants → expected `cluster_key`, including signal-type and document-scoping cases that must NOT collide.
- **Unit tests** for impact tier computation, including the 30-day boundary.
- **Service tests** for the enricher with a mocked LLM provider: success, retry, cooldown skip, supersede prior active suggestion.
- **Service tests** for the lifecycle transitions (Add `create_article`, Add `update_article`, Reject, concurrent close).
- **Integration tests** for the full ingest → cluster → enrich → list → Add pipeline against in-memory SQLite (with a stub Temporal activity runner).
- **Migration tests** for status backfill and cluster rebuild on a fixture dataset (verify merge correctness, evidence reattachment, no data loss).
- **Handler test** for `POST .../regenerate` (auth check, cooldown handling, debounce semantics).

## 12. Rollout

**No per-workspace feature flag.** Helpin's existing `featureFlags.ts` is an email-allowlist module gate (the Support module is already restricted to four internal emails). Building per-workspace runtime flag infra purely for this redesign is disproportionate.

Plan:

1. Land backend changes (clusterer, enricher, schema, lifecycle) behind no flag — they are additive to the data model and inert until the new write paths run.
2. Run dbmigrate: schema additions → status migration → cluster rebuild.
3. Cutover: switch `ProcessSupportEvent` to use the new clusterer + topic-scoped gap upsert.
4. Ship the new UI in the same release. The route already gates the whole Support module to internal emails, so blast radius is small.
5. Keep the old `support_coverage_drafts.go` apply paths reachable for one release in case rollback is needed; remove in the follow-up release.

If we later open the Support module to external customers and want safer per-customer cutover, we add a workspace-settings flag at that point — independent of this redesign.

---

## Open questions to validate before implementation

1. ~~Does the docs module support server-side draft Docs?~~ **Resolved v1.2** — `DocsDocumentService.Create` defaults to `DocStatusDraft` (`server/internal/service/docs_document.go:83`). Add for `create_article` lands as a draft; user publishes from the editor.
2. What's the existing pattern for registering Temporal cron workflows at startup? Confirm parallel registration with the existing CRM workflows in `internal/temporalapp/`.
3. LLM cost budget per workspace per day. Estimated ~50 calls/day for an active workspace; needs a real number from finance/ops before launch.
4. ~~Confirm `support_gap_evidence` carries `created_at`.~~ **Resolved v1.2** — present at `server/internal/model/support_coverage.go:152`; the 30-day computation in §6.5 works without backfill.
