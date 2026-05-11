package templates

import (
	"context"
	"encoding/json"
	"strings"
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
	assertTemplateActivity(t, db, result.Rule.ID, "template.installed", "release_notes_writer")
}

func TestInstallerInstallGitHubTemplateAddsRepositoryFilter(t *testing.T) {
	db := setupInstallerTestDB(t)
	installer := NewInstaller(db, mustTestRegistry(t))

	result, err := installer.Install(context.Background(), InstallRequest{
		WorkspaceID: "ws-1",
		TemplateKey: "review_merged_prs",
		ActorID:     "user-1",
		Inputs: map[string]any{
			"repository_id":          "repo-1",
			"base_branch":            "main",
			"report_space_id":        "space-1",
			"report_collection_id":   "collection-1",
			"create_follow_up_tasks": true,
			"destination_team_id":    "team-1",
			"destination_state_id":   "state-1",
		},
	})
	if err != nil {
		t.Fatalf("Install returned error: %v", err)
	}
	var triggerConfig model.TriggerConfigGitHubPullRequest
	if err := json.Unmarshal(result.Rule.TriggerConfig, &triggerConfig); err != nil {
		t.Fatalf("unmarshal trigger config: %v", err)
	}
	if triggerConfig.RepoFullName != "acme/api" {
		t.Fatalf("repo_full_name = %q, want acme/api", triggerConfig.RepoFullName)
	}
	if triggerConfig.BaseBranch != "main" {
		t.Fatalf("base_branch = %q, want main", triggerConfig.BaseBranch)
	}
	var actionConfig map[string]any
	if err := json.Unmarshal(result.Rule.ActionConfig, &actionConfig); err != nil {
		t.Fatalf("unmarshal action config: %v", err)
	}
	if got := actionConfig["destination_state_id"]; got != "state-1" {
		t.Fatalf("destination_state_id parameter = %#v, want state-1", got)
	}
	if result.Agent.SystemPrompt == nil || !strings.Contains(*result.Agent.SystemPrompt, "- destination_state_id: state-1") {
		t.Fatalf("agent prompt does not include destination_state_id: %v", result.Agent.SystemPrompt)
	}
}

