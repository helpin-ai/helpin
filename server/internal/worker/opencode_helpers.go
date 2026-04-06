package worker

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func renderOpenCodeStepStart(part map[string]any) string {
	title := firstNonEmptyText(
		lookupString(part, "title"),
		lookupString(part, "name"),
		lookupString(part, "tool"),
	)
	if title == "" {
		return "Step started"
	}
	return "Step started: " + title
}

func renderOpenCodeToolUse(part map[string]any) string {
	title := firstNonEmptyText(
		lookupString(part, "title"),
		lookupString(part, "name"),
		lookupString(part, "tool"),
	)
	if title == "" {
		if metadata, ok := part["metadata"].(map[string]any); ok {
			title = firstNonEmptyText(
				lookupString(metadata, "command"),
				lookupString(metadata, "description"),
				lookupString(metadata, "path"),
			)
		}
	}
	if title == "" {
		title = renderOpenCodeRawPart(part)
	}
	if title == "" {
		return "Tool used"
	}
	return "Tool: " + title
}

func renderOpenCodeStepFinish(part map[string]any, tokensUsed int) string {
	var details []string
	if stopReason := firstNonEmptyText(lookupString(part, "stopReason"), lookupString(part, "stop_reason")); stopReason != "" {
		details = append(details, "stop="+stopReason)
	}
	if tokensUsed > 0 {
		details = append(details, fmt.Sprintf("tokens=%d", tokensUsed))
	}
	if len(details) == 0 {
		return "Step finished"
	}
	return "Step finished (" + strings.Join(details, ", ") + ")"
}

func renderOpenCodeGenericEvent(eventType string, part map[string]any) string {
	label := strings.TrimSpace(eventType)
	if label == "" {
		label = "event"
	}
	raw := renderOpenCodeRawPart(part)
	if raw == "" {
		return "OpenCode " + label
	}
	return "OpenCode " + label + ": " + raw
}

func renderOpenCodeRawPart(part map[string]any) string {
	if len(part) == 0 {
		return ""
	}
	payload, err := json.Marshal(part)
	if err != nil {
		return ""
	}
	return truncate(string(payload), 1000)
}

func openCodeTokensFromPart(part map[string]any) int {
	tokens, _ := part["tokens"].(map[string]any)
	if len(tokens) == 0 {
		return 0
	}

	total := lookupInt(tokens, "input") + lookupInt(tokens, "output") + lookupInt(tokens, "reasoning")
	if cache, ok := tokens["cache"].(map[string]any); ok {
		total += lookupInt(cache, "read") + lookupInt(cache, "write")
	}
	return total
}

func lookupString(values map[string]any, key string) string {
	if len(values) == 0 {
		return ""
	}
	raw, ok := values[key]
	if !ok || raw == nil {
		return ""
	}
	switch typed := raw.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return strings.TrimSpace(fmt.Sprint(raw))
	}
}

func lookupInt(values map[string]any, key string) int {
	if len(values) == 0 {
		return 0
	}
	raw, ok := values[key]
	if !ok || raw == nil {
		return 0
	}
	switch typed := raw.(type) {
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	case int32:
		return int(typed)
	case json.Number:
		value, err := typed.Int64()
		if err == nil {
			return int(value)
		}
	case string:
		value, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return value
		}
	}
	return 0
}

func lookupInt64(values map[string]any, key string) int64 {
	if len(values) == 0 {
		return 0
	}
	raw, ok := values[key]
	if !ok || raw == nil {
		return 0
	}
	switch typed := raw.(type) {
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	case int:
		return int64(typed)
	case int64:
		return typed
	case int32:
		return int64(typed)
	case json.Number:
		value, err := typed.Int64()
		if err == nil {
			return value
		}
	case string:
		value, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err == nil {
			return value
		}
	}
	return 0
}

func lookupJSONString(values map[string]any, key string) string {
	if len(values) == 0 {
		return ""
	}
	raw, ok := values[key]
	if !ok || raw == nil {
		return ""
	}
	switch typed := raw.(type) {
	case string:
		return typed
	default:
		payload, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return string(payload)
	}
}

