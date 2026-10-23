//go:build integration

package service

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// conversationScriptedLLM answers the analyzer per conversation: conversations
// listed in failing get an empty reply from every route.
type conversationScriptedLLM struct {
	mu      sync.Mutex
	failing map[string]bool
}

func (f *conversationScriptedLLM) ChatCompletion(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	var input struct {
		ConversationID string `json:"conversation_id"`
	}
	if len(req.Messages) > 0 {
		_ = json.Unmarshal([]byte(req.Messages[0].Content), &input)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failing[input.ConversationID] {
		return &llm.ChatResponse{Content: ""}, nil
	}
	return &llm.ChatResponse{Content: coverageTestGapJSON}, nil
}

type fixedCoverageEmbeddings struct{}

func (fixedCoverageEmbeddings) CreateEmbeddings(_ context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	vectors := make([][]float32, 0, len(req.Inputs))
	for range req.Inputs {
		vector := make([]float32, 1536)
		vector[0] = 1
		vectors = append(vectors, vector)
	}
	return &llm.EmbeddingResponse{Vectors: vectors}, nil
}

func openCoverageRunPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("COVERAGE_RUN_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set COVERAGE_RUN_TEST_POSTGRES_DSN to a disposable, fully migrated PostgreSQL database")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Error(err)
		}
	})
	return db
}

func seedCoverageRunWorkspace(t *testing.T, db *gorm.DB, workspaceID string) {
	t.Helper()
	key := strings.ToUpper(strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'f' {
			return r
		}
		return -1
	}, strings.ReplaceAll(uuid.NewString(), "-", "")))
	if len(key) > 5 {
		key = key[:5]
	}
	for len(key) < 2 {
		key += "X"
	}
	if err := db.Exec(`INSERT INTO workspaces (id, name, slug, workspace_key, owner_id) VALUES (?, ?, ?, ?, ?)`,
		workspaceID, "Coverage test", "coverage-test-"+workspaceID[:8], key, uuid.NewString()).Error; err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
}

func seedCoverageRunConversation(t *testing.T, db *gorm.DB, workspaceID, conversationID string, displayID int, at time.Time) {
	t.Helper()
	if err := db.Exec(`INSERT INTO support_conversations (id, workspace_id, display_id, subject, status, created_at, updated_at, resolved_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		conversationID, workspaceID, displayID, "Instagram keeps disconnecting", model.SupportConversationStatusResolved, at.Add(-time.Hour), at, at).Error; err != nil {
		t.Fatalf("seed conversation: %v", err)
	}
	for i, message := range []struct{ sender, content string }{
		{"customer", "My Instagram account keeps disconnecting. How do I reconnect it after the token expires?"},
		{"agent", "Go to Settings, Social accounts, choose Instagram and click Reconnect. Approve the permissions again."},
	} {
		if err := db.Exec(`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, message_type, content, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			uuid.NewString(), workspaceID, conversationID, message.sender, "reply", message.content, at.Add(time.Duration(i-2)*time.Minute)).Error; err != nil {
			t.Fatalf("seed message: %v", err)
		}
	}
}

