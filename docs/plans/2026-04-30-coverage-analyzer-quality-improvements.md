# Coverage analyzer quality improvements implementation plan

This historical plan records quality guards proposed for the v3.1 coverage
analyzer. Use it for the reasoning behind those guards; the current analyzer and
materialization pipeline have evolved beyond the snippets below.

## Source review — 2026-09-18

- [The analyzer](../../server/internal/service/support_coverage_daily_analyzer.go)
  uses version `v4` and a three-hour cron, despite the historical “daily” name.
  Its knowledge-score floor is **0.015**, on reciprocal-rank-fusion scores, not
  the proposed 0.1 cosine-like assumption. Do not reuse the old production-score
  rationale as evidence for the present threshold.
- `HasHumanReply` is computed from the retained analysis messages after the
  80-message cap. It means a human reply is visible in that input, not that the
  full conversation never had a human response. The no-response override updates
  both the result and its marshaled raw output.
- The old `CoverageFindingUpsertInput`, `recommendationRowsForFinding`, and
  `createDocsSuggestionForFix` implementation points no longer exist in the
  inspected service sources. Findings now pass through
  [materialization](../../server/internal/service/support_coverage_materializer.go),
  with human-reply information retained in analysis metadata. The proposed
  article-draft gate must not be claimed as a current guard merely from this plan.
- [The coverage event service](../../server/internal/service/support_coverage.go)
  enriches an existing gap before suppressing new gaps for the two human-response
  signals when a completed analysis run exists. A lookup failure is logged and
  does not itself suppress creation.
- The actual [legacy-gap migration](../../server/internal/dbmigrate/sql/20260430202658586984_close_v1_legacy_gaps.sql)
  filters source signal, unknown category, absent topic, and open status. It does
  **not** check confidence or absence of recommendations/explanation despite the
  historical descriptive comments. This review did not execute a migration.
- Go 1.24, no-version-bump rationale, and expected test results below are dated
  implementation assumptions; `server/go.mod` currently declares Go 1.25.0.

## Original implementation plan

**Goal:** Fix five quality issues in the daily coverage gap analyzer: hallucinated article suggestions, wasted LLM calls on junk knowledge matches, speculative human_resolution, noisy v1 legacy gaps, and v1 gap creation for analyzed workspaces.

**Architecture:** Four targeted guards added to the existing analyzer pipeline. No schema changes, no version bump. One dbmigrate SQL migration to close legacy gaps. Changes are additive — no existing behavior removed for workspaces without analysis history.

**Tech Stack:** Go 1.24, GORM (PostgreSQL/SQLite for tests), existing `support_coverage_daily_analyzer.go` pipeline, `dbmigrate` migration system.

**Spec:** `docs/specs/2026-04-30-coverage-analyzer-quality-improvements.md`

---

## Files

- Modify: `server/internal/service/support_coverage_daily_analyzer.go`
  - Add `HasHumanReply` to input struct and builder
  - Add knowledge score threshold constant and guard
  - Update prompt with observed-resolution instruction
  - Post-normalization guard for speculative human_resolution
  - Pass `HasHumanReply` through to `CoverageFindingUpsertInput`
  - Gate `createDocsSuggestionForFix` on `HasHumanReply`, preserve `LinkGapArticle`

- Modify: `server/internal/service/support_coverage_daily_analyzer_test.go`
  - Tests for `HasHumanReply` computation
  - Tests for knowledge score threshold

- Modify: `server/internal/service/support_coverage.go`
  - Guard v1 gap creation for `human_reply_after_ai` and `conversation_resolved_by_human` signals

- Modify: `server/internal/repository/support_coverage.go`
  - Add `HasCompletedAnalysisRun` method

- Create: `server/internal/dbmigrate/sql/202604300020_close_v1_legacy_gaps.sql`
  - Close noisy v1 gaps

---

### Task 1: Add HasHumanReply To Analyzer Input (TDD)

**Files:**
- Modify: `server/internal/service/support_coverage_daily_analyzer.go`
- Modify: `server/internal/service/support_coverage_daily_analyzer_test.go`

- [ ] **Step 1: Write test for HasHumanReply=true when user messages exist**

Add to test file:

```go
func TestCoverageConversationAnalysisInputSetsHasHumanReplyTrue(t *testing.T) {
	base := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)
	conversation := model.SupportConversation{ID: "c-1", WorkspaceID: "ws-1"}
	messages := []model.SupportMessage{
		{ID: "m-1", SenderType: "customer", MessageType: "reply", Content: "Help", CreatedAt: base},
		{ID: "m-2", SenderType: "ai", MessageType: "reply", Content: "Let me check", CreatedAt: base.Add(time.Minute)},
		{ID: "m-3", SenderType: "user", MessageType: "reply", Content: "Here is the fix", CreatedAt: base.Add(2 * time.Minute)},
	}

	input, err := BuildCoverageConversationAnalysisInput(conversation, messages, nil)
	if err != nil {
		t.Fatalf("BuildCoverageConversationAnalysisInput: %v", err)
	}
	if !input.HasHumanReply {
		t.Fatal("expected HasHumanReply=true when user messages exist")
	}
}
```

