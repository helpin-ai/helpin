package crmsituationtest

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gorm.io/gorm"
)

func applyPlaybookMigration(t testing.TB, db *gorm.DB) {
	t.Helper()
	_, path, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate playbook migration")
	}
	source, err := os.ReadFile(filepath.Join(filepath.Dir(path), "../dbmigrate/sql/202609070001_crm_playbooks.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if db.Dialector.Name() == "postgres" {
		Exec(t, db, string(source))
		Exec(t, db, string(source))
		return
	}
	// SQLite cannot ALTER composite constraints. PostgreSQL fixtures exercise
	// the actual constraints and repeated migration; SQLite keeps equivalent columns.
	tables, rest, _ := strings.Cut(string(source), "-- Progress belongs")
	tables = strings.ReplaceAll(tables, "timestamptz", "datetime")
	Exec(t, db, tables)
	Exec(t, db, tables)
	columns, _, _ := strings.Cut(rest, "-- PostgreSQL composite constraints")
	_, columns, _ = strings.Cut(columns, "ALTER TABLE")
	columns = "ALTER TABLE" + columns
	columns = strings.NewReplacer("ADD COLUMN IF NOT EXISTS", "ADD COLUMN", "timestamptz", "datetime").Replace(columns)
	Exec(t, db, columns)
}
