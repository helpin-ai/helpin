package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupCodexWorkspaceAuthTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:codex_workspace_auth_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE codex_workspace_auths (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			provider TEXT NOT NULL,
			auth_mode TEXT NOT NULL,
			auth_json_encrypted TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := db.Exec(`
		CREATE UNIQUE INDEX idx_codex_workspace_auth_scope
			ON codex_workspace_auths (workspace_id, provider, auth_mode)
	`).Error; err != nil {
		t.Fatalf("create unique index: %v", err)
	}
	return db
}

func TestCodexWorkspaceAuthRepositoryUpsertAndGet(t *testing.T) {
	db := setupCodexWorkspaceAuthTestDB(t)
	repo := NewCodexWorkspaceAuthRepository(db)
	ctx := context.Background()

	record := &model.CodexWorkspaceAuth{
		WorkspaceID:       "ws-1",
		Provider:          "openai",
		AuthMode:          "chatgpt_device_code",
		AuthJSONEncrypted: "ciphertext-a",
	}
	if err := repo.Upsert(ctx, record); err != nil {
		t.Fatalf("Upsert create: %v", err)
	}

	fetched, err := repo.GetByScope(ctx, "ws-1", "openai", "chatgpt_device_code")
	if err != nil {
		t.Fatalf("GetByScope: %v", err)
	}
	if fetched == nil || fetched.AuthJSONEncrypted != "ciphertext-a" {
		t.Fatalf("unexpected fetched record: %+v", fetched)
	}

	record.AuthJSONEncrypted = "ciphertext-b"
	if err := repo.Upsert(ctx, record); err != nil {
		t.Fatalf("Upsert update: %v", err)
	}

	updated, err := repo.GetByScope(ctx, "ws-1", "openai", "chatgpt_device_code")
	if err != nil {
		t.Fatalf("GetByScope after update: %v", err)
	}
	if updated == nil || updated.AuthJSONEncrypted != "ciphertext-b" {
		t.Fatalf("unexpected updated record: %+v", updated)
	}

	var count int64
	if err := db.Table("codex_workspace_auths").Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("row count = %d, want 1", count)
	}
}

func TestCodexWorkspaceAuthRepositoryDeleteByScope(t *testing.T) {
	db := setupCodexWorkspaceAuthTestDB(t)
	repo := NewCodexWorkspaceAuthRepository(db)
	ctx := context.Background()

	record := &model.CodexWorkspaceAuth{
		WorkspaceID:       "ws-1",
		Provider:          "openai",
		AuthMode:          "chatgpt_device_code",
		AuthJSONEncrypted: "ciphertext-a",
	}
	if err := repo.Upsert(ctx, record); err != nil {
		t.Fatalf("Upsert create: %v", err)
	}

	if err := repo.DeleteByScope(ctx, "ws-1", "openai", "chatgpt_device_code"); err != nil {
		t.Fatalf("DeleteByScope: %v", err)
	}

	fetched, err := repo.GetByScope(ctx, "ws-1", "openai", "chatgpt_device_code")
	if err != nil {
		t.Fatalf("GetByScope after delete: %v", err)
	}
	if fetched != nil {
		t.Fatalf("expected no record after delete, got %+v", fetched)
	}
}
