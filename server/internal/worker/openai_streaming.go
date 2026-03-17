package worker

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
	defaultOpenAIBaseURL = "https://api.openai.com/v1"
	defaultOpenAIModel   = "gpt-4o"
)

// OpenAIStreamingClient implements StreamingProvider for OpenAI-compatible APIs.
type OpenAIStreamingClient struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewOpenAIStreamingClient creates a new OpenAI streaming client.
// baseURL defaults to https://api.openai.com/v1 if empty.
// model defaults to gpt-4o if empty.
func NewOpenAIStreamingClient(apiKey, baseURL, model string) *OpenAIStreamingClient {
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if model == "" {
		model = defaultOpenAIModel
	}
	return &OpenAIStreamingClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// --- OpenAI request types ---

type openAIRequest struct {
	Model         string              `json:"model"`
	Messages      []openAIMessage     `json:"messages"`
	Tools         []openAITool        `json:"tools,omitempty"`
	Stream        bool                `json:"stream"`
	StreamOptions *openAIStreamOpts   `json:"stream_options,omitempty"`
	MaxTokens     int                 `json:"max_tokens,omitempty"`
}

type openAIStreamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    interface{}      `json:"content,omitempty"`    // string or null
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"` // assistant only
	ToolCallID string           `json:"tool_call_id,omitempty"` // tool role only
}

type openAITool struct {
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

type openAIFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type openAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIFunctionCall `json:"function"`
}

type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// --- SSE chunk types ---

type openAIChunk struct {
	ID      string              `json:"id"`
	Choices []openAIChunkChoice `json:"choices"`
	Usage   *openAIUsage        `json:"usage,omitempty"`
}

type openAIChunkChoice struct {
	Index        int              `json:"index"`
	Delta        openAIChunkDelta `json:"delta"`
	FinishReason *string          `json:"finish_reason"`
}

type openAIChunkDelta struct {
	Content   *string                `json:"content,omitempty"`
	ToolCalls []openAIChunkToolCall  `json:"tool_calls,omitempty"`
}

type openAIChunkToolCall struct {
	Index    int                    `json:"index"`
	ID       string                 `json:"id,omitempty"`
	Type     string                 `json:"type,omitempty"`
	Function openAIChunkFunctionCall `json:"function,omitempty"`
}

type openAIChunkFunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// CreateMessageStream translates a Claude-shaped request to OpenAI format,
// streams the response, and emits StreamEvents matching the Claude event schema.
func (c *OpenAIStreamingClient) CreateMessageStream(ctx context.Context, req CreateMessageRequest, onEvent func(StreamEvent)) (*StreamResult, error) {
	model := c.model
	if req.Model != "" {
		model = req.Model
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 8192
	}

	// Translate messages.
	oaiMessages := c.translateMessages(req.System, req.Messages)

	// Translate tools.
	var oaiTools []openAITool
	for _, t := range req.Tools {
		oaiTools = append(oaiTools, openAITool{
			Type: "function",
			Function: openAIFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	oaiReq := openAIRequest{
		Model:     model,
		Messages:  oaiMessages,
		Tools:     oaiTools,
		Stream:    true,
		MaxTokens: maxTokens,
		StreamOptions: &openAIStreamOpts{
			IncludeUsage: true,
		},
	}

	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	// Use a client without timeout for streaming (context handles cancellation).
	streamClient := &http.Client{}
	resp, err := streamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, fmt.Errorf("openai API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return c.parseSSEStream(ctx, resp.Body, onEvent)
}

// translateMessages converts Claude-shaped messages to OpenAI format.
func (c *OpenAIStreamingClient) translateMessages(system string, messages []Message) []openAIMessage {
	var oai []openAIMessage

	// System message.
	if system != "" {
		oai = append(oai, openAIMessage{Role: "system", Content: system})
	}

	for _, msg := range messages {
		switch content := msg.Content.(type) {
		case string:
			oai = append(oai, openAIMessage{Role: msg.Role, Content: content})

		case []ContentBlock:
			oai = append(oai, c.translateBlockMessage(msg.Role, content)...)

		case []interface{}:
			// JSON-unmarshaled content blocks — convert through JSON round-trip.
			raw, _ := json.Marshal(content)
			var blocks []ContentBlock
			if json.Unmarshal(raw, &blocks) == nil && len(blocks) > 0 {
				oai = append(oai, c.translateBlockMessage(msg.Role, blocks)...)
			} else {
				oai = append(oai, openAIMessage{Role: msg.Role, Content: fmt.Sprintf("%v", content)})
			}

		default:
			// Fallback: try JSON round-trip for typed slices.
			raw, _ := json.Marshal(content)
			var blocks []ContentBlock
			if json.Unmarshal(raw, &blocks) == nil && len(blocks) > 0 {
				oai = append(oai, c.translateBlockMessage(msg.Role, blocks)...)
			} else {
				oai = append(oai, openAIMessage{Role: msg.Role, Content: fmt.Sprintf("%v", content)})
			}
		}
	}

	return oai
}

// translateBlockMessage converts a slice of ContentBlocks to one or more OpenAI messages.
func (c *OpenAIStreamingClient) translateBlockMessage(role string, blocks []ContentBlock) []openAIMessage {
	var result []openAIMessage

	// Check if these are tool_result blocks (user role).
	var toolResults []ContentBlock
	var otherBlocks []ContentBlock
	for _, b := range blocks {
		if b.Type == "tool_result" {
			toolResults = append(toolResults, b)
		} else {
			otherBlocks = append(otherBlocks, b)
		}
	}

	// Emit tool results as individual "tool" messages.
	for _, tr := range toolResults {
		result = append(result, openAIMessage{
			Role:       "tool",
			Content:    tr.Content,
			ToolCallID: tr.ToolUseID,
		})
	}

	if len(otherBlocks) == 0 {
		return result
	}

	// For assistant messages: extract text + tool_use blocks.
	if role == "assistant" {
		var textParts []string
		var toolCalls []openAIToolCall
		for _, b := range otherBlocks {
			switch b.Type {
			case "text":
				textParts = append(textParts, b.Text)
			case "tool_use":
				toolCalls = append(toolCalls, openAIToolCall{
					ID:   b.ID,
					Type: "function",
					Function: openAIFunctionCall{
						Name:      b.Name,
						Arguments: string(b.Input),
					},
				})
			}
		}
		msg := openAIMessage{Role: "assistant"}
		if len(textParts) > 0 {
			msg.Content = strings.Join(textParts, "\n")
		}
		if len(toolCalls) > 0 {
			msg.ToolCalls = toolCalls
		}
		result = append(result, msg)
		return result
	}

	// For user messages: concatenate text blocks.
	var textParts []string
	for _, b := range otherBlocks {
		if b.Type == "text" {
			textParts = append(textParts, b.Text)
		}
	}
	if len(textParts) > 0 {
		result = append(result, openAIMessage{Role: role, Content: strings.Join(textParts, "\n")})
	}

	return result
}

// parseSSEStream reads the OpenAI SSE stream and emits Claude-compatible StreamEvents.
func (c *OpenAIStreamingClient) parseSSEStream(ctx context.Context, body io.Reader, onEvent func(StreamEvent)) (*StreamResult, error) {
	result := &StreamResult{}

	// Track state for accumulation.
	var accumulatedText strings.Builder
	textStarted := false
	textIndex := 0

	type pendingToolCall struct {
		index int
		id    string
		name  string
		args  strings.Builder
	}
	pendingTools := make(map[int]*pendingToolCall) // keyed by delta tool_calls[i].index
	nextBlockIndex := 0

	emit := func(e StreamEvent) {
		if onEvent != nil {
			onEvent(e)
		}
	}

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)

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

		var chunk openAIChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// Handle usage (comes in the final chunk with stream_options.include_usage).
		if chunk.Usage != nil {
			result.Usage.InputTokens = chunk.Usage.PromptTokens
			result.Usage.OutputTokens = chunk.Usage.CompletionTokens
			emit(StreamEvent{
				Type:  "message_delta",
				Usage: &Usage{OutputTokens: chunk.Usage.CompletionTokens},
			})
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]
		delta := choice.Delta

		// Text content.
		if delta.Content != nil && *delta.Content != "" {
			if !textStarted {
				textStarted = true
				textIndex = nextBlockIndex
				nextBlockIndex++
				emit(StreamEvent{
					Type:  "content_block_start",
					Index: textIndex,
					ContentBlock: &ContentBlock{
						Type: "text",
					},
				})
			}
			accumulatedText.WriteString(*delta.Content)
			emit(StreamEvent{
				Type:  "content_block_delta",
				Index: textIndex,
				Delta: &StreamDelta{
					Type: "text_delta",
					Text: *delta.Content,
				},
			})
		}

		// Tool calls.
		for _, tc := range delta.ToolCalls {
			pt, exists := pendingTools[tc.Index]
			if !exists {
				// New tool call — close text block first if open.
				if textStarted {
					emit(StreamEvent{Type: "content_block_stop", Index: textIndex})
					textStarted = false
				}
				blockIdx := nextBlockIndex
				nextBlockIndex++
				pt = &pendingToolCall{
					index: blockIdx,
					id:    tc.ID,
					name:  tc.Function.Name,
				}
				pendingTools[tc.Index] = pt

				emit(StreamEvent{
					Type:  "content_block_start",
					Index: blockIdx,
					ContentBlock: &ContentBlock{
						Type: "tool_use",
						ID:   tc.ID,
						Name: tc.Function.Name,
					},
				})
			}

			// Accumulate arguments.
			if tc.Function.Arguments != "" {
				pt.args.WriteString(tc.Function.Arguments)
				emit(StreamEvent{
					Type:  "content_block_delta",
					Index: pt.index,
					Delta: &StreamDelta{
						Type:        "input_json_delta",
						PartialJSON: tc.Function.Arguments,
					},
				})
			}
		}

		// Finish reason.
		if choice.FinishReason != nil {
			// Close any open blocks.
			if textStarted {
				emit(StreamEvent{Type: "content_block_stop", Index: textIndex})
			}
			for _, pt := range pendingTools {
				emit(StreamEvent{Type: "content_block_stop", Index: pt.index})
			}

			stopReason := mapStopReason(*choice.FinishReason)
			result.StopReason = stopReason
			emit(StreamEvent{
				Type: "message_delta",
				Delta: &StreamDelta{
					StopReason: stopReason,
				},
			})
		}
	}

	if err := scanner.Err(); err != nil {
		result.Err = fmt.Errorf("read stream: %w", err)
	}

	// Build final content blocks.
	result.ContentBlocks = make([]ContentBlock, 0, nextBlockIndex)

	if accumulatedText.Len() > 0 {
		for len(result.ContentBlocks) <= textIndex {
			result.ContentBlocks = append(result.ContentBlocks, ContentBlock{})
		}
		result.ContentBlocks[textIndex] = ContentBlock{
			Type: "text",
			Text: accumulatedText.String(),
		}
	}

	for _, pt := range pendingTools {
		for len(result.ContentBlocks) <= pt.index {
			result.ContentBlocks = append(result.ContentBlocks, ContentBlock{})
		}
		result.ContentBlocks[pt.index] = ContentBlock{
			Type:  "tool_use",
			ID:    pt.id,
			Name:  pt.name,
			Input: json.RawMessage(pt.args.String()),
		}
	}

	return result, nil
}

// mapStopReason translates OpenAI finish reasons to Claude stop reasons.
func mapStopReason(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "tool_calls":
		return "tool_use"
	case "length":
		return "max_tokens"
	default:
		return reason
	}
}
