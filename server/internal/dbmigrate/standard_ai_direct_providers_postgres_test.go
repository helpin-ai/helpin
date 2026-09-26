//go:build integration

package dbmigrate

import (
	"context"
	"database/sql"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestStandardAIDirectProvidersPostgres(t *testing.T) {
	for _, scenario := range []string{"shipped", "custom model", "custom controls", "custom connection", "fallback", "already direct"} {
		t.Run(scenario, func(t *testing.T) {
			testStandardAIProfileResetPostgres(t, func(ctx context.Context, db *sql.DB, workspace string) {
				large := model.StandardAIProfileID(workspace, "large")
				flagship := model.StandardAIProfileID(workspace, "flagship")
				exec := func(query string, args ...any) {
					t.Helper()
					if _, err := db.ExecContext(ctx, query, args...); err != nil {
						t.Fatal(err)
					}
				}
				// Keep explicit disconnection and funding unchanged when provisioning
				// the new standard connection rows. Startup owns credential handling.
				exec(`INSERT INTO ai_connections(id,workspace_id,scope,funding,name,provider,status) VALUES($1,$2,'workspace','managed','Standard openai','openai','disconnected')`, model.StandardAIConnectionID(workspace, "openai"), workspace)
				exec(`UPDATE ai_profiles SET name='Renamed Large' WHERE id=$1`, large)
				switch scenario {
				case "custom model":
					exec(`UPDATE ai_profiles SET "primary"=jsonb_set("primary",'{model,model}','"custom-model"') WHERE id=$1`, large)
				case "custom controls":
					exec(`UPDATE ai_profiles SET "primary"=jsonb_set("primary",'{model,controls}','{"reasoning_effort":"high"}') WHERE id=$1`, large)
				case "custom connection":
					exec(`UPDATE ai_profiles SET "primary"=jsonb_set("primary",'{connection_id}','"00000000-0000-0000-0000-000000000088"') WHERE id=$1`, large)
				case "fallback":
					exec(`UPDATE ai_profiles SET fallback="primary" WHERE id=$1`, large)
				case "already direct":
					exec(`UPDATE ai_profiles SET "primary"=jsonb_build_object('connection_id',$2::text,'model',jsonb_build_object('provider','openai','model','gpt-5.6-terra','controls','{}'::jsonb)) WHERE id=$1`, large, model.StandardAIConnectionID(workspace, "openai"))
				}
				var before string
				if err := db.QueryRowContext(ctx, `SELECT "primary"::text FROM ai_profiles WHERE id=$1`, large).Scan(&before); err != nil {
					t.Fatal(err)
				}
				exec(`INSERT INTO workspace_agent_preset_versions VALUES('00000000-0000-0000-0000-000000000081',$1,'code_builder','large','openrouter','openai/gpt-5.6-terra','{"max_tool_steps":44}')`, workspace)
				exec(`INSERT INTO workspace_agent_preset_versions VALUES('00000000-0000-0000-0000-000000000082',$1,'code_builder','large','openrouter','openai/gpt-5.6-terra','{"reasoning_effort":"high"}')`, workspace)
				exec(`DELETE FROM schema_migrations WHERE version='202609140013'`)
				if err := Up(ctx, db); err != nil {
					t.Fatal(err)
				}
				var provider, modelName, connection, name, after string
				var revision int
				if err := db.QueryRowContext(ctx, `SELECT "primary"->'model'->>'provider',"primary"->'model'->>'model',"primary"->>'connection_id',name,revision,"primary"::text FROM ai_profiles WHERE id=$1`, large).Scan(&provider, &modelName, &connection, &name, &revision, &after); err != nil {
					t.Fatal(err)
				}
				if name != "Renamed Large" {
					t.Fatal("profile rename lost")
				}
				if scenario == "shipped" {
					if provider != "openai" || modelName != "gpt-5.6-terra" || connection != model.StandardAIConnectionID(workspace, "openai") || revision != 2 {
						t.Fatal("Large did not switch to the direct OpenAI connection")
					}
				} else if after != before || revision != 1 {
					t.Fatal("customized or already-direct profile changed")
				}
				if err := db.QueryRowContext(ctx, `SELECT "primary"->'model'->>'provider',"primary"->'model'->>'model',"primary"->>'connection_id' FROM ai_profiles WHERE id=$1`, flagship).Scan(&provider, &modelName, &connection); err != nil {
					t.Fatal(err)
				}
				if provider != "anthropic" || modelName != "claude-sonnet-5" || connection != model.StandardAIConnectionID(workspace, "anthropic") {
					t.Fatal("Flagship did not switch to the direct Anthropic connection")
				}
				for _, table := range []string{"agents", "agent_versions"} {
					var steps int
					if err := db.QueryRowContext(ctx, `SELECT provider,model,(execution_config->>'max_tool_steps')::int FROM `+table+` WHERE id='00000000-0000-0000-0000-000000000011'`).Scan(&provider, &modelName, &steps); err != nil {
						t.Fatal(err)
					}
					wantProvider, wantModel := "openrouter", "openai/gpt-5.6-terra"
					if scenario == "shipped" {
						wantProvider, wantModel = "openai", "gpt-5.6-terra"
					}
					wantSteps := 88
					if table == "agent_versions" {
						wantSteps = 77
					}
					if provider != wantProvider || modelName != wantModel || steps != wantSteps {
						t.Fatalf("%s route/settings changed incorrectly", table)
					}
				}
				for _, test := range []struct{ id, provider string }{{"00000000-0000-0000-0000-000000000081", "openai"}, {"00000000-0000-0000-0000-000000000082", "openrouter"}} {
					if err := db.QueryRowContext(ctx, `SELECT provider FROM workspace_agent_preset_versions WHERE id=$1`, test.id).Scan(&provider); err != nil || provider != test.provider {
						t.Fatal("preset version default/custom route changed incorrectly")
					}
				}
				var status, funding string
				if err := db.QueryRowContext(ctx, `SELECT status,funding FROM ai_connections WHERE id=$1`, model.StandardAIConnectionID(workspace, "openai")).Scan(&status, &funding); err != nil || status != "disconnected" || funding != "managed" {
					t.Fatal("existing connection overwritten")
				}
				var accepted string
				if err := db.QueryRowContext(ctx, `SELECT input->>'model_name' FROM agent_runs`).Scan(&accepted); err != nil || accepted != "accepted-old-model" {
					t.Fatal("accepted run changed")
				}
				// Also prove SQL idempotency if an operator repairs/replays the migration.
				exec(`DELETE FROM schema_migrations WHERE version='202609140013'`)
				if err := Up(ctx, db); err != nil {
					t.Fatal(err)
				}
				var replayRevision int
				if err := db.QueryRowContext(ctx, `SELECT revision FROM ai_profiles WHERE id=$1`, large).Scan(&replayRevision); err != nil || replayRevision != revision {
					t.Fatal("replayed migration changed profile again")
				}
			})
		})
	}
}
