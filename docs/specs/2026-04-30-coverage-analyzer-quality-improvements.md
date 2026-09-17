# Coverage analyzer v3.1 quality improvements

**Date:** 2026-04-30
**Status:** Historical approved design; current analyzer behavior has evolved.
**Author:** Waqar + Claude

## Current implementation review — 2026-09-18

This design records April quality findings for contributors. The production samples
and proposed thresholds below are historical, not current measurements.

The [analyzer](../../server/internal/service/support_coverage_daily_analyzer.go)
is now version `v4`. It retains `HasHumanReply`, the prompt instruction, and the
programmatic “No human response observed” override. The original no-version-bump
statement does not describe today's deduplication key, which includes analyzer
version as well as transcript hash.

The current knowledge relevance floor is **0.015**, not 0.1. The source explains
that these are reciprocal-rank fusion scores: a lexical-only top hit can be around
0.016, while a lexical-plus-vector hit can be around 0.033. The original inference
that a 0.016 score necessarily means irrelevant noise is therefore invalid for
this scoring system. Scores alone do not establish answer quality.

Current `recommendationRowsForFinding` links target articles and records
recommendations without automatically calling the proposed draft-creation helper.
Do not interpret the historical human-reply gate as the complete current policy
for every draft-generation path. Other suggestion workflows require their own
source review.

[Event processing](../../server/internal/service/support_coverage.go) can attach
human-resolution evidence to an existing gap first, then checks completed analysis
history before creating a new gap. It does not discard all such evidence once an
analyzer has run. Semantic grouping is now implemented in the
[materializer](../../server/internal/service/support_coverage_materializer.go),
so the “deferred” section is historical. Embedding failure can fall back to lexical
grouping; see the [updated diagnosis](../plans/2026-09-02-coverage-gaps-diagnosis.md)
for execution and recovery limits.

The [legacy closure migration](../../server/internal/dbmigrate/sql/20260430202658586984_close_v1_legacy_gaps.sql)
exists. Its predicate checks source signal, unknown category, missing topic, and
open status. It does **not** explicitly test absence of explanations or
recommendations, so “only closes gaps that have zero actionable content” is a
stronger claim than the SQL proves. No live migration, cleanup, reanalysis, or
runtime tests were performed during this review.

## Original April design

## Problem Statement

Production data from two active workspaces reveals five systemic issues in the daily coverage gap analyzer:

1. **Hallucinated article suggestions** — When no human has replied, the LLM invents generic content (e.g., a 30-day e-commerce refund policy for a SaaS product with a 7-day window).
2. **Wasted LLM cost on junk knowledge matches** — Every gap runs a refinement LLM pass even when all candidates score ~0.016 and are irrelevant.
3. **Speculative human_resolution** — The LLM writes "a human agent needs to..." when no human has replied, misleading workspace owners.
4. **30 noisy v1 legacy gaps** — Generic "Human resolved after AI failure" gaps with no topic, no explanation, confidence 0.3.
5. **Duplicate gaps** — Semantically identical gaps with different wording not deduped (deferred — low frequency, merge button exists).

## Evidence

Analyzed 4 gap details from workspace `e03065c1`:

| Gap | Issue | human_resolution | Knowledge scores |
|-----|-------|-----------------|-----------------|
| Meta phishing | Not Contentpen's product | Speculative | All ~0.016 |
| YouTube features | Legitimate but speculative | Speculative | All ~0.016 |
| Paddle dispute | Excellent — real resolution | Observed | All ~0.016 |
| Refund policy | Hallucinated article content | Speculative | All ~0.015 |

Common patterns:
- `ai_turn_count=0` on all four — AI never attempted a response
- Knowledge matcher returns 8 irrelevant candidates every time
- Article drafts generated even when no human has resolved the conversation
- 30 v1 gaps ("Human resolved after AI failure") pollute the page

---

## Improvement 1: Gate Article Suggestions on Observed Human Reply

### Rule
Don't generate article draft suggestions when the conversation has no observed human resolution.

