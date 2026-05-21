package service

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

func TestListAgentTemplatesSeedsSystemTemplates(t *testing.T) {
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
	var requiredContext []string
	if err := json.Unmarshal(releaseNotes.RequiredContext, &requiredContext); err != nil {
		t.Fatalf("unmarshal required_context: %v", err)
	}
	if len(requiredContext) != 2 || requiredContext[0] != "event.github.release.tag_name" || requiredContext[1] != "event.github.release.repo_full_name" {
		t.Fatalf("expected release required context, got %v", requiredContext)
	}
	var starterFlows []map[string]any
	if err := json.Unmarshal(releaseNotes.StarterFlows, &starterFlows); err != nil {
		t.Fatalf("unmarshal starter_flows: %v", err)
	}
	if len(starterFlows) != 1 || starterFlows[0]["trigger_type"] != model.TriggerGitHubReleasePub {
		t.Fatalf("expected github release starter flow, got %+v", starterFlows)
	}

	var competitiveIntel *model.AgentTemplate
	for idx := range templates {
		if templates[idx].Key == model.AgentTemplateTypeCompetitiveIntel {
			competitiveIntel = &templates[idx]
			break
		}
	}
	if competitiveIntel == nil {
		t.Fatalf("expected %q template, got %+v", model.AgentTemplateTypeCompetitiveIntel, templates)
	}
	if competitiveIntel.Name != "Competitive Intelligence Digest" {
		t.Fatalf("expected Competitive Intelligence Digest template name, got %q", competitiveIntel.Name)
	}
	if len(competitiveIntel.Skills) != 1 || competitiveIntel.Skills[0].Key != model.AgentTemplateTypeCompetitiveIntel {
		t.Fatalf("expected competitive_intelligence_digest skill ref, got %+v", competitiveIntel.Skills)
	}
	if competitiveIntel.SystemPrompt == nil || !strings.Contains(*competitiveIntel.SystemPrompt, "{{target_company}}") || !strings.Contains(*competitiveIntel.SystemPrompt, "{{raw_configuration_json}}") {
		t.Fatalf("expected competitive template prompt placeholders, got %+v", competitiveIntel.SystemPrompt)
	}
	var competitiveAllowedTools []string
	if err := json.Unmarshal(competitiveIntel.AllowedTools, &competitiveAllowedTools); err != nil {
		t.Fatalf("unmarshal competitive allowed tools: %v", err)
	}
	for _, tool := range []string{"web_search_exa", "fetch_url", "crawl_url", "create_task"} {
		if !slices.Contains(competitiveAllowedTools, tool) {
			t.Fatalf("expected competitive template allowed tools to include %q, got %v", tool, competitiveAllowedTools)
		}
	}
	var competitiveTargets []string
	if err := json.Unmarshal(competitiveIntel.AllowedTargets, &competitiveTargets); err != nil {
		t.Fatalf("unmarshal competitive allowed targets: %v", err)
	}
	if len(competitiveTargets) != 1 || competitiveTargets[0] != "workspace" {
		t.Fatalf("expected workspace target, got %v", competitiveTargets)
	}
	var competitiveFlows []map[string]any
	if err := json.Unmarshal(competitiveIntel.StarterFlows, &competitiveFlows); err != nil {
		t.Fatalf("unmarshal competitive starter_flows: %v", err)
	}
	if len(competitiveFlows) != 1 || competitiveFlows[0]["trigger_type"] != model.TriggerCron {
		t.Fatalf("expected cron starter flow, got %+v", competitiveFlows)
	}
	fields, ok := competitiveFlows[0]["fields"].([]any)
	if !ok || len(fields) == 0 {
		t.Fatalf("expected competitive starter flow fields, got %+v", competitiveFlows[0]["fields"])
	}

	var dependencyAuditor *model.AgentTemplate
	for idx := range templates {
		if templates[idx].Key == model.AgentTemplateTypeDependencyAuditor {
			dependencyAuditor = &templates[idx]
			break
		}
	}
	if dependencyAuditor == nil {
		t.Fatalf("expected %q template, got %+v", model.AgentTemplateTypeDependencyAuditor, templates)
	}
	if dependencyAuditor.Name != "Dependency Auditor" {
		t.Fatalf("expected Dependency Auditor template name, got %q", dependencyAuditor.Name)
	}
	if len(dependencyAuditor.Skills) != 1 || dependencyAuditor.Skills[0].Key != model.AgentTemplateTypeDependencyAuditor {
		t.Fatalf("expected dependency_auditor skill ref, got %+v", dependencyAuditor.Skills)
	}
	if dependencyAuditor.SystemPrompt == nil || !strings.Contains(*dependencyAuditor.SystemPrompt, "{{ecosystems}}") || !strings.Contains(*dependencyAuditor.SystemPrompt, "{{raw_configuration_json}}") {
		t.Fatalf("expected dependency template prompt placeholders, got %+v", dependencyAuditor.SystemPrompt)
	}
	var dependencyTargets []string
	if err := json.Unmarshal(dependencyAuditor.AllowedTargets, &dependencyTargets); err != nil {
		t.Fatalf("unmarshal dependency allowed targets: %v", err)
	}
	if len(dependencyTargets) != 1 || dependencyTargets[0] != "repository" {
		t.Fatalf("expected repository target, got %v", dependencyTargets)
	}
	var dependencyFlows []map[string]any
	if err := json.Unmarshal(dependencyAuditor.StarterFlows, &dependencyFlows); err != nil {
		t.Fatalf("unmarshal dependency starter_flows: %v", err)
	}
	if len(dependencyFlows) != 1 || dependencyFlows[0]["trigger_type"] != model.TriggerCron {
		t.Fatalf("expected cron starter flow, got %+v", dependencyFlows)
	}

	var sentinel *model.AgentTemplate
	for idx := range templates {
		if templates[idx].Key == model.AgentTemplateTypeSecurityTriage {
			sentinel = &templates[idx]
			break
		}
	}
	if sentinel == nil {
		t.Fatalf("expected %q template, got %+v", model.AgentTemplateTypeSecurityTriage, templates)
	}
	if sentinel.Name != "Sentinel" {
		t.Fatalf("expected Sentinel template name, got %q", sentinel.Name)
	}
	if len(sentinel.Skills) != 1 || sentinel.Skills[0].Key != model.AgentTemplateTypeSecurityTriage {
		t.Fatalf("expected security_triage skill ref, got %+v", sentinel.Skills)
	}
	if sentinel.SystemPrompt == nil || !strings.Contains(*sentinel.SystemPrompt, "{{scanners}}") || !strings.Contains(*sentinel.SystemPrompt, "{{raw_configuration_json}}") {
		t.Fatalf("expected Sentinel template prompt placeholders, got %+v", sentinel.SystemPrompt)
	}
	if !strings.Contains(*sentinel.SystemPrompt, "scan_semgrep") || !strings.Contains(*sentinel.SystemPrompt, "Do not run scanner CLIs through run_command") {
		t.Fatalf("expected Sentinel prompt to require scanner tools, got %s", *sentinel.SystemPrompt)
	}
	for _, want := range []string{
		"returned security label_id",
		"open_only: true",
		`detail_level: "compact"`,
		"Do not request full descriptions/comments",
		"Parse Sentinel markers",
		"single-line <!-- sentinel:root_cause=...",
		`detail_level: "index"`,
		"limit: 100",
		"Do not filter existing-task lookup by destination_state_id",
	} {
		if !strings.Contains(*sentinel.SystemPrompt, want) {
			t.Fatalf("expected Sentinel prompt to include %q, got %s", want, *sentinel.SystemPrompt)
		}
	}
	var sentinelTargets []string
	if err := json.Unmarshal(sentinel.AllowedTargets, &sentinelTargets); err != nil {
		t.Fatalf("unmarshal Sentinel allowed targets: %v", err)
	}
	if len(sentinelTargets) != 1 || sentinelTargets[0] != "repository" {
		t.Fatalf("expected repository target, got %v", sentinelTargets)
	}
	var sentinelTools []string
	if err := json.Unmarshal(sentinel.AllowedTools, &sentinelTools); err != nil {
		t.Fatalf("unmarshal Sentinel allowed tools: %v", err)
	}
	for _, tool := range []string{"scan_semgrep", "scan_trivy", "scan_gitleaks"} {
		if !slices.Contains(sentinelTools, tool) {
			t.Fatalf("expected Sentinel tool %q in %v", tool, sentinelTools)
		}
	}
	var sentinelCommands []string
	if err := json.Unmarshal(sentinel.AllowedCommands, &sentinelCommands); err != nil {
		t.Fatalf("unmarshal Sentinel allowed commands: %v", err)
	}
	for _, command := range []string{"semgrep", "trivy", "gitleaks", "python3"} {
		if slices.Contains(sentinelCommands, command) {
			t.Fatalf("did not expect Sentinel command %q in %v", command, sentinelCommands)
		}
	}
	var sentinelFlows []map[string]any
	if err := json.Unmarshal(sentinel.StarterFlows, &sentinelFlows); err != nil {
		t.Fatalf("unmarshal Sentinel starter_flows: %v", err)
	}
	if len(sentinelFlows) != 1 || sentinelFlows[0]["trigger_type"] != model.TriggerCron {
		t.Fatalf("expected cron starter flow, got %+v", sentinelFlows)
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

func TestCreateAgentFromCompetitiveIntelTemplateCreatesCronStarterFlow(t *testing.T) {
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
	template, err := svc.agentTemplateRepo.GetByKey(context.Background(), nil, model.AgentTemplateTypeCompetitiveIntel)
	if err != nil {
		t.Fatalf("GetByKey returned error: %v", err)
	}
	if template == nil {
		t.Fatal("expected seeded competitive intel template")
	}

	flowInput := model.JSONBlob(`{
		"target_company": "Usermaven",
		"target_domain": "usermaven.com",
		"competitors": ["jasper.ai", "writesonic.ai"],
		"schedule_preset": "daily",
		"lookback_days": 7,
		"destination_team_id": "team-marketing",
		"destination_state_id": "state-todo"
	}`)
	result, err := svc.CreateAgentFromTemplate(context.Background(), "ws-test", template.ID, model.CreateAgentFromTemplateRequest{
		CreateFlow: true,
		Flow: &model.CreateAgentFromTemplateFlow{
			FlowKey:   "competitive_intel_scheduled",
			FlowInput: flowInput,
		},
	}, "user-1")
	if err != nil {
		t.Fatalf("CreateAgentFromTemplate returned error: %v", err)
	}
	if result.Agent == nil {
		t.Fatal("expected created agent")
	}
	if result.Agent.SourceTemplateKey != model.AgentTemplateTypeCompetitiveIntel {
		t.Fatalf("expected source_template_key %q, got %q", model.AgentTemplateTypeCompetitiveIntel, result.Agent.SourceTemplateKey)
	}
	if result.Agent.SystemPrompt == nil || !strings.Contains(*result.Agent.SystemPrompt, "You are a competitive intelligence agent for Usermaven") || !strings.Contains(*result.Agent.SystemPrompt, "Do not plan or perform discovery of configuration variables") || !strings.Contains(*result.Agent.SystemPrompt, `- competitors: jasper.ai, writesonic.ai`) || !strings.Contains(*result.Agent.SystemPrompt, `"target_company": "Usermaven"`) || !strings.Contains(*result.Agent.SystemPrompt, `"schedule_preset": "daily"`) || !strings.Contains(*result.Agent.SystemPrompt, `"destination_team_id": "team-marketing"`) || strings.Contains(*result.Agent.SystemPrompt, "{{target_company}}") {
		t.Fatalf("expected configured system prompt, got %+v", result.Agent.SystemPrompt)
	}
	var allowedTargets []string
	if err := json.Unmarshal(result.Agent.AllowedTargets, &allowedTargets); err != nil {
		t.Fatalf("unmarshal allowed targets: %v", err)
	}
	if len(allowedTargets) != 1 || allowedTargets[0] != "workspace" {
		t.Fatalf("expected workspace target, got %v", allowedTargets)
	}
	if result.Flow == nil {
		t.Fatal("expected starter flow")
	}
	if result.Flow.TriggerType != model.TriggerCron {
		t.Fatalf("expected cron trigger, got %q", result.Flow.TriggerType)
	}

	var triggerCfg model.TriggerConfigCron
	if err := json.Unmarshal(result.Flow.TriggerConfig, &triggerCfg); err != nil {
		t.Fatalf("unmarshal trigger config: %v", err)
	}
	if triggerCfg.Preset != "daily" {
		t.Fatalf("expected daily preset, got %+v", triggerCfg)
	}

	var actionCfg model.ActionConfigRunAgent
	if err := json.Unmarshal(result.Flow.ActionConfig, &actionCfg); err != nil {
		t.Fatalf("unmarshal action config: %v", err)
	}
	if actionCfg.AgentID != result.Agent.ID {
		t.Fatalf("expected flow agent_id %q, got %q", result.Agent.ID, actionCfg.AgentID)
	}
	if actionCfg.TargetType != "workspace" || actionCfg.TargetID != "ws-test" {
		t.Fatalf("expected workspace target ws-test, got %q/%q", actionCfg.TargetType, actionCfg.TargetID)
	}
	if actionCfg.AdditionalContext != nil && strings.TrimSpace(*actionCfg.AdditionalContext) != "" {
		t.Fatalf("expected no starter-flow additional context, got %+v", actionCfg.AdditionalContext)
	}
}

func TestCreateAgentFromCompetitiveIntelTemplateValidatesFlowInput(t *testing.T) {
	tests := []struct {
		name      string
		flowInput model.JSONBlob
		wantError string
	}{
		{
			name:      "missing target company",
			flowInput: model.JSONBlob(`{"destination_team_id":"team-1"}`),
			wantError: "flow.flow_input.target_company is required",
		},
		{
			name:      "missing destination team",
			flowInput: model.JSONBlob(`{"target_company":"Usermaven"}`),
			wantError: "flow.flow_input.destination_team_id is required",
		},
		{
			name:      "invalid lookback",
			flowInput: model.JSONBlob(`{"target_company":"Usermaven","destination_team_id":"team-1","schedule_preset":"weekly","lookback_days":60}`),
			wantError: "flow.flow_input.lookback_days must be between 1 and 30",
		},
		{
			name:      "invalid schedule preset",
			flowInput: model.JSONBlob(`{"target_company":"Usermaven","destination_team_id":"team-1","schedule_preset":"monthly"}`),
			wantError: "flow.flow_input.schedule_preset must be daily or weekly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := competitiveIntelInputFromTemplateFlow(&model.CreateAgentFromTemplateFlow{
				FlowKey:   "competitive_intel_scheduled",
				FlowInput: tt.flowInput,
			})
			if err == nil || err.Error() != tt.wantError {
				t.Fatalf("expected error %q, got %v", tt.wantError, err)
			}
		})
	}
}

