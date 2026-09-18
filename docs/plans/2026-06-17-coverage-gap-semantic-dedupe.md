# Coverage gap semantic deduplication plan


**Goal:** Make support coverage gaps durable semantic problem records that dedupe correctly during daily ingestion, rank by explainable impact, and distinguish missing content from retrieval/content-routing failures.

**Architecture:** Use the existing `support_coverage_conversation_analyses` table as the staging/materialization surface. Store canonical embeddings on both findings and gaps with provider/model/version/dimension metadata, use pgvector for gap-to-gap nearest-neighbor search, cluster same-run findings before persistence, and make evidence insertion the only source of evidence-count increments. Keep historical gap merges conservative and use gap-to-KB matching to classify coverage failure modes.

**Original stack (current Go baseline is 1.25.0):** Go 1.24, GORM, PostgreSQL/pgvector, Temporal activities, OpenAI embeddings through `llm.EmbeddingProvider`, Chi handlers, React/Vite/TypeScript coverage UI.

---

## Source review — 2026-09-18

This is the original implementation checklist, not an outstanding task list or a migration runbook. The schema, materializer, semantic helpers, repository methods, daily-analysis wiring, rebuild service, and coverage UI exist in this checkout. Do not recreate an applied migration from the illustrative SQL below.

- [Semantic constants](../../server/internal/service/support_coverage_semantic.go) use 0.90 attachment, 0.78 suggestion, and 0.88 same-run thresholds, with canonical semantic fields and compatibility gates. Same-run union additionally checks all member-pair compatibility to avoid transitive over-merging.
- [The materializer](../../server/internal/service/support_coverage_materializer.go) reads at most 1,000 unmaterialized findings per invocation. Contrary to the original fail-and-retry requirement, a missing provider, embedding error, or response-count mismatch produces degraded findings without vectors and continues. Exact normalized customer needs can still group; this is not equivalent to semantic nearest-neighbor matching.
- The historical cluster rebuild has a stricter embedding-failure path and reports failure metadata. Do not infer the daily materializer's behavior from rebuild status, or vice versa.
- Evidence keys prevent duplicate insertion, but evidence insertion, count increment, analysis stamping, and recurrence updates are separate calls. The proposed one-transaction materialization boundary is not present in this path; the source does not justify an all-or-nothing retry guarantee across those steps.
- Closed-gap matching uses a 90-day window. Recurrence reopens a done gap after three post-close evidence increments; the proposed alternative threshold of two distinct customers is not implemented by that counter. Split-review detection samples up to 20 analysis embeddings and requires at least six vectors; it only flags review.
- [Impact ranking](../../server/internal/repository/support_coverage.go) currently uses a linear weighted formula: recent distinct conversations × 4, recent distinct customers × 12, total evidence, confidence × 2, and knowledge/actionability bonuses. The logarithmic formula below is illustrative, not current scoring.
- Knowledge proximity can classify a sufficiently relevant Docs match as `no_retrieval`, or another content match as `weak_retrieval`. This is a heuristic; proximity alone is not proof of the historical retrieval failure. The run's gap count tracks findings, not necessarily newly created gaps.
- No embedding request, database migration, historical backfill, cluster merge, or runtime test was executed for this review. Historical expected test results and rollout steps below are not current verification evidence.

## Original implementation checklist

## Core Decisions

- Gap identity embedding text is canonical semantic text: `customer_need + canonical_title + topic/title`.
- Do not embed raw evidence, failure mode, source signal, category, or gap kind into the primary gap vector.
- Gap-to-gap dedupe is semantic NN plus business gates. Do not use hybrid lexical ranking for dedupe.
- Gap-to-KB matching may use hybrid lexical + vector because product names, URLs, and error codes matter there.
- New finding to existing gap attachment can be automatic at high confidence; existing gap to existing gap merge remains conservative and usually creates review suggestions.
- Historical backfill is background/repair work. Daily ingestion must synchronously embed the batch before materializing gaps.
- A vector is either real or `NULL`; never store fake vectors or lexical substitutes in vector columns.
- Temporal retries must be safe: deterministic analysis staging, deterministic gap keys, deterministic evidence keys, and evidence-count bumps only after a successful evidence insert.

## File Map

### Backend Models And Migrations

- Modify: `server/internal/model/support_coverage.go`
  - Add gap embedding metadata fields.
  - Add evidence `SourceKey`.
  - Add impact/explanation fields to list/detail DTOs as read-only fields.
- Modify: `server/internal/model/support_coverage_analysis.go`
  - Add `CanonicalTitle`.
  - Add finding embedding metadata fields.
  - Add `MaterializationMetadata`.
- Modify: `server/internal/model/docs_embedding.go`
  - Add embedding provider/model/version/dim metadata to `DocsChunk`.
- Modify: `server/internal/model/support_content_source.go`
  - Add embedding provider/model/version/dim metadata to `SupportContentChunk`.
- Create: `server/internal/dbmigrate/sql/202606170003_coverage_gap_semantic_embeddings.sql`
  - Schema additions, vector indexes, uniqueness indexes, and metadata backfill defaults.

### Backend Repositories

- Modify: `server/internal/repository/support_coverage.go`
  - Add no-bump gap upsert methods.
  - Add vector NN search for gaps.
  - Add idempotent evidence insert by `source_key`.
  - Add transaction helpers for materialization.
  - Add impact query fields.
- Modify: `server/internal/repository/support_coverage_analysis.go`
  - Add unmaterialized finding listing.
  - Add finding embedding persistence.
  - Add transactional `gap_id` stamping.
- Modify: `server/internal/repository/docs_chunk.go`
  - Filter vector searches by embedding metadata.
  - Persist chunk embedding metadata.
- Modify: `server/internal/repository/support_content_chunk.go`
  - Filter vector searches by embedding metadata.
  - Persist chunk embedding metadata.

### Backend Services

- Create: `server/internal/service/support_coverage_semantic.go`
  - Canonical embedding text builder, text hash, version constants, cosine helpers, deterministic cluster key helpers.
- Create: `server/internal/service/support_coverage_materializer.go`
  - Batch materialization of unmaterialized analysis rows.
  - Embedding generation, existing-gap matching, same-run clustering, transaction orchestration.
- Modify: `server/internal/service/support_coverage_daily_analyzer.go`
  - Store final analysis rows with materialization payload.
  - Stop calling `UpsertFinding` inside each conversation worker.
  - Call materializer once per workspace/run.
  - Keep `UpsertFinding` only for legacy/manual paths or refactor it through the new materializer.
- Modify: `server/internal/service/support_coverage_cluster_rebuild.go`
  - Backfill missing gap embeddings.
  - Use vector candidate search for historical cluster suggestions.
  - Keep conservative auto-merge gates.
- Modify: `server/internal/service/support_coverage_knowledge_matcher.go`
  - Allow gap-to-KB matching from an existing gap embedding.
  - Return enough match detail to reclassify failure mode and explain KB proximity.
- Modify: `server/internal/service/docs_embedding.go`
  - Populate chunk embedding metadata.
- Modify: `server/internal/service/support_content_sync.go`
  - Populate support content chunk embedding metadata.

### Wiring

- Modify: `server/cmd/api/main.go`
  - Inject `supportEmbeddingProvider` and `cfg.OpenAIEmbeddingModel` into the daily analyzer/materializer.
  - Register the coverage gap vector index next to existing vector indexes if kept in startup index list.
