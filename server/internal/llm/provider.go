package llm

import "context"

// Provider defines a model-agnostic LLM interface.
type Provider interface {
	ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

// ChatRequest is a model-agnostic chat request.
type ChatRequest struct {
	SystemPrompt string
	Messages     []Message
	Temperature  float64
	MaxTokens    int
	JSONMode     bool
}

// Message represents a conversation message.
type Message struct {
	Role    string
	Content string
}

// ChatResponse is a model-agnostic chat response.
type ChatResponse struct {
	Content    string
	TokensUsed TokenUsage
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
}