func TestCreateAgentFromDependencyAuditorTemplateCreatesCronRepositoryStarterFlow(t *testing.T) {
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
	template, err := svc.agentTemplateRepo.GetByKey(context.Background(), nil, model.AgentTemplateTypeDependencyAuditor)
	if err != nil {
		t.Fatalf("GetByKey returned error: %v", err)
	}
	if template == nil {
		t.Fatal("expected seeded dependency auditor template")
	}

	flowInput := model.JSONBlob(`{
		"ecosystems": ["go", "rust", "python", "node", "java"],
		"include_indirect": false,
		"schedule_preset": "weekly",
		"destination_team_id": "team-platform",
		"destination_state_id": "state-todo",
		"max_tasks": 20
	}`)
	result, err := svc.CreateAgentFromTemplate(context.Background(), "ws-test", template.ID, model.CreateAgentFromTemplateRequest{
		CreateFlow: true,
		Flow: &model.CreateAgentFromTemplateFlow{
			FlowKey:      "dependency_audit_cron",
			FlowInput:    flowInput,
			RepositoryID: "repo-1",
			RepoFullName: "acme/api",
		},
	}, "user-1")
	if err != nil {
		t.Fatalf("CreateAgentFromTemplate returned error: %v", err)
	}
	if result.Agent == nil {
		t.Fatal("expected created agent")
	}
	if result.Agent.SourceTemplateKey != model.AgentTemplateTypeDependencyAuditor {
		t.Fatalf("expected source_template_key %q, got %q", model.AgentTemplateTypeDependencyAuditor, result.Agent.SourceTemplateKey)
	}
	if result.Agent.SystemPrompt == nil || !strings.Contains(*result.Agent.SystemPrompt, "You are an autonomous dependency auditor for the selected repository") || !strings.Contains(*result.Agent.SystemPrompt, `- ecosystems: go, rust, python, node, java`) || !strings.Contains(*result.Agent.SystemPrompt, `- destination_team_id: team-platform`) || !strings.Contains(*result.Agent.SystemPrompt, `"max_tasks": 20`) || strings.Contains(*result.Agent.SystemPrompt, "{{ecosystems}}") {
		t.Fatalf("expected configured system prompt, got %+v", result.Agent.SystemPrompt)
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
	if result.Flow.TriggerType != model.TriggerCron {
		t.Fatalf("expected cron trigger, got %q", result.Flow.TriggerType)
	}

	var triggerCfg model.TriggerConfigCron
	if err := json.Unmarshal(result.Flow.TriggerConfig, &triggerCfg); err != nil {
		t.Fatalf("unmarshal trigger config: %v", err)
	}
	if triggerCfg.Preset != "weekly" {
		t.Fatalf("expected weekly preset, got %+v", triggerCfg)
	}

	var actionCfg model.ActionConfigRunAgent
	if err := json.Unmarshal(result.Flow.ActionConfig, &actionCfg); err != nil {
		t.Fatalf("unmarshal action config: %v", err)
	}
	if actionCfg.AgentID != result.Agent.ID {
		t.Fatalf("expected flow agent_id %q, got %q", result.Agent.ID, actionCfg.AgentID)
	}
	if actionCfg.TargetType != "repository" || actionCfg.TargetID != "repo-1" {
		t.Fatalf("expected repository target repo-1, got %q/%q", actionCfg.TargetType, actionCfg.TargetID)
	}
}

func TestCreateAgentFromDependencyAuditorTemplateValidatesFlowInput(t *testing.T) {
	tests := []struct {
		name      string
		flowInput model.JSONBlob
		wantError string
	}{
		{
			name:      "missing ecosystems",
			flowInput: model.JSONBlob(`{"destination_team_id":"team-1","schedule_preset":"weekly","max_tasks":20}`),
			wantError: "flow.flow_input.ecosystems requires at least one of go, rust, python, node, or java",
		},
		{
			name:      "invalid ecosystem",
			flowInput: model.JSONBlob(`{"ecosystems":["php"],"destination_team_id":"team-1","schedule_preset":"weekly","max_tasks":20}`),
			wantError: "flow.flow_input.ecosystems must only include go, rust, python, node, or java",
		},
		{
			name:      "missing destination team",
			flowInput: model.JSONBlob(`{"ecosystems":["go"],"schedule_preset":"weekly","max_tasks":20}`),
			wantError: "flow.flow_input.destination_team_id is required",
		},
		{
			name:      "invalid schedule preset",
			flowInput: model.JSONBlob(`{"ecosystems":["go"],"destination_team_id":"team-1","schedule_preset":"monthly","max_tasks":20}`),
			wantError: "flow.flow_input.schedule_preset must be daily or weekly",
		},
		{
			name:      "invalid max tasks",
			flowInput: model.JSONBlob(`{"ecosystems":["go"],"destination_team_id":"team-1","schedule_preset":"weekly","max_tasks":200}`),
			wantError: "flow.flow_input.max_tasks must be between 1 and 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := dependencyAuditorInputFromTemplateFlow(&model.CreateAgentFromTemplateFlow{
				FlowKey:   "dependency_audit_cron",
				FlowInput: tt.flowInput,
			})
			if err == nil || err.Error() != tt.wantError {
				t.Fatalf("expected error %q, got %v", tt.wantError, err)
			}
		})
	}
}