- Modify: `server/cmd/temporal-worker/main.go`
  - Same daily analyzer/materializer wiring as API.

### Frontend

- Modify: `frontend/src/lib/supportCoverageTypes.ts`
  - Add impact score/explanation, customer counts, KB proximity, embedding status fields.
- Modify: `frontend/src/components/support/coverage/GapList.tsx`
  - Show explainable impact signals in each row.
- Modify: `frontend/src/components/support/coverage/GapDetailPane.tsx`
  - Show KB proximity/reclassification and merge/split signals.
- Modify: `frontend/src/pages/support/coverage/SupportCoveragePage.tsx`
  - Preserve Rebuild Gap Clusters as repair/backfill action with better status text.

---

## Task 1: Schema And Model Foundation

**Files:**
- Create: `server/internal/dbmigrate/sql/202606170003_coverage_gap_semantic_embeddings.sql`
- Modify: `server/internal/model/support_coverage.go`
- Modify: `server/internal/model/support_coverage_analysis.go`
- Modify: `server/internal/model/docs_embedding.go`
- Modify: `server/internal/model/support_content_source.go`
- Test: `server/internal/dbmigrate` validation through CLI
- Test: affected repository test table setup files under `server/internal/repository/*_test.go`

- [ ] **Step 1: Write schema expectations in tests/setup**

Update SQLite test schemas that explicitly create coverage tables:

- `server/internal/repository/support_coverage_test.go`
- `server/internal/repository/support_coverage_analysis_test.go`
- `server/internal/service/support_coverage_daily_analyzer_test.go`
- `server/internal/service/support_coverage_cluster_rebuild_test.go`
- any tests found by `rg -n "CREATE TABLE support_coverage_gaps|CREATE TABLE support_coverage_conversation_analyses|CREATE TABLE support_gap_evidence" server/internal`

Add columns that tests need:

```sql
-- support_coverage_gaps
embedding TEXT,
embedding_provider TEXT NOT NULL DEFAULT '',
embedding_model TEXT NOT NULL DEFAULT '',
embedding_version TEXT NOT NULL DEFAULT '',
embedding_dimensions INTEGER NOT NULL DEFAULT 0,
embedding_text_hash TEXT NOT NULL DEFAULT '',
embedding_updated_at DATETIME,
nearest_content_score REAL NOT NULL DEFAULT 0,
nearest_content_document_id TEXT,
nearest_content_title TEXT NOT NULL DEFAULT '',
nearest_content_checked_at DATETIME,
impact_score REAL NOT NULL DEFAULT 0

-- support_coverage_conversation_analyses
canonical_title TEXT NOT NULL DEFAULT '',
embedding TEXT,
embedding_provider TEXT NOT NULL DEFAULT '',
embedding_model TEXT NOT NULL DEFAULT '',
embedding_version TEXT NOT NULL DEFAULT '',
embedding_dimensions INTEGER NOT NULL DEFAULT 0,
embedding_text_hash TEXT NOT NULL DEFAULT '',
embedding_updated_at DATETIME,
materialization_metadata JSON NOT NULL DEFAULT '{}'

-- support_gap_evidence
source_key TEXT NOT NULL DEFAULT ''
```

- [ ] **Step 2: Add the migration**

Create `server/internal/dbmigrate/sql/202606170003_coverage_gap_semantic_embeddings.sql`:

```sql
ALTER TABLE support_coverage_gaps
  ADD COLUMN IF NOT EXISTS embedding vector(1536),
  ADD COLUMN IF NOT EXISTS embedding_provider text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS embedding_model text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS embedding_version text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS embedding_dimensions integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS embedding_text_hash text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS embedding_updated_at timestamptz,
  ADD COLUMN IF NOT EXISTS nearest_content_score double precision NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS nearest_content_document_id uuid,
  ADD COLUMN IF NOT EXISTS nearest_content_title text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS nearest_content_checked_at timestamptz,
  ADD COLUMN IF NOT EXISTS impact_score double precision NOT NULL DEFAULT 0;

ALTER TABLE support_coverage_conversation_analyses
  ADD COLUMN IF NOT EXISTS canonical_title text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS embedding vector(1536),
  ADD COLUMN IF NOT EXISTS embedding_provider text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS embedding_model text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS embedding_version text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS embedding_dimensions integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS embedding_text_hash text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS embedding_updated_at timestamptz,
  ADD COLUMN IF NOT EXISTS materialization_metadata jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE support_gap_evidence
  ADD COLUMN IF NOT EXISTS source_key text NOT NULL DEFAULT '';

ALTER TABLE docs_chunks
  ADD COLUMN IF NOT EXISTS embedding_provider text NOT NULL DEFAULT 'openai',
  ADD COLUMN IF NOT EXISTS embedding_model text NOT NULL DEFAULT 'text-embedding-3-small',
  ADD COLUMN IF NOT EXISTS embedding_version text NOT NULL DEFAULT 'content-chunk-v1',
  ADD COLUMN IF NOT EXISTS embedding_dimensions integer NOT NULL DEFAULT 1536;

ALTER TABLE support_content_chunks
  ADD COLUMN IF NOT EXISTS embedding_provider text NOT NULL DEFAULT 'openai',
  ADD COLUMN IF NOT EXISTS embedding_model text NOT NULL DEFAULT 'text-embedding-3-small',
  ADD COLUMN IF NOT EXISTS embedding_version text NOT NULL DEFAULT 'content-chunk-v1',
  ADD COLUMN IF NOT EXISTS embedding_dimensions integer NOT NULL DEFAULT 1536;

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_gap_evidence_workspace_source_key
  ON support_gap_evidence(workspace_id, source_key)
  WHERE source_key <> '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_coverage_conversation_analyses_transcript
  ON support_coverage_conversation_analyses(workspace_id, conversation_id, transcript_hash, analyzer_version);

CREATE INDEX IF NOT EXISTS idx_support_coverage_gaps_embedding_ivfflat
  ON support_coverage_gaps
  USING ivfflat (embedding vector_cosine_ops)
  WITH (lists = 100)
  WHERE embedding IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_support_coverage_analyses_embedding_ivfflat
  ON support_coverage_conversation_analyses
  USING ivfflat (embedding vector_cosine_ops)
  WITH (lists = 100)
  WHERE embedding IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_support_coverage_analyses_unmaterialized
  ON support_coverage_conversation_analyses(workspace_id, run_id, created_at)
  WHERE has_gap = true AND gap_id IS NULL;
```

- [ ] **Step 3: Update Go models**

Add fields to `SupportCoverageGap`:

```go
Embedding                string     `json:"-" gorm:"type:vector(1536)"`
EmbeddingProvider        string     `json:"embedding_provider" gorm:"not null;default:''"`
EmbeddingModel           string     `json:"embedding_model" gorm:"not null;default:''"`
EmbeddingVersion         string     `json:"embedding_version" gorm:"not null;default:''"`
EmbeddingDimensions      int        `json:"embedding_dimensions" gorm:"not null;default:0"`
EmbeddingTextHash        string     `json:"embedding_text_hash" gorm:"not null;default:''"`
EmbeddingUpdatedAt       *time.Time `json:"embedding_updated_at"`
NearestContentScore      float64    `json:"nearest_content_score" gorm:"not null;default:0"`
NearestContentDocumentID *string    `json:"nearest_content_document_id" gorm:"type:uuid"`
NearestContentTitle      string     `json:"nearest_content_title" gorm:"not null;default:''"`
NearestContentCheckedAt  *time.Time `json:"nearest_content_checked_at"`
ImpactScore              float64    `json:"impact_score" gorm:"not null;default:0"`
```

