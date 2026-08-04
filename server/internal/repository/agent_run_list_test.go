package repository

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

func TestAgentRunRepositoryListByWorkspaceOmitsOutputSummary(t *testing.T) {
	db := openAgentRunListTestDB(t)
	repo := NewAgentRunRepository(db)
	largeSummary := `{"summary":"` + strings.Repeat("x", 1024*1024) + `"}`
	seedAgentRunListTestRow(
		t,
		db,
		"run-workspace",
		"workspace-1",
		"agent-1",
		model.AgentRunStatusCompleted,
		model.AgentRunPauseReasonNone,
		largeSummary,
	)

	runs, total, err := repo.ListByWorkspace(
		context.Background(),
		"workspace-1",
		model.PMPagination{Page: 1, PerPage: 10},
	)
	if err != nil {
		t.Fatalf("ListByWorkspace: %v", err)
	}
	if total != 1 {
		t.Fatalf("ListByWorkspace total = %d, want 1", total)
	}
	if len(runs) != 1 {
		t.Fatalf("ListByWorkspace returned %d runs, want 1", len(runs))
	}
	if len(runs[0].OutputSummary) != 0 {
		t.Fatalf("list output_summary length = %d, want 0", len(runs[0].OutputSummary))
	}
	if got := string(runs[0].Input); got != `{"trigger":{"source":"manual"}}` {
		t.Fatalf("list input = %s, want compact trigger input", got)
	}

	detail, err := repo.GetByID(context.Background(), "workspace-1", "run-workspace")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if detail == nil {
		t.Fatal("GetByID returned nil, want full run")
	}
	if got := string(detail.OutputSummary); got != largeSummary {
		t.Fatalf("detail output_summary length = %d, want %d", len(got), len(largeSummary))
	}
}

func TestAgentRunRepositoryListByAgentOmitsOutputSummary(t *testing.T) {
	db := openAgentRunListTestDB(t)
	repo := NewAgentRunRepository(db)
	seedAgentRunListTestRow(
		t,
		db,
		"run-agent",
		"workspace-1",
		"agent-1",
		model.AgentRunStatusRunning,
		model.AgentRunPauseReasonNone,
		`{"large":"payload"}`,
	)

	runs, total, err := repo.ListByAgent(
		context.Background(),
		"workspace-1",
		"agent-1",
		model.PMPagination{Page: 1, PerPage: 10},
	)
	if err != nil {
		t.Fatalf("ListByAgent: %v", err)
	}
	if total != 1 {
		t.Fatalf("ListByAgent total = %d, want 1", total)
	}
	if len(runs) != 1 {
		t.Fatalf("ListByAgent returned %d runs, want 1", len(runs))
	}
	if len(runs[0].OutputSummary) != 0 {
		t.Fatalf("list output_summary length = %d, want 0", len(runs[0].OutputSummary))
	}
}

func TestAgentRunRepositoryListByWorkspaceUsesStableTieBreaker(t *testing.T) {
	db := openAgentRunListTestDB(t)
	repo := NewAgentRunRepository(db)
	createdAt := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)

	for _, id := range []string{"run-a", "run-c", "run-b"} {
		seedAgentRunListTestRow(
			t,
			db,
			id,
			"workspace-stable-order",
			"agent-1",
			model.AgentRunStatusCompleted,
			model.AgentRunPauseReasonNone,
			`{}`,
		)
		if err := db.Model(&model.AgentRun{}).Where("id = ?", id).Updates(map[string]any{
			"created_at": createdAt,
			"updated_at": createdAt,
		}).Error; err != nil {
			t.Fatalf("set tied timestamps for %s: %v", id, err)
		}
	}

	runs, _, err := repo.ListByWorkspace(
		context.Background(),
		"workspace-stable-order",
		model.PMPagination{Page: 1, PerPage: 10},
	)
	if err != nil {
		t.Fatalf("ListByWorkspace: %v", err)
	}
	if got, want := []string{runs[0].ID, runs[1].ID, runs[2].ID}, []string{"run-c", "run-b", "run-a"}; !slices.Equal(got, want) {
		t.Fatalf("stable run order = %v, want %v", got, want)
	}
}

