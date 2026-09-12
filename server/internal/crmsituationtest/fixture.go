// Package crmsituationtest supplies isolated database fixtures for customer-work tests.
package crmsituationtest

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	// Workspace is the authorized fixture workspace.
	Workspace = "10000000-0000-4000-8000-000000000001"
	// ForeignWorkspace is an unrelated tenant.
	ForeignWorkspace = "10000000-0000-4000-8000-000000000002"
	// SalesUser is the authenticated user behind the sales workspace membership.
	SalesUser = "90000000-0000-4000-8000-000000000001"
	// ForeignUser owns the unrelated workspace.
	ForeignUser = "90000000-0000-4000-8000-000000000002"
	// Sales owns the account and deal.
	Sales = "20000000-0000-4000-8000-000000000001"
	// Success is the account's customer-success owner.
	Success = "20000000-0000-4000-8000-000000000002"
	// Inactive is a former workspace member.
	Inactive = "20000000-0000-4000-8000-000000000003"
	// ForeignMember belongs only to the unrelated tenant.
	ForeignMember = "20000000-0000-4000-8000-000000000004"
	// Company is a permitted customer record.
	Company = "30000000-0000-4000-8000-000000000001"
	// ForeignCompany belongs to the unrelated tenant.
	ForeignCompany = "30000000-0000-4000-8000-000000000002"
	// Deal is a permitted opportunity.
	Deal = "40000000-0000-4000-8000-000000000001"
	// Contact is a permitted stakeholder.
	Contact = "50000000-0000-4000-8000-000000000001"
	// Signal is immutable evidence in the permitted workspace.
	Signal = "60000000-0000-4000-8000-000000000001"
	// ForeignSignal is evidence in another workspace.
	ForeignSignal = "60000000-0000-4000-8000-000000000002"
	// Suggestion is an existing pending CRM action.
	Suggestion = "70000000-0000-4000-8000-000000000001"
)

// Open creates an isolated fixture and applies the new migration twice. SQLite
// is the default; CRM_SITUATION_TEST_POSTGRES_DSN opts into a disposable Postgres DB.
func Open(t testing.TB) *gorm.DB {
	t.Helper()
	if dsn := os.Getenv("CRM_SITUATION_TEST_POSTGRES_DSN"); dsn != "" {
		return openPostgres(t, dsn)
	}
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared&_foreign_keys=on"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get fixture connection: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close fixture: %v", err)
		}
	})
	initialize(t, db)
	return db
}