- [ ] **Step 2: Write test for HasHumanReply=false when only customer+AI messages**

```go
func TestCoverageConversationAnalysisInputSetsHasHumanReplyFalseWhenNoUserMessages(t *testing.T) {
	base := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)
	conversation := model.SupportConversation{ID: "c-1", WorkspaceID: "ws-1"}
	messages := []model.SupportMessage{
		{ID: "m-1", SenderType: "customer", MessageType: "reply", Content: "Help", CreatedAt: base},
		{ID: "m-2", SenderType: "ai", MessageType: "reply", Content: "Try this", CreatedAt: base.Add(time.Minute)},
	}

	input, err := BuildCoverageConversationAnalysisInput(conversation, messages, nil)
	if err != nil {
		t.Fatalf("BuildCoverageConversationAnalysisInput: %v", err)
	}
	if input.HasHumanReply {
		t.Fatal("expected HasHumanReply=false when no user messages exist")
	}
}
```

- [ ] **Step 3: Run tests and confirm they fail**

Run:

```bash
cd server
go test ./internal/service -run 'TestCoverageConversationAnalysisInputSetsHasHumanReply' -count=1
```

Expected: FAIL — `HasHumanReply` field does not exist on `CoverageConversationAnalysisInput`.

- [ ] **Step 4: Add HasHumanReply field to CoverageConversationAnalysisInput**

In `support_coverage_daily_analyzer.go`, add to the `CoverageConversationAnalysisInput` struct:

```go
HasHumanReply         bool                          `json:"has_human_reply"`
```

Add after the `SegmentResolved` field.

- [ ] **Step 5: Compute HasHumanReply in BuildCoverageConversationAnalysisInputForSegment**

In `BuildCoverageConversationAnalysisInputForSegment`, after building `analysisMessages` and before constructing the return struct, add:

```go
hasHumanReply := false
for _, message := range analysisMessages {
	if message.SenderType == "user" {
		hasHumanReply = true
		break
	}
}
```

Set `HasHumanReply: hasHumanReply` in the returned struct.

- [ ] **Step 6: Run tests and confirm they pass**

Run:

```bash
cd server
go test ./internal/service -run 'TestCoverageConversationAnalysisInputSetsHasHumanReply' -count=1
```

Expected: PASS.

---

### Task 2: Skip Knowledge Refinement On Low-Score Matches

**Files:**
- Modify: `server/internal/service/support_coverage_daily_analyzer.go`
- Modify: `server/internal/service/support_coverage_daily_analyzer_test.go`

- [ ] **Step 1: Add knowledge score threshold constant**

In the `const` block at the top of `support_coverage_daily_analyzer.go`, add:

```go
coverageKnowledgeMinRelevanceScore = 0.1
```

- [ ] **Step 2: Add score threshold guard in matchCurrentKnowledgeForAnalysis**

In `matchCurrentKnowledgeForAnalysis`, after the call to `s.knowledgeMatcher.MatchKnowledge(...)` and before returning, add:

```go
if len(candidates) > 0 && candidates[0].CombinedScore < coverageKnowledgeMinRelevanceScore {
	slog.InfoContext(ctx, "skipping coverage knowledge refinement: best candidate below threshold",
		"best_score", candidates[0].CombinedScore,
		"threshold", coverageKnowledgeMinRelevanceScore,
		"workspace_id", workspaceID,
	)
	return nil, nil
}
```

Candidates are already sorted by `CombinedScore` descending, so checking the first one is sufficient.

- [ ] **Step 3: Run existing analyzer tests to confirm no breakage**

Run:

```bash
cd server
go test ./internal/service -run 'TestSupportCoverageDailyAnalyzer_|TestCoverageConversationAnalysisInput' -count=1
```

Expected: PASS.

---

### Task 3: Require Observed Resolution In LLM Prompt And Post-Normalization Guard

**Files:**
- Modify: `server/internal/service/support_coverage_daily_analyzer.go`

- [ ] **Step 1: Add observed-resolution instruction to system prompt**

In `coverageConversationAnalysisSystemPrompt`, add after the existing conciseness instructions:

```
If no message from a human agent (sender_type=user) appears in the conversation,
set human_resolution to "No human response observed" exactly.
Do not speculate what a human agent would or should do.
```

