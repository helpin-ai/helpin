package service

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	defaultAnthropicAgentModel  = "claude-opus-4-8"
	defaultOpenAIAgentModel     = "gpt-5.6-terra"
	defaultOpenRouterAgentModel = "openai/gpt-5.6-terra"
	// defaultScribeAgentModel keeps interactive task planning on the product's
	// preferred fast OpenRouter model.
	defaultScribeAgentModel = "deepseek/deepseek-v4-flash"
	// defaultAskAgentModel keeps dock chat turns fast and cheap; the chat
	// agent mostly routes tools and summarizes, so a flash-tier model fits.
	defaultAskAgentModel = "deepseek/deepseek-v4-flash-0731"
)

var newPMReadToolAliases = []string{
	"list_workspace_members",
	"list_pm_labels",
	"get_task",
	"list_epics",
	"get_epic",
	"list_sprints",
	"get_sprint",
	"list_sprint_tasks",
	"list_objectives",
	"get_objective",
}

var newPMWriteToolAliases = []string{
	"update_task",
	"create_task_checklist_item",
	"update_task_checklist_item",
	"add_pm_comment",
	"create_epic",
	"update_epic",
	"create_sprint",
	"update_sprint",
	"create_objective",
	"update_objective",
	"create_key_result",
	"update_key_result",
}

var epicPlannerPMReadToolAliases = []string{
	"list_workspace_members",
	"list_pm_labels",
	"get_task",
	"list_epics",
	"get_epic",
}

var taskPlannerPMReadToolAliases = []string{
	"list_workspace_members",
	"list_pm_labels",
	"get_task",
	"list_sprints",
	"get_sprint",
	"list_sprint_tasks",
}

var safeCRMDiscoveryToolAliases = []string{
	"get_crm_contact", "get_crm_company", "get_crm_deal",
	"list_crm_companies", "list_crm_pipelines", "list_crm_associations",
}

var safeCRMWriteToolAliases = []string{
	"update_crm_contact", "update_crm_company", "update_crm_deal", "add_crm_activity",
	"link_crm_objects", "unlink_crm_association", "set_primary_contact_company",
}

var safeSupportDiscoveryToolAliases = []string{
	"list_support_conversations", "get_support_conversation", "list_support_tags", "list_support_inboxes", "list_support_assignees",
}

