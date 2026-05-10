package templates

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInstallerInstallCreateAgentTemplateCreatesStampedAgentAndRule(t *testing.T) {
	db := setupInstallerTestDB(t)
	registry := mustTestRegistry(t)
	installer := NewInstaller(db, registry)

	result, err := installer.Install(context.Background(), InstallRequest{
		WorkspaceID: "ws-1",
		TemplateKey: "release_notes_writer",
		ActorID:     "user-1",
		Name:        "Release Notes",
		Inputs: map[string]any{
			"repository_id":             "repo-1",
			"destination_collection_id": "collection-1",
			"include_prerelease":        true,
		},
	})
	if err != nil {
		t.Fatalf("Install returned error: %v", err)
	}
	if result.Agent == nil {
		t.Fatal("expected created agent")
	}
	if result.Rule == nil {
		t.Fatal("expected created rule")
	}
	if result.Agent.TemplateKey == nil || *result.Agent.TemplateKey != "release_notes_writer" {
		t.Fatalf("agent template_key = %v, want release_notes_writer", result.Agent.TemplateKey)
	}
	if result.Agent.TemplateInstanceID == nil || *result.Agent.TemplateInstanceID == "" {
		t.Fatalf("agent template_instance_id = %v, want generated id", result.Agent.TemplateInstanceID)
	}
	if result.Agent.TemplateVersion == nil || *result.Agent.TemplateVersion != 1 {
		t.Fatalf("agent template_version = %v, want 1", result.Agent.TemplateVersion)
	}
	if result.Rule.TemplateInstanceID == nil || result.Agent.TemplateInstanceID == nil || *result.Rule.TemplateInstanceID != *result.Agent.TemplateInstanceID {
		t.Fatalf("rule/agent template_instance_id mismatch: rule=%v agent=%v", result.Rule.TemplateInstanceID, result.Agent.TemplateInstanceID)
	}

	var actionConfig model.ActionConfigRunAgent
	if err := json.Unmarshal(result.Rule.ActionConfig, &actionConfig); err != nil {
		t.Fatalf("unmarshal action config: %v", err)
	}
	if actionConfig.AgentID != result.Agent.ID {
		t.Fatalf("action_config.agent_id = %q, want %q", actionConfig.AgentID, result.Agent.ID)
	}
	if actionConfig.TargetType != "repository" || actionConfig.TargetID != "repo-1" {
		t.Fatalf("target = %s/%s, want repository/repo-1", actionConfig.TargetType, actionConfig.TargetID)
	}
}

func TestInstallerInstallReuseSystemStampsOnlyRule(t *testing.T) {
	db := setupInstallerTestDB(t)
	systemAgent := model.Agent{
		ID:                    "agent-quill",
		WorkspaceID:           "ws-1",
		IsSystem:              true,
		Name:                  "Quill",
		PresetKey:             model.AgentPresetDocumentationAgent,
		Role:                  "Documentation Agent",
		Status:                "idle",
		RuntimeKind:           "native_sdk",
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		AllowedTargets:        json.RawMessage(`[]`),
		Skills:                model.AgentSkillRefs{},
		ExecutionConfig:       model.JSONBlob(`{}`),
		ApprovalMode:          "always",
		DefaultInvocationMode: "interactive",
	}
	if err := db.Create(&systemAgent).Error; err != nil {
		t.Fatalf("create system agent: %v", err)
	}

	installer := NewInstaller(db, mustTestRegistry(t))
	result, err := installer.Install(context.Background(), InstallRequest{
		WorkspaceID: "ws-1",
		TemplateKey: "docs_freshness_sweep",
		ActorID:     "user-1",
		Inputs: map[string]any{
			"schedule": "0 9 * * 1",
		},
	})
	if err != nil {
		t.Fatalf("Install returned error: %v", err)
	}
	if result.Agent == nil || result.Agent.ID != "agent-quill" {
		t.Fatalf("agent = %#v, want existing Quill", result.Agent)
	}
	if result.Agent.TemplateInstanceID != nil || result.Agent.TemplateKey != nil || result.Agent.TemplateVersion != nil {
		t.Fatalf("system agent should not be stamped: %#v", result.Agent)
	}
	if result.Rule.TemplateKey == nil || *result.Rule.TemplateKey != "docs_freshness_sweep" {
		t.Fatalf("rule template_key = %v, want docs_freshness_sweep", result.Rule.TemplateKey)
	}
}

