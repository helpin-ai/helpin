package worker

import (
	"encoding/json"
	"fmt"
	"strings"

	appmodel "github.com/helpin-ai/helpin/server/internal/model"
)

const (
	ToolRequestHumanInput    = "request_human_input"
	ToolRequestHumanApproval = "request_human_approval"

	QuestionTypeSingleSelect = "single_select"
)

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

type HumanApprovalRequest struct {
	Phase   string `json:"phase,omitempty"`
	Title   string `json:"title"`
	Summary string `json:"summary,omitempty"`
}

func toolRequestHumanInput(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var req HumanInputRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if err := validateHumanInputRequest(&req); err != nil {
		return "", err
	}

	payload, _ := json.MarshalIndent(map[string]any{
		"status":    "awaiting_input",
		"questions": req.Questions,
	}, "", "  ")
	return string(payload), nil
}

func toolRequestHumanApproval(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var req HumanApprovalRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if err := validateHumanApprovalRequest(&req); err != nil {
		return "", err
	}

	payload, _ := json.MarshalIndent(map[string]any{
		"status":  "awaiting_input",
		"phase":   req.Phase,
		"title":   req.Title,
		"summary": req.Summary,
	}, "", "  ")
	return string(payload), nil
}

func validateHumanInputRequest(req *HumanInputRequest) error {
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

func validateHumanApprovalRequest(req *HumanApprovalRequest) error {
	if req == nil {
		return fmt.Errorf("title is required")
	}
	req.Phase = strings.ToLower(strings.TrimSpace(req.Phase))
	if req.Phase == "" {
		req.Phase = "review"
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Summary = strings.TrimSpace(req.Summary)
	if req.Title == "" {
		return fmt.Errorf("title is required")
	}
	return nil
}

func IsHumanInteractionTool(name string) bool {
	switch strings.TrimSpace(name) {
	case ToolRequestHumanInput, ToolRequestHumanApproval:
		return true
	default:
		return false
	}
}

func ExtractLatestHumanApprovalRequest(toolInvocations []appmodel.ToolInvocation) *appmodel.ApprovalRequest {
	for i := len(toolInvocations) - 1; i >= 0; i-- {
		invocation := toolInvocations[i]
		if strings.TrimSpace(invocation.ToolName) != ToolRequestHumanApproval {
			continue
		}

		var req HumanApprovalRequest
		if err := json.Unmarshal(invocation.Input, &req); err != nil {
			continue
		}
		if err := validateHumanApprovalRequest(&req); err != nil {
			continue
		}

		return &appmodel.ApprovalRequest{
			Phase:   req.Phase,
			Title:   req.Title,
			Summary: req.Summary,
		}
	}

	return nil
}

func ExtractLatestHumanInputRequest(toolInvocations []appmodel.ToolInvocation) *HumanInputRequest {
	for i := len(toolInvocations) - 1; i >= 0; i-- {
		invocation := toolInvocations[i]
		if strings.TrimSpace(invocation.ToolName) != ToolRequestHumanInput {
			continue
		}

		var req HumanInputRequest
		if err := json.Unmarshal(invocation.Input, &req); err != nil {
			continue
		}
		if err := validateHumanInputRequest(&req); err != nil {
			continue
		}
		return &req
	}

	return nil
}