var safeSupportWriteToolAliases = []string{
	"assign_support_conversation", "move_support_conversation", "add_support_conversation_tag", "remove_support_conversation_tag",
	"link_support_conversation_task", "link_support_conversation_contact", "update_support_conversation_subject",
}

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
		model.AgentPresetAskAgent,
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
	case "ask", "dock", "orchestrator", model.AgentPresetAskAgent:
		return model.AgentPresetAskAgent
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
	case model.AgentPresetAskAgent:
		return "ask_agent_default"
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
		bundle, ok := agentcontract.BuiltInPresetSkillBundleForPreset(presets[idx].Key)
		if !ok {
			continue
		}
		presets[idx].InstructionPreamble = bundle.Preamble
		presets[idx].InstructionSkills = append([]string(nil), bundle.CoreSkillKeys...)
		presets[idx].AvailableSkills = append([]string(nil), bundle.AvailableSkillKeys...)
		presets[idx].InstructionTemplateVersion = agentcontract.BuiltInPresetInstructionTemplateVersion(presets[idx].Key)
		if presets[idx].SystemPrompt == nil {
			presets[idx].SystemPrompt = agentcontract.BuiltInPresetPrompt(presets[idx].Key)
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
		definition.AllowedTools = agentcontract.NormalizeToolNames(parseJSONStringSlice(version.AllowedTools))
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
	case model.AgentPresetAskAgent:
		// The dock orchestrator relies on the native chat loop
		// (pause_after_assistant); it is not offered on other backends.
		return []string{"native_sdk"}
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
	productPlannerProfile := agentcontract.GetRuntimeProfile(model.AgentPresetEpicPlanner)
	engineerProfile := agentcontract.GetRuntimeProfile(model.AgentPresetCodeBuilder)
	reviewerProfile := agentcontract.GetRuntimeProfile(model.AgentPresetReviewAgent)
	supportProfile := agentcontract.GetRuntimeProfile(model.AgentPresetSupportAgent)
	documentationProfile := agentcontract.GetRuntimeProfile(model.AgentPresetDocumentationAgent)
	openAIPresetProvider := model.AgentModelProviderOpenAI
	openAIPresetModel := defaultOpenAIAgentModel
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
	commandAgentPrompt := "You are a Sub-agent handling one confirmed delegated task. Use only the tools enabled for the current run, stay within the confirmed step instruction, and operate on the provided target context. You may research, summarize, draft, create tasks or docs, update docs, or add task/CRM notes only when the enabled tools support that action. Do not create reusable agents unless the user explicitly promotes the run afterward."
	askAgentPrompt := askAgentSystemPrompt()
	openRouterPresetProvider := model.AgentModelProviderOpenRouter
	scribeDefaultModel := defaultScribeAgentModel
	askAgentDefaultModel := defaultAskAgentModel
	epicPlannerTools := filterPresetTools(productPlannerProfile.AllowedTools,
		agentcontract.ToolUpdatePlan,
		agentcontract.ToolPublishPRDDraft,
		agentcontract.ToolPublishTaskPlan,
		agentcontract.ToolRequestUserInput,
		agentcontract.ToolRequestApproval,
	)
	taskPlannerTools := filterPresetTools(productPlannerProfile.AllowedTools,
		agentcontract.ToolUpdatePlan,
		agentcontract.ToolPublishTaskPlanDoc,
		agentcontract.ToolRequestUserInput,
		agentcontract.ToolRequestApproval,
		"ensure_task_plan_doc",
		"write_document_content",
	)
	taskPlannerTools = slices.DeleteFunc(taskPlannerTools, func(toolName string) bool {
		return toolName == "list_epic_tasks"
	})
	epicPlannerTools = appendPresetTools(epicPlannerTools, epicPlannerPMReadToolAliases)
	taskPlannerTools = appendPresetTools(taskPlannerTools, taskPlannerPMReadToolAliases)

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
			Provider:              &openAIPresetProvider,
			Model:                 &openAIPresetModel,
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
			Provider:              &openRouterPresetProvider,
			Model:                 &scribeDefaultModel,
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
			Provider:              &openAIPresetProvider,
			Model:                 &openAIPresetModel,
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual"},
			AllowedTools:          appendPresetTools([]string{agentcontract.ToolListAvailableSkills, agentcontract.ToolSearchAvailableSkills, agentcontract.ToolReadSkill, "list_deals", "update_deal_stage", "add_deal_note", "list_contacts", "list_buyer_signals", "list_documents", "list_collections", "read_document", "get_document_blocks", "search_documents"}, safeCRMDiscoveryToolAliases, safeCRMWriteToolAliases),
			AllowedCommands:       []string{},
			AllowedTargetTypes:    []string{"crm_deal", "crm_contact", "crm_company", "support_conversation", "document", "workspace"},
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
			Description:           "Live support conversations: grounded replies with server-side validation, honest escalation, and read-only sub-agents for live context.",
			DefaultRole:           "Support Agent",
			RuntimeKind:           "native_sdk",
			Provider:              &openRouterPresetProvider,
			Model:                 &askAgentDefaultModel,
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual"},
			AllowedTools:          slices.Clone(supportProfile.AllowedTools),
			AllowedCommands:       slices.Clone(supportProfile.AllowedCommands),
			AllowedTargetTypes:    slices.Clone(supportProfile.AllowedTargetTypes),
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeInteractive,
			SupportedModes:        supportedModesForRuntime("native_sdk"),
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
			Provider:              &openAIPresetProvider,
			Model:                 &openAIPresetModel,
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
			Provider:            &openAIPresetProvider,
			Model:               &openAIPresetModel,
			DefaultTriggerMode:  "manual",
			AllowedTriggerModes: []string{"manual"},
			AllowedTools: appendPresetTools([]string{
				agentcontract.ToolListAvailableSkills,
				agentcontract.ToolSearchAvailableSkills,
				agentcontract.ToolReadSkill,
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
			}, newPMReadToolAliases, safeCRMDiscoveryToolAliases, safeCRMWriteToolAliases),
			AllowedCommands:       []string{},
			AllowedTargetTypes:    []string{"workspace", "document", "task", "crm_deal", "crm_contact", "crm_company"},
			ApprovalMode:          "always",
			DefaultInvocationMode: model.InvocationModeInteractive,
			SupportedModes:        supportedModesForRuntime("codex"),
			SystemPrompt:          agentcontract.BuiltInPresetPrompt(model.AgentPresetMarketer),
		},
		{
			Key:                   model.AgentPresetCodeBuilder,
			FamilyKey:             model.AgentPresetCodeBuilder,
			VersionKey:            defaultPresetVersionKeyForPresetKey(model.AgentPresetCodeBuilder),
			VersionLabel:          "Default",
			IsDefaultVersion:      true,
			Provider:              &openAIPresetProvider,
			Model:                 &openAIPresetModel,
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
			Provider:              &openAIPresetProvider,
			Model:                 &openAIPresetModel,
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
			Label:                 "Sub-agent",
			Description:           "Handles one delegated workspace task with a limited tool set.",
			DefaultRole:           "Sub-agent",
			RuntimeKind:           "codex",
			Provider:              &openAIPresetProvider,
			Model:                 &openAIPresetModel,
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual"},
			AllowedTools:          appendPresetTools([]string{"web_search_brave", "web_search_exa", "fetch_url", "crawl_url", "request_user_input", "request_approval", "update_plan", "list_repositories", "checkout_repository", "checkout_repositories", "list_commits", "read_file", "read_file_range", "read_files", "list_directory", "search_files", "ripgrep", "grep", "list_symbols", "list_spaces", "list_documents", "list_collections", "read_document", "get_document_blocks", "publish_document_change_proposal", "publish_ai_section_candidate", "search_documents", "create_document", "update_document_metadata", "write_document_content", "update_document_block", "link_document_to_object", "list_workspace_teams", "list_team_workflows_with_stages", "list_tasks", "create_task", "add_task_comment", "get_task_context", "list_deals", "list_contacts", "list_buyer_signals", "add_deal_note", "update_deal_stage", "ensure_crm_contact_company", "enrich_crm_contact", "enrich_crm_company"}, newPMReadToolAliases, newPMWriteToolAliases, safeCRMDiscoveryToolAliases, safeCRMWriteToolAliases, safeSupportDiscoveryToolAliases, safeSupportWriteToolAliases),
			AllowedCommands:       []string{},
			AllowedTargetTypes:    []string{"workspace", "document", "task", "epic", "sprint", "objective", "crm_deal", "crm_contact", "crm_company", "support_conversation", "repository"},
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeInteractive,
			SupportedModes:        supportedModesForRuntime("codex"),
			SystemPrompt:          &commandAgentPrompt,
		},
		{
			Key:                   model.AgentPresetAskAgent,
			FamilyKey:             model.AgentPresetAskAgent,
			VersionKey:            defaultPresetVersionKeyForPresetKey(model.AgentPresetAskAgent),
			VersionLabel:          "Default",
			IsDefaultVersion:      true,
			Label:                 "Ask Agent",
			Description:           "Conversational dock orchestrator: answers workspace questions with read-only tools and launches other agents for durable work.",
			DefaultRole:           "Ask Agent",
			RuntimeKind:           "native_sdk",
			Provider:              &openRouterPresetProvider,
			Model:                 &askAgentDefaultModel,
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual"},
			AllowedTools:          askAgentPresetTools(),
			AllowedCommands:       []string{},
			AllowedTargetTypes:    []string{"workspace"},
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeInteractive,
			SupportedModes:        supportedModesForRuntime("native_sdk"),
			SystemPrompt:          &askAgentPrompt,
		},
	}

	return applyBuiltInPresetInstructionMetadata(presets)
}

