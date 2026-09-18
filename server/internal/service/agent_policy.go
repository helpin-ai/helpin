package service

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func normalizeModelProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "":
		return ""
	case model.AgentModelProviderAnthropic, "claude", "claude_direct":
		return model.AgentModelProviderAnthropic
	case model.AgentModelProviderOpenAI, "openai_direct":
		return model.AgentModelProviderOpenAI
	case model.AgentModelProviderOpenRouter:
		return model.AgentModelProviderOpenRouter
	case model.AgentModelProviderOpenRouterResponses:
		return model.AgentModelProviderOpenRouterResponses
	default:
		return strings.TrimSpace(provider)
	}
}

func validateAgentPresetKey(presetKey string) error {
	normalized := normalizePresetKey(presetKey)
	if normalized == "" {
		return nil
	}
	if _, ok := agentPresetDefinition(normalized); ok {
		return nil
	}
	return fmt.Errorf("preset_key %q is not supported", normalized)
}

// normalizeJSONSlice returns the input if non-nil, or an empty JSON array.
func normalizeJSONSlice(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage("[]")
	}
	return raw
}

var supportedAgentReasoningEfforts = sdk.ReasoningEfforts()
var supportedAgentServiceTiers = sdk.ServiceTiers()

var supportedAgentIconKeys = []string{
	"violet_star", "ocean_orbit", "forest_cap", "sunset_flame",
	"rose_wave", "teal_signal", "sky_quill", "amber_lens",
}

func normalizeAgentIconKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if slices.Contains(supportedAgentIconKeys, value) {
		return value
	}
	return ""
}

func validateAgentIconKey(value string) error {
	value = strings.TrimSpace(value)
	if value == "" || normalizeAgentIconKey(value) != "" {
		return nil
	}
	return fmt.Errorf("icon_key %q is not supported", value)
}

func jsonSliceIsEmpty(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return true
	}
	trimmed := strings.TrimSpace(string(raw))
	return trimmed == "" || trimmed == "null" || trimmed == "[]"
}

func normalizeTokenBudget(value *int) *int {
	if value == nil {
		return nil
	}
	if *value <= 0 {
		return nil
	}
	return value
}

func validateTriggerModeForAgent(triggerMode string, agent *model.Agent) error {
	if err := validateTriggerMode(triggerMode); err != nil {
		return err
	}
	presetKey := ""
	if agent != nil {
		presetKey = agent.EffectivePresetKey()
	}
	allowed := allowedTriggerModesForPresetKey(presetKey)
	if len(allowed) == 0 {
		allowed = []string{"manual"}
	}
	if !slices.Contains(allowed, triggerMode) {
		if presetKey := normalizePresetKey(presetKey); presetKey != "" {
			return fmt.Errorf("trigger_mode %q is not allowed for preset %q", triggerMode, presetKey)
		}
		return fmt.Errorf("trigger_mode %q is not allowed", triggerMode)
	}
	return nil
}

