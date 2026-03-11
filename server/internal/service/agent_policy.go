package service

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/worker"
)

func normalizeAgentClass(agentClass, capabilityProfile, role, agentKind string) string {
	if strings.EqualFold(strings.TrimSpace(agentKind), "human") {
		return model.AgentClassHuman
	}

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
		model.AgentClassSupport,
		model.AgentClassHuman:
		return raw
	default:
		return raw
	}
}

func capabilityProfileForAgentClass(agentClass string) string {
	switch normalizeAgentClass(agentClass, "", "", "") {
	case model.AgentClassProductPlanner:
		return model.AgentClassProductPlanner
	case model.AgentClassReviewer:
		return model.AgentClassReviewer
	case model.AgentClassSupport:
		return model.AgentClassSupport
	case model.AgentClassHuman:
		return model.AgentClassHuman
	default:
		return model.AgentClassEngineer
	}
}

func agentKindForAgentClass(agentClass string) string {
	if normalizeAgentClass(agentClass, "", "", "") == model.AgentClassHuman {
		return "human"
	}
	return "llm"
}

func defaultRoleForAgentClass(agentClass string) string {
	switch normalizeAgentClass(agentClass, "", "", "") {
	case model.AgentClassProductPlanner:
		return "Product Planner"
	case model.AgentClassReviewer:
		return "Reviewer"
	case model.AgentClassSupport:
		return "Support"
	case model.AgentClassHuman:
		return "Human"
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
	switch normalizeAgentClass(agentClass, "", "", "") {
	case model.AgentClassProductPlanner,
		model.AgentClassEngineer,
		model.AgentClassReviewer,
		model.AgentClassSupport,
		model.AgentClassHuman:
		return nil
	default:
		return fmt.Errorf("agent_class must be one of product_planner, engineer, reviewer, support, human")
	}
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
	switch normalizeAgentClass(agentClass, "", "", "") {
	case model.AgentClassEngineer, model.AgentClassReviewer:
		return []string{"manual", "auto_on_assignment", "auto_on_event"}
	case model.AgentClassProductPlanner, model.AgentClassSupport, model.AgentClassHuman:
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
		return fmt.Errorf("trigger_mode %q is not allowed for agent_class %q", triggerMode, normalizeAgentClass(agentClass, "", "", ""))
	}
	return nil
}

func normalizeAgentRecord(agent *model.Agent) {
	if agent == nil {
		return
	}

	agent.AgentClass = normalizeAgentClass(agent.AgentClass, agent.CapabilityProfile, agent.Role, agent.AgentKind)
	if strings.TrimSpace(agent.Role) == "" {
		agent.Role = defaultRoleForAgentClass(agent.AgentClass)
	}
	agent.CapabilityProfile = capabilityProfileForAgentClass(agent.AgentClass)
	if strings.TrimSpace(agent.RuntimeKind) == "" {
		agent.RuntimeKind = defaultRuntimeKindForAgentClass(agent.AgentClass)
	}
	if agent.Skills == nil {
		agent.Skills = json.RawMessage("[]")
	}
	if agent.Tools == nil {
		agent.Tools = json.RawMessage("[]")
	}

	if agent.AgentClass == model.AgentClassHuman {
		agent.AgentKind = "human"
		if strings.TrimSpace(agent.TriggerMode) == "" || !slices.Contains(allowedTriggerModesForAgentClass(agent.AgentClass), agent.TriggerMode) {
			agent.TriggerMode = defaultTriggerModeForAgentClass(agent.AgentClass)
		}
		agent.Model = nil
		agent.Provider = nil
		agent.SystemPrompt = nil
		agent.PlanningNotes = nil
		agent.Skills = json.RawMessage("[]")
		agent.Tools = json.RawMessage("[]")
		agent.MonthlyTokenBudget = nil
		return
	}

	if agent.AgentKind == "" || agent.AgentKind == "human" {
		agent.AgentKind = "llm"
	}
	agent.BackingUserID = nil
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
}

func validateModelProvider(provider string) error {
	switch normalizeModelProvider(provider) {
	case model.AgentModelProviderAnthropic, model.AgentModelProviderOpenAI, model.AgentModelProviderOpenRouter:
		return nil
	default:
		return fmt.Errorf("provider must be one of anthropic, openai, openrouter")
	}
}

func validateAgentTarget(agent *model.Agent, targetType string) error {
	normalizeAgentRecord(agent)
	if agent.AgentKind != "llm" {
		return fmt.Errorf("only LLM agents can be assigned to %s targets", targetType)
	}
	profile := worker.GetRuntimeProfile(agent.CapabilityProfile)
	if len(profile.AllowedTargetTypes) == 0 {
		return fmt.Errorf("%s agents are not runnable", agent.AgentClass)
	}
	if !slices.Contains(profile.AllowedTargetTypes, targetType) {
		return fmt.Errorf("%s agents can only be assigned to %s", agent.AgentClass, strings.Join(profile.AllowedTargetTypes, ", "))
	}
	return nil
}

func normalizePlanningMethodology(value string) string {
	return model.NormalizePlanningMethodology(strings.TrimSpace(value))
}
