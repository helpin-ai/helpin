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

func TestTieredAIUsageCutoverMigrationContract(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	var migration *Migration
	for i := range migrations {
		if migrations[i].Version == "202608130002" {
			migration = &migrations[i]
			break
		}
	}
	if migration == nil {
		t.Fatal("expected tiered AI usage cutover migration 202608130002")
	}
	sql := strings.ToLower(strings.Join(strings.Fields(migration.SQL), " "))
	for _, clause := range []string{
		"create table if not exists billing_ai_usage_periods",
		"create table if not exists billing_ai_usage_ledger",
		"create table if not exists billing_ai_usage_reservations",
		"create table if not exists billing_ai_usage_settlements",
		"create table if not exists billing_ai_usage_task_estimates",
		"create table if not exists billing_ai_usage_pricing_state",
		"insert into billing_ai_usage_periods",
		"'2026-08-13'",
		"rename to billing_credit_ledger_legacy",
	} {
		if !strings.Contains(sql, clause) {
			t.Errorf("migration missing behavior %q", clause)
		}
	}
	for _, destructive := range []string{
		"rename column on_demand_enabled", "drop column if exists included_credits",
		"drop column if exists credits_used", "drop column if exists on_demand_blocks_invoiced",
	} {
		if strings.Contains(sql, destructive) {
			t.Errorf("activation migration contains premature contract change %q", destructive)
		}
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
		"where from_object_type = 'company' and to_object_type = 'contact'",
		"having count(distinct company_id) = 1",
		"conversation.crm_company_id is null",
		"conversation.crm_contact_id = association.contact_id",
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

func TestCustomerIOLifecycleOutboxMigrationContract(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}

	var migration *Migration
	for i := range migrations {
		if migrations[i].Version == "202608100002" {
			migration = &migrations[i]
			break
		}
	}
	if migration == nil {
		t.Fatal("expected Customer.io lifecycle outbox migration 202608100002 to be registered")
	}
	if migration.Name != "customer_io_lifecycle_outbox" {
		t.Fatalf("migration name = %q, want %q", migration.Name, "customer_io_lifecycle_outbox")
	}

	sql := strings.ToLower(strings.Join(strings.Fields(migration.SQL), " "))
	for _, clause := range []string{
		"attributes jsonb not null default '{}'::jsonb",
		"recipient_snapshot jsonb not null default '[]'::jsonb",
		"check (status in ('pending', 'processing', 'delivered', 'failed'))",
		"check (attempts >= 0)",
		"unique (semantic_key)",
		"workspace_id uuid null references workspaces(id) on delete set null",
		"create index if not exists idx_customer_io_outbox_due_work on customer_io_outbox (status, next_attempt_at, lease_expires_at)",
		"where status in ('pending', 'processing')",
	} {
		if !strings.Contains(sql, clause) {
			t.Errorf("migration SQL missing contract clause %q", clause)
		}
	}

	if !strings.Contains(sql, "create table if not exists customer_io_outbox") {
		t.Error("migration must create the outbox table idempotently")
	}
}

func TestSupportEmailRouteVerificationBackfillMigrationContract(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}

	var prerequisite *Migration
	var migration *Migration
	for i := range migrations {
		if migrations[i].Version == "20260811000350000000" {
			prerequisite = &migrations[i]
		}
		if migrations[i].Version == "202608110004" {
			migration = &migrations[i]
		}
	}
	if prerequisite == nil {
		t.Fatal("expected support email route verification column migration 20260811000350000000 to be registered")
	}
	if migration == nil {
		t.Fatal("expected support email route verification backfill migration 202608110004 to be registered")
	}
	if prerequisite.Version >= migration.Version {
		t.Fatalf("verification column migration %s must sort before backfill migration %s", prerequisite.Version, migration.Version)
	}
	prerequisiteSQL := strings.ToLower(strings.Join(strings.Fields(prerequisite.SQL), " "))
	for _, clause := range []string{
		"alter table if exists support_email_routes",
		"add column if not exists forwarding_verified_at timestamptz",
	} {
		if !strings.Contains(prerequisiteSQL, clause) {
			t.Errorf("prerequisite migration SQL missing contract clause %q", clause)
		}
	}
	if migration.Name != "backfill_support_email_route_verification" {
		t.Fatalf("migration name = %q, want %q", migration.Name, "backfill_support_email_route_verification")
	}

	sql := strings.ToLower(strings.Join(strings.Fields(migration.SQL), " "))
	for _, clause := range []string{
		"update support_email_routes as route",
		"set forwarding_verified_at = evidence.verified_at",
		"from support_email_logs as email_log",
		"email_log.direction = 'inbound'",
		"email_log.email_route_id is not null",
		"forwarding-noreply@google.com",
		"mail-settings.google.com/mail/vf-",
		"zoho",
		"route.forwarding_verified_at is null",
	} {
		if !strings.Contains(sql, clause) {
			t.Errorf("migration SQL missing contract clause %q", clause)
		}
	}
}

func TestCompanyDealTimelineDedupeMigrationContract(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}

	var migration *Migration
	for i := range migrations {
		if migrations[i].Version == "202608210003" {
			migration = &migrations[i]
			break
		}
	}
	if migration == nil {
		t.Fatal("expected company deal timeline dedupe migration 202608210003")
	}

	sql := strings.ToLower(strings.Join(strings.Fields(migration.SQL), " "))
	for _, clause := range []string{
		"delete from pm_activity_log as backfilled",
		"coalesce(backfilled.metadata->>'backfilled', 'false') = 'true'",
		"canonical.new_value is not distinct from backfilled.new_value",
		"interval '5 minutes'",
	} {
		if !strings.Contains(sql, clause) {
			t.Errorf("migration missing behavior %q", clause)
		}
	}
}

func TestAgentModelTierBackfillMigrationContract(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	for _, migration := range migrations {
		if migration.Version != "202608210001" {
			continue
		}
		sql := strings.ToLower(migration.SQL)
		for _, table := range []string{"agents", "workspace_agent_preset_versions", "agent_versions", "agent_runs"} {
			if !strings.Contains(sql, table) || !strings.Contains(sql, "model_tier") {
				t.Fatalf("agent model tier migration is missing %s", table)
			}
		}
		for _, clause := range []string{"gpt-5-mini", "then 'medium'", "haiku-4", "then 'large'", "deepseek", "then 'small'"} {
			if !strings.Contains(sql, clause) {
				t.Fatalf("agent model tier migration is missing catalog backfill clause %q", clause)
			}
		}
		return
	}
	t.Fatal("expected agent model tier migration 202608210001 to be registered")
}
