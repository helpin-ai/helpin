package dbmigrate

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestMigrationSourcesPreserveCoreChecksums(t *testing.T) {
	core, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	extra := Source{FS: fstest.MapFS{"sql/209901010001_ee.sql": {Data: []byte("SELECT 1;")}}, Directory: "sql"}
	all, err := loadMigrations(extra)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != len(core)+1 {
		t.Fatalf("source count=%d", len(all))
	}
	for i := range core {
		if all[i] != core[i] {
			t.Fatalf("historical migration changed: %s", core[i].Version)
		}
	}
	if all[len(all)-1].Version != "209901010001" {
		t.Fatal("source ordering changed")
	}
	duplicate := Source{FS: fstest.MapFS{"sql/" + core[0].Version + "_collision.sql": {Data: []byte("SELECT 2;")}}, Directory: "sql"}
	if _, err := loadMigrations(duplicate); err == nil || !strings.Contains(err.Error(), "duplicate migration version") {
		t.Fatalf("duplicate error=%v", err)
	}
}

func TestCommunityMigratorAcceptsAppliedEERows(t *testing.T) {
	core, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	db := migrationLedgerDB(t, core, false)
	ctx := context.Background()
	if err := Up(ctx, db); err != nil {
		t.Fatalf("community startup migration: %v", err)
	}
	issues, err := Validate(ctx, db)
	if err != nil || len(issues) != 0 {
		t.Fatalf("community validation: %+v %v", issues, err)
	}
	rows, err := Status(ctx, db)
	if err != nil || len(rows) != len(core) {
		t.Fatalf("community status: %d %v", len(rows), err)
	}
	for _, row := range rows {
		if !row.Applied {
			t.Fatalf("core migration is unexpectedly pending: %s", row.Version)
		}
	}
	pending, err := Pending(ctx, db)
	if err != nil || len(pending) != 0 {
		t.Fatalf("community pending: %+v %v", pending, err)
	}
}

func TestOptionalSourcesDoNotWeakenCoreChecksums(t *testing.T) {
	core, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	db := migrationLedgerDB(t, core, true)
	if err := Up(context.Background(), db); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("checksum enforcement: %v", err)
	}
	issues, err := Validate(context.Background(), db)
	if err != nil || len(issues) != 1 || issues[0].Kind != "checksum_mismatch" {
		t.Fatalf("validation=%+v %v", issues, err)
	}
}

// This SQL driver exercises the public migration entry points with an applied
// ledger. It rejects any attempt to apply SQL or rewrite historical ledger rows.
func migrationLedgerDB(t *testing.T, core []Migration, corrupt bool) *sql.DB {
	t.Helper()
	rows := make([][]driver.Value, 0, len(core)+1)
	for i, m := range core {
		checksum := m.Checksum
		if corrupt && i == 0 {
			checksum = "changed"
		}
		rows = append(rows, []driver.Value{m.Version, checksum, time.Now().UTC()})
	}
	rows = append(rows, []driver.Value{"209901010001", "ee-checksum-without-source", time.Now().UTC()})
	name := fmt.Sprintf("migration-ledger-%d", time.Now().UnixNano())
	sql.Register(name, &migrationLedgerDriver{rows: rows})
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db
}

type migrationLedgerDriver struct{ rows [][]driver.Value }

func (d *migrationLedgerDriver) Open(string) (driver.Conn, error) {
	return &migrationLedgerConn{rows: d.rows}, nil
}

type migrationLedgerConn struct{ rows [][]driver.Value }

func (c *migrationLedgerConn) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("unexpected prepare")
}
func (c *migrationLedgerConn) Close() error { return nil }
func (c *migrationLedgerConn) Begin() (driver.Tx, error) {
	return nil, fmt.Errorf("unexpected migration transaction")
}
func (c *migrationLedgerConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if strings.Contains(query, "pg_try_advisory_lock") {
		return &migrationLedgerRows{columns: []string{"locked"}, values: [][]driver.Value{{true}}}, nil
	}
	if strings.Contains(query, "SELECT version, checksum, applied_at") {
		return &migrationLedgerRows{columns: []string{"version", "checksum", "applied_at"}, values: c.rows}, nil
	}
	return nil, fmt.Errorf("unexpected migration query: %s", query)
}
func (c *migrationLedgerConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	if strings.Contains(query, "CREATE TABLE IF NOT EXISTS schema_migrations") || strings.Contains(query, "pg_advisory_unlock") {
		return driver.RowsAffected(0), nil
	}
	return nil, fmt.Errorf("unexpected historical migration write: %s", query)
}

type migrationLedgerRows struct {
	columns []string
	values  [][]driver.Value
	next    int
}

func (r *migrationLedgerRows) Columns() []string { return r.columns }
func (r *migrationLedgerRows) Close() error      { return nil }
func (r *migrationLedgerRows) Next(dest []driver.Value) error {
	if r.next >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.next])
	r.next++
	return nil
}
