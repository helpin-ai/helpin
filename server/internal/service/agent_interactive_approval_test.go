package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
)

func TestSendRunMessageTreatsExplicitApprovalAsNormalUserReply(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Epic Planner", model.AgentPresetEpicPlanner, "Planner", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:             "run-1",
		WorkspaceID:    "ws-1",
		AgentID:        "agent-1",
		TargetType:     "epic",
		TargetID:       "epic-1",
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeInteractive,
		ApprovalState:  "not_required",
		Status:         model.AgentRunStatusAwaitingInput,
		OutputSummary:  []byte(`{"status":"waiting"}`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := runMessageRepo.Create(context.Background(), &model.AgentRunMessage{
		WorkspaceID: "ws-1",
		RunID:       run.ID,
		Role:        "assistant",
		Content:     "Please review the latest PRD draft in the preview pane.",
		MessageType: "assistant_turn",
		SequenceNo:  1,
	}); err != nil {
		t.Fatalf("create message: %v", err)
	}

	svc := &AgentService{
		agentRepo:      agentRepo,
		runRepo:        runRepo,
		runMessageRepo: runMessageRepo,
		runEngine:      &temporalapp.RunEngine{},
	}

	message, err := svc.SendRunMessage(context.Background(), "ws-1", run.ID, "user-1", model.SendAgentRunMessageRequest{
		Content: "good to go",
	})
	if err != nil {
		t.Fatalf("SendRunMessage returned error: %v", err)
	}
	if message.MessageType != "user_reply" {
		t.Fatalf("expected user_reply message type, got %q", message.MessageType)
	}

	updated, err := runRepo.GetByID(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if updated.Status != model.AgentRunStatusRunning {
		t.Fatalf("expected run status %q, got %q", model.AgentRunStatusRunning, updated.Status)
	}
	if updated.ApprovalState != "not_required" {
		t.Fatalf("expected approval_state not_required, got %q", updated.ApprovalState)
	}
	if updated.ExecutionStage == nil || *updated.ExecutionStage != "resuming" {
		t.Fatalf("expected execution stage resuming, got %#v", updated.ExecutionStage)
	}
	if string(updated.OutputSummary) != `{"status":"waiting"}` {
		t.Fatalf("expected output_summary to remain unchanged, got %s", string(updated.OutputSummary))
	}
}

func TestSendRunMessageTreatsLongApprovalPhraseAsNormalUserReply(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Epic Planner", model.AgentPresetEpicPlanner, "Planner", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:             "run-long-approve",
		WorkspaceID:    "ws-1",
		AgentID:        "agent-1",
		TargetType:     "epic",
		TargetID:       "epic-1",
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeInteractive,
		ApprovalState:  "not_required",
		Status:         model.AgentRunStatusAwaitingInput,
		OutputSummary:  []byte(`{"status":"waiting"}`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	svc := &AgentService{
		agentRepo:      agentRepo,
		runRepo:        runRepo,
		runMessageRepo: runMessageRepo,
		runEngine:      &temporalapp.RunEngine{},
	}

	message, err := svc.SendRunMessage(context.Background(), "ws-1", run.ID, "user-1", model.SendAgentRunMessageRequest{
		Content: "approve and create the PRD doc",
	})
	if err != nil {
		t.Fatalf("SendRunMessage returned error: %v", err)
	}
	if message.MessageType != "user_reply" {
		t.Fatalf("expected user_reply message type, got %q", message.MessageType)
	}
}

func TestSendRunMessageKeepsInteractiveRunResumingOnFeedback(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Epic Planner", model.AgentPresetEpicPlanner, "Planner", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:             "run-2",
		WorkspaceID:    "ws-1",
		AgentID:        "agent-1",
		TargetType:     "epic",
		TargetID:       "epic-1",
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeInteractive,
		ApprovalState:  "not_required",
		Status:         model.AgentRunStatusAwaitingInput,
		OutputSummary:  []byte(`{"status":"waiting"}`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	svc := &AgentService{
		agentRepo:      agentRepo,
		runRepo:        runRepo,
		runMessageRepo: runMessageRepo,
		runEngine:      &temporalapp.RunEngine{},
	}

	message, err := svc.SendRunMessage(context.Background(), "ws-1", run.ID, "user-1", model.SendAgentRunMessageRequest{
		Content: "add one more edge case before approval",
	})
	if err != nil {
		t.Fatalf("SendRunMessage returned error: %v", err)
	}
	if message.MessageType != "user_reply" {
		t.Fatalf("expected user_reply message type, got %q", message.MessageType)
	}

	updated, err := runRepo.GetByID(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if updated.ExecutionStage == nil || *updated.ExecutionStage != "resuming" {
		t.Fatalf("expected execution stage resuming, got %#v", updated.ExecutionStage)
	}
	if string(updated.OutputSummary) != `{"status":"waiting"}` {
		t.Fatalf("expected output_summary to remain unchanged, got %s", string(updated.OutputSummary))
	}
}

func newInteractiveApprovalTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:interactive-approval-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	registerAgentTestUUIDCallback(t, db)

	statements := []string{
		`CREATE TABLE agents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			preset_key TEXT,
			role TEXT,
			status TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			skills BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			trigger_mode TEXT NOT NULL,
			provider TEXT,
			model TEXT,
			system_prompt TEXT,
			planning_notes TEXT,
			tools BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			monthly_token_budget INTEGER,
			tokens_used_this_month INTEGER NOT NULL DEFAULT 0,
			active_story_id TEXT,
			team_id TEXT,
			allowed_tools BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			allowed_commands BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			allowed_targets BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			schedule TEXT,
			target_selector TEXT,
			trigger_events BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			approval_mode TEXT NOT NULL DEFAULT 'never',
			max_concurrent_runs INTEGER NOT NULL DEFAULT 1,
			default_invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_runs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			story_id TEXT,
			conversation_id TEXT,
			target_type TEXT NOT NULL,
			target_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			invocation_mode TEXT NOT NULL,
			parent_run_id TEXT,
			handoff_state TEXT,
			approval_state TEXT NOT NULL,
			triggered_by_user_id TEXT,
			status TEXT NOT NULL,
			workflow_id TEXT,
			workflow_run_id TEXT,
			task_queue TEXT,
			runner_pool TEXT,
			repository_id TEXT,
			repo_full_name TEXT,
			base_branch TEXT,
			working_branch TEXT,
			delivery_target_id TEXT,
			execution_stage TEXT,
			last_heartbeat_at DATETIME,
			input BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			output_summary BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			tokens_used INTEGER NOT NULL DEFAULT 0,
			error_message TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_run_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			message_type TEXT NOT NULL,
			content_blocks BLOB,
			tool_invocations BLOB,
			token_usage BLOB,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("exec schema: %v", err)
		}
	}

	return db
}
