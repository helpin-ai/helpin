package llm

import (
	"context"
	"fmt"
	"strings"

	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

const claudeJSONToolName = "emit_json_response"

// ClaudeProvider wraps the existing Claude API client.
type ClaudeProvider struct {
	client *workerpkg.ClaudeClient
}

// NewClaudeProvider creates a Claude LLM provider.
func NewClaudeProvider(apiKey string) *ClaudeProvider {
	if apiKey == "" {
		return nil
	}
	return &ClaudeProvider{
		client: workerpkg.NewClaudeClient(apiKey),
	}
}

func (p *ClaudeProvider) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	apiReq := buildClaudeMessageRequest(req)

	resp, err := p.client.CreateMessage(ctx, apiReq)
	if err != nil {
		return nil, fmt.Errorf("claude completion: %w", err)
	}

	content := extractClaudeResponseContent(resp, req.JSONMode)

	return &ChatResponse{
		Content: content,
		TokensUsed: TokenUsage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
		},
	}, nil
}

func buildClaudeMessageRequest(req ChatRequest) workerpkg.CreateMessageRequest {
	messages := make([]workerpkg.Message, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, workerpkg.Message{Role: m.Role, Content: buildClaudeMessageContent(m)})
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	apiReq := workerpkg.CreateMessageRequest{
		Model:     req.Model,
		System:    req.SystemPrompt,
		Messages:  messages,
		MaxTokens: maxTokens,
	}
	if req.JSONMode {
		apiReq.Tools = []workerpkg.ToolDefinition{
			{
				Name:        claudeJSONToolName,
				Description: "Return the final response as a single JSON object that matches the schema requested in the prompt. Do not emit free-form text outside the tool input.",
				InputSchema: map[string]any{
					"type":                 "object",
					"additionalProperties": true,
				},
			},
		}
		apiReq.ToolChoice = &workerpkg.ToolChoice{
			Type: "tool",
			Name: claudeJSONToolName,
		}
	}

	return apiReq
}

func buildClaudeMessageContent(message Message) any {
	if len(message.ContentParts) == 0 {
		return message.Content
	}

	blocks := make([]workerpkg.ContentBlock, 0, len(message.ContentParts))
	for _, part := range message.ContentParts {
		switch part.Type {
		case "image_url":
			if part.ImageURL == nil || strings.TrimSpace(part.ImageURL.URL) == "" {
				continue
			}
			blocks = append(blocks, workerpkg.ContentBlock{
				Type: "image",
				Source: map[string]any{
					"type": "url",
					"url":  strings.TrimSpace(part.ImageURL.URL),
				},
			})
		default:
			text := strings.TrimSpace(part.Text)
			if text == "" {
				continue
			}
			blocks = append(blocks, workerpkg.ContentBlock{
				Type: "text",
				Text: text,
			})
		}
	}

	if len(blocks) == 0 {
		return message.Content
	}
	return blocks
}

func extractClaudeResponseContent(resp *workerpkg.CreateMessageResponse, jsonMode bool) string {
	if resp == nil {
		return ""
	}

	if jsonMode {
		for _, block := range resp.Content {
			if block.Type != "tool_use" || block.Name != claudeJSONToolName || len(block.Input) == 0 {
				continue
			}
			return strings.TrimSpace(string(block.Input))
		}
	}

	var content strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" {
			content.WriteString(block.Text)
		}
	}

	return content.String()
}
