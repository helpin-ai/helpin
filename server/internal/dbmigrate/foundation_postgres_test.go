//go:build integration

package dbmigrate

import (
	"context"
	"database/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"os"
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
