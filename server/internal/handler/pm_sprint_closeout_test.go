package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestPMSprintHandler_GetCloseout(t *testing.T) {
	t.Parallel()

	db := newPMSprintCloseoutHandlerTestDB(t)
	now := time.Now().UTC()

	const (
		workspaceID    = "ws-handler-closeout"
		teamID         = "team-handler-closeout"
		sourceSprintID = "sprint-handler-source"
		targetSprintID = "sprint-handler-target"
	)

	mustExecHandler(t, db,
		`INSERT INTO workspace_teams (id, workspace_id, name, sprints_enabled, created_at, updated_at) VALUES (?, ?, ?, 1, ?, ?)`,
		teamID, workspaceID, "Growth", now, now,
	)
	mustExecHandler(t, db,
		`INSERT INTO pm_sprints (id, workspace_id, name, start_date, end_date, team_id, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		sourceSprintID, workspaceID, "Sprint 12", now.AddDate(0, 0, -14), now.AddDate(0, 0, -7), teamID, now, now,
	)
	mustExecHandler(t, db,
		`INSERT INTO pm_sprints (id, workspace_id, name, start_date, end_date, team_id, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		targetSprintID, workspaceID, "Sprint 13", now.AddDate(0, 0, -1), now.AddDate(0, 0, 7), teamID, now, now,
	)
	mustExecHandler(t, db,
		`INSERT INTO pm_sprint_closeouts (id, sprint_id, workspace_id, team_id, rolled_to_sprint_id, committed_count, completed_count, unfinished_count, rolled_over_count, committed_points, completed_points, unfinished_points, rolled_over_points, closed_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"closeout-handler-source", sourceSprintID, workspaceID, teamID, targetSprintID, 7, 4, 3, 3, 21, 12, 9, 9, now.AddDate(0, 0, -7), now, now,
	)

	sprintService := newPMSprintCloseoutHandlerService(db)
	handler := NewPMSprintHandler(sprintService)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/pm/sprints/%s/closeout?workspace_id=%s", targetSprintID, workspaceID), nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", targetSprintID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	handler.GetCloseout(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rr.Code, rr.Body.String())
	}

	var payload model.SprintCloseoutResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Closeout != nil {
		t.Fatal("expected target sprint to have no direct closeout")
	}
	if len(payload.RolledInFrom) != 1 {
		t.Fatalf("rolled_in_from len = %d, want 1", len(payload.RolledInFrom))
	}
	if payload.RolledInFrom[0].SourceSprintID != sourceSprintID {
		t.Fatalf("source_sprint_id = %q, want %q", payload.RolledInFrom[0].SourceSprintID, sourceSprintID)
	}
}

func TestPMSprintHandler_ListCloseouts(t *testing.T) {
	t.Parallel()

	db := newPMSprintCloseoutHandlerTestDB(t)
	now := time.Now().UTC()

	const (
		workspaceID = "ws-handler-closeouts"
		teamAID     = "team-handler-closeouts-a"
		teamBID     = "team-handler-closeouts-b"
	)

	mustExecHandler(t, db,
		`INSERT INTO workspace_teams (id, workspace_id, name, sprints_enabled, created_at, updated_at) VALUES (?, ?, ?, 1, ?, ?)`,
		teamAID, workspaceID, "Alpha", now, now,
	)
	mustExecHandler(t, db,
		`INSERT INTO workspace_teams (id, workspace_id, name, sprints_enabled, created_at, updated_at) VALUES (?, ?, ?, 1, ?, ?)`,
		teamBID, workspaceID, "Beta", now, now,
	)
	mustExecHandler(t, db,
		`INSERT INTO pm_sprints (id, workspace_id, name, start_date, end_date, team_id, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		"sprint-handler-alpha", workspaceID, "Sprint Alpha", now.AddDate(0, 0, -14), now.AddDate(0, 0, -7), teamAID, now, now,
	)
	mustExecHandler(t, db,
		`INSERT INTO pm_sprints (id, workspace_id, name, start_date, end_date, team_id, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		"sprint-handler-beta", workspaceID, "Sprint Beta", now.AddDate(0, 0, -28), now.AddDate(0, 0, -21), teamBID, now, now,
	)
	mustExecHandler(t, db,
		`INSERT INTO pm_sprint_closeouts (id, sprint_id, workspace_id, team_id, committed_count, completed_count, unfinished_count, rolled_over_count, committed_points, completed_points, unfinished_points, rolled_over_points, closed_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"closeout-handler-alpha", "sprint-handler-alpha", workspaceID, teamAID, 5, 3, 2, 1, 10, 6, 4, 2, now.AddDate(0, 0, -7), now, now,
	)
	mustExecHandler(t, db,
		`INSERT INTO pm_sprint_closeouts (id, sprint_id, workspace_id, team_id, committed_count, completed_count, unfinished_count, rolled_over_count, committed_points, completed_points, unfinished_points, rolled_over_points, closed_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"closeout-handler-beta", "sprint-handler-beta", workspaceID, teamBID, 6, 4, 2, 0, 12, 8, 4, 0, now.AddDate(0, 0, -21), now, now,
	)

	sprintService := newPMSprintCloseoutHandlerService(db)
	handler := NewPMSprintHandler(sprintService)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/pm/sprints/closeouts?workspace_id=%s&team_id=%s", workspaceID, teamAID), nil)
	rr := httptest.NewRecorder()

	handler.ListCloseouts(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rr.Code, rr.Body.String())
	}

	var payload model.SprintCloseoutListResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(payload.Items))
	}
	if payload.Items[0].TeamID == nil || *payload.Items[0].TeamID != teamAID {
		t.Fatalf("team_id = %v, want %q", payload.Items[0].TeamID, teamAID)
	}
}

func newPMSprintCloseoutHandlerService(db *gorm.DB) *service.PMSprintService {
	return service.NewPMSprintService(
		repository.NewPMSprintRepository(db),
		repository.NewPMTaskRepository(db),
		repository.NewPMLabelRepository(db),
		repository.NewPMAttachmentRepository(db),
		repository.NewWorkspaceRepository(db),
		nil,
		service.NewPMActivityService(repository.NewPMActivityRepository(db)),
		nil,
		nil,
		repository.NewPMSprintCloseoutRepository(db),
	)
}

func newPMSprintCloseoutHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:pm-sprint-closeout-handler-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	statements := []string{
		`CREATE TABLE workspace_teams (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			sprints_enabled BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_sprints (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			start_date DATETIME,
			end_date DATETIME,
			team_id TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_sprint_closeouts (
			id TEXT PRIMARY KEY,
			sprint_id TEXT NOT NULL UNIQUE,
			workspace_id TEXT NOT NULL,
			team_id TEXT,
			rolled_to_sprint_id TEXT,
			committed_count INTEGER NOT NULL DEFAULT 0,
			completed_count INTEGER NOT NULL DEFAULT 0,
			unfinished_count INTEGER NOT NULL DEFAULT 0,
			rolled_over_count INTEGER NOT NULL DEFAULT 0,
			committed_points INTEGER NOT NULL DEFAULT 0,
			completed_points INTEGER NOT NULL DEFAULT 0,
			unfinished_points INTEGER NOT NULL DEFAULT 0,
			rolled_over_points INTEGER NOT NULL DEFAULT 0,
			closed_at DATETIME NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_sprint_closeout_tasks (
			id TEXT PRIMARY KEY,
			closeout_id TEXT NOT NULL,
			task_id TEXT NOT NULL,
			outcome TEXT NOT NULL,
			estimate INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME
		)`,
		`CREATE TABLE pm_labels (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			color TEXT,
			team_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_sprint_labels (
			sprint_id TEXT NOT NULL,
			label_id TEXT NOT NULL
		)`,
		`CREATE TABLE pm_tasks (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			workflow_id TEXT,
			workflow_state_id TEXT,
			sprint_id TEXT,
			team_id TEXT,
			estimate INTEGER,
			completed BOOLEAN NOT NULL DEFAULT 0,
			archived BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_workflow_states (
			id TEXT PRIMARY KEY,
			workflow_id TEXT NOT NULL,
			name TEXT NOT NULL,
			state_type TEXT NOT NULL,
			position INTEGER NOT NULL DEFAULT 0,
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

func mustExecHandler(t *testing.T, db *gorm.DB, query string, args ...any) {
	t.Helper()
	if err := db.Exec(query, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}
