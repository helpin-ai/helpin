package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func builtInAgentTemplates() []model.AgentTemplate {
	releaseNotesDescription := "Creates release notes from GitHub releases, related tasks, and linked docs."
	releaseNotesStarterFlows := model.JSONBlob(mustJSONValue([]map[string]any{
		{
			"key":               "github_release_notes",
			"label":             "Start on GitHub releases",
			"description":       "Runs when GitHub publishes a matching release and writes release notes to Docs.",
			"trigger_type":      model.TriggerGitHubReleasePub,
			"default_enabled":   true,
			"config_schema_key": "release_notes_github",
			"output_type":       "docs_document",
		},
	}))
	competitiveIntelDescription := "Tracks recent competitor launches and creates a recurring marketing digest task."
	competitiveIntelSystemPrompt := `You are a competitive intelligence agent for {{target_company}}.

Configured digest:
- target_company: {{target_company}}
- target_domain: {{target_domain}}
- competitors: {{competitors}}
- lookback_days: {{lookback_days}}
- destination_team_id: {{destination_team_id}}
- destination_state_id: {{destination_state_id}}
- schedule_preset: {{schedule_preset}}

Treat these configured values as already resolved and authoritative. Do not plan or perform discovery of configuration variables, workspace context, teams, stages, cadence, or lookback settings.

Use the configured competitor list when it is not empty. If no competitors are configured, discover competitors with web search and cite sources.

Create exactly one marketing digest task with create_task. Pass destination_team_id directly as team_id. Pass destination_state_id directly as state_id only when it is configured; otherwise let the team default stage apply.

Raw configuration:
{{raw_configuration_json}}`
	competitiveIntelStarterFlows := model.JSONBlob(mustJSONValue([]map[string]any{
		{
			"key":               "competitive_intel_scheduled",
			"label":             "Run competitor digest on a schedule",
			"description":       "Runs on the selected cadence, researches recent competitor updates, and creates one marketing task.",
			"trigger_type":      model.TriggerCron,
			"default_enabled":   true,
			"config_schema_key": "competitive_intel_cron",
			"output_type":       "task",
			"fields": []map[string]any{
				{
					"key":         "target_company",
					"label":       "Target company",
					"type":        "text",
					"required":    true,
					"placeholder": "Usermaven",
				},
				{
					"key":         "target_domain",
					"label":       "Target domain",
					"type":        "text",
					"required":    false,
					"placeholder": "usermaven.com",
				},
				{
					"key":         "competitors",
					"label":       "Known competitors",
					"type":        "string_list",
					"required":    false,
					"placeholder": "jasper.ai",
					"help_text":   "Optional. If empty, the agent discovers competitors during each run.",
				},
				{
					"key":      "schedule_preset",
					"label":    "Run cadence",
					"type":     "select",
					"required": true,
					"default":  "weekly",
					"options": []map[string]any{
						{"value": "daily", "label": "Daily"},
						{"value": "weekly", "label": "Weekly"},
					},
				},
				{
					"key":      "lookback_days",
					"label":    "Lookback window",
					"type":     "number",
					"required": true,
					"default":  7,
					"min":      1,
					"max":      30,
				},
				{
					"key":      "destination_team_id",
					"label":    "Task team",
					"type":     "team_select",
					"required": true,
				},
				{
					"key":        "destination_state_id",
					"label":      "Task stage",
					"type":       "workflow_state_select",
					"required":   false,
					"depends_on": "destination_team_id",
					"help_text":  "Optional. Defaults to the team's default stage.",
				},
			},
		},
	}))
	return []model.AgentTemplate{
		{
			Key:         model.AgentTemplateTypeReleaseNotes,
			Name:        "Release Notes Writer",
			Description: &releaseNotesDescription,
			RuntimeKind: model.AgentTemplateRuntimeKindNativeSDK,
			DefaultRole: "Release Notes Writer",
			Skills: model.AgentSkillRefs{
				{Key: model.AgentTemplateTypeReleaseNotes},
			},
			AllowedTools: model.JSONBlob(mustJSONStringSlice([]string{
				"get_release_context",
				"get_task_context",
				"find_tasks_for_git_changes",
				"read_document",
				"search_documents",
				"list_collections",
				"create_document",
				"write_document_content",
				"link_document_to_object",
			})),
			AllowedCommands:       model.JSONBlob(mustJSONStringSlice(nil)),
			AllowedTargets:        model.JSONBlob(mustJSONStringSlice([]string{"repository"})),
			RequiredContext:       model.JSONBlob(mustJSONStringSlice([]string{"event.github.release.tag_name", "event.github.release.repo_full_name"})),
			StarterFlows:          releaseNotesStarterFlows,
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeAutonomous,
			IsEnabled:             true,
		},
		{
			Key:          model.AgentTemplateTypeCompetitiveIntel,
			Name:         "Competitive Intelligence Digest",
			Description:  &competitiveIntelDescription,
			SystemPrompt: &competitiveIntelSystemPrompt,
			RuntimeKind:  model.AgentTemplateRuntimeKindNativeSDK,
			DefaultRole:  "Competitive Intelligence Analyst",
			Skills: model.AgentSkillRefs{
				{Key: model.AgentTemplateTypeCompetitiveIntel},
			},
			AllowedTools: model.JSONBlob(mustJSONStringSlice([]string{
				"update_plan",
				"web_search_exa",
				"list_workspace_teams",
				"list_team_workflows_with_stages",
				"create_task",
			})),
			AllowedCommands:       model.JSONBlob(mustJSONStringSlice(nil)),
			AllowedTargets:        model.JSONBlob(mustJSONStringSlice([]string{"workspace"})),
			RequiredContext:       model.JSONBlob(mustJSONStringSlice(nil)),
			StarterFlows:          competitiveIntelStarterFlows,
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeAutonomous,
			IsEnabled:             true,
		},
	}
}