func normalizeAgentRecord(agent *model.Agent) {
	if agent == nil {
		return
	}
	agent.IconKey = normalizeAgentIconKey(agent.IconKey)

	presetKey := ""
	presetVersionKey := ""
	if agent.IsSystem {
		presetKey = normalizePresetKey(agent.PresetKey)
		if presetKey == "" {
			presetKey = defaultPresetKeyForAgent(true)
		}
		agent.PresetKey = presetKey
		presetVersionKey = normalizePresetVersionKey(agent.PresetVersionKey)
		if presetVersionKey == "" {
			presetVersionKey = defaultPresetVersionKeyForPresetKey(presetKey)
		}
		agent.PresetVersionKey = presetVersionKey
	} else {
		agent.PresetKey = ""
		agent.PresetVersionKey = ""
		agent.SourcePresetKey = ""
		agent.SourcePresetVersionKey = ""
	}
	preset, hasPreset := agentPresetVersionDefinition(presetKey, presetVersionKey)
	if agent.IsSystem && !hasPreset {
		preset, hasPreset = agentPresetVersionDefinition(presetKey, defaultPresetVersionKeyForPresetKey(presetKey))
	}
	if hasPreset {
		if agent.IsSystem && !strings.Contains(presetVersionKey, "_workspace_") {
			agent.SystemPrompt, agent.InstructionTemplateVersion = syncManagedSystemPromptForPreset(presetKey, agent.SystemPrompt, agent.PlanningNotes, agent.InstructionTemplateVersion)
		}
		agent.AllowedTools = normalizeAllowedToolsJSON(agent.AllowedTools)
		agent.AllowedTools = migrateLegacyPreviewTools(agent.AllowedTools, presetKey)
		agent.AllowedTools = sanitizePlannerAgentTools(agent.AllowedTools, presetKey)
		if jsonSliceIsEmpty(agent.AllowedTools) {
			agent.AllowedTools = mustJSONStringSlice(preset.AllowedTools)
		}
		if jsonSliceIsEmpty(agent.AllowedCommands) {
			agent.AllowedCommands = mustJSONStringSlice(preset.AllowedCommands)
		}
		if jsonSliceIsEmpty(agent.AllowedTargets) {
			agent.AllowedTargets = mustJSONStringSlice(preset.AllowedTargetTypes)
		}
		if !agent.IsSystem && (strings.TrimSpace(agent.ApprovalMode) == "" || strings.TrimSpace(agent.ApprovalMode) == "preset_default") {
			agent.ApprovalMode = preset.ApprovalMode
		}
	} else if strings.TrimSpace(agent.ApprovalMode) == "" || strings.TrimSpace(agent.ApprovalMode) == "preset_default" {
		agent.ApprovalMode = "never"
	}
	if agent.IsSystem {
		if normalizePresetKey(agent.EffectivePresetKey()) == model.AgentPresetAskAgent && hasPreset {
			agent.ApprovalMode = preset.ApprovalMode
		} else {
			agent.ApprovalMode = "never"
		}
	}
	if strings.TrimSpace(agent.Role) == "" {
		if hasPreset && preset.DefaultRole != "" {
			agent.Role = preset.DefaultRole
		} else {
			agent.Role = "Agent"
		}
	}
	if strings.TrimSpace(agent.RuntimeKind) == "" {
		if hasPreset && preset.RuntimeKind != "" {
			agent.RuntimeKind = preset.RuntimeKind
		} else {
			agent.RuntimeKind = "native_sdk"
		}
	} else if hasPreset && !runtimeAllowedForPreset(presetKey, agent.RuntimeKind) {
		if hasPreset && preset.RuntimeKind != "" {
			agent.RuntimeKind = preset.RuntimeKind
		} else {
			agent.RuntimeKind = "native_sdk"
		}
	}
	if agent.Skills == nil {
		agent.Skills = model.AgentSkillRefs{}
	}
	agent.ExecutionConfig = normalizeExecutionConfigJSON(agent.ExecutionConfig)
	if agent.Provider != nil {
		normalized := normalizeModelProvider(*agent.Provider)
		if normalized == "" {
			agent.Provider = nil
		} else {
			agent.Provider = &normalized
		}
	} else if agent.Model != nil && strings.TrimSpace(*agent.Model) != "" {
		legacyProvider := model.AgentModelProviderAnthropic
		agent.Provider = &legacyProvider
	}
	if agent.IsSystem && hasPreset && strings.TrimSpace(preset.ModelTier) != "" {
		agent.ModelTier = strings.TrimSpace(preset.ModelTier)
	} else if strings.TrimSpace(agent.ModelTier) == "" {
		agent.ModelTier = deriveAgentModelTier(agent.Provider, agent.Model, agent.ExecutionConfig)
	}
	if strings.TrimSpace(agent.TriggerMode) == "" || validateTriggerModeForAgent(agent.TriggerMode, agent) != nil {
		if hasPreset && preset.DefaultTriggerMode != "" {
			agent.TriggerMode = preset.DefaultTriggerMode
		} else {
			agent.TriggerMode = "manual"
		}
	}
	if normalizePresetKey(presetKey) != model.AgentPresetEpicPlanner {
		agent.PlanningNotes = nil
	}
	agent.SupportedModes = supportedModesForAgent(agent)
	agent.DefaultInvocationMode = normalizeDefaultInvocationMode(agent.DefaultInvocationMode, agent)
}

