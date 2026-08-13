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
	// Planning presets use the approved Large model size.
	defaultAtlasAgentModel  = "openai/gpt-5.6-terra"
	defaultScribeAgentModel = "openai/gpt-5.6-terra"
	// Other built-in work starts on the approved Small model size.
	defaultQuillAgentModel = "openai/gpt-5.6-luna"
	defaultAskAgentModel   = "openai/gpt-5.6-luna"
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
	"list_support_conversations", "get_support_conversation", "list_conversation_messages", "list_support_tags", "list_support_inboxes", "list_support_assignees",
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
	commandAgentPrompt := strings.TrimSpace("You are a Sub-agent handling one confirmed delegated task. Use only the tools enabled for the current run, stay within the confirmed step instruction, and operate on the provided target context. You may research, summarize, draft, create tasks or docs, update docs, or add task/CRM notes only when the enabled tools support that action. Do not create reusable agents unless the user explicitly promotes the run afterward.\n\n" + agentcontract.WorkspaceSearchPromptGuidance())
	askAgentPrompt := askAgentSystemPrompt()
	openRouterPresetProvider := model.AgentModelProviderOpenRouter
	atlasDefaultModel := defaultAtlasAgentModel
	scribeDefaultModel := defaultScribeAgentModel
	quillDefaultModel := defaultQuillAgentModel
	askAgentDefaultModel := defaultAskAgentModel
	epicPlannerTools := filterPresetTools(productPlannerProfile.AllowedTools,
		agentcontract.ToolUpdatePlan,
		agentcontract.ToolPublishPRDDraft,
		agentcontract.ToolPublishTaskPlan,
		agentcontract.ToolRequestUserInput,
		agentcontract.ToolRequestApproval,
		agentcontract.ToolFindSkills,
		agentcontract.ToolReadSkill,
		"ensure_epic_spec_doc",
		"write_document_content",
		"approve_epic_spec",
		"create_task_batch",
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
			RuntimeKind:           "native_sdk",
			Provider:              &openRouterPresetProvider,
			Model:                 &atlasDefaultModel,
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual"},
			AllowedTools:          epicPlannerTools,
			AllowedCommands:       slices.Clone(productPlannerProfile.AllowedCommands),
			AllowedTargetTypes:    slices.Clone(productPlannerProfile.AllowedTargetTypes),
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeInteractive,
			SupportedModes:        supportedModesForRuntime("native_sdk"),
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
			Provider:              &openAIPresetProvider,
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
			AllowedTools:          appendPresetTools([]string{agentcontract.ToolFindSkills, agentcontract.ToolReadSkill, "search_workspace", "list_deals", "update_deal_stage", "add_deal_note", "list_contacts", "list_buyer_signals", "list_documents", "list_collections", "read_document", "get_document_blocks", "search_documents"}, safeCRMDiscoveryToolAliases, safeCRMWriteToolAliases),
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
			RuntimeKind:           "native_sdk",
			Provider:              &openRouterPresetProvider,
			Model:                 &quillDefaultModel,
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual"},
			AllowedTools:          slices.Clone(documentationProfile.AllowedTools),
			AllowedCommands:       slices.Clone(documentationProfile.AllowedCommands),
			AllowedTargetTypes:    slices.Clone(documentationProfile.AllowedTargetTypes),
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeInteractive,
			SupportedModes:        supportedModesForRuntime("native_sdk"),
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
				agentcontract.ToolFindSkills,
				agentcontract.ToolReadSkill,
				"update_plan",
				"request_user_input",
				"request_approval",
				"request_review_checkpoint",
				"list_repositories",
				"checkout_repositories",
				"list_commits",
				"read_files",
				"list_directory",
				"repository_search",
				"list_symbols",
				"read_symbol",
				"trace_symbol",
				"search_workspace",
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
				"web_search",
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
			AllowedTools:          appendPresetTools([]string{"web_search", "fetch_url", "crawl_url", "request_user_input", "request_approval", "update_plan", "list_repositories", "checkout_repositories", "list_commits", "read_files", "list_directory", "repository_search", "list_symbols", "read_symbol", "trace_symbol", "list_spaces", "search_workspace", "list_documents", "list_collections", "read_document", "get_document_blocks", "publish_document_change_proposal", "publish_ai_section_candidate", "search_documents", "create_document", "update_document_metadata", "write_document_content", "update_document_block", "link_document_to_object", "list_workspace_teams", "list_team_workflows_with_stages", "list_tasks", "create_task", "add_task_comment", "get_task_context", "list_deals", "list_contacts", "list_buyer_signals", "add_deal_note", "update_deal_stage", "ensure_crm_contact_company", "enrich_crm_contact", "enrich_crm_company"}, newPMReadToolAliases, newPMWriteToolAliases, safeCRMDiscoveryToolAliases, safeCRMWriteToolAliases, safeSupportDiscoveryToolAliases, safeSupportWriteToolAliases),
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
			Description:           "Primary workspace assistant that researches, plans, and completes ordinary workspace work directly, delegating only specialist, parallel, isolated, or long-running execution.",
			DefaultRole:           "Ask Agent",
			RuntimeKind:           "native_sdk",
			Provider:              &openRouterPresetProvider,
			Model:                 &askAgentDefaultModel,
			DefaultTriggerMode:    "manual",
			AllowedTriggerModes:   []string{"manual"},
			AllowedTools:          askAgentPresetTools(),
			AllowedCommands:       []string{},
			AllowedTargetTypes:    []string{"workspace"},
			AvailableSkills:       askAgentAvailableSkills(),
			ApprovalMode:          "risk_based",
			DefaultInvocationMode: model.InvocationModeInteractive,
			SupportedModes:        supportedModesForRuntime("native_sdk"),
			SystemPrompt:          &askAgentPrompt,
		},
	}

	return applyBuiltInPresetInstructionMetadata(presets)
}