func (s *AgentService) EnsureSystemTemplates(ctx context.Context) error {
	if s == nil || s.agentTemplateRepo == nil {
		return nil
	}
	for _, desired := range builtInAgentTemplates() {
		if err := s.ensureSystemTemplate(ctx, desired); err != nil {
			return err
		}
	}
	return nil
}

func (s *AgentService) ensureSystemTemplate(ctx context.Context, desired model.AgentTemplate) error {
	existing, err := s.agentTemplateRepo.GetByKey(ctx, nil, desired.Key)
	if err != nil {
		return err
	}
	if existing == nil {
		template := desired
		template.WorkspaceID = nil
		return s.agentTemplateRepo.Create(ctx, &template)
	}

	changed := false
	if strings.TrimSpace(existing.Name) != strings.TrimSpace(desired.Name) {
		existing.Name = desired.Name
		changed = true
	}
	if stringOrDefault(existing.Description, "") != stringOrDefault(desired.Description, "") {
		existing.Description = trimPtr(desired.Description)
		changed = true
	}
	if strings.TrimSpace(existing.RuntimeKind) != strings.TrimSpace(desired.RuntimeKind) {
		existing.RuntimeKind = desired.RuntimeKind
		changed = true
	}
	if strings.TrimSpace(existing.DefaultRole) != strings.TrimSpace(desired.DefaultRole) {
		existing.DefaultRole = desired.DefaultRole
		changed = true
	}
	if string(normalizeExecutionConfigJSON(existing.ExecutionConfig)) != string(normalizeExecutionConfigJSON(desired.ExecutionConfig)) {
		existing.ExecutionConfig = normalizeExecutionConfigJSON(desired.ExecutionConfig)
		changed = true
	}
	if stringOrDefault(existing.SystemPrompt, "") != stringOrDefault(desired.SystemPrompt, "") {
		existing.SystemPrompt = trimPtr(desired.SystemPrompt)
		changed = true
	}
	if stringOrDefault(existing.PlanningNotes, "") != stringOrDefault(desired.PlanningNotes, "") {
		existing.PlanningNotes = trimPtr(desired.PlanningNotes)
		changed = true
	}
	if !equalAgentSkillRefs(existing.Skills, desired.Skills) {
		existing.Skills = desired.Skills.Normalize()
		changed = true
	}
	if string(normalizeAllowedToolsJSON(json.RawMessage(existing.AllowedTools))) != string(normalizeAllowedToolsJSON(json.RawMessage(desired.AllowedTools))) {
		existing.AllowedTools = model.JSONBlob(normalizeAllowedToolsJSON(json.RawMessage(desired.AllowedTools)))
		changed = true
	}
	if string(normalizeJSONSlice(json.RawMessage(existing.AllowedCommands))) != string(normalizeJSONSlice(json.RawMessage(desired.AllowedCommands))) {
		existing.AllowedCommands = model.JSONBlob(normalizeJSONSlice(json.RawMessage(desired.AllowedCommands)))
		changed = true
	}
	if string(normalizeJSONSlice(json.RawMessage(existing.AllowedTargets))) != string(normalizeJSONSlice(json.RawMessage(desired.AllowedTargets))) {
		existing.AllowedTargets = model.JSONBlob(normalizeJSONSlice(json.RawMessage(desired.AllowedTargets)))
		changed = true
	}
	if string(normalizeJSONSlice(json.RawMessage(existing.RequiredContext))) != string(normalizeJSONSlice(json.RawMessage(desired.RequiredContext))) {
		existing.RequiredContext = model.JSONBlob(normalizeJSONSlice(json.RawMessage(desired.RequiredContext)))
		changed = true
	}
	if string(normalizeJSONValue(existing.StarterFlows)) != string(normalizeJSONValue(desired.StarterFlows)) {
		existing.StarterFlows = model.JSONBlob(normalizeJSONValue(desired.StarterFlows))
		changed = true
	}
	if strings.TrimSpace(existing.ApprovalMode) != strings.TrimSpace(desired.ApprovalMode) {
		existing.ApprovalMode = desired.ApprovalMode
		changed = true
	}
	if strings.TrimSpace(existing.DefaultInvocationMode) != strings.TrimSpace(desired.DefaultInvocationMode) {
		existing.DefaultInvocationMode = desired.DefaultInvocationMode
		changed = true
	}
	if normalizeTokenBudget(existing.MonthlyTokenBudget) != normalizeTokenBudget(desired.MonthlyTokenBudget) {
		existing.MonthlyTokenBudget = normalizeTokenBudget(desired.MonthlyTokenBudget)
		changed = true
	}
	if existing.IsEnabled != desired.IsEnabled {
		existing.IsEnabled = desired.IsEnabled
		changed = true
	}
	if changed {
		return s.agentTemplateRepo.Update(ctx, existing)
	}
	return nil
}

