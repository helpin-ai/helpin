package repository

import (
	"context"
	"fmt"
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

func TestAgentRunRepositorySummarizeFleetSinceScopesAndAggregates(t *testing.T) {
	db := openAgentRunListTestDB(t)
	repo := NewAgentRunRepository(db)
	now := time.Now().UTC()
	since := now.Add(-7 * 24 * time.Hour)

	rows := []struct {
		id          string
		workspaceID string
		agentID     string
		status      string
		tokens      int
		createdAt   time.Time
	}{
		{id: "completed", workspaceID: "workspace-1", agentID: "agent-1", status: model.AgentRunStatusCompleted, tokens: 120, createdAt: now.Add(-time.Hour)},
		{id: "failed", workspaceID: "workspace-1", agentID: "agent-1", status: model.AgentRunStatusFailed, tokens: 30, createdAt: now.Add(-2 * time.Hour)},
		{id: "running", workspaceID: "workspace-1", agentID: "agent-2", status: model.AgentRunStatusRunning, tokens: 10, createdAt: now.Add(-3 * time.Hour)},
		{id: "old", workspaceID: "workspace-1", agentID: "agent-1", status: model.AgentRunStatusFailed, tokens: 999, createdAt: since.Add(-time.Hour)},
		{id: "hidden-agent", workspaceID: "workspace-1", agentID: "agent-3", status: model.AgentRunStatusCompleted, tokens: 999, createdAt: now},
		{id: "other-workspace", workspaceID: "workspace-2", agentID: "agent-1", status: model.AgentRunStatusCompleted, tokens: 999, createdAt: now},
	}
	for _, row := range rows {
		seedAgentRunListTestRow(t, db, row.id, row.workspaceID, row.agentID, row.status, model.AgentRunPauseReasonNone, `{}`)
		if err := db.Model(&model.AgentRun{}).Where("id = ?", row.id).Updates(map[string]any{
			"tokens_used": row.tokens,
			"created_at":  row.createdAt,
			"updated_at":  row.createdAt,
		}).Error; err != nil {
			t.Fatalf("configure fleet run %q: %v", row.id, err)
		}
	}

	aggregates, err := repo.SummarizeFleetSince(context.Background(), "workspace-1", []string{"agent-1", "agent-2"}, since)
	if err != nil {
		t.Fatalf("SummarizeFleetSince: %v", err)
	}
	byAgent := make(map[string]AgentRunFleetAggregate, len(aggregates))
	for _, aggregate := range aggregates {
		byAgent[aggregate.AgentID] = aggregate
	}
	if got := byAgent["agent-1"]; got.RecentRuns != 2 || got.RecentCompleted != 1 || got.RecentFailed != 1 || got.RecentTokens != 150 {
		t.Fatalf("agent-1 aggregate = %+v, want runs=2 completed=1 failed=1 tokens=150", got)
	}
	if got := byAgent["agent-2"]; got.RecentRuns != 1 || got.RecentCompleted != 0 || got.RecentFailed != 0 || got.RecentTokens != 10 {
		t.Fatalf("agent-2 aggregate = %+v, want runs=1 tokens=10", got)
	}
}

func TestAgentRunRepositoryListRecentByAgentIDsLimitsPerAgent(t *testing.T) {
	db := openAgentRunListTestDB(t)
	repo := NewAgentRunRepository(db)
	base := time.Now().UTC().Add(-time.Hour)

	for _, agentID := range []string{"agent-1", "agent-2"} {
		for idx := 0; idx < 7; idx++ {
			id := fmt.Sprintf("%s-run-%d", agentID, idx)
			seedAgentRunListTestRow(t, db, id, "workspace-1", agentID, model.AgentRunStatusCompleted, model.AgentRunPauseReasonNone, `{}`)
			createdAt := base.Add(time.Duration(idx) * time.Minute)
			if err := db.Model(&model.AgentRun{}).Where("id = ?", id).Updates(map[string]any{
				"created_at": createdAt,
				"updated_at": createdAt,
			}).Error; err != nil {
				t.Fatalf("set recent run timestamp: %v", err)
			}
		}
	}
	seedAgentRunListTestRow(t, db, "other-workspace", "workspace-2", "agent-1", model.AgentRunStatusCompleted, model.AgentRunPauseReasonNone, `{}`)

	runs, err := repo.ListRecentByAgentIDs(context.Background(), "workspace-1", []string{"agent-1", "agent-2"}, 5)
	if err != nil {
		t.Fatalf("ListRecentByAgentIDs: %v", err)
	}
	byAgent := map[string][]string{}
	for _, run := range runs {
		byAgent[run.AgentID] = append(byAgent[run.AgentID], run.ID)
	}
	for _, agentID := range []string{"agent-1", "agent-2"} {
		if len(byAgent[agentID]) != 5 {
			t.Fatalf("%s recent run count = %d, want 5", agentID, len(byAgent[agentID]))
		}
		if want := agentID + "-run-6"; byAgent[agentID][0] != want {
			t.Fatalf("%s first recent run = %q, want %q", agentID, byAgent[agentID][0], want)
		}
	}
}

func TestAgentRunRepositoryListDockRunsForActorScopesAndRetainsActiveRuns(t *testing.T) {
	db := openAgentRunListTestDB(t)
	repo := NewAgentRunRepository(db)
	cutoff := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	old := cutoff.Add(-time.Hour)
	recent := cutoff.Add(time.Hour)

	rows := []struct {
		id          string
		workspaceID string
		status      string
		actorID     string
		updatedAt   time.Time
		dockChatID  *string
	}{
		{id: "active-old", workspaceID: "workspace-1", status: model.AgentRunStatusRunning, actorID: "user-1", updatedAt: old},
		{id: "terminal-recent", workspaceID: "workspace-1", status: model.AgentRunStatusCompleted, actorID: "user-1", updatedAt: recent},
		{id: "terminal-old", workspaceID: "workspace-1", status: model.AgentRunStatusCompleted, actorID: "user-1", updatedAt: old},
		{id: "other-actor", workspaceID: "workspace-1", status: model.AgentRunStatusRunning, actorID: "user-2", updatedAt: recent},
		{id: "other-workspace", workspaceID: "workspace-2", status: model.AgentRunStatusRunning, actorID: "user-1", updatedAt: recent},
	}
	chatID := "chat-1"
	rows = append(rows, struct {
		id          string
		workspaceID string
		status      string
		actorID     string
		updatedAt   time.Time
		dockChatID  *string
	}{id: "chat-backing", workspaceID: "workspace-1", status: model.AgentRunStatusRunning, actorID: "user-1", updatedAt: recent, dockChatID: &chatID})

	for _, row := range rows {
		seedAgentRunListTestRow(t, db, row.id, row.workspaceID, "agent-1", row.status, model.AgentRunPauseReasonNone, `{}`)
		if err := db.Model(&model.AgentRun{}).Where("id = ?", row.id).Updates(map[string]any{
			"triggered_by_user_id": row.actorID,
			"dock_chat_id":         row.dockChatID,
			"created_at":           row.updatedAt,
			"updated_at":           row.updatedAt,
		}).Error; err != nil {
			t.Fatalf("configure dock run %q: %v", row.id, err)
		}
	}

	runs, err := repo.ListDockRunsForActor(context.Background(), "workspace-1", "user-1", cutoff, 100)
	if err != nil {
		t.Fatalf("ListDockRunsForActor: %v", err)
	}
	got := make([]string, 0, len(runs))
	for _, run := range runs {
		got = append(got, run.ID)
	}
	if want := []string{"terminal-recent", "active-old"}; !slices.Equal(got, want) {
		t.Fatalf("dock run ids = %v, want %v", got, want)
	}

	active, err := repo.ListActiveDockRunsForActor(context.Background(), "workspace-1", "user-1")
	if err != nil || len(active) != 1 || active[0].ID != "active-old" {
		t.Fatalf("active dock runs = %#v, err=%v", active, err)
	}
	settled, err := repo.ListSettledDockRunsForActor(context.Background(), "workspace-1", "user-1", 1, nil, "")
	if err != nil || len(settled) != 1 || settled[0].ID != "terminal-recent" {
		t.Fatalf("first settled page = %#v, err=%v", settled, err)
	}
	before := settled[0].UpdatedAt
	settled, err = repo.ListSettledDockRunsForActor(context.Background(), "workspace-1", "user-1", 1, &before, settled[0].ID)
	if err != nil || len(settled) != 1 || settled[0].ID != "terminal-old" {
		t.Fatalf("second settled page = %#v, err=%v", settled, err)
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
	if err := db.Exec(`CREATE TABLE dock_chats (id TEXT PRIMARY KEY, workspace_id TEXT, user_id TEXT, active_run_id TEXT, archived_at DATETIME)`).Error; err != nil {
		t.Fatal(err)
	}
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