func TestAgentRunRepositoryUpdateReconciledFailurePreservesOutputSummary(t *testing.T) {
	db := openAgentRunListTestDB(t)
	repo := NewAgentRunRepository(db)
	const outputSummary = `{"result":"must survive reconciliation"}`
	seedAgentRunListTestRow(
		t,
		db,
		"run-stale",
		"workspace-1",
		"agent-1",
		model.AgentRunStatusRunning,
		model.AgentRunPauseReasonNone,
		outputSummary,
	)

	now := time.Now().UTC()
	errMessage := "run remained stuck"
	stage := "failed"
	partial := &model.AgentRun{
		ID:              "run-stale",
		WorkspaceID:     "workspace-1",
		Status:          model.AgentRunStatusFailed,
		PauseReason:     model.AgentRunPauseReasonNone,
		CompletedAt:     &now,
		ErrorMessage:    &errMessage,
		ExecutionStage:  &stage,
		LastHeartbeatAt: &now,
	}
	if err := repo.UpdateReconciledFailure(context.Background(), partial); err != nil {
		t.Fatalf("UpdateReconciledFailure: %v", err)
	}

	var stored model.AgentRun
	if err := db.Where("id = ?", partial.ID).First(&stored).Error; err != nil {
		t.Fatalf("load reconciled run: %v", err)
	}
	if got := string(stored.OutputSummary); got != outputSummary {
		t.Fatalf("output_summary = %s, want %s", got, outputSummary)
	}
	if stored.Status != model.AgentRunStatusFailed {
		t.Fatalf("status = %q, want failed", stored.Status)
	}
}
func TestAgentRunRepositoryCountWorkspaceRunsRequiringAttention(t *testing.T) {
	db := openAgentRunListTestDB(t)
	repo := NewAgentRunRepository(db)
	rows := []struct {
		id          string
		workspaceID string
		status      string
		pauseReason string
	}{
		{id: "human-input", workspaceID: "workspace-1", status: model.AgentRunStatusPaused, pauseReason: model.AgentRunPauseReasonHumanInput},
		{id: "human-approval", workspaceID: "workspace-1", status: model.AgentRunStatusPaused, pauseReason: model.AgentRunPauseReasonHumanApproval},
		{id: "authentication", workspaceID: "workspace-1", status: model.AgentRunStatusPaused, pauseReason: model.AgentRunPauseReasonAuthentication},
		{id: "legacy-pause", workspaceID: "workspace-1", status: model.AgentRunStatusPaused, pauseReason: model.AgentRunPauseReasonNone},
		{id: "chat-reply", workspaceID: "workspace-1", status: model.AgentRunStatusPaused, pauseReason: model.AgentRunPauseReasonUserMessage},
		{id: "running", workspaceID: "workspace-1", status: model.AgentRunStatusRunning, pauseReason: model.AgentRunPauseReasonNone},
		{id: "other-workspace", workspaceID: "workspace-2", status: model.AgentRunStatusPaused, pauseReason: model.AgentRunPauseReasonHumanInput},
	}
	for _, row := range rows {
		seedAgentRunListTestRow(t, db, row.id, row.workspaceID, "agent-1", row.status, row.pauseReason, `{}`)
	}

	count, err := repo.CountWorkspaceRunsRequiringAttention(context.Background(), "workspace-1")
	if err != nil {
		t.Fatalf("CountWorkspaceRunsRequiringAttention: %v", err)
	}
	if count != 4 {
		t.Fatalf("attention count = %d, want 4", count)
	}
}

func TestAgentRunListPagination(t *testing.T) {
	page, perPage := agentRunListPagination(model.PMPagination{Page: -1, PerPage: 5000})
	if page != 1 {
		t.Fatalf("page = %d, want 1", page)
	}
	if perPage != _agentRunListMaxPageSize {
		t.Fatalf("perPage = %d, want %d", perPage, _agentRunListMaxPageSize)
	}

	page, perPage = agentRunListPagination(model.PMPagination{})
	if page != 1 {
		t.Fatalf("default page = %d, want 1", page)
	}
	if perPage != _agentRunListDefaultPageSize {
		t.Fatalf("default perPage = %d, want %d", perPage, _agentRunListDefaultPageSize)
	}
}

func openAgentRunListTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := openAgentVersionColumnCompatDB(t)
	if err := db.Exec(`ALTER TABLE agent_runs ADD COLUMN agent_version_id TEXT`).Error; err != nil {
		t.Fatalf("add agent_version_id column: %v", err)
	}
	return db
}

func seedAgentRunListTestRow(t *testing.T, db *gorm.DB, id, workspaceID, agentID, status, pauseReason, outputSummary string) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Exec(
		`INSERT INTO agent_runs (id, workspace_id, agent_id, target_type, target_id, runtime_kind, invocation_mode, approval_state, pause_reason, status, input, output_summary, created_at, updated_at)
		 VALUES (?, ?, ?, 'workspace', ?, 'codex', 'autonomous', 'not_required', ?, ?, ?, ?, ?, ?)`,
		id,
		workspaceID,
		agentID,
		workspaceID,
		pauseReason,
		status,
		[]byte(`{"trigger":{"source":"manual"}}`),
		[]byte(outputSummary),
		now,
		now,
	).Error; err != nil {
		t.Fatalf("seed agent run %q: %v", id, err)
	}
}