func buildOpenCodeConfigContent(execCtx *ExecutionContext, modelID, systemPrompt string, providerConfig map[string]any) (string, error) {
	agentName := openCodeAgentName(execCtx)
	agentConfig := map[string]any{
		"description": openCodeAgentDescription(execCtx),
		"mode":        "primary",
		"prompt":      systemPrompt,
	}
	if modelID != "" {
		agentConfig["model"] = modelID
	}
	if permissions := buildOpenCodePermissions(execCtx); len(permissions) > 0 {
		agentConfig["permission"] = permissions
	}

	config := map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"agent": map[string]any{
			agentName: agentConfig,
		},
	}
	if modelID != "" {
		config["model"] = modelID
		config["small_model"] = modelID
	}

	if len(providerConfig) > 0 {
		config["provider"] = providerConfig
	}

	payload, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func buildOpenCodeProviderConfig(agent *model.Agent, anthropicBaseURL, openRouterBaseURL string) map[string]any {
	provider := model.AgentModelProviderAnthropic
	modelName := ""
	if agent != nil {
		if configuredProvider := normalizeOpenCodeProvider(strings.TrimSpace(derefOpenCodeString(agent.Provider))); configuredProvider != "" {
			provider = configuredProvider
		}
		modelName = normalizeOpenCodeConfiguredModelName(provider, strings.TrimSpace(derefOpenCodeString(agent.Model)))
	}

	options := map[string]any{}
	switch provider {
	case model.AgentModelProviderAnthropic:
		if strings.TrimSpace(anthropicBaseURL) != "" {
			options["baseURL"] = strings.TrimSpace(anthropicBaseURL)
		}
	case model.AgentModelProviderOpenRouter:
		if strings.TrimSpace(openRouterBaseURL) != "" {
			options["baseURL"] = strings.TrimSpace(openRouterBaseURL)
		}
	}

	providerEntry := map[string]any{}
	if len(options) > 0 {
		providerEntry["options"] = options
	}
	if modelName != "" {
		providerEntry["models"] = map[string]any{
			modelName: map[string]any{},
		}
	}
	if len(providerEntry) == 0 {
		return nil
	}

	return map[string]any{
		provider: providerEntry,
	}
}

func openCodeAgentDescription(execCtx *ExecutionContext) string {
	resolved := resolvedProfileFor(execCtx)
	if execCtx == nil {
		return "Helpin runtime agent"
	}
	switch {
	case slices.Contains(resolved.TargetTypes, "support_conversation"):
		return "Helpin support agent for structured support triage."
	case slices.Contains(resolved.TargetTypes, "crm_deal"):
		return "Helpin operator agent for cross-app planning and CRM execution."
	case slices.Contains(resolved.TargetTypes, "epic"):
		return "Helpin planning agent for interactive product planning."
	case hasRepoMutationTools(resolved.Tools):
		return "Helpin build agent for story implementation runs."
	case slices.Contains(resolved.TargetTypes, "story"):
		return "Helpin review agent for story validation and quality checks."
	default:
		return "Helpin runtime agent"
	}
}

func buildOpenCodePermissions(execCtx *ExecutionContext) map[string]any {
	resolved := resolvedProfileFor(execCtx)
	if execCtx == nil {
		return nil
	}

	permissions := map[string]any{}
	if hasRepoMutationTools(resolved.Tools) {
		permissions["edit"] = "allow"
	} else {
		permissions["edit"] = "deny"
	}

	if len(resolved.Commands) == 0 {
		permissions["bash"] = "deny"
	} else if bashRules := buildOpenCodeBashPermissions(execCtx, resolved, execCtx.Config); len(bashRules) > 0 {
		permissions["bash"] = bashRules
	}

	return permissions
}

func resolvedProfileFor(execCtx *ExecutionContext) ResolvedProfile {
	if execCtx == nil {
		return ResolveAgentProfile(nil)
	}
	if len(execCtx.ResolvedProfile.Tools) > 0 || len(execCtx.ResolvedProfile.Commands) > 0 || len(execCtx.ResolvedProfile.TargetTypes) > 0 {
		return execCtx.ResolvedProfile
	}
	if execCtx.Agent != nil {
		return ResolveAgentProfile(execCtx.Agent)
	}
	return ResolveAgentProfile(nil)
}

func buildOpenCodeBashPermissions(execCtx *ExecutionContext, resolved ResolvedProfile, config *WorkflowConfig) map[string]string {
	allowed := allowedCommandsFor(resolved, config)
	rules := map[string]string{"*": "deny"}
	for _, command := range allowed {
		command = strings.TrimSpace(command)
		if command == "" || command == "git" {
			continue
		}
		rules[command] = "allow"
		rules[command+" *"] = "allow"
	}

	if slices.Contains(resolved.Commands, "git") {
		for _, pattern := range readOnlyGitPermissionPatterns() {
			rules[pattern] = "allow"
		}
	}

	return rules
}