func TestInstallerInstallNoneTemplateCreatesRuleWithoutAgent(t *testing.T) {
	db := setupInstallerTestDB(t)
	installer := NewInstaller(db, mustTestRegistry(t))

	result, err := installer.Install(context.Background(), InstallRequest{
		WorkspaceID: "ws-1",
		TemplateKey: "advance_on_approval",
		ActorID:     "user-1",
		Inputs: map[string]any{
			"workflow_id":   "workflow-1",
			"from_state_id": "state-review",
			"to_state_id":   "state-done",
		},
	})
	if err != nil {
		t.Fatalf("Install returned error: %v", err)
	}
	if result.Agent != nil {
		t.Fatalf("expected no agent, got %#v", result.Agent)
	}
	if result.Rule.ActionType != model.ActionMoveToState {
		t.Fatalf("action_type = %q, want move_to_state", result.Rule.ActionType)
	}
	if result.Rule.WorkflowID == nil || *result.Rule.WorkflowID != "workflow-1" {
		t.Fatalf("workflow_id = %v, want workflow-1", result.Rule.WorkflowID)
	}
	var actionConfig model.ActionConfigMoveToState
	if err := json.Unmarshal(result.Rule.ActionConfig, &actionConfig); err != nil {
		t.Fatalf("unmarshal action config: %v", err)
	}
	if actionConfig.TargetStateID != "state-done" {
		t.Fatalf("target_state_id = %q, want state-done", actionConfig.TargetStateID)
	}
}

func TestInstallerInstallPickExistingTemplateUsesExistingAgentAndStampsOnlyRule(t *testing.T) {
	db := setupInstallerTestDB(t)
	existingAgent := model.Agent{
		ID:                    "agent-existing",
		WorkspaceID:           "ws-1",
		IsSystem:              false,
		Name:                  "Ship Notes",
		PresetKey:             model.AgentPresetDocumentationAgent,
		Role:                  "Documentation Agent",
		Status:                "idle",
		RuntimeKind:           "native_sdk",
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		AllowedTargets:        json.RawMessage(`["repository"]`),
		Skills:                model.AgentSkillRefs{},
		ExecutionConfig:       model.JSONBlob(`{}`),
		ApprovalMode:          "always",
		DefaultInvocationMode: "interactive",
	}
	if err := db.Create(&existingAgent).Error; err != nil {
		t.Fatalf("create existing agent: %v", err)
	}

	installer := NewInstaller(db, mustTestRegistry(t))
	result, err := installer.Install(context.Background(), InstallRequest{
		WorkspaceID: "ws-1",
		TemplateKey: "run_on_release",
		ActorID:     "user-1",
		Inputs: map[string]any{
			"agent_id":      "agent-existing",
			"repository_id": "repo-1",
		},
	})
	if err != nil {
		t.Fatalf("Install returned error: %v", err)
	}
	if result.Agent == nil || result.Agent.ID != "agent-existing" {
		t.Fatalf("agent = %#v, want existing agent", result.Agent)
	}
	if result.Agent.TemplateInstanceID != nil || result.Agent.TemplateKey != nil || result.Agent.TemplateVersion != nil {
		t.Fatalf("picked agent should not be stamped: %#v", result.Agent)
	}

	var storedAgent model.Agent
	if err := db.First(&storedAgent, "id = ?", "agent-existing").Error; err != nil {
		t.Fatalf("load stored agent: %v", err)
	}
	if storedAgent.TemplateInstanceID != nil || storedAgent.TemplateKey != nil || storedAgent.TemplateVersion != nil {
		t.Fatalf("stored picked agent should not be stamped: %#v", storedAgent)
	}
	if result.Rule.TemplateKey == nil || *result.Rule.TemplateKey != "run_on_release" {
		t.Fatalf("rule template_key = %v, want run_on_release", result.Rule.TemplateKey)
	}

	var actionConfig model.ActionConfigRunAgent
	if err := json.Unmarshal(result.Rule.ActionConfig, &actionConfig); err != nil {
		t.Fatalf("unmarshal action config: %v", err)
	}
	if actionConfig.AgentID != "agent-existing" {
		t.Fatalf("action_config.agent_id = %q, want agent-existing", actionConfig.AgentID)
	}
	if actionConfig.TargetType != "repository" || actionConfig.TargetID != "repo-1" {
		t.Fatalf("target = %s/%s, want repository/repo-1", actionConfig.TargetType, actionConfig.TargetID)
	}
}

