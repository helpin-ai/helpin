package service

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type agentDraftLLM interface {
	ChatCompletion(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}

func (s *AgentService) SetAgentDraftLLM(client agentDraftLLM) *AgentService {
	s.agentDraftLLM = client
	return s
}

func (s *AgentService) DraftCustomAgent(ctx context.Context, workspaceID string, req model.CustomAgentDraftRequest) (*model.CustomAgentDraftResponse, error) {
	skillCatalog, err := s.ListSkillCatalog(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	toolCatalog, err := s.ListToolCatalogForWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return s.DraftCustomAgentWithCatalog(ctx, workspaceID, req, toolCatalog.Tools, skillCatalog.Skills)
}

func (s *AgentService) DraftCustomAgentWithCatalog(
	ctx context.Context,
	workspaceID string,
	req model.CustomAgentDraftRequest,
	tools []model.ToolCatalogEntry,
	skills []model.SkillCatalogEntry,
) (*model.CustomAgentDraftResponse, error) {
	description := strings.TrimSpace(req.Description)
	if len(description) < 10 {
		return nil, fmt.Errorf("description must be at least 10 characters")
	}
	if s.agentDraftLLM == nil {
		return nil, fmt.Errorf("agent draft LLM is not configured")
	}

	resp, err := completeAI(ctx, s.agentDraftLLM, AICompletionRequest{
		WorkspaceID:    workspaceID,
		FeatureKey:     BillingFeatureCustomAgentDraft,
		IdempotencyKey: aiUsageIdempotencyKey(workspaceID, BillingFeatureCustomAgentDraft, "draft", aiUsageStableHash(description)),
		Metadata: map[string]interface{}{
			"action": "custom_agent_draft",
		},
		Chat: llm.ChatRequest{
			SystemPrompt: customAgentDraftSystemPrompt(tools, skills),
			Messages: []llm.Message{{
				Role:    "user",
				Content: description,
			}},
			Temperature: 0.2,
			MaxTokens:   1600,
			JSONMode:    true,
			JSONSchema:  customAgentDraftJSONSchema(),
		},
	})
	if err != nil {
		return nil, err
	}

	var parsed model.CustomAgentDraftLLMResponse
	if err := llm.UnmarshalResponse(resp.Content, &parsed); err != nil {
		return nil, fmt.Errorf("parse agent draft response: %w", err)
	}
	draft, warnings := validateCustomAgentDraft(parsed.Draft, tools, skills)
	reasons := filterCustomAgentDraftReasons(parsed.Reasons, draft)
	return &model.CustomAgentDraftResponse{
		Draft:    draft,
		Reasons:  reasons,
		Warnings: warnings,
	}, nil
}

func validateCustomAgentDraft(
	raw model.CustomAgentDraft,
	tools []model.ToolCatalogEntry,
	skills []model.SkillCatalogEntry,
) (model.CustomAgentDraft, []string) {
	warnings := []string{}
	toolSet := make(map[string]bool, len(tools))
	for _, tool := range tools {
		if name := strings.TrimSpace(tool.Name); name != "" {
			toolSet[name] = true
		}
	}
	skillByKey := make(map[string]model.SkillCatalogEntry, len(skills))
	for _, skill := range skills {
		if key := strings.TrimSpace(skill.Key); key != "" {
			skillByKey[key] = skill
		}
	}

	draft := model.CustomAgentDraft{
		Name:                  strings.TrimSpace(raw.Name),
		Role:                  strings.TrimSpace(raw.Role),
		SystemPrompt:          strings.TrimSpace(raw.SystemPrompt),
		AllowedTargets:        []string{},
		AllowedTools:          []string{},
		Skills:                model.AgentSkillRefs{},
		ApprovalMode:          normalizeDraftApprovalMode(raw.ApprovalMode),
		RuntimeKind:           normalizeDraftRuntimeKind(raw.RuntimeKind),
		Provider:              normalizeDraftProvider(raw.Provider),
		Model:                 strings.TrimSpace(raw.Model),
		DefaultInvocationMode: normalizeDraftInvocationMode(raw.DefaultInvocationMode),
		MaxConcurrentRuns:     raw.MaxConcurrentRuns,
	}
	if draft.Name == "" {
		draft.Name = "Custom Agent"
	}
	if draft.Role == "" {
		draft.Role = "Custom Agent"
	}
	if draft.SystemPrompt == "" {
		draft.SystemPrompt = "Help with the user's requested work. Ask for clarification when requirements are unclear, and summarize completed work clearly."
	}
	if draft.MaxConcurrentRuns <= 0 {
		draft.MaxConcurrentRuns = 1
	}
	if draft.Provider != strings.TrimSpace(raw.Provider) {
		draft.Model = ""
	}

	for _, target := range raw.AllowedTargets {
		target = strings.TrimSpace(target)
		if !isSupportedCustomAgentTarget(target) {
			if target != "" {
				warnings = append(warnings, fmt.Sprintf("Removed unsupported target %q.", target))
			}
			continue
		}
		if !slices.Contains(draft.AllowedTargets, target) {
			draft.AllowedTargets = append(draft.AllowedTargets, target)
		}
	}
	if len(draft.AllowedTargets) == 0 {
		draft.AllowedTargets = []string{"task"}
		warnings = append(warnings, "Defaulted work area to tasks.")
	}

	for _, toolName := range raw.AllowedTools {
		toolName = strings.TrimSpace(toolName)
		if toolName == "" {
			continue
		}
		if !toolSet[toolName] {
			warnings = append(warnings, fmt.Sprintf("Removed unknown tool %q.", toolName))
			continue
		}
		if !slices.Contains(draft.AllowedTools, toolName) {
			draft.AllowedTools = append(draft.AllowedTools, toolName)
		}
	}

	for _, ref := range raw.Skills.Normalize() {
		key := strings.TrimSpace(ref.Key)
		skill, ok := skillByKey[key]
		if !ok {
			if key != "" {
				warnings = append(warnings, fmt.Sprintf("Removed unknown skill %q.", key))
			}
			continue
		}
		normalizedRef := model.AgentSkillRef{Key: key}
		if skill.ID != nil {
			normalizedRef.SkillID = skill.ID
		}
		if strings.TrimSpace(skill.VersionKey) != "" {
			versionKey := strings.TrimSpace(skill.VersionKey)
			normalizedRef.VersionKey = &versionKey
		}
		draft.Skills = append(draft.Skills, normalizedRef)
		for _, requiredTool := range skill.RequiredTools {
			requiredTool = strings.TrimSpace(requiredTool)
			if requiredTool == "" || !toolSet[requiredTool] || slices.Contains(draft.AllowedTools, requiredTool) {
				continue
			}
			draft.AllowedTools = append(draft.AllowedTools, requiredTool)
			warnings = append(warnings, fmt.Sprintf("Added tool %q because it is required by skill %q.", requiredTool, key))
		}
	}

	if raw.ApprovalMode != draft.ApprovalMode && strings.TrimSpace(raw.ApprovalMode) != "" {
		warnings = append(warnings, fmt.Sprintf("Changed unsupported approval mode %q to %q.", raw.ApprovalMode, draft.ApprovalMode))
	}
	if raw.RuntimeKind != draft.RuntimeKind && strings.TrimSpace(raw.RuntimeKind) != "" {
		warnings = append(warnings, fmt.Sprintf("Changed unsupported runtime %q to %q.", raw.RuntimeKind, draft.RuntimeKind))
	}
	if raw.Provider != draft.Provider && strings.TrimSpace(raw.Provider) != "" {
		warnings = append(warnings, fmt.Sprintf("Changed unsupported provider %q to %q.", raw.Provider, draft.Provider))
	}
	if raw.DefaultInvocationMode != draft.DefaultInvocationMode && strings.TrimSpace(raw.DefaultInvocationMode) != "" {
		warnings = append(warnings, fmt.Sprintf("Changed unsupported run mode %q to %q.", raw.DefaultInvocationMode, draft.DefaultInvocationMode))
	}
	return draft, warnings
}

func normalizeDraftApprovalMode(value string) string {
	switch strings.TrimSpace(value) {
	case "never", "risk_based", "mutating_tools", "preset_default":
		return strings.TrimSpace(value)
	default:
		return "mutating_tools"
	}
}

func normalizeDraftRuntimeKind(value string) string {
	switch strings.TrimSpace(value) {
	case "opencode", "native_sdk", "codex":
		return strings.TrimSpace(value)
	default:
		return "codex"
	}
}

func normalizeDraftProvider(value string) string {
	switch strings.TrimSpace(value) {
	case "anthropic", "openai", "openrouter":
		return strings.TrimSpace(value)
	default:
		return "anthropic"
	}
}

func normalizeDraftInvocationMode(value string) string {
	switch strings.TrimSpace(value) {
	case "autonomous", "interactive":
		return strings.TrimSpace(value)
	default:
		return "interactive"
	}
}

func isSupportedCustomAgentTarget(target string) bool {
	switch target {
	case "task", "epic", "sprint", "objective", "repository", "workspace", "crm_deal", "document", "support_conversation":
		return true
	default:
		return false
	}
}

func filterCustomAgentDraftReasons(reasons []model.CustomAgentDraftReason, draft model.CustomAgentDraft) []model.CustomAgentDraftReason {
	allowedValues := map[string]bool{}
	for _, value := range draft.AllowedTargets {
		allowedValues[value] = true
	}
	for _, value := range draft.AllowedTools {
		allowedValues[value] = true
	}
	for _, ref := range draft.Skills {
		allowedValues[ref.Key] = true
	}
	allowedValues[draft.Name] = true
	allowedValues[draft.RuntimeKind] = true
	allowedValues[draft.Provider] = true
	allowedValues[draft.ApprovalMode] = true
	allowedValues[draft.DefaultInvocationMode] = true

	out := make([]model.CustomAgentDraftReason, 0, len(reasons))
	for _, reason := range reasons {
		reason.Field = strings.TrimSpace(reason.Field)
		reason.Value = strings.TrimSpace(reason.Value)
		reason.Reason = strings.TrimSpace(reason.Reason)
		if reason.Field == "" || reason.Reason == "" {
			continue
		}
		if reason.Value != "" && !allowedValues[reason.Value] {
			continue
		}
		out = append(out, reason)
	}
	return out
}

func customAgentDraftSystemPrompt(tools []model.ToolCatalogEntry, skills []model.SkillCatalogEntry) string {
	toolRows := make([]string, 0, len(tools))
	for _, tool := range tools {
		toolRows = append(toolRows, fmt.Sprintf("- %s: %s", tool.Name, tool.Description))
	}
	skillRows := make([]string, 0, len(skills))
	for _, skill := range skills {
		skillRows = append(skillRows, fmt.Sprintf("- %s: %s Required tools: %s", skill.Key, skill.Description, strings.Join(skill.RequiredTools, ", ")))
	}
	return fmt.Sprintf(`Create a draft for a persistent workspace agent.

Return JSON only. The user will review and edit the draft before anything is created.

Choose only from these target types:
- task
- epic
- sprint
- objective
- repository
- workspace
- crm_deal
- document
- support_conversation

Choose only these tools:
%s

Choose only these skills:
%s

Defaults:
- role: Custom Agent
- approval_mode: mutating_tools unless the user explicitly asks to approve before any work or to execute writes without approval
- runtime_kind: native_sdk
- provider: anthropic
- model: empty string unless the user explicitly names a model
- default_invocation_mode: interactive
- max_concurrent_runs: 1

Write concise but useful system_prompt instructions. Include reasons for important target, tool, skill, and approval choices.`, strings.Join(toolRows, "\n"), strings.Join(skillRows, "\n"))
}

func customAgentDraftJSONSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"draft", "reasons"},
		"properties": map[string]any{
			"draft": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required": []string{
					"name", "role", "system_prompt", "allowed_targets", "allowed_tools", "skills",
					"approval_mode", "runtime_kind", "provider", "model", "default_invocation_mode", "max_concurrent_runs",
				},
				"properties": map[string]any{
					"name":                    map[string]any{"type": "string"},
					"role":                    map[string]any{"type": "string"},
					"system_prompt":           map[string]any{"type": "string"},
					"allowed_targets":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"allowed_tools":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"skills":                  map[string]any{"type": "array", "items": map[string]any{"type": "object", "required": []string{"key"}, "additionalProperties": false, "properties": map[string]any{"key": map[string]any{"type": "string"}, "skill_id": map[string]any{"type": "string"}, "version_key": map[string]any{"type": "string"}}}},
					"approval_mode":           map[string]any{"type": "string"},
					"runtime_kind":            map[string]any{"type": "string"},
					"provider":                map[string]any{"type": "string"},
					"model":                   map[string]any{"type": "string"},
					"default_invocation_mode": map[string]any{"type": "string"},
					"max_concurrent_runs":     map[string]any{"type": "integer"},
				},
			},
			"reasons": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"field", "value", "reason"},
					"properties": map[string]any{
						"field":  map[string]any{"type": "string"},
						"value":  map[string]any{"type": "string"},
						"reason": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
}

func mustJSONForDraft(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(payload)
}