// A run in which one conversation's model reply is unusable must still finish,
// save both analyses, turn the good one into a gap, and leave the bad one to
// retry.
func TestCoverageWorkspaceRunPostgresFinishesDespiteFailedConversation(t *testing.T) {
	db := openCoverageRunPostgres(t)
	ctx := context.Background()
	workspaceID := uuid.NewString()
	good, bad := uuid.NewString(), uuid.NewString()
	now := time.Now().UTC()
	seedCoverageRunWorkspace(t, db, workspaceID)
	seedCoverageRunConversation(t, db, workspaceID, good, 1, now.Add(-time.Hour))
	seedCoverageRunConversation(t, db, workspaceID, bad, 2, now.Add(-50*time.Minute))
	llmFake := &conversationScriptedLLM{failing: map[string]bool{bad: true}}
	analyzer := NewSupportCoverageDailyAnalyzer(llmFake, "", "").
		SetCoverageRepositories(repository.NewSupportCoverageRepository(db), repository.NewSupportCoverageAnalysisRepository(db)).
		SetCoverageV2Repository(repository.NewCoverageV2Repository(db)).
		SetEmbeddingProvider(fixedCoverageEmbeddings{}, "").
		SetConversationRepositories(repository.NewSupportConversationRepository(db), repository.NewSupportMessageRepository(db))

	if err := analyzer.RunWorkspaceDailyAnalysis(ctx, workspaceID, now.Add(-24*time.Hour), now); err != nil {
		t.Fatalf("run should finish despite one failed conversation: %v", err)
	}

	var batch model.CoverageBatch
	if err := db.Where("workspace_id = ?", workspaceID).Order("created_at DESC").First(&batch).Error; err != nil {
		t.Fatalf("load batch: %v", err)
	}
	if batch.Status != model.CoverageBatchPartialFailed || batch.RetryableCount != 1 || batch.SucceededCount != 1 {
		t.Fatalf("batch = %s succeeded=%d retryable=%d, want partial_failed 1/1", batch.Status, batch.SucceededCount, batch.RetryableCount)
	}
	if strings.Contains(batch.FailureMessage, "type vector") {
		t.Fatalf("failure recording hit the vector column: %s", batch.FailureMessage)
	}
	var runStatus string
	if err := db.Raw(`SELECT status FROM support_coverage_analysis_runs WHERE workspace_id = ? ORDER BY started_at DESC LIMIT 1`, workspaceID).Scan(&runStatus).Error; err != nil {
		t.Fatal(err)
	}
	if runStatus != model.SupportCoverageAnalysisRunStatusCompleted {
		t.Fatalf("run status = %q, want completed so the cursor advances", runStatus)
	}
	var failedAnalyses, gaps int64
	if err := db.Model(&model.SupportCoverageConversationAnalysis{}).Where("workspace_id = ? AND conversation_id = ? AND status = ?", workspaceID, bad, model.SupportCoverageConversationAnalysisStatusFailed).Count(&failedAnalyses).Error; err != nil {
		t.Fatal(err)
	}
	if failedAnalyses != 1 {
		t.Fatalf("failed analyses saved = %d, want 1", failedAnalyses)
	}
	if err := db.Model(&model.SupportCoverageGap{}).Where("workspace_id = ?", workspaceID).Count(&gaps).Error; err != nil {
		t.Fatal(err)
	}
	if gaps != 1 {
		t.Fatalf("gaps = %d, want 1 from the good conversation", gaps)
	}
	var retryable int64
	if err := db.Model(&model.CoverageAnalysisAttempt{}).Where("workspace_id = ? AND source_id = ? AND status = ?", workspaceID, bad, model.CoverageAttemptRetryable).Count(&retryable).Error; err != nil {
		t.Fatal(err)
	}
	if retryable != 1 {
		t.Fatalf("retryable attempts for the failed conversation = %d, want 1", retryable)
	}

	// The provider recovers; the next run retries the failed conversation and
	// its finding joins the existing gap.
	llmFake.mu.Lock()
	llmFake.failing = map[string]bool{}
	llmFake.mu.Unlock()
	if err := db.Exec(`UPDATE coverage_analysis_attempts SET retry_at = ? WHERE workspace_id = ? AND status = ?`, now.Add(-time.Minute), workspaceID, model.CoverageAttemptRetryable).Error; err != nil {
		t.Fatal(err)
	}
	if err := analyzer.RunWorkspaceDailyAnalysis(ctx, workspaceID, now.Add(-24*time.Hour), now.Add(time.Minute)); err != nil {
		t.Fatalf("retry run: %v", err)
	}
	var retried model.SupportCoverageConversationAnalysis
	if err := db.Where("workspace_id = ? AND conversation_id = ?", workspaceID, bad).First(&retried).Error; err != nil {
		t.Fatal(err)
	}
	if retried.Status != model.SupportCoverageConversationAnalysisStatusAnalyzed || retried.GapID == nil {
		t.Fatalf("retried analysis status=%s gap=%v, want analyzed and attached to a gap", retried.Status, retried.GapID)
	}
	var evidence int64
	if err := db.Model(&model.SupportGapEvidence{}).Where("workspace_id = ? AND gap_id = ?", workspaceID, *retried.GapID).Count(&evidence).Error; err != nil {
		t.Fatal(err)
	}
	if evidence != 2 {
		t.Fatalf("evidence on the gap = %d, want both conversations", evidence)
	}
}

