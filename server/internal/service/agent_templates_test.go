package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

func TestListAgentTemplatesSeedsReleaseNotesWriter(t *testing.T) {
	db := newAgentServiceTestDB(t)
	addAgentTemplateTable(t, db)

	svc := (&AgentService{}).SetAgentTemplateRepository(repository.NewAgentTemplateRepository(db))

	templates, err := svc.ListAgentTemplates(context.Background(), "ws-test")
	if err != nil {
		t.Fatalf("ListAgentTemplates returned error: %v", err)
	}
	if len(templates) == 0 {
		t.Fatal("expected seeded templates")
	}

	var releaseNotes *model.AgentTemplate
	for idx := range templates {
		if templates[idx].Key == model.AgentTemplateTypeReleaseNotes {
			releaseNotes = &templates[idx]
			break
		}
	}
	if releaseNotes == nil {
		t.Fatalf("expected %q template, got %+v", model.AgentTemplateTypeReleaseNotes, templates)
	}
	if releaseNotes.WorkspaceID != nil {
		t.Fatalf("expected system template workspace_id to be nil, got %v", *releaseNotes.WorkspaceID)
	}
	if releaseNotes.Name != "Release Notes Writer" {
		t.Fatalf("expected Release Notes Writer template name, got %q", releaseNotes.Name)
	}
	if releaseNotes.RuntimeKind != model.AgentTemplateRuntimeKindNativeSDK {
		t.Fatalf("expected native_sdk runtime, got %q", releaseNotes.RuntimeKind)
	}
	if len(releaseNotes.Skills) != 1 || releaseNotes.Skills[0].Key != model.AgentTemplateTypeReleaseNotes {
		t.Fatalf("expected release_notes_writer skill ref, got %+v", releaseNotes.Skills)
	}
}

