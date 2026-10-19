//go:build integration

package dbmigrate

import (
	"context"
	"database/sql"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestDeepSeekFlash1017Postgres(t *testing.T) {
	for _, customization := range []string{"", "model", "controls", "revision", "fallback"} {
		t.Run("customization="+customization, func(t *testing.T) {
			testStandardAIProfileResetPostgres(t, func(ctx context.Context, db *sql.DB, workspace string) {
				exec := func(query string, args ...any) {
					t.Helper()
					if _, err := db.ExecContext(ctx, query, args...); err != nil {
						t.Fatal(err)
					}
				}
				exec("DELETE FROM schema_migrations WHERE version='202609140012'")
				if err := Up(ctx, db); err != nil {
					t.Fatal(err)
				}
				small := model.StandardAIProfileID(workspace, "small")
				switch customization {
				case "model":
					exec(`UPDATE ai_profiles SET "primary"=jsonb_set("primary",'{model,model}','"custom-model"') WHERE id=$1`, small)
				case "controls":
					exec(`UPDATE ai_profiles SET "primary"=jsonb_set("primary",'{model,controls}','{}') WHERE id=$1`, small)
				case "revision":
					exec(`UPDATE ai_profiles SET revision=3 WHERE id=$1`, small)
				case "fallback":
					exec(`UPDATE ai_profiles SET fallback="primary" WHERE id=$1`, small)
				}
				snapshot := func() string {
					t.Helper()
					var value string
					err := db.QueryRowContext(ctx, `SELECT jsonb_build_object(
      'profiles',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM ai_profiles p),
      'agents',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM agents a),
      'versions',(SELECT jsonb_agg(to_jsonb(v) ORDER BY id) FROM agent_versions v),
      'runs',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM agent_runs r),
      'presets',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM workspace_agent_preset_versions p))::text`).Scan(&value)
					if err != nil {
						t.Fatal(err)
					}
					return value
				}
				var copiedBefore string
				if err := db.QueryRowContext(ctx, "SELECT model FROM workspace_agent_preset_versions").Scan(&copiedBefore); err != nil {
					t.Fatal(err)
				}
				before := snapshot()
				exec("DELETE FROM schema_migrations WHERE version='202610190002'")
				if err := Up(ctx, db); err != nil {
					t.Fatal(err)
				}
				after := snapshot()
				if customization != "" {
					if before != after {
						t.Fatal("custom selection was changed")
					}
				} else {
					var modelName string
					var revision int
					if err := db.QueryRowContext(ctx, `SELECT "primary"->'model'->>'model',revision FROM ai_profiles WHERE id=$1`, small).Scan(&modelName, &revision); err != nil {
						t.Fatal(err)
					}
					if modelName != "deepseek/deepseek-v4.1-flash-1017:nitro" || revision != 3 {
						t.Fatalf("model=%s revision=%d", modelName, revision)
					}
					for _, table := range []string{"agents", "agent_versions"} {
						var name string
						var steps int
						if err := db.QueryRowContext(ctx, "SELECT model,(execution_config->>'max_tool_steps')::int FROM "+table+" WHERE id='00000000-0000-0000-0000-000000000010'").Scan(&name, &steps); err != nil {
							t.Fatal(err)
						}
						wantSteps := 88
						if table == "agent_versions" {
							wantSteps = 77
						}
						if name != modelName || steps != wantSteps {
							t.Fatalf("%s: model=%s steps=%d", table, name, steps)
						}
					}
					var accepted, copied string
					if err := db.QueryRowContext(ctx, "SELECT input->>'model_name' FROM agent_runs").Scan(&accepted); err != nil {
						t.Fatal(err)
					}
					if err := db.QueryRowContext(ctx, "SELECT model FROM workspace_agent_preset_versions").Scan(&copied); err != nil {
						t.Fatal(err)
					}
					if accepted != "accepted-old-model" || copied != copiedBefore {
						t.Fatal("run or custom preset changed")
					}
				}
				// Execute again, bypassing the ledger, to verify the SQL itself is idempotent.
				data, err := migrationFiles.ReadFile("sql/202610190002_deepseek_flash_1017.sql")
				if err != nil {
					t.Fatal(err)
				}
				exec(string(data))
				if snapshot() != after {
					t.Fatal("migration is not idempotent")
				}
			})
		})
	}
}
