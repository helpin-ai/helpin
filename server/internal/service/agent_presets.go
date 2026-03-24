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
	return out
}

func agentPresetDefinition(key string) (model.AgentPresetDefinition, bool) {
	normalized := normalizePresetKey(key)
	for _, preset := range agentPresetDefinitions() {
		if preset.Key == normalized {
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

func defaultPresetKeyForAgent(isSystem bool) string {
	if isSystem {
		return model.AgentPresetEpicPlanner
	}
	return model.AgentPresetCodeBuilder
}

func presetDefinitionForAgent(agent *model.Agent) (model.AgentPresetDefinition, bool) {
	if agent == nil {
		return model.AgentPresetDefinition{}, false
	}
	if preset, ok := agentPresetDefinition(agent.PresetKey); ok {
		return preset, true
	}
	return agentPresetDefinition(defaultPresetKeyForAgent(agent.IsSystem))
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
	if strings.TrimSpace(runtimeKind) == "native_sdk" {
		return []string{model.InvocationModeAutonomous, model.InvocationModeInteractive}
	}
	return []string{model.InvocationModeAutonomous}
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

	return []model.AgentPresetDefinition{
		{
			Key:                   model.AgentPresetEpicPlanner,
			Label:                 "Epic Planner",
			Description:           "Interactive product planning for epics, PRDs, documents, and story creation.",
			DefaultRole:           "Epic Planner",
			RuntimeKind:           productPlannerProfile.RuntimeKind,
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual"},
			AllowedTools:          slices.Clone(productPlannerProfile.AllowedTools),
			AllowedCommands:       slices.Clone(productPlannerProfile.AllowedCommands),
			AllowedTargetTypes:    slices.Clone(productPlannerProfile.AllowedTargetTypes),
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeInteractive,
			SupportedModes:        supportedModesForRuntime(productPlannerProfile.RuntimeKind),
			SystemPrompt:          epicPlannerPrompt,
		},
		{
			Key:                   model.AgentPresetStoryPlanner,
			Label:                 "Story Planner",
			Description:           "Interactive decomposition and story refinement across existing specs and code context.",
			DefaultRole:           "Story Planner",
			RuntimeKind:           productPlannerProfile.RuntimeKind,
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual"},
			AllowedTools:          slices.Clone(productPlannerProfile.AllowedTools),
			AllowedCommands:       slices.Clone(productPlannerProfile.AllowedCommands),
			AllowedTargetTypes:    []string{"story", "epic"},
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeInteractive,
			SupportedModes:        supportedModesForRuntime(productPlannerProfile.RuntimeKind),
			SystemPrompt:          storyPlannerPrompt,
		},
		{
			Key:                   model.AgentPresetCRMOperator,
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