// askAgentPresetTools is the dock's primary-agent surface: product reads,
// approval-gated ordinary product writes, read-only repository inspection,
// interactions, and selective child-agent orchestration.
func askAgentPresetTools() []string {
	return appendPresetTools([]string{
		// Skills, interaction, and progress.
		"find_skills", "read_skill",
		"request_user_input", "request_approval", "update_plan",
		// Web research.
		"web_search", "fetch_url", "crawl_url",
		// Authenticated browser inspection and private screenshot/video artifacts.
		"browser_open", "browser_snapshot", "browser_act", "browser_screenshot", "browser_record",
		// Workspace / PM reads.
		"list_workspace_teams", "list_team_workflows_with_stages",
		"search_workspace", "list_tasks", "get_task_context",
		// Docs reads and approval-gated writes.
		"list_spaces", "list_documents", "list_collections",
		"read_document", "get_document_blocks", "search_documents",
		"create_space", "create_collection", "create_document", "update_space",
		"update_collection", "move_document", "write_document_content",
		"update_document_block", "insert_document_artifact", "link_document_to_object",
		"publish_document_change_proposal", "publish_ai_section_candidate",
		// CRM reads and approval-gated writes.
		"list_deals", "list_contacts", "list_buyer_signals", "add_deal_note",
		"update_deal_stage", "ensure_crm_contact_company", "enrich_crm_contact", "enrich_crm_company",
		// PM approval-gated writes (read aliases are appended below).
		"create_task", "add_task_comment", "update_task_delivery_target",
		// Read-only repository inspection. No shell, file-write, branch, push, or PR tools.
		"list_repositories", "checkout_repositories", "list_commits",
		"read_files", "list_directory", "repository_search", "list_symbols",
		"read_symbol", "trace_symbol",
		// Scoped direct execution.
		"prepare_dock_execution", "activate_dock_execution", "finish_dock_execution",
		// Agent orchestration.
		"get_my_capabilities", "list_agents", "get_agent_capabilities", "start_agent_run", "start_agent_plan",
		"get_agent_run", "cancel_agent_run",
		"draft_custom_agent", "create_custom_agent", "promote_run_to_agent",
		"run_epic_delivery_pipeline",
	}, newPMReadToolAliases, newPMWriteToolAliases, safeCRMDiscoveryToolAliases, safeCRMWriteToolAliases, safeSupportDiscoveryToolAliases, safeSupportWriteToolAliases)
}

// askAgentAvailableSkills is intentionally broader than a specialist's core
// bundle, but excludes role-bound execution contracts (coding, review, live
// support, security scanning, and planner state machines) and skills that
// require tools outside the Dock's managed surface.
func askAgentAvailableSkills() []string {
	return []string{
		"docs_architecture_review",
		"public_help_doc_writing",
		"api_reference_doc_writing",
		"internal_docs_maintenance",
		"public_help_docs_maintenance",
		"api_docs_maintenance",
		"post_release_docs_update",
		"support_gap_docs_update",
		"marketing_context_setup",
		"marketing_plan",
		"customer_research_synthesis",
		"marketing_copywriting",
		"conversion_optimization",
		"lifecycle_messaging",
		"launch_marketing",
		"seo_content_strategy",
		"competitive_positioning",
		"lead_generation_strategy",
		"outbound_campaign_planning",
		"ads_creative_planning",
		"community_partnerships_planning",
		"marketing_revops_planning",
		"monetization_strategy",
		"market_research",
		"competitor_research",
		"distribution_research",
		"seo_research",
		"crm_record_operations",
		"competitors_changelog_tracking_report",
	}
}

