package repository

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAIActionExecutionRepositoryStartAndFinishAreIdempotent(t *testing.T) {
	db := setupAIActionExecutionTestDB(t, "ai_action_execution")
	repo := NewAIActionExecutionRepository(db)
	started := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	input := &model.AIActionExecution{
		WorkspaceID:    "11111111-1111-1111-1111-111111111111",
		ActionKey:      "support.coverage.analyze.v1",
		PolicyVersion:  "v1",
		FeatureKey:     "coverage_gap_analysis",
		Category:       "Support AI",
		Origin:         "coverage",
		Modality:       "chat",
		Provider:       "anthropic",
		Model:          "claude-sonnet-4-6",
		IdempotencyKey: "coverage:item-1",
		Attempt:        1,
		Status:         model.AIActionExecutionRunning,
		StartedAt:      started,
	}
	first, err := repo.Start(context.Background(), input)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	again, err := repo.Start(context.Background(), input)
	if err != nil {
		t.Fatalf("Start duplicate: %v", err)
	}
	if again.ID != first.ID {
		t.Fatalf("duplicate ID = %q, want %q", again.ID, first.ID)
	}

	completed := started.Add(2 * time.Second)
	result := aipolicy.ExecutionResult{
		Status:       model.AIActionExecutionSucceeded,
		InputTokens:  100,
		OutputTokens: 25,
		CompletedAt:  completed,
	}
	if err := repo.Finish(context.Background(), first.ID, result); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := repo.Finish(context.Background(), first.ID, result); err != nil {
		t.Fatalf("Finish duplicate: %v", err)
	}

	var stored model.AIActionExecution
	if err := db.First(&stored, "id = ?", first.ID).Error; err != nil {
		t.Fatalf("load stored execution: %v", err)
	}
	if stored.Status != model.AIActionExecutionSucceeded || stored.InputTokens != 100 || stored.OutputTokens != 25 {
		t.Fatalf("stored execution = %+v", stored)
	}
}

func TestAIActionExecutionRepositoryDoesNotMutateRequestIdentity(t *testing.T) {
	db := setupAIActionExecutionTestDB(t, "ai_action_identity")
	repo := NewAIActionExecutionRepository(db)
	execution, err := repo.Start(context.Background(), &model.AIActionExecution{
		ActionKey: "support.test.v1", PolicyVersion: "v1", FeatureKey: "test",
		Category: "Support AI", Origin: "test", Modality: "chat", Provider: "openai",
		Model: "gpt-test", IdempotencyKey: "stable", Attempt: 1,
		Status: model.AIActionExecutionRunning, StartedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := repo.Finish(context.Background(), execution.ID, aipolicy.ExecutionResult{
		Status: model.AIActionExecutionFailed, FailureClass: "llm_provider",
		FailureMessage: "provider unavailable", CompletedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	var stored model.AIActionExecution
	if err := db.First(&stored, "id = ?", execution.ID).Error; err != nil {
		t.Fatalf("load: %v", err)
	}
	if stored.ActionKey != "support.test.v1" || stored.Provider != "openai" || stored.IdempotencyKey != "stable" {
		t.Fatalf("request identity mutated: %+v", stored)
	}
}

func setupAIActionExecutionTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE ai_action_executions (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL DEFAULT '',
		action_key TEXT NOT NULL,
		policy_version TEXT NOT NULL,
		feature_key TEXT NOT NULL,
		category TEXT NOT NULL,
		origin TEXT NOT NULL,
		modality TEXT NOT NULL,
		provider TEXT NOT NULL,
		model TEXT NOT NULL,
		idempotency_key TEXT NOT NULL,
		attempt INTEGER NOT NULL,
		status TEXT NOT NULL,
		failure_class TEXT NOT NULL DEFAULT '',
		failure_message TEXT NOT NULL DEFAULT '',
		input_tokens INTEGER NOT NULL DEFAULT 0,
		output_tokens INTEGER NOT NULL DEFAULT 0,
		reasoning_tokens INTEGER NOT NULL DEFAULT 0,
		cached_input_tokens INTEGER NOT NULL DEFAULT 0,
		metadata TEXT NOT NULL DEFAULT '{}',
		started_at DATETIME NOT NULL,
		completed_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(idempotency_key, attempt)
	)`).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}
