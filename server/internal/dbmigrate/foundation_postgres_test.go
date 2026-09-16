//go:build integration

package dbmigrate

import (
	"context"
	"database/sql"
	"github.com/helpin-ai/helpin/server/internal/dbschema"
	_ "github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// This fixture must be a dedicated empty database with vector and pgcrypto
// provisioned. It refuses to run against an existing application database.
func TestFreshCommunityFoundationPostgres(t *testing.T) {
	dsn := os.Getenv("COMMUNITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set COMMUNITY_TEST_DATABASE_URL to a disposable empty PostgreSQL database")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	var tables int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE'").Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 0 {
		t.Fatal("refusing non-empty Community test database")
	}
	// Refuse partial/foreign schemas instead of creating a mixed foundation.
	for _, table := range []string{"users", "unexpected_existing_table"} {
		if _, err := db.ExecContext(ctx, "CREATE TABLE "+table+"(id integer)"); err != nil {
			t.Fatal(err)
		}
		if err := Up(ctx, db); err == nil {
			t.Fatal("foundation accepted an incomplete existing schema")
		}
		if _, err := db.ExecContext(ctx, "DROP TABLE "+table); err != nil {
			t.Fatal(err)
		}
	}
	if err := Up(ctx, db); err != nil {
		t.Fatal(err)
	}
	t.Run("API model schema parity", func(t *testing.T) { assertAPIModelSchemaParity(t, db) })
	var before int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&before); err != nil {
		t.Fatal(err)
	}
	if before < 100 {
		t.Fatal("historical ledger did not run")
	}
	// Prove a populated retry preserves application data and the original ledger.
	if _, err := db.ExecContext(ctx, "CREATE TABLE community_restore_probe(id integer PRIMARY KEY, value text NOT NULL); INSERT INTO community_restore_probe VALUES (1,'preserve me')"); err != nil {
		t.Fatal(err)
	}
	// Simulate an existing pre-foundation installation adopting this new ledger
	// entry. Its tables and application data must remain untouched.
	if _, err := db.ExecContext(ctx, "DELETE FROM schema_migrations WHERE version='000000000001'"); err != nil {
		t.Fatal(err)
	}
	if err := Up(ctx, db); err != nil {
		t.Fatal(err)
	}
	var after int
	var value string
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&after); err != nil || before != after {
		t.Fatalf("ledger changed: before=%d after=%d error=%v", before, after, err)
	}
	if err := db.QueryRowContext(ctx, "SELECT value FROM community_restore_probe WHERE id=1").Scan(&value); err != nil || value != "preserve me" {
		t.Fatal("populated retry lost data")
	}
	issues, err := Validate(ctx, db)
	if err != nil || len(issues) != 0 {
		t.Fatalf("validation: %v %v", issues, err)
	}
	// A failed migration must roll back its DDL and ledger row before retry.
	broken := Source{FS: fstest.MapFS{"sql/209901010099_interrupted.sql": {Data: []byte("CREATE TABLE interrupted_probe(id integer PRIMARY KEY); INSERT INTO interrupted_probe VALUES(1),(1);")}}, Directory: "sql"}
	if err := Up(ctx, db, broken); err == nil {
		t.Fatal("invalid migration succeeded")
	}

	if err := Up(ctx, db); err != nil {
		t.Fatal(err)
	}
	var exists bool
	if err := db.QueryRowContext(ctx, "SELECT to_regclass('public.interrupted_probe') IS NOT NULL").Scan(&exists); err != nil || exists {
		t.Fatal("interrupted work persisted")
	}
}

// Compare the actual PostgreSQL schema, not logged DDL: GORM can issue harmless
// ALTERs even when the schema is unchanged. Roll back all probe changes.
func assertAPIModelSchemaParity(t *testing.T, db *sql.DB) {
	t.Helper()
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	tx := gdb.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	snapshot := func() []string {
		var rows []string
		err := tx.Raw(`
SELECT 'column ' || table_name || '.' || column_name || ' ' || udt_name || ' ' || is_nullable || ' ' || coalesce(column_default,'') || ' ' || coalesce(character_maximum_length::text,'') || ' ' || coalesce(numeric_precision::text,'') || ' ' || coalesce(numeric_scale::text,'') AS definition
FROM information_schema.columns WHERE table_schema='public'
UNION ALL SELECT 'index ' || tablename || '.' || indexname || ' ' || indexdef FROM pg_indexes WHERE schemaname='public'
UNION ALL SELECT 'constraint ' || c.relname || '.' || con.conname || ' ' || pg_get_constraintdef(con.oid) FROM pg_constraint con JOIN pg_class c ON c.oid=con.conrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public'
ORDER BY definition`).Scan(&rows).Error
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	before := snapshot()
	if err := tx.AutoMigrate(dbschema.AutoMigrationModels()...); err != nil {
		t.Fatalf("API AutoMigrate after ledger: %v", err)
	}
	after := snapshot()
	if !slices.Equal(before, after) {
		var diff []string
		for _, row := range before {
			if !slices.Contains(after, row) {
				diff = append(diff, "- "+row)
			}
		}
		for _, row := range after {
			if !slices.Contains(before, row) {
				diff = append(diff, "+ "+row)
			}
		}
		t.Fatalf("API models differ from the fresh ledger schema; add a versioned SQL migration:\n%s", strings.Join(diff, "\n"))
	}
}