func equalAgentSkillRefs(a, b model.AgentSkillRefs) bool {
	left := a.Normalize()
	right := b.Normalize()
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if stringOrDefault(left[idx].SkillID, "") != stringOrDefault(right[idx].SkillID, "") {
			return false
		}
		if strings.TrimSpace(left[idx].Key) != strings.TrimSpace(right[idx].Key) {
			return false
		}
		if stringOrDefault(left[idx].VersionKey, "") != stringOrDefault(right[idx].VersionKey, "") {
			return false
		}
		if string(normalizeExecutionConfigJSON(left[idx].Config)) != string(normalizeExecutionConfigJSON(right[idx].Config)) {
			return false
		}
	}
	return true
}

func mustJSONValue(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("null")
	}
	return payload
}

func normalizeJSONValue(raw model.JSONBlob) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage("[]")
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return json.RawMessage("[]")
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("[]")
	}
	return payload
}

func (s *AgentService) ListAgentTemplates(ctx context.Context, workspaceID string) ([]model.AgentTemplate, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if err := s.EnsureSystemTemplates(ctx); err != nil {
		return nil, err
	}
	if s.agentTemplateRepo == nil {
		return []model.AgentTemplate{}, nil
	}
	return s.agentTemplateRepo.ListVisible(ctx, workspaceID)
}

