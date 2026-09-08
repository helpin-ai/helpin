package crmsituationtest

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EnablePolicy supplies real versioned policy tables for source integration tests.
func EnablePolicy(t testing.TB, db *gorm.DB) {
	t.Helper()
	for _, statement := range []string{
		`CREATE TABLE crm_signal_rule_configs (id uuid PRIMARY KEY, workspace_id uuid, rule_key text, version integer, enabled boolean, shadow_mode boolean, activation_eligible boolean, business_weight double precision, half_life_days double precision)`,
		`CREATE TABLE crm_signal_scoring_configs (id uuid PRIMARY KEY, workspace_id uuid, version integer, enabled boolean, heuristic boolean, parameters jsonb)`,
		`CREATE TABLE crm_signal_routing_policies (id uuid PRIMARY KEY, workspace_id uuid, version integer, enabled boolean, minimum_priority double precision, required_trust text, route_to_owner boolean, channels jsonb)`,
		`CREATE TABLE crm_signal_rollout_settings (id uuid PRIMARY KEY, workspace_id uuid, mode text)`,
	} {
		Exec(t, db, statement)
	}
	Exec(t, db, `INSERT INTO crm_signal_rule_configs (id,workspace_id,rule_key,version,enabled,shadow_mode,activation_eligible,business_weight,half_life_days) VALUES (?,?,?,1,TRUE,FALSE,TRUE,20,30)`, uuid.NewString(), Workspace, "customer_event")
	Exec(t, db, `INSERT INTO crm_signal_scoring_configs (id,workspace_id,version,enabled,heuristic,parameters) VALUES (?,?,1,TRUE,FALSE,'{}')`, uuid.NewString(), Workspace)
	Exec(t, db, `INSERT INTO crm_signal_routing_policies (id,workspace_id,version,enabled,minimum_priority,required_trust,route_to_owner,channels) VALUES (?,?,1,TRUE,5,'verified',TRUE,'["feed"]')`, uuid.NewString(), Workspace)
}

// Schema executes test DDL with only PostgreSQL timestamp spelling adapted.
func Schema(t testing.TB, db *gorm.DB, statement string) {
	t.Helper()
	if db.Dialector.Name() == "postgres" {
		statement = strings.ReplaceAll(statement, "datetime", "timestamptz")
	}
	Exec(t, db, statement)
}