Add to `SupportGapEvidence`:

```go
SourceKey string `json:"source_key" gorm:"not null;default:''"`
```

Add to `SupportCoverageConversationAnalysis`:

```go
CanonicalTitle          string          `json:"canonical_title" gorm:"type:text;not null;default:''"`
Embedding               string          `json:"-" gorm:"type:vector(1536)"`
EmbeddingProvider       string          `json:"embedding_provider" gorm:"not null;default:''"`
EmbeddingModel          string          `json:"embedding_model" gorm:"not null;default:''"`
EmbeddingVersion        string          `json:"embedding_version" gorm:"not null;default:''"`
EmbeddingDimensions     int             `json:"embedding_dimensions" gorm:"not null;default:0"`
EmbeddingTextHash       string          `json:"embedding_text_hash" gorm:"not null;default:''"`
EmbeddingUpdatedAt      *time.Time      `json:"embedding_updated_at"`
MaterializationMetadata json.RawMessage `json:"materialization_metadata" gorm:"type:jsonb;not null;default:'{}'"`
```

Add chunk metadata fields to `DocsChunk` and `SupportContentChunk`.

- [ ] **Step 4: Validate migrations**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go run ./cmd/migrate validate
```

Expected: exits 0 with all migrations valid.

- [ ] **Step 5: Run focused model/repository compile checks**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/model ./internal/repository -run 'TestSupportCoverage|TestDocs|TestSupportContent' -count=1
```

Expected: compile succeeds. Some tests may fail until repository code is updated; failures should be limited to missing new behavior, not migration syntax.

- [ ] **Step 6: Commit**

```bash
git add server/internal/dbmigrate/sql/202606170003_coverage_gap_semantic_embeddings.sql server/internal/model server/internal/repository/*_test.go server/internal/service/*_test.go
git commit -m "Add coverage gap semantic embedding schema"
```

---

## Task 2: Canonical Embedding Helpers

**Files:**
- Create: `server/internal/service/support_coverage_semantic.go`
- Create: `server/internal/service/support_coverage_semantic_test.go`

- [ ] **Step 1: Write failing helper tests**

Add tests:

```go
func TestCoverageGapEmbeddingTextUsesStableSemanticFields(t *testing.T) {
	item := model.SupportCoverageGapListItem{
		SupportCoverageGap: model.SupportCoverageGap{
			Title: "Password reset email never arrives",
			GapKind: "content",
			GapCategory: "knowledge",
			FailureMode: "weak_retrieval",
			SourceSignal: "daily_conversation_analysis",
		},
		CanonicalTitle: "Password reset email delivery failure",
		CustomerNeedText: "Customers need to recover access when reset emails do not arrive.",
		EvidenceText: "thanks regards unrelated transcript boilerplate",
	}

	text := coverageGapEmbeddingText(item)

	require.Contains(t, text, "Customers need to recover access")
	require.Contains(t, text, "Password reset email delivery failure")
	require.NotContains(t, text, "thanks regards")
	require.NotContains(t, text, "weak_retrieval")
	require.NotContains(t, text, "daily_conversation_analysis")
}

func TestCoverageEmbeddingTextHashChangesOnlyWhenSemanticTextChanges(t *testing.T) {
	base := coverageEmbeddingTextHash("customers need refunds")
	require.Equal(t, base, coverageEmbeddingTextHash(" customers   need refunds "))
	require.NotEqual(t, base, coverageEmbeddingTextHash("customers need invoice exports"))
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/service -run 'TestCoverageGapEmbeddingText|TestCoverageEmbeddingTextHash' -count=1
```

Expected: FAIL because helpers do not exist.

- [ ] **Step 3: Implement helpers**

Implement:

```go
const (
	coverageEmbeddingProviderName = "openai"
	coverageGapEmbeddingVersion   = "coverage-gap-canonical-v1"
	coverageFindingEmbeddingVersion = "coverage-finding-canonical-v1"
	coverageEmbeddingDimensions   = 1536
	coverageDefaultEmbeddingModel = "text-embedding-3-small"
)

func coverageGapEmbeddingText(item model.SupportCoverageGapListItem) string
func coverageFindingEmbeddingText(analysis model.SupportCoverageConversationAnalysis) string
func coverageEmbeddingTextHash(text string) string
func coverageNormalizeEmbeddingText(text string) string
func coverageCosineSimilarity(a, b []float32) float64
func coverageDeterministicClusterKey(workspaceID string, text string) string
func coverageEvidenceSourceKey(analysisID string) string
func coverageEmbeddingModel(configured string) string
```

Rules:

- Normalize whitespace and lowercase only for hashes, not display fields.
- Truncate individual text parts defensively, but never split multibyte runes.
- Keep category/kind/source/failure mode out of embedding text.
- Deterministic cluster keys must be stable across Temporal retries.

- [ ] **Step 4: Verify helper tests pass**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/service -run 'TestCoverageGapEmbeddingText|TestCoverageEmbeddingTextHash' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/service/support_coverage_semantic.go server/internal/service/support_coverage_semantic_test.go
git commit -m "Add coverage gap semantic identity helpers"
```

---

## Task 3: Repository Idempotency And Vector Search Primitives

**Files:**
- Modify: `server/internal/repository/support_coverage.go`
- Modify: `server/internal/repository/support_coverage_analysis.go`
- Modify: `server/internal/repository/docs_chunk.go`
- Modify: `server/internal/repository/support_content_chunk.go`
- Test: `server/internal/repository/support_coverage_test.go`
- Test: `server/internal/repository/support_coverage_analysis_test.go`

- [ ] **Step 1: Write failing evidence idempotency test**

Add a test proving duplicate `source_key` does not double-insert or double-count:

```go
func TestSupportCoverageRepository_CreateEvidenceIfAbsentBySourceKey(t *testing.T) {
	ctx := context.Background()
	repo := NewSupportCoverageRepository(db)
	gap := seedOpenGapWithEvidenceCount(t, db, "gap-1", 0)

	inserted, err := repo.CreateEvidenceIfAbsent(ctx, &model.SupportGapEvidence{
		GapID: "gap-1", WorkspaceID: "ws-1", SourceKey: "coverage_analysis:analysis-1",
		EvidenceType: model.SupportCoverageGapSourceDailyConversationAnalysis,
		CreatedAt: time.Now(),
	})
	require.NoError(t, err)
	require.True(t, inserted)

	inserted, err = repo.CreateEvidenceIfAbsent(ctx, &model.SupportGapEvidence{
		GapID: "gap-1", WorkspaceID: "ws-1", SourceKey: "coverage_analysis:analysis-1",
		EvidenceType: model.SupportCoverageGapSourceDailyConversationAnalysis,
		CreatedAt: time.Now(),
	})
	require.NoError(t, err)
	require.False(t, inserted)

	var count int64
	require.NoError(t, db.Model(&model.SupportGapEvidence{}).Where("gap_id = ?", gap.ID).Count(&count).Error)
	require.Equal(t, int64(1), count)
}
```

- [ ] **Step 2: Write failing unmaterialized analysis listing test**

Add:

```go
func TestSupportCoverageAnalysisRepository_ListUnmaterializedGapAnalysesForRun(t *testing.T) {
	// Seed: one has_gap with nil gap_id, one has_gap with gap_id, one no-gap.
	// Expect only the nil gap_id row.
}
```

- [ ] **Step 3: Run tests and verify failure**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/repository -run 'TestSupportCoverageRepository_CreateEvidenceIfAbsentBySourceKey|TestSupportCoverageAnalysisRepository_ListUnmaterializedGapAnalysesForRun' -count=1
```

