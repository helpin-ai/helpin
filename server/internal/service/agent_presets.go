package service

import (
	"encoding/json"
	"slices"
	"strings"

	worker "github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
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
		model.AgentPresetDocumentationAgent,
		model.AgentPresetMarketer,
		model.AgentPresetCodeBuilder,
		model.AgentPresetReviewAgent,
		model.AgentPresetCommandAgent,
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
	case "docs", "documentation", model.AgentPresetDocumentationAgent:
		return model.AgentPresetDocumentationAgent
	case "marketing", "mira", model.AgentPresetMarketer:
		return model.AgentPresetMarketer
	case "engineer", "coder", model.AgentPresetCodeBuilder:
		return model.AgentPresetCodeBuilder
	case "reviewer", model.AgentPresetReviewAgent:
		return model.AgentPresetReviewAgent
	case "command", "command_agent", "one_shot", "one_shot_agent", "one_shot_command", "one_shot_command_agent", "research", "doc_researcher", "general_researcher", model.AgentPresetResearcher:
		return model.AgentPresetCommandAgent
	default:
		return strings.TrimSpace(key)
	}
}

func normalizePresetVersionKey(versionKey string) string {
	versionKey = strings.TrimSpace(versionKey)
	if versionKey == "researcher_default" {
		return "command_agent_default"
	}
	return versionKey
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
	case model.AgentPresetDocumentationAgent:
		return "documentation_agent_default"
	case model.AgentPresetMarketer:
		return "marketer_default"
	case model.AgentPresetCodeBuilder:
		return "code_builder_local_commit_delivery"
	case model.AgentPresetReviewAgent:
		return "review_agent_interactive_loop"
	case model.AgentPresetCommandAgent:
		return "command_agent_default"
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
		presets[idx].InstructionSkills = append([]string(nil), bundle.CoreSkillKeys...)
		presets[idx].AvailableSkills = append([]string(nil), bundle.AvailableSkillKeys...)
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
	definition.CreatedAt = &version.CreatedAt
	definition.UpdatedAt = &version.UpdatedAt
	if version.Provider != nil {
		definition.Provider = trimPtr(version.Provider)
	}
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
	} else if version.SystemPrompt != nil && version.InstructionPreamble != nil {
		definition.InstructionTemplateVersion = ""
	}
	if version.InstructionPreamble != nil {
		definition.InstructionPreamble = strings.TrimSpace(*version.InstructionPreamble)
	}
	if len(version.InstructionSkills) > 0 && string(version.InstructionSkills) != "null" {
		if skills := parseJSONStringSlice(version.InstructionSkills); skills != nil {
			definition.InstructionSkills = skills
		} else {
			definition.InstructionSkills = []string{}
		}
	}
	if len(version.AvailableSkills) > 0 && string(version.AvailableSkills) != "null" {
		if skills := parseJSONStringSlice(json.RawMessage(version.AvailableSkills)); skills != nil {
			definition.AvailableSkills = skills
		} else {
			definition.AvailableSkills = []string{}
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
	if len(version.AllowedTargets) > 0 {
		if targets := parseJSONStringSlice(version.AllowedTargets); targets != nil {
			definition.AllowedTargetTypes = targets
		} else {
			definition.AllowedTargetTypes = []string{}
		}
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
	case model.AgentPresetEpicPlanner, model.AgentPresetTaskPlanner, model.AgentPresetCRMOperator, model.AgentPresetSupportAgent, model.AgentPresetDocumentationAgent, model.AgentPresetMarketer:
		return []string{"codex", "native_sdk"}
	default:
		if preset, ok := agentPresetDefinition(presetKey); ok && strings.TrimSpace(preset.RuntimeKind) != "" {
			return []string{"codex", preset.RuntimeKind}
		}
		return []string{"codex"}
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
	documentationProfile := worker.GetRuntimeProfile(model.AgentPresetDocumentationAgent)
	supportAgentProvider := model.AgentModelProviderOpenAI
	supportAgentModel := "gpt-5.6-terra"
	codeBuilderProvider := model.AgentModelProviderOpenAI
	codeBuilderModel := "gpt-5.5"
	reviewAgentProvider := model.AgentModelProviderOpenAI
	reviewAgentModel := "gpt-5.5"
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
	documentationPrompt := defaultSystemPromptForPreset(model.AgentPresetDocumentationAgent)
	codeBuilderPrompt := defaultSystemPromptForPreset(model.AgentPresetCodeBuilder)
	reviewPrompt := defaultSystemPromptForPreset(model.AgentPresetReviewAgent)
	commandAgentPrompt := "You are Command Agent, a one-shot workspace operator for confirmed command-bar runs. Use only the tools enabled for the current run, stay within the confirmed step instruction, and operate on the provided target context. You may research, summarize, draft, create tasks or docs, update docs, or add task/CRM notes only when the enabled tools support that action. Do not create reusable agents unless the user explicitly promotes the run afterward."
	epicPlannerTools := filterPresetTools(productPlannerProfile.AllowedTools,
		worker.ToolUpdatePlan,
		worker.ToolPublishPRDDraft,
		worker.ToolPublishTaskPlan,
		worker.ToolRequestUserInput,
		worker.ToolRequestApproval,
	)
	taskPlannerTools := filterPresetTools(productPlannerProfile.AllowedTools,
		worker.ToolUpdatePlan,
		worker.ToolPublishTaskPlanDoc,
		worker.ToolRequestUserInput,
		worker.ToolRequestApproval,
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
			RuntimeKind:           "codex",
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual"},
			AllowedTools:          epicPlannerTools,
			AllowedCommands:       slices.Clone(productPlannerProfile.AllowedCommands),
			AllowedTargetTypes:    slices.Clone(productPlannerProfile.AllowedTargetTypes),
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeInteractive,
			SupportedModes:        supportedModesForRuntime("codex"),
			SystemPrompt:          epicPlannerPrompt,
		},
		{
			Key:                   model.AgentPresetTaskPlanner,
			FamilyKey:             model.AgentPresetTaskPlanner,
			VersionKey:            defaultPresetVersionKeyForPresetKey(model.AgentPresetTaskPlanner),
			VersionLabel:          "Default",
			IsDefaultVersion:      true,
			Label:                 "Coding Task Planner",
			Description:           "Interactive decomposition and task refinement across existing specs and code context.",
			DefaultRole:           "Coding Task Planner",
			RuntimeKind:           "codex",
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual"},
			AllowedTools:          taskPlannerTools,
			AllowedCommands:       slices.Clone(productPlannerProfile.AllowedCommands),
			AllowedTargetTypes:    []string{"task", "epic", "workspace"},
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeInteractive,
			SupportedModes:        supportedModesForRuntime("codex"),
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
			RuntimeKind:           "codex",
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual"},
			AllowedTools:          []string{worker.ToolListAvailableSkills, worker.ToolSearchAvailableSkills, worker.ToolReadSkill, "list_deals", "update_deal_stage", "add_deal_note", "list_contacts", "list_buyer_signals", "list_documents", "list_collections", "read_document", "get_document_blocks", "search_documents"},
			AllowedCommands:       []string{},
			AllowedTargetTypes:    []string{"crm_deal", "crm_contact", "support_conversation", "document", "workspace"},
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeInteractive,
			SupportedModes:        supportedModesForRuntime("codex"),
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
			RuntimeKind:           "codex",
			Provider:              &supportAgentProvider,
			Model:                 &supportAgentModel,
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual"},
			AllowedTools:          slices.Clone(supportProfile.AllowedTools),
			AllowedCommands:       slices.Clone(supportProfile.AllowedCommands),
			AllowedTargetTypes:    slices.Clone(supportProfile.AllowedTargetTypes),
			ApprovalMode:          "always",
			DefaultInvocationMode: model.InvocationModeAutonomous,
			SupportedModes:        supportedModesForRuntime("codex"),
			SystemPrompt:          supportPrompt,
		},
		{
			Key:                   model.AgentPresetDocumentationAgent,
			FamilyKey:             model.AgentPresetDocumentationAgent,
			VersionKey:            defaultPresetVersionKeyForPresetKey(model.AgentPresetDocumentationAgent),
			VersionLabel:          "Default",
			IsDefaultVersion:      true,
			Label:                 "Documentation Agent",
			Description:           "Keeps internal docs, public help docs, and API docs accurate, organized, and current.",
			DefaultRole:           "Documentation Agent",
			RuntimeKind:           "codex",
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual"},
			AllowedTools:          slices.Clone(documentationProfile.AllowedTools),
			AllowedCommands:       slices.Clone(documentationProfile.AllowedCommands),
			AllowedTargetTypes:    slices.Clone(documentationProfile.AllowedTargetTypes),
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeInteractive,
			SupportedModes:        supportedModesForRuntime("codex"),
			SystemPrompt:          documentationPrompt,
		},
		{
			Key:                 model.AgentPresetMarketer,
			FamilyKey:           model.AgentPresetMarketer,
			VersionKey:          defaultPresetVersionKeyForPresetKey(model.AgentPresetMarketer),
			VersionLabel:        "Default",
			IsDefaultVersion:    true,
			Label:               "Mira",
			Description:         "Marketing agent for growth plans, campaigns, copy, lifecycle messaging, content strategy, launches, and customer-signal synthesis.",
			DefaultRole:         "Marketer",
			RuntimeKind:         "codex",
			DefaultTriggerMode:  "manual",
			AllowedTriggerModes: []string{"manual"},
			AllowedTools: []string{
				worker.ToolListAvailableSkills,
				worker.ToolSearchAvailableSkills,
				worker.ToolReadSkill,
				"update_plan",
				"request_user_input",
				"request_approval",
				"request_review_checkpoint",
				"list_repositories",
				"checkout_repository",
				"checkout_repositories",
				"list_commits",
				"read_file",
				"read_file_range",
				"read_files",
				"list_directory",
				"search_files",
				"ripgrep",
				"grep",
				"list_symbols",
				"list_documents",
				"list_collections",
				"read_document",
				"get_document_blocks",
				"search_documents",
				"create_document",
				"write_document_content",
				"publish_document_change_proposal",
				"link_document_to_object",
				"list_tasks",
				"create_task",
				"add_task_comment",
				"get_task_context",
				"list_workspace_teams",
				"list_team_workflows_with_stages",
				"list_deals",
				"list_contacts",
				"list_buyer_signals",
				"add_deal_note",
				"web_search_exa",
				"fetch_url",
				"crawl_url",
				"get_release_context",
				"find_tasks_for_git_changes",
			},
			AllowedCommands:       []string{},
			AllowedTargetTypes:    []string{"workspace", "document", "task", "crm_deal", "crm_contact"},
			ApprovalMode:          "always",
			DefaultInvocationMode: model.InvocationModeInteractive,
			SupportedModes:        supportedModesForRuntime("codex"),
			SystemPrompt:          worker.BuiltInPresetPrompt(model.AgentPresetMarketer),
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
			Label:                 "QA & Code Reviewer",
			Description:           "Review-first agent for validation, follow-up discussion, and agreed fixes in the same branch.",
			DefaultRole:           "QA & Code Reviewer",
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
		{
			Key:                   model.AgentPresetCommandAgent,
			FamilyKey:             model.AgentPresetCommandAgent,
			VersionKey:            defaultPresetVersionKeyForPresetKey(model.AgentPresetCommandAgent),
			VersionLabel:          "Default",
			IsDefaultVersion:      true,
			Label:                 "Command Agent",
			Description:           "One-shot workspace operator for command-bar intents that do not fit narrower saved agents.",
			DefaultRole:           "Command Agent",
			RuntimeKind:           "codex",
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual"},
			AllowedTools:          []string{"web_search_brave", "web_search_exa", "fetch_url", "crawl_url", "request_user_input", "request_approval", "update_plan", "list_repositories", "checkout_repository", "checkout_repositories", "list_commits", "read_file", "read_file_range", "read_files", "list_directory", "search_files", "ripgrep", "grep", "list_symbols", "list_spaces", "list_documents", "list_collections", "read_document", "get_document_blocks", "publish_document_change_proposal", "publish_ai_section_candidate", "search_documents", "create_document", "write_document_content", "update_document_block", "link_document_to_object", "list_workspace_teams", "list_team_workflows_with_stages", "list_tasks", "create_task", "add_task_comment", "get_task_context", "list_deals", "list_contacts", "list_buyer_signals", "add_deal_note", "update_deal_stage", "ensure_crm_contact_company", "enrich_crm_contact", "enrich_crm_company"},
			AllowedCommands:       []string{},
			AllowedTargetTypes:    []string{"workspace", "document", "task", "epic", "crm_deal", "crm_contact", "repository"},
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeInteractive,
			SupportedModes:        supportedModesForRuntime("codex"),
			SystemPrompt:          &commandAgentPrompt,
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
			worker.ToolPublishTaskPlanDoc:
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
