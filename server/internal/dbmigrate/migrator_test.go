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

func TestMigrationNoTransactionDirective(t *testing.T) {
	t.Run("directive on first line disables transaction", func(t *testing.T) {
		migration := Migration{SQL: "-- dbmigrate:no-transaction\nCREATE INDEX CONCURRENTLY example_idx ON example_table (id);\n"}
		if !migration.NoTransaction() {
			t.Fatal("expected no-transaction directive to be detected")
		}
	})

	t.Run("directive elsewhere is ignored", func(t *testing.T) {
		migration := Migration{SQL: "-- Migration comment\n-- dbmigrate:no-transaction\nSELECT 1;\n"}
		if migration.NoTransaction() {
			t.Fatal("directive must be the first line")
		}
	})
}

func TestNonTransactionalMigrationStatementsExecuteSeparately(t *testing.T) {
	migration := Migration{SQL: "-- dbmigrate:no-transaction\n\nCREATE INDEX CONCURRENTLY first_idx ON first_table (id);\nCREATE INDEX CONCURRENTLY second_idx ON second_table (id);\n"}
	statements, err := migration.NonTransactionalStatements()
	if err != nil {
		t.Fatalf("split statements: %v", err)
	}
	if len(statements) != 2 {
		t.Fatalf("statement count = %d, want 2", len(statements))
	}
	if strings.Contains(statements[0], "second_idx") || !strings.Contains(statements[1], "second_idx") {
		t.Fatalf("statements were not separated: %#v", statements)
	}
}

func TestNonTransactionalMigrationRejectsTransactionControl(t *testing.T) {
	migration := Migration{SQL: "-- dbmigrate:no-transaction\nBEGIN;\nCREATE INDEX CONCURRENTLY example_idx ON example_table (id);\nCOMMIT;\n"}
	if _, err := migration.NonTransactionalStatements(); err == nil {
		t.Fatal("expected explicit transaction control to be rejected")
	}
}

func TestSupportInboxStateFoundationMigrationContract(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}

	var foundation, indexes *Migration
	for i := range migrations {
		switch migrations[i].Version {
		case "202609020003":
			foundation = &migrations[i]
		case "202609020004":
			indexes = &migrations[i]
		}
	}
	if foundation == nil {
		t.Fatal("expected support inbox state foundation migration")
	}
	if indexes == nil {
		t.Fatal("expected support inbox concurrent indexes migration")
	}

	foundationSQL := strings.ToLower(strings.Join(strings.Fields(foundation.SQL), " "))
	for _, clause := range []string{
		"add column if not exists customer_awaiting_response boolean",
		"add column if not exists needs_human_reply boolean",
		"create table if not exists support_conversation_user_states",
		"primary key (conversation_id, user_id)",
		"create table if not exists support_inbox_counter_buckets",
		"create table if not exists support_inbox_counter_contributions",
		"create table if not exists support_inbox_scope_heads",
		"create table if not exists support_inbox_user_scope_heads",
		"create table if not exists support_inbox_conversation_changes",
		"create table if not exists support_realtime_outbox",
	} {
		if !strings.Contains(foundationSQL, clause) {
			t.Errorf("foundation migration missing %q", clause)
		}
	}

	if !indexes.NoTransaction() {
		t.Fatal("concurrent index migration must opt out of the transaction wrapper")
	}
	indexSQL := strings.ToLower(strings.Join(strings.Fields(indexes.SQL), " "))
	for _, clause := range []string{
		"create index concurrently if not exists idx_support_messages_public_customer_replies",
		"create index concurrently if not exists idx_support_messages_metadata_path_ops",
		"create index concurrently if not exists idx_support_widget_sessions_workspace_anonymous_created",
		"create index concurrently if not exists idx_support_widget_sessions_conversation_created",
		"create index concurrently if not exists idx_support_inbox_counter_contributions_scope",
	} {
		if !strings.Contains(indexSQL, clause) {
			t.Errorf("index migration missing %q", clause)
		}
	}
}

func TestSupportInboxStateProjectionTriggersMigrationContract(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	var migration *Migration
	for i := range migrations {
		if migrations[i].Version == "202609020005" {
			migration = &migrations[i]
			break
		}
	}
	if migration == nil {
		t.Fatal("expected support inbox projection trigger migration")
	}
	sql := strings.ToLower(strings.Join(strings.Fields(migration.SQL), " "))
	for _, clause := range []string{
		"create or replace function project_support_message_state",
		"create trigger support_message_state_inserted",
		"create or replace function recompute_support_message_state",
		"create trigger support_message_state_updated",
		"create trigger support_message_state_deleted",
		"after update of deleted_at, is_internal, sender_type, message_type, content, created_at",
		"after delete on support_messages",
		"when is_customer_reply then true",
		"unread_customer_message_count = support_conversation_user_states.unread_customer_message_count + 1",
		"relevance_mask = support_conversation_user_states.relevance_mask | excluded.relevance_mask",
		"create or replace function project_support_conversation_workload",
		"create trigger support_conversation_workload_changed",
		"create or replace function project_support_conversation_change",
		"create trigger support_conversation_state_updated",
		"update of status, mailbox_id, assigned_user_id, opened_by_user_id, flow_state, ai_state, human_takeover, subject, customer_name, customer_email",
		"insert into support_realtime_outbox",
		"old.status in ('resolved', 'spam')",
		"state.relevance_mask & ~4",
	} {
		if !strings.Contains(sql, clause) {
			t.Errorf("projection trigger migration missing %q", clause)
		}
	}
}

