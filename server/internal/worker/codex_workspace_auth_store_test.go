package worker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	appmodel "github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupCodexWorkspaceAuthStoreTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:codex_workspace_auth_store_%d?mode=memory&cache=shared", time.Now().UnixNano())
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

func TestCodexWorkspaceAuthStorePromoteAndRestore(t *testing.T) {
	db := setupCodexWorkspaceAuthStoreTestDB(t)
	store := NewCodexWorkspaceAuthStore(repository.NewCodexWorkspaceAuthRepository(db), make([]byte, 32))
	ctx := context.Background()

	sessionHome := filepath.Join(t.TempDir(), "session", ".codex")
	if err := os.MkdirAll(sessionHome, 0o755); err != nil {
		t.Fatalf("mkdir session home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionHome, codexAuthFileName), []byte(`{"tokens":{"access_token":"abc"}}`), 0o600); err != nil {
		t.Fatalf("write session auth: %v", err)
	}

	if err := store.Promote(ctx, "ws-1", appmodel.AgentModelProviderOpenAI, codexOpenAIAuthModeDevice, sessionHome); err != nil {
		t.Fatalf("promote auth: %v", err)
	}

	restoreHome := filepath.Join(t.TempDir(), "restore", ".codex")
	if err := os.MkdirAll(restoreHome, 0o755); err != nil {
		t.Fatalf("mkdir restore home: %v", err)
	}
	if err := store.Restore(ctx, "ws-1", appmodel.AgentModelProviderOpenAI, codexOpenAIAuthModeDevice, restoreHome); err != nil {
		t.Fatalf("restore auth: %v", err)
	}

	restored, err := os.ReadFile(filepath.Join(restoreHome, codexAuthFileName))
	if err != nil {
		t.Fatalf("read restored auth: %v", err)
	}
	if string(restored) != `{"tokens":{"access_token":"abc"}}` {
		t.Fatalf("unexpected restored auth: %s", string(restored))
	}
}

func TestCodexWorkspaceAuthStoreSkipsUnsupportedModes(t *testing.T) {
	db := setupCodexWorkspaceAuthStoreTestDB(t)
	store := NewCodexWorkspaceAuthStore(repository.NewCodexWorkspaceAuthRepository(db), make([]byte, 32))
	ctx := context.Background()

	sessionHome := filepath.Join(t.TempDir(), "session", ".codex")
	if err := os.MkdirAll(sessionHome, 0o755); err != nil {
		t.Fatalf("mkdir session home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionHome, codexAuthFileName), []byte(`{"api_key":"x"}`), 0o600); err != nil {
		t.Fatalf("write session auth: %v", err)
	}

	if err := store.Promote(ctx, "ws-1", appmodel.AgentModelProviderOpenAI, codexOpenAIAuthModeAPIKey, sessionHome); err != nil {
		t.Fatalf("promote unsupported mode: %v", err)
	}

	record, err := repository.NewCodexWorkspaceAuthRepository(db).GetByScope(ctx, "ws-1", appmodel.AgentModelProviderOpenAI, codexOpenAIAuthModeAPIKey)
	if err != nil {
		t.Fatalf("get by scope: %v", err)
	}
	if record != nil {
		t.Fatalf("expected no persisted record for unsupported mode, got %+v", record)
	}
}
