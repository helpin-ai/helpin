package repository

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestBrowserBehavioralQueriesRequireBotExclusion(t *testing.T) {
	repository := &ClickHouseEventRepository{}
	_, err := repository.queryBrowserBehavioralRows(
		context.Background(),
		"test browser activity",
		"SELECT 1",
	)
	if err == nil {
		t.Fatal("browser behavioral query without a bot filter must fail closed")
	}
	if !strings.Contains(err.Error(), "must exclude detected bots") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPreIdentificationQueryAnchorsActualFirstIdentification(t *testing.T) {
	end := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	start := end.Add(-time.Hour)
	query, args := preIdentificationRuleQuery([]string{"project-1", "project-2"}, end.AddDate(0, 0, -180), start, end)
	for _, required := range []string{
		"minIf(_timestamp, user_id != '') AS identified_at",
		"HAVING identified_at >= ? AND identified_at < ?",
		"event._timestamp < transition.identified_at",
		"GROUP BY event.user_anonymous_id, transition.identified_at",
	} {
		if !strings.Contains(query, required) {
			t.Fatalf("pre-identification query missing %q", required)
		}
	}
	if len(args) != 10 {
		t.Fatalf("pre-identification args = %d, want 10", len(args))
	}
}
