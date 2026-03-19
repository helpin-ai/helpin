package worker

import (
	"encoding/json"
	"fmt"
	"regexp"
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

func buildOpenCodeConfigContent(execCtx *ExecutionContext, modelID, systemPrompt string) (string, error) {
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

	payload, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func openCodeAgentDescription(execCtx *ExecutionContext) string {
	if execCtx == nil || execCtx.Agent == nil {
		return "Teampulse runtime agent"
	}
	switch execCtx.Agent.AgentClass {
	case model.AgentClassProductPlanner:
		return "Teampulse planner agent for OpenSpec-style product planning."
	case model.AgentClassEngineer:
		return "Teampulse engineer agent for story implementation runs."
	case model.AgentClassReviewer:
		return "Teampulse reviewer agent for code review and validation."
	case model.AgentClassSupport:
		return "Teampulse support agent for structured support triage."
	default:
		return "Teampulse runtime agent"
	}
}

func buildOpenCodePermissions(execCtx *ExecutionContext) map[string]any {
	if execCtx == nil || execCtx.Agent == nil {
		return nil
	}

	permissions := map[string]any{}
	switch execCtx.Agent.AgentClass {
	case model.AgentClassEngineer:
		permissions["edit"] = "allow"
	case model.AgentClassProductPlanner, model.AgentClassReviewer, model.AgentClassSupport:
		permissions["edit"] = "deny"
	}

	switch execCtx.Agent.AgentClass {
	case model.AgentClassSupport:
		permissions["bash"] = "deny"
	default:
		if bashRules := buildOpenCodeBashPermissions(execCtx, runtimeProfileFor(execCtx), execCtx.Config); len(bashRules) > 0 {
			permissions["bash"] = bashRules
		}
	}

	return permissions
}

func runtimeProfileFor(execCtx *ExecutionContext) model.RuntimeProfile {
	if execCtx == nil {
		return GetRuntimeProfile("")
	}
	if execCtx.RuntimeProfile.Name != "" {
		return execCtx.RuntimeProfile
	}
	if execCtx.Agent != nil {
		return GetRuntimeProfile(execCtx.Agent.CapabilityProfile)
	}
	return GetRuntimeProfile("")
}

func buildOpenCodeBashPermissions(execCtx *ExecutionContext, profile model.RuntimeProfile, config *WorkflowConfig) map[string]string {
	allowed := allowedCommandsFor(profile, config)
	rules := map[string]string{"*": "deny"}
	for _, command := range allowed {
		command = strings.TrimSpace(command)
		if command == "" || command == "git" {
			continue
		}
		rules[command] = "allow"
		rules[command+" *"] = "allow"
	}

	if execCtx != nil && execCtx.Agent != nil {
		switch execCtx.Agent.AgentClass {
		case model.AgentClassEngineer, model.AgentClassProductPlanner, model.AgentClassReviewer:
			for _, pattern := range readOnlyGitPermissionPatterns() {
				rules[pattern] = "allow"
			}
		}
	}

	return rules
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
	if execCtx != nil && execCtx.Agent != nil && execCtx.Agent.AgentClass == model.AgentClassEngineer && execCtx.Story != nil {
		parts = append(parts, "This is an implementation run, not an analysis-only pass. Make the code changes in the repository, run relevant validation when practical, and finish with a concise summary of the concrete files changed.")
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func openCodeAgentName(execCtx *ExecutionContext) string {
	if execCtx == nil || execCtx.Agent == nil {
		return "teampulse"
	}
	switch execCtx.Agent.AgentClass {
	case model.AgentClassProductPlanner:
		return "teampulse-planner"
	case model.AgentClassEngineer:
		return "teampulse-engineer"
	case model.AgentClassReviewer:
		return "teampulse-reviewer"
	case model.AgentClassSupport:
		return "teampulse-support"
	default:
		return "teampulse"
	}
}

func defaultOpenCodeModelForProvider(provider string) string {
	switch strings.TrimSpace(provider) {
	case model.AgentModelProviderOpenAI:
		return "gpt-5-mini"
	case model.AgentModelProviderOpenRouter:
		return "openai/gpt-5-mini"
	default:
		return "claude-sonnet-4-20250514"
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