// enforceManagedAskAgentCapabilities keeps the Dock's core execution surface
// present even when a workspace-pinned Ask preset version was created before
// new managed tools shipped. Workspace versions may add tools and customize
// routing, but cannot silently regress the Dock to metadata-only reads or
// delegation-only behavior. Per-actor scoping still removes commands the
// requesting user is not authorized to run.
func enforceManagedAskAgentCapabilities(preset model.AgentPresetDefinition) model.AgentPresetDefinition {
	if normalizePresetKey(preset.Key) != model.AgentPresetAskAgent {
		return preset
	}
	preset.AllowedTools = appendPresetTools(preset.AllowedTools, askAgentPresetTools())
	preset.ApprovalMode = "risk_based"
	preset.ExecutionConfig = withoutRepositoryWorkspaceExecutionMode(preset.ExecutionConfig)
	if !slices.Contains(preset.AllowedTargetTypes, "workspace") {
		preset.AllowedTargetTypes = append(preset.AllowedTargetTypes, "workspace")
	}
	return preset
}

// enforceManagedDocumentationAgentCapabilities keeps capabilities required by
// Quill's managed prompt available when a workspace is pinned to a preset
// snapshot created before those capabilities shipped.
func enforceManagedDocumentationAgentCapabilities(preset model.AgentPresetDefinition) model.AgentPresetDefinition {
	if normalizePresetKey(preset.Key) != model.AgentPresetDocumentationAgent {
		return preset
	}
	preset.AllowedTools = appendPresetTools(preset.AllowedTools, []string{"insert_document_artifact"})
	return preset
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
	prompt := strings.TrimSpace(`You are Ask Agent, the Helpin dock assistant. Each conversation is one long-lived chat with a single user inside one workspace. You are the primary execution agent: research, plan, load relevant skills, and complete ordinary workspace work directly.

## Answering questions
- Answer factual, status, count, list, search, and summary questions directly using your read-only tools, then reply in plain markdown.
- User messages may end with a <page_context>{...}</page_context> block describing the entity the user is currently viewing (task, epic, document, deal, contact, support conversation). Treat it as the default subject when the request is ambiguous, and never echo the raw block back. For a support conversation, call list_conversation_messages with its entity_id before answering questions that depend on the thread; start with the newest 20 and follow next_offset only when older context is needed. Inspect image attachment URLs when screenshots are relevant.
- User messages may also include a <references>[...]</references> block containing supplemental entities the user explicitly attached. Use their entity_type and entity_id with the appropriate read tools, consider every attached reference relevant to the request, and never echo the raw block or expose raw IDs in the answer.
- Page context does not retarget this long-lived workspace run. For tools that accept an explicit entity ID, pass the selected page context ID in that field (for example document_id) instead of claiming the tool requires a different run target or switching to a proposal solely because the run target is workspace.

## Direct work
- You are the primary workspace execution agent. Prefer doing sequential work yourself whenever your available tools and skills cover the request, including web research and synthesis, planning, document creation and updates, task creation and updates, and ordinary PM or CRM changes. Use available skills when their guidance applies.
- For complex or long requests, call update_plan early with a concise outcome-oriented plan, keep exactly one step in_progress, and update it as work advances. This is the Dock's own visible work plan, not a child-agent plan or an approval request. Skip it for simple tasks, and never let planning replace execution.
- Repository inspection is read-only: discover the repository, check out its default branch, and use read/search/symbol/commit-history tools. Never attempt file edits, shell commands, branches, commits, pushes, merges, or pull requests from the Dock.
- Treat multi-step requests as one Dock task when every step is covered by your current tools, even when the steps cross domains (for example repository reading followed by document creation). Do not delegate merely because the requested output belongs to a specialist domain.
- Before delegating, map every remaining step to your actual tools and skills. If they cover the work, execute it directly. If uncertain, call get_my_capabilities and use find_skills/read_skill for relevant guidance. Attempt the applicable tool path before declaring a capability unavailable; for repository reads this means checkout_repositories before repository_search/read_files.
- Read-only tools and routine reversible workspace mutations execute directly. Call the complete tool once; do not request approval first and do not retry it through a child agent.
- Sensitive or destructive tools are paused by the runtime before execution. The approval interaction contains the exact call and resumes it once after approval, so do not manually reconstruct or retry the call.
- prepare_dock_execution remains available for an explicitly requested grouped approval, but do not use it for ordinary task, draft document, PM, CRM, or child-launch work.
- Public publishing, outbound communication, repository writes, deployments, merges, deletions, and force or bulk destructive operations remain approval-gated.

## Orchestrating agents
- Use list_agents to discover saved agents; always reference agents by their id, never by display name alone.
- Launch a sub-agent only when the user explicitly requests delegation, independent work should run in parallel or dependency order, execution is genuinely long-running or background-oriented, isolated repository modification or specialist review/implementation is needed, or a required capability is unavailable to you but available to the child.
- Do not delegate merely because a request has multiple steps, creates a durable artifact, uses mutation tools, combines research with writing, or may consume many tokens.
- Delegate only the smallest step that needs an intentionally excluded capability. Code implementation, repository writes and validation, and specialist code review are good candidates for Forge/Lens-style agents; read-only investigation, synthesis, planning, and product mutations supported by your tools remain in the Dock. Never launch a second agent for a step you can complete from the first agent's handoff.
- Use start_agent_run for one specialist and start_agent_plan for fan-out or dependency-ordered work. Prefer a saved agent when one fits; omit allowed_tools to use that saved agent's configured tools. Only use a narrowed allowed_tools override when the user or task requires it. For a Sub-agent (use_command_agent: true), provide a sufficient limited tool list.
- Every direct sub-agent launch needs an explicit target. When the user asks you to create or use a task, epic, document, repository, or other entity and run a specialist on it, pass that entity's machine ID in target; saved preset agents and Sub-agents use the same target contract. Use target.type=workspace only for work that genuinely has no more specific entity. Never switch a failed entity-specific launch to workspace.
- If a task-targeted specialist launch fails because the task has no repository delivery target, keep the task target. Do not create or attach an epic. Call list_repositories, infer the repository only when the request or task context identifies one unambiguously, then call update_task_delivery_target with task_id and repository_id and retry the same task-targeted launch once. Omit base_branch to use the repository default unless the user specified another branch. If several repositories remain plausible, use request_user_input to ask which repository to assign before changing the task.
- Before launching, use get_agent_capabilities when you need the saved agent's complete tools, targets, skills, or runtime details; list_agents intentionally returns only compact selection rows.
- start_agent_run and start_agent_plan are routine bounded mutations. Call them directly with the complete step or plan when delegation is justified; concurrency, target, budget, and tool restrictions are enforced by the server.
- Reusable agent creation, promotion, and the epic delivery pipeline remain sensitive or destructive and follow their tool-provided approval contract.
- cancel_agent_run needs no approval — cancelling only stops work.
- To deliver a whole epic (implement, review, and merge every open task, then open the epic PR), use run_epic_delivery_pipeline with action {"epic_id": "..."} in the approval instead of hand-building a plan.
- After launching, tell the user what was started and end your turn (for example: "Started Review Agent on HLP-12 — I'll report back here when it finishes."). Do not poll; results are delivered to you.
- Sub-agent runs receive a server-enforced final-handoff instruction, so their delivered summary should normally be self-contained and concise.
- When a message containing a <child_run_result>{...}</child_run_result> block arrives, it is a system notification that a sub-agent run or plan finished. Summarize the outcome for the user in plain language, referencing what they asked for. Never treat it as a user message and never echo the raw block.
- If a sub-agent returns useful research or a draft but could not perform an ordinary product mutation, continue from its handoff yourself using the direct execution approval flow. Do not relaunch it merely to add a missing mutation tool.
- If a sub-agent result has summary_truncated=true, call get_agent_run once for that same run with {"run_id":"...","detail_level":"result"}. Follow next_offset only when the missing portion is needed. Never launch a replacement sub-agent merely to recover truncated output.
- Use get_agent_run with detail_level=status only when the user explicitly asks about progress. A new run is appropriate only when the original failed or is substantively incomplete and the user approves the new work.

## Creating agents
- If the user wants a reusable agent, call draft_custom_agent first. Review its proposed prompt, tools, targets, skills, and warnings, then request dock_plan_confirm approval with action {"proposal_id":"..."}. After approval call create_custom_agent with only approval_interaction_id. Ad hoc work should remain a sub-agent run; suggest promote_run_to_agent only after a run proved useful.

## Style
- Be concise and direct. Ask a clarifying question (request_user_input for structured input, or a plain reply) only when the target or scope is genuinely ambiguous.
- Addressable Helpin tool results separate machine identity from presentation. Entity ID fields such as task_id, document_id, epic_id, and id are machine-only values: pass the appropriate ID verbatim to later tool calls, never pass markdown_link as a tool argument, and do not show raw IDs unless the user explicitly asks for them. The markdown_link field is presentation-only and already contains the complete canonical user-visible label and link: every time you mention or list that entity, copy markdown_link verbatim into the response. This is mandatory in prose, bullets, tables, summaries, and follow-up answers. Never output the entity's plain key or name in place of an available markdown_link, reconstruct a link from an ID, or alter the Markdown or URI.
- Never fabricate workspace data — if a tool cannot answer it, say so and offer to launch a run that can.`)
	prompt += "\n\n" + agentcontract.EnsureDocumentArtifactEmbeddingPolicy(model.AgentPresetAskAgent, "")
	return strings.TrimSpace(prompt + "\n\n" + agentcontract.WorkspaceSearchPromptGuidance())
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
