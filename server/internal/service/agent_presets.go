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
		model.AgentPresetTaskPlanner,
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
	case model.AgentPresetTaskPlanner:
		return model.AgentPresetTaskPlanner
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
	case model.AgentPresetTaskPlanner:
		return "task_planner_default"
	case model.AgentPresetCRMOperator:
		return "crm_operator_default"
	case model.AgentPresetSupportAgent:
		return "support_agent_default"
	case model.AgentPresetCodeBuilder:
		return "code_builder_local_commit_delivery"
	case model.AgentPresetReviewAgent:
		return "review_agent_interactive_loop"
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

func applyBuiltInPresetInstructionMetadata(presets []model.AgentPresetDefinition) []model.AgentPresetDefinition {
	for idx := range presets {
		bundle, ok := worker.BuiltInPresetSkillBundleForPreset(presets[idx].Key)
		if !ok {
			continue
		}
		presets[idx].InstructionPreamble = bundle.Preamble
		presets[idx].InstructionSkills = append([]string(nil), bundle.SkillKeys...)
		presets[idx].InstructionTemplateVersion = worker.BuiltInPresetInstructionTemplateVersion(presets[idx].Key)
		if presets[idx].SystemPrompt == nil {
			presets[idx].SystemPrompt = worker.BuiltInPresetPrompt(presets[idx].Key)
		}
	}
	return presets
}

