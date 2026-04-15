# Docs Coverage Loop V1 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build Docs Coverage Loop v1: turn failed support and self-service signals into rule-based docs gaps, article drafts/updates, recurrence measurement, and a weekly digest.

**Architecture:** Add a generic support coverage substrate (`support_resolution_events`, `support_coverage_gaps`, evidence, suggestions, snapshots) while exposing a docs-focused v1 UI under Support. Event emitters are best-effort and must never block the existing support AI, widget, help center, or docs publish flows. Gap pages read from gap/evidence/suggestion/snapshot tables, not from raw event scans.

**Tech Stack:** Go 1.24, Chi, GORM, PostgreSQL/Neon, dbmigrate SQL, React 19, TypeScript, TanStack Router, TanStack Query, shadcn/ui, TipTap JSON docs content.

---

## Release Boundary

Ship these together:

- Coverage event ledger
- Rule-based docs gap detection
- Gap inbox and detail UI
- Article draft generation
- Existing article update suggestions
- Self-service weak-article signals
- Agent "Docs issue?" Yes/No feedback
- Post-publish recurrence tracking
- Weekly digest

Do not ship in v1:

- LLM-based gap classification as source of truth
- Context/action/workflow/policy gap UI
- Simulation/replay
- Cross-workspace intelligence
- Autonomous actions
- Executive heatmap/dashboard

## Cross-Checked Existing Integration Points

- Support AI already has `SupportQueryPlanContract.IssueKey`, retrieval results, `response.CanAnswer`, confidence, source doc IDs, and escalation reasons in `server/internal/service/support_ai.go`.
- Support messages store AI metadata JSON in `model.SupportMessage.Metadata`.
- Support resolution happens through `SupportInboxService.UpdateConversationStatus` in `server/internal/service/support_inbox.go`.
- Widget sessions already have `SupportWidgetSession.ID`, `ConversationID`, and `AnonymousID`.
- Public help center search is handled by `DocsHandler.PublicSearchArticles` in `server/internal/handler/docs.go`.
- Public article feedback is handled by `DocsHandler.PublicSubmitFeedback` and `DocsHelpcenterService.SubmitFeedback*`.
- Widget article views are served by `SupportInboxWidget.GetHelpArticle`.
- Docs drafts can be created with `DocsDocumentService.Create` and content saved with `DocsContentService.Save`.
- Existing article updates must snapshot current content first with `DocsVersionService.CreateSnapshot`, then use `DocsContentService.Save`.
- Protected support routes use `/api/support/*`, `middleware.RequireWorkspaceID`, support module access, and permissions from `authorization.PermSupportRead/Edit/Admin`.

---

## File Structure

### Backend Create

- `server/internal/model/support_coverage.go`
  - Coverage models, constants, DTOs.
- `server/internal/repository/support_coverage.go`
  - GORM data access for events, topics, gaps, evidence, suggestions, snapshots, article relations.
- `server/internal/repository/support_coverage_test.go`
  - Repository lifecycle and dedupe tests.
- `server/internal/service/support_coverage.go`
  - Main orchestration service and public methods for APIs/emitters.
- `server/internal/service/support_coverage_rules.go`
  - Deterministic v1 gap rules and dedupe keys.
- `server/internal/service/support_coverage_drafts.go`
  - Article draft/update generation and apply workflow.
- `server/internal/service/support_coverage_digest.go`
  - Weekly digest summary and delivery.
- `server/internal/service/support_coverage_test.go`
  - Rule tests, draft/update tests with fake LLM, recurrence tests.
- `server/internal/handler/support_coverage.go`
  - Protected support coverage HTTP endpoints.
- `server/internal/handler/support_coverage_test.go`
  - Handler request/response and permission-safe behavior tests.
- `server/internal/dbmigrate/sql/202604150003_docs_coverage_loop_v1.sql`
  - Idempotent indexes, FK constraints, unique constraints, and join tables.

### Backend Modify

- `server/cmd/api/main.go`
  - AutoMigrate new models, instantiate repositories/services/handler, inject coverage recorder, start weekly digest ticker.
- `server/internal/router/router.go`
  - Add `SupportCoverage` handler to router aggregate and protected `/api/support/coverage` routes.
- `server/internal/service/support_ai.go`
  - Emit coverage events for AI attempt/retrieval/answer/handoff best-effort.
- `server/internal/service/support_inbox.go`
  - Emit conversation resolution events and support docs-issue feedback.
- `server/internal/service/support_inbox_widget.go`
  - Emit widget message and self-service article/open events where session context exists.
- `server/internal/handler/docs.go`
  - Emit public search and feedback events.
- `server/internal/handler/support_inbox_widget.go`
  - Pass widget article opened context to coverage service through support service.
- `server/internal/service/docs_helpcenter.go`
  - Emit publish events for recurrence measurement if the fix came from a coverage suggestion.
- `server/internal/authorization/permissions.go`
  - No new permission in v1. Use existing `support.read`, `support.edit`, `support.admin`.

### Frontend Create

- `frontend/src/lib/supportCoverageTypes.ts`
  - TS types for gaps, evidence, suggestions, summary.
- `frontend/src/lib/services/supportCoverageService.ts`
  - API methods.
- `frontend/src/hooks/queries/useSupportCoverage.ts`
  - TanStack query/mutation hooks.
- `frontend/src/pages/support/coverage/SupportCoveragePage.tsx`
  - Summary + gap inbox route page.
