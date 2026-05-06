package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestPMSprintHandler_PlanningWorkspace(t *testing.T) {
	t.Parallel()

	db := newPMSprintPlanningHandlerTestDB(t)
	now := time.Now().UTC()

	const (
		workspaceID = "ws-handler-planning"
		teamID      = "team-handler-planning"
		workflowID  = "wf-handler-planning"
		todoStateID = "state-handler-todo"
	)

	if err := db.Exec(
		`INSERT INTO pm_sprints (id, workspace_id, name, start_date, end_date, team_id, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		"sprint-handler-active", workspaceID, "Handler active sprint", now.AddDate(0, 0, -1), now.AddDate(0, 0, 7), teamID, now, now,
	).Error; err != nil {
		t.Fatalf("seed sprint: %v", err)
	}
	if err := db.Exec(
		`INSERT INTO pm_workflow_states (id, workflow_id, name, state_type, position, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 0, ?, ?)`,
		todoStateID, workflowID, "Todo", model.PMStateTypeUnstarted, 0, now, now,
	).Error; err != nil {
		t.Fatalf("seed state: %v", err)
	}
	if err := db.Exec(
		`INSERT INTO pm_tasks (id, workspace_id, display_id, name, workflow_id, workflow_state_id, sprint_id, team_id, estimate, position, priority, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		"story-handler-backlog", workspaceID, 3001, "Backlog handler story", workflowID, todoStateID, nil, teamID, 3, 1, model.PMTaskPriorityMedium, now, now,
	).Error; err != nil {
		t.Fatalf("seed story: %v", err)
	}

	sprintRepo := repository.NewPMSprintRepository(db)
	labelRepo := repository.NewPMLabelRepository(db)
	activityRepo := repository.NewPMActivityRepository(db)
	activityService := service.NewPMActivityService(activityRepo)
	sprintService := service.NewPMSprintService(sprintRepo, labelRepo, repository.NewPMAttachmentRepository(db), repository.NewWorkspaceRepository(db), nil, activityService, nil, nil, nil)
	handler := NewPMSprintHandler(sprintService)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/pm/sprints/planning?workspace_id=%s&team_id=%s", workspaceID, teamID), nil)
	rr := httptest.NewRecorder()

	handler.PlanningWorkspace(rr, req.WithContext(context.Background()))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rr.Code, rr.Body.String())
	}

	var payload model.SprintPlanningWorkspace
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Buckets) != 3 {
		t.Fatalf("buckets = %d, want 3", len(payload.Buckets))
	}
	if payload.Buckets[0].Key != "active" {
		t.Fatalf("bucket[0].key = %q, want active", payload.Buckets[0].Key)
	}
	if payload.BacklogTotal != 1 {
		t.Fatalf("backlog_total = %d, want 1", payload.BacklogTotal)
	}
}

func newPMSprintPlanningHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:pm-sprint-handler-planning-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	statements := []string{
		`CREATE TABLE pm_sprints (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			external_id TEXT,
			start_date DATETIME,
			end_date DATETIME,
			team_id TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_tasks (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			task_type TEXT NOT NULL DEFAULT 'feature',
			workflow_id TEXT NOT NULL,
			workflow_state_id TEXT NOT NULL,
			epic_id TEXT,
			sprint_id TEXT,
			team_id TEXT,
			owner_id TEXT,
			requester_id TEXT,
			requester_member_id TEXT,
			estimate INTEGER,
			priority TEXT NOT NULL DEFAULT 'none',
			severity TEXT NOT NULL DEFAULT 'none',
			deadline DATETIME,
			position INTEGER NOT NULL DEFAULT 0,
			started BOOLEAN NOT NULL DEFAULT 0,
			started_at DATETIME,
			completed BOOLEAN NOT NULL DEFAULT 0,
			completed_at DATETIME,
			moved_at DATETIME,
			blocked BOOLEAN NOT NULL DEFAULT 0,
			blocker TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			assigned_agent_id TEXT,
			plan_document_id TEXT,
			template_id TEXT,
			recurring_template_id TEXT,
			recurring_run_id TEXT,
			recurring_occurrence_number INTEGER,
			external_id TEXT,
			slice_type TEXT,
			implementation_brief TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE workspace_members (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			display_name TEXT,
			email TEXT,
			role TEXT,
			status TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_task_owners (
			task_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (task_id, user_id)
		)`,
		`CREATE TABLE pm_workflow_states (
			id TEXT PRIMARY KEY,
			workflow_id TEXT NOT NULL,
			name TEXT NOT NULL,
			state_type TEXT NOT NULL,
			position INTEGER NOT NULL DEFAULT 0,
			color TEXT,
			description TEXT,
			wip_limit INTEGER,
			is_default BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}

	return db
}