func normalizeExecutionConfigJSON(raw []byte) model.JSONBlob {
	config, err := model.ParseAgentExecutionConfig(raw)
	if err != nil {
		return model.JSONBlob("{}")
	}
	return model.MarshalAgentExecutionConfig(config)
}

func parseAndValidateExecutionConfig(agent *model.Agent) (model.AgentExecutionConfig, error) {
	if agent == nil {
		return model.AgentExecutionConfig{}, nil
	}

	config, err := model.ParseAgentExecutionConfig(agent.ExecutionConfig)
	if err != nil {
		return model.AgentExecutionConfig{}, fmt.Errorf("execution_config must be a JSON object")
	}
	if config.IsZero() {
		return config, nil
	}

	runtimeKind := strings.TrimSpace(agent.RuntimeKind)
	if config.NativeContext != nil {
		if runtimeKind != "native_sdk" {
			return model.AgentExecutionConfig{}, fmt.Errorf("execution_config.native_context is only supported for runtime_kind native_sdk")
		}
		if err := config.NativeContext.Validate(); err != nil {
			return model.AgentExecutionConfig{}, err
		}
	}
	if config.MaxToolSteps != nil {
		if runtimeKind != "native_sdk" {
			return model.AgentExecutionConfig{}, fmt.Errorf("execution_config.max_tool_steps is only supported for runtime_kind native_sdk")
		}
		if *config.MaxToolSteps < model.MinNativeToolSteps || *config.MaxToolSteps > model.MaxNativeToolSteps {
			return model.AgentExecutionConfig{}, fmt.Errorf(
				"execution_config.max_tool_steps must be between %d and %d",
				model.MinNativeToolSteps,
				model.MaxNativeToolSteps,
			)
		}
	}

	if (config.ReasoningEffort != nil || config.ServiceTier != nil) && runtimeKind != "native_sdk" {
		return model.AgentExecutionConfig{}, fmt.Errorf("execution_config model controls are only supported for runtime_kind native_sdk")
	}

	provider := "openai"
	if agent.Provider != nil && strings.TrimSpace(*agent.Provider) != "" {
		provider = normalizeModelProvider(*agent.Provider)
	}
	controls := sdk.ModelControls{ReasoningEffort: config.ReasoningEffort, ServiceTier: config.ServiceTier}
	if config.OpenRouter != nil {
		controls.OpenRouter = &sdk.OpenRouterModelControls{}
		if config.OpenRouter.Provider != nil {
			controls.OpenRouter.Provider = &sdk.OpenRouterProviderPreferences{Quantizations: config.OpenRouter.Provider.Quantizations}
		}
	}
	if err := sdk.ValidateModelControls(provider, controls); err != nil {
		return model.AgentExecutionConfig{}, fmt.Errorf("execution_config.%w", err)
	}

	return config, nil
}

