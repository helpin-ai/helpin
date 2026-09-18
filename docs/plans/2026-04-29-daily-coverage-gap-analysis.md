# Scheduled support coverage analysis implementation plan

This historical plan explains the April 2026 design for turning support conversations into coverage findings and recommended fixes. Contributors investigating the analyzer should start with the source review below; the original checklist is not the current schedule, schema, or UI contract.

## Source review — 2026-09-18

- The [analyzer](../../server/internal/service/support_coverage_daily_analyzer.go) now uses workflow ID `coverage-analysis-v2`, analyzer version `v4`, and cron `0 */3 * * *` (every three hours), replacing the proposed daily 04:30 UTC run. Its two-hour cursor overlap and ten-minute settle delay remain. The [workflow](../../server/internal/temporalapp/coverage_analysis_workflow.go) supplies a three-hour initial window; the service's 30-day bootstrap applies only when no starting window is supplied. A first scheduled run therefore does not automatically analyze 30 days.
- Processing uses paginated candidates with four concurrent conversations per page. The current loop has no proposed 1,000-conversation run cap; the 1,000 constant limits workspace discovery. Workspace selection merges changed-conversation workspaces with queued workspaces and respects rollout policy. Workspace-child failures are logged by the parent workflow; they do not make its final return an error, although each child activity has retries.
- Input is the latest issue segment, not the full lifetime conversation: resolution events establish boundaries, internal messages are excluded from model input, and only the latest 80 public messages are retained. See the [segmentation review](2026-04-30-coverage-analysis-segmentation.md). Deterministic classification can skip non-support conversations before a model call; code also replaces invented human-resolution text when no human replied.
- V2 batches, leased analysis attempts, findings, and deferred materialization now supplement the legacy analysis records described below. Their logical keys include segment and policy information. Do not recreate only this plan's tables or assume its topic/gap writes describe the complete persistence contract.
- Conditional knowledge matching remains, but refinement runs only when current matches are nonempty; refinement failure logs a warning and retains the first recommendations. The [matcher](../../server/internal/service/support_coverage_knowledge_matcher.go) supports lexical fallback when embeddings fail and deduplicates document candidates by document **and block** when a block ID is available, rather than always by document.
- Commands and expected results below are historical implementation instructions, not results of this documentation review. No Temporal workflow, provider call, database migration, or model execution was run. Use current source and the current coverage UI before planning further work.

## Original implementation plan


**Goal:** Make Coverage Gaps a daily, evidence-backed AI improvement workflow that analyzes all recent support conversations, identifies why AI did not resolve or could resolve better, and recommends concrete fixes across docs, website content, data, actions, workflow, and policy.

**Architecture:** Keep support events and live AI retrieval traces as the raw evidence layer, but stop relying on event-time rule classification as the primary customer-facing gap creator. Add a daily Temporal analyzer that processes every conversation changed since the last successful cursor, loads full transcripts plus original RAG traces, conditionally re-runs retrieval against the same customer-facing RAG surfaces used by inbox AI, upserts topic-scoped gaps, writes multi-fix recommendation bundles, and records per-conversation analysis for idempotency and explainability.

**Tech Stack:** Go 1.24, GORM, PostgreSQL/Neon, dbmigrate SQL migrations, Temporal workflows, existing `internal/llm` provider, existing docs/content chunk hybrid search, existing Coverage gap/evidence/suggestion models, React Coverage UI.

---

## Current Code To Reuse

- `server/internal/model/support_inbox.go`
  - `SupportConversation` already has `status`, `flow_state`, `ai_state`, `ai_turn_count`, `human_takeover`, `resolved_at`, `customer_requested_human_at`.
  - `SupportMessage` already stores customer/user/ai messages and AI metadata JSON.
- `server/internal/repository/support_inbox.go`
  - `SupportMessageRepository.ListByConversation` already loads full transcripts.
  - `SupportConversationRepository` already owns conversation querying; add analyzer-specific listing here.
- `server/internal/model/support_coverage.go`
  - Existing taxonomy already includes knowledge, context, action, workflow, policy.
  - Existing `SupportCoverageGap`, `SupportGapEvidence`, `SupportGapSuggestion`, `SupportCoverageTopic` should stay the durable product state.
- `server/internal/repository/support_content_chunk.go`
  - Reuse `HybridSearch` for customer-facing website and crawled support content recommendations.
- `server/internal/service/support_coverage_clusterer.go`
  - Reuse topic/gap upsert mechanics, but add an analyzer-specific path that accepts structured findings instead of `SupportEvent`.
- `server/internal/service/support_coverage_enrichment.go`
  - Reuse suggestion versioning and topic enrichment update pattern, but replace shallow title-overlap KB context with inbox-AI-style knowledge retrieval.
- `server/internal/repository/docs_chunk.go`
  - Reuse `HybridSearch` for create-vs-update article decisions.
- `server/internal/service/support_ai.go`
  - Reuse the existing embedding provider pattern and knowledge retrieval approach.
- `server/internal/temporalapp/coverage_gap_workflow.go`
  - Extend the existing Coverage Temporal area rather than creating a separate scheduler subsystem.

## File Structure

### New Backend Files

- `server/internal/model/support_coverage_analysis.go`
  - GORM models and DTOs for daily analysis runs, per-conversation analysis records, retrieval traces, and coverage recommendations.

- `server/internal/dbmigrate/sql/202604290002_daily_coverage_analysis.sql`
  - Adds durable analysis run, conversation-analysis, retrieval-trace, and recommendation tables and indexes.

- `server/internal/repository/support_coverage_analysis.go`
  - Data access for analysis runs, cursor state, idempotency checks, retrieval traces, recommendations, and per-conversation analysis records.

- `server/internal/service/support_coverage_daily_analyzer.go`
  - Main service: select changed conversations, load transcripts, load original retrieval traces, call LLM, conditionally retrieve current knowledge, upsert coverage gaps, write recommendations/evidence.

- `server/internal/service/support_coverage_knowledge_matcher.go`
  - Focused helper for docs/content retrieval and target-surface candidate preparation.

- `server/internal/service/support_coverage_retrieval_trace.go`
  - Captures compact live inbox AI retrieval traces for later daily diagnosis.

- `server/internal/temporalapp/coverage_analysis_workflow.go`
  - Daily workflow and activities for conversation-level analysis.

### Modified Backend Files

- `server/cmd/api/main.go`
  - Wire repository/service dependencies and start the daily analysis cron.

- `server/cmd/temporal-worker/main.go`
  - Register new coverage analysis workflows/activities.

- `server/internal/model/support_coverage.go`
  - Add optional fields to detail/list DTOs for explanation metadata and recommendation bundles.

- `server/internal/repository/support_inbox.go`
  - Add an analyzer-specific conversation listing method.

- `server/internal/repository/support_coverage.go`
  - Add helper methods for analyzer upserts if current methods are too event-shaped.

- `server/internal/router/router.go`
  - No new route is required for v1 unless adding a manual run endpoint.

### Modified Frontend Files

- `frontend/src/lib/supportCoverageTypes.ts`
  - Add analysis/explanation metadata and recommendation bundle fields surfaced on gaps.

