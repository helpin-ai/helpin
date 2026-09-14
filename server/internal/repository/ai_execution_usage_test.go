package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func setupExecutionUsageDB(t *testing.T) (*gorm.DB, *AIExecutionUsageRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:execution_usage_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.AIExecutionUsage{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE agent_runs (id TEXT PRIMARY KEY, workspace_id TEXT, output_summary TEXT, updated_at DATETIME)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO agent_runs(id,workspace_id,output_summary) VALUES ('run','ws','{}')`).Error; err != nil {
		t.Fatal(err)
	}
	return db, NewAIExecutionUsageRepository(db)
}

func TestExecutionUsageRetryDoesNotOverwriteNewerSummary(t *testing.T) {
	db, repo := setupExecutionUsageDB(t)
	entry := model.AIExecutionUsage{WorkspaceID: "ws", IdempotencyKey: "turn-1", RunID: "run", Provider: "openai", Model: "custom", InputTokens: 100, PaidTools: []byte(`[]`)}
	if err := repo.RecordExecutionUsage(context.Background(), entry, []byte(`{"ai_usage_checkpoint":{"turn":1,"input_tokens":100}}`)); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordExecutionUsage(context.Background(), entry, []byte(`{"ai_usage_checkpoint":{"turn":1,"input_tokens":100}}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE agent_runs SET output_summary = '{"ai_usage_checkpoint":{"turn":2,"input_tokens":150}}' WHERE id = 'run'`).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordExecutionUsage(context.Background(), entry, []byte(`{"ai_usage_checkpoint":{"turn":1,"input_tokens":100}}`)); !errors.Is(err, ErrAIUsageWatermarkChanged) {
		t.Fatalf("stale retry error = %v", err)
	}
	var summary string
	if err := db.Raw(`SELECT output_summary FROM agent_runs WHERE id = 'run'`).Scan(&summary).Error; err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.AIExecutionUsage{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 || summary != `{"ai_usage_checkpoint":{"turn":2,"input_tokens":150}}` {
		t.Fatalf("count=%d summary=%s", count, summary)
	}
	entry.InputTokens++
	if err := repo.RecordExecutionUsage(context.Background(), entry, []byte(`{}`)); err == nil {
		t.Fatal("conflicting usage accepted")
	}
}

func TestExecutionUsageWrongWorkspaceRollsBack(t *testing.T) {
	db, repo := setupExecutionUsageDB(t)
	err := repo.RecordExecutionUsage(context.Background(), model.AIExecutionUsage{WorkspaceID: "other", RunID: "run", IdempotencyKey: "key"}, []byte(`{}`))
	if err == nil {
		t.Fatal("cross-workspace run accepted")
	}
	var count int64
	if err := db.Model(&model.AIExecutionUsage{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("usage insert survived failed watermark transaction")
	}
}