func migrateLegacyPreviewTools(raw json.RawMessage, presetKey string) json.RawMessage {
	tools := parseJSONStringSlice(raw)
	if len(tools) == 0 || !slices.Contains(tools, agentcontract.ToolPublishPreview) {
		return raw
	}
	if slices.Contains(tools, agentcontract.ToolPublishPRDDraft) || slices.Contains(tools, agentcontract.ToolPublishTaskPlan) || slices.Contains(tools, agentcontract.ToolPublishTaskPlanDoc) {
		return raw
	}

	migrated := make([]string, 0, len(tools)+3)
	for _, toolName := range tools {
		switch toolName {
		case agentcontract.ToolPublishPreview, agentcontract.ToolPreviewMarkdown, agentcontract.ToolPreviewJSON, agentcontract.ToolPublishPRDDraft, agentcontract.ToolPublishTaskPlan, agentcontract.ToolPublishTaskPlanDoc:
			continue
		}
		migrated = append(migrated, toolName)
	}
	switch normalizePresetKey(presetKey) {
	case model.AgentPresetEpicPlanner:
		migrated = append(migrated, agentcontract.ToolPublishPRDDraft, agentcontract.ToolPublishTaskPlan)
	case model.AgentPresetTaskPlanner:
		migrated = append(migrated, agentcontract.ToolPublishTaskPlanDoc)
	}
	return mustJSONStringSlice(migrated)
}

func normalizeAllowedToolsJSON(raw json.RawMessage) json.RawMessage {
	tools := parseJSONStringSlice(raw)
	if len(tools) == 0 {
		return raw
	}
	normalized := agentcontract.NormalizeToolNames(tools)
	if len(normalized) == 0 {
		return json.RawMessage("[]")
	}
	if slices.Equal(normalized, tools) {
		return raw
	}
	return mustJSONStringSlice(normalized)
}

func sanitizePlannerAgentTools(raw json.RawMessage, presetKey string) json.RawMessage {
	tools := parseJSONStringSlice(raw)
	if len(tools) == 0 {
		return raw
	}

	type plannerPolicy struct {
		requiredTools        []string
		disallowedExtraTools []string
	}
	var policy plannerPolicy
	switch normalizePresetKey(presetKey) {
	case model.AgentPresetEpicPlanner:
		policy.requiredTools = []string{
			agentcontract.ToolUpdatePlan,
			agentcontract.ToolRequestApproval,
			agentcontract.ToolPublishPRDDraft,
			agentcontract.ToolPublishTaskPlan,
			"ensure_epic_spec_doc",
			"write_document_content",
			"approve_epic_spec",
			"create_task_batch",
		}
		policy.disallowedExtraTools = []string{
			agentcontract.ToolPreviewMarkdown,
			agentcontract.ToolPreviewJSON,
			agentcontract.ToolPublishPreview,
			agentcontract.ToolPublishTaskPlanDoc,
			"ensure_task_plan_doc",
			"link_document_to_object",
			"assign_task_agent",
			"set_task_dependencies",
			"write_file",
			"edit_file",
			"apply_patch",
		}
	case model.AgentPresetTaskPlanner:
		policy.requiredTools = []string{
			agentcontract.ToolUpdatePlan,
			agentcontract.ToolRequestApproval,
			agentcontract.ToolPublishTaskPlanDoc,
			"ensure_task_plan_doc",
			"write_document_content",
		}
		policy.disallowedExtraTools = []string{
			agentcontract.ToolPreviewMarkdown,
			agentcontract.ToolPreviewJSON,
			agentcontract.ToolPublishPreview,
			agentcontract.ToolPublishPRDDraft,
			agentcontract.ToolPublishTaskPlan,
			"ensure_epic_spec_doc",
			"link_document_to_object",
			"approve_epic_spec",
			"create_task_batch",
			"assign_task_agent",
			"set_task_dependencies",
			"list_epic_tasks",
			"write_file",
			"edit_file",
			"apply_patch",
		}
	default:
		return raw
	}

	requiredSet := make(map[string]bool, len(policy.requiredTools))
	for _, toolName := range policy.requiredTools {
		requiredSet[toolName] = true
	}

	disallowedSet := make(map[string]bool, len(policy.disallowedExtraTools))
	for _, toolName := range policy.disallowedExtraTools {
		disallowedSet[toolName] = true
	}

	filtered := make([]string, 0, len(tools))
	for _, toolName := range tools {
		if disallowedSet[toolName] && !requiredSet[toolName] {
			continue
		}
		filtered = append(filtered, toolName)
	}
	for _, toolName := range policy.requiredTools {
		if !slices.Contains(filtered, toolName) {
			filtered = append(filtered, toolName)
		}
	}
	return mustJSONStringSlice(filtered)
}

