package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAgentRepositoryWritesWithoutActiveVersionColumn(t *testing.T) {
	db := openAgentVersionColumnCompatDB(t)
	repo := NewAgentRepository(db)

	activeVersionID := "version-1"
	agent := &model.Agent{
		ID:                    "agent-1",
		WorkspaceID:           "workspace-1",
		Name:                  "Legacy Schema Agent",
		Status:                "idle",
		RuntimeKind:           "codex",
		TriggerMode:           "manual",
		ExecutionConfig:       model.JSONBlob(`{}`),
		Skills:                model.AgentSkillRefs{},
		ActiveVersionID:       &activeVersionID,
		AllowedTools:          []byte(`[]`),
		AllowedCommands:       []byte(`[]`),
		AllowedTargets:        []byte(`["task"]`),
		ApprovalMode:          "always",
		MaxConcurrentRuns:     1,
		DefaultInvocationMode: model.InvocationModeInteractive,
	}
	if err := repo.Create(context.Background(), agent); err != nil {
		t.Fatalf("create agent against schema without active_version_id: %v", err)
	}

	agent.Status = "running"
	if err := repo.Update(context.Background(), agent); err != nil {
		t.Fatalf("update agent against schema without active_version_id: %v", err)
	}
}

func TestAgentRunRepositoryWritesWithoutAgentVersionColumn(t *testing.T) {
	db := openAgentVersionColumnCompatDB(t)
	repo := NewAgentRunRepository(db)

	agentVersionID := "version-1"
	run := &model.AgentRun{
		ID:             "run-1",
		WorkspaceID:    "workspace-1",
		AgentID:        "agent-1",
		TargetType:     "task",
		TargetID:       "task-1",
		RuntimeKind:    "codex",
		InvocationMode: model.InvocationModeInteractive,
		ApprovalState:  "not_required",
		PauseReason:    "none",
		Status:         "queued",
		AgentVersionID: &agentVersionID,
		Input:          []byte(`{}`),
		OutputSummary:  []byte(`{}`),
	}
	if err := repo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run against schema without agent_version_id: %v", err)
	}

	run.Status = "running"
	if err := repo.Update(context.Background(), run); err != nil {
		t.Fatalf("update run against schema without agent_version_id: %v", err)
	}
}

func openAgentVersionColumnCompatDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := fmt.Sprintf("file:agent_version_column_compat_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agents (
 ai_profile_id TEXT,
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			icon_key TEXT NOT NULL DEFAULT '',
			preset_key TEXT,
			preset_version_key TEXT,
			source_preset_key TEXT,
			source_preset_version_key TEXT,
			source_template_id TEXT,
			source_template_key TEXT NOT NULL DEFAULT '',
			template_key TEXT,
			template_instance_id TEXT,
			template_version INTEGER,
			role TEXT,
			status TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			model_tier TEXT NOT NULL DEFAULT '',
			skills BLOB NOT NULL DEFAULT '[]',
			trigger_mode TEXT NOT NULL,
			provider TEXT,
			model TEXT,
			execution_config BLOB NOT NULL DEFAULT x'7b7d',
			system_prompt TEXT,
			instruction_template_version TEXT NOT NULL DEFAULT '',
			planning_notes TEXT,
			monthly_token_budget INTEGER,
			tokens_used_this_month INTEGER NOT NULL DEFAULT 0,
			active_task_id TEXT,
			team_id TEXT,
			allowed_tools BLOB NOT NULL DEFAULT '[]',
			allowed_commands BLOB NOT NULL DEFAULT '[]',
			allowed_targets BLOB NOT NULL DEFAULT x'5b5d',
			approval_mode TEXT NOT NULL DEFAULT 'never',
			max_concurrent_runs INTEGER NOT NULL DEFAULT 1,
			default_invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("exec schema: %v", err)
		}
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_runs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			task_id TEXT,
			conversation_id TEXT,
			target_type TEXT NOT NULL DEFAULT 'task',
			target_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL DEFAULT 'native_sdk',
			model_tier TEXT NOT NULL DEFAULT '',
			invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			parent_run_id TEXT,
			dock_chat_id TEXT,
			handoff_state TEXT,
			approval_state TEXT NOT NULL DEFAULT 'not_required',
			pause_reason TEXT NOT NULL DEFAULT 'none',
			triggered_by_user_id TEXT,
			status TEXT NOT NULL DEFAULT 'queued',
			workflow_id TEXT,
			workflow_run_id TEXT,
			external_runtime TEXT,
			external_runtime_id TEXT,
			task_queue TEXT,
			runner_pool TEXT,
			repository_id TEXT,
			repo_full_name TEXT,
			base_branch TEXT,
			working_branch TEXT,
			delivery_target_id TEXT,
			execution_stage TEXT,
			last_heartbeat_at DATETIME,
			input BLOB NOT NULL DEFAULT x'7b7d',
			output_summary BLOB NOT NULL DEFAULT x'7b7d',
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
		`CREATE TABLE agent_team_access (
			agent_id TEXT NOT NULL,
			team_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (agent_id, team_id)
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("exec schema: %v", err)
		}
	}
	return db
}
