package crmsituationtest

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gorm.io/gorm"
)

// PlaybookConnectionStorage installs only isolated test schema and returns saved settings IDs.
func PlaybookConnectionStorage(t testing.TB, db *gorm.DB) (string, string) {
	t.Helper()
	_, current, _, _ := runtime.Caller(0)
	source, err := os.ReadFile(filepath.Join(filepath.Dir(current), "../dbmigrate/sql/202609070002_crm_playbook_connections.sql"))
	if err != nil {
		t.Fatal(err)
	}
	statement := string(source)
	if db.Dialector.Name() == "sqlite" {
		statement = strings.ReplaceAll(statement, "timestamptz", "datetime")
	}
	Exec(t, db, statement)
	Exec(t, db, statement)
	for _, statement := range []string{
		`CREATE TABLE agents (id uuid PRIMARY KEY, workspace_id uuid, is_system boolean, name text, preset_key text, preset_version_key text,
		status text, runtime_kind text, system_prompt text, model_tier text, provider text, model text, execution_config jsonb,
		monthly_token_budget integer, tokens_used_this_month integer, max_concurrent_runs integer, team_id uuid,
		allowed_tools jsonb, allowed_targets jsonb, allowed_commands jsonb, skills jsonb, approval_mode text, default_invocation_mode text, created_at timestamp, updated_at timestamp)`,
		`CREATE TABLE agent_team_access (agent_id uuid, team_id uuid, created_at timestamp, PRIMARY KEY (agent_id, team_id))`,
		`CREATE TABLE automation_rules (id uuid PRIMARY KEY, workspace_id uuid, name text, enabled boolean, team_id uuid,
		trigger_type text, trigger_config jsonb, action_type text, action_config jsonb, created_by uuid, created_at timestamp, updated_at timestamp)`,
	} {
		Exec(t, db, statement)
	}
	flowID, agentID := "a0000000-0000-4000-8000-000000000001", "b0000000-0000-4000-8000-000000000001"
	Exec(t, db, `INSERT INTO agents (id,workspace_id,is_system,name,preset_key,preset_version_key,status,runtime_kind,system_prompt,
		execution_config,monthly_token_budget,tokens_used_this_month,max_concurrent_runs,allowed_tools,allowed_targets,allowed_commands,skills,approval_mode,default_invocation_mode)
		VALUES (?,?,true,'Beacon','crm_operator','v1','idle','native_sdk','Workspace-reviewed Beacon instructions','{}',1000,0,1,?,?,?,'[]','always','autonomous')`, agentID, Workspace,
		[]byte(`["get_crm_deal"]`), []byte(`["crm_deal","crm_contact","crm_company"]`), []byte(`[]`))
	Exec(t, db, `INSERT INTO automation_rules (id,workspace_id,name,enabled,trigger_type,trigger_config,action_type,action_config)
		VALUES (?,?,'Customer follow-through',false,'cron',?,'start_agent_run',?)`,
		flowID, Workspace, []byte(`{"preset":"daily"}`), []byte(`{"agent_id":"`+agentID+`","target_type":"crm_company","target_id":"`+Company+`"}`))
	return flowID, agentID
}