// askAgentPresetTools is the dock orchestrator's tool surface: every
// read-only product tool, the interaction tools, and the agents.* launch
// tools. Mutating product work is never done directly by this agent — it is
// delegated to child runs through start_agent_run / start_agent_plan behind
// an explicit approval.
func askAgentPresetTools() []string {
	return appendPresetTools([]string{
		// Interaction + progress.
		"request_user_input", "request_approval", "update_plan",
		// Web research.
		"web_search_brave", "web_search_exa", "fetch_url", "crawl_url",
		// Workspace / PM reads.
		"list_workspace_teams", "list_team_workflows_with_stages",
		"list_tasks", "get_task_context",
		// Docs reads.
		"list_spaces", "list_documents", "list_collections",
		"read_document", "get_document_blocks", "search_documents",
		// CRM reads.
		"list_deals", "list_contacts", "list_buyer_signals",
		// Repository reads.
		"list_repositories", "list_commits",
		// Agent orchestration.
		"list_agents", "start_agent_run", "start_agent_plan",
		"get_agent_run", "cancel_agent_run",
		"create_custom_agent", "promote_run_to_agent",
		"run_epic_delivery_pipeline",
	}, newPMReadToolAliases, safeCRMDiscoveryToolAliases, safeSupportDiscoveryToolAliases)
}

