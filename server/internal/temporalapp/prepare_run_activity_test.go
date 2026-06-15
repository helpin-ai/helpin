package temporalapp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestPrepareRunActivityMarksRunRunning(t *testing.T) {
	dbName := fmt.Sprintf("file:prepare-run-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
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
			source_template_key TEXT NOT NULL DEFAULT '',
			template_key TEXT,
			template_instance_id TEXT,
			template_version INTEGER,
			active_version_id TEXT,
			role TEXT,
			status TEXT NOT NULL DEFAULT 'idle',
			runtime_kind TEXT NOT NULL DEFAULT 'opencode',
			skills BLOB NOT NULL DEFAULT '[]',
			trigger_mode TEXT NOT NULL DEFAULT 'manual',
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
			allowed_targets BLOB NOT NULL DEFAULT '[]',
			schedule TEXT,
			approval_mode TEXT NOT NULL DEFAULT 'preset_default',
			max_concurrent_runs INTEGER NOT NULL DEFAULT 1,
			default_invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_runs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			task_id TEXT,
			conversation_id TEXT,
			target_type TEXT NOT NULL DEFAULT 'story',
			target_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL DEFAULT 'opencode',
			invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
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
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("exec schema: %v", err)
		}
	}

	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)

	agent := &model.Agent{
		ID:              "agent-1",
		WorkspaceID:     "workspace-1",
		Name:            "Planner",
		PresetKey:       model.AgentPresetEpicPlanner,
		RuntimeKind:     "native_sdk",
		Status:          "idle",
		Skills:          model.AgentSkillRefs{},
		AllowedTools:    []byte("[]"),
		AllowedCommands: []byte("[]"),
		AllowedTargets:  []byte("[]"),
	}
	if err := agentRepo.Create(context.Background(), agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	run := &model.AgentRun{
		ID:            "run-1",
		WorkspaceID:   agent.WorkspaceID,
		AgentID:       agent.ID,
		TargetType:    "noop",
		TargetID:      "target-1",
		Status:        model.AgentRunStatusQueued,
		Input:         []byte(`{}`),
		OutputSummary: []byte(`{}`),
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	activities := &AgentRunActivities{
		runRepo:   runRepo,
		agentRepo: agentRepo,
	}

	if err := activities.PrepareRunActivity(context.Background(), run.ID); err != nil {
		t.Fatalf("prepare run activity: %v", err)
	}

	updated, err := runRepo.GetByIDAny(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("get updated run: %v", err)
	}
	if updated == nil {
		t.Fatal("expected updated run")
	}
	if updated.Status != model.AgentRunStatusRunning {
		t.Fatalf("expected run status %q, got %q", model.AgentRunStatusRunning, updated.Status)
	}
	if updated.StartedAt == nil {
		t.Fatal("expected started_at to be set")
	}
	if updated.ExecutionStage == nil || *updated.ExecutionStage != "preparing" {
		t.Fatalf("expected execution stage preparing, got %#v", updated.ExecutionStage)
	}
	if updated.LastHeartbeatAt == nil {
		t.Fatal("expected last_heartbeat_at to be set")
	}
}
