package dbmigrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateUsesTimestampVersion(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 4, 29, 10, 11, 12, 345678000, time.UTC)

	path, err := createAt(dir, "Add Billing Events!", now)
	if err != nil {
		t.Fatalf("create migration: %v", err)
	}

	const wantName = "20260429101112345678_add_billing_events.sql"
	if filepath.Base(path) != wantName {
		t.Fatalf("migration filename = %q, want %q", filepath.Base(path), wantName)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if string(contents) != "-- Migration: add_billing_events\n" {
		t.Fatalf("migration contents = %q", string(contents))
	}
}

func TestCreateBumpsTimestampVersionOnExactCollision(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 4, 29, 10, 11, 12, 345678000, time.UTC)

	if _, err := createAt(dir, "first", now); err != nil {
		t.Fatalf("create first migration: %v", err)
	}
	path, err := createAt(dir, "second", now)
	if err != nil {
		t.Fatalf("create second migration: %v", err)
	}

	const wantPrefix = "20260429101112345679_second.sql"
	if filepath.Base(path) != wantPrefix {
		t.Fatalf("migration filename = %q, want %q", filepath.Base(path), wantPrefix)
	}
}

func TestCreateKeepsLegacySequentialVersionsAsExistingVersions(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "202604290001_existing.sql"), []byte("SELECT 1;\n"), 0644); err != nil {
		t.Fatalf("write legacy migration: %v", err)
	}

	path, err := createAt(dir, "new migration", time.Date(2026, 4, 29, 0, 0, 1, 0, time.UTC))
	if err != nil {
		t.Fatalf("create migration: %v", err)
	}

	if !strings.HasPrefix(filepath.Base(path), "20260429000001000000_") {
		t.Fatalf("migration filename = %q, want timestamp prefix", filepath.Base(path))
	}
}

func TestSupportConversationCompanyContextMigrationContract(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}

	var migration *Migration
	for i := range migrations {
		if migrations[i].Version == "202608050001" {
			migration = &migrations[i]
			break
		}
	}
	if migration == nil {
		t.Fatal("expected support conversation company context migration 202608050001 to be registered")
	}
	if migration.Name != "support_conversation_company_context" {
		t.Fatalf("migration name = %q, want %q", migration.Name, "support_conversation_company_context")
	}

	sql := strings.ToLower(strings.Join(strings.Fields(migration.SQL), " "))
	for _, clause := range []string{
		"alter table support_conversations add column if not exists crm_company_id uuid",
		"alter table support_widget_sessions add column if not exists crm_company_id uuid",
		"create index if not exists idx_support_conversations_crm_company_id on support_conversations (crm_company_id)",
		"create index if not exists idx_support_widget_sessions_crm_company_id on support_widget_sessions (crm_company_id)",
		"foreign key (crm_company_id) references crm_companies(id) on delete set null",
		"where from_object_type = 'contact' and to_object_type = 'company'",
		"having count(distinct to_object_id) = 1",
		"conversation.crm_company_id is null",
		"conversation.crm_contact_id = association.from_object_id",
		"conversation.workspace_id = association.workspace_id",
	} {
		if !strings.Contains(sql, clause) {
			t.Errorf("migration SQL missing contract clause %q", clause)
		}
	}

	if got := strings.Count(sql, "foreign key (crm_company_id) references crm_companies(id) on delete set null"); got != 2 {
		t.Errorf("company foreign key count = %d, want 2", got)
	}
	if got := strings.Count(sql, "if not exists ( select 1 from pg_constraint"); got != 2 {
		t.Errorf("idempotent foreign-key guard count = %d, want 2", got)
	}
	if strings.Contains(sql, "update support_widget_sessions") {
		t.Error("migration must not infer company context for existing widget sessions")
	}
}
