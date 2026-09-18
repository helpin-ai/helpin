//go:build integration

package repository

import (
	"context"
	"database/sql"
	_ "github.com/ClickHouse/clickhouse-go/v2"
	"os"
	"testing"
	"time"
)

// Requires an isolated empty instance; refuses to modify an existing helpin database.
func TestBehavioralQueriesClickHouseIdentityFilters(t *testing.T) {
	dsn := os.Getenv("CRM_SIGNAL_TEST_CLICKHOUSE_DSN")
	if dsn == "" {
		t.Skip("set CRM_SIGNAL_TEST_CLICKHOUSE_DSN to an isolated empty instance")
	}
	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `CREATE DATABASE helpin`); err != nil {
		t.Fatal(err)
	}
	defer db.ExecContext(ctx, `DROP DATABASE helpin SYNC`)
	_, err = db.ExecContext(ctx, `CREATE TABLE helpin.events (
 project_id String,user_anonymous_id String,user_id String,company_id String,identity_method String,identity_trust String,
 _timestamp DateTime64(3,'UTC'),session_id String,event_type String,doc_path String,event_attributes String,parsed_ua_bot String
 ) ENGINE=ReplacingMergeTree ORDER BY (project_id,event_type,_timestamp,session_id)`)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []string{"pageview", "article_view", "payment_failed", "downgrade_requested"} {
		for _, session := range []string{"one", "two"} {
			_, err = db.ExecContext(ctx, `INSERT INTO helpin.events VALUES ('project','anonymous','user','company','server_event','verified', '2026-09-17 12:00:00',?,?,'/pricing',?,'false')`, session, event, `{"article_id":"article"}`)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	repo := &ClickHouseEventRepository{db: db}
	start := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 1)
	projects := []string{"project"}
	tests := []struct {
		name string
		run  func() ([]behavioralRuleRow, error)
		want int
	}{
		{"pricing", func() ([]behavioralRuleRow, error) {
			return repo.pathActivityRows(ctx, projects, start, end, []string{"/pricing"}, 2, true)
		}, 1},
		{"article", func() ([]behavioralRuleRow, error) { return repo.identifiedArticleRows(ctx, projects, start, end, nil) }, 1},
		{"high intent", func() ([]behavioralRuleRow, error) {
			return repo.highIntentEventRows(ctx, projects, start, end, []string{"payment_failed"})
		}, 1},
		{"payment", func() ([]behavioralRuleRow, error) {
			return repo.commercialEdgeRows(ctx, projects, start, end, "payment_failed")
		}, 1},
		{"downgrade", func() ([]behavioralRuleRow, error) {
			return repo.commercialEdgeRows(ctx, projects, start, end, "downgrade_requested")
		}, 1},
		{"dormancy", func() ([]behavioralRuleRow, error) {
			return repo.returnedAfterDormancyRows(ctx, projects, start.AddDate(0, 0, -60), start, end, 30)
		}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := tt.run()
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != tt.want {
				t.Fatalf("got %d rows want %d", len(rows), tt.want)
			}
			for _, row := range rows {
				if row.IdentityTrust != "verified" || row.IdentityMethod != "server_event" {
					t.Fatalf("identity fields lost: %#v", row)
				}
			}
		})
	}
}