func TestInstallerInstallCreateAgentTemplateAppliesAgentSetup(t *testing.T) {
	db := setupInstallerTestDB(t)
	installer := NewInstaller(db, mustTestRegistry(t))
	maxRuns := 2

	result, err := installer.Install(context.Background(), InstallRequest{
		WorkspaceID: "ws-1",
		TemplateKey: "release_notes_writer",
		ActorID:     "user-1",
		Name:        "Release Notes",
		AgentName:   "Release Docs Reviewer",
		Inputs: map[string]any{
			"repository_id":             "repo-1",
			"destination_collection_id": "collection-1",
		},
		AgentOverrides: &model.CreateAgentFromTemplateOverrides{
			SystemPrompt:          strPtr("Use the release notes playbook."),
			ApprovalMode:          strPtr("never"),
			AllowedTargets:        model.JSONBlob(`["repository"]`),
			MaxConcurrentRuns:     &maxRuns,
			DefaultInvocationMode: strPtr("interactive"),
		},
	})
	if err != nil {
		t.Fatalf("Install returned error: %v", err)
	}
	if result.Agent == nil {
		t.Fatal("expected created agent")
	}
	if result.Agent.Name != "Release Docs Reviewer" {
		t.Fatalf("agent name = %q, want override", result.Agent.Name)
	}
	if result.Agent.SystemPrompt == nil || *result.Agent.SystemPrompt != "Use the release notes playbook." {
		t.Fatalf("system_prompt = %v, want override", result.Agent.SystemPrompt)
	}
	if result.Agent.ApprovalMode != "never" {
		t.Fatalf("approval_mode = %q, want never", result.Agent.ApprovalMode)
	}
	if result.Agent.MaxConcurrentRuns != 2 {
		t.Fatalf("max_concurrent_runs = %d, want 2", result.Agent.MaxConcurrentRuns)
	}
	var targets []string
	if err := json.Unmarshal(result.Agent.AllowedTargets, &targets); err != nil {
		t.Fatalf("unmarshal allowed_targets: %v", err)
	}
	if len(targets) != 1 || targets[0] != "repository" {
		t.Fatalf("allowed_targets = %#v, want [repository]", targets)
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
			"schedule":                "0 9 * * 1",
			"docs_scope":              "collection",
			"space_id":                "space-1",
			"collection_id":           "collection-1",
			"source_repository_id":    "repo-1",
			"report_space_id":         "space-1",
			"report_collection_id":    "collection-1",
			"create_follow_up_tasks":  true,
			"destination_team_id":     "team-1",
			"destination_state_id":    "state-1",
			"additional_instructions": "Focus on API examples.",
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
	var actionConfig map[string]any
	if err := json.Unmarshal(result.Rule.ActionConfig, &actionConfig); err != nil {
		t.Fatalf("unmarshal action config: %v", err)
	}
	if got := actionConfig["docs_scope"]; got != "collection" {
		t.Fatalf("docs_scope = %#v, want collection", got)
	}
	if got := actionConfig["space_id"]; got != "space-1" {
		t.Fatalf("space_id = %#v, want space-1", got)
	}
	if got := actionConfig["collection_id"]; got != "collection-1" {
		t.Fatalf("collection_id = %#v, want collection-1", got)
	}
	if got := actionConfig["source_repository_id"]; got != "repo-1" {
		t.Fatalf("source_repository_id = %#v, want repo-1", got)
	}
	if got := actionConfig["target_type"]; got != "repository" {
		t.Fatalf("target_type = %#v, want repository", got)
	}
	if got := actionConfig["target_id"]; got != "repo-1" {
		t.Fatalf("target_id = %#v, want repo-1", got)
	}
	if got := actionConfig["report_space_id"]; got != "space-1" {
		t.Fatalf("report_space_id = %#v, want space-1", got)
	}
	if got := actionConfig["report_collection_id"]; got != "collection-1" {
		t.Fatalf("report_collection_id = %#v, want collection-1", got)
	}
	if got := actionConfig["create_follow_up_tasks"]; got != true {
		t.Fatalf("create_follow_up_tasks = %#v, want true", got)
	}
	if got := actionConfig["destination_team_id"]; got != "team-1" {
		t.Fatalf("destination_team_id = %#v, want team-1", got)
	}
	if got := actionConfig["destination_state_id"]; got != "state-1" {
		t.Fatalf("destination_state_id = %#v, want state-1", got)
	}
	context, _ := actionConfig["additional_context"].(string)
	if !strings.Contains(context, "- docs_scope: collection") {
		t.Fatalf("additional_context missing scope: %s", context)
	}
	if !strings.Contains(context, "- collection_id: collection-1") {
		t.Fatalf("additional_context missing collection: %s", context)
	}
	if !strings.Contains(context, "- source_repository_id: repo-1") {
		t.Fatalf("additional_context missing source repository: %s", context)
	}
	if !strings.Contains(context, "Source verification:") {
		t.Fatalf("additional_context missing source verification guidance: %s", context)
	}
	if !strings.Contains(context, "Always create a new internal Docs freshness report") {
		t.Fatalf("additional_context missing report-first instruction: %s", context)
	}
	if !strings.Contains(context, "- report_collection_id: collection-1") {
		t.Fatalf("additional_context missing report collection: %s", context)
	}
	if !strings.Contains(context, "If create_follow_up_tasks is true") {
		t.Fatalf("additional_context missing optional task instruction: %s", context)
	}
	if !strings.Contains(context, "Focus on API examples.") {
		t.Fatalf("additional_context missing user instructions: %s", context)
	}
}

func TestRenderTemplateTextBlanksMissingPlaceholders(t *testing.T) {
	rendered := renderTemplateText("Additional notes:\n{{additional_instructions}}", map[string]any{})
	if strings.Contains(rendered, "{{additional_instructions}}") {
		t.Fatalf("rendered text kept unresolved placeholder: %q", rendered)
	}
}

func TestInstallerInstallAPIDocsFreshnessRequiresSourceRepositoryTarget(t *testing.T) {
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
	_, err := installer.Install(context.Background(), InstallRequest{
		WorkspaceID: "ws-1",
		TemplateKey: "api_docs_freshness_sweep",
		ActorID:     "user-1",
		Inputs: map[string]any{
			"schedule":             "0 9 * * 1",
			"docs_scope":           "space",
			"space_id":             "space-1",
			"report_space_id":      "space-1",
			"report_collection_id": "collection-1",
		},
	})
	if err == nil || !strings.Contains(err.Error(), `input "source_repository_id" is required`) {
		t.Fatalf("Install error = %v, want required source_repository_id", err)
	}

	result, err := installer.Install(context.Background(), InstallRequest{
		WorkspaceID: "ws-1",
		TemplateKey: "api_docs_freshness_sweep",
		ActorID:     "user-1",
		Inputs: map[string]any{
			"schedule":             "0 9 * * 1",
			"docs_scope":           "space",
			"space_id":             "space-1",
			"source_repository_id": "repo-1",
			"report_space_id":      "space-1",
			"report_collection_id": "collection-1",
		},
	})
	if err != nil {
		t.Fatalf("Install returned error: %v", err)
	}
	var actionConfig map[string]any
	if err := json.Unmarshal(result.Rule.ActionConfig, &actionConfig); err != nil {
		t.Fatalf("unmarshal action config: %v", err)
	}
	if got := actionConfig["target_type"]; got != "repository" {
		t.Fatalf("target_type = %#v, want repository", got)
	}
	if got := actionConfig["target_id"]; got != "repo-1" {
		t.Fatalf("target_id = %#v, want repo-1", got)
	}
	context, _ := actionConfig["additional_context"].(string)
	if !strings.Contains(context, "Use read-only repository inspection as the source of truth") {
		t.Fatalf("additional_context missing API source-of-truth guidance: %s", context)
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
	assertTemplateActivity(t, db, installed.Rule.ID, "template.uninstalled", "release_notes_writer")
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
		`CREATE TABLE pm_activity_log (
			id text PRIMARY KEY,
			workspace_id text NOT NULL,
			entity_type text NOT NULL,
			entity_id text NOT NULL,
			actor_id text,
			action text NOT NULL,
			field_name text,
			old_value text,
			new_value text,
			metadata text,
			created_at datetime
		)`,
		`CREATE TABLE git_repositories (
			id text PRIMARY KEY,
			workspace_id text NOT NULL,
			integration_id text NOT NULL,
			provider text NOT NULL DEFAULT 'github',
			external_id text NOT NULL DEFAULT '',
			full_name text NOT NULL,
			default_branch text NOT NULL DEFAULT 'main',
			permissions text NOT NULL DEFAULT '{}',
			private boolean NOT NULL DEFAULT true,
			archived boolean NOT NULL DEFAULT false,
			selected boolean NOT NULL DEFAULT true,
			active boolean NOT NULL DEFAULT true,
			deleted_at datetime,
			created_at datetime,
			updated_at datetime
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create test schema: %v", err)
		}
	}
	if err := db.Exec(`INSERT INTO git_repositories (id, workspace_id, integration_id, provider, external_id, full_name, default_branch, permissions, private, archived, selected, active)
		VALUES ('repo-1', 'ws-1', 'integration-1', 'github', '101', 'acme/api', 'main', '{}', true, false, true, true)`).Error; err != nil {
		t.Fatalf("seed repository: %v", err)
	}
	return db
}

func assertTemplateActivity(t *testing.T, db *gorm.DB, ruleID, action, templateKey string) {
	t.Helper()
	var entry model.PMActivityLog
	if err := db.Where("entity_type = ? AND entity_id = ? AND action = ?", "automation_flow", ruleID, action).First(&entry).Error; err != nil {
		t.Fatalf("load %s activity: %v", action, err)
	}
	var metadata struct {
		TemplateKey string `json:"template_key"`
	}
	if err := json.Unmarshal(entry.Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal activity metadata: %v", err)
	}
	if metadata.TemplateKey != templateKey {
		t.Fatalf("activity template_key = %q, want %q", metadata.TemplateKey, templateKey)
	}
}

func mustTestRegistry(t *testing.T) *Registry {
	t.Helper()
	registry, err := LoadSystemRegistry()
	if err != nil {
		t.Fatalf("load system registry: %v", err)
	}
	return registry
}
