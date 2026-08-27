package repository

import (
	"context"
	"strings"
	"testing"
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
