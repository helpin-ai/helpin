package llm

import (
	"context"
	"fmt"

	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

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
	var messages []workerpkg.Message
	for _, m := range req.Messages {
		messages = append(messages, workerpkg.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	apiReq := workerpkg.CreateMessageRequest{
		System:    req.SystemPrompt,
		Messages:  messages,
		MaxTokens: maxTokens,
	}

	resp, err := p.client.CreateMessage(ctx, apiReq)
	if err != nil {
		return nil, fmt.Errorf("claude completion: %w", err)
	}

	var content string
	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	return &ChatResponse{
		Content: content,
		TokensUsed: TokenUsage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
		},
	}, nil
}