func TestCreateAgentFromSecurityTriageTemplateCreatesCronRepositoryStarterFlow(t *testing.T) {
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
	template, err := svc.agentTemplateRepo.GetByKey(context.Background(), nil, model.AgentTemplateTypeSecurityTriage)
	if err != nil {
		t.Fatalf("GetByKey returned error: %v", err)
	}
	if template == nil {
		t.Fatal("expected seeded Sentinel template")
	}

	flowInput := model.JSONBlob(`{
		"scanners": ["semgrep", "trivy", "gitleaks"],
		"severity_threshold": "medium",
		"include_low_info": false,
		"schedule_preset": "weekly",
		"destination_team_id": "team-security",
		"destination_state_id": "state-todo",
		"max_tasks": 20
	}`)
	result, err := svc.CreateAgentFromTemplate(context.Background(), "ws-test", template.ID, model.CreateAgentFromTemplateRequest{
		CreateFlow: true,
		Flow: &model.CreateAgentFromTemplateFlow{
			FlowKey:      "security_triage_cron",
			FlowInput:    flowInput,
			RepositoryID: "repo-1",
			RepoFullName: "acme/api",
		},
	}, "user-1")
	if err != nil {
		t.Fatalf("CreateAgentFromTemplate returned error: %v", err)
	}
	if result.Agent == nil {
		t.Fatal("expected created agent")
	}
	if result.Agent.SourceTemplateKey != model.AgentTemplateTypeSecurityTriage {
		t.Fatalf("expected source_template_key %q, got %q", model.AgentTemplateTypeSecurityTriage, result.Agent.SourceTemplateKey)
	}
	if result.Agent.SystemPrompt == nil || !strings.Contains(*result.Agent.SystemPrompt, "You are Sentinel") || !strings.Contains(*result.Agent.SystemPrompt, `- scanners: semgrep, trivy, gitleaks`) || !strings.Contains(*result.Agent.SystemPrompt, `- destination_team_id: team-security`) || !strings.Contains(*result.Agent.SystemPrompt, `"severity_threshold": "medium"`) || strings.Contains(*result.Agent.SystemPrompt, "{{scanners}}") {
		t.Fatalf("expected configured system prompt, got %+v", result.Agent.SystemPrompt)
	}
	for _, want := range []string{
		"returned security label_id",
		"open_only: true",
		`detail_level: "compact"`,
		"Do not request full descriptions/comments",
		"Parse Sentinel markers",
		"single-line <!-- sentinel:root_cause=...",
		`detail_level: "index"`,
		"limit: 100",
		"Do not filter existing-task lookup by destination_state_id",
	} {
		if !strings.Contains(*result.Agent.SystemPrompt, want) {
			t.Fatalf("expected configured Sentinel prompt to include %q, got %s", want, *result.Agent.SystemPrompt)
		}
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
	if result.Flow.TriggerType != model.TriggerCron {
		t.Fatalf("expected cron trigger, got %q", result.Flow.TriggerType)
	}

	var triggerCfg model.TriggerConfigCron
	if err := json.Unmarshal(result.Flow.TriggerConfig, &triggerCfg); err != nil {
		t.Fatalf("unmarshal trigger config: %v", err)
	}
	if triggerCfg.Preset != "weekly" {
		t.Fatalf("expected weekly preset, got %+v", triggerCfg)
	}

	var actionCfg model.ActionConfigRunAgent
	if err := json.Unmarshal(result.Flow.ActionConfig, &actionCfg); err != nil {
		t.Fatalf("unmarshal action config: %v", err)
	}
	if actionCfg.AgentID != result.Agent.ID {
		t.Fatalf("expected flow agent_id %q, got %q", result.Agent.ID, actionCfg.AgentID)
	}
	if actionCfg.TargetType != "repository" || actionCfg.TargetID != "repo-1" {
		t.Fatalf("expected repository target repo-1, got %q/%q", actionCfg.TargetType, actionCfg.TargetID)
	}
}

func TestCreateAgentFromSecurityTriageTemplateValidatesFlowInput(t *testing.T) {
	tests := []struct {
		name      string
		flowInput model.JSONBlob
		wantError string
	}{
		{
			name:      "missing scanners",
			flowInput: model.JSONBlob(`{"severity_threshold":"medium","destination_team_id":"team-1","schedule_preset":"weekly","max_tasks":20}`),
			wantError: "flow.flow_input.scanners requires at least one of semgrep, trivy, or gitleaks",
		},
		{
			name:      "invalid scanner",
			flowInput: model.JSONBlob(`{"scanners":["sonarqube"],"severity_threshold":"medium","destination_team_id":"team-1","schedule_preset":"weekly","max_tasks":20}`),
			wantError: "flow.flow_input.scanners must only include semgrep, trivy, or gitleaks",
		},
		{
			name:      "invalid severity",
			flowInput: model.JSONBlob(`{"scanners":["semgrep"],"severity_threshold":"low","destination_team_id":"team-1","schedule_preset":"weekly","max_tasks":20}`),
			wantError: "flow.flow_input.severity_threshold must be critical, high, or medium",
		},
		{
			name:      "missing destination team",
			flowInput: model.JSONBlob(`{"scanners":["semgrep"],"severity_threshold":"medium","schedule_preset":"weekly","max_tasks":20}`),
			wantError: "flow.flow_input.destination_team_id is required",
		},
		{
			name:      "invalid schedule preset",
			flowInput: model.JSONBlob(`{"scanners":["semgrep"],"severity_threshold":"medium","destination_team_id":"team-1","schedule_preset":"monthly","max_tasks":20}`),
			wantError: "flow.flow_input.schedule_preset must be daily or weekly",
		},
		{
			name:      "invalid max tasks",
			flowInput: model.JSONBlob(`{"scanners":["semgrep"],"severity_threshold":"medium","destination_team_id":"team-1","schedule_preset":"weekly","max_tasks":200}`),
			wantError: "flow.flow_input.max_tasks must be between 1 and 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := securityTriageInputFromTemplateFlow(&model.CreateAgentFromTemplateFlow{
				FlowKey:   "security_triage_cron",
				FlowInput: tt.flowInput,
			})
			if err == nil || err.Error() != tt.wantError {
				t.Fatalf("expected error %q, got %v", tt.wantError, err)
			}
		})
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
		required_context BLOB NOT NULL DEFAULT '[]',
		starter_flows BLOB NOT NULL DEFAULT '[]',
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
		template_key TEXT,
		template_instance_id TEXT,
		template_version INTEGER,
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
