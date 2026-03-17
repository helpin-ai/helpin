package worker

import "context"

// StreamingProvider abstracts LLM providers that support streaming with tool use.
// ClaudeClient and OpenAIStreamingClient both implement this interface.
type StreamingProvider interface {
	CreateMessageStream(ctx context.Context, req CreateMessageRequest, onEvent func(StreamEvent)) (*StreamResult, error)
}
