package service

import (
	"slices"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/worker"
)

func ListAgentPresets() []model.AgentPresetDefinition {
	presets := agentPresetDefinitions()
	out := make([]model.AgentPresetDefinition, len(presets))
	copy(out, presets)
	for idx := range out {
		if strings.TrimSpace(out[idx].Scope) == "" {
			out[idx].Scope = "product"
		}
	}
	return out
}

func builtInPresetKeys() []string {
	return []string{
		model.AgentPresetEpicPlanner,
		model.AgentPresetStoryPlanner,
		model.AgentPresetCRMOperator,
		model.AgentPresetSupportAgent,
		model.AgentPresetCodeBuilder,
		model.AgentPresetReviewAgent,
	}
}

func agentPresetDefinition(key string) (model.AgentPresetDefinition, bool) {
	return agentPresetVersionDefinition(key, "")
}

func agentPresetVersionDefinition(key, versionKey string) (model.AgentPresetDefinition, bool) {
	familyKey := normalizePresetKey(key)
	if familyKey == "" {
		return model.AgentPresetDefinition{}, false
	}
	effectiveVersionKey := normalizePresetVersionKey(versionKey)
	if effectiveVersionKey == "" {
		effectiveVersionKey = defaultPresetVersionKeyForPresetKey(familyKey)
	}
	for _, preset := range agentPresetDefinitions() {
		if preset.Key == familyKey && preset.VersionKey == effectiveVersionKey {
			return preset, true
		}
	}
	return model.AgentPresetDefinition{}, false
}

func normalizePresetKey(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "", "none":
		return ""
	case "planner", "product_planner", model.AgentPresetEpicPlanner:
		return model.AgentPresetEpicPlanner
	case model.AgentPresetStoryPlanner:
		return model.AgentPresetStoryPlanner
	case "crm", "crm_agent", model.AgentPresetCRMOperator:
		return model.AgentPresetCRMOperator
	case "support", model.AgentPresetSupportAgent:
		return model.AgentPresetSupportAgent
	case "engineer", "coder", model.AgentPresetCodeBuilder:
		return model.AgentPresetCodeBuilder
	case "reviewer", model.AgentPresetReviewAgent:
		return model.AgentPresetReviewAgent
	default:
		return strings.TrimSpace(key)
	}
}

func normalizePresetVersionKey(versionKey string) string {
	return strings.TrimSpace(versionKey)
}

func defaultPresetKeyForAgent(isSystem bool) string {
	if isSystem {
		return model.AgentPresetEpicPlanner
	}
	return model.AgentPresetCodeBuilder
}

func defaultPresetVersionKeyForPresetKey(presetKey string) string {
	switch normalizePresetKey(presetKey) {
	case model.AgentPresetEpicPlanner:
		return "epic_planner_default"
	case model.AgentPresetStoryPlanner:
		return "story_planner_default"
	case model.AgentPresetCRMOperator:
		return "crm_operator_default"
	case model.AgentPresetSupportAgent:
		return "support_agent_default"
	case model.AgentPresetCodeBuilder:
		return "code_builder_default"
	case model.AgentPresetReviewAgent:
		return "review_agent_default"
	default:
		return ""
	}
}

func presetDefinitionForAgent(agent *model.Agent) (model.AgentPresetDefinition, bool) {
	if agent == nil {
		return model.AgentPresetDefinition{}, false
	}
	if preset, ok := agentPresetVersionDefinition(agent.EffectivePresetKey(), agent.EffectivePresetVersionKey()); ok {
		return preset, true
	}
	familyKey := normalizePresetKey(agent.EffectivePresetKey())
	if familyKey == "" {
		familyKey = defaultPresetKeyForAgent(agent.IsSystem)
	}
	if preset, ok := agentPresetVersionDefinition(familyKey, defaultPresetVersionKeyForPresetKey(familyKey)); ok {
		return preset, true
	}
	defaultPresetKey := defaultPresetKeyForAgent(agent.IsSystem)
	return agentPresetVersionDefinition(defaultPresetKey, defaultPresetVersionKeyForPresetKey(defaultPresetKey))
}