func TestCreateAgentFromTemplateCreatesCustomAgentAndStarterFlow(t *testing.T) {
	db := newAgentServiceTestDB(t)
	addAgentTemplateTable(t, db)
	addAutomationRuleTable(t, db)

	agentRepo := repository.NewAgentRepository(db)
	activitySvc := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := (&AgentService{
		agentRepo:   agentRepo,
		activitySvc: activitySvc,
	}).SetAgentTemplateRepository(repository.NewAgentTemplateRepository(db))
	ruleEngine := NewAutomationRuleEngine(
		repository.NewAutomationRuleRepository(db),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	svc.SetRuleEngine(ruleEngine)

	if err := svc.EnsureSystemTemplates(context.Background()); err != nil {
		t.Fatalf("EnsureSystemTemplates returned error: %v", err)
	}
	template, err := svc.agentTemplateRepo.GetByKey(context.Background(), nil, model.AgentTemplateTypeReleaseNotes)
	if err != nil {
		t.Fatalf("GetByKey returned error: %v", err)
	}
	if template == nil {
		t.Fatal("expected seeded release notes template")
	}

	result, err := svc.CreateAgentFromTemplate(context.Background(), "ws-test", template.ID, model.CreateAgentFromTemplateRequest{
		CreateFlow: true,
		Flow: &model.CreateAgentFromTemplateFlow{
			RepositoryID: "repo-1",
			RepoFullName: "acme/api",
			SpaceID:      "space-1",
			CollectionID: strPtr("collection-1"),
		},
	}, "user-1")
	if err != nil {
		t.Fatalf("CreateAgentFromTemplate returned error: %v", err)
	}
	if result.Agent == nil {
		t.Fatal("expected created agent")
	}
	if result.Agent.IsSystem {
		t.Fatal("expected template-created agent to remain custom")
	}
	if result.Agent.SourceTemplateID == nil || *result.Agent.SourceTemplateID != template.ID {
		t.Fatalf("expected source_template_id %q, got %+v", template.ID, result.Agent.SourceTemplateID)
	}
	if result.Agent.SourceTemplateKey != model.AgentTemplateTypeReleaseNotes {
		t.Fatalf("expected source_template_key %q, got %q", model.AgentTemplateTypeReleaseNotes, result.Agent.SourceTemplateKey)
	}
	if result.Agent.RuntimeKind != model.AgentTemplateRuntimeKindNativeSDK {
		t.Fatalf("expected native_sdk runtime, got %q", result.Agent.RuntimeKind)
	}

	var allowedTargets []string
	if err := json.Unmarshal(result.Agent.AllowedTargets, &allowedTargets); err != nil {
		t.Fatalf("unmarshal allowed targets: %v", err)
	}
	if len(allowedTargets) != 1 || allowedTargets[0] != "repository" {
		t.Fatalf("expected repository target, got %v", allowedTargets)
	}
	if result.Flow == nil {
		t.Fatal("expected starter flow")
	}
	if result.Flow.TriggerType != model.TriggerGitHubReleasePub {
		t.Fatalf("expected github.release_published trigger, got %q", result.Flow.TriggerType)
	}

	var triggerCfg model.TriggerConfigGitHubReleasePublished
	if err := json.Unmarshal(result.Flow.TriggerConfig, &triggerCfg); err != nil {
		t.Fatalf("unmarshal trigger config: %v", err)
	}
	if triggerCfg.RepoFullName != "acme/api" {
		t.Fatalf("expected repo_full_name acme/api, got %q", triggerCfg.RepoFullName)
	}
	if len(triggerCfg.ReleaseKinds) != 1 || triggerCfg.ReleaseKinds[0] != "minor" {
		t.Fatalf("expected default minor release kind, got %v", triggerCfg.ReleaseKinds)
	}

	var actionCfg model.ActionConfigRunAgent
	if err := json.Unmarshal(result.Flow.ActionConfig, &actionCfg); err != nil {
		t.Fatalf("unmarshal action config: %v", err)
	}
	if actionCfg.AgentID != result.Agent.ID {
		t.Fatalf("expected flow agent_id %q, got %q", result.Agent.ID, actionCfg.AgentID)
	}
	if actionCfg.TargetType != "repository" {
		t.Fatalf("expected repository target type, got %q", actionCfg.TargetType)
	}
	if actionCfg.Output == nil || actionCfg.Output.Type != "docs_document" || actionCfg.Output.SpaceID != "space-1" {
		t.Fatalf("expected docs output config, got %+v", actionCfg.Output)
	}
}

func TestCreateAgentFromTemplateAppliesDrawerOverrides(t *testing.T) {
	db := newAgentServiceTestDB(t)
	addAgentTemplateTable(t, db)

	agentRepo := repository.NewAgentRepository(db)
	activitySvc := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := (&AgentService{
		agentRepo:   agentRepo,
		activitySvc: activitySvc,
	}).SetAgentTemplateRepository(repository.NewAgentTemplateRepository(db))

	if err := svc.EnsureSystemTemplates(context.Background()); err != nil {
		t.Fatalf("EnsureSystemTemplates returned error: %v", err)
	}
	template, err := svc.agentTemplateRepo.GetByKey(context.Background(), nil, model.AgentTemplateTypeReleaseNotes)
	if err != nil {
		t.Fatalf("GetByKey returned error: %v", err)
	}
	if template == nil {
		t.Fatal("expected seeded release notes template")
	}

	systemPrompt := "Write concise launch notes."
	runtimeKind := model.AgentTemplateRuntimeKindNativeSDK
	approvalMode := "always"
	invocationMode := model.InvocationModeInteractive
	maxConcurrentRuns := 3
	result, err := svc.CreateAgentFromTemplate(context.Background(), "ws-test", template.ID, model.CreateAgentFromTemplateRequest{
		Name: strPtr("Edited Release Notes Agent"),
		Overrides: &model.CreateAgentFromTemplateOverrides{
			RuntimeKind:           &runtimeKind,
			SystemPrompt:          &systemPrompt,
			AllowedTools:          model.JSONBlob(`["get_release_context","create_document"]`),
			AllowedTargets:        model.JSONBlob(`["repository","workspace"]`),
			Skills:                &model.AgentSkillRefs{{Key: "release_notes_writer"}},
			ApprovalMode:          &approvalMode,
			MaxConcurrentRuns:     &maxConcurrentRuns,
			DefaultInvocationMode: &invocationMode,
		},
	}, "user-1")
	if err != nil {
		t.Fatalf("CreateAgentFromTemplate returned error: %v", err)
	}
	if result.Agent == nil {
		t.Fatal("expected created agent")
	}
	if result.Agent.Name != "Edited Release Notes Agent" {
		t.Fatalf("expected edited name, got %q", result.Agent.Name)
	}
	if result.Agent.SystemPrompt == nil || *result.Agent.SystemPrompt != systemPrompt {
		t.Fatalf("expected edited system prompt, got %+v", result.Agent.SystemPrompt)
	}
	if result.Agent.ApprovalMode != approvalMode {
		t.Fatalf("expected approval mode %q, got %q", approvalMode, result.Agent.ApprovalMode)
	}
	if result.Agent.MaxConcurrentRuns != maxConcurrentRuns {
		t.Fatalf("expected max_concurrent_runs %d, got %d", maxConcurrentRuns, result.Agent.MaxConcurrentRuns)
	}
	if result.Agent.DefaultInvocationMode != invocationMode {
		t.Fatalf("expected default_invocation_mode %q, got %q", invocationMode, result.Agent.DefaultInvocationMode)
	}

	var allowedTargets []string
	if err := json.Unmarshal(result.Agent.AllowedTargets, &allowedTargets); err != nil {
		t.Fatalf("unmarshal allowed targets: %v", err)
	}
	if len(allowedTargets) != 2 || allowedTargets[0] != "repository" || allowedTargets[1] != "workspace" {
		t.Fatalf("expected overridden targets, got %v", allowedTargets)
	}

	var allowedTools []string
	if err := json.Unmarshal(result.Agent.AllowedTools, &allowedTools); err != nil {
		t.Fatalf("unmarshal allowed tools: %v", err)
	}
	if len(allowedTools) != 2 || allowedTools[0] != "get_release_context" || allowedTools[1] != "create_document" {
		t.Fatalf("expected overridden tools, got %v", allowedTools)
	}
}

func addAgentTemplateTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	statement := `CREATE TABLE agent_templates (
		id TEXT PRIMARY KEY,
		workspace_id TEXT,
		key TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		runtime_kind TEXT NOT NULL DEFAULT 'native_sdk',
		default_role TEXT NOT NULL DEFAULT '',
		execution_config BLOB NOT NULL DEFAULT x'7b7d',
		system_prompt TEXT,
		planning_notes TEXT,
		skills BLOB NOT NULL DEFAULT '[]',
		allowed_tools BLOB NOT NULL DEFAULT '[]',
		allowed_commands BLOB NOT NULL DEFAULT '[]',
		allowed_targets BLOB NOT NULL DEFAULT '[]',
		approval_mode TEXT NOT NULL DEFAULT 'preset_default',
		default_invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
		monthly_token_budget INTEGER,
		is_enabled BOOLEAN NOT NULL DEFAULT 1,
		created_by TEXT,
		updated_by TEXT,
		deleted_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME
	)`
	if err := db.Exec(statement).Error; err != nil {
		t.Fatalf("create agent_templates table: %v", err)
	}
}

func addAutomationRuleTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	statement := `CREATE TABLE automation_rules (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		enabled BOOLEAN NOT NULL DEFAULT 1,
		team_id TEXT,
		workflow_id TEXT,
		trigger_type TEXT NOT NULL,
		trigger_config BLOB NOT NULL DEFAULT '{}',
		action_type TEXT NOT NULL,
		action_config BLOB NOT NULL DEFAULT '{}',
		position INTEGER NOT NULL DEFAULT 0,
		stop_on_match BOOLEAN NOT NULL DEFAULT 0,
		created_by TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`
	if err := db.Exec(statement).Error; err != nil {
		t.Fatalf("create automation_rules table: %v", err)
	}
}
