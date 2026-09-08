package repository

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAgentRuntimeRecoveryIncludesUnsettledTerminalRuns(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementSoft, 1000)
	testAgentRuntimeRecovery(t, repo)
}

func testAgentRuntimeRecovery(t *testing.T, repo *AIUsageRepository) {
	t.Helper()
	summaryType, timeType := "blob", "datetime"
	if repo.db.Dialector.Name() == "postgres" {
		summaryType, timeType = "jsonb", "timestamptz"
	}

	if err := repo.db.Exec(`CREATE TABLE agent_runs (id text PRIMARY KEY, workspace_id text, external_runtime text, external_runtime_id text, status text, input_tokens bigint, output_tokens bigint, output_summary ` + summaryType + `, updated_at ` + timeType + `)`).Error; err != nil {
		t.Fatal(err)
	}
	rows := []struct {
		id, status, summary string
		input               int
	}{
		{"pending", "cancelled", `{"ai_usage_metering":{},"ai_usage_checkpoint":{"input_tokens":100}}`, 200},
		{"late", "completed", `{"ai_usage_metering":{},"agent_runtime_usage_consumed":true,"ai_usage_checkpoint":{"input_tokens":100}}`, 200},
		{"settled", "completed", `{"ai_usage_metering":{},"agent_runtime_usage_consumed":true,"ai_usage_checkpoint":{"input_tokens":200}}`, 200},
		{"legacy", "failed", `{}`, 200},
	}
	for _, row := range rows {
		if err := repo.db.Exec("INSERT INTO agent_runs VALUES (?, 'ws', 'agent_runtime', ?, ?, ?, 0, ?, ?)", row.id, row.id, row.status, row.input, []byte(row.summary), time.Now().Add(-time.Hour)).Error; err != nil {
			t.Fatal(err)
		}
	}
	runs, err := NewAgentRunRepository(repo.db).ListRuntimeReconciliationCandidates(context.Background(), "agent_runtime", time.Now(), 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].ID != "pending" {
		t.Fatalf("first batch=%+v", runs)
	}
	next, err := NewAgentRunRepository(repo.db).ListRuntimeReconciliationCandidates(context.Background(), "agent_runtime", time.Now(), 1, &runs[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(next) != 1 || next[0].ID != "late" {
		t.Fatalf("next batch=%+v", next)
	}
}