func (s *AgentService) GetAgentTemplate(ctx context.Context, workspaceID, templateID string) (*model.AgentTemplate, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if strings.TrimSpace(templateID) == "" {
		return nil, fmt.Errorf("template_id is required")
	}
	if err := s.EnsureSystemTemplates(ctx); err != nil {
		return nil, err
	}
	if s.agentTemplateRepo == nil {
		return nil, fmt.Errorf("agent template repository is not configured")
	}
	template, err := s.agentTemplateRepo.GetByID(ctx, workspaceID, templateID)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, fmt.Errorf("agent template not found")
	}
	return template, nil
}

func (s *AgentService) CreateAgentFromTemplate(ctx context.Context, workspaceID, templateID string, req model.CreateAgentFromTemplateRequest, actorID string) (*model.CreateAgentFromTemplateResponse, error) {
	template, err := s.GetAgentTemplate(ctx, workspaceID, templateID)
	if err != nil {
		return nil, err
	}

	createReq := materializeCreateAgentRequestFromTemplate(workspaceID, template, req)
	agent, err := s.createCustomAgent(ctx, createReq, actorID, template)
	if err != nil {
		return nil, err
	}

	var flow *model.AutomationRule
	if req.CreateFlow {
		flow, err = s.createStarterFlowForTemplate(ctx, workspaceID, template, agent, req)
		if err != nil {
			if deleteErr := s.agentRepo.Delete(ctx, workspaceID, agent.ID); deleteErr != nil {
				slog.WarnContext(ctx, "failed to clean up template-created agent after starter flow error",
					"workspace_id", workspaceID,
					"agent_id", agent.ID,
					"template_id", template.ID,
					"error", deleteErr,
				)
			}
			return nil, err
		}
	}

	return &model.CreateAgentFromTemplateResponse{
		Agent: agent,
		Flow:  flow,
	}, nil
}

