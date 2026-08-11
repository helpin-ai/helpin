package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
)

const claudeJSONToolName = "emit_json_response"

// ClaudeProvider wraps the existing Claude API client.
type ClaudeProvider struct {
	client *agentcontract.ClaudeClient
}

// NewClaudeProvider creates a Claude LLM provider.
func NewClaudeProvider(apiKey string) *ClaudeProvider {
	if apiKey == "" {
		return nil
	}
	return &ClaudeProvider{
		client: agentcontract.NewClaudeClient(apiKey),
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
		Content:    content,
		TokensUsed: tokenUsageFromClaude(resp.Usage),
	}, nil
}

func tokenUsageFromClaude(usage agentcontract.Usage) TokenUsage {
	return TokenUsage{
		InputTokens:       usage.InputTokens + usage.CacheCreationInputTokens + usage.CacheReadInputTokens,
		CachedInputTokens: usage.CacheReadInputTokens,
		CacheWriteTokens:  usage.CacheCreationInputTokens,
		OutputTokens:      usage.OutputTokens,
	}
}

func buildClaudeMessageRequest(req ChatRequest) agentcontract.CreateMessageRequest {
	messages := make([]agentcontract.Message, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, agentcontract.Message{Role: m.Role, Content: buildClaudeMessageContent(m)})
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	apiReq := agentcontract.CreateMessageRequest{
		Model:     req.Model,
		System:    req.SystemPrompt,
		Messages:  messages,
		MaxTokens: maxTokens,
	}
	if req.JSONMode {
		schema := req.JSONSchema
		if schema == nil {
			schema = defaultClaudeJSONSchema()
		}
		apiReq.Tools = []agentcontract.ToolDefinition{
			{
				Name:        claudeJSONToolName,
				Description: "Return the final response as a single JSON object that matches the schema requested in the prompt. Do not emit free-form text outside the tool input.",
				InputSchema: schema,
			},
		}
		apiReq.ToolChoice = &agentcontract.ToolChoice{
			Type: "tool",
			Name: claudeJSONToolName,
		}
	}

	return apiReq
}

func defaultClaudeJSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"content": map[string]any{
				"type": "string",
			},
			"can_answer": map[string]any{
				"type": "boolean",
			},
			"source_doc_ids": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "string",
				},
			},
			"confidence": map[string]any{
				"type": "number",
			},
		},
		"required":             []string{"content", "can_answer", "source_doc_ids", "confidence"},
		"additionalProperties": false,
	}
}

func buildClaudeMessageContent(message Message) any {
	if len(message.ContentParts) == 0 {
		return message.Content
	}

	blocks := make([]agentcontract.ContentBlock, 0, len(message.ContentParts))
	for _, part := range message.ContentParts {
		switch part.Type {
		case "image_url":
			if part.ImageURL == nil || strings.TrimSpace(part.ImageURL.URL) == "" {
				continue
			}
			blocks = append(blocks, agentcontract.ContentBlock{
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
			blocks = append(blocks, agentcontract.ContentBlock{
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

func extractClaudeResponseContent(resp *agentcontract.CreateMessageResponse, jsonMode bool) string {
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
