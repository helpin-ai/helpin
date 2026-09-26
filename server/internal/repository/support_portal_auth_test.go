package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPortalAuthTokensAreScopedSingleUseAndRevocable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE support_portal_magic_links (id TEXT PRIMARY KEY, workspace_id TEXT, email TEXT, token_hash TEXT, expires_at DATETIME, used_at DATETIME, created_at DATETIME)`,
		`CREATE TABLE support_portal_sessions (id TEXT PRIMARY KEY, workspace_id TEXT, identity_id TEXT, token_hash TEXT, expires_at DATETIME, revoked_at DATETIME, created_at DATETIME)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := NewPortalAuthRepository(db)
	ctx := context.Background()
	now := time.Now()
	link := &model.PortalMagicLink{ID: "link", WorkspaceID: "workspace-a", Email: "customer@example.com", TokenHash: "link-hash", ExpiresAt: now.Add(time.Minute)}
	if err := repo.CreateLink(ctx, link); err != nil {
		t.Fatal(err)
	}
	create := func(tx *gorm.DB, link *model.PortalMagicLink) (*model.PortalSession, error) {
		return &model.PortalSession{ID: "session", WorkspaceID: link.WorkspaceID, IdentityID: "identity", TokenHash: "session-hash", ExpiresAt: now.Add(time.Hour)}, nil
	}
	if _, err := repo.ConsumeLink(ctx, "workspace-b", "link-hash", now, create); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-workspace exchange: %v", err)
	}
	if _, err := repo.ConsumeLink(ctx, "workspace-a", "link-hash", now.Add(time.Hour), create); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expired exchange: %v", err)
	}
	if _, err := repo.ConsumeLink(ctx, "workspace-a", "link-hash", now, create); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ConsumeLink(ctx, "workspace-a", "link-hash", now, create); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("reused exchange: %v", err)
	}
	if _, err := repo.FindSession(ctx, "workspace-b", "session-hash", now); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-workspace session: %v", err)
	}
	if _, err := repo.FindSession(ctx, "workspace-a", "session-hash", now); err != nil {
		t.Fatal(err)
	}
	if err := repo.RevokeSession(ctx, "workspace-a", "session-hash", now); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindSession(ctx, "workspace-a", "session-hash", now); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("revoked session: %v", err)
	}
}
