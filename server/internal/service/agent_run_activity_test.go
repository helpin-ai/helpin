package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
	"gorm.io/gorm"
)

func setupAgentRunActivityTest(t *testing.T) (*AgentService, *gormTestDB) {
	t.Helper()
	db := newTestDB(t)
	createAgentRunActivityTables(t, db)
	svc := &AgentService{
		agentRepo:      repository.NewAgentRepository(db),
		runRepo:        repository.NewAgentRunRepository(db),
		runMessageRepo: repository.NewAgentRunMessageRepository(db),
		activitySvc:    NewPMActivityService(repository.NewPMActivityRepository(db)),
		runEngine:      &temporalapp.RunEngine{},
	}
	return svc, &gormTestDB{DB: db}
}

type gormTestDB struct {
	*gorm.DB
}

func createAgentRunActivityTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	tables := []string{
		`CREATE TABLE agents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			preset_key TEXT,
			preset_version_key TEXT,
			source_preset_key TEXT,
			source_preset_version_key TEXT,
			source_template_id TEXT,
			source_template_key TEXT,
			template_key TEXT,
			template_instance_id TEXT,
			template_version INTEGER,
			active_version_id TEXT,
			role TEXT,
			status TEXT NOT NULL DEFAULT 'idle',
			runtime_kind TEXT NOT NULL DEFAULT 'native_sdk',
			skills BLOB NOT NULL DEFAULT '[]',
			trigger_mode TEXT NOT NULL DEFAULT 'manual',
			provider TEXT,
			model TEXT,
			execution_config BLOB NOT NULL DEFAULT '{}',
			system_prompt TEXT,
			instruction_template_version TEXT NOT NULL DEFAULT '',
			planning_notes TEXT,
			monthly_token_budget INTEGER,
			tokens_used_this_month INTEGER NOT NULL DEFAULT 0,
			active_task_id TEXT,
			team_id TEXT,
			allowed_tools BLOB NOT NULL DEFAULT '[]',
			allowed_commands BLOB NOT NULL DEFAULT '[]',
			allowed_targets BLOB NOT NULL DEFAULT '[]',
			approval_mode TEXT NOT NULL DEFAULT 'preset_default',
			max_concurrent_runs INTEGER NOT NULL DEFAULT 1,
			default_invocation_mode TEXT NOT NULL DEFAULT 'interactive',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_runs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			task_id TEXT,
			conversation_id TEXT,
			target_type TEXT NOT NULL DEFAULT 'task',
			target_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL DEFAULT 'native_sdk',
			invocation_mode TEXT NOT NULL DEFAULT 'interactive',
			parent_run_id TEXT,
			handoff_state TEXT,
			approval_state TEXT NOT NULL DEFAULT 'not_required',
			pause_reason TEXT NOT NULL DEFAULT 'none',
			triggered_by_user_id TEXT,
			status TEXT NOT NULL DEFAULT 'queued',
			workflow_id TEXT,
			workflow_run_id TEXT,
			task_queue TEXT,
			runner_pool TEXT,
			agent_version_id TEXT,
			repository_id TEXT,
			repo_full_name TEXT,
			base_branch TEXT,
			working_branch TEXT,
			delivery_target_id TEXT,
			execution_stage TEXT,
			last_heartbeat_at DATETIME,
			input BLOB NOT NULL DEFAULT '{}',
			output_summary BLOB NOT NULL DEFAULT '{}',
			cached_input_tokens INTEGER NOT NULL DEFAULT 0,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			tokens_used INTEGER NOT NULL DEFAULT 0,
			error_message TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_run_messages (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			message_type TEXT NOT NULL DEFAULT 'message',
			content_blocks BLOB,
			turn_segments BLOB,
			tool_invocations BLOB,
			token_usage BLOB,
			sequence_no INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME
		)`,
	}
	for _, table := range tables {
		if err := db.Exec(table).Error; err != nil {
			t.Fatalf("create agent run activity test table: %v", err)
		}
	}
}