- `frontend/src/components/support/coverage/GapInboxTable.tsx`
- `frontend/src/components/support/coverage/GapDetailPanel.tsx`
- `frontend/src/components/support/coverage/GapEvidenceList.tsx`
- `frontend/src/components/support/coverage/GapSuggestionActions.tsx`
- `frontend/src/components/support/coverage/CreateCoverageDraftDialog.tsx`
- `frontend/src/components/support/coverage/ArticleUpdateDiff.tsx`
- `frontend/src/components/support/coverage/DocsIssuePrompt.tsx`
- `frontend/src/components/support/coverage/__tests__/GapInboxTable.test.tsx`
- `frontend/src/components/support/coverage/__tests__/DocsIssuePrompt.test.tsx`

### Frontend Modify

- `frontend/src/lib/queryKeys.ts`
  - Add `support.coverage*` keys.
- `frontend/src/hooks/queries/index.ts`
  - Export `useSupportCoverage`.
- `frontend/src/lib/services/supportService.ts`
  - Add conversation docs-issue feedback method if not kept in coverage service.
- `frontend/src/hooks/queries/useSupport.ts`
  - Invalidate coverage queries after docs issue feedback and resolution.
- `frontend/src/components/support/MessageThread.tsx`
  - Add `DocsIssuePrompt` after resolved AI/handoff conversations.
- `frontend/src/components/support/SupportInboxLayout.tsx`
  - Keep inbox unchanged. Coverage gets its own route/page.
- `frontend/src/routes/_authenticated/w/$slug/support.tsx`
  - Existing module route remains parent.
- Create `frontend/src/routes/_authenticated/w/$slug/support/coverage.tsx`
  - Route to `SupportCoveragePage`.

---

## Task 1: Schema, Models, And Migration

**Files:**
- Create: `server/internal/model/support_coverage.go`
- Create: `server/internal/dbmigrate/sql/202604150003_docs_coverage_loop_v1.sql`
- Modify: `server/cmd/api/main.go`
- Test: `server/internal/repository/support_coverage_test.go`

- [ ] **Step 1: Define model constants first**

Add constants for event types, gap categories, v1 gap types, failure modes, source signals, statuses, and suggestion types.

Required v1 values:

```go
const (
	SupportCoverageEventCustomerMessageCreated = "customer_message_created"
	SupportCoverageEventAIAttemptStarted      = "ai_attempt_started"
	SupportCoverageEventAIRetrievalCompleted  = "ai_retrieval_completed"
	SupportCoverageEventAIAnswerSent          = "ai_answer_sent"
	SupportCoverageEventAIHandoffTriggered    = "ai_handoff_triggered"
	SupportCoverageEventConversationResolved  = "conversation_resolved"
	SupportCoverageEventWidgetSearchPerformed = "widget_search_performed"
	SupportCoverageEventWidgetArticleOpened   = "widget_article_opened"
	SupportCoverageEventArticleFeedback       = "article_feedback_submitted"
)

const (
	SupportCoverageGapCategoryKnowledge  = "knowledge"
	SupportCoverageGapCategoryStructure  = "structure"
	SupportCoverageGapCategoryConflict   = "conflict"
	SupportCoverageGapCategoryContext    = "context"
	SupportCoverageGapCategoryAction     = "action"
	SupportCoverageGapCategoryWorkflow   = "workflow"
	SupportCoverageGapCategoryPolicy     = "policy"
	SupportCoverageGapCategoryEvaluation = "evaluation"
	SupportCoverageGapCategoryUnknown    = "unknown"
)

const (
	SupportCoverageV1GapMissingArticle              = "missing_article"
	SupportCoverageV1GapWeakArticle                 = "weak_article"
	SupportCoverageV1GapOutdatedOrConflictingArticle = "outdated_or_conflicting_article"
	SupportCoverageV1GapNeedsReview                 = "needs_review"
)
```

- [ ] **Step 2: Define core models**

Create these models:

- `SupportResolutionEvent`
- `SupportCoverageTopic`
- `SupportCoverageGap`
- `SupportGapEvidence`
- `SupportGapSuggestion`
- `SupportCoverageGapArticle`
- `SupportCoverageSnapshot`
- `SupportCoverageDigestDelivery`

Important model details:

- Use UUID primary keys with `default:gen_random_uuid()`.
- Use `jsonb` metadata fields as `json.RawMessage`.
- Store `workspace_id` on every table.
- Store `issue_key` on event, topic, and gap where available.
- Store `can_answer` and `can_resolve` as nullable strings (`yes`, `no`, `unknown`) or nullable bool-like string fields; do not use plain bool because unknown matters.
- Store `v1_gap_type` separately from broad `gap_category`.
- Store `dedupe_key` on gaps with a unique workspace-scoped index.
- Store suggestion `result_document_id` and `result_article_id` so published fixes can be measured.
- Store digest delivery rows by `workspace_id`, `week_start`, and `recipient_user_id` so weekly emails cannot duplicate after restarts or repeated sweeps.

- [ ] **Step 3: Write migration SQL**

Create `server/internal/dbmigrate/sql/202604150003_docs_coverage_loop_v1.sql`.

Use `CREATE TABLE IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, and `CREATE UNIQUE INDEX IF NOT EXISTS`.

Minimum required constraints:

```sql
CREATE UNIQUE INDEX IF NOT EXISTS idx_support_coverage_gaps_workspace_dedupe
  ON support_coverage_gaps (workspace_id, dedupe_key)
  WHERE status != 'merged';