func materializeCreateAgentRequestFromTemplate(workspaceID string, template *model.AgentTemplate, req model.CreateAgentFromTemplateRequest) model.CreateAgentRequest {
	name := strings.TrimSpace(stringOrDefault(req.Name, ""))
	if name == "" && template != nil {
		name = strings.TrimSpace(template.Name)
	}
	role := ""
	runtimeKind := ""
	systemPrompt := (*string)(nil)
	planningNotes := (*string)(nil)
	monthlyTokenBudget := (*int)(nil)
	allowedTools := json.RawMessage(mustJSONStringSlice(nil))
	allowedCommands := json.RawMessage(mustJSONStringSlice(nil))
	allowedTargets := json.RawMessage(mustJSONStringSlice([]string{"task"}))
	approvalMode := "never"
	defaultInvocationMode := model.InvocationModeAutonomous
	skills := model.AgentSkillRefs{}
	executionConfig := normalizeExecutionConfigJSON(nil)

	if template != nil {
		role = strings.TrimSpace(template.DefaultRole)
		runtimeKind = strings.TrimSpace(template.RuntimeKind)
		systemPrompt = trimPtr(template.SystemPrompt)
		planningNotes = trimPtr(template.PlanningNotes)
		monthlyTokenBudget = normalizeTokenBudget(template.MonthlyTokenBudget)
		allowedTools = json.RawMessage(normalizeAllowedToolsJSON(json.RawMessage(template.AllowedTools)))
		allowedCommands = json.RawMessage(normalizeJSONSlice(json.RawMessage(template.AllowedCommands)))
		allowedTargets = json.RawMessage(normalizeJSONSlice(json.RawMessage(template.AllowedTargets)))
		approvalMode = strings.TrimSpace(template.ApprovalMode)
		if approvalMode == "" {
			approvalMode = "never"
		}
		if strings.TrimSpace(template.DefaultInvocationMode) != "" {
			defaultInvocationMode = strings.TrimSpace(template.DefaultInvocationMode)
		}
		skills = template.Skills.Normalize()
		executionConfig = normalizeExecutionConfigJSON(template.ExecutionConfig)
	}
	if role == "" {
		role = "Custom Agent"
	}
	if runtimeKind == "" {
		runtimeKind = "opencode"
	}

	createReq := model.CreateAgentRequest{
		WorkspaceID:           workspaceID,
		Name:                  name,
		Role:                  role,
		RuntimeKind:           &runtimeKind,
		Skills:                skills,
		TriggerMode:           strPtr("manual"),
		ExecutionConfig:       json.RawMessage(executionConfig),
		SystemPrompt:          systemPrompt,
		PlanningNotes:         planningNotes,
		MonthlyTokenBudget:    monthlyTokenBudget,
		TeamID:                trimPtr(req.TeamID),
		AllowedTools:          allowedTools,
		AllowedCommands:       allowedCommands,
		AllowedTargets:        allowedTargets,
		ApprovalMode:          &approvalMode,
		DefaultInvocationMode: &defaultInvocationMode,
	}
	if req.Overrides != nil {
		if req.Overrides.Role != nil {
			roleValue := strings.TrimSpace(*req.Overrides.Role)
			if roleValue != "" {
				createReq.Role = roleValue
			}
		}
		if req.Overrides.RuntimeKind != nil {
			createReq.RuntimeKind = trimPtr(req.Overrides.RuntimeKind)
		}
		if req.Overrides.Skills != nil {
			normalized := req.Overrides.Skills.Normalize()
			createReq.Skills = normalized
		}
		if req.Overrides.Provider != nil {
			createReq.Provider = trimPtr(req.Overrides.Provider)
		}
		if req.Overrides.Model != nil {
			createReq.Model = trimPtr(req.Overrides.Model)
		}
		if req.Overrides.MonthlyTokenBudget != nil {
			createReq.MonthlyTokenBudget = normalizeTokenBudget(req.Overrides.MonthlyTokenBudget)
		}
		if req.Overrides.ExecutionConfig != nil {
			createReq.ExecutionConfig = json.RawMessage(normalizeExecutionConfigJSON(req.Overrides.ExecutionConfig))
		}
		if req.Overrides.SystemPrompt != nil {
			createReq.SystemPrompt = trimPtr(req.Overrides.SystemPrompt)
		}
		if req.Overrides.PlanningNotes != nil {
			createReq.PlanningNotes = trimPtr(req.Overrides.PlanningNotes)
		}
		if req.Overrides.AllowedTools != nil {
			createReq.AllowedTools = json.RawMessage(normalizeAllowedToolsJSON(json.RawMessage(req.Overrides.AllowedTools)))
		}
		if req.Overrides.AllowedCommands != nil {
			createReq.AllowedCommands = json.RawMessage(normalizeJSONSlice(json.RawMessage(req.Overrides.AllowedCommands)))
		}
		if req.Overrides.AllowedTargets != nil {
			createReq.AllowedTargets = json.RawMessage(normalizeJSONSlice(json.RawMessage(req.Overrides.AllowedTargets)))
		}
		if req.Overrides.ApprovalMode != nil {
			createReq.ApprovalMode = trimPtr(req.Overrides.ApprovalMode)
		}
		if req.Overrides.MaxConcurrentRuns != nil {
			createReq.MaxConcurrentRuns = req.Overrides.MaxConcurrentRuns
		}
		if req.Overrides.DefaultInvocationMode != nil {
			createReq.DefaultInvocationMode = trimPtr(req.Overrides.DefaultInvocationMode)
		}
	}
	if template != nil && strings.TrimSpace(template.Key) == model.AgentTemplateTypeCompetitiveIntel && req.CreateFlow {
		if input, err := competitiveIntelInputFromTemplateFlow(req.Flow); err == nil {
			createReq.SystemPrompt = renderCompetitiveIntelSystemPrompt(createReq.SystemPrompt, input)
		}
	}
	return createReq
}

