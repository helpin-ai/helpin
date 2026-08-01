package worker

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	appmodel "github.com/helpin-ai/helpin/server/internal/model"
)

const (
	ToolRequestUserInput        = "request_user_input"
	ToolRequestApproval         = "request_approval"
	ToolRequestReviewCheckpoint = "request_review_checkpoint"
	ToolRequestHumanInput       = "request_human_input"
	ToolRequestHumanApproval    = "request_human_approval"

	QuestionTypeSingleSelect = "single_select"
)

var interactionOptionSlugSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

type UserInputOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type UserInputQuestion struct {
	ID       string            `json:"id"`
	Header   string            `json:"header,omitempty"`
	Question string            `json:"question"`
	IsOther  bool              `json:"isOther,omitempty"`
	IsSecret bool              `json:"isSecret,omitempty"`
	Options  []UserInputOption `json:"options,omitempty"`
}

type UserInputRequest struct {
	Questions []UserInputQuestion `json:"questions"`
}

type ApprovalRequest = appmodel.ApprovalRequest
type ReviewCheckpointRequest = appmodel.ReviewCheckpointRequest

// HumanInput* types remain as a temporary decode alias for request_human_input.
type HumanInputOption struct {
	Value    string `json:"value"`
	Label    string `json:"label"`
	Freetext bool   `json:"freetext,omitempty"`
}

type HumanInputQuestion struct {
	ID      string             `json:"id"`
	Type    string             `json:"type,omitempty"`
	Text    string             `json:"text"`
	Options []HumanInputOption `json:"options"`
}

type HumanInputRequest struct {
	Questions []HumanInputQuestion `json:"questions"`
}

// HumanApprovalRequest remains as a temporary decode alias for request_human_approval.
type HumanApprovalRequest = ApprovalRequest

func CanonicalToolName(name string) string {
	trimmed := strings.TrimSpace(name)
	// Codex app-server events use <server>/<tool>, while model-facing MCP
	// names use mcp__<server>__<tool>. Product contracts use logical tool names.
	if strings.HasPrefix(trimmed, "mcp__") {
		qualified := strings.TrimPrefix(trimmed, "mcp__")
		if separator := strings.Index(qualified, "__"); separator >= 0 && separator+2 < len(qualified) {
			trimmed = qualified[separator+2:]
		}
	}
	if separator := strings.Index(trimmed, "/"); separator >= 0 && separator+1 < len(trimmed) {
		trimmed = trimmed[separator+1:]
	}
	trimmed = strings.TrimSpace(trimmed)
	switch trimmed {
	case ToolRequestHumanInput:
		return ToolRequestUserInput
	case ToolRequestHumanApproval:
		return ToolRequestApproval
	case "add_story_comment":
		return "add_task_comment"
	case "list_story_checklist":
		return "list_task_checklist"
	case "update_story_state":
		return "update_task_state"
	case "run_semgrep":
		return ToolScanSemgrep
	case "run_trivy":
		return ToolScanTrivy
	case "run_gitleaks":
		return ToolScanGitleaks
	default:
		return trimmed
	}
}

func NormalizeToolNames(names []string) []string {
	if len(names) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		canonical := CanonicalToolName(name)
		if canonical == "" {
			continue
		}
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		normalized = append(normalized, canonical)
	}
	return normalized
}

func toolRequestUserInput(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var req UserInputRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if err := validateUserInputRequest(&req); err != nil {
		return "", err
	}

	return toCompactJSONString(map[string]any{
		"status":       appmodel.AgentRunStatusPaused,
		"pause_reason": appmodel.AgentRunPauseReasonHumanInput,
		"questions":    req.Questions,
	}), nil
}

func toolRequestHumanInput(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var legacy HumanInputRequest
	if err := json.Unmarshal(input, &legacy); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if err := validateLegacyHumanInputRequest(&legacy); err != nil {
		return "", err
	}
	normalized, err := json.Marshal(convertLegacyHumanInputRequest(&legacy))
	if err != nil {
		return "", fmt.Errorf("marshal input: %w", err)
	}
	return toolRequestUserInput(ctx, normalized)
}

