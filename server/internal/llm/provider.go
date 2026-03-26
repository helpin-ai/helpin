package llm

import "context"

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
	SystemPrompt string
	Messages     []Message
	Provider     string
	Model        string
	Temperature  float64
	MaxTokens    int
	JSONMode     bool
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
	Content    string
	TokensUsed TokenUsage
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
	InputTokens  int
	OutputTokens int
}