Expected: FAIL because repository methods do not exist.

- [ ] **Step 4: Add repository methods**

Add to `SupportCoverageRepository`:

```go
func (r *SupportCoverageRepository) WithTx(tx *gorm.DB) *SupportCoverageRepository
func (r *SupportCoverageRepository) UpsertOpenGapByDedupeKeyNoBump(ctx context.Context, gap *model.SupportCoverageGap) (*model.SupportCoverageGap, bool, error)
func (r *SupportCoverageRepository) CreateEvidenceIfAbsent(ctx context.Context, evidence *model.SupportGapEvidence) (bool, error)
func (r *SupportCoverageRepository) IncrementGapEvidenceAfterInsert(ctx context.Context, workspaceID, gapID string, lastSeenAt time.Time) error
func (r *SupportCoverageRepository) FindNearestOpenGapsByEmbedding(ctx context.Context, workspaceID, embedding, provider, modelName, version string, dimensions int, limit int) ([]model.SupportCoverageGapListItem, error)
func (r *SupportCoverageRepository) FindNearestRecentClosedGapsByEmbedding(ctx context.Context, workspaceID, embedding, provider, modelName, version string, dimensions int, since time.Time, limit int) ([]model.SupportCoverageGapListItem, error)
func (r *SupportCoverageRepository) UpdateGapEmbedding(ctx context.Context, gapID string, embedding string, provider string, modelName string, version string, dimensions int, textHash string, updatedAt time.Time) error
func (r *SupportCoverageRepository) UpdateGapKnowledgeMatch(ctx context.Context, workspaceID, gapID string, match CoverageKnowledgeMatchUpdate) error
```

Implementation notes:

- `CreateEvidenceIfAbsent` should use `ON CONFLICT DO NOTHING` on Postgres.
- For SQLite tests, manually check existing `(workspace_id, source_key)` before create.
- `UpsertOpenGapByDedupeKeyNoBump` creates gaps with `evidence_count = 0`.
- Existing bumping methods may remain for old paths, but new batch materialization must not use them.
- Vector searches must return no rows on non-Postgres instead of lexical fallback.
- Vector searches must filter:

```sql
workspace_id = ?
status = 'open'
embedding IS NOT NULL
embedding_provider = ?
embedding_model = ?
embedding_version = ?
embedding_dimensions = ?
```

Add to `SupportCoverageAnalysisRepository`:

```go
func (r *SupportCoverageAnalysisRepository) WithTx(tx *gorm.DB) *SupportCoverageAnalysisRepository
func (r *SupportCoverageAnalysisRepository) ListUnmaterializedGapAnalysesForRun(ctx context.Context, workspaceID, runID string, limit int) ([]model.SupportCoverageConversationAnalysis, error)
func (r *SupportCoverageAnalysisRepository) UpdateAnalysisEmbedding(ctx context.Context, analysisID string, embedding string, provider string, modelName string, version string, dimensions int, textHash string, updatedAt time.Time) error
func (r *SupportCoverageAnalysisRepository) SetConversationAnalysisGapTx(ctx context.Context, analysisID, gapID, primaryRecommendationType string) error
```

- [ ] **Step 5: Add embedding metadata filters to content repositories**

Modify `DocsChunkRepository.HybridSearch` and `SupportContentChunkRepository.HybridSearch` vector queries to accept optional metadata parameters or to use repository defaults:

```go
WHERE c.embedding_provider = ?
  AND c.embedding_model = ?
  AND c.embedding_version = ?
  AND c.embedding_dimensions = ?
```

Do not break lexical-only search if query embedding is empty.

- [ ] **Step 6: Verify repository tests pass**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/repository -run 'TestSupportCoverage|TestDocsChunk|TestSupportContentChunk' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add server/internal/repository
git commit -m "Add idempotent coverage gap repository primitives"
```

---

## Task 4: Persist Final Analysis Materialization Payload

**Files:**
- Modify: `server/internal/service/support_coverage_daily_analyzer.go`
- Test: `server/internal/service/support_coverage_daily_analyzer_test.go`

- [ ] **Step 1: Write failing analysis persistence test**

Add a test that proves a gap analysis row stores final canonical fields and materialization metadata without creating a gap immediately:

```go
func TestSupportCoverageDailyAnalyzer_RunConversationStoresMaterializationPayload(t *testing.T) {
	// Seed conversation/messages/retrieval traces.
	// Use fake LLM returning HasGap=true, CanonicalTitle, CustomerNeed, RecommendedFixes.
	// Run runConversationCoverageAnalysis.
	// Assert analysis row exists with gap_id NULL, canonical_title set,
	// customer_need set, materialization_metadata contains segment_id and has_human_reply.
	// Assert no support_coverage_gaps row was created yet.
}
```

- [ ] **Step 2: Run test and verify failure**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/service -run TestSupportCoverageDailyAnalyzer_RunConversationStoresMaterializationPayload -count=1
```

Expected: FAIL because current code calls `UpsertFinding`.

- [ ] **Step 3: Refactor conversation analysis flow**

Change `runConversationCoverageAnalysis`:

- Keep local classification and failed-analysis behavior.
- For support gap results:
  - run knowledge matching/refinement before recording the analysis row
  - record `CanonicalTitle`, final `CustomerNeed`, final `DecisionReason`, final `PrimaryRecommendationType`
  - store a JSON `MaterializationMetadata` payload:

```json
{
  "matched_knowledge_candidates": [],
  "recommendation_decision_reason": "...",
  "segment_id": "...",
  "segment_start_message_id": "...",
  "segment_end_message_id": "...",
  "segment_resolved": false,
  "has_human_reply": true
}
```

- Do not create or update coverage gaps in this method.
- Return `true` when a materializable gap finding was recorded.

- [ ] **Step 4: Keep legacy `UpsertFinding` compiling**

Do not delete `UpsertFinding` yet. It is covered by tests and may be used by manual/event paths. Mark it internally as legacy if needed, and later route it through the materializer if usage remains.

