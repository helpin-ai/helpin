//go:build integration

package dbmigrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestCommunityMigrationLedgerPostgres(t *testing.T) {
	dsn := os.Getenv("AI_PROFILES_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("AI_PROFILES_TEST_DATABASE_URL is required for the isolated PostgreSQL fixture")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	schema := fmt.Sprintf("ai_profile_review_%d", time.Now().UnixNano())
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
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = ensureSchemaMigrationsTable(ctx, conn)
	if closeErr := conn.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	core, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	// Represent an upgraded installation without rerunning unrelated historical DDL.
	for _, m := range core {
		if _, err := db.ExecContext(ctx, "INSERT INTO schema_migrations(version,name,checksum) VALUES ($1,$2,$3)", m.Version, m.Name, m.Checksum); err != nil {
			t.Fatal(err)
		}
	}
	extra := Source{FS: fstest.MapFS{"sql/209901010001_ee.sql": {Data: []byte("CREATE TABLE ee_fixture (id integer PRIMARY KEY); INSERT INTO ee_fixture VALUES (1);")}}, Directory: "sql"}
	if err := Up(ctx, db, extra); err != nil {
		t.Fatal(err)
	}
	if err := Up(ctx, db); err != nil {
		t.Fatalf("Community rejected applied EE row: %v", err)
	}
	if err := Up(ctx, db, extra); err != nil {
		t.Fatalf("EE retry failed: %v", err)
	}
	issues, err := Validate(ctx, db)
	if err != nil || len(issues) != 0 {
		t.Fatalf("Community validation: %v %v", issues, err)
	}
	// A real unique-key conflict must roll back both the business write and ledger.
	broken := Source{FS: fstest.MapFS{"sql/209901010002_conflict.sql": {Data: []byte("INSERT INTO ee_fixture VALUES (2); INSERT INTO ee_fixture VALUES (1);")}}, Directory: "sql"}
	if err := Up(ctx, db, extra, broken); err == nil {
		t.Fatal("constraint conflict was accepted")
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM ee_fixture WHERE id=2").Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial business commit: %d %v", count, err)
	}
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations WHERE version='209901010002'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed migration recorded: %d %v", count, err)
	}
	repaired := Source{FS: fstest.MapFS{"sql/209901010002_conflict.sql": {Data: []byte("INSERT INTO ee_fixture VALUES (2);")}}, Directory: "sql"}
	if err := Up(ctx, db, extra, repaired); err != nil {
		t.Fatalf("retry after rollback: %v", err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE schema_migrations SET checksum='changed' WHERE version=$1", core[0].Version); err != nil {
		t.Fatal(err)
	}
	if err := Up(ctx, db); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("core checksum not enforced: %v", err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE schema_migrations SET checksum=$1 WHERE version=$2", core[0].Checksum, core[0].Version); err != nil {
		t.Fatal(err)
	}
	// Run the new index migration against real PostgreSQL twice (idempotency).
	// Agents are hard-deleted: unlike profiles, their schema has no deleted_at.
	if _, err := db.ExecContext(ctx, "CREATE TABLE agents (ai_profile_id uuid, workspace_id uuid); CREATE TABLE agent_versions (ai_profile_id uuid)"); err != nil {
		t.Fatal(err)
	}
	data, err := migrationFiles.ReadFile("sql/202609140008_ai_profile_reference_indexes.sql")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := db.ExecContext(ctx, string(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM pg_indexes WHERE schemaname=$1 AND indexname IN ('idx_agents_ai_profile_id','idx_agent_versions_ai_profile_id')", schema).Scan(&count); err != nil || count != 2 {
		t.Fatalf("missing indexes: %d %v", count, err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE ai_profiles (id uuid PRIMARY KEY, workspace_id uuid, user_id uuid, scope text, deleted_at timestamptz);
 INSERT INTO ai_profiles VALUES ('00000000-0000-0000-0000-000000000001','00000000-0000-0000-0000-000000000002',NULL,'workspace',NULL)`); err != nil {
		t.Fatal(err)
	}
	// Leave the liveness migration pending, as on an installation upgrading from
	// 008. Exercise the same transactional admission and ledger path as cmd/migrate.
	if _, err := db.ExecContext(ctx, "DELETE FROM schema_migrations WHERE version='202609140009'"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := Up(ctx, db); err != nil {
			t.Fatalf("upgrade with hard-deleted agents: %v", err)
		}
	}
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations WHERE version='202609140009'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("liveness migration not recorded: %d %v", count, err)
	}
	const assign = `INSERT INTO agents(ai_profile_id,workspace_id) VALUES ('00000000-0000-0000-0000-000000000001','00000000-0000-0000-0000-000000000002')`
	if _, err := db.ExecContext(ctx, assign); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "DELETE FROM agents; UPDATE ai_profiles SET deleted_at=now()"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, assign); err == nil {
		t.Fatal("new agent references deleted profile")
	}

}