func toolRequestApproval(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var req ApprovalRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if err := validateApprovalRequest(&req); err != nil {
		return "", err
	}

	return toCompactJSONString(map[string]any{
		"status":       appmodel.AgentRunStatusPaused,
		"pause_reason": appmodel.AgentRunPauseReasonHumanApproval,
		"phase":        req.Phase,
		"title":        req.Title,
		"summary":      req.Summary,
	}), nil
}

func toolRequestReviewCheckpoint(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var req ReviewCheckpointRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if err := validateReviewCheckpointRequest(&req); err != nil {
		return "", err
	}

	return toCompactJSONString(map[string]any{
		"status":       appmodel.AgentRunStatusPaused,
		"pause_reason": appmodel.AgentRunPauseReasonHumanApproval,
		"phase":        req.Phase,
		"title":        req.Title,
		"summary":      req.Summary,
	}), nil
}

func toolRequestHumanApproval(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var legacy HumanApprovalRequest
	if err := json.Unmarshal(input, &legacy); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if err := validateApprovalRequest(&legacy); err != nil {
		return "", err
	}
	normalized, err := json.Marshal(legacy)
	if err != nil {
		return "", fmt.Errorf("marshal input: %w", err)
	}
	return toolRequestApproval(ctx, normalized)
}

func validateUserInputRequest(req *UserInputRequest) error {
	if req == nil || len(req.Questions) == 0 {
		return fmt.Errorf("questions are required")
	}
	if len(req.Questions) > 10 {
		return fmt.Errorf("questions cannot exceed 10 per request")
	}

	for i := range req.Questions {
		question := &req.Questions[i]
		question.ID = strings.TrimSpace(question.ID)
		question.Header = strings.TrimSpace(question.Header)
		question.Question = strings.TrimSpace(question.Question)
		if question.ID == "" {
			return fmt.Errorf("question id is required")
		}
		if question.Question == "" {
			return fmt.Errorf("question %q question is required", question.ID)
		}
		for j := range question.Options {
			option := &question.Options[j]
			option.Label = strings.TrimSpace(option.Label)
			option.Description = strings.TrimSpace(option.Description)
			if option.Label == "" {
				return fmt.Errorf("question %q option %d label is required", question.ID, j+1)
			}
		}
	}
	return nil
}

func validateLegacyHumanInputRequest(req *HumanInputRequest) error {
	if req == nil {
		return fmt.Errorf("questions are required")
	}
	if len(req.Questions) == 0 {
		return fmt.Errorf("questions are required")
	}
	if len(req.Questions) > 10 {
		return fmt.Errorf("questions cannot exceed 10 per request")
	}

	for i := range req.Questions {
		question := &req.Questions[i]
		question.ID = strings.TrimSpace(question.ID)
		question.Text = strings.TrimSpace(question.Text)
		question.Type = strings.TrimSpace(question.Type)
		if question.Type == "" {
			question.Type = QuestionTypeSingleSelect
		}
		if question.Type != QuestionTypeSingleSelect {
			return fmt.Errorf("question %q must use type %q", question.ID, QuestionTypeSingleSelect)
		}
		if question.ID == "" {
			return fmt.Errorf("question id is required")
		}
		if question.Text == "" {
			return fmt.Errorf("question %q text is required", question.ID)
		}
		if len(question.Options) == 0 {
			return fmt.Errorf("question %q options are required", question.ID)
		}
		for j := range question.Options {
			option := &question.Options[j]
			option.Value = strings.TrimSpace(option.Value)
			option.Label = strings.TrimSpace(option.Label)
			if option.Value == "" {
				return fmt.Errorf("question %q option %d value is required", question.ID, j+1)
			}
			if option.Label == "" {
				return fmt.Errorf("question %q option %d label is required", question.ID, j+1)
			}
		}
	}

	return nil
}

func validateApprovalRequest(req *ApprovalRequest) error {
	if req == nil {
		return fmt.Errorf("title is required")
	}
	normalizeApprovalRequestFields(&req.Phase, &req.PreviewPanelKey, &req.Title, &req.Summary)
	if req.Title == "" {
		return fmt.Errorf("title is required")
	}
	return nil
}