func initialize(t testing.TB, db *gorm.DB) {
	t.Helper()
	statements := []string{
		`CREATE TABLE workspaces (id uuid PRIMARY KEY, owner_id uuid NOT NULL)`,
		`CREATE TABLE workspace_members (id uuid PRIMARY KEY, workspace_id uuid, user_id uuid, display_name text, role text, status text, created_at datetime DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE team_workspace_memberships (id uuid PRIMARY KEY, team_id uuid, workspace_member_id uuid, role text)`,
		`CREATE TABLE crm_companies (id uuid PRIMARY KEY, workspace_id uuid, name text, domain text, owner_member_id uuid, customer_success_owner_member_id uuid)`,
		`CREATE TABLE crm_contacts (id uuid PRIMARY KEY, workspace_id uuid, first_name text, last_name text)`,
		`CREATE TABLE crm_deals (revenue_type TEXT NOT NULL DEFAULT 'one_time', id uuid PRIMARY KEY, workspace_id uuid, name text, display_id text, amount real, probability integer, owner_member_id uuid,
			pipeline_id uuid, stage_id uuid, currency text, close_date date, commercial_motion text, custom_properties jsonb, created_at datetime, updated_at datetime)`,
		`CREATE TABLE crm_signals (id uuid PRIMARY KEY, workspace_id uuid, reviewed_at datetime, commercial_motion text,
			company_id uuid, contact_id uuid, deal_id uuid, summary text, dismissed_at datetime, superseded_at datetime, acted_at datetime,
			signal_type text, source_type text, detected_at datetime, rule_key text, rule_version integer, confidence double precision,
			evidence_identity_trust text, metadata jsonb, business_weight_snapshot double precision, half_life_days_snapshot double precision,
			signal_domain text, polarity text, evidence_identity_method text, evidence_fingerprint text,
			source_id uuid, source_thread_id uuid, evidence_excerpt text, detector_kind text, window_started_at datetime, window_ended_at datetime,
			observation_id uuid, interpretation_version integer, interpretation_snapshot jsonb, meaning_fingerprint text,
			recommended_action_key text, recommended_action_label text, replay_calibration_excluded boolean,
			superseded_reason text, direction_changed_by_supersession boolean, dismissed_by_member_id uuid, dismissal_reason text, created_at datetime)`,
		`CREATE TABLE crm_suggestions (id uuid PRIMARY KEY, workspace_id uuid, user_id uuid, suggestion_type text,
			object_type text, object_id uuid, title text, description text, context jsonb, signal_ids text,
			status text, execution_status text, confidence double precision, dismissal_reason text,
			execution_error text, executed_at datetime, created_at datetime, updated_at datetime)`,
	}
	for _, statement := range statements {
		if db.Dialector.Name() == "postgres" {
			statement = strings.ReplaceAll(statement, "datetime", "timestamptz")
			statement = strings.ReplaceAll(statement, "signal_ids text", "signal_ids text[]")
		}
		Exec(t, db, statement)
	}
	Exec(t, db, "INSERT INTO workspaces (id,owner_id) VALUES (?,?), (?,?)", Workspace, SalesUser, ForeignWorkspace, ForeignUser)
	for _, member := range []struct{ id, ws, name, role, status string }{
		{Sales, Workspace, "Sales owner", "member", "active"},
		{Success, Workspace, "Success owner", "member", "active"},
		{Inactive, Workspace, "Former owner", "member", "inactive"},
		{ForeignMember, ForeignWorkspace, "Foreign owner", "member", "active"},
	} {
		userID := uuid.NewString()
		if member.id == Sales {
			userID = SalesUser
		} else if member.id == ForeignMember {
			userID = ForeignUser
		}
		Exec(t, db, "INSERT INTO workspace_members (id,workspace_id,user_id,display_name,role,status) VALUES (?,?,?,?,?,?)",
			member.id, member.ws, userID, member.name, member.role, member.status)
	}
	Exec(t, db, `INSERT INTO crm_companies (id,workspace_id,name,domain,owner_member_id,customer_success_owner_member_id)
		VALUES (?,?,?,?,?,?), (?,?,?,?,?,?)`, Company, Workspace, "Northstar", "northstar.example", Sales, Success,
		ForeignCompany, ForeignWorkspace, "Private customer", "private.example", ForeignMember, ForeignMember)
	Exec(t, db, "INSERT INTO crm_contacts (id,workspace_id,first_name,last_name) VALUES (?,?,?,?)", Contact, Workspace, "Anna", "Lee")
	Exec(t, db, "INSERT INTO crm_deals (id,workspace_id,name,display_id,amount,probability,owner_member_id) VALUES (?,?,?,?,?,?,?)",
		Deal, Workspace, "Annual agreement", "D-1", 10000, 50, Sales)
	Exec(t, db, "INSERT INTO crm_signals (id,workspace_id,commercial_motion) VALUES (?,?,?), (?,?,?)",
		Signal, Workspace, "retention", ForeignSignal, ForeignWorkspace, "retention")
	Exec(t, db, "INSERT INTO crm_suggestions (id,workspace_id,status,execution_status) VALUES (?,?,?,?)", Suggestion, Workspace, "pending", "pending")
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate fixture source")
	}
	source, err := os.ReadFile(filepath.Join(filepath.Dir(current), "../dbmigrate/sql/202609050002_crm_customer_situations.sql"))
	if err != nil {
		t.Fatalf("read situation migration: %v", err)
	}
	sourceMigration, err := os.ReadFile(filepath.Join(filepath.Dir(current), "../dbmigrate/sql/202609060001_crm_situation_sources.sql"))
	if err != nil {
		t.Fatalf("read situation source migration: %v", err)
	}
	lifecycleMigration, err := os.ReadFile(filepath.Join(filepath.Dir(current), "../dbmigrate/sql/202609060002_crm_situation_lifecycle.sql"))
	if err != nil {
		t.Fatalf("read situation lifecycle migration: %v", err)
	}
	migration := string(source)
	if db.Dialector.Name() == "sqlite" {
		// SQLite cannot replace CHECK constraints in place. Build the equivalent
		// final schema; the PostgreSQL run exercises both actual ALTER migrations.
		migration = strings.ReplaceAll(migration, "created_by_member_id uuid NOT NULL", "created_by_member_id uuid")
		migration = strings.ReplaceAll(migration, "creation_fingerprint text NOT NULL,", "creation_fingerprint text NOT NULL, origin_kind text NOT NULL DEFAULT 'manual',")
		migration = strings.ReplaceAll(migration, "'renewal', 'retention'))", "'renewal', 'retention', 'needs_context'))")
		migration = strings.ReplaceAll(migration, "CHECK (company_id IS NOT NULL OR contact_id IS NOT NULL OR deal_id IS NOT NULL)",
			"CHECK ((origin_kind = 'manual' AND created_by_member_id IS NOT NULL AND commercial_motion <> 'needs_context' AND (company_id IS NOT NULL OR contact_id IS NOT NULL OR deal_id IS NOT NULL)) OR (origin_kind = 'signal' AND commercial_motion <> 'needs_context') OR origin_kind = 'suggestion' OR (origin_kind = 'deal_won' AND deal_id IS NOT NULL AND commercial_motion = 'onboarding'))")
		_, sourceTables, ok := strings.Cut(string(sourceMigration), "CREATE TABLE IF NOT EXISTS crm_situation_source_links")
		if !ok {
			t.Fatal("source table migration missing")
		}
		migration += "\nCREATE TABLE IF NOT EXISTS crm_situation_source_links" + sourceTables
		migration = strings.ReplaceAll(migration, "creation_fingerprint text NOT NULL,",
			"creation_fingerprint text NOT NULL, revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0), outcome_basis text CHECK (outcome_basis IN ('human_assessment')), duplicate_of_situation_id uuid,")
		_, historyTable, ok := strings.Cut(string(lifecycleMigration), "CREATE TABLE IF NOT EXISTS crm_situation_changes")
		if !ok {
			t.Fatal("lifecycle history migration missing")
		}
		migration += "\nCREATE TABLE IF NOT EXISTS crm_situation_changes" + historyTable
		migration = strings.ReplaceAll(migration, "actor_kind IN ('member', 'signal', 'suggestion')", "actor_kind IN ('member', 'signal', 'suggestion', 'deal_won')")
		migration = strings.ReplaceAll(migration, "'resume', 'close'))", "'resume', 'close', 'apply_playbook', 'update_milestone'))")
		migration = strings.NewReplacer("DEFAULT gen_random_uuid()", "DEFAULT ''", "timestamptz", "datetime", "now()", "CURRENT_TIMESTAMP").Replace(migration)
	}
	Exec(t, db, migration)
	Exec(t, db, migration)
	if db.Dialector.Name() == "postgres" {
		Exec(t, db, string(sourceMigration))
		Exec(t, db, string(sourceMigration))
		Exec(t, db, string(lifecycleMigration))
		Exec(t, db, string(lifecycleMigration))
	}
	eventMigration, err := os.ReadFile(filepath.Join(filepath.Dir(current), "../dbmigrate/sql/202609060003_automation_scheduled_events.sql"))
	if err != nil {
		t.Fatalf("read scheduled event migration: %v", err)
	}
	eventSQL := string(eventMigration)
	if db.Dialector.Name() == "sqlite" {
		eventSQL = strings.NewReplacer("gen_random_uuid()", "lower(hex(randomblob(16)))", "timestamptz", "datetime",
			"now()", "CURRENT_TIMESTAMP", "id::text", "CAST(id AS text)", "revision::text", "CAST(revision AS text)").Replace(eventSQL)
	}
	Exec(t, db, eventSQL)
	Exec(t, db, eventSQL)
	applyPlaybookMigration(t, db)
}

