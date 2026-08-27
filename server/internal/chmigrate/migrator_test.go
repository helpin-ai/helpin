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
	if len(migrations) != 4 {
		t.Fatalf("migration count = %d, want 4", len(migrations))
	}
	wantVersions := []string{"202608250001", "202608250002", "202608250003", "202608270001"}
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

func TestUsermavenEventSchemaContract(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	baseline := migrations[0].SQL
	for _, column := range []string{
		"`project_id`", "`event_id`", "`event_type`", "`user_anonymous_id`",
		"`company_id`", "`company_name`", "`parsed_ua_bot`", "`parsed_referer_channel`",
		"`identity_method`", "`identity_trust`", "`raw_event`", "`event_date`",
		"`event_received_at`", "`visitor_shard`", "`_nats_stream_sequence`",
		"`_nats_delivery_attempt`", "`_retro_generation`", "`_ingest_version`",
	} {
		if !strings.Contains(baseline, column) {
			t.Errorf("baseline migration missing column %s", column)
		}
	}
	for _, contract := range []string{
		"ENGINE = ReplacingMergeTree(_ingest_version)",
		"ORDER BY (project_id, event_date, event_id)",
		"parseDateTime64BestEffort(JSONExtractString(raw_event, 'timestamp')",
	} {
		if !strings.Contains(baseline, contract) {
			t.Errorf("baseline migration missing contract %q", contract)
		}
	}
	if strings.Contains(
		baseline,
		"parseDateTime64BestEffortOrZero(JSONExtractString(raw_event, 'timestamp')",
	) {
		t.Fatal("event timestamp must not silently fall back to the Unix epoch")
	}
	ingestion := migrations[2].SQL
	for _, contract := range []string{
		"CREATE TABLE IF NOT EXISTS usermaven.session_seed_events",
		"INDEX `idx_seed_event_timestamp` event_timestamp TYPE minmax",
		"JSONExtractString(raw_event, 'project_id')",
		"TTL event_received_at + INTERVAL 49 HOUR DELETE",
		"WHERE _retro_generation = 0",
	} {
		if !strings.Contains(ingestion, contract) {
			t.Errorf("session seed migration missing contract %q", contract)
		}
	}
	if strings.Contains(ingestion, "ENGINE = Kafka") {
		t.Fatal("clean-slate ingestion migration must not create a Kafka engine")
	}
	aiClassification := migrations[3].SQL
	for _, contract := range []string{
		"`parsed_ua_bot_category` LowCardinality(String)",
		"`parsed_ua_bot_name` LowCardinality(String)",
		"`parsed_ua_bot_provider` LowCardinality(String)",
		"INDEX IF NOT EXISTS `idx_parsed_ua_bot_category`",
	} {
		if !strings.Contains(aiClassification, contract) {
			t.Errorf("AI bot classification migration missing contract %q", contract)
		}
	}
	if strings.Contains(aiClassification, "MATERIALIZE COLUMN") {
		t.Fatal("AI bot classification migration must not rewrite historical event parts")
	}
}