- [ ] **Step 2: Add post-normalization guard in runConversationCoverageAnalysis**

In `runConversationCoverageAnalysis`, after the `AnalyzeConversation` call returns (which internally calls `normalizeCoverageConversationAnalysisResult`), after the `if result == nil { return false, nil }` check and before the `analysisStatus` assignment, add:

```go
if !input.HasHumanReply && result.HumanResolution != "No human response observed" {
	result.HumanResolution = "No human response observed"
}
```

This ensures both the analysis record and evidence metadata get the corrected value regardless of LLM compliance.

- [ ] **Step 3: Run focused tests**

Run:

```bash
cd server
go test ./internal/service -run 'TestSupportCoverageDailyAnalyzer_|TestCoverageConversationAnalysisInput|TestBuildLatestCoverageConversationSegment' -count=1
```

Expected: PASS.

---

### Task 4: Gate Article Suggestion On HasHumanReply

**Files:**
- Modify: `server/internal/service/support_coverage_daily_analyzer.go`

- [ ] **Step 1: Add HasHumanReply to CoverageFindingUpsertInput**

Add to the `CoverageFindingUpsertInput` struct:

```go
HasHumanReply bool
```

- [ ] **Step 2: Pass HasHumanReply from runConversationCoverageAnalysis to UpsertFinding**

In the `CoverageFindingUpsertInput` construction in `runConversationCoverageAnalysis`, add:

```go
HasHumanReply: input.HasHumanReply,
```

- [ ] **Step 3: Gate createDocsSuggestionForFix but preserve LinkGapArticle**

In `recommendationRowsForFinding`, find the `if isCoverageDocsFix(fix)` block. Replace:

```go
if isCoverageDocsFix(fix) {
	suggestion, err := s.createDocsSuggestionForFix(ctx, input, gapID, fix, metadata, now)
	if err != nil {
		return nil, err
	}
	row.SuggestionID = &suggestion.ID
	if strings.TrimSpace(fix.TargetID) != "" {
		if err := s.coverageRepo.LinkGapArticle(ctx, gapID, strings.TrimSpace(fix.TargetID), input.WorkspaceID); err != nil {
			return nil, fmt.Errorf("link recommendation article: %w", err)
		}
	}
}
```

With:

```go
if isCoverageDocsFix(fix) {
	if strings.TrimSpace(fix.TargetID) != "" {
		if err := s.coverageRepo.LinkGapArticle(ctx, gapID, strings.TrimSpace(fix.TargetID), input.WorkspaceID); err != nil {
			return nil, fmt.Errorf("link recommendation article: %w", err)
		}
	}
	if input.HasHumanReply {
		suggestion, err := s.createDocsSuggestionForFix(ctx, input, gapID, fix, metadata, now)
		if err != nil {
			return nil, err
		}
		row.SuggestionID = &suggestion.ID
	}
}
```

This preserves article linking always but only generates the draft suggestion when a human has actually replied.

- [ ] **Step 4: Update existing UpsertFinding test to set HasHumanReply**

The existing `TestSupportCoverageDailyAnalyzer_UpsertFindingCreatesGapEvidenceRecommendationsAndSuggestion` test constructs a `CoverageFindingUpsertInput` with docs-type fixes and expects `SuggestionID` to be set. After gating on `HasHumanReply`, the zero value (`false`) will skip suggestion creation and the test will fail. Add `HasHumanReply: true` to the test's `CoverageFindingUpsertInput` to preserve existing behavior.

- [ ] **Step 5: Run all focused analyzer tests**

Run:

```bash
cd server
go test ./internal/service -run 'TestSupportCoverageDailyAnalyzer_|TestCoverageConversationAnalysisInput|TestBuildLatestCoverageConversationSegment|TestCoverageSegmentTranscriptHash' -count=1
```

Expected: PASS.

---

### Task 5: Auto-Close V1 Legacy Gaps (Migration)

**Files:**
- Create: `server/internal/dbmigrate/sql/202604300020_close_v1_legacy_gaps.sql`

- [ ] **Step 1: Create migration file**

Use the dbmigrate CLI to generate the file with the correct timestamp:

```bash
cd server
go run ./cmd/migrate create close_v1_legacy_gaps
```

Note the generated filename — it will use the current timestamp (e.g., `202604301234_close_v1_legacy_gaps.sql`). Use that filename in subsequent steps.

- [ ] **Step 2: Write migration SQL**

