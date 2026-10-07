//go:build integration

package dbmigrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestAIModelSetupPreservesExistingProfilesPostgres(t *testing.T) {
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
	schema := fmt.Sprintf("ai_model_setup_%d", time.Now().UnixNano())
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
	if _, err := db.ExecContext(ctx, `CREATE TABLE ai_profiles(id text PRIMARY KEY, name text, "primary" jsonb, fallback jsonb, revision bigint);
 CREATE TABLE ai_connections(id text PRIMARY KEY, encrypted_secret bytea);
 INSERT INTO ai_profiles VALUES ('saved','My variant','{"model":{"model":"old","controls":{"reasoning_effort":"high"}}}','{"connection_id":"backup"}',7);
 INSERT INTO ai_connections VALUES ('connection',decode('abc123','hex'));`); err != nil {
		t.Fatal(err)
	}
	data, err := migrationFiles.ReadFile("sql/202610070001_ai_model_setup.sql")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := db.ExecContext(ctx, string(data)); err != nil {
			t.Fatal(err)
		}
	}
	var hidden bool
	var name, primary, fallback string
	var revision int
	if err := db.QueryRowContext(ctx, `SELECT hidden_from_ask_agent,name,"primary"->'model'->>'model',fallback->>'connection_id',revision FROM ai_profiles WHERE id='saved'`).Scan(&hidden, &name, &primary, &fallback, &revision); err != nil {
		t.Fatal(err)
	}
	if hidden || name != "My variant" || primary != "old" || fallback != "backup" || revision != 7 {
		t.Fatalf("migration changed existing selection: %v %s %s %s %d", hidden, name, primary, fallback, revision)
	}
	var secret string
	if err := db.QueryRowContext(ctx, `SELECT encode(encrypted_secret,'hex') FROM ai_connections`).Scan(&secret); err != nil || secret != "abc123" {
		t.Fatalf("credential changed: %v", err)
	}
}