func seedAgentRunActivityAgent(t *testing.T, db *gorm.DB, workspaceID, agentID string) {
	t.Helper()
	if err := db.Create(&model.Agent{
		ID:                    agentID,
		WorkspaceID:           workspaceID,
		Name:                  "Atlas",
		RuntimeKind:           "native_sdk",
		Status:                "idle",
		AllowedTargets:        json.RawMessage(`["task","epic"]`),
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		DefaultInvocationMode: model.InvocationModeInteractive,
	}).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
}

func seedAgentRunActivityRun(t *testing.T, db *gorm.DB, run *model.AgentRun) {
	t.Helper()
	if run.RuntimeKind == "" {
		run.RuntimeKind = "native_sdk"
	}
	if run.InvocationMode == "" {
		run.InvocationMode = model.InvocationModeInteractive
	}
	if run.ApprovalState == "" {
		run.ApprovalState = "not_required"
	}
	if run.PauseReason == "" {
		run.PauseReason = model.AgentRunPauseReasonNone
	}
	if len(run.Input) == 0 {
		run.Input = json.RawMessage(`{}`)
	}
	if len(run.OutputSummary) == 0 {
		run.OutputSummary = json.RawMessage(`{}`)
	}
	if err := db.Create(run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}
}

func latestAgentRunActivity(t *testing.T, db *gorm.DB, entityType, entityID string) model.PMActivityLog {
	t.Helper()
	var activity model.PMActivityLog
	if err := db.Where("entity_type = ? AND entity_id = ? AND field_name = ?", entityType, entityID, "agent_run").
		Order("created_at DESC").
		First(&activity).Error; err != nil {
		t.Fatalf("find agent run activity: %v", err)
	}
	return activity
}

func TestCancelRunLogsCancellingActorForTaskActivity(t *testing.T) {
	svc, testDB := setupAgentRunActivityTest(t)
	ctx := context.Background()
	workspaceID := "ws-agent-activity"
	agentID := "agent-atlas"
	taskID := "task-1"
	runID := "run-cancel"
	actorID := "user-cancel"

	seedAgentRunActivityAgent(t, testDB.DB, workspaceID, agentID)
	seedAgentRunActivityRun(t, testDB.DB, &model.AgentRun{
		ID:          runID,
		WorkspaceID: workspaceID,
		AgentID:     agentID,
		TaskID:      &taskID,
		TargetType:  "task",
		TargetID:    taskID,
		Status:      model.AgentRunStatusRunning,
	})

	if _, err := svc.CancelRun(ctx, workspaceID, runID, actorID); err != nil {
		t.Fatalf("CancelRun: %v", err)
	}

	activity := latestAgentRunActivity(t, testDB.DB, "task", taskID)
	if activity.ActorID == nil || *activity.ActorID != actorID {
		t.Fatalf("expected actor %q, got %#v", actorID, activity.ActorID)
	}
	if activity.NewValue == nil || *activity.NewValue != "cancelled" {
		t.Fatalf("expected cancelled activity, got %#v", activity.NewValue)
	}
}

func TestApproveRunLogsApprovingActorForEpicActivity(t *testing.T) {
	svc, testDB := setupAgentRunActivityTest(t)
	ctx := context.Background()
	workspaceID := "ws-agent-activity"
	agentID := "agent-atlas"
	epicID := "epic-1"
	runID := "run-approve"
	actorID := "user-approve"

	seedAgentRunActivityAgent(t, testDB.DB, workspaceID, agentID)
	heartbeat := time.Now()
	seedAgentRunActivityRun(t, testDB.DB, &model.AgentRun{
		ID:              runID,
		WorkspaceID:     workspaceID,
		AgentID:         agentID,
		TargetType:      "epic",
		TargetID:        epicID,
		Status:          model.AgentRunStatusPaused,
		PauseReason:     model.AgentRunPauseReasonHumanApproval,
		ApprovalState:   "pending",
		LastHeartbeatAt: &heartbeat,
	})

	if _, err := svc.ApproveRun(ctx, workspaceID, runID, actorID, model.ApproveAgentRunRequest{}); err != nil {
		t.Fatalf("ApproveRun: %v", err)
	}

	activity := latestAgentRunActivity(t, testDB.DB, "epic", epicID)
	if activity.ActorID == nil || *activity.ActorID != actorID {
		t.Fatalf("expected actor %q, got %#v", actorID, activity.ActorID)
	}
	if activity.NewValue == nil || *activity.NewValue != "approved" {
		t.Fatalf("expected approved activity, got %#v", activity.NewValue)
	}
}