```sql
-- Close noisy v1 legacy gaps that have no actionable information.
-- These were created by heuristic event rules (human_reply, conversation_resolved_by_human)
-- before the daily LLM analyzer was active. They have no topic, no analysis explanation,
-- no recommendations, and confidence 0.3-0.4.
UPDATE support_coverage_gaps
SET status = 'done',
    closed_at = NOW(),
    updated_at = NOW()
WHERE source_signal IN ('human_reply', 'conversation_resolved_by_human')
  AND gap_category = 'unknown'
  AND topic_id IS NULL
  AND status = 'open';
```

- [ ] **Step 3: Run migration tests**

Run:

```bash
cd server
go test ./internal/dbmigrate -count=1
```

Expected: PASS (migration is embedded and validates).

---

### Task 6: Stop Creating V1 Gaps For Analyzed Workspaces

**Files:**
- Modify: `server/internal/repository/support_coverage.go`
- Modify: `server/internal/service/support_coverage.go`

- [ ] **Step 1: Add HasCompletedAnalysisRun to SupportCoverageRepository**

Add to `support_coverage.go` repository:

```go
// HasCompletedAnalysisRun returns true if the workspace has at least one
// completed daily coverage analysis run, meaning the LLM analyzer is active
// and v1 heuristic gap creation can be suppressed for human-resolution signals.
func (r *SupportCoverageRepository) HasCompletedAnalysisRun(ctx context.Context, workspaceID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Table("support_coverage_analysis_runs").
		Where("workspace_id = ? AND status = ?", workspaceID, model.SupportCoverageAnalysisRunStatusCompleted).
		Limit(1).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check completed analysis run: %w", err)
	}
	return count > 0, nil
}
```

- [ ] **Step 2: Add v1 signal guard in ProcessSupportEvent**

In `ProcessSupportEvent` in `support_coverage.go`, find where it handles `human_reply_after_ai` and `conversation_resolved_by_human` event types. Before creating a gap from these two signals, add:

```go
if event.SourceSignal == model.SupportCoverageSourceHumanReply || event.SourceSignal == model.SupportCoverageSourceConversationResolvedByHuman {
	analyzed, err := s.coverageRepo.HasCompletedAnalysisRun(ctx, event.WorkspaceID)
	if err != nil {
		s.logger.Error("check analysis run status", "error", err, "workspace_id", event.WorkspaceID)
	}
	if analyzed {
		return nil
	}
}
```

Place this **after** the existing evidence-attachment block (which tries to attach evidence to an existing gap for these same signals), not before it. The guard should wrap only the fall-through to gap creation, preserving evidence enrichment for existing gaps in analyzed workspaces. Look for the `return nil` after evidence attachment and place this guard after it, before the clusterer/gap-creation logic.

- [ ] **Step 3: Run existing coverage service tests**

Run:

```bash
cd server
go test ./internal/service -run 'TestSupportCoverage' -count=1
```

Expected: PASS (no existing tests should break — the guard only activates for workspaces with completed analysis runs).

---

### Task 7: Verification

**Files:**
- Verify only.

- [ ] **Step 1: Format changed files**

Run:

```bash
cd server
gofmt -w internal/service/support_coverage_daily_analyzer.go internal/service/support_coverage_daily_analyzer_test.go internal/service/support_coverage.go internal/repository/support_coverage.go
```

- [ ] **Step 2: Run all focused analyzer tests**

Run:

```bash
cd server
go test ./internal/service -run 'TestCoverageConversationAnalysisInput|TestSupportCoverageDailyAnalyzer_|TestClassifyCoverageConversationLocally|TestNormalizeCoverageConversationAnalysisResult|TestBuildLatestCoverageConversationSegment|TestCoverageSegmentTranscriptHash' -count=1
```

Expected: PASS.

- [ ] **Step 3: Run repository tests**

Run:

```bash
cd server
go test ./internal/repository -run 'TestSupportCoverageRepository' -count=1
```

Expected: PASS.

- [ ] **Step 4: Run migration validation**

Run:

```bash
cd server
go test ./internal/dbmigrate -count=1
```

Expected: PASS.

---

## Risks And Decisions

- **Why not bump to v4?** The transcript hash is unchanged. These are guards around suggestion generation, knowledge refinement, and prompt wording — not input schema changes. Existing v3 analyses remain valid. New/changed conversations get the improved behavior naturally.
- **Why gate suggestions but not gap creation?** Gaps without human resolution are still valuable signals — they show where the AI isn't attempting to help. The gap + recommendations serve as a backlog. Only the auto-drafted article content is unreliable without observed resolution.
- **Why only suppress two v1 event types?** Other v1 signals (article feedback, widget no-result, docs issue feedback) are not conversation-based and aren't covered by the daily analyzer. They must continue.
- **Why 0.1 as the knowledge threshold?** Production data shows all irrelevant candidates at ~0.015-0.016. Relevant matches typically score 0.3+. The 0.1 threshold gives generous buffer while filtering obvious noise.