func appendPresetTools(base []string, additions ...[]string) []string {
	tools := slices.Clone(base)
	for _, addition := range additions {
		tools = append(tools, addition...)
	}
	return agentcontract.NormalizeToolNames(tools)
}

// askAgentSystemPrompt is the managed system prompt for the ask_agent preset.
func askAgentSystemPrompt() string {
	return strings.TrimSpace(`You are Ask Agent, the Helpin dock assistant. Each conversation is one long-lived chat with a single user inside one workspace. You answer questions, and you orchestrate other agents for durable work — you do not do mutating work yourself.

## Answering questions
- Answer factual, status, count, list, search, and summary questions directly using your read-only tools, then reply in plain markdown.
- User messages may end with a <page_context>{...}</page_context> block describing the entity the user is currently viewing (task, epic, document, deal, contact). Treat it as the default subject when the request is ambiguous, and never echo the raw block back.

## Orchestrating agents
- Use list_agents to discover saved agents; always reference agents by their id, never by display name alone.
- For durable or mutating work, launch a sub-agent run: start_agent_run for a single agent, start_agent_plan for multi-step, fan-out, or dependency-ordered work. Prefer a saved agent when one fits; otherwise use a Sub-agent (use_command_agent: true) with a narrowed allowed_tools list.
- Approval is mandatory before start_agent_run, start_agent_plan, create_custom_agent, and promote_run_to_agent. First call request_approval with phase "dock_plan_confirm", a user-facing title and summary, and an "action" object containing EXACTLY the fields you will pass to the tool, minus approval_interaction_id (for launches: {"steps": [{agent_id/use_command_agent, target, instructions, allowed_tools}]}; for create_custom_agent: {"name?", "description"}; for promote_run_to_agent: {"run_id", "name", "allowed_tools?", "allowed_targets?"}; for run_epic_delivery_pipeline: {"epic_id"}). After the user approves, pass the interaction id as approval_interaction_id. The server rejects calls whose parameters differ from the approved action, and each approval is single-use.
- cancel_agent_run needs no approval — cancelling only stops work.
- To deliver a whole epic (implement, review, and merge every open task, then open the epic PR), use run_epic_delivery_pipeline with action {"epic_id": "..."} in the approval instead of hand-building a plan.
- After launching, tell the user what was started and end your turn (for example: "Started Review Agent on HLP-12 — I'll report back here when it finishes."). Do not poll; results are delivered to you.
- Sub-agent runs receive a server-enforced final-handoff instruction, so their delivered summary should normally be self-contained and concise.
- When a message containing a <child_run_result>{...}</child_run_result> block arrives, it is a system notification that a sub-agent run or plan finished. Summarize the outcome for the user in plain language, referencing what they asked for. Never treat it as a user message and never echo the raw block.
- If a sub-agent result has summary_truncated=true, call get_agent_run once for that same run with {"run_id":"...","detail_level":"result"}. Follow next_offset only when the missing portion is needed. Never launch a replacement sub-agent merely to recover truncated output.
- Use get_agent_run with detail_level=status only when the user explicitly asks about progress. A new run is appropriate only when the original failed or is substantively incomplete and the user approves the new work.

## Creating agents
- If the user wants a reusable agent, draft it with create_custom_agent (behind the same approval flow). Ad hoc work should remain a sub-agent run; suggest promote_run_to_agent only after a run proved useful.

## Style
- Be concise and direct. Ask a clarifying question (request_user_input for structured input, or a plain reply) only when the target or scope is genuinely ambiguous.
- Never fabricate workspace data — if a tool cannot answer it, say so and offer to launch a run that can.`)
}

func filterPresetTools(base []string, required ...string) []string {
	requiredSet := make(map[string]bool, len(required))
	for _, toolName := range required {
		requiredSet[toolName] = true
	}
	filtered := make([]string, 0, len(base))
	for _, toolName := range base {
		switch toolName {
		case agentcontract.ToolPreviewMarkdown,
			agentcontract.ToolPreviewJSON,
			agentcontract.ToolPublishPreview,
			agentcontract.ToolPublishPRDDraft,
			agentcontract.ToolPublishTaskPlan,
			agentcontract.ToolPublishTaskPlanDoc:
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