// A raw event-detection gap must never absorb an analysed finding, even when
// its embedding is identical: the finding becomes its own gap titled by the
// customer need.
func TestCoverageWorkspaceRunPostgresSkipsRawEventGaps(t *testing.T) {
	db := openCoverageRunPostgres(t)
	ctx := context.Background()
	workspaceID, conversationID := uuid.NewString(), uuid.NewString()
	now := time.Now().UTC()
	seedCoverageRunWorkspace(t, db, workspaceID)
	seedCoverageRunConversation(t, db, workspaceID, conversationID, 1, now.Add(-time.Hour))
	vector := make([]float32, 1536)
	vector[0] = 1
	raw, _, err := repository.NewSupportCoverageRepository(db).UpsertOpenGapByDedupeKeyNoBump(ctx, &model.SupportCoverageGap{
		ID: uuid.NewString(), WorkspaceID: workspaceID, DedupeKey: "raw-" + conversationID,
		GapKind: "content", GapCategory: model.SupportCoverageGapCategoryUnknown, V1GapType: model.SupportCoverageV1GapNeedsReview,
		Title: "Human resolved after AI failure", Status: model.SupportCoverageGapStatusOpen,
		Metadata: []byte(`{"source":"event_detection"}`), FirstSeenAt: now, LastSeenAt: now,
		Embedding: coverageVectorLiteral(vector), EmbeddingProvider: coverageEmbeddingProviderName,
		EmbeddingModel: coverageEmbeddingModel(""), EmbeddingVersion: coverageGapEmbeddingVersion, EmbeddingDimensions: len(vector),
	})
	if err != nil {
		t.Fatalf("seed raw gap: %v", err)
	}
	analyzer := NewSupportCoverageDailyAnalyzer(&conversationScriptedLLM{}, "", "").
		SetCoverageRepositories(repository.NewSupportCoverageRepository(db), repository.NewSupportCoverageAnalysisRepository(db)).
		SetCoverageV2Repository(repository.NewCoverageV2Repository(db)).
		SetEmbeddingProvider(fixedCoverageEmbeddings{}, "").
		SetConversationRepositories(repository.NewSupportConversationRepository(db), repository.NewSupportMessageRepository(db))

	if err := analyzer.RunWorkspaceDailyAnalysis(ctx, workspaceID, now.Add(-24*time.Hour), now); err != nil {
		t.Fatalf("run: %v", err)
	}
	var analysis model.SupportCoverageConversationAnalysis
	if err := db.Where("workspace_id = ? AND conversation_id = ?", workspaceID, conversationID).First(&analysis).Error; err != nil {
		t.Fatal(err)
	}
	if analysis.GapID == nil || *analysis.GapID == raw.ID {
		t.Fatalf("finding attached to %v, want a new gap instead of the raw gap %s", analysis.GapID, raw.ID)
	}
	var gap model.SupportCoverageGap
	if err := db.Where("id = ?", *analysis.GapID).First(&gap).Error; err != nil {
		t.Fatal(err)
	}
	if gap.Title != "Billing update steps" || gap.SourceSignal != model.SupportCoverageGapSourceDailyConversationAnalysis {
		t.Fatalf("gap title=%q source=%q, want the analysed customer need", gap.Title, gap.SourceSignal)
	}
}

// titleCoverageEmbeddings gives "Password reset instructions" a vector at
// cosine 0.8 from every other text: close enough to ask the judge.
type titleCoverageEmbeddings struct{}

func (titleCoverageEmbeddings) CreateEmbeddings(_ context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	vectors := make([][]float32, 0, len(req.Inputs))
	for _, input := range req.Inputs {
		vector := make([]float32, 1536)
		if strings.Contains(input, "Password reset instructions") {
			vector[0], vector[1] = 0.8, 0.6
		} else {
			vector[0] = 1
		}
		vectors = append(vectors, vector)
	}
	return &llm.EmbeddingResponse{Vectors: vectors}, nil
}

func TestCoverageDuplicateCheckPostgresMergesJudgedDuplicates(t *testing.T) {
	db := openCoverageRunPostgres(t)
	ctx := context.Background()
	workspaceID := uuid.NewString()
	seedCoverageRunWorkspace(t, db, workspaceID)
	now := time.Now().UTC()
	target, source := uuid.NewString(), uuid.NewString()
	for _, gap := range []model.SupportCoverageGap{
		{ID: target, DedupeKey: "reset-" + target, Title: "Users cannot reset passwords", EvidenceCount: 3},
		{ID: source, DedupeKey: "password-" + source, Title: "Password reset instructions are missing", EvidenceCount: 2},
	} {
		gap.WorkspaceID, gap.GapKind, gap.GapCategory = workspaceID, "content", model.SupportCoverageGapCategoryKnowledge
		gap.Status, gap.Confidence, gap.FirstSeenAt, gap.LastSeenAt = model.SupportCoverageGapStatusOpen, 0.8, now.Add(-time.Hour), now
		gap.Metadata = json.RawMessage(`{}`)
		if err := db.Omit("Embedding").Create(&gap).Error; err != nil {
			t.Fatalf("seed gap: %v", err)
		}
	}
	judge := &needMatchLLM{match: true}
	checker := NewSupportCoverageClusterRebuildService(repository.NewSupportCoverageRepository(db), titleCoverageEmbeddings{}, "").
		SetNeedJudge(judge, nil)

	result, err := checker.RebuildWorkspace(ctx, workspaceID)

	if err != nil {
		t.Fatalf("RebuildWorkspace: %v", err)
	}
	if result.AutoMerged != 1 || result.SuggestionsCreated != 0 || judge.calls != 1 {
		t.Fatalf("result = %+v, judge calls = %d; want one judged merge and nothing left", result, judge.calls)
	}
	var status string
	if err := db.Raw(`SELECT status FROM support_coverage_gaps WHERE id = ?`, source).Scan(&status).Error; err != nil {
		t.Fatal(err)
	}
	if status == model.SupportCoverageGapStatusOpen {
		t.Fatal("judged duplicate should no longer be open")
	}
}
