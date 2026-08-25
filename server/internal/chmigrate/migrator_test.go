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

func TestRenderMigrationSQLUsesKafkaEnvironment(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "cluster-kafka-bootstrap.eventpipeline.svc:9092")
	t.Setenv("KAFKA_SESSIONIZED_TOPIC", "helpin.events.production")
	t.Setenv("CLICKHOUSE_KAFKA_GROUP", "helpin-clickhouse-production")
	t.Setenv("KAFKA_AUTH", "true")
	t.Setenv("KAFKA_SECURITY_PROTOCOL", "SASL_PLAINTEXT")
	t.Setenv("KAFKA_SASL", "SCRAM-SHA-512")
	t.Setenv("KAFKA_USERNAME", "helpin-eventpipeline")
	t.Setenv("KAFKA_PASSWORD", "secret'with\\characters")

	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	rendered, err := renderMigrationSQL(migrations[2].SQL)
	if err != nil {
		t.Fatalf("render migration: %v", err)
	}
	for _, value := range []string{
		"'cluster-kafka-bootstrap.eventpipeline.svc:9092'",
		"'helpin.events.production'",
		"'helpin-clickhouse-production'",
		"kafka_security_protocol = 'sasl_plaintext'",
		"kafka_sasl_mechanism = 'SCRAM-SHA-512'",
		"kafka_sasl_username = 'helpin-eventpipeline'",
		`kafka_sasl_password = 'secret\'with\\characters'`,
	} {
		if !strings.Contains(rendered, value) {
			t.Errorf("rendered migration missing %q", value)
		}
	}
}

func TestRenderMigrationSQLRequiresCompleteKafkaAuth(t *testing.T) {
	t.Setenv("KAFKA_AUTH", "true")
	for _, name := range []string{
		"KAFKA_SECURITY_PROTOCOL", "KAFKA_SASL", "KAFKA_USERNAME", "KAFKA_PASSWORD",
	} {
		t.Setenv(name, "")
	}
	if _, err := renderMigrationSQL("SELECT 1"); err == nil {
		t.Fatal("renderMigrationSQL returned nil error with incomplete Kafka auth")
	}
}

func TestRenderMigrationSQLRequiresProductionKafkaConfig(t *testing.T) {
	t.Setenv("CLICKHOUSE_MIGRATION_REQUIRE_KAFKA_CONFIG", "true")
	t.Setenv("KAFKA_BROKERS", "")
	t.Setenv("KAFKA_SESSIONIZED_TOPIC", "helpin.events.sessionized")
	t.Setenv("KAFKA_AUTH", "false")
	if _, err := renderMigrationSQL("SELECT 1"); err == nil {
		t.Fatal("renderMigrationSQL returned nil error without production Kafka brokers")
	}
}