func (s *AgentService) createStarterFlowForTemplate(ctx context.Context, workspaceID string, template *model.AgentTemplate, agent *model.Agent, req model.CreateAgentFromTemplateRequest) (*model.AutomationRule, error) {
	if s == nil || s.ruleEngine == nil {
		return nil, fmt.Errorf("automation rule engine is not configured")
	}
	switch strings.TrimSpace(template.Key) {
	case model.AgentTemplateTypeReleaseNotes:
		return s.createReleaseNotesStarterFlow(ctx, workspaceID, agent, req.Flow)
	case model.AgentTemplateTypeCompetitiveIntel:
		return s.createCompetitiveIntelStarterFlow(ctx, workspaceID, agent, req.Flow)
	default:
		return nil, fmt.Errorf("starter flow is not supported for template %q", strings.TrimSpace(template.Key))
	}
}

func (s *AgentService) createReleaseNotesStarterFlow(ctx context.Context, workspaceID string, agent *model.Agent, flow *model.CreateAgentFromTemplateFlow) (*model.AutomationRule, error) {
	if flow == nil {
		return nil, fmt.Errorf("flow configuration is required when create_flow is true")
	}
	repoFullName := strings.TrimSpace(flow.RepoFullName)
	repositoryID := strings.TrimSpace(flow.RepositoryID)
	if repoFullName == "" && repositoryID != "" {
		if s.gitService == nil {
			return nil, fmt.Errorf("git service is not configured")
		}
		repo, err := s.gitService.GetRepositoryByID(ctx, workspaceID, repositoryID)
		if err != nil {
			return nil, err
		}
		if repo == nil {
			return nil, fmt.Errorf("repository not found")
		}
		repoFullName = strings.TrimSpace(repo.FullName)
	}
	if repoFullName == "" {
		return nil, fmt.Errorf("flow.repo_full_name or flow.repository_id is required")
	}
	if strings.TrimSpace(flow.SpaceID) == "" {
		return nil, fmt.Errorf("flow.space_id is required")
	}

	releaseKinds := normalizedReleaseKinds(flow.ReleaseKinds)
	triggerConfig, err := json.Marshal(model.TriggerConfigGitHubReleasePublished{
		RepoFullName:      repoFullName,
		TagPattern:        strings.TrimSpace(flow.TagPattern),
		ReleaseKinds:      releaseKinds,
		IncludePrerelease: flow.IncludePrerelease,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal release trigger config: %w", err)
	}

	targetType := ""
	if repositoryID != "" {
		targetType = "repository"
	}
	actionConfig, err := json.Marshal(model.ActionConfigRunAgent{
		TargetType: targetType,
		TargetID:   repositoryID,
		AgentID:    agent.ID,
		Output: &model.ActionConfigRunAgentOutput{
			Type:         "docs_document",
			SpaceID:      strings.TrimSpace(flow.SpaceID),
			CollectionID: trimPtr(flow.CollectionID),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal release action config: %w", err)
	}

	name := fmt.Sprintf("%s for %s releases", strings.TrimSpace(agent.Name), repoFullName)
	description := fmt.Sprintf("Runs %s when GitHub publishes matching releases for %s and writes release notes to Docs.", strings.TrimSpace(agent.Name), repoFullName)
	rule, err := s.ruleEngine.CreateRule(ctx, workspaceID, model.CreateAutomationRuleRequest{
		WorkspaceID:   workspaceID,
		Name:          name,
		Description:   &description,
		TriggerType:   model.TriggerGitHubReleasePub,
		TriggerConfig: triggerConfig,
		ActionType:    model.ActionStartAgentRun,
		ActionConfig:  actionConfig,
	})
	if err != nil {
		return nil, err
	}
	return rule, nil
}

type competitiveIntelStarterFlowInput struct {
	TargetCompany      string   `json:"target_company"`
	TargetDomain       string   `json:"target_domain,omitempty"`
	Competitors        []string `json:"competitors,omitempty"`
	SchedulePreset     string   `json:"schedule_preset,omitempty"`
	LookbackDays       int      `json:"lookback_days,omitempty"`
	DestinationTeamID  string   `json:"destination_team_id"`
	DestinationStateID string   `json:"destination_state_id,omitempty"`
}

func (s *AgentService) createCompetitiveIntelStarterFlow(ctx context.Context, workspaceID string, agent *model.Agent, flow *model.CreateAgentFromTemplateFlow) (*model.AutomationRule, error) {
	input, err := competitiveIntelInputFromTemplateFlow(flow)
	if err != nil {
		return nil, err
	}

	triggerConfig, err := json.Marshal(model.TriggerConfigCron{
		Preset: input.SchedulePreset,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal competitive intel trigger config: %w", err)
	}

	actionConfig, err := json.Marshal(model.ActionConfigRunAgent{
		TargetType: "workspace",
		TargetID:   workspaceID,
		AgentID:    agent.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal competitive intel action config: %w", err)
	}

	name := fmt.Sprintf("%s %s digest for %s", strings.TrimSpace(agent.Name), input.SchedulePreset, input.TargetCompany)
	description := fmt.Sprintf("Runs %s on a %s schedule to research competitor updates for %s and create one marketing task.", strings.TrimSpace(agent.Name), input.SchedulePreset, input.TargetCompany)
	rule, err := s.ruleEngine.CreateRule(ctx, workspaceID, model.CreateAutomationRuleRequest{
		WorkspaceID:   workspaceID,
		Name:          name,
		Description:   &description,
		TriggerType:   model.TriggerCron,
		TriggerConfig: triggerConfig,
		ActionType:    model.ActionStartAgentRun,
		ActionConfig:  actionConfig,
	})
	if err != nil {
		return nil, err
	}
	return rule, nil
}

func competitiveIntelInputFromTemplateFlow(flow *model.CreateAgentFromTemplateFlow) (competitiveIntelStarterFlowInput, error) {
	if flow == nil {
		return competitiveIntelStarterFlowInput{}, fmt.Errorf("flow configuration is required when create_flow is true")
	}
	flowKey := strings.TrimSpace(flow.FlowKey)
	if flowKey != "" && flowKey != "competitive_intel_scheduled" {
		return competitiveIntelStarterFlowInput{}, fmt.Errorf("unsupported competitive intel flow_key %q", flowKey)
	}
	if len(flow.FlowInput) == 0 || strings.TrimSpace(string(flow.FlowInput)) == "" || strings.TrimSpace(string(flow.FlowInput)) == "null" {
		return competitiveIntelStarterFlowInput{}, fmt.Errorf("flow.flow_input is required for competitive intelligence starter flow")
	}

	var input competitiveIntelStarterFlowInput
	if err := json.Unmarshal(flow.FlowInput, &input); err != nil {
		return competitiveIntelStarterFlowInput{}, fmt.Errorf("parse competitive intel flow_input: %w", err)
	}
	input.TargetCompany = strings.TrimSpace(input.TargetCompany)
	input.TargetDomain = strings.TrimSpace(input.TargetDomain)
	input.SchedulePreset = strings.ToLower(strings.TrimSpace(input.SchedulePreset))
	input.DestinationTeamID = strings.TrimSpace(input.DestinationTeamID)
	input.DestinationStateID = strings.TrimSpace(input.DestinationStateID)
	input.Competitors = normalizeTemplateStringList(input.Competitors)
	if input.TargetCompany == "" {
		return competitiveIntelStarterFlowInput{}, fmt.Errorf("flow.flow_input.target_company is required")
	}
	if input.DestinationTeamID == "" {
		return competitiveIntelStarterFlowInput{}, fmt.Errorf("flow.flow_input.destination_team_id is required")
	}
	if input.SchedulePreset != "daily" && input.SchedulePreset != "weekly" {
		return competitiveIntelStarterFlowInput{}, fmt.Errorf("flow.flow_input.schedule_preset must be daily or weekly")
	}
	if input.LookbackDays < 1 || input.LookbackDays > 30 {
		return competitiveIntelStarterFlowInput{}, fmt.Errorf("flow.flow_input.lookback_days must be between 1 and 30")
	}
	return input, nil
}

func renderCompetitiveIntelSystemPrompt(base *string, input competitiveIntelStarterFlowInput) *string {
	prompt := strings.TrimSpace(derefString(base))
	if prompt == "" {
		fallback, err := competitiveIntelSystemPromptSection(input)
		if err != nil || strings.TrimSpace(fallback) == "" {
			return base
		}
		return &fallback
	}
	rendered, err := renderCompetitiveIntelPromptVariables(prompt, input)
	if err != nil || strings.TrimSpace(rendered) == "" {
		return base
	}
	return &rendered
}

func renderCompetitiveIntelPromptVariables(prompt string, input competitiveIntelStarterFlowInput) (string, error) {
	payload, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal competitive intel run configuration: %w", err)
	}
	values := competitiveIntelPromptValues(input, string(payload))
	replacerArgs := make([]string, 0, len(values)*2)
	for key, value := range values {
		replacerArgs = append(replacerArgs, "{{"+key+"}}", value)
	}
	return strings.TrimSpace(strings.NewReplacer(replacerArgs...).Replace(prompt)), nil
}

func competitiveIntelPromptValues(input competitiveIntelStarterFlowInput, rawJSON string) map[string]string {
	competitors := "none configured; discover competitors during this run"
	if len(input.Competitors) > 0 {
		competitors = strings.Join(input.Competitors, ", ")
	}
	destinationState := "not configured; use the team's default stage"
	if input.DestinationStateID != "" {
		destinationState = input.DestinationStateID
	}
	targetDomain := "not configured"
	if input.TargetDomain != "" {
		targetDomain = input.TargetDomain
	}
	return map[string]string{
		"target_company":         input.TargetCompany,
		"target_domain":          targetDomain,
		"competitors":            competitors,
		"lookback_days":          fmt.Sprintf("%d", input.LookbackDays),
		"destination_team_id":    input.DestinationTeamID,
		"destination_state_id":   destinationState,
		"schedule_preset":        input.SchedulePreset,
		"raw_configuration_json": "```json\n" + rawJSON + "\n```",
	}
}

func competitiveIntelSystemPromptSection(input competitiveIntelStarterFlowInput) (string, error) {
	payload, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal competitive intel run configuration: %w", err)
	}
	competitors := "none configured; discover competitors during this run"
	if len(input.Competitors) > 0 {
		competitors = strings.Join(input.Competitors, ", ")
	}
	destinationState := "not configured; use the team's default stage"
	if input.DestinationStateID != "" {
		destinationState = input.DestinationStateID
	}
	targetDomain := "not configured"
	if input.TargetDomain != "" {
		targetDomain = input.TargetDomain
	}

	context := fmt.Sprintf(`Competitive intelligence configuration:

These values were configured when this custom agent was created. Treat them as already resolved and authoritative. Do not plan or perform discovery of configuration variables, workspace context, teams, stages, cadence, or lookback settings.

- target_company: %s
- target_domain: %s
- competitors: %s
- lookback_days: %d
- destination_team_id: %s
- destination_state_id: %s
- schedule_preset: %s (informational; the automation rule already handled cadence)

Use the destination IDs directly when calling create_task. Only discover competitors if the configured competitors list is empty.

Raw configuration:

`, input.TargetCompany, targetDomain, competitors, input.LookbackDays, input.DestinationTeamID, destinationState, input.SchedulePreset)
	return strings.TrimSpace(context + "```json\n" + string(payload) + "\n```"), nil
}

func normalizedReleaseKinds(values []string) []string {
	if len(values) == 0 {
		return []string{"minor"}
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return []string{"minor"}
	}
	return out
}

func normalizeTemplateStringList(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		key := strings.ToLower(normalized)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, normalized)
	}
	return out
}
