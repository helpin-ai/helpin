package llm

import (
	"context"
	"encoding/json"
	"errors"
)

// ErrInsufficientCredits indicates the upstream LLM provider rejected the
// request for billing reasons (HTTP 402 — insufficient credits or exhausted
// quota). Callers use errors.Is to degrade with a clear, actionable message
// instead of a generic failure.
var ErrInsufficientCredits = errors.New("llm: insufficient credits")

// Provider defines a model-agnostic LLM interface.
type Provider interface {
	ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

// EmbeddingProvider defines a model-agnostic embedding interface.
type EmbeddingProvider interface {
	CreateEmbeddings(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error)
}

// ChatRequest is a model-agnostic chat request.
type ChatRequest struct {
	SystemPrompt     string
	Messages         []Message
	Provider         string
	Model            string
	Temperature      float64
	MaxTokens        int
	JSONMode         bool
	JSONSchema       map[string]any
	JSONSchemaStrict bool
	Reasoning        *ReasoningConfig
	ProviderOptions  json.RawMessage
}

// ReasoningConfig controls provider reasoning budgets for compatible models.
type ReasoningConfig struct {
	Effort    string `json:"effort,omitempty"`
	MaxTokens int    `json:"max_tokens,omitempty"`
	Enabled   *bool  `json:"enabled,omitempty"`
	Exclude   bool   `json:"exclude,omitempty"`
}

// Message represents a conversation message.
type Message struct {
	Role         string
	Content      string
	ContentParts []ContentPart
}

// ContentPart represents a multimodal chat content part.
type ContentPart struct {
	Type     string
	Text     string
	ImageURL *ImageURLPart
}

// ImageURLPart references an image the model can inspect.
type ImageURLPart struct {
	URL    string
	Detail string
}

// ChatResponse is a model-agnostic chat response.
type ChatResponse struct {
	Content                string
	TokensUsed             TokenUsage
	Provider, Model, Route string
	ServiceTier            string
	FinishReason           string
}

// ChatPricingIdentity is the exact route selected before a provider call.
type ChatPricingIdentity struct {
	Provider, Model, Route, ServiceTier string
}

// PricingIdentityResolver exposes a provider's exact request route for preflight pricing.
type PricingIdentityResolver interface {
	ResolvePricingIdentity(ChatRequest) (ChatPricingIdentity, error)
}

// EmbeddingRequest is a model-agnostic embedding request.
type EmbeddingRequest struct {
	Provider string
	Model    string
	Inputs   []string
}

// EmbeddingResponse contains the generated embedding vectors.
type EmbeddingResponse struct {
	Vectors [][]float32
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	InputTokens, InputTokensTotal       int
	CachedInputTokens, CacheReadTokens  int
	CacheWriteTokens                    int
	OutputTokens, CompletionTokensTotal int
	ReasoningTokens                     int
	CompletionIncludesReasoning         bool
}
