package chmigrate

import (
	"strings"
	"testing"
)

func TestLoadMigrations(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	if len(migrations) != 3 {
		t.Fatalf("migration count = %d, want 3", len(migrations))
	}
	wantVersions := []string{"202608250001", "202608250002", "202608250003"}
	for index, want := range wantVersions {
		if migrations[index].Version != want {
			t.Errorf("migration %d version = %q, want %q", index, migrations[index].Version, want)
		}
		if len(migrations[index].Checksum) != 64 {
			t.Errorf("migration %d checksum length = %d, want 64", index, len(migrations[index].Checksum))
		}
	}
}

func TestSplitStatementsPreservesSemicolonsInsideSyntax(t *testing.T) {
	sql := `
-- comment; ignored
CREATE TABLE test (` + "`value`" + ` Enum8('one;still one' = 1));
/* another; comment */
ALTER TABLE test ADD COLUMN "quoted;name" String DEFAULT 'it\'s; fine';
`
	statements, err := splitStatements(sql)
	if err != nil {
		t.Fatalf("split statements: %v", err)
	}
	if len(statements) != 2 {
		t.Fatalf("statement count = %d, want 2: %#v", len(statements), statements)
	}
	if !strings.Contains(statements[0], "one;still one") {
		t.Errorf("first statement lost quoted semicolon: %q", statements[0])
	}
	if !strings.Contains(statements[1], "quoted;name") {
		t.Errorf("second statement lost quoted identifier: %q", statements[1])
	}
}

func TestSplitStatementsRejectsUnterminatedSyntax(t *testing.T) {
	for _, input := range []string{"SELECT 'unterminated", "SELECT 1 /* unterminated"} {
		if _, err := splitStatements(input); err == nil {
			t.Errorf("splitStatements(%q) returned nil error", input)
		}
	}
}

func TestReplacementTableStatementUsesBaselineSchema(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	statement, err := replacementTableStatement(migrations[0].SQL)
	if err != nil {
		t.Fatalf("replacement table statement: %v", err)
	}
	if !strings.Contains(statement, "CREATE TABLE IF NOT EXISTS usermaven.events_next") {
		t.Fatal("replacement statement does not target events_next")
	}
	if !strings.Contains(statement, "ENGINE = ReplacingMergeTree(ver)") {
		t.Fatal("replacement statement lost the canonical table engine")
	}
}

func TestUsermavenEventSchemaContract(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	baseline := migrations[0].SQL
	for _, column := range []string{
		"`project_id`", "`event_id`", "`event_type`", "`user_anonymous_id`",
		"`company_id`", "`company_name`", "`parsed_ua_bot`", "`parsed_referer_channel`",
		"`identity_method`", "`identity_trust`", "`raw_event`",
	} {
		if !strings.Contains(baseline, column) {
			t.Errorf("baseline migration missing column %s", column)
		}
	}
	if !strings.Contains(migrations[2].SQL, "ENGINE = Kafka") {
		t.Fatal("ingestion migration missing Kafka table engine")
	}
	if !strings.Contains(migrations[2].SQL, "CREATE MATERIALIZED VIEW") {
		t.Fatal("ingestion migration missing materialized view")
	}
}
