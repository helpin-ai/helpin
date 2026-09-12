package dbmigrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestNativeCutoverPreservesHistoryAndGatesPausedRuns(t *testing.T) {
	dsn := os.Getenv("HELPIN_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set HELPIN_TEST_POSTGRES_DSN for migration integration")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	schema := fmt.Sprintf("native_cutover_%d", time.Now().UnixNano())
	if _, err = tx.ExecContext(ctx, "CREATE SCHEMA "+schema+"; SET LOCAL search_path TO "+schema); err != nil {
		t.Fatal(err)
	}
	fixture := `
 CREATE TABLE agents(id uuid PRIMARY KEY, is_system bool, active_version_id uuid, runtime_kind text, preset_key text, updated_at timestamptz);
 CREATE TABLE agent_versions(id uuid PRIMARY KEY,agent_id uuid,version_key text,label text,runtime_kind text,deleted_at timestamptz,created_at timestamptz,updated_at timestamptz,system_prompt text);
 CREATE TABLE agent_runs(id text PRIMARY KEY,runtime_kind text,status text,agent_version_id uuid);
 CREATE TABLE workspace_agent_preset_versions(id text PRIMARY KEY,runtime_kind text,deleted_at timestamptz,updated_at timestamptz);
 CREATE TABLE workspace_skills(id text PRIMARY KEY,supported_runtimes jsonb,updated_at timestamptz);
 CREATE TABLE codex_workspace_auths(id text PRIMARY KEY);
 INSERT INTO agents VALUES ('00000000-0000-0000-0000-000000000001',false,'00000000-0000-0000-0000-000000000002','codex','code_builder',now());
 INSERT INTO agent_versions VALUES ('00000000-0000-0000-0000-000000000002','00000000-0000-0000-0000-000000000001','default','Custom','codex',null,now(),now(),'keep my prompt');
 INSERT INTO agent_runs VALUES ('paused','codex','paused','00000000-0000-0000-0000-000000000002');
 INSERT INTO workspace_agent_preset_versions VALUES ('preset','opencode',null,now());
 INSERT INTO workspace_skills VALUES ('skill','["codex"]',now());
 SAVEPOINT before_cutover;`
	if _, err = tx.ExecContext(ctx, fixture); err != nil {
		t.Fatal(err)
	}
	migration, err := migrationFiles.ReadFile("sql/202609120001_native_only_agents.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, string(migration)); err == nil {
		t.Fatal("accepted paused legacy run")
	}
	if _, err = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT before_cutover; UPDATE agent_runs SET status='failed'"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err = tx.ExecContext(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	var kind, oldKind, newKind, prompt string
	var versions int
	if err = tx.QueryRowContext(ctx, `SELECT a.runtime_kind,old.runtime_kind,n.runtime_kind,n.system_prompt FROM agents a JOIN agent_versions n ON n.id=a.active_version_id JOIN agent_runs r ON r.id='paused' JOIN agent_versions old ON old.id=r.agent_version_id`).Scan(&kind, &oldKind, &newKind, &prompt); err != nil {
		t.Fatal(err)
	}
	if kind != "native_sdk" || newKind != "native_sdk" || oldKind != "codex" || prompt != "keep my prompt" {
		t.Fatalf("history/defaults changed incorrectly: %s %s %s %s", kind, oldKind, newKind, prompt)
	}
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM agent_versions").Scan(&versions); err != nil || versions != 2 {
		t.Fatalf("migration duplicated versions: %d %v", versions, err)
	}
	var table sql.NullString
	if err = tx.QueryRowContext(ctx, "SELECT to_regclass('codex_workspace_auths')::text").Scan(&table); err != nil || table.Valid {
		t.Fatalf("obsolete auth table remains: %v %v", table, err)
	}
}
