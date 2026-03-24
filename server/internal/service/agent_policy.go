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
		presetKey = agent.PresetKey
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

	presetKey := normalizePresetKey(agent.PresetKey)
	if presetKey == "" {
		presetKey = defaultPresetKeyForAgent(agent.IsSystem)
	}
	agent.PresetKey = presetKey

	preset, hasPreset := agentPresetDefinition(presetKey)
	if hasPreset {
		if jsonSliceIsEmpty(agent.AllowedTools) {
			agent.AllowedTools = mustJSONStringSlice(preset.AllowedTools)
		}
		if jsonSliceIsEmpty(agent.AllowedCommands) {
			agent.AllowedCommands = mustJSONStringSlice(preset.AllowedCommands)
		}
		if jsonSliceIsEmpty(agent.AllowedTargets) {
			agent.AllowedTargets = mustJSONStringSlice(preset.AllowedTargetTypes)
		}
		if strings.TrimSpace(agent.ApprovalMode) == "" || strings.TrimSpace(agent.ApprovalMode) == "preset_default" {
			agent.ApprovalMode = preset.ApprovalMode
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
	if normalizePresetKey(agent.PresetKey) != model.AgentPresetEpicPlanner {
		agent.PlanningNotes = nil
	}
	agent.SupportedModes = supportedModesForAgent(agent)
	agent.DefaultInvocationMode = normalizeDefaultInvocationMode(agent.DefaultInvocationMode, agent)
}

func normalizeDefaultInvocationMode(value string, agent *model.Agent) string {
	presetKey := ""
	if agent != nil {
		presetKey = normalizePresetKey(agent.PresetKey)
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
	if err := validateAgentPresetKey(agent.PresetKey); err != nil {
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
