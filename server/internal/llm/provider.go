package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
)

// ErrInsufficientCredits indicates the upstream LLM provider rejected the
// request for billing reasons (HTTP 402 — insufficient credits or exhausted
// quota). Callers use errors.Is to degrade with a clear, actionable message
// instead of a generic failure.
var ErrInsufficientCredits = errors.New("llm: insufficient credits")

// ProviderErrorKind identifies provider failures that do not have an HTTP status.
type ProviderErrorKind string

const (
	// ProviderErrorUnavailable means the requested provider is not configured or reachable.
	ProviderErrorUnavailable ProviderErrorKind = "unavailable"
)

// ProviderError preserves provider failure identity for safe fallback decisions.
type ProviderError struct {
	Provider   string
	Operation  string
	Kind       ProviderErrorKind
	StatusCode int
	Message    string
	Err        error
}

func (e *ProviderError) Error() string {
	if e == nil {
		return "llm provider error"
	}
	message := e.Message
	if message == "" && e.Err != nil {
		message = e.Err.Error()
	}
	if message == "" {
		message = "request failed"
	}
	if e.StatusCode != 0 {
		return fmt.Sprintf("%s provider error (status %d): %s", e.Provider, e.StatusCode, message)
	}
	return fmt.Sprintf("%s provider error: %s", e.Provider, message)
}

// Unwrap exposes an underlying transport or sentinel error.
func (e *ProviderError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Retryable reports whether a declared fallback may safely be attempted.
func (e *ProviderError) Retryable() bool {
	if e == nil || errors.Is(e.Err, context.Canceled) || errors.Is(e.Err, context.DeadlineExceeded) {
		return false
	}
	if e.Kind == ProviderErrorUnavailable || errors.Is(e.Err, ErrInsufficientCredits) {
		return true
	}
	if e.StatusCode == http.StatusRequestTimeout || e.StatusCode == http.StatusTooManyRequests || e.StatusCode >= 500 {
		return true
	}
	var networkErr net.Error
	return errors.As(e.Err, &networkErr) && networkErr.Timeout()
}

// IsRetryableProviderError reports whether err permits a declared route fallback.
func IsRetryableProviderError(err error) bool {
	var providerErr *ProviderError
	return errors.As(err, &providerErr) && providerErr.Retryable()
}

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
