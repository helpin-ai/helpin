package repository

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPublicShareRepositoryCreateActiveIsIdempotentAndRevokeRotatesToken(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Exec(`CREATE TABLE public_shares (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))), workspace_id TEXT NOT NULL,
		resource_type TEXT NOT NULL, resource_id TEXT NOT NULL, token TEXT NOT NULL UNIQUE,
		created_by TEXT NOT NULL, revoked_by TEXT, revoked_at DATETIME,
		created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create public shares: %v", err)
	}
	repo := NewPublicShareRepository(db)
	ctx := context.Background()

	first, err := repo.CreateActive(ctx, "ws-1", "dock_chat", "chat-1", "user-1")
	if err != nil {
		t.Fatalf("create first share: %v", err)
	}
	second, err := repo.CreateActive(ctx, "ws-1", "dock_chat", "chat-1", "user-2")
	if err != nil {
		t.Fatalf("create existing share: %v", err)
	}
	if first.Token != second.Token {
		t.Fatalf("idempotent token changed from %q to %q", first.Token, second.Token)
	}
	active, err := repo.GetActiveByResource(ctx, "ws-1", "dock_chat", "chat-1")
	if err != nil || active == nil || active.Token != first.Token {
		t.Fatalf("active resource lookup = %#v, %v", active, err)
	}
	if err := repo.RevokeActive(ctx, "ws-1", "dock_chat", "chat-1", "user-2"); err != nil {
		t.Fatalf("revoke share: %v", err)
	}
	if found, err := repo.GetActiveByToken(ctx, first.Token); err != nil || found != nil {
		t.Fatalf("revoked token lookup = %#v, %v", found, err)
	}
	rotated, err := repo.CreateActive(ctx, "ws-1", "dock_chat", "chat-1", "user-2")
	if err != nil {
		t.Fatalf("create rotated share: %v", err)
	}
	if rotated.Token == first.Token {
		t.Fatal("new share reused revoked token")
	}
}
