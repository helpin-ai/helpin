package agentcontract

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	claudeAPIURL = "https://api.anthropic.com/v1/messages"
	claudeModel  = "claude-sonnet-4-6"
)

// ClaudeClient calls the Anthropic Messages API.
type ClaudeClient struct {
	apiKey     string
	httpClient *http.Client
}

// NewClaudeClient creates a new Claude API client.
func NewClaudeClient(apiKey string) *ClaudeClient {
	return &ClaudeClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// Message represents a conversation message.
type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []ContentBlock
}

// ContentBlock represents a content block in a message.
type ContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Source    interface{}     `json:"source,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

// ToolDefinition describes a tool for the Claude API.
type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
}

// ToolChoice forces the model to call a specific tool when tool use is enabled.
type ToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// CreateMessageRequest is the request body for the Messages API.
type CreateMessageRequest struct {
	Model      string           `json:"model"`
	MaxTokens  int              `json:"max_tokens"`
	System     string           `json:"system,omitempty"`
	Messages   []Message        `json:"messages"`
	Tools      []ToolDefinition `json:"tools,omitempty"`
	ToolChoice *ToolChoice      `json:"tool_choice,omitempty"`
	Stream     bool             `json:"stream,omitempty"`
}

// CreateMessageResponse is the response from the Messages API.
type CreateMessageResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	StopReason   string         `json:"stop_reason"`
	StopSequence *string        `json:"stop_sequence"`
	Usage        Usage          `json:"usage"`
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	OutputTokens             int `json:"output_tokens"`
}

// CreateMessage sends a request to the Claude Messages API.
func (c *ClaudeClient) CreateMessage(ctx context.Context, req CreateMessageRequest) (*CreateMessageResponse, error) {
	if req.Model == "" {
		req.Model = claudeModel
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = 16384
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", claudeAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("claude API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result CreateMessageResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &result, nil
}

// StreamEvent represents a single SSE event from the Claude streaming API.
type StreamEvent struct {
	Type         string                 `json:"type"`
	Index        int                    `json:"index,omitempty"`
	ContentBlock *ContentBlock          `json:"content_block,omitempty"`
	Delta        *StreamDelta           `json:"delta,omitempty"`
	Message      *CreateMessageResponse `json:"message,omitempty"`
	Usage        *Usage                 `json:"usage,omitempty"`
	Error        *StreamError           `json:"error,omitempty"`
}

// StreamDelta carries incremental content changes.
type StreamDelta struct {
	Type        string `json:"type,omitempty"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}

// StreamError carries API error information.
type StreamError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// StreamResult is the final result returned after a streaming call completes.
type StreamResult struct {
	ContentBlocks []ContentBlock
	StopReason    string
	Usage         Usage
	Err           error
}

// CreateMessageStream sends a streaming request to the Claude Messages API.
// The callback is invoked for each SSE event as it arrives.
// Returns the final aggregated result when the stream ends.
func (c *ClaudeClient) CreateMessageStream(ctx context.Context, req CreateMessageRequest, onEvent func(StreamEvent)) (*StreamResult, error) {
	req.Stream = true
	if req.Model == "" {
		req.Model = claudeModel
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = 8192
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", claudeAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	// Use a client without timeout for streaming (context handles cancellation).
	streamClient := &http.Client{}
	resp, err := streamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, fmt.Errorf("claude API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse SSE stream.
	result := &StreamResult{}
	var currentBlock *ContentBlock
	var currentIndex int
	var inputJSON strings.Builder

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024) // 256KB buffer for large tool inputs

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			result.Err = ctx.Err()
			return result, nil
		default:
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var event StreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue // skip malformed events
		}

		switch event.Type {
		case "message_start":
			if event.Message != nil && event.Message.Usage.InputTokens > 0 {
				result.Usage.InputTokens = event.Message.Usage.InputTokens
			}

		case "content_block_start":
			if event.ContentBlock != nil {
				currentIndex = event.Index
				block := *event.ContentBlock
				currentBlock = &block
				inputJSON.Reset()
			}

		case "content_block_delta":
			if event.Delta != nil && currentBlock != nil {
				switch event.Delta.Type {
				case "text_delta":
					currentBlock.Text += event.Delta.Text
				case "input_json_delta":
					inputJSON.WriteString(event.Delta.PartialJSON)
				}
			}

		case "content_block_stop":
			if currentBlock != nil {
				if currentBlock.Type == "tool_use" && inputJSON.Len() > 0 {
					currentBlock.Input = json.RawMessage(inputJSON.String())
				}
				// Grow the blocks slice if needed.
				for len(result.ContentBlocks) <= currentIndex {
					result.ContentBlocks = append(result.ContentBlocks, ContentBlock{})
				}
				result.ContentBlocks[currentIndex] = *currentBlock
				currentBlock = nil
			}

		case "message_delta":
			if event.Delta != nil && event.Delta.StopReason != "" {
				result.StopReason = event.Delta.StopReason
			}
			if event.Usage != nil {
				result.Usage.OutputTokens = event.Usage.OutputTokens
			}

		case "message_stop":
			// Stream complete.

		case "error":
			if event.Error != nil {
				result.Err = fmt.Errorf("stream error: %s: %s", event.Error.Type, event.Error.Message)
				return result, nil
			}
		}

		if onEvent != nil {
			onEvent(event)
		}
	}

	if err := scanner.Err(); err != nil {
		result.Err = fmt.Errorf("read stream: %w", err)
	}

	return result, nil
}