func validateReviewCheckpointRequest(req *ReviewCheckpointRequest) error {
	if req == nil {
		return fmt.Errorf("title is required")
	}
	normalizeApprovalRequestFields(&req.Phase, &req.PreviewPanelKey, &req.Title, &req.Summary)
	if req.Title == "" {
		return fmt.Errorf("title is required")
	}
	if req.Phase == "" {
		req.Phase = "review"
	}
	req.OverallCorrectness = strings.TrimSpace(req.OverallCorrectness)
	req.OverallExplanation = strings.TrimSpace(req.OverallExplanation)
	for i := range req.Findings {
		finding := &req.Findings[i]
		finding.ID = strings.TrimSpace(finding.ID)
		if finding.ID == "" {
			finding.ID = fmt.Sprintf("finding_%d", i+1)
		}
		finding.Title = strings.TrimSpace(finding.Title)
		finding.Body = strings.TrimSpace(finding.Body)
		finding.Priority = strings.ToUpper(strings.TrimSpace(finding.Priority))
		finding.CodeLocation = strings.TrimSpace(finding.CodeLocation)
		if finding.Title == "" {
			return fmt.Errorf("finding %d title is required", i+1)
		}
		if finding.Body == "" {
			return fmt.Errorf("finding %d body is required", i+1)
		}
	}
	return nil
}

func normalizeApprovalRequestFields(phase, previewPanelKey, title, summary *string) {
	if phase != nil {
		*phase = strings.ToLower(strings.TrimSpace(*phase))
	}
	if previewPanelKey != nil {
		*previewPanelKey = strings.ToLower(strings.TrimSpace(*previewPanelKey))
	}
	if title != nil {
		*title = strings.TrimSpace(*title)
	}
	if summary != nil {
		*summary = strings.TrimSpace(*summary)
	}
}

func convertLegacyHumanInputRequest(req *HumanInputRequest) UserInputRequest {
	out := UserInputRequest{
		Questions: make([]UserInputQuestion, 0, len(req.Questions)),
	}
	for _, question := range req.Questions {
		item := UserInputQuestion{
			ID:       strings.TrimSpace(question.ID),
			Question: strings.TrimSpace(question.Text),
			Options:  make([]UserInputOption, 0, len(question.Options)),
		}
		for _, option := range question.Options {
			if option.Freetext {
				item.IsOther = true
				continue
			}
			item.Options = append(item.Options, UserInputOption{
				Label: strings.TrimSpace(option.Label),
			})
		}
		out.Questions = append(out.Questions, item)
	}
	return out
}

func IsHumanInteractionTool(name string) bool {
	switch CanonicalToolName(name) {
	case ToolRequestUserInput, ToolRequestApproval, ToolRequestReviewCheckpoint, ToolRequestHumanInput, ToolRequestHumanApproval:
		return true
	default:
		return false
	}
}

func ExtractLatestApprovalRequest(toolInvocations []appmodel.ToolInvocation) *appmodel.ApprovalRequest {
	for i := len(toolInvocations) - 1; i >= 0; i-- {
		invocation := toolInvocations[i]
		switch CanonicalToolName(invocation.ToolName) {
		case ToolRequestApproval, ToolRequestHumanApproval:
		default:
			continue
		}

		var req ApprovalRequest
		if err := json.Unmarshal(invocation.Input, &req); err != nil {
			continue
		}
		if err := validateApprovalRequest(&req); err != nil {
			continue
		}

		return &appmodel.ApprovalRequest{
			Phase:           req.Phase,
			PreviewPanelKey: req.PreviewPanelKey,
			Title:           req.Title,
			Summary:         req.Summary,
		}
	}

	return nil
}