### Detection
In `BuildCoverageConversationAnalysisInputForSegment`, compute `HasHumanReply` by checking if any message in the segment has `sender_type == "user"` and is not internal/system.

### Changes

**`support_coverage_daily_analyzer.go`:**
- Add `HasHumanReply bool` field to `CoverageConversationAnalysisInput`
- Compute it in `BuildCoverageConversationAnalysisInputForSegment` by scanning the final `analysisMessages` for any `sender_type == "user"`
- In `runConversationCoverageAnalysis`, pass `HasHumanReply` through to `CoverageFindingUpsertInput`
- In `recommendationRowsForFinding`, at the `isCoverageDocsFix(fix)` block, split the logic:
  - Always run `LinkGapArticle` when `fix.TargetID` is present (preserves article linking)
  - Only call `createDocsSuggestionForFix` when `input.HasHumanReply` is true
  - This also avoids the `GenerateKnowledgeSuggestion` LLM call, saving one LLM invocation per gap without human resolution

### What is preserved
- Gap is still detected and created
- Recommendations still appear (create article, improve workflow, etc.)
- Article linking (`LinkGapArticle`) still works even without human reply
- User can manually generate a suggestion later once the conversation is resolved
- Only the auto-generated article draft is suppressed

### Tests
- Test that `BuildCoverageConversationAnalysisInput` sets `HasHumanReply=true` when user messages exist
- Test that `HasHumanReply=false` when only customer + AI messages exist
- Test that suggestion is not created when `HasHumanReply=false` (requires DB test)

---

## Improvement 2: Skip Knowledge Refinement on Low-Score Matches

### Rule
Don't call `RefineFixBundleWithKnowledge` when the best knowledge candidate scores below 0.1.

### Rationale
Production data shows all candidates at ~0.015-0.016 — random noise from vector search. Relevant matches typically score 0.3+. Threshold of 0.1 gives generous buffer.

### Changes

**`support_coverage_daily_analyzer.go`:**
- Add constant `coverageKnowledgeMinRelevanceScore = 0.1`
- In `matchCurrentKnowledgeForAnalysis`, after getting results: if `len(candidates) > 0 && candidates[0].CombinedScore < coverageKnowledgeMinRelevanceScore`, return `nil, nil`
- Candidates are already sorted by score descending, so checking first is sufficient
- Log skip: `slog.InfoContext(ctx, "skipping coverage knowledge refinement: best candidate below threshold", "best_score", candidates[0].CombinedScore, "threshold", coverageKnowledgeMinRelevanceScore)`

### Impact
- Saves one LLM call per gap with irrelevant knowledge (100% of observed gaps in production)
- Original analyzer recommendations pass through unchanged

### Tests
- Test that `matchCurrentKnowledgeForAnalysis` returns nil when best score is below threshold
- Verify refinement is not called when no candidates returned

---

## Improvement 3: Require Observed Resolution in LLM Prompt

### Rule
The LLM must distinguish between what a human actually did vs. what it thinks a human should do.

### Changes

**`support_coverage_daily_analyzer.go`:**
- Add to `coverageConversationAnalysisSystemPrompt` conciseness section:

```
If no message from a human agent (sender_type=user) appears in the conversation,
set human_resolution to "No human response observed" exactly.
Do not speculate what a human agent would or should do.
```

- The `HasHumanReply` field (from Improvement 1) is included in the LLM input, making the instruction unambiguous

### Defense-in-depth: programmatic override
In `runConversationCoverageAnalysis`, after `AnalyzeConversation` returns and `normalizeCoverageConversationAnalysisResult` runs, add a post-normalization guard:

```go
if !input.HasHumanReply && result.HumanResolution != "No human response observed" {
    result.HumanResolution = "No human response observed"
}
```

This runs on the normalized result before it's stored or passed to `UpsertFinding`, ensuring both the analysis record and evidence metadata get the corrected value. No changes to the normalizer signature needed.

---

## Improvement 4: Auto-Close V1 Legacy Gaps + Stop Creating New Ones

### Part A: Migration — close existing v1 gaps