func workspacePresetDefinition(base model.AgentPresetDefinition, version model.WorkspaceAgentPresetVersion) model.AgentPresetDefinition {
	definition := base
	definition.Key = normalizePresetKey(version.FamilyKey)
	definition.FamilyKey = normalizePresetKey(version.FamilyKey)
	definition.VersionKey = strings.TrimSpace(version.VersionKey)
	definition.VersionLabel = strings.TrimSpace(version.Label)
	definition.IsDefaultVersion = false
	definition.Scope = "workspace"
	definition.WorkspaceID = &version.WorkspaceID
	definition.SourceVersionKey = version.SourceVersionKey
	definition.Provider = trimPtr(version.Provider)
	definition.Model = trimPtr(version.Model)
	definition.SystemPrompt = trimPtr(version.SystemPrompt)
	if description := strings.TrimSpace(stringOrDefault(version.Description, "")); description != "" {
		definition.Description = description
	}
	if runtime := strings.TrimSpace(version.RuntimeKind); runtime != "" {
		definition.RuntimeKind = runtime
	}
	if len(version.AllowedTools) > 0 {
		definition.AllowedTools = parseJSONStringSlice(version.AllowedTools)
	}
	if len(version.SupportedModes) > 0 {
		definition.SupportedModes = parseJSONStringSlice(version.SupportedModes)
	}
	if approvalMode := strings.TrimSpace(version.ApprovalMode); approvalMode != "" {
		definition.ApprovalMode = approvalMode
	}
	if mode := strings.TrimSpace(version.DefaultInvocationMode); mode != "" {
		definition.DefaultInvocationMode = mode
		if len(definition.SupportedModes) == 0 {
			definition.SupportedModes = supportedModesForRuntime(definition.RuntimeKind)
		}
	}
	if len(definition.SupportedModes) == 0 {
		definition.SupportedModes = supportedModesForRuntime(definition.RuntimeKind)
	}
	return definition
}

func defaultRoleForPresetKey(presetKey string) string {
	if preset, ok := agentPresetDefinition(presetKey); ok {
		return preset.DefaultRole
	}
	return ""
}

func defaultRuntimeKindForPresetKey(presetKey string) string {
	if preset, ok := agentPresetDefinition(presetKey); ok {
		return preset.RuntimeKind
	}
	return ""
}

func defaultTriggerModeForPresetKey(presetKey string) string {
	if preset, ok := agentPresetDefinition(presetKey); ok {
		return preset.DefaultTriggerMode
	}
	return "manual"
}

func allowedTriggerModesForPresetKey(presetKey string) []string {
	if preset, ok := agentPresetDefinition(presetKey); ok {
		return slices.Clone(preset.AllowedTriggerModes)
	}
	return []string{"manual"}
}

func supportedModesForRuntime(runtimeKind string) []string {
	switch strings.TrimSpace(runtimeKind) {
	case "native_sdk", "codex":
		return []string{model.InvocationModeAutonomous, model.InvocationModeInteractive}
	default:
		return []string{model.InvocationModeAutonomous}
	}
}

func allowedRuntimeKindsForPresetKey(presetKey string) []string {
	switch normalizePresetKey(presetKey) {
	case model.AgentPresetCodeBuilder:
		// native_sdk remains available for compatibility with existing agents.
		return []string{"opencode", "codex", "native_sdk"}
	case model.AgentPresetReviewAgent:
		// native_sdk remains available for compatibility with existing agents.
		return []string{"opencode", "codex", "native_sdk"}
	case model.AgentPresetEpicPlanner, model.AgentPresetStoryPlanner, model.AgentPresetCRMOperator, model.AgentPresetSupportAgent:
		return []string{"native_sdk"}
	default:
		if preset, ok := agentPresetDefinition(presetKey); ok && strings.TrimSpace(preset.RuntimeKind) != "" {
			return []string{preset.RuntimeKind}
		}
		return []string{"opencode"}
	}
}

func runtimeAllowedForPreset(presetKey, runtimeKind string) bool {
	return slices.Contains(allowedRuntimeKindsForPresetKey(presetKey), strings.TrimSpace(runtimeKind))
}