- `frontend/src/components/support/coverage/GapDetailPane.tsx`
  - Show evidence-backed explanation and recommendation cards.

- `frontend/src/components/support/coverage/GapList.tsx`
  - Make analyzed title/recommendation prominent.

- `frontend/src/pages/support/coverage/SupportCoveragePage.tsx`
  - Keep existing list/detail structure; remove assumptions that raw gaps need manual generation.

---

## Phase 1 - Persistence And Idempotency

### Task 1: Add daily analysis persistence models

**Files:**
- Create: `server/internal/model/support_coverage_analysis.go`

- [ ] **Step 1: Add models**

```go
package model

import (
	"encoding/json"
	"time"
)

const (
	SupportCoverageAnalysisRunStatusRunning   = "running"
	SupportCoverageAnalysisRunStatusCompleted = "completed"
	SupportCoverageAnalysisRunStatusFailed    = "failed"

	SupportCoverageConversationAnalysisStatusAnalyzed = "analyzed"
	SupportCoverageConversationAnalysisStatusSkipped  = "skipped"
	SupportCoverageConversationAnalysisStatusFailed   = "failed"

	SupportCoverageFixCreateArticle = "create_article"
	SupportCoverageFixUpdateArticle = "update_article"
	SupportCoverageFixUpdateWebsitePage = "update_website_page"
	SupportCoverageFixCreateWebsitePage = "create_website_page"
	SupportCoverageFixAddData       = "add_data"
	SupportCoverageFixAddAction     = "add_action"
	SupportCoverageFixDefinePolicy  = "define_policy"
	SupportCoverageFixImproveWorkflow = "improve_workflow"
	SupportCoverageFixNoFix         = "no_fix"

	SupportCoverageRecommendationStatusOpen      = "open"
	SupportCoverageRecommendationStatusAccepted  = "accepted"
	SupportCoverageRecommendationStatusDismissed = "dismissed"
	SupportCoverageRecommendationStatusApplied   = "applied"

	SupportCoverageRecommendationPriorityPrimary   = "primary"
	SupportCoverageRecommendationPrioritySecondary = "secondary"
)

type SupportCoverageAnalysisRun struct {
	ID              string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	WindowStart     time.Time       `json:"window_start" gorm:"not null"`
	WindowEnd       time.Time       `json:"window_end" gorm:"not null"`
	CursorStartedAt time.Time       `json:"cursor_started_at" gorm:"not null"`
	CursorEndedAt   time.Time       `json:"cursor_ended_at" gorm:"not null"`
	AnalyzerVersion string          `json:"analyzer_version" gorm:"not null;default:'v1'"`
	Status          string          `json:"status" gorm:"not null;default:'running'"`
	ConversationCnt int             `json:"conversation_count" gorm:"not null;default:0"`
	GapCount        int             `json:"gap_count" gorm:"not null;default:0"`
	ErrorMessage    *string         `json:"error_message"`
	Metadata        json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	StartedAt       time.Time       `json:"started_at" gorm:"not null"`
	CompletedAt     *time.Time      `json:"completed_at"`
	CreatedAt       time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportCoverageAnalysisRun) TableName() string { return "support_coverage_analysis_runs" }

type SupportCoverageConversationAnalysis struct {
	ID                 string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID        string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	RunID              string          `json:"run_id" gorm:"type:uuid;not null;index"`
	ConversationID     string          `json:"conversation_id" gorm:"type:uuid;not null;index"`
	Status             string          `json:"status" gorm:"not null"`
	HasGap             bool            `json:"has_gap" gorm:"not null;default:false"`
	GapID              *string         `json:"gap_id" gorm:"type:uuid"`
	GapKind            string          `json:"gap_kind" gorm:"not null;default:''"`
	GapCategory        string          `json:"gap_category" gorm:"not null;default:''"`
	PrimaryRecommendationType string  `json:"primary_recommendation_type" gorm:"not null;default:''"`
	TranscriptHash     string          `json:"transcript_hash" gorm:"not null;default:''"`
	AnalyzerVersion    string          `json:"analyzer_version" gorm:"not null;default:'v1'"`
	CustomerNeed       string          `json:"customer_need" gorm:"type:text;not null;default:''"`
	AIFailure          string          `json:"ai_failure" gorm:"type:text;not null;default:''"`
	HumanResolution    string          `json:"human_resolution" gorm:"type:text;not null;default:''"`
	DecisionReason     string          `json:"decision_reason" gorm:"type:text;not null;default:''"`
	Confidence         float64         `json:"confidence" gorm:"not null;default:0"`
	ErrorMessage       *string         `json:"error_message"`
	RawOutput          json.RawMessage `json:"raw_output" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt          time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportCoverageConversationAnalysis) TableName() string {
	return "support_coverage_conversation_analyses"
}