- [ ] **Step 5: Verify daily analyzer tests**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/service -run 'TestSupportCoverageDailyAnalyzer_(RunConversationStoresMaterializationPayload|AnalyzeConversation|RefineFixBundle|UpsertFinding)' -count=1
```

Expected: PASS after updating expectations around gap creation for the new run-conversation test only.

- [ ] **Step 6: Commit**

```bash
git add server/internal/service/support_coverage_daily_analyzer.go server/internal/service/support_coverage_daily_analyzer_test.go
git commit -m "Stage coverage findings before materialization"
```

---

## Task 5: Batch Materializer With Same-Run Clustering

**Files:**
- Create: `server/internal/service/support_coverage_materializer.go`
- Create: `server/internal/service/support_coverage_materializer_test.go`
- Modify: `server/internal/service/support_coverage_daily_analyzer.go`
- Modify: `server/internal/repository/support_coverage.go`
- Modify: `server/internal/repository/support_coverage_analysis.go`

- [ ] **Step 1: Write failing same-run clustering test**

Test two unmaterialized analyses in the same run with similar embeddings:

```go
func TestCoverageMaterializerClustersSameRunFindingsIntoOneGap(t *testing.T) {
	// Seed two analyses in run-1:
	// - same customer need/canonical title
	// - gap_id NULL
	// Fake embedder returns near-identical vectors.
	// Materialize run.
	// Assert one support_coverage_gaps row.
	// Assert two support_gap_evidence rows.
	// Assert gap.evidence_count == 2.
	// Assert both analyses have same gap_id.
}
```

- [ ] **Step 2: Write failing retry idempotency test**

```go
func TestCoverageMaterializerRetryDoesNotDuplicateEvidenceOrCount(t *testing.T) {
	// Seed one unmaterialized analysis.
	// Run materializer twice.
	// Assert one gap, one evidence row, evidence_count == 1.
}
```

- [ ] **Step 3: Write failing existing-gap attach test**

```go
func TestCoverageMaterializerAttachesHighConfidenceFindingToExistingGap(t *testing.T) {
	// Seed open gap with canonical embedding metadata and vector.
	// Seed analysis whose embedding is very close.
	// Materialize.
	// Assert no new gap was created.
	// Assert analysis.gap_id == existing gap.
	// Assert evidence_count increments exactly once.
}
```

- [ ] **Step 4: Run tests and verify failure**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/service -run 'TestCoverageMaterializer' -count=1
```

Expected: FAIL because materializer does not exist.

- [ ] **Step 5: Implement materializer result and input types**

In `support_coverage_materializer.go`:

```go
type CoverageMaterializationResult struct {
	FindingsScanned       int
	EmbeddingsCreated     int
	ExistingGapAttached   int
	NewGapsCreated        int
	SameRunFindingsMerged int
	EvidenceInserted      int
	AlreadyMaterialized   int
	MissingEmbeddings     int
}
```

Add method on `SupportCoverageDailyAnalyzer`:

```go
func (s *SupportCoverageDailyAnalyzer) materializeRunFindings(ctx context.Context, workspaceID, runID string) (*CoverageMaterializationResult, error)
```

This can be a method on the analyzer rather than a new public service so it can reuse `recommendationRowsForFinding`, `createDocsSuggestionForFix`, configured repositories, and LLM provider.

- [ ] **Step 6: Implement embedding generation**

For unmaterialized analyses:

- Build `coverageFindingEmbeddingText`.
- Compute hash.
- If analysis embedding is missing/stale/model-mismatched, batch embed all stale findings in one `CreateEmbeddings` call.
- Persist real vectors and metadata to analysis rows.
- If the embedding provider is nil or embedding call fails, return an error and leave `gap_id` NULL.

No lexical fallback in this path.

- [ ] **Step 7: Implement candidate matching**

For each finding:

- Query nearest open gaps by pgvector when Postgres is available.
- Require matching provider/model/version/dim and non-NULL gap embedding.
- Apply business compatibility gates:
  - same workspace
  - open status
  - compatible gap kind/category, with kind treated as soft unless it is an action/policy mismatch
  - no conflicting related document
- Suggested thresholds:
  - `>= 0.90`: attach new evidence to existing gap if gates pass
  - `0.78 - 0.90`: create merge suggestion or keep as new same-run candidate
  - `< 0.78`: do not attach

Keep thresholds as constants in `support_coverage_semantic.go` so they are testable.

- [ ] **Step 8: Implement same-run clustering**

For findings not attached to existing gaps:

- Use in-memory cosine between their newly generated embeddings.
- Use union-find to cluster same-run findings above threshold.
- Choose primary by:
  - higher confidence
  - more actionable recommendation
  - earlier created_at
  - stable analysis ID tie-breaker
- Create one new gap per cluster.

Suggested same-run threshold: `>= 0.88`, with exact normalized customer need as a deterministic merge.

- [ ] **Step 9: Implement transactional materialization**

For each existing attach or new cluster:

- Open one DB transaction.
- Upsert topic by deterministic cluster key.
- Upsert gap by deterministic dedupe key with `evidence_count = 0`.
- Set gap embedding if new or stale.
- For each analysis in the cluster:
  - create evidence with `source_key = coverage_analysis:<analysis_id>`
  - only if evidence was inserted, increment `evidence_count` and `last_seen_at`
  - create/replace recommendations for that analysis/gap
  - stamp analysis `gap_id`
- Commit transaction.

If transaction fails, retry must safely re-run because source keys and dedupe keys are deterministic.

- [ ] **Step 10: Handle recommendations without double side effects**

`recommendationRowsForFinding` currently links articles and may create suggestions. Make it safe under retries:

- Replace recommendations by `(workspace_id, analysis_id, gap_id)` only after evidence insert or confirmed existing source key.
- Do not supersede active suggestions repeatedly for duplicate retry when no new evidence inserted.
- Use existing `ReplaceRecommendations` but ensure it is scoped to the current analysis/gap.

- [ ] **Step 11: Verify materializer tests pass**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/service -run 'TestCoverageMaterializer' -count=1
```

Expected: PASS.

- [ ] **Step 12: Commit**

```bash
git add server/internal/service/support_coverage_materializer.go server/internal/service/support_coverage_materializer_test.go server/internal/service/support_coverage_daily_analyzer.go server/internal/repository/support_coverage.go server/internal/repository/support_coverage_analysis.go
git commit -m "Materialize coverage findings in semantic batches"
```

---

## Task 6: Daily Analysis Integration And Wiring

**Files:**
- Modify: `server/internal/service/support_coverage_daily_analyzer.go`
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/temporal-worker/main.go`
- Test: `server/internal/service/support_coverage_daily_analyzer_test.go`
- Test: `server/internal/temporalapp/coverage_analysis_workflow_test.go`

- [ ] **Step 1: Write failing workspace-run integration test**

Add a test where `RunWorkspaceDailyAnalysis` processes multiple conversations and then materializes them:

```go
func TestSupportCoverageDailyAnalyzer_RunWorkspaceDailyAnalysisMaterializesBatch(t *testing.T) {
	// Seed two conversations in the analysis window.
	// Fake LLM returns similar gaps.
	// Fake embedder returns near-identical vectors.
	// Run workspace analysis.
	// Assert CompleteRun gapCount reflects materialized gap/evidence result.
	// Assert one gap with two evidence rows.
}
```

- [ ] **Step 2: Run test and verify failure**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/service -run TestSupportCoverageDailyAnalyzer_RunWorkspaceDailyAnalysisMaterializesBatch -count=1
```

Expected: FAIL until integration is complete.

- [ ] **Step 3: Add embedding provider wiring to analyzer**

Modify `SupportCoverageDailyAnalyzer`:

```go
embeddingProvider llm.EmbeddingProvider
embeddingModel    string

func (s *SupportCoverageDailyAnalyzer) SetEmbeddingProvider(provider llm.EmbeddingProvider, model string) *SupportCoverageDailyAnalyzer
```

Use `coverageEmbeddingModel(model)` for default fallback.

- [ ] **Step 4: Call materializer after conversation workers**

In `RunWorkspaceDailyAnalysis`, after `group.Wait()`:

```go
materialized, err := s.materializeRunFindings(ctx, workspaceID, run.ID)
if err != nil {
	_ = s.analysisRepo.FailRun(ctx, run.ID, err)
	return err
}
return s.analysisRepo.CompleteRun(ctx, run.ID, len(conversations), materialized.EvidenceInserted)
```

Decide whether `gapCount` means findings/evidence inserted or new gaps created. Keep API semantics clear in run metadata. Prefer:

- `gapCount` existing column = evidence/materialized findings for backward compatibility.
- run metadata includes `new_gaps_created`, `existing_gap_attached`, `same_run_findings_merged`.

- [ ] **Step 5: Wire API and worker**

In both `server/cmd/api/main.go` and `server/cmd/temporal-worker/main.go`:

```go
supportCoverageDailyAnalyzer := service.NewSupportCoverageDailyAnalyzer(llmProvider, cfg.CRMLLMProvider, cfg.CRMLLMModel).
	SetCoverageRepositories(...).
	SetConversationRepositories(...).
	SetKnowledgeMatcher(...).
	SetEmbeddingProvider(supportEmbeddingProvider, cfg.OpenAIEmbeddingModel).
	SetTemporalClient(...)