func TestRequestRunChangesLogsActorForTaskActivity(t *testing.T) {
	svc, testDB := setupAgentRunActivityTest(t)
	ctx := context.Background()
	workspaceID := "ws-agent-activity"
	agentID := "agent-atlas"
	taskID := "task-1"
	runID := "run-changes"
	actorID := "user-changes"

	seedAgentRunActivityAgent(t, testDB.DB, workspaceID, agentID)
	seedAgentRunActivityRun(t, testDB.DB, &model.AgentRun{
		ID:            runID,
		WorkspaceID:   workspaceID,
		AgentID:       agentID,
		TaskID:        &taskID,
		TargetType:    "task",
		TargetID:      taskID,
		Status:        model.AgentRunStatusPaused,
		PauseReason:   model.AgentRunPauseReasonHumanApproval,
		ApprovalState: "pending",
	})

	if _, err := svc.RequestRunChanges(ctx, workspaceID, runID, actorID, model.SendAgentRunRequestChangesRequest{Content: "Please simplify the plan."}); err != nil {
		t.Fatalf("RequestRunChanges: %v", err)
	}

	activity := latestAgentRunActivity(t, testDB.DB, "task", taskID)
	if activity.NewValue == nil || *activity.NewValue != "changes_requested" {
		t.Fatalf("expected changes_requested activity, got %#v", activity.NewValue)
	}
}

func TestSendRunMessageLogsNoteSnippetForTaskActivity(t *testing.T) {
	svc, testDB := setupAgentRunActivityTest(t)
	ctx := context.Background()
	workspaceID := "ws-agent-activity"
	agentID := "agent-atlas"
	taskID := "task-1"
	runID := "run-note"
	actorID := "user-note"
	content := "Please keep the implementation focused on the smallest possible change and avoid broad refactors."

	seedAgentRunActivityAgent(t, testDB.DB, workspaceID, agentID)
	seedAgentRunActivityRun(t, testDB.DB, &model.AgentRun{
		ID:          runID,
		WorkspaceID: workspaceID,
		AgentID:     agentID,
		TaskID:      &taskID,
		TargetType:  "task",
		TargetID:    taskID,
		Status:      model.AgentRunStatusPaused,
		PauseReason: model.AgentRunPauseReasonHumanInput,
	})

	if _, err := svc.SendRunMessage(ctx, workspaceID, runID, actorID, model.SendAgentRunMessageRequest{Content: content}); err != nil {
		t.Fatalf("SendRunMessage: %v", err)
	}

	activity := latestAgentRunActivity(t, testDB.DB, "task", taskID)
	if activity.NewValue == nil || *activity.NewValue != "note_added" {
		t.Fatalf("expected note_added activity, got %#v", activity.NewValue)
	}
	var metadata map[string]string
	if err := json.Unmarshal(activity.Metadata, &metadata); err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	if !strings.Contains(metadata["note_snippet"], "Please keep the implementation focused") {
		t.Fatalf("expected note snippet in metadata, got %#v", metadata)
	}
	if len(metadata["note_snippet"]) > 80 {
		t.Fatalf("expected short note snippet, got %d chars", len(metadata["note_snippet"]))
	}
}