func agentPresetDefinitions() []model.AgentPresetDefinition {
	productPlannerProfile := worker.GetRuntimeProfile(model.AgentPresetEpicPlanner)
	engineerProfile := worker.GetRuntimeProfile(model.AgentPresetCodeBuilder)
	reviewerProfile := worker.GetRuntimeProfile(model.AgentPresetReviewAgent)
	supportProfile := worker.GetRuntimeProfile(model.AgentPresetSupportAgent)

	epicPlannerPrompt := defaultSystemPromptForPreset(model.AgentPresetEpicPlanner)
	storyPlannerPrompt := defaultSystemPromptForPreset(model.AgentPresetStoryPlanner)
	crmOperatorPrompt := defaultSystemPromptForPreset(model.AgentPresetCRMOperator)
	supportPrompt := defaultSystemPromptForPreset(model.AgentPresetSupportAgent)
	codeBuilderPrompt := defaultSystemPromptForPreset(model.AgentPresetCodeBuilder)
	reviewPrompt := defaultSystemPromptForPreset(model.AgentPresetReviewAgent)
	epicPlannerTools := filterPresetTools(productPlannerProfile.AllowedTools,
		worker.ToolPublishPRDDraft,
		worker.ToolPublishStoryPlan,
		worker.ToolRequestHumanInput,
		worker.ToolRequestHumanApproval,
	)
	storyPlannerTools := filterPresetTools(productPlannerProfile.AllowedTools,
		worker.ToolPublishStoryPlanDoc,
		worker.ToolRequestHumanInput,
		worker.ToolRequestHumanApproval,
	)

	return []model.AgentPresetDefinition{
		{
			Key:                   model.AgentPresetEpicPlanner,
			FamilyKey:             model.AgentPresetEpicPlanner,
			VersionKey:            defaultPresetVersionKeyForPresetKey(model.AgentPresetEpicPlanner),
			VersionLabel:          "Default",
			IsDefaultVersion:      true,
			Label:                 "Epic Planner",
			Description:           "Interactive product planning for epics, PRDs, documents, and story creation.",
			DefaultRole:           "Epic Planner",
			RuntimeKind:           productPlannerProfile.RuntimeKind,
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual"},
			AllowedTools:          epicPlannerTools,
			AllowedCommands:       slices.Clone(productPlannerProfile.AllowedCommands),
			AllowedTargetTypes:    slices.Clone(productPlannerProfile.AllowedTargetTypes),
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeInteractive,
			SupportedModes:        supportedModesForRuntime(productPlannerProfile.RuntimeKind),
			SystemPrompt:          epicPlannerPrompt,
		},
		{
			Key:                   model.AgentPresetStoryPlanner,
			FamilyKey:             model.AgentPresetStoryPlanner,
			VersionKey:            defaultPresetVersionKeyForPresetKey(model.AgentPresetStoryPlanner),
			VersionLabel:          "Default",
			IsDefaultVersion:      true,
			Label:                 "Story Planner",
			Description:           "Interactive decomposition and story refinement across existing specs and code context.",
			DefaultRole:           "Story Planner",
			RuntimeKind:           productPlannerProfile.RuntimeKind,
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual"},
			AllowedTools:          storyPlannerTools,
			AllowedCommands:       slices.Clone(productPlannerProfile.AllowedCommands),
			AllowedTargetTypes:    []string{"story", "epic"},
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeInteractive,
			SupportedModes:        supportedModesForRuntime(productPlannerProfile.RuntimeKind),
			SystemPrompt:          storyPlannerPrompt,
		},
		{
			Key:                   model.AgentPresetCRMOperator,
			FamilyKey:             model.AgentPresetCRMOperator,
			VersionKey:            defaultPresetVersionKeyForPresetKey(model.AgentPresetCRMOperator),
			VersionLabel:          "Default",
			IsDefaultVersion:      true,
			Label:                 "CRM Operator",
			Description:           "Cross-app CRM execution with deal, contact, support, and doc context.",
			DefaultRole:           "CRM Operator",
			RuntimeKind:           productPlannerProfile.RuntimeKind,
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual"},
			AllowedTools:          []string{"list_deals", "update_deal_stage", "add_deal_note", "list_contacts", "list_buyer_signals", "list_documents", "read_document", "search_documents"},
			AllowedCommands:       []string{},
			AllowedTargetTypes:    []string{"crm_deal", "support_conversation", "document"},
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeInteractive,
			SupportedModes:        supportedModesForRuntime(productPlannerProfile.RuntimeKind),
			SystemPrompt:          crmOperatorPrompt,
		},
		{
			Key:                   model.AgentPresetSupportAgent,
			FamilyKey:             model.AgentPresetSupportAgent,
			VersionKey:            defaultPresetVersionKeyForPresetKey(model.AgentPresetSupportAgent),
			VersionLabel:          "Default",
			IsDefaultVersion:      true,
			Label:                 "Support Agent",
			Description:           "Support conversation triage and reply drafting with review by default.",
			DefaultRole:           "Support Agent",
			RuntimeKind:           supportProfile.RuntimeKind,
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual"},
			AllowedTools:          slices.Clone(supportProfile.AllowedTools),
			AllowedCommands:       slices.Clone(supportProfile.AllowedCommands),
			AllowedTargetTypes:    slices.Clone(supportProfile.AllowedTargetTypes),
			ApprovalMode:          "always",
			DefaultInvocationMode: model.InvocationModeAutonomous,
			SupportedModes:        supportedModesForRuntime(supportProfile.RuntimeKind),
			SystemPrompt:          supportPrompt,
		},
		{
			Key:                   model.AgentPresetCodeBuilder,
			FamilyKey:             model.AgentPresetCodeBuilder,
			VersionKey:            defaultPresetVersionKeyForPresetKey(model.AgentPresetCodeBuilder),
			VersionLabel:          "Default",
			IsDefaultVersion:      true,
			Label:                 "Code Builder",
			Description:           "Repository-writing implementation agent for story execution.",
			DefaultRole:           "Code Builder",
			RuntimeKind:           engineerProfile.RuntimeKind,
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual", "auto_on_assignment", "auto_on_event"},
			AllowedTools:          slices.Clone(engineerProfile.AllowedTools),
			AllowedCommands:       slices.Clone(engineerProfile.AllowedCommands),
			AllowedTargetTypes:    slices.Clone(engineerProfile.AllowedTargetTypes),
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeAutonomous,
			SupportedModes:        supportedModesForRuntime(engineerProfile.RuntimeKind),
			SystemPrompt:          codeBuilderPrompt,
		},
		{
			Key:                   model.AgentPresetReviewAgent,
			FamilyKey:             model.AgentPresetReviewAgent,
			VersionKey:            defaultPresetVersionKeyForPresetKey(model.AgentPresetReviewAgent),
			VersionLabel:          "Default",
			IsDefaultVersion:      true,
			Label:                 "Review Agent",
			Description:           "Validation and review agent for story quality checks without repo mutation.",
			DefaultRole:           "Review Agent",
			RuntimeKind:           reviewerProfile.RuntimeKind,
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual", "auto_on_assignment", "auto_on_event"},
			AllowedTools:          slices.Clone(reviewerProfile.AllowedTools),
			AllowedCommands:       slices.Clone(reviewerProfile.AllowedCommands),
			AllowedTargetTypes:    slices.Clone(reviewerProfile.AllowedTargetTypes),
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeAutonomous,
			SupportedModes:        supportedModesForRuntime(reviewerProfile.RuntimeKind),
			SystemPrompt:          reviewPrompt,
		},
	}
}

func filterPresetTools(base []string, required ...string) []string {
	requiredSet := make(map[string]bool, len(required))
	for _, toolName := range required {
		requiredSet[toolName] = true
	}
	filtered := make([]string, 0, len(base))
	for _, toolName := range base {
		switch toolName {
		case worker.ToolPreviewMarkdown, worker.ToolPreviewJSON, worker.ToolPublishPreview, worker.ToolPublishPRDDraft, worker.ToolPublishStoryPlan, worker.ToolPublishStoryPlanDoc:
			if !requiredSet[toolName] {
				continue
			}
		}
		filtered = append(filtered, toolName)
	}
	for _, toolName := range required {
		if !slices.Contains(filtered, toolName) {
			filtered = append(filtered, toolName)
		}
	}
	return filtered
}
