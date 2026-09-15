//go:build integration

package dbmigrate

import (
	"context"
	"database/sql"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestReuseManagedAIConnectionsPostgres(t *testing.T) {
	for _, scenario := range []string{"managed duplicate", "empty placeholder", "disconnected", "customer key", "renamed"} {
		t.Run(scenario, func(t *testing.T) {
			testStandardAIProfileResetPostgres(t, func(ctx context.Context, db *sql.DB, workspace string) {
				exec := func(query string, args ...any) {
					t.Helper()
					if _, err := db.ExecContext(ctx, query, args...); err != nil {
						t.Fatal(err)
					}
				}
				canonical := map[string]string{
					"openai":     "00000000-0000-0000-0000-000000000100",
					"anthropic":  "00000000-0000-0000-0000-000000000101",
					"openrouter": "00000000-0000-0000-0000-000000000102",
				}
				for provider, id := range canonical {
					exec(`INSERT INTO ai_connections(id,workspace_id,scope,funding,name,provider,status,encrypted_secret) VALUES($1,$2,'workspace','managed',$3,$4,'connected',decode('abcd','hex'))`, id, workspace, provider+" (managed)", provider)
				}
				oldRouter := model.StandardAIConnectionID(workspace, "openrouter")
				if scenario != "empty placeholder" {
					exec(`UPDATE ai_connections SET funding='managed',status='connected',encrypted_secret=decode('1234','hex') WHERE id=$1`, oldRouter)
				}
				switch scenario {
				case "disconnected":
					exec(`UPDATE ai_connections SET status='disconnected' WHERE id=$1`, oldRouter)
				case "customer key":
					exec(`UPDATE ai_connections SET funding='customer' WHERE id=$1`, oldRouter)
				case "renamed":
					exec(`UPDATE ai_connections SET name='Deliberate custom connection' WHERE id=$1`, oldRouter)
				}
				// Both routes must update in one pass, even across two providers.
				exec(`INSERT INTO ai_profiles(id,workspace_id,scope,name,revision,"primary",fallback) VALUES(
 '00000000-0000-0000-0000-000000000200',$1,'workspace','Custom route',7,
 jsonb_build_object('connection_id',$2::text,'model',jsonb_build_object('provider','openrouter','model','my-custom-model','controls','{"reasoning_effort":"high"}'::jsonb)),
 jsonb_build_object('connection_id',$3::text,'model',jsonb_build_object('provider','openai','model','my-fallback-model','controls','{}'::jsonb)))`, workspace, oldRouter, model.StandardAIConnectionID(workspace, "openai"))
				exec(`UPDATE agent_runs SET input=jsonb_set(input,'{model_connection_id}',to_jsonb($1::text))`, oldRouter)
				var frozen string
				if err := db.QueryRowContext(ctx, `SELECT input::text FROM agent_runs`).Scan(&frozen); err != nil {
					t.Fatal(err)
				}
				exec(`DELETE FROM schema_migrations WHERE version IN ('202609140013','202609140014')`)
				if err := Up(ctx, db); err != nil {
					t.Fatal(err)
				}
				mergeRouter := scenario == "managed duplicate" || scenario == "empty placeholder"
				for tier, provider := range map[string]string{"small": "openrouter", "medium": "openrouter", "large": "openai", "flagship": "anthropic"} {
					var connection, actualProvider string
					if err := db.QueryRowContext(ctx, `SELECT "primary"->>'connection_id',"primary"->'model'->>'provider' FROM ai_profiles WHERE id=$1`, model.StandardAIProfileID(workspace, tier)).Scan(&connection, &actualProvider); err != nil {
						t.Fatal(err)
					}
					wantConnection := canonical[provider]
					if provider == "openrouter" && !mergeRouter {
						wantConnection = oldRouter
					}
					if connection != wantConnection || actualProvider != provider {
						t.Fatalf("%s uses %s/%s", tier, actualProvider, connection)
					}
				}
				var primary, fallback, customModel, effort string
				var revision int
				if err := db.QueryRowContext(ctx, `SELECT "primary"->>'connection_id',fallback->>'connection_id',"primary"->'model'->>'model',"primary"->'model'->'controls'->>'reasoning_effort',revision FROM ai_profiles WHERE id='00000000-0000-0000-0000-000000000200'`).Scan(&primary, &fallback, &customModel, &effort, &revision); err != nil {
					t.Fatal(err)
				}
				wantPrimary := oldRouter
				if mergeRouter {
					wantPrimary = canonical["openrouter"]
				}
				if primary != wantPrimary || fallback != canonical["openai"] || customModel != "my-custom-model" || effort != "high" || revision != 8 {
					t.Fatal("primary/fallback references or custom controls migrated incorrectly")
				}
				var visible int
				if err := db.QueryRowContext(ctx, `SELECT count(*) FROM ai_connections WHERE superseded_by IS NULL`).Scan(&visible); err != nil {
					t.Fatal(err)
				}
				wantVisible := 3
				if !mergeRouter {
					wantVisible++
				}
				if visible != wantVisible {
					t.Fatalf("visible connections=%d, want %d", visible, wantVisible)
				}
				var after string
				if err := db.QueryRowContext(ctx, `SELECT input::text FROM agent_runs`).Scan(&after); err != nil || after != frozen {
					t.Fatal("accepted run input changed")
				}
				var oldSecret sql.NullString
				if err := db.QueryRowContext(ctx, `SELECT encode(encrypted_secret,'hex') FROM ai_connections WHERE id=$1`, oldRouter).Scan(&oldSecret); err != nil {
					t.Fatal("old connection removed")
				}
				if scenario != "empty placeholder" && oldSecret.String != "1234" {
					t.Fatal("old credential changed")
				}
				exec(`DELETE FROM schema_migrations WHERE version='202609140014'`)
				if err := Up(ctx, db); err != nil {
					t.Fatal(err)
				}
				if err := db.QueryRowContext(ctx, `SELECT revision FROM ai_profiles WHERE id='00000000-0000-0000-0000-000000000200'`).Scan(&revision); err != nil || revision != 8 {
					t.Fatal("replay changed profiles again")
				}
			})
		})
	}
}
