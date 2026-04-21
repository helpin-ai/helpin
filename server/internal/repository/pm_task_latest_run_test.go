package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestPMTaskRepository_ApplyLatestRunMetadata(t *testing.T) {
	t.Parallel()

	dbName := fmt.Sprintf("file:pm-task-latest-run-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE agent_runs (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		agent_id TEXT NOT NULL,
		target_type TEXT NOT NULL,
		target_id TEXT NOT NULL,
		status TEXT NOT NULL,
		pause_reason TEXT NOT NULL DEFAULT 'none',
		started_at DATETIME,
		created_at DATETIME NOT NULL,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create agent_runs table: %v", err)
	}

	repo := NewPMTaskRepository(db)
	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)

	seedRun := func(id, workspaceID, agentID, targetID, status, pauseReason string, startedAt *time.Time, createdAt time.Time) {
		if err := db.Exec(
			`INSERT INTO agent_runs (id, workspace_id, agent_id, target_type, target_id, status, pause_reason, started_at, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, workspaceID, agentID, "task", targetID, status, pauseReason, startedAt, createdAt, createdAt,
		).Error; err != nil {
			t.Fatalf("seed agent_run %s: %v", id, err)
		}
	}

	seedRun("run-old", "ws-1", "agent-old", "task-1", model.AgentRunStatusRunning, model.AgentRunPauseReasonNone, testTimePointer(now.Add(-2*time.Hour)), now.Add(-2*time.Hour))
	seedRun("run-latest", "ws-1", "agent-latest", "task-1", model.AgentRunStatusCompleted, model.AgentRunPauseReasonNone, testTimePointer(now.Add(-30*time.Minute)), now.Add(-31*time.Minute))
	seedRun("run-other-task", "ws-1", "agent-other", "task-2", model.AgentRunStatusPaused, model.AgentRunPauseReasonHumanInput, nil, now.Add(-10*time.Minute))

	tasks := repo.applyLatestRunMetadata(context.Background(), []model.PMTask{
		{ID: "task-1"},
		{ID: "task-2"},
		{ID: "task-3"},
	})

	if got := derefString(tasks[0].LatestRunID); got != "run-latest" {
		t.Fatalf("task-1 latest_run_id = %q, want run-latest", got)
	}
	if got := derefString(tasks[0].LatestRunAgentID); got != "agent-latest" {
		t.Fatalf("task-1 latest_run_agent_id = %q, want agent-latest", got)
	}
	if got := derefString(tasks[0].LatestRunStatus); got != model.AgentRunStatusCompleted {
		t.Fatalf("task-1 latest_run_status = %q, want %q", got, model.AgentRunStatusCompleted)
	}
	if tasks[0].LatestRunPauseReason != nil {
		t.Fatalf("task-1 latest_run_pause_reason = %v, want nil", tasks[0].LatestRunPauseReason)
	}
	if tasks[0].LatestRunAt == nil || !tasks[0].LatestRunAt.Equal(now.Add(-30*time.Minute)) {
		t.Fatalf("task-1 latest_run_at = %v, want %v", tasks[0].LatestRunAt, now.Add(-30*time.Minute))
	}

	if got := derefString(tasks[1].LatestRunID); got != "run-other-task" {
		t.Fatalf("task-2 latest_run_id = %q, want run-other-task", got)
	}
	if got := derefString(tasks[1].LatestRunAgentID); got != "agent-other" {
		t.Fatalf("task-2 latest_run_agent_id = %q, want agent-other", got)
	}
	if got := derefString(tasks[1].LatestRunPauseReason); got != model.AgentRunPauseReasonHumanInput {
		t.Fatalf("task-2 latest_run_pause_reason = %q, want %q", got, model.AgentRunPauseReasonHumanInput)
	}
	if tasks[2].LatestRunID != nil || tasks[2].LatestRunAgentID != nil || tasks[2].LatestRunStatus != nil || tasks[2].LatestRunPauseReason != nil || tasks[2].LatestRunAt != nil {
		t.Fatalf("task-3 expected no latest run metadata, got %+v", tasks[2])
	}
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func testTimePointer(value time.Time) *time.Time {
	return &value
}