func ExtractLatestReviewCheckpointRequest(toolInvocations []appmodel.ToolInvocation) *appmodel.ReviewCheckpointRequest {
	for i := len(toolInvocations) - 1; i >= 0; i-- {
		invocation := toolInvocations[i]
		if CanonicalToolName(invocation.ToolName) != ToolRequestReviewCheckpoint {
			continue
		}

		var req ReviewCheckpointRequest
		if err := json.Unmarshal(invocation.Input, &req); err != nil {
			continue
		}
		if err := validateReviewCheckpointRequest(&req); err != nil {
			continue
		}

		return &appmodel.ReviewCheckpointRequest{
			Phase:                  req.Phase,
			PreviewPanelKey:        req.PreviewPanelKey,
			Title:                  req.Title,
			Summary:                req.Summary,
			Findings:               req.Findings,
			OverallCorrectness:     req.OverallCorrectness,
			OverallExplanation:     req.OverallExplanation,
			OverallConfidenceScore: req.OverallConfidenceScore,
		}
	}

	return nil
}

func ExtractLatestHumanInputRequest(toolInvocations []appmodel.ToolInvocation) *UserInputRequest {
	for i := len(toolInvocations) - 1; i >= 0; i-- {
		invocation := toolInvocations[i]
		rawName := strings.TrimSpace(invocation.ToolName)
		canonicalName := CanonicalToolName(rawName)
		if rawName == ToolRequestHumanInput {
			var legacy HumanInputRequest
			if err := json.Unmarshal(invocation.Input, &legacy); err != nil {
				continue
			}
			if err := validateLegacyHumanInputRequest(&legacy); err != nil {
				continue
			}
			req := convertLegacyHumanInputRequest(&legacy)
			return &req
		}
		switch canonicalName {
		case ToolRequestUserInput:
			var req UserInputRequest
			if err := json.Unmarshal(invocation.Input, &req); err != nil {
				continue
			}
			if err := validateUserInputRequest(&req); err != nil {
				continue
			}
			return &req
		}
	}

	return nil
}

func HumanInputArtifactFromUserInputRequest(req *UserInputRequest) appmodel.HumanInputArtifact {
	if req == nil {
		return appmodel.HumanInputArtifact{}
	}

	out := appmodel.HumanInputArtifact{
		Questions: make([]appmodel.HumanInputArtifactQuestion, 0, len(req.Questions)),
	}
	for _, question := range req.Questions {
		item := appmodel.HumanInputArtifactQuestion{
			ID:      strings.TrimSpace(question.ID),
			Type:    QuestionTypeSingleSelect,
			Text:    userInputQuestionPrompt(question),
			Options: make([]appmodel.HumanInputArtifactOption, 0, len(question.Options)+1),
		}

		usedValues := map[string]int{}
		for _, option := range question.Options {
			label := strings.TrimSpace(option.Label)
			if label == "" {
				continue
			}
			item.Options = append(item.Options, appmodel.HumanInputArtifactOption{
				Value: interactionOptionValue(label, usedValues),
				Label: label,
			})
		}
		if question.IsOther || len(item.Options) == 0 {
			item.Options = append(item.Options, appmodel.HumanInputArtifactOption{
				Value:    interactionOptionValue("other", usedValues),
				Label:    "Other",
				Freetext: true,
			})
		}
		out.Questions = append(out.Questions, item)
	}

	return out
}

func UserInputSummary(req *UserInputRequest) string {
	if req == nil || len(req.Questions) == 0 {
		return ""
	}
	return userInputQuestionPrompt(req.Questions[0])
}

func userInputQuestionPrompt(question UserInputQuestion) string {
	header := strings.TrimSpace(question.Header)
	prompt := strings.TrimSpace(question.Question)
	switch {
	case header != "" && prompt != "" && !strings.EqualFold(header, prompt):
		return header + ": " + prompt
	case prompt != "":
		return prompt
	default:
		return firstNonEmptyText(header, strings.TrimSpace(question.ID), "Question")
	}
}

func interactionOptionValue(label string, used map[string]int) string {
	base := strings.ToLower(strings.TrimSpace(label))
	base = interactionOptionSlugSanitizer.ReplaceAllString(base, "_")
	base = strings.Trim(base, "_")
	if base == "" {
		base = "option"
	}
	if used == nil {
		return base
	}
	used[base]++
	if used[base] == 1 {
		return base
	}
	return fmt.Sprintf("%s_%d", base, used[base])
}
