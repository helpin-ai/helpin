package service

import (
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/worker"
)

func promptMatchesDefault(presetKey string, prompt *string) bool {
	normalizedPrompt := trimPtr(prompt)
	if normalizedPrompt == nil {
		return true
	}
	defaultPrompt := defaultSystemPromptForPreset(presetKey)
	if defaultPrompt == nil {
		return false
	}
	return strings.TrimSpace(*normalizedPrompt) == strings.TrimSpace(*defaultPrompt)
}

func legacyPromptIsManaged(presetKey string, prompt *string) bool {
	normalizedPrompt := trimPtr(prompt)
	if normalizedPrompt == nil {
		return true
	}
	if promptMatchesDefault(presetKey, normalizedPrompt) {
		return true
	}

	normalized := strings.TrimSpace(*normalizedPrompt)
	if normalized == "" {
		return true
	}

	switch normalizePresetKey(presetKey) {
	case model.AgentPresetEpicPlanner:
		for _, marker := range []string{
			"1. `prd_draft`",
			"2. `awaiting_prd_approval`",
			"3. `persist_prd`",
			"4. `task_plan`",
			"5. `awaiting_task_approval`",
			"6. `create_tasks`",
			"The runtime will tell you the current planner phase.",
			"Tool access is gated server-side by phase.",
			"<approval_request phase=\"prd\">",
			"<questions>",
		} {
			if strings.Contains(normalized, marker) {
				return true
			}
		}
	case model.AgentPresetTaskPlanner:
		for _, marker := range []string{
			"You are Task Planner for Helpin.",
			"`publish_preview`",
			"`request_human_input`",
			"`request_human_approval`",
			"formal approval action is taken through the UI",
			"The runtime will tell you the current planner phase.",
		} {
			if strings.Contains(normalized, marker) {
				return true
			}
		}
	case model.AgentPresetCodeBuilder:
		for _, marker := range []string{
			"You are Code Builder for Helpin.",
			"an AI coding agent. You write clean, correct code and follow existing project conventions.",
			"Use the provided tools to read, write, and search files.",
			"prepare delivery artifacts.",
		} {
			if strings.Contains(normalized, marker) {
				return true
			}
		}
	case model.AgentPresetReviewAgent:
		if strings.Contains(normalized, "You are Review Agent for Helpin.") {
			return true
		}
		// Older legacy prompts without interactive loop markers.
		if strings.Contains(normalized, "You are Review Agent.") &&
			(strings.Contains(normalized, "Inspect the relevant code and run targeted validation") ||
				strings.Contains(normalized, "Focus on correctness, regressions, missing tests")) {
			return true
		}
		for _, snippet := range []string{
			"You are Review Agent.",
			"`request_user_input`",
			"Treat review as an interactive loop, not a one-shot report.",
			"Do not finish immediately after posting findings unless the latest human reply clearly says the review is done",
			"If the human asks you to implement changes based on the review",
		} {
			if !strings.Contains(normalized, snippet) {
				return false
			}
		}
		return true
	case model.AgentPresetCRMOperator:
		return strings.Contains(normalized, "You are CRM Operator for Helpin.")
	case model.AgentPresetSupportAgent:
		return strings.Contains(normalized, "You are Support Agent for Helpin.")
	}

	return false
}

func defaultSystemPromptForPreset(presetKey string) *string {
	return worker.BuiltInPresetPrompt(normalizePresetKey(presetKey))
}

func mergeLegacyPlanningNotes(prompt, legacyPlanningNotes *string) *string {
	normalizedNotes := trimPtr(legacyPlanningNotes)
	if normalizedNotes == nil {
		return trimPtr(prompt)
	}
	if trimPtr(prompt) == nil {
		merged := strings.TrimSpace(*normalizedNotes)
		return &merged
	}
	merged := strings.TrimSpace(*trimPtr(prompt)) + "\n\n## Additional Instructions\n" + strings.TrimSpace(*legacyPlanningNotes)
	return &merged
}

func syncManagedSystemPromptForPreset(presetKey string, systemPrompt, legacyPlanningNotes *string, instructionTemplateVersion string) (*string, string) {
	normalizedPresetKey := normalizePresetKey(presetKey)
	currentVersion := strings.TrimSpace(worker.BuiltInPresetInstructionTemplateVersion(normalizedPresetKey))
	if currentVersion == "" {
		return trimPtr(systemPrompt), strings.TrimSpace(instructionTemplateVersion)
	}

	normalizedPrompt := trimPtr(systemPrompt)
	normalizedVersion := strings.TrimSpace(instructionTemplateVersion)
	// A non-empty template version historically means Helpin managed the prompt, but only if the row is still in managed storage mode.
	managedPrompt := normalizedPrompt == nil || normalizedVersion == currentVersion || (normalizedVersion != "" && normalizedPrompt == nil) || legacyPromptIsManaged(normalizedPresetKey, normalizedPrompt)
	if managedPrompt {
		if trimPtr(legacyPlanningNotes) != nil {
			return mergeLegacyPlanningNotes(defaultSystemPromptForPreset(normalizedPresetKey), legacyPlanningNotes), ""
		}
		return nil, currentVersion
	}

	if trimPtr(legacyPlanningNotes) != nil {
		return mergeLegacyPlanningNotes(normalizedPrompt, legacyPlanningNotes), ""
	}
	return normalizedPrompt, ""
}

func resolveEffectiveSystemPromptForPreset(presetKey string, systemPrompt *string) *string {
	normalizedPrompt := trimPtr(systemPrompt)
	if normalizedPrompt != nil {
		return normalizedPrompt
	}
	return defaultSystemPromptForPreset(presetKey)
}