// Exec fails immediately if a fixture statement cannot be applied.
func Exec(t testing.TB, db *gorm.DB, statement string, args ...any) {
	t.Helper()
	if err := db.Exec(statement, args...).Error; err != nil {
		t.Fatalf("fixture statement: %v", err)
	}
}

// Situation creates an independent open record for one of the customer journeys.
func Situation(motion string) model.CRMSituation {
	id := uuid.NewString()
	return model.CRMSituation{
		ID: id, WorkspaceID: Workspace, CreationKey: id, CreationFingerprint: "fingerprint:" + id,
		Title: fmt.Sprintf("%s customer work", motion), Objective: "Agree the customer's next milestone",
		CommercialMotion: motion, CompanyID: Ptr(Company), OwnerMemberID: Ptr(Sales),
		NextActionOwnerMemberID: Ptr(Sales), Lifecycle: model.CRMSituationOpen,
		Attention: model.CRMSituationNeedsContext, NextStep: "Review the customer's request",
		CreatedByMemberID: Ptr(Sales),
	}
}

// Actor returns an authenticated fixture member with the requested role.
func Actor(role string) *authorization.Actor {
	return &authorization.Actor{WorkspaceID: Workspace, WorkspaceMemberID: Sales, UserID: SalesUser, Role: role, Status: "active"}
}

// Ptr creates optional fixture identities without shared mutable state.
func Ptr(value string) *string { return &value }
