package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type failingPlanningWorkflowStarter struct {
	err error
}

func (f failingPlanningWorkflowStarter) StartPlanningSession(ctx context.Context, sessionID string) error {
	return f.err
}

func (f failingPlanningWorkflowStarter) SignalPlanningMessage(ctx context.Context, sessionID string) error {
	return nil
}

func (f failingPlanningWorkflowStarter) SignalPlanningFinalize(ctx context.Context, sessionID, actorID string) error {
	return nil
}

func (f failingPlanningWorkflowStarter) SignalPlanningAbandon(ctx context.Context, sessionID string) error {
	return nil
}

func (f failingPlanningWorkflowStarter) SignalFlowChildState(ctx context.Context, flowRunID, nodeRunID, childType, childID, childStatus string) error {
	return nil
}

func TestPlanningSessionServiceStartSessionRollsBackFailedWorkflowStart(t *testing.T) {
	db := newTestDB(t)
	mustExec(t, db, `CREATE TABLE agents (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		name TEXT NOT NULL,
		agent_class TEXT NOT NULL,
		role TEXT,
		status TEXT NOT NULL DEFAULT 'idle',
		runtime_kind TEXT NOT NULL,
		capability_profile TEXT NOT NULL,
		skills BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
		trigger_mode TEXT NOT NULL DEFAULT 'manual',
		provider TEXT,
		model TEXT,
		system_prompt TEXT,
		planning_notes TEXT,
		monthly_token_budget INTEGER,
		tokens_used_this_month INTEGER NOT NULL DEFAULT 0,
		active_story_id TEXT,
		team_id TEXT,
		allowed_tools BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
		allowed_commands BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
		allowed_targets BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
		schedule TEXT,
		approval_mode TEXT NOT NULL DEFAULT 'class_default',
		max_concurrent_runs INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME,
		updated_at DATETIME
	)`)
	mustExec(t, db, `CREATE TABLE planning_sessions (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		workspace_id TEXT NOT NULL,
		epic_id TEXT NOT NULL,
		flow_run_id TEXT,
		flow_node_run_id TEXT,
		agent_id TEXT NOT NULL,
		status TEXT NOT NULL,
		planning_methodology TEXT NOT NULL,
		allowed_tools BLOB NOT NULL,
		spec_document_id TEXT,
		stage TEXT NOT NULL,
		spec_draft TEXT NOT NULL DEFAULT '',
		plan_draft TEXT NOT NULL DEFAULT '',
		spec_sections BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
		context_snapshot BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
		token_usage BLOB NOT NULL DEFAULT (CAST('{\"input\":0,\"output\":0}' AS BLOB)),
		started_by TEXT,
		started_at DATETIME,
		last_active_at DATETIME,
		completed_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME
	)`)

	const (
		userID  = "user-1"
		wsID    = "ws-1"
		epicID  = "epic-1"
		agentID = "agent-1"
	)

	seedUser(t, db, userID, "owner@example.com", "Owner", "hash")
	seedWorkspace(t, db, wsID, "Workspace", "workspace", userID)
	seedWorkspaceMember(t, db, "member-1", wsID, userID, "owner@example.com", "Owner", "owner")
	seedSettings(t, db, "settings-1", wsID)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO pm_epics (id, workspace_id, name, planning_state, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		epicID, wsID, "Epic", model.EpicPlanningStateNotStarted, now, now)
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, name, agent_class, role, status, runtime_kind, capability_profile,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		agentID, wsID, "Planner", model.AgentClassProductPlanner, "planner", "idle", "native_sdk", model.AgentClassProductPlanner,
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "class_default", 1, now, now)

	sessionRepo := repository.NewPlanningSessionRepository(db)
	epicRepo := repository.NewPMEpicRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)

	svc := NewPlanningSessionService(
		sessionRepo,
		epicRepo,
		agentRepo,
		settingsRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	svc.SetWorkflowStarter(failingPlanningWorkflowStarter{err: errors.New("temporal unavailable")})

	_, err := svc.StartSession(context.Background(), wsID, epicID, userID, model.StartPlanningSessionRequest{
		AgentID: agentID,
	})
	if err == nil {
		t.Fatal("expected start session to fail")
	}

	active, err := sessionRepo.GetActiveByEpicID(context.Background(), epicID)
	if err != nil {
		t.Fatalf("get active session: %v", err)
	}
	if active != nil {
		t.Fatalf("expected no active session after rollback, got %s", active.ID)
	}

	var sessions []model.PlanningSession
	if err := db.Find(&sessions).Error; err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 persisted session, got %d", len(sessions))
	}
	if sessions[0].Status != model.PlanningSessionStatusAbandoned {
		t.Fatalf("session status = %q, want %q", sessions[0].Status, model.PlanningSessionStatusAbandoned)
	}
	if sessions[0].CompletedAt == nil {
		t.Fatal("expected abandoned session to be completed")
	}

	epicWithStats, err := epicRepo.GetByID(context.Background(), epicID)
	if err != nil {
		t.Fatalf("get epic: %v", err)
	}
	if epicWithStats == nil {
		t.Fatal("expected epic to exist")
	}
	if epicWithStats.Epic.PlanningState != model.EpicPlanningStateNotStarted {
		t.Fatalf("planning_state = %q, want %q", epicWithStats.Epic.PlanningState, model.EpicPlanningStateNotStarted)
	}
	if epicWithStats.Epic.ActivePlanningSessionID != nil {
		t.Fatalf("expected active_planning_session_id to be cleared, got %s", *epicWithStats.Epic.ActivePlanningSessionID)
	}
}