func TestUninstallerKeepCreatedAgentClearsTemplateFields(t *testing.T) {
	db := setupInstallerTestDB(t)
	installer := NewInstaller(db, mustTestRegistry(t))
	installed, err := installer.Install(context.Background(), InstallRequest{
		WorkspaceID: "ws-1",
		TemplateKey: "release_notes_writer",
		ActorID:     "user-1",
		Inputs: map[string]any{
			"repository_id":             "repo-1",
			"destination_collection_id": "collection-1",
		},
	})
	if err != nil {
		t.Fatalf("Install returned error: %v", err)
	}

	uninstaller := NewUninstaller(db)
	result, err := uninstaller.Uninstall(context.Background(), UninstallRequest{
		WorkspaceID:        "ws-1",
		TemplateInstanceID: *installed.Rule.TemplateInstanceID,
		DeleteCreatedAgent: false,
	})
	if err != nil {
		t.Fatalf("Uninstall returned error: %v", err)
	}
	if result.AgentAction != AgentActionKept {
		t.Fatalf("agent action = %q, want %q", result.AgentAction, AgentActionKept)
	}

	var count int64
	if err := db.Model(&model.AutomationRule{}).Where("id = ?", installed.Rule.ID).Count(&count).Error; err != nil {
		t.Fatalf("count rule: %v", err)
	}
	if count != 0 {
		t.Fatalf("rule count = %d, want 0", count)
	}

	var agent model.Agent
	if err := db.First(&agent, "id = ?", installed.Agent.ID).Error; err != nil {
		t.Fatalf("load kept agent: %v", err)
	}
	if agent.TemplateKey != nil || agent.TemplateInstanceID != nil || agent.TemplateVersion != nil {
		t.Fatalf("kept agent template fields = %v/%v/%v, want cleared", agent.TemplateKey, agent.TemplateInstanceID, agent.TemplateVersion)
	}
}

func TestUninstallerDeleteCreatedAgent(t *testing.T) {
	db := setupInstallerTestDB(t)
	installer := NewInstaller(db, mustTestRegistry(t))
	installed, err := installer.Install(context.Background(), InstallRequest{
		WorkspaceID: "ws-1",
		TemplateKey: "release_notes_writer",
		ActorID:     "user-1",
		Inputs: map[string]any{
			"repository_id":             "repo-1",
			"destination_collection_id": "collection-1",
		},
	})
	if err != nil {
		t.Fatalf("Install returned error: %v", err)
	}

	result, err := NewUninstaller(db).Uninstall(context.Background(), UninstallRequest{
		WorkspaceID:        "ws-1",
		TemplateInstanceID: *installed.Rule.TemplateInstanceID,
		DeleteCreatedAgent: true,
	})
	if err != nil {
		t.Fatalf("Uninstall returned error: %v", err)
	}
	if result.AgentAction != AgentActionDeleted {
		t.Fatalf("agent action = %q, want %q", result.AgentAction, AgentActionDeleted)
	}

	var count int64
	if err := db.Model(&model.Agent{}).Where("id = ?", installed.Agent.ID).Count(&count).Error; err != nil {
		t.Fatalf("count agent: %v", err)
	}
	if count != 0 {
		t.Fatalf("agent count = %d, want 0", count)
	}
}

