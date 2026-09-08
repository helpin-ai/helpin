package crmsituationtest

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gorm.io/gorm"
)

// PlaybookExecutionStorage installs execution gates in an isolated fixture only.
func PlaybookExecutionStorage(t testing.TB, db *gorm.DB) {
	t.Helper()
	_, path, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate execution migration")
	}
	source, err := os.ReadFile(filepath.Join(filepath.Dir(path), "../dbmigrate/sql/202609080001_crm_playbook_execution.sql"))
	if err != nil {
		t.Fatal(err)
	}
	statement := string(source)
	if db.Dialector.Name() == "sqlite" {
		statement = strings.ReplaceAll(statement, "timestamptz", "datetime")
	}
	Exec(t, db, statement)
	Exec(t, db, statement)
	if db.Dialector.Name() == "postgres" {
		won, err := os.ReadFile(filepath.Join(filepath.Dir(path), "../dbmigrate/sql/202609080003_crm_won_deal_entry.sql"))
		if err != nil {
			t.Fatal(err)
		}
		Exec(t, db, string(won))
		Exec(t, db, string(won))
	}
	statement = `CREATE TABLE agent_runs (
		id uuid PRIMARY KEY, workspace_id uuid NOT NULL, agent_id uuid NOT NULL,
		task_id uuid, conversation_id uuid, target_type text NOT NULL, target_id uuid NOT NULL,
		runtime_kind text NOT NULL, model_tier text, invocation_mode text, parent_run_id uuid,
		dock_chat_id uuid, handoff_state text, approval_state text, pause_reason text, triggered_by_user_id uuid,
		status text NOT NULL, workflow_id text, workflow_run_id text, external_runtime text, external_runtime_id text,
		task_queue text, runner_pool text, agent_version_id uuid, repository_id uuid, repo_full_name text,
		base_branch text, working_branch text, delivery_target_id uuid, execution_stage text, last_heartbeat_at datetime,
		input jsonb NOT NULL, output_summary jsonb, cached_input_tokens integer, input_tokens integer,
		output_tokens integer, tokens_used integer, error_message text, started_at datetime, completed_at datetime,
		created_at datetime, updated_at datetime)`
	if db.Dialector.Name() == "postgres" {
		statement = strings.ReplaceAll(statement, "datetime", "timestamptz")
	}
	Exec(t, db, statement)
}