func TestSupportInboxStateBackfillMigrationContract(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	var migration *Migration
	for i := range migrations {
		if migrations[i].Version == "202609020006" {
			migration = &migrations[i]
			break
		}
	}
	if migration == nil {
		t.Fatal("expected support inbox state backfill migration")
	}
	sql := strings.ToLower(strings.Join(strings.Fields(migration.SQL), " "))
	for _, clause := range []string{
		"create or replace function support_inbox_backfill_batch",
		"for update of candidate skip locked",
		"insert into support_conversation_user_states",
		"insert into support_inbox_conversation_projection_states",
		"on conflict (conversation_id, user_id) do nothing",
		"notification.latest_event_category = 'support_replies'",
		"perform support_refresh_core_counters(conversation.id)",
	} {
		if !strings.Contains(sql, clause) {
			t.Errorf("backfill migration missing %q", clause)
		}
	}
}

func TestSupportInboxCoreCountersMigrationContract(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	var migration *Migration
	for i := range migrations {
		if migrations[i].Version == "202609020007" {
			migration = &migrations[i]
			break
		}
	}
	if migration == nil {
		t.Fatal("expected support inbox core counter migration")
	}
	sql := strings.ToLower(strings.Join(strings.Fields(migration.SQL), " "))
	for _, clause := range []string{
		"create or replace function support_apply_core_counter_contribution",
		"create or replace function support_refresh_core_counters",
		"create trigger support_user_state_core_counters_inserted",
		"create trigger support_user_state_core_counters_updated",
		"create trigger support_conversation_core_counters_updated",
		"before delete on support_conversations",
		"on conflict (conversation_id, audience_type, audience_id, bucket_id)",
	} {
		if !strings.Contains(sql, clause) {
			t.Errorf("core counter migration missing %q", clause)
		}
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

func TestDockChatVisibilityMigrationPrerequisiteContract(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}

	var prerequisite *Migration
	var backfill *Migration
	for i := range migrations {
		switch migrations[i].Version {
		case "20260814000050000000":
			prerequisite = &migrations[i]
		case "202608140001":
			backfill = &migrations[i]
		}
	}
	if prerequisite == nil {
		t.Fatal("expected dock chat support conversation column migration 20260814000050000000 to be registered")
	}
	if backfill == nil {
		t.Fatal("expected dock chat visibility migration 202608140001 to be registered")
	}
	if prerequisite.Version >= backfill.Version {
		t.Fatalf("support conversation column migration %s must sort before visibility migration %s", prerequisite.Version, backfill.Version)
	}

	sql := strings.ToLower(strings.Join(strings.Fields(prerequisite.SQL), " "))
	for _, clause := range []string{
		"alter table if exists dock_chats",
		"add column if not exists support_conversation_id uuid",
	} {
		if !strings.Contains(sql, clause) {
			t.Errorf("prerequisite migration SQL missing contract clause %q", clause)
		}
	}
}

func TestCRMMeetingMigrationPrerequisiteContract(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}

	var prerequisite *Migration
	var calendarMigration *Migration
	for i := range migrations {
		switch migrations[i].Version {
		case "20260815000050000000":
			prerequisite = &migrations[i]
		case "202608150001":
			calendarMigration = &migrations[i]
		}
	}
	if prerequisite == nil {
		t.Fatal("expected CRM meeting prerequisite migration 20260815000050000000 to be registered")
	}
	if calendarMigration == nil {
		t.Fatal("expected calendar meeting capture migration 202608150001 to be registered")
	}
	if prerequisite.Version >= calendarMigration.Version {
		t.Fatalf("CRM meeting prerequisite migration %s must sort before calendar migration %s", prerequisite.Version, calendarMigration.Version)
	}

	sql := strings.ToLower(strings.Join(strings.Fields(prerequisite.SQL), " "))
	for _, clause := range []string{
		"create table if not exists crm_meetings",
		"calendar_event_id uuid",
		"activity_id uuid",
		"recording_object_key text",
		"create table if not exists crm_meeting_intelligence",
		"meeting_id uuid not null",
	} {
		if !strings.Contains(sql, clause) {
			t.Errorf("prerequisite migration SQL missing contract clause %q", clause)
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

func TestCRMSignalInterpretationCoverageMigrationIsGuarded(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	var spine, customer *Migration
	for i := range migrations {
		switch migrations[i].Version {
		case "202608280003":
			spine = &migrations[i]
		case "202608280004":
			customer = &migrations[i]
		}
	}
	if spine == nil || customer == nil {
		t.Fatalf("motion migrations missing: spine=%v customer=%v", spine != nil, customer != nil)
	}
	spineSQL := strings.ToLower(strings.Join(strings.Fields(spine.SQL), " "))
	for _, clause := range []string{
		"join mappings on mappings.rule_key = cfg.rule_key",
		"where cfg.enabled = true",
		"conversation_signal_extraction",
		"external_provider_evidence",
	} {
		if !strings.Contains(spineSQL, clause) {
			t.Fatalf("motion spine is missing mapping coverage clause %q", clause)
		}
	}
	customerSQL := strings.ToLower(strings.Join(strings.Fields(customer.SQL), " "))
	for _, clause := range []string{
		"enabled crm signal rules lack interpretation mappings",
		"interpretation.rule_version = cfg.version",
		"interpretation.enabled = true",
	} {
		if !strings.Contains(customerSQL, clause) {
			t.Fatalf("customer foundation is missing mapping assertion %q", clause)
		}
	}
}
