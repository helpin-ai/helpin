package service

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/worker"
)

func normalizeAgentClass(agentClass, capabilityProfile, role string) string {
	raw := strings.TrimSpace(agentClass)
	if raw == "" {
		raw = strings.TrimSpace(capabilityProfile)
	}
	if raw == "" {
		raw = defaultCapabilityProfileForRole(role)
	}
	raw = worker.NormalizeCapabilityProfile(raw)

	switch raw {
	case model.AgentClassProductPlanner,
		model.AgentClassEngineer,
		model.AgentClassReviewer,
		model.AgentClassSupport:
		return raw
	default:
		return raw
	}
}

func capabilityProfileForAgentClass(agentClass string) string {
	switch normalizeAgentClass(agentClass, "", "") {
	case model.AgentClassProductPlanner:
		return model.AgentClassProductPlanner
	case model.AgentClassReviewer:
		return model.AgentClassReviewer
	case model.AgentClassSupport:
		return model.AgentClassSupport
	default:
		return model.AgentClassEngineer
	}
}

func defaultRoleForAgentClass(agentClass string) string {
	switch normalizeAgentClass(agentClass, "", "") {
	case model.AgentClassProductPlanner:
		return "Product Planner"
	case model.AgentClassReviewer:
		return "Reviewer"
	case model.AgentClassSupport:
		return "Support"
	default:
		return "Engineer"
	}
}

func defaultRuntimeKindForAgentClass(agentClass string) string {
	return worker.GetRuntimeProfile(capabilityProfileForAgentClass(agentClass)).RuntimeKind
}

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
	default:
		return strings.TrimSpace(provider)
	}
}

func validateAgentClass(agentClass string) error {
	switch normalizeAgentClass(agentClass, "", "") {
	case model.AgentClassProductPlanner,
		model.AgentClassEngineer,
		model.AgentClassReviewer,
		model.AgentClassSupport:
		return nil
	default:
		return fmt.Errorf("agent_class must be one of product_planner, engineer, reviewer, support")
	}
}

// normalizeJSONSlice returns the input if non-nil, or an empty JSON array.
func normalizeJSONSlice(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage("[]")
	}
	return raw
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

func defaultTriggerModeForAgentClass(agentClass string) string {
	modes := allowedTriggerModesForAgentClass(agentClass)
	if len(modes) == 0 {
		return "manual"
	}
	return modes[0]
}

func allowedTriggerModesForAgentClass(agentClass string) []string {
	switch normalizeAgentClass(agentClass, "", "") {
	case model.AgentClassEngineer, model.AgentClassReviewer:
		return []string{"manual", "auto_on_assignment", "auto_on_event"}
	case model.AgentClassProductPlanner, model.AgentClassSupport:
		return []string{"manual"}
	default:
		return []string{"manual"}
	}
}

func validateTriggerModeForAgentClass(triggerMode, agentClass string) error {
	if err := validateTriggerMode(triggerMode); err != nil {
		return err
	}
	if !slices.Contains(allowedTriggerModesForAgentClass(agentClass), triggerMode) {
		return fmt.Errorf("trigger_mode %q is not allowed for agent_class %q", triggerMode, normalizeAgentClass(agentClass, "", ""))
	}
	return nil
}

func normalizeAgentRecord(agent *model.Agent) {
	if agent == nil {
		return
	}

	agent.AgentClass = normalizeAgentClass(agent.AgentClass, agent.CapabilityProfile, agent.Role)
	if strings.TrimSpace(agent.Role) == "" {
		agent.Role = defaultRoleForAgentClass(agent.AgentClass)
	}
	switch normalizedProfile := worker.NormalizeCapabilityProfile(strings.TrimSpace(agent.CapabilityProfile)); {
	case strings.TrimSpace(agent.CapabilityProfile) == "":
		agent.CapabilityProfile = capabilityProfileForAgentClass(agent.AgentClass)
	case normalizedProfile != strings.TrimSpace(agent.CapabilityProfile):
		// Preserve explicit custom profiles, but normalize known legacy aliases like "orchestrator".
		agent.CapabilityProfile = normalizedProfile
	}
	if strings.TrimSpace(agent.RuntimeKind) == "" {
		agent.RuntimeKind = defaultRuntimeKindForAgentClass(agent.AgentClass)
	}
	if agent.Skills == nil {
		agent.Skills = json.RawMessage("[]")
	}
	if agent.Tools == nil {
		agent.Tools = json.RawMessage("[]")
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
	if strings.TrimSpace(agent.TriggerMode) == "" || !slices.Contains(allowedTriggerModesForAgentClass(agent.AgentClass), agent.TriggerMode) {
		agent.TriggerMode = defaultTriggerModeForAgentClass(agent.AgentClass)
	}
	if agent.AgentClass != model.AgentClassProductPlanner {
		agent.PlanningNotes = nil
	}
	agent.SupportedModes = supportedModesForAgent(agent)
}

func supportedModesForAgent(agent *model.Agent) []string {
	if agent == nil {
		return []string{}
	}
	switch normalizeAgentClass(agent.AgentClass, agent.CapabilityProfile, agent.Role) {
	case model.AgentClassProductPlanner:
		modes := []string{model.InvocationModeAutonomous}
		if agentSupportsInteractive(agent) {
			modes = append(modes, model.InvocationModeInteractive)
		}
		return modes
	case model.AgentClassSupport:
		modes := []string{model.InvocationModeAutonomous}
		if agentSupportsInteractive(agent) {
			modes = append(modes, model.InvocationModeInteractive)
		}
		return modes
	default:
		return []string{model.InvocationModeAutonomous}
	}
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
		case model.AgentModelProviderAnthropic, model.AgentModelProviderOpenAI, model.AgentModelProviderOpenRouter:
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
	if err := validateAgentClass(agent.AgentClass); err != nil {
		return err
	}
	resolved := worker.ResolveAgentProfile(agent)
	if len(resolved.TargetTypes) == 0 {
		return fmt.Errorf("%s agents are not runnable", agent.AgentClass)
	}
	if !slices.Contains(resolved.TargetTypes, targetType) {
		return fmt.Errorf("%s agents can only be assigned to %s", agent.AgentClass, strings.Join(resolved.TargetTypes, ", "))
	}
	return nil
}

func normalizePlanningMethodology(value string) string {
	return model.NormalizePlanningMethodology(strings.TrimSpace(value))
}