CREATE INDEX IF NOT EXISTS idx_support_resolution_events_workspace_time
  ON support_resolution_events (workspace_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_support_coverage_gaps_workspace_status_seen
  ON support_coverage_gaps (workspace_id, status, last_seen_at DESC);

CREATE INDEX IF NOT EXISTS idx_support_gap_evidence_gap
  ON support_gap_evidence (gap_id, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_coverage_gap_articles_unique
  ON support_coverage_gap_articles (gap_id, document_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_coverage_digest_deliveries_workspace_week_recipient
  ON support_coverage_digest_deliveries (workspace_id, week_start, recipient_user_id);
```

Do not add cascading deletes from conversations/docs to gaps. Gaps are historical records.

- [ ] **Step 4: Add models to AutoMigrate**

Modify `server/cmd/api/main.go` AutoMigrate list near support models:

```go
&model.SupportResolutionEvent{},
&model.SupportCoverageTopic{},
&model.SupportCoverageGap{},
&model.SupportGapEvidence{},
&model.SupportGapSuggestion{},
&model.SupportCoverageGapArticle{},
&model.SupportCoverageSnapshot{},
&model.SupportCoverageDigestDelivery{},
```

- [ ] **Step 5: Write repository test setup**

Create `server/internal/repository/support_coverage_test.go`.

Use sqlite or test DB pattern already used in repository tests. AutoMigrate only these new models plus minimum referenced models if needed.

- [ ] **Step 6: Verify schema compiles**

Run:

```bash
cd server
go test ./internal/model ./internal/repository -run SupportCoverage -count=1
```

Expected: tests compile; new repository tests fail until repository is implemented.

- [ ] **Step 7: Commit**

```bash
git add server/internal/model/support_coverage.go server/internal/dbmigrate/sql/202604150003_docs_coverage_loop_v1.sql server/cmd/api/main.go server/internal/repository/support_coverage_test.go
git commit -m "feat: add support coverage schema"
```

---

## Task 2: Coverage Repository

**Files:**
- Create: `server/internal/repository/support_coverage.go`
- Modify: `server/internal/repository/support_coverage_test.go`

- [ ] **Step 1: Write failing repository tests**

Test these behaviors:

- `CreateEvent` stores append-only event metadata.
- `UpsertTopicByIssueKey` returns existing topic for same workspace + issue key.
- `UpsertGapByDedupeKey` increments `evidence_count`, updates `last_seen_at`, preserves status.
- `CreateEvidence` links a gap to a conversation/message/search/article event.
- `CreateSuggestion` links a gap to a proposed fix.
- `ListGaps` does not read `support_resolution_events`.
- `MergeGaps` marks source as `merged` and moves evidence/suggestions/articles to target.

- [ ] **Step 2: Implement repository methods**

Required methods:

```go
func (r *SupportCoverageRepository) CreateEvent(ctx context.Context, event *model.SupportResolutionEvent) error
func (r *SupportCoverageRepository) UpsertTopicByIssueKey(ctx context.Context, workspaceID, issueKey, title string) (*model.SupportCoverageTopic, error)
func (r *SupportCoverageRepository) UpsertGapByDedupeKey(ctx context.Context, gap *model.SupportCoverageGap) (*model.SupportCoverageGap, bool, error)
func (r *SupportCoverageRepository) CreateEvidence(ctx context.Context, evidence *model.SupportGapEvidence) error
func (r *SupportCoverageRepository) ListGaps(ctx context.Context, workspaceID string, filter model.SupportCoverageGapFilter) ([]model.SupportCoverageGapListItem, int64, error)
func (r *SupportCoverageRepository) GetGapDetail(ctx context.Context, workspaceID, gapID string) (*model.SupportCoverageGapDetail, error)
func (r *SupportCoverageRepository) CreateSuggestion(ctx context.Context, suggestion *model.SupportGapSuggestion) (*model.SupportGapSuggestion, error)
func (r *SupportCoverageRepository) UpdateSuggestionResult(ctx context.Context, suggestionID string, documentID *string, articleID *string, status string) error
func (r *SupportCoverageRepository) GetConversationCoverageState(ctx context.Context, workspaceID, conversationID string) (*model.SupportConversationCoverageState, error)
func (r *SupportCoverageRepository) CreateDigestDelivery(ctx context.Context, delivery *model.SupportCoverageDigestDelivery) error
func (r *SupportCoverageRepository) GetDigestDelivery(ctx context.Context, workspaceID string, weekStart time.Time, recipientUserID string) (*model.SupportCoverageDigestDelivery, error)
func (r *SupportCoverageRepository) UpdateGapStatus(ctx context.Context, workspaceID, gapID, status string) error
func (r *SupportCoverageRepository) ReclassifyGap(ctx context.Context, workspaceID, gapID, v1GapType string) error
func (r *SupportCoverageRepository) MergeGaps(ctx context.Context, workspaceID, sourceGapID, targetGapID string) error
```

- [ ] **Step 3: Keep queries scoped and indexed**

Every repository method must include `workspace_id` in WHERE clauses except direct internal updates where parent row was already loaded by workspace.

- [ ] **Step 4: Run repository tests**

```bash
cd server
go test ./internal/repository -run SupportCoverage -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/repository/support_coverage.go server/internal/repository/support_coverage_test.go
git commit -m "feat: add support coverage repository"
```

---

## Task 3: Rule-Based Gap Engine

**Files:**
- Create: `server/internal/service/support_coverage.go`
- Create: `server/internal/service/support_coverage_rules.go`
- Create: `server/internal/service/support_coverage_test.go`

- [ ] **Step 1: Write rule tests first**

Test deterministic v1 rules:

- `ai_handoff_triggered` + `failure_mode=no_retrieval` + same `issue_key` creates/upserts `missing_article`.
- `ai_handoff_triggered` + `failure_mode=weak_retrieval` + related article creates/upserts `weak_article`.
- `article_feedback_submitted` + `rating=not_helpful` creates/upserts `weak_article`.
- `widget_search_performed` + `result_count=0` creates/upserts `missing_article` when `issue_key` exists, otherwise `needs_review`.
- Agent docs issue Yes creates/upserts `needs_review` unless retrieval metadata makes it missing/weak.
- Unknown or missing issue key never creates a high-confidence missing article; use `needs_review`.

- [ ] **Step 2: Define event input DTO**

Add a service input like:

```go
type CoverageEventInput struct {
	WorkspaceID      string
	EventType        string
	ConversationID  *string
	MessageID       *string
	WidgetSessionID *string
	AnonymousID     *string
	DocumentID      *string
	ArticleID       *string
	ArticlePublicID *string
	IssueKey        string
	IssueSummary    string
	V1GapType       string
	FailureMode     string
	SourceSignal    string
	CanAnswer       string
	CanResolve      string
	Metadata        map[string]any
	OccurredAt      time.Time
}
```

- [ ] **Step 3: Implement `RecordEvent` and `RecordEventBestEffort`**

`RecordEvent` returns errors for tests/API use. `RecordEventBestEffort` logs and swallows errors for hot paths.

Do not let coverage writes break:

- support AI response generation
- widget message creation
- help center public search
- article feedback submission
- conversation resolution

- [ ] **Step 4: Implement dedupe key builder**

Dedupe rules:

```text
missing_article:
  workspace_id + v1_gap_type + issue_key

weak_article:
  workspace_id + v1_gap_type + issue_key + document_id/public_id

outdated_or_conflicting_article:
  workspace_id + v1_gap_type + issue_key + document_id/public_id

needs_review:
  workspace_id + v1_gap_type + issue_key if issue_key exists
  otherwise workspace_id + v1_gap_type + normalized short evidence hash
```

- [ ] **Step 5: Implement evidence creation**

Each gap upsert must attach evidence with:

- evidence type
- conversation/message/search/article identifiers
- source signal
- metadata
- excerpt where safe

Do not store entire conversation transcripts in evidence. Store message IDs plus short snippets.

- [ ] **Step 6: Run service tests**

```bash
cd server
go test ./internal/service -run SupportCoverage -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add server/internal/service/support_coverage.go server/internal/service/support_coverage_rules.go server/internal/service/support_coverage_test.go
git commit -m "feat: add docs coverage gap engine"
```

---

## Task 4: Emit Coverage Events From Existing Flows

**Files:**
- Modify: `server/internal/service/support_ai.go`
- Modify: `server/internal/service/support_inbox.go`
- Modify: `server/internal/service/support_inbox_widget.go`
- Modify: `server/internal/handler/docs.go`
- Modify: `server/internal/service/docs_helpcenter.go`
- Test: `server/internal/service/support_ai_coverage_test.go`
- Test: `server/internal/service/support_inbox_coverage_test.go`
- Test: `server/internal/handler/docs_coverage_test.go`

- [ ] **Step 1: Add coverage recorder interfaces**

Avoid hard dependencies that make tests brittle:

```go
type SupportCoverageRecorder interface {
	RecordEventBestEffort(ctx context.Context, input CoverageEventInput)
}
```

Add setters:

- `SupportAIService.SetCoverageRecorder(recorder SupportCoverageRecorder)`
- `SupportInboxService.SetCoverageRecorder(recorder SupportCoverageRecorder)`
- `DocsHelpcenterService.SetCoverageRecorder(recorder SupportCoverageRecorder)` if needed
- Docs handler may receive service through constructor only if handler-level search/feedback emits are cleaner.

- [ ] **Step 2: Emit support AI events**

In `SupportAIService.HandleIncomingMessage`:

- after planner: `ai_attempt_started`
- after retrieval: `ai_retrieval_completed` with result count, best score, source doc IDs, issue key
- after AI answer message created: `ai_answer_sent`
- before/after handoff: `ai_handoff_triggered` with reason/failure mode

Map failure modes conservatively:

- no results: `no_retrieval`
- low retrieval score or no cited source docs: `weak_retrieval`
- confidence below threshold: `low_confidence`
- planner handoff customer human: `customer_requested_human`
- same issue stalled: `stuck`

- [ ] **Step 3: Emit conversation resolution**

In `SupportInboxService.UpdateConversationStatus`, when status becomes `resolved`, emit `conversation_resolved`.

Include:

- previous status
- final flow state
- `ai_state`
- whether a human message exists after `AIEscalatedAt`

- [ ] **Step 4: Emit widget customer message/session events**

In `SupportInboxService.WidgetCreateMessage`, emit `customer_message_created` with `widget_session_id`, `anonymous_id`, `conversation_id`, and `message_id`.

- [ ] **Step 5: Emit public search events**

In `DocsHandler.PublicSearchArticles`, after results are loaded, emit `widget_search_performed` or `helpcenter_search_performed` using existing request context:

- workspace ID from help center config
- locale
- query
- space slug
- result count
- top document IDs/public IDs if available

If no coverage service is available, do nothing.

- [ ] **Step 6: Emit article opened events**

For widget article views, instrument `SupportInboxWidget.GetHelpArticle` or the service method it calls. Include widget session where available.

For public help center article views, defer unless identity/session is available. Do not add tracking cookies in v1.

- [ ] **Step 7: Emit article feedback events**

In `DocsHandler.PublicSubmitFeedback` and authenticated `SubmitArticleFeedback`, emit `article_feedback_submitted` with rating/helpful value, locale, article/document IDs.

- [ ] **Step 8: Test non-blocking behavior**

Use fake recorder that returns errors and assert existing service methods still succeed.

- [ ] **Step 9: Run tests**

```bash
cd server
go test ./internal/service ./internal/handler -run 'Coverage|SupportAI|SupportInbox|Docs' -count=1
```

Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add server/internal/service/support_ai.go server/internal/service/support_inbox.go server/internal/service/support_inbox_widget.go server/internal/handler/docs.go server/internal/service/docs_helpcenter.go server/internal/service/*coverage*_test.go server/internal/handler/docs_coverage_test.go
git commit -m "feat: emit docs coverage events"
```

---

## Task 5: Coverage HTTP API

**Files:**
- Create: `server/internal/handler/support_coverage.go`
- Modify: `server/internal/router/router.go`
- Modify: `server/cmd/api/main.go`
- Test: `server/internal/handler/support_coverage_test.go`

- [ ] **Step 1: Define endpoints**

Add protected routes under `/api/support/coverage`:

```text
GET  /api/support/coverage/summary
GET  /api/support/coverage/gaps
GET  /api/support/coverage/gaps/{gapId}
POST /api/support/coverage/gaps/{gapId}/status
POST /api/support/coverage/gaps/{gapId}/reclassify
POST /api/support/coverage/gaps/{gapId}/merge
POST /api/support/coverage/gaps/{gapId}/suggestions/article-draft
POST /api/support/coverage/gaps/{gapId}/suggestions/article-update
POST /api/support/coverage/suggestions/{suggestionId}/apply
GET  /api/support/coverage/conversations/{conversationId}/state
POST /api/support/coverage/conversations/{conversationId}/docs-issue
```

Permissions:

- Read summary/gaps/details: `support.read`
- Status, reclassify, merge, docs issue feedback: `support.edit`
- Generate/apply suggestions: require both `support.edit` and `docs.edit` with chained route middleware; service should still validate the target docs entity belongs to the workspace.
- Digest preview/admin later: `support.admin`.

- [ ] **Step 2: Add handler methods**

Handler must:

- read workspace ID from middleware
- decode JSON
- call service
- write JSON
- return 400 for validation, 404 for missing gap/suggestion, 500 only for unexpected errors

- [ ] **Step 3: Wire router aggregate**

Add `SupportCoverage *handler.SupportCoverageHandler` to `router.Handlers`.

In support route group:

```go
r.Route("/coverage", func(r chi.Router) {
	r.With(requirePerm(authorization.PermSupportRead)).Get("/summary", h.SupportCoverage.GetSummary)
	r.With(requirePerm(authorization.PermSupportRead)).Get("/gaps", h.SupportCoverage.ListGaps)
	r.With(requirePerm(authorization.PermSupportRead)).Get("/gaps/{gapId}", h.SupportCoverage.GetGap)
	r.With(requirePerm(authorization.PermSupportEdit)).Post("/gaps/{gapId}/status", h.SupportCoverage.UpdateGapStatus)
	r.With(requirePerm(authorization.PermSupportEdit)).Post("/gaps/{gapId}/reclassify", h.SupportCoverage.ReclassifyGap)
	r.With(requirePerm(authorization.PermSupportEdit)).Post("/gaps/{gapId}/merge", h.SupportCoverage.MergeGap)
	r.With(requirePerm(authorization.PermSupportEdit), requirePerm(authorization.PermDocsEdit)).Post("/gaps/{gapId}/suggestions/article-draft", h.SupportCoverage.CreateArticleDraftSuggestion)
	r.With(requirePerm(authorization.PermSupportEdit), requirePerm(authorization.PermDocsEdit)).Post("/gaps/{gapId}/suggestions/article-update", h.SupportCoverage.CreateArticleUpdateSuggestion)
	r.With(requirePerm(authorization.PermSupportEdit), requirePerm(authorization.PermDocsEdit)).Post("/suggestions/{suggestionId}/apply", h.SupportCoverage.ApplySuggestion)
	r.With(requirePerm(authorization.PermSupportRead)).Get("/conversations/{conversationId}/state", h.SupportCoverage.GetConversationState)
	r.With(requirePerm(authorization.PermSupportEdit)).Post("/conversations/{conversationId}/docs-issue", h.SupportCoverage.SubmitDocsIssueFeedback)
})
```

- [ ] **Step 4: Wire main**

Instantiate:

- `supportCoverageRepo := repository.NewSupportCoverageRepository(db)`
- `supportCoverageService := service.NewSupportCoverageService(supportCoverageRepo, supportConversationRepo, supportMessageRepo, docsDocumentService, docsContentService, docsVersionService, docsSpaceRepo, docsCollectionRepo, docsHelpCenterRepo, llmProvider, emailClient, workspaceRepo, userRepo, appBaseURL)`
- `handler.NewSupportCoverageHandler(supportCoverageService)`

Constructor dependencies should be explicit:

- coverage repository
- support conversation repository
- support message repository
- docs document service
- docs content service
- docs version service
- docs space and collection repositories or services for target validation
- docs help center repository for article IDs/PublicIDs
- LLM provider for article draft/update writing
- email client for digest delivery
- workspace and user repositories for digest recipients
- app base URL for digest links

Inject recorder into support AI, support inbox, docs/help center.

- [ ] **Step 5: Handler tests**

Test:

- list gaps requires workspace
- invalid status returns 400
- docs issue Yes creates event/gap
- conversation state returns whether docs issue feedback already exists
- missing gap returns 404

- [ ] **Step 6: Run handler tests**

```bash
cd server
go test ./internal/handler -run SupportCoverage -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add server/internal/handler/support_coverage.go server/internal/handler/support_coverage_test.go server/internal/router/router.go server/cmd/api/main.go
git commit -m "feat: add support coverage api"
```

---

## Task 6: Article Draft And Update Suggestions

**Files:**
- Create: `server/internal/service/support_coverage_drafts.go`
- Modify: `server/internal/service/support_coverage.go`
- Modify: `server/cmd/api/main.go`
- Test: `server/internal/service/support_coverage_drafts_test.go`

- [ ] **Step 1: Define generation contract**

Use LLM only for draft/update writing, not gap classification.

Input evidence:

- customer question snippets
- human answer snippets
- related article title/content if update
- suggested target space/collection

Output:

```go
type CoverageArticleDraft struct {
	Title              string
	Excerpt            string
	Content            json.RawMessage // TipTap JSON
	CommonQuestions    []string
	EvidenceSummary    string
	SuggestedSpaceID   string
	SuggestedCollectionID *string
}
```

- [ ] **Step 2: Generate safe TipTap JSON**

Server should construct TipTap JSON from structured LLM output rather than trusting arbitrary JSON from the model.

Request model output as sections:

```json
{
  "title": "...",
  "excerpt": "...",
  "common_questions": ["..."],
  "sections": [
    { "heading": "...", "paragraphs": ["...", "..."] }
  ]
}
```

Convert to:

```json
{
  "type": "doc",
  "content": [
    { "type": "heading", "attrs": { "level": 2 }, "content": [{ "type": "text", "text": "..." }] },
    { "type": "paragraph", "content": [{ "type": "text", "text": "..." }] }
  ]
}
```

- [ ] **Step 3: Create draft article from gap**

Flow:

1. Load gap detail and evidence.
2. Generate draft preview.
3. Create `SupportGapSuggestion` with `suggestion_type=create_article`, `status=draft`.
4. On apply:
   - call `DocsDocumentService.Create`
   - call `DocsContentService.Save`
   - update suggestion with `result_document_id`
   - update gap status to `drafted`

Do not publish externally in v1.

- [ ] **Step 4: Suggest update for existing article**

Flow:

1. Load existing document/content.
2. Create snapshot with `DocsVersionService.CreateSnapshot` before applying any update.
3. Store proposed content and evidence summary in suggestion metadata.
4. UI shows diff before apply.
5. On apply, call `DocsContentService.Save`.
6. Update suggestion and gap status.

- [ ] **Step 5: Do not mutate docs without explicit apply**

Generation endpoints create suggestions/previews. Only apply endpoint changes docs.

- [ ] **Step 6: Tests**

Test with fake LLM:

- draft generation creates suggestion but not document
- apply creates draft document and content
- update apply snapshots existing content before save
- evidence snippets are included in suggestion metadata
- no publish occurs

- [ ] **Step 7: Run tests**

```bash
cd server
go test ./internal/service -run SupportCoverageDrafts -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add server/internal/service/support_coverage_drafts.go server/internal/service/support_coverage_drafts_test.go server/internal/service/support_coverage.go server/cmd/api/main.go
git commit -m "feat: generate docs fixes from coverage gaps"
```

---

## Task 7: Recurrence Measurement And Snapshots

**Files:**
- Modify: `server/internal/service/support_coverage.go`
- Create or update: `server/internal/service/support_coverage_snapshots.go`
- Test: `server/internal/service/support_coverage_test.go`

- [ ] **Step 1: Define recurrence window**

V1 recurrence metrics:

- `affected_conversations_7d`
- `affected_conversations_30d`
- `handoffs_7d`
- `handoffs_30d`
- `no_result_searches_7d`
- `no_result_searches_30d`
- `post_fix_recurrences`

- [ ] **Step 2: Link published/applied fixes to gaps**

When a suggestion is applied:

- store result document/article IDs
- store `applied_at`
- if an existing article update is applied, store pre-update snapshot/version ID in metadata

- [ ] **Step 3: Snapshot job**

Implement:

```go
func (s *SupportCoverageService) RefreshWorkspaceSnapshots(ctx context.Context, workspaceID string, now time.Time) error
func (s *SupportCoverageService) RefreshAllWorkspaceSnapshots(ctx context.Context, now time.Time, limit int) error
```

Do not scan raw events for page loads. Use the snapshot table for summary metrics.

- [ ] **Step 4: Wire periodic snapshot refresh**

In `server/cmd/api/main.go`, add a low-frequency ticker, e.g. hourly, to refresh coverage snapshots for active workspaces.

Use repository workspace list helper if available. If no efficient active workspace list exists, v1 can refresh only when digest runs and when gap pages are requested with stale snapshots.

- [ ] **Step 5: Tests**

Test before/after applied suggestion counts:

- events before `applied_at` counted as before
- events after `applied_at` counted as recurrence
- page summary reads snapshot rows

- [ ] **Step 6: Run tests**

```bash
cd server
go test ./internal/service -run 'SupportCoverage.*Snapshot|SupportCoverage.*Recurrence' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add server/internal/service/support_coverage*.go server/internal/service/support_coverage_test.go server/cmd/api/main.go
git commit -m "feat: measure coverage fix recurrence"
```

---

## Task 8: Weekly Digest

**Files:**
- Create: `server/internal/service/support_coverage_digest.go`
- Modify: `server/cmd/api/main.go`
- Test: `server/internal/service/support_coverage_digest_test.go`

- [ ] **Step 1: Write digest tests**

Test:

- digest selects workspace admins and support admins
- digest includes top gaps, suggested drafts/updates, fixed gaps still recurring
- digest does not send if no meaningful gaps
- digest records `SupportCoverageDigestDelivery` rows to avoid duplicates per workspace, week, and recipient

- [ ] **Step 2: Recipient selection**

V1 default:

- workspace-wide
- Monday weekly
- recipients are users with admin/owner/manager role or effective `support.admin`

If effective permission checks are hard in batch jobs, use roles that map to `support.admin` in `authorization/rbac.go`: manager, admin, owner.

- [ ] **Step 3: Email rendering**

Use existing `email.Client` if configured. If not configured, log the digest.

Before sending to a recipient, check `support_coverage_digest_deliveries` for that workspace/week/recipient. After a successful send, insert the delivery row. If email is not configured and the digest is only logged, also insert a delivery row in non-test environments so the sweep does not log the same digest every 6 hours.

Subject:

```text
Docs coverage: {N} support gaps found this week
```

Body must include:

- top 5 docs gaps
- affected conversation count
- no-result search count
- suggested action
- direct app links to coverage gap detail

- [ ] **Step 4: Wire weekly sweep**

In `server/cmd/api/main.go`, add a background ticker similar to existing notification digest sweep.

Run once on startup if desired, then every 6 hours check whether a workspace is due. Do not send more than once per workspace per week.

- [ ] **Step 5: Tests**

```bash
cd server
go test ./internal/service -run SupportCoverageDigest -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add server/internal/service/support_coverage_digest.go server/internal/service/support_coverage_digest_test.go server/cmd/api/main.go
git commit -m "feat: send docs coverage weekly digest"
```

---

## Task 9: Frontend Types, Service, And Hooks

**Files:**
- Create: `frontend/src/lib/supportCoverageTypes.ts`
- Create: `frontend/src/lib/services/supportCoverageService.ts`
- Create: `frontend/src/hooks/queries/useSupportCoverage.ts`
- Modify: `frontend/src/lib/queryKeys.ts`
- Modify: `frontend/src/hooks/queries/index.ts`

- [ ] **Step 1: Define TypeScript types**

Types:

- `SupportCoverageGapListItem`
- `SupportCoverageGapDetail`
- `SupportGapEvidence`
- `SupportGapSuggestion`
- `SupportCoverageSummary`
- `UpdateGapStatusRequest`
- `ReclassifyGapRequest`
- `MergeGapRequest`
- `CreateArticleDraftRequest`
- `CreateArticleUpdateRequest`
- `DocsIssueFeedbackRequest`

- [ ] **Step 2: Add service methods**

Use `/support/coverage/*` routes and `workspace_id` query param.

Methods:

```ts
listGaps(workspaceId, filters)
getGap(workspaceId, gapId)
getSummary(workspaceId)
getConversationState(workspaceId, conversationId)
updateGapStatus(workspaceId, gapId, status)
reclassifyGap(workspaceId, gapId, v1GapType)
mergeGap(workspaceId, gapId, targetGapId)
createArticleDraft(workspaceId, gapId, payload)
createArticleUpdate(workspaceId, gapId, payload)
applySuggestion(workspaceId, suggestionId)
submitDocsIssueFeedback(workspaceId, conversationId, docsIssue)
```

- [ ] **Step 3: Add query keys**

Add under `queryKeys.support`:

```ts
coverageSummary: (wsId: string) => ['support', wsId, 'coverage', 'summary'] as const,
coverageGaps: (wsId: string, filters?: Record<string, unknown>) => ['support', wsId, 'coverage', 'gaps', filters] as const,
coverageGap: (wsId: string, gapId: string) => ['support', wsId, 'coverage', 'gaps', gapId] as const,
coverageConversationState: (wsId: string, conversationId: string) => ['support', wsId, 'coverage', 'conversations', conversationId, 'state'] as const,
```

- [ ] **Step 4: Add hooks**

Implement queries and mutations. Invalidate:

- gap list
- gap detail
- summary
- docs document/content queries after apply suggestion

- [ ] **Step 5: Type check**

```bash
cd frontend
npm run build
```

Expected: build may fail until UI routes are added; type errors should only reference missing components/routes.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/supportCoverageTypes.ts frontend/src/lib/services/supportCoverageService.ts frontend/src/hooks/queries/useSupportCoverage.ts frontend/src/lib/queryKeys.ts frontend/src/hooks/queries/index.ts
git commit -m "feat: add support coverage frontend api"
```

---

## Task 10: Coverage UI

**Files:**
- Create: `frontend/src/pages/support/coverage/SupportCoveragePage.tsx`
- Create: `frontend/src/components/support/coverage/GapInboxTable.tsx`
- Create: `frontend/src/components/support/coverage/GapDetailPanel.tsx`
- Create: `frontend/src/components/support/coverage/GapEvidenceList.tsx`
- Create: `frontend/src/components/support/coverage/GapSuggestionActions.tsx`
- Create: `frontend/src/components/support/coverage/CreateCoverageDraftDialog.tsx`
- Create: `frontend/src/components/support/coverage/ArticleUpdateDiff.tsx`
- Create: `frontend/src/routes/_authenticated/w/$slug/support/coverage.tsx`
- Test: `frontend/src/components/support/coverage/__tests__/GapInboxTable.test.tsx`

- [ ] **Step 1: Build route**

Add TanStack route:

```tsx
export const Route = createFileRoute('/_authenticated/w/$slug/support/coverage')({
  component: SupportCoverageRoute,
})
```

Use existing support parent module access from `/support`.

- [ ] **Step 2: Build summary header**

Show:

- new gaps this week
- top recurring gaps
- gaps fixed this week
- handoffs after published fixes

Use simple numbers and links. Do not build a coverage score.

- [ ] **Step 3: Build gap inbox table**

Columns:

- Gap
- Type
- Affected conversations
- Related article
- Suggested fix
- Status
- Last seen

Rows open detail panel.

- [ ] **Step 4: Build detail panel**

Sections:

- why this gap exists
- customer question examples
- AI failure reasons
- human resolution examples
- no-result searches
- related articles
- suggestions

Actions:

- create draft
- suggest article update
- ignore
- mark fixed
- reclassify
- merge

- [ ] **Step 5: Build draft/update dialogs**

Create draft dialog requires user to choose target docs space and optional collection.

Update dialog requires a related existing article/document.

Never apply generated content without user clicking Apply.

- [ ] **Step 6: UI tests**

Test:

- gap rows render
- selecting row opens evidence
- missing article gap shows Create draft
- weak article gap shows Suggest update
- ignored status disables draft/update actions

- [ ] **Step 7: Run frontend checks**

```bash
cd frontend
npm exec vitest run src/components/support/coverage/__tests__/GapInboxTable.test.tsx
npm run build
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/pages/support/coverage frontend/src/components/support/coverage frontend/src/routes/_authenticated/w/\\$slug/support/coverage.tsx
git commit -m "feat: add docs coverage inbox UI"
```

---

## Task 11: Agent Docs-Issue Feedback Prompt

**Files:**
- Create: `frontend/src/components/support/coverage/DocsIssuePrompt.tsx`
- Modify: `frontend/src/components/support/MessageThread.tsx`
- Modify: `frontend/src/hooks/queries/useSupportCoverage.ts`
- Test: `frontend/src/components/support/coverage/__tests__/DocsIssuePrompt.test.tsx`

- [ ] **Step 1: Add prompt component**

Prompt copy:

```text
AI couldn't answer this. Docs issue?
```

Buttons:

- Yes
- No

No taxonomy dropdown in v1.

- [ ] **Step 2: Show only when appropriate**

Show when:

- conversation is resolved
- conversation had `ai_state=escalated` or `flow_state=waiting_for_human/assigned_to_human/resolved_by_human`
- user has support edit permission if available; otherwise rely on API permission
- `useSupportCoverageConversationState` says docs issue feedback was not already submitted

Do not show for AI-resolved conversations.

- [ ] **Step 3: Submit feedback**

Call:

```ts
supportCoverageService.submitDocsIssueFeedback(workspaceId, conversationId, { docs_issue: true | false })
```

On Yes:

- invalidate coverage summary/gaps
- toast "Added to docs coverage"

On No:

- toast "Marked as not a docs issue"

- [ ] **Step 4: Tests**

Test:

- prompt renders for resolved AI handoff
- clicking Yes calls mutation
- clicking No calls mutation
- prompt hides while pending

- [ ] **Step 5: Run frontend tests**

```bash
cd frontend
npm exec vitest run src/components/support/coverage/__tests__/DocsIssuePrompt.test.tsx
npm run build
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/support/coverage/DocsIssuePrompt.tsx frontend/src/components/support/MessageThread.tsx frontend/src/hooks/queries/useSupportCoverage.ts frontend/src/components/support/coverage/__tests__/DocsIssuePrompt.test.tsx
git commit -m "feat: collect docs issue feedback from support"
```

---

## Task 12: End-To-End Verification And Hardening

**Files:**
- Modify as needed only for bugs found during verification.

- [ ] **Step 1: Run backend targeted tests**

```bash
cd server
go test ./internal/repository ./internal/service ./internal/handler -run 'SupportCoverage|SupportAI|SupportInbox|Docs' -count=1
```

Expected: PASS.

- [ ] **Step 2: Run full backend tests if targeted tests pass**

```bash
cd server
go test ./...
```

Expected: PASS.

- [ ] **Step 3: Run frontend checks**

```bash
cd frontend
npm exec vitest run src/components/support/coverage/__tests__/GapInboxTable.test.tsx src/components/support/coverage/__tests__/DocsIssuePrompt.test.tsx
npm run build
```

Expected: PASS.

- [ ] **Step 4: Manual QA scenario**

Use local dev environment:

1. Enable AI-first support for a workspace.
2. Send widget question with no relevant docs.
3. Confirm AI escalates.
4. Confirm coverage gap appears as missing/needs-review.
5. Resolve conversation as human.
6. Click "Docs issue? Yes".
7. Confirm evidence count increases.
8. Generate article draft.
9. Confirm docs draft is created and not published.
10. Publish/update docs manually.
11. Send similar widget question.
12. Confirm recurrence/metrics update after snapshot refresh.

- [ ] **Step 5: Check non-regression paths**

Verify existing flows still work:

- support widget message creation
- AI answer success path
- AI handoff path
- support inbox resolve/unresolve
- public help center search
- public article feedback
- docs document create/save/publish
- widget article open

- [ ] **Step 6: Performance sanity**

Confirm:

- gap list endpoint does not scan `support_resolution_events`
- event insert indexes are not overbroad
- event emitters are best-effort and log only on failure
- digest cannot send duplicate weekly emails

- [ ] **Step 7: Final commit**

```bash
git status --short
git add .
git commit -m "feat: ship docs coverage loop v1"
```

Only use this final commit if previous task commits were not made. Prefer the smaller task commits above.

---

## Rollout Notes

- Hide the `/support/coverage` route behind the support module and normal support permissions. No separate feature flag is required unless product wants a staged rollout.
- If staging data volume is low, seed test gaps through the API or a one-off dev script. Do not add permanent seed data.
- Keep all event recording best-effort in v1. The product should degrade to "no coverage analytics" rather than breaking support.
- Do not run raw event backfills until v1 is stable. Backfill can be a follow-up if needed.

## Verification Checklist Before Merge

- [ ] `server/internal/dbmigrate/sql/202604150003_docs_coverage_loop_v1.sql` is idempotent.
- [ ] New models are included in AutoMigrate.
- [ ] All new protected routes are workspace-scoped and permission-scoped.
- [ ] No public endpoint leaks workspace/customer data in event responses.
- [ ] Coverage event emitters do not return errors to existing support/docs flows.
- [ ] Gap inbox reads from gap/evidence/suggestion tables, not raw events.
- [ ] Article draft/update apply never auto-publishes.
- [ ] Existing docs slug/PublicID behavior is untouched.
- [ ] Existing support AI tests still pass.
- [ ] Existing docs/help center tests still pass.