func hasRepoMutationTools(tools []string) bool {
	for _, tool := range tools {
		switch tool {
		case "write_file", "create_branch", "commit_and_push", "open_pr":
			return true
		}
	}
	return false
}

func readOnlyGitPermissionPatterns() []string {
	return []string{
		"git status", "git status *",
		"git diff", "git diff *",
		"git log", "git log *",
		"git show", "git show *",
		"git rev-parse", "git rev-parse *",
		"git branch", "git branch *",
		"git ls-files", "git ls-files *",
		"git grep", "git grep *",
	}
}

func buildOpenCodeUserPrompt(execCtx *ExecutionContext, userPrompt string) string {
	parts := []string{strings.TrimSpace(userPrompt)}
	if execCtx != nil && execCtx.Task != nil && hasRepoMutationTools(resolvedProfileFor(execCtx).Tools) {
		parts = append(parts, "This is an implementation run, not an analysis-only pass. Make the code changes in the repository, run relevant validation when practical, and finish with a concise summary of the concrete files changed.")
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func openCodeAgentName(execCtx *ExecutionContext) string {
	resolved := resolvedProfileFor(execCtx)
	if execCtx == nil {
		return "teampulse"
	}
	switch {
	case slices.Contains(resolved.TargetTypes, "support_conversation"):
		return "teampulse-support"
	case slices.Contains(resolved.TargetTypes, "crm_deal"), slices.Contains(resolved.TargetTypes, "epic"):
		return "teampulse-operator"
	case hasRepoMutationTools(resolved.Tools):
		return "teampulse-engineer"
	case slices.Contains(resolved.TargetTypes, "story"):
		return "teampulse-reviewer"
	default:
		return "teampulse"
	}
}

func defaultOpenCodeModelForProvider(provider string) string {
	switch normalizeOpenCodeProvider(strings.TrimSpace(provider)) {
	case model.AgentModelProviderOpenAI:
		return "gpt-5-mini"
	case model.AgentModelProviderOpenRouter:
		return "openai/gpt-5-mini"
	default:
		return "claude-sonnet-4-6"
	}
}

func stripANSI(value string) string {
	return ansiEscapePattern.ReplaceAllString(value, "")
}

func sanitizeOpenCodeOutput(value string) string {
	return strings.TrimSpace(stripANSI(value))
}

func firstNonEmptyText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func derefOpenCodeString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func normalizeOpenCodeProvider(provider string) string {
	switch strings.TrimSpace(provider) {
	case "", model.AgentModelProviderAnthropic:
		return model.AgentModelProviderAnthropic
	case model.AgentModelProviderOpenAI:
		return model.AgentModelProviderOpenAI
	case model.AgentModelProviderOpenRouter, model.AgentModelProviderOpenRouterResponses:
		return model.AgentModelProviderOpenRouter
	default:
		return strings.TrimSpace(provider)
	}
}

func normalizeOpenCodeConfiguredModelName(provider, modelName string) string {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return ""
	}
	provider = normalizeOpenCodeProvider(provider)
	prefix := provider + "/"
	if strings.HasPrefix(modelName, prefix) {
		return strings.TrimPrefix(modelName, prefix)
	}
	return modelName
}

func appendIfMissingEnv(env []string, key, value string) []string {
	if strings.TrimSpace(value) == "" {
		return env
	}
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return env
		}
	}
	return append(env, prefix+value)
}

func upsertEnv(env []string, key, value string) []string {
	prefix := key + "="
	for idx, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			env[idx] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

type openCodeSupportDraftReply struct {
	Content           string  `json:"content"`
	IsInternal        bool    `json:"is_internal"`
	SenderDisplayName *string `json:"sender_display_name,omitempty"`
	ApprovalRequired  bool    `json:"approval_required"`
}

type openCodeSupportRunSummary struct {
	Status     *string                    `json:"status,omitempty"`
	DraftReply *openCodeSupportDraftReply `json:"draft_reply,omitempty"`
}

func extractSupportRunSummaryFromResponseText(responseText string) (*openCodeSupportRunSummary, error) {
	var summary openCodeSupportRunSummary
	if err := unmarshalLatestJSON(responseText, &summary); err != nil {
		return nil, fmt.Errorf("failed to parse support summary: %w", err)
	}
	if summary.DraftReply == nil || strings.TrimSpace(summary.DraftReply.Content) == "" {
		return nil, fmt.Errorf("support run did not return a draft reply")
	}
	if !summary.DraftReply.ApprovalRequired {
		summary.DraftReply.ApprovalRequired = true
	}
	return &summary, nil
}
