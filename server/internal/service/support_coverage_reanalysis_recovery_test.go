package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCoverageReanalysisRecoversTerminalFailureOutsideTheWindow(t *testing.T) {
	db := setupCoverageFindingUpsertTestDB(t)
	for _, statement := range []string{
		`CREATE TABLE support_coverage_analysis_runs (id TEXT PRIMARY KEY,workspace_id TEXT,window_start DATETIME,window_end DATETIME,cursor_started_at DATETIME,cursor_ended_at DATETIME,analyzer_version TEXT,status TEXT,conversation_cnt INTEGER,gap_count INTEGER,error_message TEXT,metadata TEXT,started_at DATETIME,completed_at DATETIME,created_at DATETIME,updated_at DATETIME)`,
		`CREATE TABLE support_conversations (id TEXT PRIMARY KEY,workspace_id TEXT,subject TEXT,status TEXT,created_at DATETIME,updated_at DATETIME,resolved_at DATETIME)`,
		`CREATE TABLE sample_data_items (entity_id TEXT)`,
		`CREATE TABLE coverage_batches (id TEXT PRIMARY KEY,workspace_id TEXT,window_start DATETIME,window_end DATETIME,analyzer_version TEXT,policy_version TEXT,status TEXT,cursor TEXT,lease_owner TEXT,lease_expires_at DATETIME,candidate_count INTEGER,succeeded_count INTEGER,retryable_count INTEGER,dead_letter_count INTEGER,failure_class TEXT,failure_message TEXT,correlation_id TEXT,metadata TEXT,started_at DATETIME,completed_at DATETIME,created_at DATETIME,updated_at DATETIME,UNIQUE(workspace_id,window_start,window_end,analyzer_version,policy_version))`,
		`CREATE TABLE coverage_analysis_attempts (id TEXT PRIMARY KEY,workspace_id TEXT,batch_id TEXT,logical_work_key TEXT,source_kind TEXT,source_id TEXT,segment_id TEXT,content_hash TEXT,analyzer_version TEXT,policy_version TEXT,attempt INTEGER,status TEXT,stage TEXT,lease_owner TEXT,lease_expires_at DATETIME,retry_at DATETIME,retry_budget_used INTEGER,failure_class TEXT,failure_message TEXT,ai_execution_id TEXT,correlation_id TEXT,metadata TEXT,started_at DATETIME,completed_at DATETIME,created_at DATETIME,UNIQUE(workspace_id,logical_work_key,attempt))`,
		`CREATE TABLE coverage_findings (id TEXT PRIMARY KEY,workspace_id TEXT,is_current BOOLEAN,assignment_status TEXT,created_at DATETIME)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	now := time.Now().UTC()
	old := now.Add(-60 * 24 * time.Hour)
	if err := db.Exec(`INSERT INTO support_conversations VALUES ('c-1','ws-1','Billing help','resolved',?,?,?)`, old, old, old).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO support_messages (id,workspace_id,conversation_id,sender_type,message_type,content,created_at) VALUES ('m-1','ws-1','c-1','customer','reply','How do I update my billing details?',?)`, old).Error; err != nil {
		t.Fatal(err)
	}
	v2 := repository.NewCoverageV2Repository(db)
	if err := v2.EnqueueAttempt(ctx, &model.CoverageAnalysisAttempt{
		ID: "old-failure", WorkspaceID: "ws-1", BatchID: "old-batch", LogicalWorkKey: "old-work",
		SourceKind: "conversation", SourceID: "c-1", SegmentID: "old-segment", ContentHash: "old-hash",
		AnalyzerVersion: "v4", PolicyVersion: "v1", Attempt: 1, Status: model.CoverageAttemptDeadLetter,
		RetryBudgetUsed: 3, FailureClass: model.CoverageFailureLLMContract, CreatedAt: old,
	}); err != nil {
		t.Fatal(err)
	}
	provider := &scriptedSupportPlannerLLM{responses: []llm.ChatResponse{{Content: `{"conversation_type":"support_query","has_gap":false,"decision_reason":"Current knowledge covers this question."}`}}}
	analyzer := NewSupportCoverageDailyAnalyzer(provider, "", "").
		SetCoverageRepositories(repository.NewSupportCoverageRepository(db), repository.NewSupportCoverageAnalysisRepository(db)).
		SetCoverageV2Repository(v2).
		SetConversationRepositories(repository.NewSupportConversationRepository(db), repository.NewSupportMessageRepository(db))
	if err := analyzer.RunWorkspaceReanalysis(ctx, "ws-1", now.Add(-30*24*time.Hour), now, "manual-recovery"); err != nil {
		t.Fatal(err)
	}
	if len(provider.requests) != 1 {
		t.Fatalf("model calls = %d, want one fresh reassessment", len(provider.requests))
	}
	var recovered model.SupportCoverageConversationAnalysis
	if err := db.Where("conversation_id = ?", "c-1").First(&recovered).Error; err != nil {
		t.Fatal(err)
	}
	if recovered.Status != model.SupportCoverageConversationAnalysisStatusAnalyzed {
		t.Fatalf("analysis status = %s", recovered.Status)
	}
	var succeeded int64
	if err := db.Model(&model.CoverageAnalysisAttempt{}).Where("source_id = ? AND status = ?", "c-1", model.CoverageAttemptSucceeded).Count(&succeeded).Error; err != nil {
		t.Fatal(err)
	}
	if succeeded != 1 {
		t.Fatalf("successful attempts = %d, want one", succeeded)
	}
	failures, err := v2.ListFailedAttempts(ctx, "ws-1", 25)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 0 {
		t.Fatalf("recovered conversation still unhealthy: %+v", failures)
	}
	var historical model.CoverageAnalysisAttempt
	if err := db.First(&historical, "id = ?", "old-failure").Error; err != nil {
		t.Fatal(err)
	}
	if historical.Status != model.CoverageAttemptDeadLetter {
		t.Fatal("historical failure was erased rather than recovered by a fresh attempt")
	}
}