```

- [ ] **Step 6: Verify integration tests pass**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/service ./internal/temporalapp -run 'TestSupportCoverageDailyAnalyzer_RunWorkspaceDailyAnalysisMaterializesBatch|TestCoverageDailyAnalysisWorkflow' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add server/internal/service/support_coverage_daily_analyzer.go server/cmd/api/main.go server/cmd/temporal-worker/main.go server/internal/service/support_coverage_daily_analyzer_test.go server/internal/temporalapp/coverage_analysis_workflow_test.go
git commit -m "Run coverage materialization after daily analysis batches"
```

---

## Task 7: Rebuild Gap Clusters As Repair/Backfill

**Files:**
- Modify: `server/internal/service/support_coverage_cluster_rebuild.go`
- Modify: `server/internal/service/support_coverage_cluster_rebuild_test.go`
- Modify: `server/internal/model/support_coverage.go`
- Modify: `server/internal/repository/support_coverage.go`

- [ ] **Step 1: Write failing rebuild backfill test**

```go
func TestSupportCoverageClusterRebuildBackfillsMissingGapEmbeddings(t *testing.T) {
	// Seed open gap without embedding.
	// Fake embedder returns vector.
	// Run rebuild.
	// Assert gap embedding metadata is persisted.
	// Assert result metadata reports embeddings_created = 1.
}
```

- [ ] **Step 2: Write failing no-silent-fallback test**

```go
func TestSupportCoverageClusterRebuildReportsEmbeddingFailure(t *testing.T) {
	// Seed open gaps without embeddings.
	// Fake embedder returns error.
	// Run rebuild.
	// Assert run status failed or result embedding_status = "failed".
	// Assert no lexical auto-merge happened.
}
```

- [ ] **Step 3: Run tests and verify failure**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/service -run 'TestSupportCoverageClusterRebuild(BackfillsMissingGapEmbeddings|ReportsEmbeddingFailure)' -count=1
```

Expected: FAIL until rebuild is converted.

- [ ] **Step 4: Convert rebuild flow**

Rebuild should:

- List open gaps, including embedding metadata.
- Backfill missing/stale gap embeddings in batches.
- Exclude gaps that still have `NULL` embeddings from NN candidate search.
- For each embedded gap, find top-K embedded neighbors through pgvector.
- Build candidate pairs and union-find clusters.
- Use direct primary-to-duplicate score for auto-merge eligibility.
- Auto-merge only when:
  - direct score is very high, and
  - same related article or exact normalized customer need, and
  - no conflicting recommendations/target docs.
- Otherwise create merge suggestions.

Remove lexical fallback from production rebuild. If a non-Postgres test DB is used, keep deterministic test-only/in-memory paths isolated behind dialector checks and make the result report `embedding_status`.

- [ ] **Step 5: Improve rebuild run metadata**

Store in `SupportCoverageClusterRebuildRun.Metadata`:

```json
{
  "embedding_status": "complete|failed|degraded",
  "embeddings_created": 12,
  "missing_embeddings": 0,
  "vector_candidates_scanned": 240,
  "auto_merge_policy": "strict-v1"
}
```

- [ ] **Step 6: Verify rebuild tests pass**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/service -run 'TestSupportCoverageClusterRebuild' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add server/internal/service/support_coverage_cluster_rebuild.go server/internal/service/support_coverage_cluster_rebuild_test.go server/internal/model/support_coverage.go server/internal/repository/support_coverage.go
git commit -m "Use coverage gap embeddings for cluster rebuild"
```

---

## Task 8: Gap-To-KB Matching And Failure Reclassification

**Files:**
- Modify: `server/internal/service/support_coverage_knowledge_matcher.go`
- Modify: `server/internal/service/support_coverage_daily_analyzer.go`
- Modify: `server/internal/service/support_coverage_materializer.go`
- Modify: `server/internal/repository/support_coverage.go`
- Test: `server/internal/service/support_coverage_knowledge_matcher_test.go`
- Test: `server/internal/service/support_coverage_materializer_test.go`

- [ ] **Step 1: Write failing KB reclassification test**

```go
func TestCoverageMaterializerReclassifiesStrongKBMatchAsRetrievalFailure(t *testing.T) {
	// Seed support/docs content chunk that strongly matches a finding.
	// Materialize finding.
	// Assert gap.failure_mode is weak_retrieval or no_retrieval, not missing_content.
	// Assert related article link / nearest_content fields are populated.
}
```

- [ ] **Step 2: Run test and verify failure**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/service -run 'TestCoverageMaterializerReclassifiesStrongKBMatchAsRetrievalFailure|TestCoverageKnowledgeMatcher' -count=1
```

Expected: FAIL until KB matching is wired.

- [ ] **Step 3: Add gap-to-KB matcher method**

Add:

```go
func (m *CoverageKnowledgeMatcher) MatchGapKnowledge(ctx context.Context, workspaceID string, spaceIDs []string, contentSourceIDs []string, query string, queryEmbedding string, limit int) ([]CoverageKnowledgeCandidate, error)
```

Rules:

- Use existing `queryEmbedding` when present instead of re-embedding.
- Use hybrid search for KB matching.
- If embedding is missing, lexical-only is allowed but must be marked as degraded in caller metadata.
- Filter content/doc chunk vector matches by metadata where vector search is used.

- [ ] **Step 4: Update materialization classification**

During materialization after gap creation/attach:

- Run gap-to-KB match for the canonical text.
- If best score is high, link article/content and update failure mode:
  - existing content strongly covers need and retrieval traces lacked it: `no_retrieval`
  - existing content weakly matches or is stale: `weak_retrieval`
  - conflicting content: `conflicting_guidance`
  - no nearby content: keep/create missing content classification
- Persist nearest content score/title/document.
- Include KB proximity in recommendation metadata.

- [ ] **Step 5: Verify tests pass**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/service -run 'TestCoverageMaterializerReclassifiesStrongKBMatchAsRetrievalFailure|TestCoverageKnowledgeMatcher' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add server/internal/service/support_coverage_knowledge_matcher.go server/internal/service/support_coverage_daily_analyzer.go server/internal/service/support_coverage_materializer.go server/internal/repository/support_coverage.go server/internal/service/*knowledge*test.go server/internal/service/support_coverage_materializer_test.go
git commit -m "Classify coverage gaps against existing knowledge"
```

---

## Task 9: Explainable Impact Score And Ranking

**Files:**
- Modify: `server/internal/model/support_coverage.go`
- Modify: `server/internal/repository/support_coverage.go`
- Modify: `server/internal/service/support_coverage_snapshots.go`
- Modify: `server/internal/service/support_coverage_digest.go`
- Test: `server/internal/repository/support_coverage_test.go`
- Test: `server/internal/service/support_coverage_snapshots_test.go` if present, otherwise add focused tests near existing coverage tests.

