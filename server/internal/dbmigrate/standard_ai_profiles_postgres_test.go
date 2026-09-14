//go:build integration

package dbmigrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestStandardAIProfileResetPostgres(t *testing.T) {
	testStandardAIProfileResetPostgres(t, nil)
}

func testStandardAIProfileResetPostgres(t *testing.T, afterReset func(context.Context, *sql.DB, string)) {
	t.Helper()
	dsn := os.Getenv("AI_PROFILES_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated PostgreSQL fixture required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	schema := fmt.Sprintf("standard_ai_%d", time.Now().UnixNano())
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatal(err)
	}
	const workspace = "00000000-0000-0000-0000-000000000001"
	fixture := `CREATE TABLE users(id uuid PRIMARY KEY);
 CREATE TABLE workspaces(id uuid PRIMARY KEY);
 INSERT INTO workspaces VALUES ('00000000-0000-0000-0000-000000000001');
 CREATE TABLE agents(id uuid PRIMARY KEY,workspace_id uuid,name text,is_system boolean,preset_key text,source_preset_key text,model_tier text,provider text,model text,execution_config jsonb);
 CREATE TABLE agent_versions(id uuid PRIMARY KEY,agent_id uuid,workspace_id uuid,model_tier text,provider text,model text,execution_config jsonb);
 CREATE TABLE workspace_agent_preset_versions(id uuid PRIMARY KEY,workspace_id uuid,family_key text,model_tier text,provider text,model text,execution_config jsonb);
 CREATE TABLE agent_runs(id uuid PRIMARY KEY,input jsonb);
 INSERT INTO agent_runs VALUES ('00000000-0000-0000-0000-000000000099','{"model_name":"accepted-old-model","ai_selection":{"profile_id":"old-profile"}}');`
	if _, err := db.ExecContext(ctx, fixture); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"202609120002_personal_ai_connections.sql", "202609140002_shared_ai_connections.sql", "202609140003_ai_profiles.sql", "202609140004_agent_ai_profiles.sql", "202609140005_ai_connection_funding.sql", "202609140009_agent_ai_profile_liveness.sql", "202609140010_ai_profile_bootstrap_completion.sql"} {
		data, err := migrationFiles.ReadFile("sql/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, string(data)); err != nil {
			t.Fatalf("fixture %s: %v", name, err)
		}
	}
	for i, selection := range []struct{ preset, source, want string }{{"epic_planner", "", "small"}, {"task_planner", "", "large"}, {"", "code_builder", "large"}, {"", "", "small"}, {"ask_agent", "", "medium"}} {
		id := fmt.Sprintf("00000000-0000-0000-0000-%012d", i+10)
		if _, err := db.ExecContext(ctx, `INSERT INTO agents(id,workspace_id,preset_key,source_preset_key,provider,model,model_tier,execution_config) VALUES($1,$2,$3,$4,'anthropic','old-model','flagship','{"reasoning_effort":"high","native_context":{"enabled":true},"max_tool_steps":88}')`, id, workspace, selection.preset, selection.source); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO agent_versions(id,agent_id,workspace_id,provider,model,execution_config) VALUES($1,$1,$2,'anthropic','historical-model','{"max_tool_steps":77}')`, id, workspace); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO workspace_agent_preset_versions VALUES('00000000-0000-0000-0000-000000000080',$1,'task_planner','small','anthropic','copied-model','{"max_tool_steps":66}')`, workspace); err != nil {
		t.Fatal(err)
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = ensureSchemaMigrationsTable(ctx, conn)
	closeErr := conn.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("ledger: %v %v", err, closeErr)
	}
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range migrations {
		if m.Version != "202609140011" {
			if _, err := db.ExecContext(ctx, "INSERT INTO schema_migrations(version,name,checksum) VALUES($1,$2,$3)", m.Version, m.Name, m.Checksum); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := Up(ctx, db); err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"small", "large", "large", "small", "medium"} {
		id := fmt.Sprintf("00000000-0000-0000-0000-%012d", i+10)
		var tier, profile string
		var steps int
		var native bool
		if err := db.QueryRowContext(ctx, `SELECT model_tier,ai_profile_id, (execution_config->>'max_tool_steps')::int,(execution_config->'native_context'->>'enabled')::bool FROM agents WHERE id=$1`, id).Scan(&tier, &profile, &steps, &native); err != nil {
			t.Fatal(err)
		}
		if tier != want || profile != model.StandardAIProfileID(workspace, want) || steps != 88 || !native {
			t.Fatalf("agent %s reset incorrectly", id)
		}
		if err := db.QueryRowContext(ctx, `SELECT model_tier,ai_profile_id,(execution_config->>'max_tool_steps')::int FROM agent_versions WHERE id=$1`, id).Scan(&tier, &profile, &steps); err != nil {
			t.Fatal(err)
		}
		if tier != want || profile != model.StandardAIProfileID(workspace, want) || steps != 77 {
			t.Fatal("saved version did not receive default")
		}
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM ai_profiles").Scan(&count); err != nil || count != 4 {
		t.Fatalf("standard profiles: %d %v", count, err)
	}
	var connection string
	if err := db.QueryRowContext(ctx, "SELECT id FROM ai_connections").Scan(&connection); err != nil {
		t.Fatal(err)
	}
	if connection != model.StandardAIConnectionID(workspace, "openrouter") {
		t.Fatal("SQL and Go connection identities differ")
	}
	var copiedTier string
	if err := db.QueryRowContext(ctx, "SELECT model_tier FROM workspace_agent_preset_versions").Scan(&copiedTier); err != nil || copiedTier != "large" {
		t.Fatal("preset copy did not reset to family default")
	}
	var input string
	if err := db.QueryRowContext(ctx, "SELECT input->>'model_name' FROM agent_runs").Scan(&input); err != nil || input != "accepted-old-model" {
		t.Fatal("accepted run changed")
	}
	if afterReset != nil {
		afterReset(ctx, db, workspace)
	}
	if _, err := db.ExecContext(ctx, "UPDATE agents SET ai_profile_id=NULL; UPDATE ai_workspace_settings SET default_profile_id=NULL"); err != nil {
		t.Fatal(err)
	}
	if err := Up(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM agents WHERE ai_profile_id IS NOT NULL").Scan(&count); err != nil || count != 0 {
		t.Fatal("repeat migration reset later choices")
	}
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.columns WHERE table_schema=$1 AND table_name='ai_workspace_settings' AND column_name='profiles_bootstrapped_at'", schema).Scan(&count); err != nil || count != 0 {
		t.Fatal("bootstrap marker retained")
	}
}
