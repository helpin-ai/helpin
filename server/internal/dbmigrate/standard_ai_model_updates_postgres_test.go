//go:build integration

package dbmigrate

import (
	"context"
	"database/sql"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestStandardAIModelUpdatesPostgres(t *testing.T) {
	for _, customized := range []bool{false, true} {
		name := "shipped defaults"
		if customized {
			name = "customized standard profile"
		}
		t.Run(name, func(t *testing.T) {
			testStandardAIProfileResetPostgres(t, func(ctx context.Context, db *sql.DB, workspace string) {
				small := model.StandardAIProfileID(workspace, "small")
				medium := model.StandardAIProfileID(workspace, "medium")
				large := model.StandardAIProfileID(workspace, "large")
				exec := func(query string, args ...any) {
					t.Helper()
					if _, err := db.ExecContext(ctx, query, args...); err != nil {
						t.Fatal(err)
					}
				}
				// An explicitly selected different profile is not reset just because this is Ask Agent.
				exec(`INSERT INTO agents(id,workspace_id,preset_key,ai_profile_id,model_tier,provider,model,execution_config) VALUES('00000000-0000-0000-0000-000000000090',$1,'ask_agent',$2,'large','openrouter','openai/gpt-5.6-terra','{"max_tool_steps":55}')`, workspace, large)
				exec(`INSERT INTO workspace_agent_preset_versions VALUES('00000000-0000-0000-0000-000000000081',$1,'ask_agent','medium','openrouter','google/gemini-3.7-flash','{"max_tool_steps":44}')`, workspace)
				if customized {
					exec(`UPDATE ai_profiles SET "primary"=jsonb_set("primary",'{model,model}','"my-custom-model"') WHERE id=$1`, small)
				}
				exec(`DELETE FROM schema_migrations WHERE version='202609140012'`)
				if err := Up(ctx, db); err != nil {
					t.Fatal(err)
				}
				var smallModel, mediumModel string
				if err := db.QueryRowContext(ctx, `SELECT "primary"->'model'->>'model' FROM ai_profiles WHERE id=$1`, small).Scan(&smallModel); err != nil {
					t.Fatal(err)
				}
				if err := db.QueryRowContext(ctx, `SELECT "primary"->'model'->>'model' FROM ai_profiles WHERE id=$1`, medium).Scan(&mediumModel); err != nil {
					t.Fatal(err)
				}
				wantSmall := "deepseek/deepseek-v4.1-flash:nitro"
				wantAsk := small
				if customized {
					wantSmall = "my-custom-model"
					wantAsk = medium
				}
				if smallModel != wantSmall || mediumModel != "google/gemini-3.8-flash" {
					t.Fatalf("models: small=%s medium=%s", smallModel, mediumModel)
				}
				for _, table := range []string{"agents", "agent_versions"} {
					var profile string
					var steps int
					if err := db.QueryRowContext(ctx, `SELECT ai_profile_id, (execution_config->>'max_tool_steps')::int FROM `+table+` WHERE id='00000000-0000-0000-0000-000000000014'`).Scan(&profile, &steps); err != nil {
						t.Fatal(err)
					}
					if profile != wantAsk {
						t.Fatalf("%s Ask profile=%s want %s", table, profile, wantAsk)
					}
					wantSteps := 88
					if table == "agent_versions" {
						wantSteps = 77
					}
					if steps != wantSteps {
						t.Fatalf("%s lost execution settings", table)
					}
				}
				var explicit string
				if err := db.QueryRowContext(ctx, `SELECT ai_profile_id FROM agents WHERE id='00000000-0000-0000-0000-000000000090'`).Scan(&explicit); err != nil || explicit != large {
					t.Fatalf("explicit profile changed: %s %v", explicit, err)
				}
				var copiedTier, copiedModel string
				var quantization string
				if err := db.QueryRowContext(ctx, `SELECT model_tier,model,execution_config->'openrouter'->'provider'->'quantizations'->>0 FROM workspace_agent_preset_versions WHERE id='00000000-0000-0000-0000-000000000081'`).Scan(&copiedTier, &copiedModel, &quantization); err != nil {
					t.Fatal(err)
				}
				if copiedTier != "small" || copiedModel != "deepseek/deepseek-v4.1-flash:nitro" || quantization != "fp8" {
					t.Fatal("Ask preset copy not updated")
				}
				var accepted string
				if err := db.QueryRowContext(ctx, `SELECT input->>'model_name' FROM agent_runs`).Scan(&accepted); err != nil || accepted != "accepted-old-model" {
					t.Fatal("accepted run changed")
				}
				if err := Up(ctx, db); err != nil {
					t.Fatal(err)
				}
			})
		})
	}
}