- [ ] **Step 1: Write failing impact query test**

```go
func TestSupportCoverageRepositoryListGapsRanksByExplainableImpact(t *testing.T) {
	// Seed two gaps:
	// gap A: many conversations from one customer.
	// gap B: fewer conversations from several customers and no nearby content.
	// Expect impact components populated and sort by impact_score DESC.
}
```

- [ ] **Step 2: Run test and verify failure**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/repository -run TestSupportCoverageRepositoryListGapsRanksByExplainableImpact -count=1
```

Expected: FAIL until impact fields are computed.

- [ ] **Step 3: Add DTO fields**

Add read-only fields to `SupportCoverageGapListItem` and detail:

```go
ImpactScore          float64 `json:"impact_score" gorm:"column:impact_score"`
DistinctCustomers30d int     `json:"distinct_customers_30d" gorm:"column:distinct_customers_30d"`
DistinctCustomersAll int     `json:"distinct_customers_all" gorm:"column:distinct_customers_all"`
EvidenceAll          int     `json:"evidence_all" gorm:"column:evidence_all"`
NearestContentScore  float64 `json:"nearest_content_score"`
ImpactExplanation    string  `json:"impact_explanation" gorm:"-"`
```

- [ ] **Step 4: Compute explainable impact in query**

Update `ListGaps` query to compute:

- `evidence_30d`
- total evidence
- distinct customers 30d/all using:

```sql
COALESCE(sc.crm_contact_id, LOWER(sc.customer_email), sc.anonymous_id, e.conversation_id::text, e.id::text)
```

- recency factor
- KB gap factor:
  - low/zero nearest content score increases missing-content urgency
  - high nearest content score increases retrieval-failure urgency
- confidence factor
- actionable fix factor based on active recommendations/suggestions

Keep formula simple and documented in code. Example:

```text
impact =
  log1p(evidence_30d) * 4
  + log1p(distinct_customers_30d) * 5
  + log1p(total_evidence) * 2
  + recency_bonus
  + actionable_bonus
  + kb_gap_bonus
```

- [ ] **Step 5: Add explanation builder**

Build a short deterministic explanation:

```text
"12 conversations, 5 customers this month, no nearby content"
"8 conversations, 3 customers, existing article nearby but not retrieved"
```

- [ ] **Step 6: Update digest/snapshot ordering**

Use impact score for top recurring gaps instead of only `evidence_30d DESC`.

- [ ] **Step 7: Verify repository/service tests**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/repository ./internal/service -run 'TestSupportCoverage.*Impact|TestSupportCoverage.*Snapshot|TestSupportCoverage.*Digest|TestSupportCoverageRepositoryListGaps' -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add server/internal/model/support_coverage.go server/internal/repository/support_coverage.go server/internal/service/support_coverage_snapshots.go server/internal/service/support_coverage_digest.go server/internal/repository/support_coverage_test.go server/internal/service/*coverage*test.go
git commit -m "Rank coverage gaps by explainable impact"
```

---

## Task 10: Recurrence After Closure

**Files:**
- Modify: `server/internal/service/support_coverage_materializer.go`
- Modify: `server/internal/repository/support_coverage.go`
- Modify: `server/internal/model/support_coverage.go`
- Test: `server/internal/service/support_coverage_materializer_test.go`

- [ ] **Step 1: Write failing regression test**

```go
func TestCoverageMaterializerFlagsRecurrenceAfterDoneGap(t *testing.T) {
	// Seed a done gap with matching embedding and closed_at yesterday.
	// Materialize a new matching finding.
	// Assert it does not silently create an unrelated duplicate.
	// Assert recurrence metadata/reopened status follows the chosen policy.
}
```

- [ ] **Step 2: Decide and implement recurrence policy**

Use this policy:

- Search open gaps first.
- Search recently done gaps second, within a configurable window, e.g. 90 days.
- If a strong match hits a done gap:
  - insert evidence against the done gap with source key
  - increment post-fix evidence metadata
  - reopen to `open` only after threshold is reached, e.g. 2 distinct customers or 3 conversations after closure
  - before threshold, mark metadata as `recurrence_watch: true`

Do not attach new evidence to rejected gaps unless exact customer need and same target article match; otherwise create a new gap.

- [ ] **Step 3: Add metadata helpers**

Store recurrence fields in gap metadata:

```json
{
  "recurrence_watch": true,
  "post_close_evidence_count": 2,
  "last_post_close_evidence_at": "..."
}
```

- [ ] **Step 4: Run recurrence tests**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/service -run 'TestCoverageMaterializer.*Recurrence|TestCoverageMaterializerFlagsRecurrenceAfterDoneGap' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/service/support_coverage_materializer.go server/internal/repository/support_coverage.go server/internal/model/support_coverage.go server/internal/service/support_coverage_materializer_test.go
git commit -m "Track coverage gap recurrence after closure"
```

---

## Task 11: Over-Attachment Detection Signals

**Files:**
- Modify: `server/internal/service/support_coverage_materializer.go`
- Modify: `server/internal/repository/support_coverage_analysis.go`
- Modify: `server/internal/model/support_coverage.go`
- Test: `server/internal/service/support_coverage_materializer_test.go`

- [ ] **Step 1: Write failing split-signal test**

```go
func TestCoverageMaterializerFlagsPotentialOverAttachment(t *testing.T) {
	// Seed one gap with existing finding embeddings forming cluster A.
	// Add new evidence forming cluster B far away from A but still attached by an exact gate.
	// Assert gap metadata contains split_review_needed=true.
}
```

- [ ] **Step 2: Implement lightweight split signal**

Do not build split UI yet. Add a conservative signal:

- For a gap with at least 6 finding embeddings:
  - sample recent finding embeddings
  - if two tight groups have low inter-group similarity, set metadata:

```json
{
  "split_review_needed": true,
  "split_review_reason": "Evidence appears to contain two separate semantic clusters"
}
```

- This signal should never automatically split or merge anything.

- [ ] **Step 3: Verify tests**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/service -run 'TestCoverageMaterializerFlagsPotentialOverAttachment' -count=1
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add server/internal/service/support_coverage_materializer.go server/internal/repository/support_coverage_analysis.go server/internal/model/support_coverage.go server/internal/service/support_coverage_materializer_test.go
git commit -m "Flag possible over-attached coverage gaps"
```

---

## Task 12: Frontend First-Class Coverage UI Updates

**Files:**
- Modify: `frontend/src/lib/supportCoverageTypes.ts`
- Modify: `frontend/src/components/support/coverage/GapList.tsx`
- Modify: `frontend/src/components/support/coverage/GapDetailPane.tsx`
- Modify: `frontend/src/pages/support/coverage/SupportCoveragePage.tsx`
- Test: `frontend/src/components/support/coverage/__tests__/coverageUi.test.ts`
- Test: `frontend/src/lib/__tests__/supportCoverageClusterRebuild.test.ts`

- [ ] **Step 1: Write failing UI utility tests**

Add tests for impact explanation rendering and rebuild status handling:

```ts
it('renders impact explanation from gap fields', () => {
  const gap = { impact_score: 18.4, impact_explanation: '12 conversations, 5 customers this month, no nearby content' }
  expect(formatCoverageImpact(gap)).toContain('5 customers')
})
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
cd frontend
npm test -- --run src/components/support/coverage/__tests__/coverageUi.test.ts src/lib/__tests__/supportCoverageClusterRebuild.test.ts
```