func TestUninstallerDoesNotDeleteAgentReferencedByAnotherRule(t *testing.T) {
	db := setupInstallerTestDB(t)
	installer := NewInstaller(db, mustTestRegistry(t))
	installed, err := installer.Install(context.Background(), InstallRequest{
		WorkspaceID: "ws-1",
		TemplateKey: "release_notes_writer",
		ActorID:     "user-1",
		Inputs: map[string]any{
			"repository_id":             "repo-1",
			"destination_collection_id": "collection-1",
		},
	})
	if err != nil {
		t.Fatalf("Install returned error: %v", err)
	}
	otherRule := model.AutomationRule{
		ID:            "rule-other",
		WorkspaceID:   "ws-1",
		Name:          "Other flow",
		Enabled:       true,
		TriggerType:   model.TriggerGitHubReleasePub,
		TriggerConfig: json.RawMessage(`{}`),
		ActionType:    model.ActionStartAgentRun,
		ActionConfig:  json.RawMessage(`{"agent_id":"` + installed.Agent.ID + `","target_type":"repository","target_id":"repo-2"}`),
	}
	if err := db.Create(&otherRule).Error; err != nil {
		t.Fatalf("create other rule: %v", err)
	}

	result, err := NewUninstaller(db).Uninstall(context.Background(), UninstallRequest{
		WorkspaceID:        "ws-1",
		TemplateInstanceID: *installed.Rule.TemplateInstanceID,
		DeleteCreatedAgent: true,
	})
	if err != nil {
		t.Fatalf("Uninstall returned error: %v", err)
	}
	if result.AgentAction != AgentActionStillReferenced {
		t.Fatalf("agent action = %q, want %q", result.AgentAction, AgentActionStillReferenced)
	}

	var agent model.Agent
	if err := db.First(&agent, "id = ?", installed.Agent.ID).Error; err != nil {
		t.Fatalf("load referenced agent: %v", err)
	}
	if agent.TemplateInstanceID == nil || *agent.TemplateInstanceID != *installed.Rule.TemplateInstanceID {
		t.Fatalf("referenced agent template_instance_id = %v, want preserved", agent.TemplateInstanceID)
	}
}

func setupInstallerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	statements := []string{
		`CREATE TABLE agents (
			id text PRIMARY KEY,
			workspace_id text NOT NULL,
			is_system boolean NOT NULL DEFAULT false,
			name text NOT NULL,
			preset_key text,
			preset_version_key text,
			source_preset_key text,
			source_preset_version_key text,
			source_template_id text,
			source_template_key text,
			template_key text,
			template_instance_id text,
			template_version integer,
			role text,
			status text NOT NULL DEFAULT 'idle',
			runtime_kind text NOT NULL DEFAULT 'opencode',
			skills text NOT NULL DEFAULT '[]',
			trigger_mode text NOT NULL DEFAULT 'manual',
			provider text,
			model text,
			execution_config text NOT NULL DEFAULT '{}',
			system_prompt text,
			instruction_template_version text NOT NULL DEFAULT '',
			planning_notes text,
			monthly_token_budget integer,
			tokens_used_this_month integer NOT NULL DEFAULT 0,
			active_task_id text,
			team_id text,
			allowed_tools text NOT NULL DEFAULT '[]',
			allowed_commands text NOT NULL DEFAULT '[]',
			allowed_targets text NOT NULL DEFAULT '[]',
			approval_mode text NOT NULL DEFAULT 'preset_default',
			max_concurrent_runs integer NOT NULL DEFAULT 1,
			default_invocation_mode text NOT NULL DEFAULT 'autonomous',
			created_at datetime,
			updated_at datetime
		)`,
		`CREATE TABLE agent_team_access (
			agent_id text NOT NULL,
			team_id text NOT NULL,
			created_at datetime,
			PRIMARY KEY (agent_id, team_id)
		)`,
		`CREATE TABLE automation_rules (
			id text PRIMARY KEY,
			workspace_id text NOT NULL,
			name text NOT NULL,
			description text,
			enabled boolean NOT NULL DEFAULT true,
			team_id text,
			workflow_id text,
			trigger_type text NOT NULL,
			trigger_config text NOT NULL DEFAULT '{}',
			action_type text NOT NULL,
			action_config text NOT NULL DEFAULT '{}',
			template_key text,
			template_instance_id text,
			template_version integer,
			position integer NOT NULL DEFAULT 0,
			stop_on_match boolean NOT NULL DEFAULT false,
			created_by text,
			created_at datetime,
			updated_at datetime
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create test schema: %v", err)
		}
	}
	return db
}

func mustTestRegistry(t *testing.T) *Registry {
	t.Helper()
	registry, err := LoadSystemRegistry()
	if err != nil {
		t.Fatalf("load system registry: %v", err)
	}
	return registry
}