func normalizeDefaultInvocationMode(value string, agent *model.Agent) string {
	presetKey := ""
	if agent != nil {
		presetKey = normalizePresetKey(agent.EffectivePresetKey())
	}
	switch strings.TrimSpace(value) {
	case model.InvocationModeInteractive:
		if agentSupportsMode(agent, model.InvocationModeInteractive) {
			return model.InvocationModeInteractive
		}
	case model.InvocationModeAutonomous:
		if agentSupportsMode(agent, model.InvocationModeAutonomous) {
			return model.InvocationModeAutonomous
		}
	}

	if preset, ok := agentPresetDefinition(presetKey); ok {
		if slices.Contains(supportedModesForAgent(agent), preset.DefaultInvocationMode) {
			return preset.DefaultInvocationMode
		}
	}

	return model.InvocationModeAutonomous
}

func supportedModesForAgent(agent *model.Agent) []string {
	if agent == nil {
		return []string{}
	}
	modes := []string{model.InvocationModeAutonomous}
	if agentSupportsInteractive(agent) {
		modes = append(modes, model.InvocationModeInteractive)
	}
	return modes
}

func agentSupportsMode(agent *model.Agent, mode string) bool {
	return slices.Contains(supportedModesForAgent(agent), mode)
}

func agentSupportsInteractive(agent *model.Agent) bool {
	if agent == nil {
		return false
	}
	switch strings.TrimSpace(agent.RuntimeKind) {
	case "native_sdk":
		return true
	}
	if agent.Provider != nil && strings.TrimSpace(*agent.Provider) != "" {
		switch normalizeModelProvider(*agent.Provider) {
		case model.AgentModelProviderAnthropic, model.AgentModelProviderOpenAI, model.AgentModelProviderOpenRouter, model.AgentModelProviderOpenRouterResponses:
			return true
		}
	}
	return false
}

func validateRuntimeForAgent(agent *model.Agent) error {
	return validateRuntimeForAgentWithPreset(agent, nil)
}

func validateRuntimeForAgentWithPreset(agent *model.Agent, presetOverride *model.AgentPresetDefinition) error {
	if agent == nil {
		return nil
	}
	if agent.RuntimeKind != "native_sdk" {
		return fmt.Errorf("only native_sdk is available for new runs; select a native agent version")
	}
	presetKey := normalizePresetKey(agent.EffectivePresetKey())
	if presetKey != "" && !runtimeAllowedForPreset(presetKey, agent.RuntimeKind) {
		return fmt.Errorf("runtime_kind %q is not allowed for preset %q", strings.TrimSpace(agent.RuntimeKind), presetKey)
	}
	return nil
}

func validateModelProvider(provider string) error {
	switch normalizeModelProvider(provider) {
	case model.AgentModelProviderAnthropic, model.AgentModelProviderOpenAI, model.AgentModelProviderOpenRouter, model.AgentModelProviderOpenRouterResponses:
		return nil
	default:
		return fmt.Errorf("provider must be one of anthropic, openai, openrouter, openrouter-responses")
	}
}

func validateAgentTarget(agent *model.Agent, targetType string) error {
	normalizeAgentRecord(agent)
	if err := validateAgentPresetKey(agent.EffectivePresetKey()); err != nil {
		return err
	}
	resolved := agentcontract.ResolveAgentProfile(agent, agent.DefaultInvocationMode)
	if len(resolved.TargetTypes) == 0 {
		return fmt.Errorf("agent is not runnable")
	}
	if !slices.Contains(resolved.TargetTypes, targetType) {
		return fmt.Errorf("agent can only be assigned to %s", strings.Join(resolved.TargetTypes, ", "))
	}
	return nil
}

func normalizePlanningMethodology(value string) string {
	return model.NormalizePlanningMethodology(strings.TrimSpace(value))
}