**New migration file:** `server/internal/dbmigrate/sql/YYYYMMDDNNNN_close_v1_legacy_gaps.sql`

```sql
UPDATE support_coverage_gaps
SET status = 'done',
    closed_at = NOW(),
    updated_at = NOW()
WHERE source_signal IN ('human_reply', 'conversation_resolved_by_human')
  AND gap_category = 'unknown'
  AND topic_id IS NULL
  AND status = 'open';
```

Idempotent — safe to re-run.

### Part B: Stop creating new v1 gaps for analyzed workspaces

**`support_coverage.go` → `ProcessSupportEvent`:**
- Only suppress the two noisy human-resolution signals: `human_reply_after_ai` and `conversation_resolved_by_human`. Other v1 event types (article feedback, widget no-result searches, docs issue feedback, AI handoffs) must continue — the daily analyzer is conversation-based and does not replace these.
- Before creating a gap from one of these two signals, check if the workspace has at least one completed `SupportCoverageAnalysisRun`
- If yes, skip gap creation for that signal — the daily analyzer will find the same conversation with proper LLM analysis
- If no (new workspace, no daily cron yet), preserve v1 behavior as a fallback

**Repository method needed:** `HasCompletedAnalysisRun(ctx, workspaceID) (bool, error)` on `SupportCoverageRepository` (not `SupportCoverageAnalysisRepository`) — simple existence check on `support_coverage_analysis_runs` with `status = 'completed'` and matching workspace_id. This avoids injecting a second repository into `SupportCoverageService`.

### What this preserves
- V1 gap creation for workspaces not yet covered by the daily analyzer
- All existing evidence rows (only gap status changes, evidence is untouched)
- The "done" status is visible in the UI if someone filters for closed gaps

---

## Improvement 5: Semantic Deduplication — Deferred

### Decision
Defer. The duplicate rate is low (1 pair in 7 gaps). The merge button exists in the UI. Automated dedup risks false merges. Revisit if duplicate rate increases.

---

## Version Bump

**Not bumped.** Adding `HasHumanReply` does change the JSON input sent to the LLM, but the transcript hash is unchanged so `AlreadyAnalyzedConversation` will skip previously analyzed conversations. This is intentionally future-only — new conversations and re-opened conversations get the improved behavior; existing v3 analyses are not re-run.

New and changed conversations (new messages, reopened threads) will pick up the improved behavior naturally since their transcript hash changes. Conversations with unchanged v3 transcript hashes will still be skipped by `AlreadyAnalyzedConversation`, even via the reanalyze button.

For already-created hallucinated drafts, a targeted one-time cleanup remains an option if needed: close suggestions where the linked analysis has no user-type messages. These are all in "draft" status (not published), so the blast radius is low. This can be a follow-up.

---

## Files Modified

| File | Changes |
|------|---------|
| `server/internal/service/support_coverage_daily_analyzer.go` | HasHumanReply field, knowledge score threshold, prompt update |
| `server/internal/service/support_coverage_daily_analyzer_test.go` | Tests for HasHumanReply, knowledge threshold |
| `server/internal/service/support_coverage.go` | V1 gap creation guard for analyzed workspaces |
| `server/internal/repository/support_coverage.go` | `HasCompletedAnalysisRun` method |
| `server/internal/dbmigrate/sql/` | V1 legacy gap closure migration |

## Tests for Improvement 4B

- Test that v1 gap creation is skipped when a completed analysis run exists for the workspace
- Test that v1 gap creation proceeds when no analysis run exists for the workspace

## Note on Improvement 2 Side Effect

When `matchCurrentKnowledgeForAnalysis` returns nil due to low scores, `matchedKnowledge` in `runConversationCoverageAnalysis` remains as the original trace-derived candidates (from `coverageKnowledgeCandidatesFromTraceInput`). These are stored in evidence metadata. This is intentional — the trace candidates reflect what the AI actually searched during the conversation, regardless of whether current knowledge refinement runs.

## Risk Assessment

All changes are additive guards — no existing behavior is removed for workspaces without analysis history. The migration is idempotent and only closes gaps that have zero actionable content.
