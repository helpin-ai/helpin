package service

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/worker"
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
		agent.SystemPrompt = storedSystemPromptForPreset(presetKey, agent.SystemPrompt, agent.PlanningNotes)
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
		agent.ApprovalMode = "never"
		agent.Schedule = nil
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
			agent.RuntimeKind = "opencode"
		}
	} else if hasPreset && !runtimeAllowedForPreset(presetKey, agent.RuntimeKind) {
		if hasPreset && preset.RuntimeKind != "" {
			agent.RuntimeKind = preset.RuntimeKind
		} else {
			agent.RuntimeKind = "opencode"
		}
	}
	if agent.Skills == nil {
		agent.Skills = json.RawMessage("[]")
	}
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

func migrateLegacyPreviewTools(raw json.RawMessage, presetKey string) json.RawMessage {
	tools := parseJSONStringSlice(raw)
	if len(tools) == 0 || !slices.Contains(tools, worker.ToolPublishPreview) {
		return raw
	}
	if slices.Contains(tools, worker.ToolPublishPRDDraft) || slices.Contains(tools, worker.ToolPublishStoryPlan) || slices.Contains(tools, worker.ToolPublishStoryPlanDoc) {
		return raw
	}

	migrated := make([]string, 0, len(tools)+3)
	for _, toolName := range tools {
		switch toolName {
		case worker.ToolPublishPreview, worker.ToolPreviewMarkdown, worker.ToolPreviewJSON, worker.ToolPublishPRDDraft, worker.ToolPublishStoryPlan, worker.ToolPublishStoryPlanDoc:
			continue
		}
		migrated = append(migrated, toolName)
	}
	switch normalizePresetKey(presetKey) {
	case model.AgentPresetEpicPlanner:
		migrated = append(migrated, worker.ToolPublishPRDDraft, worker.ToolPublishStoryPlan)
	case model.AgentPresetStoryPlanner:
		migrated = append(migrated, worker.ToolPublishStoryPlanDoc)
	}
	return mustJSONStringSlice(migrated)
}

func normalizeAllowedToolsJSON(raw json.RawMessage) json.RawMessage {
	tools := parseJSONStringSlice(raw)
	if len(tools) == 0 {
		return raw
	}
	normalized := worker.NormalizeToolNames(tools)
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
			worker.ToolUpdatePlan,
			worker.ToolPublishPRDDraft,
			worker.ToolPublishStoryPlan,
		}
		policy.disallowedExtraTools = []string{
			worker.ToolPreviewMarkdown,
			worker.ToolPreviewJSON,
			worker.ToolPublishPreview,
			worker.ToolPublishStoryPlanDoc,
			"ensure_epic_spec_doc",
			"ensure_story_plan_doc",
			"write_document_content",
			"link_document_to_object",
			"approve_epic_spec",
			"create_story_batch",
			"assign_story_agent",
			"set_story_dependencies",
		}
	case model.AgentPresetStoryPlanner:
		policy.requiredTools = []string{
			worker.ToolUpdatePlan,
			worker.ToolPublishStoryPlanDoc,
		}
		policy.disallowedExtraTools = []string{
			worker.ToolPreviewMarkdown,
			worker.ToolPreviewJSON,
			worker.ToolPublishPreview,
			worker.ToolPublishPRDDraft,
			worker.ToolPublishStoryPlan,
			"ensure_epic_spec_doc",
			"ensure_story_plan_doc",
			"write_document_content",
			"link_document_to_object",
			"approve_epic_spec",
			"create_story_batch",
			"assign_story_agent",
			"set_story_dependencies",
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
	case "opencode":
		return false
	case "codex":
		switch normalizePresetKey(agent.EffectivePresetKey()) {
		case model.AgentPresetCodeBuilder, model.AgentPresetReviewAgent:
			return true
		default:
			return false
		}
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
	presetKey := normalizePresetKey(agent.EffectivePresetKey())
	if presetKey != "" && !runtimeAllowedForPreset(presetKey, agent.RuntimeKind) {
		return fmt.Errorf("runtime_kind %q is not allowed for preset %q", strings.TrimSpace(agent.RuntimeKind), presetKey)
	}
	if strings.TrimSpace(agent.RuntimeKind) == "codex" {
		if presetKey == "" {
			return fmt.Errorf("runtime_kind codex requires a system preset agent")
		}
		return validateCodexAgentPolicy(agent, presetOverride)
	}
	return nil
}

func validateCodexAgentPolicy(agent *model.Agent, presetOverride *model.AgentPresetDefinition) error {
	if agent == nil {
		return nil
	}
	var (
		preset model.AgentPresetDefinition
		ok     bool
	)
	if presetOverride != nil {
		preset = *presetOverride
		ok = true
	} else {
		preset, ok = presetDefinitionForAgent(agent)
	}
	if !ok {
		return fmt.Errorf("runtime_kind codex requires a supported preset")
	}
	if !stringSliceSetEqual(parseJSONStringSlice(agent.AllowedTools), preset.AllowedTools) {
		return fmt.Errorf("runtime_kind codex does not support custom allowed_tools; use the preset defaults")
	}
	if !stringSliceSetEqual(parseJSONStringSlice(agent.AllowedCommands), preset.AllowedCommands) {
		return fmt.Errorf("runtime_kind codex does not support custom allowed_commands; use the preset defaults")
	}
	if !stringSliceSetEqual(parseJSONStringSlice(agent.AllowedTargets), preset.AllowedTargetTypes) {
		return fmt.Errorf("runtime_kind codex does not support custom allowed_targets; use the preset defaults")
	}
	return nil
}

func stringSliceSetEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	set := make(map[string]int, len(left))
	for _, value := range left {
		set[strings.TrimSpace(value)]++
	}
	for _, value := range right {
		normalized := strings.TrimSpace(value)
		if set[normalized] == 0 {
			return false
		}
		set[normalized]--
	}
	for _, remaining := range set {
		if remaining != 0 {
			return false
		}
	}
	return true
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
	resolved := worker.ResolveAgentProfile(agent, agent.DefaultInvocationMode)
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
