package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func setupUserTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:user_repo_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			email TEXT NOT NULL,
			password_hash TEXT NOT NULL DEFAULT '',
			full_name TEXT NOT NULL DEFAULT '',
			email_verified_at DATETIME,
			google_subject TEXT,
			avatar_url TEXT,
			avatar_style TEXT,
			avatar_seed TEXT,
			avatar_background_mode TEXT,
			avatar_background_color TEXT,
			default_workspace_id TEXT,
			totp_secret_encrypted TEXT,
			totp_verified BOOLEAN NOT NULL DEFAULT 0,
			recovery_codes_encrypted TEXT,
			is_platform_admin BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

// TestUserRepository_GetByEmail_CaseInsensitive proves that a stored email
// with different casing than the (already normalized/lowercased) query email
// is still found. This is the defect that forked duplicate accounts in prod:
// callers normalize the lookup email to lowercase, but the row was stored with
// mixed case, so a case-sensitive lookup missed it and auth minted a new user.
func TestUserRepository_GetByEmail_CaseInsensitive(t *testing.T) {
	db := setupUserTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	// Row stored with mixed-case email (legacy / invited casing).
	if _, err := repo.CreateUser(ctx, &model.User{
		Email:    "Mixed.Case@example.com",
		FullName: "Mixed Case",
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Auth normalizes the input to lowercase before looking up.
	user, err := repo.GetByEmail(ctx, "mixed.case@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if user == nil {
		t.Fatal("GetByEmail returned nil for a case-variant of an existing email; this forks duplicate accounts")
	}
	if user.FullName != "Mixed Case" {
		t.Fatalf("full_name = %q, want the existing user", user.FullName)
	}
}