type SupportAIRetrievalTrace struct {
	ID             string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ConversationID string          `json:"conversation_id" gorm:"type:uuid;not null;index"`
	MessageID      string          `json:"message_id" gorm:"type:uuid;not null;index"`
	SearchQueries  json.RawMessage `json:"search_queries" gorm:"type:jsonb;not null;default:'[]'"`
	Results        json.RawMessage `json:"results" gorm:"type:jsonb;not null;default:'[]'"`
	CitedSourceIDs json.RawMessage `json:"cited_source_ids" gorm:"type:jsonb;not null;default:'[]'"`
	AIConfidence  float64         `json:"ai_confidence" gorm:"not null;default:0"`
	CanAnswer     *string         `json:"can_answer"`
	CanResolve    *string         `json:"can_resolve"`
	FailureMode   string          `json:"failure_mode" gorm:"not null;default:''"`
	Metadata      json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt     time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

func (SupportAIRetrievalTrace) TableName() string { return "support_ai_retrieval_traces" }

type SupportCoverageRecommendation struct {
	ID                 string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID        string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	GapID              string          `json:"gap_id" gorm:"type:uuid;not null;index"`
	AnalysisID         *string         `json:"analysis_id" gorm:"type:uuid"`
	RecommendationType string          `json:"recommendation_type" gorm:"not null"`
	TargetType         string          `json:"target_type" gorm:"not null;default:''"`
	TargetID           *string         `json:"target_id"`
	TargetTitle        string          `json:"target_title" gorm:"not null;default:''"`
	TargetURL          string          `json:"target_url" gorm:"type:text;not null;default:''"`
	Priority           string          `json:"priority" gorm:"not null;default:'secondary'"`
	Status             string          `json:"status" gorm:"not null;default:'open'"`
	Rationale          string          `json:"rationale" gorm:"type:text;not null;default:''"`
	SuggestedChange    string          `json:"suggested_change" gorm:"type:text;not null;default:''"`
	ImplementationNotes string         `json:"implementation_notes" gorm:"type:text;not null;default:''"`
	SuggestionID       *string         `json:"suggestion_id" gorm:"type:uuid"`
	Metadata           json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt          time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportCoverageRecommendation) TableName() string {
	return "support_coverage_recommendations"
}
```

- [ ] **Step 2: Run model package tests**

Run:

```bash
cd server && go test ./internal/model
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add server/internal/model/support_coverage_analysis.go
git commit -m "feat(coverage): add daily analysis models"
```

### Task 2: Add dbmigrate SQL

**Files:**
- Create: `server/internal/dbmigrate/sql/202604290002_daily_coverage_analysis.sql`

- [ ] **Step 1: Create migration**

```sql
CREATE TABLE IF NOT EXISTS support_coverage_analysis_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL,
  window_start timestamptz NOT NULL,
  window_end timestamptz NOT NULL,
  cursor_started_at timestamptz NOT NULL,
  cursor_ended_at timestamptz NOT NULL,
  analyzer_version text NOT NULL DEFAULT 'v1',
  status text NOT NULL DEFAULT 'running',
  conversation_cnt int NOT NULL DEFAULT 0,
  gap_count int NOT NULL DEFAULT 0,
  error_message text,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  started_at timestamptz NOT NULL,
  completed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_coverage_analysis_runs_window
  ON support_coverage_analysis_runs(workspace_id, window_start, window_end);

CREATE INDEX IF NOT EXISTS idx_support_coverage_analysis_runs_workspace_status
  ON support_coverage_analysis_runs(workspace_id, status, started_at DESC);

CREATE TABLE IF NOT EXISTS support_coverage_conversation_analyses (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL,
  run_id uuid NOT NULL,
  conversation_id uuid NOT NULL,
  status text NOT NULL,
  has_gap boolean NOT NULL DEFAULT false,
  gap_id uuid,
  gap_kind text NOT NULL DEFAULT '',
  gap_category text NOT NULL DEFAULT '',
  primary_recommendation_type text NOT NULL DEFAULT '',
  transcript_hash text NOT NULL DEFAULT '',
  analyzer_version text NOT NULL DEFAULT 'v1',
  customer_need text NOT NULL DEFAULT '',
  ai_failure text NOT NULL DEFAULT '',
  human_resolution text NOT NULL DEFAULT '',
  decision_reason text NOT NULL DEFAULT '',
  confidence double precision NOT NULL DEFAULT 0,
  error_message text,
  raw_output jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_coverage_conversation_analyses_once
  ON support_coverage_conversation_analyses(workspace_id, conversation_id, run_id);

CREATE INDEX IF NOT EXISTS idx_support_coverage_conversation_analyses_gap
  ON support_coverage_conversation_analyses(gap_id)
  WHERE gap_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_support_coverage_conversation_analyses_conversation
  ON support_coverage_conversation_analyses(workspace_id, conversation_id, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_coverage_conversation_analyses_transcript
  ON support_coverage_conversation_analyses(workspace_id, conversation_id, transcript_hash, analyzer_version);

CREATE TABLE IF NOT EXISTS support_ai_retrieval_traces (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL,
  conversation_id uuid NOT NULL,
  message_id uuid NOT NULL,
  search_queries jsonb NOT NULL DEFAULT '[]'::jsonb,
  results jsonb NOT NULL DEFAULT '[]'::jsonb,
  cited_source_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
  ai_confidence double precision NOT NULL DEFAULT 0,
  can_answer text,
  can_resolve text,
  failure_mode text NOT NULL DEFAULT '',
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_ai_retrieval_traces_message
  ON support_ai_retrieval_traces(workspace_id, message_id);

CREATE INDEX IF NOT EXISTS idx_support_ai_retrieval_traces_conversation
  ON support_ai_retrieval_traces(workspace_id, conversation_id, created_at DESC);

CREATE TABLE IF NOT EXISTS support_coverage_recommendations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL,
  gap_id uuid NOT NULL,
  analysis_id uuid,
  recommendation_type text NOT NULL,
  target_type text NOT NULL DEFAULT '',
  target_id text,
  target_title text NOT NULL DEFAULT '',
  target_url text NOT NULL DEFAULT '',
  priority text NOT NULL DEFAULT 'secondary',
  status text NOT NULL DEFAULT 'open',
  rationale text NOT NULL DEFAULT '',
  suggested_change text NOT NULL DEFAULT '',
  implementation_notes text NOT NULL DEFAULT '',
  suggestion_id uuid,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_support_coverage_recommendations_gap
  ON support_coverage_recommendations(gap_id, status, priority);

CREATE INDEX IF NOT EXISTS idx_support_coverage_recommendations_workspace
  ON support_coverage_recommendations(workspace_id, status, created_at DESC);
```

- [ ] **Step 2: Validate migrations**

Run:

```bash
cd server && go run ./cmd/migrate validate
```

Expected: PASS with no validation errors.

- [ ] **Step 3: Commit**

```bash
git add server/internal/dbmigrate/sql/202604290002_daily_coverage_analysis.sql
git commit -m "feat(coverage): add daily analysis persistence migration"
```

### Task 3: Repository for analysis runs

**Files:**
- Create: `server/internal/repository/support_coverage_analysis.go`
- Test: `server/internal/repository/support_coverage_analysis_test.go`

- [ ] **Step 1: Write repository tests**

Cover:
- `CreateRun` is idempotent for workspace/window/analyzer version.
- `CompleteRun` sets completed status, counts, and cursor end.
- `RecordConversationAnalysis` writes one analysis per transcript hash/analyzer version.
- `AlreadyAnalyzedConversation` returns true for a conversation already analyzed with the same transcript hash and analyzer version.
- `UpsertRetrievalTrace` stores compact live RAG traces by message.
- `ReplaceRecommendations` writes a recommendation bundle for a gap.

- [ ] **Step 2: Implement repository**

Required interface:

```go
type SupportCoverageAnalysisRepository struct {
	db *gorm.DB
}

func NewSupportCoverageAnalysisRepository(db *gorm.DB) *SupportCoverageAnalysisRepository
func (r *SupportCoverageAnalysisRepository) CreateRun(ctx context.Context, run *model.SupportCoverageAnalysisRun) (*model.SupportCoverageAnalysisRun, error)
func (r *SupportCoverageAnalysisRepository) CompleteRun(ctx context.Context, runID string, conversationCount, gapCount int) error
func (r *SupportCoverageAnalysisRepository) FailRun(ctx context.Context, runID string, err error) error
func (r *SupportCoverageAnalysisRepository) RecordConversationAnalysis(ctx context.Context, analysis *model.SupportCoverageConversationAnalysis) error
func (r *SupportCoverageAnalysisRepository) AlreadyAnalyzedConversation(ctx context.Context, workspaceID, conversationID, transcriptHash, analyzerVersion string) (bool, error)
func (r *SupportCoverageAnalysisRepository) LastSuccessfulCursor(ctx context.Context, workspaceID, analyzerVersion string) (*time.Time, error)
func (r *SupportCoverageAnalysisRepository) UpsertRetrievalTrace(ctx context.Context, trace *model.SupportAIRetrievalTrace) error
func (r *SupportCoverageAnalysisRepository) ListRetrievalTracesByConversation(ctx context.Context, workspaceID, conversationID string) ([]model.SupportAIRetrievalTrace, error)
func (r *SupportCoverageAnalysisRepository) ReplaceRecommendations(ctx context.Context, workspaceID, gapID string, recommendations []model.SupportCoverageRecommendation) error
```

Replacement rule:
- `ReplaceRecommendations` may replace only recommendation rows where `status = 'open'`.
- Preserve `accepted`, `applied`, and `dismissed` rows as historical user decisions.
- New rows should be inserted with `status='open'`.
- If a recommendation is linked to a docs `SupportGapSuggestion`, store `suggestion_id`.

- [ ] **Step 3: Run repository tests**

Run:

```bash
cd server && go test ./internal/repository -run 'TestSupportCoverageAnalysis'
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add server/internal/repository/support_coverage_analysis.go server/internal/repository/support_coverage_analysis_test.go
git commit -m "feat(coverage): add analysis run repository"
```

---

## Phase 2 - Conversation Selection, Retrieval Traces, And Transcript Assembly

### Task 4: List changed conversations for daily analysis

**Files:**
- Modify: `server/internal/repository/support_inbox.go`
- Test: `server/internal/repository/support_inbox_coverage_analysis_test.go`

- [ ] **Step 1: Write failing tests**

Test cursor selection:
- Includes any conversation whose `updated_at` is inside the cursor window.
- Includes any conversation whose `resolved_at` is inside the cursor window.
- Excludes spam.
- Does not require AI involvement.
- Orders by `updated_at ASC`, then `id ASC` so cursor processing is deterministic.
- Accepts an overlap window from the caller; repository does not compute overlap.

- [ ] **Step 2: Implement method**

Add:

```go
func (r *SupportConversationRepository) ListCoverageAnalysisCandidates(
	ctx context.Context,
	workspaceID string,
	windowStart, windowEnd time.Time,
	limit int,
) ([]model.SupportConversation, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var conversations []model.SupportConversation
	err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Where("status <> ?", model.SupportConversationStatusSpam).
		Where(`(
			(updated_at >= ? AND updated_at < ?)
			OR (resolved_at IS NOT NULL AND resolved_at >= ? AND resolved_at < ?)
		)`, windowStart, windowEnd, windowStart, windowEnd).
		Order("updated_at ASC, id ASC").
		Limit(limit).
		Find(&conversations).Error
	if err != nil {
		return nil, fmt.Errorf("list coverage analysis candidates: %w", err)
	}
	return conversations, nil
}
```

- [ ] **Step 3: Run tests**

Run:

```bash
cd server && go test ./internal/repository -run 'TestSupportConversationRepository_ListCoverageAnalysisCandidates'
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add server/internal/repository/support_inbox.go server/internal/repository/support_inbox_coverage_analysis_test.go
git commit -m "feat(coverage): list changed conversations for daily analysis"
```

### Task 4.5: Capture live inbox AI retrieval traces

**Files:**
- Create: `server/internal/service/support_coverage_retrieval_trace.go`
- Modify: `server/internal/service/support_ai.go`
- Test: `server/internal/service/support_coverage_retrieval_trace_test.go`

- [ ] **Step 1: Write failing tests**

Cover:
- Search queries, retrieved docs chunks, retrieved website/content chunks, scores, cited source IDs, confidence, `can_answer`, and `can_resolve` are compacted into a trace.
- Trace content is capped to avoid storing full documents.
- Trace upsert is best-effort and does not block support AI replies.

- [ ] **Step 2: Implement trace capture helper**

Add a small interface so `SupportAIService` does not depend directly on the coverage analyzer:

```go
type SupportAIRetrievalTraceRecorder interface {
	RecordSupportAIRetrievalTrace(ctx context.Context, trace *model.SupportAIRetrievalTrace) error
}
```

Capture after RAG search and AI response generation, using the same `KnowledgeSearchResult` values already available in `SupportAIService`.

Invocation rule:
- Invoke trace capture after the AI message has been successfully persisted, because the trace references `message_id`.
- Do not include trace writes in the support message transaction.
- Use a detached bounded context for best-effort persistence:

```go
go func(trace *model.SupportAIRetrievalTrace) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := recorder.RecordSupportAIRetrievalTrace(ctx, trace); err != nil {
		slog.WarnContext(ctx, "record support AI retrieval trace failed",
			"error", err,
			"workspace_id", trace.WorkspaceID,
			"conversation_id", trace.ConversationID,
			"message_id", trace.MessageID,
		)
	}
}(trace)
```

The goroutine must receive a fully materialized trace value. Do not capture request-scoped mutable values in the closure.

- [ ] **Step 3: Run tests**

Run:

```bash
cd server && go test ./internal/service -run 'TestSupportAIRetrievalTrace'
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add server/internal/service/support_coverage_retrieval_trace.go server/internal/service/support_coverage_retrieval_trace_test.go server/internal/service/support_ai.go
git commit -m "feat(coverage): capture support AI retrieval traces"
```

### Task 5: Build analyzer transcript input

**Files:**
- Create: `server/internal/service/support_coverage_daily_analyzer.go`
- Test: `server/internal/service/support_coverage_daily_analyzer_test.go`

- [ ] **Step 1: Add input DTOs and tests**

Test:
- Internal messages are excluded by default.
- AI metadata is parsed when present.
- Human replies after AI are included as likely resolution.
- Long transcripts are truncated deterministically.
- Transcript hash changes when message content changes.
- Transcript hash is stable when messages are already ordered or loaded in a different order.

- [ ] **Step 2: Implement transcript assembly types**

```go
type CoverageConversationMessage struct {
	ID              string    `json:"id"`
	SenderType      string    `json:"sender_type"`
	Content         string    `json:"content"`
	CreatedAt       time.Time `json:"created_at"`
	AIConfidence    float64   `json:"ai_confidence,omitempty"`
	AIReplyKind     string    `json:"ai_reply_kind,omitempty"`
	AIIssueKey      string    `json:"ai_issue_key,omitempty"`
	AIIssueSummary  string    `json:"ai_issue_summary,omitempty"`
	AIProgressState string    `json:"ai_progress_state,omitempty"`
}

type CoverageConversationAnalysisInput struct {
	WorkspaceID    string                        `json:"workspace_id"`
	ConversationID string                        `json:"conversation_id"`
	Subject        string                        `json:"subject"`
	Status         string                        `json:"status"`
	FlowState      string                        `json:"flow_state"`
	AITurnCount    int                           `json:"ai_turn_count"`
	Messages       []CoverageConversationMessage `json:"messages"`
	RetrievalTraces []CoverageRetrievalTraceInput `json:"retrieval_traces"`
}

type CoverageRetrievalTraceInput struct {
	MessageID      string                     `json:"message_id"`
	SearchQueries  []string                   `json:"search_queries"`
	Results        []CoverageKnowledgeCandidate `json:"results"`
	CitedSourceIDs []string                   `json:"cited_source_ids"`
	AIConfidence  float64                    `json:"ai_confidence"`
	FailureMode   string                     `json:"failure_mode"`
}
```

Transcript hash specification:
- Sort messages by `created_at ASC`, then `id ASC`.
- For each non-internal message, include exactly these fields in order: `id`, `sender_type`, `message_type`, normalized `content`, `created_at.UTC().Format(time.RFC3339Nano)`, and normalized `metadata`.
- Normalize content with `strings.TrimSpace` and collapse CRLF to LF. Do not lowercase; casing can matter for IDs and product names.
- Normalize metadata by parsing JSON and re-marshalling it with Go's `encoding/json`; if parsing fails, use trimmed raw metadata.
- Join fields with `\x1f` and records with `\x1e`.
- Hash with SHA-256 and store lowercase hex.

Add:

```go
func CoverageTranscriptHash(messages []model.SupportMessage) string
```

Use local parsing logic equivalent to `AIMessageMetadata`. Do not export or depend on unexported `parseAIMessageMetadata`.

- [ ] **Step 3: Run tests**

Run:

```bash
cd server && go test ./internal/service -run 'TestCoverageConversationAnalysisInput'
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add server/internal/service/support_coverage_daily_analyzer.go server/internal/service/support_coverage_daily_analyzer_test.go
git commit -m "feat(coverage): assemble daily analysis conversation input"
```

---

## Phase 3 - Analyzer LLM Contract

### Task 6: Add structured conversation analyzer

**Files:**
- Modify: `server/internal/service/support_coverage_daily_analyzer.go`
- Test: `server/internal/service/support_coverage_daily_analyzer_test.go`

- [ ] **Step 1: Write tests**

Cover:
- Parses a knowledge gap.
- Parses a data/context gap.
- Parses an action gap.
- Parses no-gap result.
- Rejects malformed JSON.

- [ ] **Step 2: Add output DTO and schema**

```go
type CoverageConversationAnalysisResult struct {
	HasGap             bool    `json:"has_gap"`
	GapKind            string  `json:"gap_kind"`
	GapCategory        string  `json:"gap_category"`
	CanonicalTitle     string  `json:"canonical_title"`
	CustomerNeed       string  `json:"customer_need"`
	AIFailure          string  `json:"ai_failure"`
	HumanResolution    string  `json:"human_resolution"`
	DecisionReason     string  `json:"decision_reason"`
	SearchQuery        string  `json:"search_query"`
	ShouldRunRetrieval bool    `json:"should_run_retrieval"`
	RecommendedFixes   []CoverageRecommendedFix `json:"recommended_fixes"`
	Confidence         float64 `json:"confidence"`
}

type CoverageRecommendedFix struct {
	Type                string `json:"type"`
	TargetType          string `json:"target_type"`
	TargetID            string `json:"target_id"`
	TargetTitle         string `json:"target_title"`
	TargetURL           string `json:"target_url"`
	Priority            string `json:"priority"`
	Rationale           string `json:"rationale"`
	SuggestedChange     string `json:"suggested_change"`
	ImplementationNotes string `json:"implementation_notes"`
}
```

Cost guardrails:
- Run one first-pass analyzer call per changed conversation unless `AlreadyAnalyzedConversation` returns true for the same transcript hash and analyzer version.
- Do not run retrieval refinement unless `should_run_retrieval=true`.
- Do not run docs draft generation unless the final recommendation bundle includes `create_article` or `update_article`.
- Enforce the per-workspace 1,000-conversation cap from the workflow activity.

Allowed values:
- `gap_kind`: `content`, `data`, `action`, `policy`
- `gap_category`: existing coverage categories from `model/support_coverage.go`
- `recommended_fixes[].type`: `create_article`, `update_article`, `update_website_page`, `create_website_page`, `add_data`, `add_action`, `define_policy`, `improve_workflow`, `no_fix`
- `recommended_fixes[].target_type`: `docs`, `website_page`, `content_source`, `data_source`, `tool_action`, `policy`, `workflow`, `agent_instruction`

- [ ] **Step 3: Implement prompt**

System prompt requirements:
- Return JSON only.
- Decide from full conversation outcome, not one message.
- Treat human replies as the best evidence of what was missing.
- Use live retrieval traces to diagnose what AI actually searched and saw during the conversation.
- Do not create a gap if the AI correctly resolved the issue.
- Set `should_run_retrieval=true` only when current docs/website/content search can materially improve the recommendation.
- Prefer `knowledge/content` only when customer-facing knowledge could reasonably fix the issue.
- Use `data/context` when the human used customer/account/order/subscription data.
- Use `action` when the human performed an operation the AI could not perform.
- Use `policy` when the human applied judgment, approval, exception, or escalation policy.
- Recommend multiple fixes when one surface alone will not reduce repeated human intervention.
- Limit to 3 fixes. Mark exactly one as `primary` unless two fixes are equally necessary.
- Recommend website/content changes for prospect, sales, pricing, migration, integration, security, comparison, or pre-purchase questions.
- Recommend docs changes for setup, usage, troubleshooting, and post-signup workflows.

- [ ] **Step 4: Implement `AnalyzeConversation`**

```go
func (s *SupportCoverageDailyAnalyzer) AnalyzeConversation(
	ctx context.Context,
	input CoverageConversationAnalysisInput,
) (*CoverageConversationAnalysisResult, json.RawMessage, error)
```

Use `llm.ChatRequest{JSONMode: true, JSONSchema: coverageConversationAnalysisJSONSchema()}`.

- [ ] **Step 5: Run tests**

Run:

```bash
cd server && go test ./internal/service -run 'TestSupportCoverageDailyAnalyzer_AnalyzeConversation'
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add server/internal/service/support_coverage_daily_analyzer.go server/internal/service/support_coverage_daily_analyzer_test.go
git commit -m "feat(coverage): analyze support conversations for gaps"
```

---

## Phase 4 - Knowledge Matching Across Docs And Website Content

### Task 7: Add knowledge matcher using existing hybrid search

**Files:**
- Create: `server/internal/service/support_coverage_knowledge_matcher.go`
- Test: `server/internal/service/support_coverage_knowledge_matcher_test.go`

- [ ] **Step 1: Write tests**

Cover:
- Searches external help center docs through `DocsChunkRepository.HybridSearch`.
- Searches website/crawled support content through `SupportContentChunkRepository.HybridSearch`.
- With no knowledge results, returns an empty candidate list for create-content judgment.
- With strong matching docs or website candidates, returns candidate list for LLM judgment.
- Falls back to lexical search when embedding provider is nil.
- Dedupes multiple chunks from the same target.
- Does not search internal/private docs.

- [ ] **Step 2: Implement matcher types**

```go
type CoverageKnowledgeCandidate struct {
	SourceType    string  `json:"source_type"` // docs | website
	TargetType    string  `json:"target_type"` // docs | website_page
	DocumentID    string  `json:"document_id,omitempty"`
	PageID        string  `json:"page_id,omitempty"`
	Title         string  `json:"title"`
	URL           string  `json:"url,omitempty"`
	Excerpt       string  `json:"excerpt"`
	CombinedScore float64 `json:"combined_score"`
}

type CoverageKnowledgeMatcher struct {
	docsChunkRepo       *repository.DocsChunkRepository
	contentChunkRepo    *repository.SupportContentChunkRepository
	embeddingProvider   llm.EmbeddingProvider
	embeddingModel      string
}
```

- [ ] **Step 3: Implement retrieval**

```go
func (m *CoverageKnowledgeMatcher) MatchKnowledge(
	ctx context.Context,
	workspaceID string,
	spaceIDs []string,
	contentSourceIDs []string,
	query string,
	limit int,
) ([]CoverageKnowledgeCandidate, error)
```

Use the same embedding request pattern as `SupportAIService.searchSingleQuery`.

- [ ] **Step 4: Run tests**

Run:

```bash
cd server && go test ./internal/service -run 'TestCoverageKnowledgeMatcher'
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/service/support_coverage_knowledge_matcher.go server/internal/service/support_coverage_knowledge_matcher_test.go
git commit -m "feat(coverage): match coverage findings to customer-facing knowledge"
```

### Task 8: Add LLM fix-bundle refinement from knowledge candidates

**Files:**
- Modify: `server/internal/service/support_coverage_daily_analyzer.go`
- Test: `server/internal/service/support_coverage_daily_analyzer_test.go`

- [ ] **Step 1: Write tests**

Cover:
- Existing article incomplete -> recommendation includes `update_article` with `target_id`.
- Existing website page incomplete -> recommendation includes `update_website_page` with `target_url`.
- No relevant docs or website page -> recommendation includes `create_article`, `create_website_page`, or both based on audience.
- Non-knowledge gaps do not re-run retrieval unless the first-pass analyzer requested it.
- Mixed cases can produce multiple recommendations, capped at 3.

- [ ] **Step 2: Add route decision DTO**

```go
type CoverageFixBundleDecision struct {
	RecommendedFixes []CoverageRecommendedFix `json:"recommended_fixes"`
	DecisionReason   string                   `json:"decision_reason"`
	Confidence       float64                  `json:"confidence"`
}
```

- [ ] **Step 3: Implement route decision**

```go
func (s *SupportCoverageDailyAnalyzer) RefineFixBundleWithKnowledge(
	ctx context.Context,
	result CoverageConversationAnalysisResult,
	candidates []CoverageKnowledgeCandidate,
) (*CoverageFixBundleDecision, error)
```

Prompt rule:
- Use `update_article` only when the candidate help article is clearly about the same customer need but missing/outdated/unclear.
- Use `update_website_page` when a website/content page should answer a prospect, sales, pricing, integration, migration, security, or pre-purchase question.
- Use `create_article` or `create_website_page` when no candidate covers the same topic.
- Preserve data/action/policy/workflow recommendations from first-pass analysis if they remain relevant.
- Return a short reason suitable for UI.

- [ ] **Step 4: Run tests**

Run:

```bash
cd server && go test ./internal/service -run 'TestSupportCoverageDailyAnalyzer_RefineFixBundleWithKnowledge'
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/service/support_coverage_daily_analyzer.go server/internal/service/support_coverage_daily_analyzer_test.go
git commit -m "feat(coverage): refine coverage fix bundles from knowledge matches"
```

---

## Phase 5 - Upsert Findings Into Coverage Gaps

### Task 9: Add analyzer finding upsert

**Files:**
- Modify: `server/internal/service/support_coverage_daily_analyzer.go`
- Modify: `server/internal/repository/support_coverage.go`
- Test: `server/internal/service/support_coverage_daily_analyzer_test.go`

- [ ] **Step 1: Write tests**

Cover:
- Knowledge finding creates topic, open gap, conversation evidence, recommendations, and optional docs suggestion.
- Same canonical title/need on second conversation increments evidence instead of duplicate visible gap.
- Data/action/policy/workflow findings create gaps and recommendation cards.
- Website/content findings create recommendations without one-click apply.
- Analysis record links to created gap.

- [ ] **Step 2: Implement cluster key for analyzed findings**

Use deterministic key:

```go
clusterKey := ComputeSupportCoverageClusterKey(
	workspaceID,
	"daily_conversation_analysis",
	primaryTargetID,
	"",
	result.CanonicalTitle+" "+result.CustomerNeed,
)
```

- [ ] **Step 3: Implement evidence metadata**

Evidence should store enough explanation for the UI:

```json
{
  "customer_need": "...",
  "ai_failure": "...",
  "human_resolution": "...",
  "decision_reason": "...",
  "recommended_fixes": [...],
  "conversation_analysis_id": "..."
}
```

- [ ] **Step 4: Implement recommendation writes**

For every gap, replace open recommendation rows with the current bundle:

```json
{
  "recommendation_type": "update_website_page",
  "target_type": "website_page",
  "target_url": "https://example.com/pricing",
  "priority": "primary",
  "rationale": "...",
  "suggested_change": "...",
  "implementation_notes": "..."
}
```

Recommendation rows are the primary UX for data/action/policy/website fixes.

- [ ] **Step 5: Implement docs suggestion metadata**

For knowledge suggestions:

```json
{
  "source": "daily_conversation_analysis",
  "decision_reason": "...",
  "matched_knowledge_candidates": [...],
  "recommendation_id": "..."
}
```

- [ ] **Step 6: Ensure active suggestion versioning**

Before writing a new active suggestion for the gap:
- Set prior active suggestions to `is_active=false`.
- Set `superseded_at=now`.

This should mirror `SupportCoverageEnrichmentService.writeEnrichmentResult`.

- [ ] **Step 7: Run tests**

Run:

```bash
cd server && go test ./internal/service -run 'TestSupportCoverageDailyAnalyzer_UpsertFinding'
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add server/internal/service/support_coverage_daily_analyzer.go server/internal/repository/support_coverage.go server/internal/service/support_coverage_daily_analyzer_test.go
git commit -m "feat(coverage): upsert daily findings and recommendations"
```

### Task 10: Generate executable docs suggestions from recommendation bundle

**Files:**
- Modify: `server/internal/service/support_coverage_daily_analyzer.go`
- Reuse: `server/internal/tiptap`
- Test: `server/internal/service/support_coverage_daily_analyzer_test.go`

- [ ] **Step 1: Write tests**

Cover:
- `create_article` recommendation returns TipTap JSON suggestion content.
- `update_article` recommendation returns TipTap JSON suggestion content and target document ID.
- Website/data/action/policy recommendations do not create `SupportGapSuggestion` rows.
- Content includes human resolution and avoids unsupported claims.

- [ ] **Step 2: Add draft-generation LLM method**

```go
func (s *SupportCoverageDailyAnalyzer) GenerateKnowledgeSuggestion(
	ctx context.Context,
	result CoverageConversationAnalysisResult,
	fix CoverageRecommendedFix,
) (title string, tiptapContent json.RawMessage, err error)
```

For v1, generate concise markdown and convert with `tiptap.MarkdownToJSON`.

- [ ] **Step 3: Run tests**

Run:

```bash
cd server && go test ./internal/service -run 'TestSupportCoverageDailyAnalyzer_GenerateKnowledgeSuggestion'
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add server/internal/service/support_coverage_daily_analyzer.go server/internal/service/support_coverage_daily_analyzer_test.go
git commit -m "feat(coverage): generate suggestions from daily analysis"
```

---

## Phase 6 - Temporal Daily Workflow

### Task 11: Add coverage analysis Temporal workflow

**Files:**
- Create: `server/internal/temporalapp/coverage_analysis_workflow.go`
- Test: `server/internal/temporalapp/coverage_analysis_workflow_test.go`

- [ ] **Step 1: Write workflow tests**

Cover:
- Daily workflow lists workspaces and starts per-workspace child workflows.
- Per-workspace workflow calls analyzer activity with the expected 24-hour window.
- Activity failures fail the workflow and rely on Temporal retry.
- Workspace fanout concurrency is capped at 10 child workflows in flight.
- Per-workspace conversation processing concurrency is controlled inside the activity, not the workflow.

- [ ] **Step 2: Implement constants and activities**

```go
const (
	CoverageDailyAnalysisWorkflowType        = "CoverageDailyAnalysisWorkflow"
	CoverageWorkspaceAnalysisWorkflowType    = "CoverageWorkspaceAnalysisWorkflow"
	CoverageListAnalysisWorkspacesActivity   = "CoverageAnalysisActivities.ListWorkspacesActivity"
	CoverageRunWorkspaceAnalysisActivityName = "CoverageAnalysisActivities.RunWorkspaceAnalysisActivity"
)
```

Activities:

```go
type coverageDailyAnalyzer interface {
	ListWorkspacesForDailyAnalysis(ctx context.Context) ([]string, error)
	RunWorkspaceDailyAnalysis(ctx context.Context, workspaceID string, windowStart, windowEnd time.Time) error
}
```

Workspace selection rule:
- `ListWorkspacesForDailyAnalysis` should return workspaces that have at least one non-spam support conversation updated or resolved after the last successful cursor minus overlap.
- If a workspace has never successfully run, look back 30 days for bootstrap.
- Do not return every workspace unconditionally.
- Order by workspace ID for deterministic fanout.

- [ ] **Step 3: Implement workflow**

Cron is started by API process, similar to existing coverage daily batch. The schedule is fixed at `04:30 UTC` daily; do not add workspace-local scheduling in v1. Workflow itself:
- Computes a cursor window from the last successful run per workspace/analyzer version.
- Uses a 2-hour overlap (`window_start = last_successful_cursor - 2h`) to catch late updates.
- Uses a 10-minute settle delay (`window_end = now - 10m`) to avoid analyzing conversations still being written.
- Dedupes by `conversation_id + transcript_hash + analyzer_version`.
- Starts one child workflow per workspace.
- Bounds workspace child-workflow concurrency to 10.
- The per-workspace activity should bound conversation-level LLM analysis concurrency to 4.
- The per-workspace activity should cap analyzed conversations to 1,000 per run in v1. If more are pending, record `metadata.truncated=true` on the run and the next daily run will continue from the cursor/overlap.

- [ ] **Step 4: Run tests**

Run:

```bash
cd server && go test ./internal/temporalapp -run 'TestCoverageDailyAnalysis'
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/temporalapp/coverage_analysis_workflow.go server/internal/temporalapp/coverage_analysis_workflow_test.go
git commit -m "feat(coverage): add daily analysis Temporal workflow"
```

### Task 12: Wire workflow into API and worker

**Files:**
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/temporal-worker/main.go`
- Modify: `server/internal/service/support_coverage_daily_analyzer.go`
- Test: existing package builds

- [ ] **Step 1: Add daily analysis starter**

In analyzer service:

```go
func (s *SupportCoverageDailyAnalyzer) EnsureDailyAnalysis(ctx context.Context) error
```

Use Temporal `ExecuteWorkflow` with:

```go
ID: "coverage-daily-analysis"
TaskQueue: temporalapp.QueueAutomation
CronSchedule: "30 4 * * *"
```

Use 04:30 UTC as the fixed v1 runtime. It keeps operations simple and the cursor model prevents missed conversations if the cron runs late or a previous run fails.

- [ ] **Step 2: Wire repositories/services in `cmd/api`**

Create:
- `SupportCoverageAnalysisRepository`
- `CoverageKnowledgeMatcher`
- `SupportCoverageDailyAnalyzer`

Call `EnsureDailyAnalysis` at startup when Temporal client exists.

- [ ] **Step 3: Wire worker activities in `cmd/temporal-worker`**

Register:
- `CoverageDailyAnalysisWorkflow`
- `CoverageWorkspaceAnalysisWorkflow`
- `CoverageAnalysisActivities.ListWorkspacesActivity`
- `CoverageAnalysisActivities.RunWorkspaceAnalysisActivity`

- [ ] **Step 4: Run backend build**

Run:

```bash
cd server && go test ./cmd/api ./cmd/temporal-worker ./internal/service ./internal/temporalapp
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/cmd/api/main.go server/cmd/temporal-worker/main.go server/internal/service/support_coverage_daily_analyzer.go
git commit -m "feat(coverage): wire daily analysis workflow"
```

---

## Phase 7 - UI Explainability

### Task 13: Surface analysis metadata in gap detail

**Files:**
- Modify: `server/internal/model/support_coverage.go`
- Modify: `server/internal/repository/support_coverage.go`
- Modify: `frontend/src/lib/supportCoverageTypes.ts`
- Test: `server/internal/repository/support_coverage_test.go`

- [ ] **Step 1: Write repository test**

Given evidence metadata with `customer_need`, `ai_failure`, `human_resolution`, and `decision_reason`, `GetGapDetail` exposes a compact `analysis_explanation` field.

- [ ] **Step 2: Add DTO**

```go
type SupportCoverageAnalysisExplanation struct {
	CustomerNeed       string `json:"customer_need"`
	AIFailure          string `json:"ai_failure"`
	HumanResolution    string `json:"human_resolution"`
	DecisionReason     string `json:"decision_reason"`
}
```

Add to `SupportCoverageGapDetail`:

```go
AnalysisExplanation *SupportCoverageAnalysisExplanation `json:"analysis_explanation,omitempty"`
Recommendations []SupportCoverageRecommendation `json:"recommendations"`
```

- [ ] **Step 3: Populate from newest evidence metadata**

Only use metadata from evidence with `source_signal = "daily_conversation_analysis"` or metadata source equivalent. Load recommendation rows from `support_coverage_recommendations` ordered by primary first, then newest.

- [ ] **Step 4: Run tests**

Run:

```bash
cd server && go test ./internal/repository -run 'TestSupportCoverageRepository_GetGapDetail.*Analysis'
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/model/support_coverage.go server/internal/repository/support_coverage.go frontend/src/lib/supportCoverageTypes.ts server/internal/repository/support_coverage_test.go
git commit -m "feat(coverage): expose analysis explanation on gap detail"
```

### Task 14: Update detail pane to show evidence-backed diagnosis and fix bundle

**Files:**
- Modify: `frontend/src/components/support/coverage/GapDetailPane.tsx`

- [ ] **Step 1: Add UI section**

In the detail pane, before the docs suggestion card, render:

```text
Why this matters
Customer needed: ...
AI missed: ...
Human resolved by: ...
```

Keep this compact. Do not add marketing-style explanatory copy.

- [ ] **Step 2: Add recommendation cards**

Render `gap.recommendations` as cards:

```text
Primary recommendation
Update website page
Target: /pricing
Suggested change: ...
Why: ...
```

Rules:
- Docs recommendations with linked `SupportGapSuggestion` keep `Review Add`.
- Website/content recommendations show target URL, suggested change, and implementation notes.
- Data/action/policy/workflow recommendations show the missing capability and implementation notes.
- Avoid pretending non-docs fixes are one-click.

- [ ] **Step 3: Preserve current suggestion actions**

The existing `Review Add`, `Discard`, and split-button behavior stays unchanged.

- [ ] **Step 4: Run frontend build**

Run:

```bash
cd frontend && npm run build
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/support/coverage/GapDetailPane.tsx
git commit -m "feat(coverage): show analysis diagnosis and recommendations"
```

---

## Phase 8 - Transition From Raw Event Gaps

### Task 15: Hide raw low-confidence gaps from default Coverage UI

**Files:**
- Modify: `server/internal/repository/support_coverage.go`
- Modify: `server/internal/service/support_coverage.go`
- Test: `server/internal/service/support_coverage_test.go`

- [ ] **Step 1: Write tests**

Default list behavior:
- Includes daily-analyzed gaps.
- Includes existing gaps with active suggestions.
- Excludes `needs_review` raw event gaps with `evidence_30d == 1`.
- Excludes raw gaps with no active suggestion unless `show_raw=true` or equivalent filter is provided.

- [ ] **Step 2: Add filter field**

In `SupportCoverageGapFilter`:

```go
ShowRaw bool `json:"show_raw"`
```

- [ ] **Step 3: Implement default filtering**

For default list:
- Keep open/done/rejected status behavior.
- Prefer analyzed gaps and gaps with active suggestions.
- Keep a debug/raw filter for internal review.

- [ ] **Step 4: Run tests**

Run:

```bash
cd server && go test ./internal/service -run 'TestSupportCoverage_ListGaps'
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/model/support_coverage.go server/internal/repository/support_coverage.go server/internal/service/support_coverage.go server/internal/service/support_coverage_test.go
git commit -m "feat(coverage): default to analyzed coverage gaps"
```

### Task 16: Keep event-time coverage as evidence, not primary UX

**Files:**
- Modify: `server/internal/service/support_coverage.go`
- Modify: `server/internal/service/support_coverage_clusterer.go`
- Test: `server/internal/service/support_coverage_test.go`

- [ ] **Step 1: Write tests**

Cover:
- `ProcessSupportEvent` still records/upserts evidence for high-signal events.
- Single low-confidence raw events do not become prominent visible gaps by default.
- Existing spike trigger behavior remains unchanged during transition.

- [ ] **Step 2: Add metadata marker to event-created gaps**

When `UpsertTopicGap` creates gaps from event-time detection, write metadata:

```json
{
  "source": "event_detection"
}
```

Daily analyzer gaps should write:

```json
{
  "source": "daily_conversation_analysis"
}
```

- [ ] **Step 3: Run tests**

Run:

```bash
cd server && go test ./internal/service -run 'TestSupportCoverage_ProcessSupportEvent|TestSupportCoverage_ListGaps'
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add server/internal/service/support_coverage.go server/internal/service/support_coverage_clusterer.go server/internal/service/support_coverage_test.go
git commit -m "feat(coverage): mark raw event gaps separately from daily findings"
```

---

## Phase 9 - Verification

### Task 17: End-to-end backend verification

**Files:**
- No code changes expected.

- [ ] **Step 1: Run targeted backend tests**

```bash
cd server && go test ./internal/repository ./internal/service ./internal/temporalapp
```

Expected: PASS.

- [ ] **Step 2: Run full backend tests**

```bash
cd server && go test ./...
```

Expected: PASS.

- [ ] **Step 3: Validate migrations**

```bash
cd server && go run ./cmd/migrate validate
```

Expected: PASS.

### Task 18: Frontend verification

**Files:**
- No code changes expected.

- [ ] **Step 1: Build frontend**

```bash
cd frontend && npm run build
```

Expected: PASS.

- [ ] **Step 2: Start dev server**

```bash
cd frontend && npm run dev
```

Expected: Vite prints a local URL.

- [ ] **Step 3: Browser check**

Open Coverage page and verify:
- Gap list shows analyzed findings, not raw one-off detections.
- Detail pane shows customer need, AI miss, human resolution, and recommended fix.
- Knowledge gap with create route shows draft preview and `Review Add`.
- Knowledge gap with update route shows suggested additions and target article.
- Website/content recommendations show target page URL and suggested copy changes.
- Data/action/policy/workflow gaps show diagnosis and implementation recommendations without docs apply controls.

### Task 19: Final cleanup

**Files:**
- Update if needed: `docs/specs/2026-04-27-coverage-gaps-redesign-design.md`
- Update if needed: `docs/plans/2026-04-27-coverage-gaps-redesign.md`

- [ ] **Step 1: Update docs**

Add a short note that daily conversation-level analysis is now the primary visible gap creator, while event-time detection remains evidence/spike support.

- [ ] **Step 2: Check git status**

```bash
git status --short
```

Expected: only intentional changes.

- [ ] **Step 3: Commit docs**

```bash
git add docs/specs/2026-04-27-coverage-gaps-redesign-design.md docs/plans/2026-04-27-coverage-gaps-redesign.md
git commit -m "docs(coverage): document daily analysis coverage model"
```

---

## Decisions Locked For This Plan

1. **Analyzer scope:** Analyze all conversations changed since the last successful cursor. The analyzer decides `has_gap`; the repository must not pre-filter to AI/human-touched conversations.
2. **No missed conversations:** Use cursor-based windows, 2-hour overlap, 10-minute settle delay, and `conversation_id + transcript_hash + analyzer_version` idempotency.
3. **Recommendation scope:** Data, action, policy, workflow, website/content, and docs gaps all receive recommendations. Only docs recommendations get the existing one-click draft/update flow in v1.
4. **Daily runtime:** Fixed at `04:30 UTC` daily. Do not add workspace timezone scheduling in v1.
5. **Knowledge scope:** Search the same customer-facing RAG surfaces inbox AI can use: external help center docs plus website/crawled support content sources. Do not search internal/private docs for customer-facing knowledge recommendations.
6. **Retrieval strategy:** Store compact live inbox AI retrieval traces. Daily analysis uses those traces to understand what AI saw, and conditionally re-runs retrieval only for conversations where current knowledge search can materially improve recommendations.
7. **Multi-fix bundles:** One gap can have multiple recommendations. The LLM may recommend up to 3 fixes and should mark primary/secondary priority.
8. **Outcome measurement:** Deferred. Store enough metadata to support later measurement, but do not add outcome dashboards or AI resolution metrics in this plan.
9. **Workspace selection:** Daily fanout includes only workspaces with changed non-spam support conversations since the last successful cursor minus overlap. Never fan out to every workspace unconditionally.
10. **Trace write path:** Live retrieval traces are written after AI message persistence in a detached, timeout-bounded goroutine with warning logs on failure.
11. **Transcript hash:** Use the SHA-256 transcript hash specification in Task 5. This hash is part of idempotency and must not be changed without bumping `analyzer_version`.
12. **Recommendation replacement:** Daily analysis replaces only `open` recommendation rows. Accepted, applied, and dismissed recommendations are preserved.
13. **Cost guardrails:** Per-workspace analysis concurrency is 4 conversations, workspace fanout concurrency is 10, and each workspace analyzes at most 1,000 changed conversations per run in v1.
