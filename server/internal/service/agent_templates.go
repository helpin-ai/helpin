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
	description := "Creates release notes from GitHub releases, related tasks, and linked docs."
	return []model.AgentTemplate{
		{
			Key:         model.AgentTemplateTypeReleaseNotes,
			Name:        "Release Notes Writer",
			Description: &description,
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
	return createReq
}

func (s *AgentService) createStarterFlowForTemplate(ctx context.Context, workspaceID string, template *model.AgentTemplate, agent *model.Agent, req model.CreateAgentFromTemplateRequest) (*model.AutomationRule, error) {
	if s == nil || s.ruleEngine == nil {
		return nil, fmt.Errorf("automation rule engine is not configured")
	}
	switch strings.TrimSpace(template.Key) {
	case model.AgentTemplateTypeReleaseNotes:
		return s.createReleaseNotesStarterFlow(ctx, workspaceID, agent, req.Flow)
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