func workspacePresetDefinition(base model.AgentPresetDefinition, version model.WorkspaceAgentPresetVersion) model.AgentPresetDefinition {
	definition := base
	definition.ID = &version.ID
	definition.Key = normalizePresetKey(version.FamilyKey)
	definition.FamilyKey = normalizePresetKey(version.FamilyKey)
	definition.VersionKey = strings.TrimSpace(version.VersionKey)
	definition.VersionLabel = strings.TrimSpace(version.Label)
	definition.IsDefaultVersion = false
	definition.Scope = "workspace"
	definition.WorkspaceID = &version.WorkspaceID
	definition.SourceVersionKey = version.SourceVersionKey
	definition.Provider = trimPtr(version.Provider)
	if version.Model != nil {
		modelValue := strings.TrimSpace(*version.Model)
		definition.Model = &modelValue
	}
	definition.ExecutionConfig = normalizeExecutionConfigJSON(version.ExecutionConfig)
	if prompt := trimPtr(version.SystemPrompt); prompt != nil {
		definition.SystemPrompt = prompt
	}
	if versionValue := strings.TrimSpace(version.InstructionTemplateVersion); versionValue != "" {
		definition.InstructionTemplateVersion = versionValue
	}
	if preamble := trimPtr(version.InstructionPreamble); preamble != nil {
		definition.InstructionPreamble = *preamble
	}
	if len(version.InstructionSkills) > 0 {
		if skills := parseJSONStringSlice(version.InstructionSkills); len(skills) > 0 {
			definition.InstructionSkills = skills
		}
	}
	if description := strings.TrimSpace(stringOrDefault(version.Description, "")); description != "" {
		definition.Description = description
	}
	if runtime := strings.TrimSpace(version.RuntimeKind); runtime != "" {
		definition.RuntimeKind = runtime
	}
	if len(version.AllowedTools) > 0 {
		definition.AllowedTools = worker.NormalizeToolNames(parseJSONStringSlice(version.AllowedTools))
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
	case model.AgentPresetEpicPlanner, model.AgentPresetTaskPlanner, model.AgentPresetCRMOperator, model.AgentPresetSupportAgent:
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
	codeBuilderProvider := model.AgentModelProviderOpenAI
	codeBuilderModel := "gpt-5.4"
	reviewAgentProvider := model.AgentModelProviderOpenAI
	reviewAgentModel := "gpt-5.4"
	highReasoning := "high"
	fastServiceTier := "fast"
	codexOpenAIDefaultExecutionConfig := model.MarshalAgentExecutionConfig(model.AgentExecutionConfig{
		ReasoningEffort: &highReasoning,
		ServiceTier:     &fastServiceTier,
	})

	epicPlannerPrompt := defaultSystemPromptForPreset(model.AgentPresetEpicPlanner)
	taskPlannerPrompt := defaultSystemPromptForPreset(model.AgentPresetTaskPlanner)
	crmOperatorPrompt := defaultSystemPromptForPreset(model.AgentPresetCRMOperator)
	supportPrompt := defaultSystemPromptForPreset(model.AgentPresetSupportAgent)
	codeBuilderPrompt := defaultSystemPromptForPreset(model.AgentPresetCodeBuilder)
	reviewPrompt := defaultSystemPromptForPreset(model.AgentPresetReviewAgent)
	epicPlannerTools := filterPresetTools(productPlannerProfile.AllowedTools,
		worker.ToolUpdatePlan,
		worker.ToolPublishPRDDraft,
		worker.ToolPublishTaskPlan,
		worker.ToolRequestUserInput,
		worker.ToolRequestReviewCheckpoint,
	)
	taskPlannerTools := filterPresetTools(productPlannerProfile.AllowedTools,
		worker.ToolUpdatePlan,
		worker.ToolPublishTaskPlanDoc,
		worker.ToolRequestUserInput,
		worker.ToolRequestReviewCheckpoint,
	)
	taskPlannerTools = slices.DeleteFunc(taskPlannerTools, func(toolName string) bool {
		return toolName == "list_epic_tasks"
	})

	presets := []model.AgentPresetDefinition{
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
			Key:                   model.AgentPresetTaskPlanner,
			FamilyKey:             model.AgentPresetTaskPlanner,
			VersionKey:            defaultPresetVersionKeyForPresetKey(model.AgentPresetTaskPlanner),
			VersionLabel:          "Default",
			IsDefaultVersion:      true,
			Label:                 "Task Planner",
			Description:           "Interactive decomposition and task refinement across existing specs and code context.",
			DefaultRole:           "Task Planner",
			RuntimeKind:           productPlannerProfile.RuntimeKind,
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual"},
			AllowedTools:          taskPlannerTools,
			AllowedCommands:       slices.Clone(productPlannerProfile.AllowedCommands),
			AllowedTargetTypes:    []string{"task", "epic"},
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeInteractive,
			SupportedModes:        supportedModesForRuntime(productPlannerProfile.RuntimeKind),
			SystemPrompt:          taskPlannerPrompt,
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
			Provider:              &codeBuilderProvider,
			Model:                 &codeBuilderModel,
			ExecutionConfig:       codexOpenAIDefaultExecutionConfig,
			Label:                 "Code Builder",
			Description:           "Repository-writing implementation agent for story execution.",
			DefaultRole:           "Code Builder",
			RuntimeKind:           "codex",
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual", "auto_on_assignment", "auto_on_event"},
			AllowedTools:          slices.Clone(engineerProfile.AllowedTools),
			AllowedCommands:       slices.Clone(engineerProfile.AllowedCommands),
			AllowedTargetTypes:    slices.Clone(engineerProfile.AllowedTargetTypes),
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeAutonomous,
			SupportedModes:        supportedModesForRuntime("codex"),
			SystemPrompt:          codeBuilderPrompt,
		},
		{
			Key:                   model.AgentPresetReviewAgent,
			FamilyKey:             model.AgentPresetReviewAgent,
			VersionKey:            defaultPresetVersionKeyForPresetKey(model.AgentPresetReviewAgent),
			VersionLabel:          "Default",
			IsDefaultVersion:      true,
			Provider:              &reviewAgentProvider,
			Model:                 &reviewAgentModel,
			ExecutionConfig:       codexOpenAIDefaultExecutionConfig,
			Label:                 "Review Agent",
			Description:           "Review-first agent for validation, follow-up discussion, and agreed fixes in the same branch.",
			DefaultRole:           "Review Agent",
			RuntimeKind:           "codex",
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual", "auto_on_assignment", "auto_on_event"},
			AllowedTools:          slices.Clone(reviewerProfile.AllowedTools),
			AllowedCommands:       slices.Clone(reviewerProfile.AllowedCommands),
			AllowedTargetTypes:    slices.Clone(reviewerProfile.AllowedTargetTypes),
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeInteractive,
			SupportedModes:        supportedModesForRuntime("codex"),
			SystemPrompt:          reviewPrompt,
		},
	}

	return applyBuiltInPresetInstructionMetadata(presets)
}

func filterPresetTools(base []string, required ...string) []string {
	requiredSet := make(map[string]bool, len(required))
	for _, toolName := range required {
		requiredSet[toolName] = true
	}
	filtered := make([]string, 0, len(base))
	for _, toolName := range base {
		switch toolName {
		case worker.ToolPreviewMarkdown,
			worker.ToolPreviewJSON,
			worker.ToolPublishPreview,
			worker.ToolPublishPRDDraft,
			worker.ToolPublishTaskPlan,
			worker.ToolPublishTaskPlanDoc,
			worker.ToolPublishStoryPlan,
			worker.ToolPublishStoryPlanDoc:
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