Expected: FAIL until types/helpers are updated.

- [ ] **Step 3: Update TypeScript types**

Add fields:

```ts
impact_score: number
impact_explanation?: string
distinct_customers_30d?: number
distinct_customers_all?: number
nearest_content_score?: number
nearest_content_title?: string
embedding_provider?: string
embedding_model?: string
embedding_version?: string
embedding_updated_at?: string | null
```

- [ ] **Step 4: Update list row**

In `GapList.tsx`:

- Show impact explanation in muted text.
- Keep tabs borderless per prior user preference.
- Do not reintroduce tab counts.
- Show subtle KB signal:
  - “No nearby content”
  - “Existing article nearby”
  - “Retrieval issue likely”

- [ ] **Step 5: Update detail pane**

In `GapDetailPane.tsx`:

- Show why the gap is ranked high.
- Show merge suggestions separately from split-review signal.
- Show KB proximity and failure mode reclassification.

- [ ] **Step 6: Update rebuild status text**

In `SupportCoveragePage.tsx`:

- Rebuild button copy remains “Rebuild Gap Clusters”.
- Success toast should include:
  - embeddings backfilled
  - suggestions created
  - auto-merged
  - skipped/missing embeddings
- If API returns failed/degraded metadata, do not show “complete” as success.

- [ ] **Step 7: Verify frontend tests/build**

Run:

```bash
cd frontend
npm test -- --run src/components/support/coverage/__tests__/coverageUi.test.ts src/lib/__tests__/supportCoverageClusterRebuild.test.ts
npm run build
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/lib/supportCoverageTypes.ts frontend/src/components/support/coverage/GapList.tsx frontend/src/components/support/coverage/GapDetailPane.tsx frontend/src/pages/support/coverage/SupportCoveragePage.tsx frontend/src/components/support/coverage/__tests__/coverageUi.test.ts frontend/src/lib/__tests__/supportCoverageClusterRebuild.test.ts
git commit -m "Show explainable coverage gap impact"
```

---

## Task 13: Backward Compatibility And Legacy Path Cleanup

**Files:**
- Modify: `server/internal/service/support_coverage_daily_analyzer.go`
- Modify: `server/internal/service/support_coverage_clusterer.go`
- Modify: `server/internal/repository/support_coverage.go`
- Test: `server/internal/service/support_coverage_daily_analyzer_test.go`
- Test: `server/internal/service/support_coverage_clusterer_test.go` if present, otherwise add coverage near existing tests.

- [ ] **Step 1: Search all gap creation paths**

Run:

```bash
rg -n "UpsertFinding\\(|UpsertGapByDedupeKey\\(|UpsertOpenGapByTopic\\(|IncrementOpenGapEvidence\\(" server/internal server/cmd
```

Expected: identify all paths that can create/increment coverage gaps.

- [ ] **Step 2: Decide path ownership**

Rules:

- Daily analyzer uses batch materializer only.
- Manual/user-created gaps may use no-embedding path, then enqueue/rebuild embedding.
- Event detection path should either:
  - route through a single-finding materializer wrapper, or
  - clearly remain legacy and not claim semantic dedupe.

- [ ] **Step 3: Add tests for remaining legacy paths**

Ensure old paths do not double-count evidence under retry. If a legacy path still increments directly, either refactor it or add explicit source-key support.

- [ ] **Step 4: Remove or quarantine unsafe direct bump methods**

Do not delete public repository methods if many callers still use them. Instead:

- Add comments marking direct bump methods as legacy.
- Ensure new code uses evidence-insert-driven methods.
- Where easy, refactor callers to `CreateEvidenceIfAbsent + IncrementGapEvidenceAfterInsert`.

- [ ] **Step 5: Verify coverage service tests**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/service ./internal/repository -run 'TestSupportCoverage' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add server/internal/service server/internal/repository
git commit -m "Route coverage gap writes through idempotent evidence paths"
```

---

## Task 14: End-To-End Verification

**Files:**
- No planned source edits unless failures reveal real issues.

- [ ] **Step 1: Run backend focused tests**

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/service ./internal/repository ./internal/handler ./internal/temporalapp -run 'TestSupportCoverage|TestCoverageDailyAnalysisWorkflow' -count=1
```

Expected: PASS.

- [ ] **Step 2: Run backend full build**

```bash
cd server
GOCACHE=/tmp/go-build-cache go build ./...
```

Expected: PASS.

- [ ] **Step 3: Run migration validation**

```bash
cd server
GOCACHE=/tmp/go-build-cache go run ./cmd/migrate validate
```

Expected: PASS.

- [ ] **Step 4: Run frontend tests/build**

```bash
cd frontend
npm test -- --run src/components/support/coverage/__tests__/coverageUi.test.ts src/lib/__tests__/supportCoverageClusterRebuild.test.ts
npm run build
```

Expected: PASS.

- [ ] **Step 5: Manual local smoke test**

Start backend and frontend with reachable API host if needed:

```bash
cd server
set -a && . ./.env && set +a && GOCACHE=/tmp/go-build-cache go run ./cmd/api
```

```bash
cd frontend
VITE_API_URL=http://localhost:8080/api npm run dev -- --host 127.0.0.1 --port 5173
```

Check:

- Coverage gaps list loads.
- Impact explanations appear.
- Rebuild Gap Clusters does not show false success on API error.
- Rebuild reports embedding backfill/suggestions accurately.
- Creating/processing duplicate same-run findings yields one gap with multiple evidence rows.

- [ ] **Step 6: Inspect final diff**

```bash
git status --short
git diff --stat
git diff --check
```

Expected:

- no whitespace errors
- no unrelated files staged
- untracked user docs remain untouched unless explicitly requested

- [ ] **Step 7: Final commit if needed**

If verification required small fixes, inspect `git status --short`, stage only the exact files changed by those fixes, and commit them. Do not use `git add .` in this repository because unrelated user files may be present.

```bash
git status --short
git commit -m "Stabilize coverage gap semantic materialization"
```

---

## Rollout Notes

- Deploy schema before relying on materializer fields.
- After deploy, daily ingestion creates embeddings synchronously inside the background job.
- Historical gaps receive embeddings through Rebuild Gap Clusters repair/backfill.
- Existing docs/support content chunks get metadata defaults in migration; future embeddings must write metadata explicitly.
- If `OPENAI_API_KEY` or embedding provider is unavailable, daily materialization should fail and retry rather than silently creating low-quality lexical duplicates.
- Rebuild should expose failed/degraded embedding status to the UI.

## Acceptance Criteria

- Same-run duplicate findings create one gap with multiple evidence rows.
- Retrying a daily analysis/materialization run does not duplicate gaps, evidence, recommendations, or evidence counts.
- New gaps and finding rows store real embeddings with provider/model/version/dim/hash metadata.
- Vector NN searches exclude `NULL` or mismatched embeddings.
- Historical cluster rebuild backfills embeddings and creates suggestions without aggressive irreversible merges.
- Gap list ranks by explainable impact, including distinct customers and KB proximity.
- Strong KB proximity reclassifies gaps as retrieval/content-routing issues instead of always missing content.
- Recently closed gaps can collect recurrence evidence and reopen/flag only after threshold.
- Potential over-attachment can be detected from stored finding embeddings.
- Frontend shows accurate rebuild status and impact explanations without tab counts or bordered tabs.
