package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	einoclaude "github.com/cloudwego/eino-ext/components/model/claude"
	einomodel "github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"

	"github.com/helpin-ai/helpin/server/internal/llm"
)

// supportTaskDraftLLM generates a structured PM task draft from a support
// conversation. Implementations are expected to return fully-populated drafts
// or an error — never a draft with empty required fields — so callers can
// trust the response without silently substituting deterministic fallbacks.
type supportTaskDraftLLM interface {
	GenerateTaskDraft(ctx context.Context, req supportTaskDraftRequest) (*supportConversationTaskDraft, error)
}

// supportTaskDraftRequest carries the fully prepared prompt inputs that the
// service layer has already built (sanitized history + user message + system
// prompt + model selection).
type supportTaskDraftRequest struct {
	WorkspaceID    string
	ConversationID string
	SystemPrompt   string
	Messages       []llm.Message
	Model          string
}

// supportTaskDraftToolName is the name of the forced tool Claude must call.
const supportTaskDraftToolName = "write_support_task_draft"

// einoSupportTaskDraftLLM forces Claude to emit schema-validated task draft
// fields via a tool call, which eliminates the class of silent failures where
// Claude returns JSON with field names that don't match our expected schema.
type einoSupportTaskDraftLLM struct {
	apiKey       string
	defaultModel string
	maxTokens    int
}

// NewEinoSupportTaskDraftLLM builds an Eino-backed task draft generator. The
// defaultModel is used when the caller does not specify a model on the
// request (typically "claude-sonnet-4-6").
func NewEinoSupportTaskDraftLLM(apiKey, defaultModel string) *einoSupportTaskDraftLLM {
	return &einoSupportTaskDraftLLM{
		apiKey:       strings.TrimSpace(apiKey),
		defaultModel: strings.TrimSpace(defaultModel),
		maxTokens:    4096,
	}
}

func (e *einoSupportTaskDraftLLM) GenerateTaskDraft(ctx context.Context, req supportTaskDraftRequest) (*supportConversationTaskDraft, error) {
	if e == nil || e.apiKey == "" {
		return nil, fmt.Errorf("eino support task draft LLM is not configured")
	}

	modelName := strings.TrimSpace(req.Model)
	if modelName == "" {
		modelName = e.defaultModel
	}

	chat, err := einoclaude.NewChatModel(ctx, &einoclaude.Config{
		APIKey:    e.apiKey,
		Model:     modelName,
		MaxTokens: e.maxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("init eino claude chat model: %w", err)
	}

	withTool, err := chat.WithTools([]*einoschema.ToolInfo{supportTaskDraftToolInfo()})
	if err != nil {
		return nil, fmt.Errorf("attach task draft tool: %w", err)
	}

	messages := make([]*einoschema.Message, 0, len(req.Messages)+1)
	if sys := strings.TrimSpace(req.SystemPrompt); sys != "" {
		messages = append(messages, einoschema.SystemMessage(sys))
	}
	for _, m := range req.Messages {
		messages = append(messages, toEinoMessage(m))
	}

	out, err := withTool.Generate(
		ctx,
		messages,
		einomodel.WithToolChoice(einoschema.ToolChoiceForced, supportTaskDraftToolName),
	)
	if err != nil {
		return nil, fmt.Errorf("eino task draft generate: %w", err)
	}

	var (
		args   string
		found  bool
		callID string
	)
	for _, tc := range out.ToolCalls {
		if tc.Function.Name == supportTaskDraftToolName {
			args = tc.Function.Arguments
			callID = tc.ID
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("eino task draft: model did not call %s (calls=%d)", supportTaskDraftToolName, len(out.ToolCalls))
	}
	if strings.TrimSpace(args) == "" {
		return nil, fmt.Errorf("eino task draft: %s call %s produced empty arguments", supportTaskDraftToolName, callID)
	}

	var parsed struct {
		Title               string `json:"title"`
		Summary             string `json:"summary"`
		DescriptionMarkdown string `json:"description_markdown"`
		TaskType            string `json:"task_type"`
		Priority            string `json:"priority"`
	}
	if err := json.Unmarshal([]byte(args), &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal task draft tool args: %w", err)
	}

	draft := &supportConversationTaskDraft{
		Title:       strings.TrimSpace(parsed.Title),
		Summary:     strings.TrimSpace(parsed.Summary),
		Description: strings.TrimSpace(parsed.DescriptionMarkdown),
		TaskType:    strings.TrimSpace(parsed.TaskType),
		Priority:    strings.TrimSpace(parsed.Priority),
	}
	if draft.Title == "" || draft.Description == "" {
		return nil, fmt.Errorf("eino task draft: tool call returned empty title or description (title_len=%d desc_len=%d)", len(draft.Title), len(draft.Description))
	}
	return draft, nil
}

// supportTaskDraftToolInfo is the schema Claude is forced to fill. Enum
// values constrain task_type and priority so they never arrive as free-text.
func supportTaskDraftToolInfo() *einoschema.ToolInfo {
	return &einoschema.ToolInfo{
		Name: supportTaskDraftToolName,
		Desc: "Record the internal PM task draft distilled from the support conversation. Call this tool exactly once with every field populated.",
		ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
			"title": {
				Type:     einoschema.String,
				Desc:     "Concise, issue-oriented PM task title under 90 characters. Do not paste raw log prefixes like 'Error:' or '[ApplicationError]'.",
				Required: true,
			},
			"summary": {
				Type:     einoschema.String,
				Desc:     "One or two sentences stating the underlying issue in plain language.",
				Required: true,
			},
			"description_markdown": {
				Type:     einoschema.String,
				Desc:     "Internal markdown task description. Include a '## Problem' section and, when you can infer them from the conversation, '## Impact' and '## Requested Outcome' sections. Never use customer pleasantries or sign-offs as Impact. Omit any section you cannot populate with real information.",
				Required: true,
			},
			"task_type": {
				Type:     einoschema.String,
				Desc:     "PM task type classification.",
				Enum:     []string{"feature", "bug", "chore"},
				Required: true,
			},
			"priority": {
				Type:     einoschema.String,
				Desc:     "PM task priority.",
				Enum:     []string{"none", "low", "medium", "high", "urgent"},
				Required: true,
			},
		}),
	}
}

func toEinoMessage(m llm.Message) *einoschema.Message {
	switch strings.ToLower(strings.TrimSpace(m.Role)) {
	case "system":
		return einoschema.SystemMessage(m.Content)
	case "assistant":
		return einoschema.AssistantMessage(m.Content, nil)
	default:
		return einoschema.UserMessage(m.Content)
	}
}
