//go:build integration

package dbmigrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestLegacyNativeVersionUpgradePostgres(t *testing.T) {
	dsn := os.Getenv("AI_PROFILES_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("AI_PROFILES_TEST_DATABASE_URL is required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	schema := fmt.Sprintf("merge_versions_%d", time.Now().UnixNano())
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema+"; SET search_path TO "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = ensureSchemaMigrationsTable(ctx, conn)
	if closeErr := conn.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	// Only the four incoming CRM migrations are pending. Native tables are
	// deliberately absent: replaying already-applied native SQL must fail.
	if _, err := db.ExecContext(ctx, `
CREATE TABLE crm_pipelines(id uuid, workspace_id uuid, is_default bool, name text, default_commercial_motion text);
CREATE TABLE crm_pipeline_stages(id uuid, pipeline_id uuid, name text, position bigint, probability bigint, stage_type text);
CREATE TABLE crm_deals(id uuid, stage_id uuid);
CREATE TABLE crm_suggestions(id uuid);
CREATE TABLE automation_rules(id uuid);
CREATE TABLE crm_email_accounts(id uuid, provider text, email_address text);
CREATE TABLE crm_email_messages(id uuid, email_account_id uuid, message_external_id text, workspace_id uuid, sent_at timestamptz, direction text, from_address text);
`); err != nil {
		t.Fatal(err)
	}
	core, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	oldVersions := map[string]bool{}
	for _, legacy := range legacyNativeVersions {
		oldVersions[legacy] = true
	}
	stamp := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	for _, migration := range core {
		if oldVersions[migration.Version] {
			continue
		}
		version := migration.Version
		if legacy, ok := legacyNativeVersions[version]; ok {
			version = legacy
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO schema_migrations(version,name,checksum,applied_at) VALUES ($1,$2,$3,$4)", version, migration.Name, migration.Checksum, stamp); err != nil {
			t.Fatal(err)
		}
	}
	issues, err := Validate(ctx, db)
	if err != nil || len(issues) != 4 {
		t.Fatalf("pre-upgrade validation: %v %v", issues, err)
	}
	for _, issue := range issues {
		if issue.Kind != "pending" || !oldVersions[issue.Version] {
			t.Fatalf("wrong pending migration: %+v", issue)
		}
	}
	pending, err := Pending(ctx, db)
	if err != nil || len(pending) != 4 {
		t.Fatalf("pending=%v err=%v", pending, err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations WHERE version IN ('20260912000101','20260912000201','20260914000101','20260914000201')").Scan(&count); err != nil || count != 0 {
		t.Fatalf("read-only commands modified ledger: %d %v", count, err)
	}
	for range 2 {
		if err := Up(ctx, db); err != nil {
			t.Fatal(err)
		}
	}
	issues, err = Validate(ctx, db)
	if err != nil || len(issues) != 0 {
		t.Fatalf("post-upgrade validation: %v %v", issues, err)
	}
	for _, migration := range core {
		if _, ok := legacyNativeVersions[migration.Version]; !ok {
			continue
		}
		var name, checksum string
		var appliedAt time.Time
		if err := db.QueryRowContext(ctx, "SELECT name,checksum,applied_at FROM schema_migrations WHERE version=$1", migration.Version).Scan(&name, &checksum, &appliedAt); err != nil {
			t.Fatal(err)
		}
		if name != migration.Name || checksum != migration.Checksum || !appliedAt.Equal(stamp) {
			t.Fatal("native migration history changed")
		}
	}
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.columns WHERE table_schema=$1 AND table_name='crm_deals' AND column_name='revenue_type'", schema).Scan(&count); err != nil || count != 1 {
		t.Fatalf("CRM SQL was not applied: %d %v", count, err)
	}
	// Now represent Develop's history: CRM is applied at the original versions,
	// while the four native migrations are pending. The native SQL must run, and
	// the CRM ledger rows must retain their timestamps and checksums.
	if _, err := db.ExecContext(ctx, `
CREATE TEMP TABLE crm_ledger_before AS SELECT * FROM schema_migrations WHERE version IN ('202609120001','202609120002','202609140001','202609140002');
DELETE FROM schema_migrations WHERE version IN ('20260912000101','20260912000201','20260914000101','20260914000201');
CREATE TABLE workspaces(id uuid PRIMARY KEY);
CREATE TABLE users(id uuid PRIMARY KEY);
CREATE TABLE agents(id uuid PRIMARY KEY, is_system bool, active_version_id uuid, runtime_kind text, preset_key text, updated_at timestamptz);
CREATE TABLE agent_versions(id uuid PRIMARY KEY, agent_id uuid, version_key text, label text, runtime_kind text, deleted_at timestamptz, created_at timestamptz, updated_at timestamptz);
CREATE TABLE agent_runs(id text PRIMARY KEY, runtime_kind text, status text);
CREATE TABLE workspace_agent_preset_versions(id text PRIMARY KEY, runtime_kind text, deleted_at timestamptz, updated_at timestamptz);
`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := Up(ctx, db); err != nil {
			t.Fatalf("develop upgrade: %v", err)
		}
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM crm_ledger_before b JOIN schema_migrations m USING(version) WHERE b.name=m.name AND b.checksum=m.checksum AND b.applied_at=m.applied_at`).Scan(&count); err != nil || count != 4 {
		t.Fatalf("develop ledger was modified: %d %v", count, err)
	}
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.columns WHERE table_schema=$1 AND table_name='ai_connections' AND column_name='scope'", schema).Scan(&count); err != nil || count != 1 {
		t.Fatalf("native SQL was not applied: %d %v", count, err)
	}
	// Neither an unknown historical checksum nor an occupied destination may
	// be mistaken for a known legacy native migration.
	if _, err := db.ExecContext(ctx, "UPDATE schema_migrations SET checksum='corrupt' WHERE version='202609120001'"); err != nil {
		t.Fatal(err)
	}
	if err := Up(ctx, db); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("corrupt ledger accepted: %v", err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE schema_migrations SET checksum=(SELECT checksum FROM schema_migrations WHERE version='20260912000101') WHERE version='202609120001'"); err != nil {
		t.Fatal(err)
	}
	if err := Up(ctx, db); err == nil || !strings.Contains(err.Error(), "occupied version") {
		t.Fatalf("occupied destination accepted: %v", err)
	}
}
