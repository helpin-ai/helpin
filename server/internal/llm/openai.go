package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxOpenAIEmbeddingsResponseBytes = 32 << 20

// OpenAIProvider implements Provider for OpenAI-compatible APIs.
type OpenAIProvider struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

type openAIPromptTokenDetails struct {
	CachedTokens     int `json:"cached_tokens"`
	CacheWriteTokens int `json:"cache_write_tokens"`
}

type openAICompletionTokenDetails struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}

type openAIUsage struct {
	PromptTokens            int                          `json:"prompt_tokens"`
	CompletionTokens        int                          `json:"completion_tokens"`
	PromptTokensDetails     openAIPromptTokenDetails     `json:"prompt_tokens_details"`
	CompletionTokensDetails openAICompletionTokenDetails `json:"completion_tokens_details"`
}

type openAIChatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage openAIUsage `json:"usage"`
}

// NewOpenAIProvider creates an OpenAI-compatible LLM provider.
func NewOpenAIProvider(apiKey, baseURL, model string) *OpenAIProvider {
	if apiKey == "" {
		return nil
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "gpt-4o"
	}
	return &OpenAIProvider{
		apiKey:     apiKey,
		baseURL:    baseURL,
		model:      model,
		httpClient: &http.Client{Timeout: 5 * time.Minute},
	}
}

func (p *OpenAIProvider) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if p == nil {
		return nil, fmt.Errorf("openai provider is not configured")
	}

	var messages []map[string]any
	if req.SystemPrompt != "" {
		messages = append(messages, map[string]any{"role": "system", "content": req.SystemPrompt})
	}
	for _, m := range req.Messages {
		messages = append(messages, buildOpenAIMessage(m))
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	modelName := p.model
	if req.Model != "" {
		modelName = req.Model
	}

	body := p.buildChatCompletionBody(modelName, messages, maxTokens, req.Temperature, req.JSONMode, req.JSONSchema, req.JSONSchemaStrict, req.ProviderOptions)

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusPaymentRequired {
			return nil, fmt.Errorf("openai API error (status %d): %s: %w", resp.StatusCode, string(respBody), ErrInsufficientCredits)
		}
		return nil, fmt.Errorf("openai API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result openAIChatCompletionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	content := ""
	if len(result.Choices) > 0 {
		content = result.Choices[0].Message.Content
	}

	return &ChatResponse{
		Content:    content,
		TokensUsed: tokenUsageFromOpenAI(result.Usage),
		Provider:   req.Provider, Model: modelName, Route: modelName, ServiceTier: "standard",
	}, nil
}

// ResolvePricingIdentity returns the exact OpenAI-compatible route before execution.
func (p *OpenAIProvider) ResolvePricingIdentity(req ChatRequest) (ChatPricingIdentity, error) {
	if p == nil {
		return ChatPricingIdentity{}, fmt.Errorf("openai provider is not configured")
	}
	modelName := p.model
	if req.Model != "" {
		modelName = req.Model
	}
	provider := normalizeProviderName(req.Provider)
	if provider == "" {
		provider = "openai"
	}
	return ChatPricingIdentity{Provider: provider, Model: modelName, Route: modelName, ServiceTier: "standard"}, nil
}

func tokenUsageFromOpenAI(usage openAIUsage) TokenUsage {
	outputTokens := usage.CompletionTokens - usage.CompletionTokensDetails.ReasoningTokens
	if outputTokens < 0 {
		outputTokens = 0
	}
	return TokenUsage{
		InputTokens:                 usage.PromptTokens,
		InputTokensTotal:            usage.PromptTokens,
		CachedInputTokens:           usage.PromptTokensDetails.CachedTokens,
		CacheReadTokens:             usage.PromptTokensDetails.CachedTokens,
		CacheWriteTokens:            usage.PromptTokensDetails.CacheWriteTokens,
		OutputTokens:                outputTokens,
		CompletionTokensTotal:       usage.CompletionTokens,
		ReasoningTokens:             usage.CompletionTokensDetails.ReasoningTokens,
		CompletionIncludesReasoning: true,
	}
}

func (p *OpenAIProvider) buildChatCompletionBody(
	modelName string,
	messages []map[string]any,
	maxTokens int,
	temperature float64,
	jsonMode bool,
	jsonSchema map[string]any,
	jsonSchemaStrict bool,
	providerOptions json.RawMessage,
) map[string]interface{} {
	body := map[string]interface{}{
		"model":    modelName,
		"messages": messages,
	}
	if len(providerOptions) > 0 && strings.TrimSpace(string(providerOptions)) != "" {
		body["provider"] = json.RawMessage(providerOptions)
	}
	if p.usesMaxCompletionTokens(modelName) {
		body["max_completion_tokens"] = maxTokens
	} else {
		body["max_tokens"] = maxTokens
		body["temperature"] = temperature
	}
	if jsonMode && jsonSchemaStrict && len(jsonSchema) > 0 {
		body["response_format"] = map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "helpin_structured_response",
				"strict": true,
				"schema": jsonSchema,
			},
		}
	} else if jsonMode {
		body["response_format"] = map[string]string{"type": "json_object"}
	}
	return body
}

func buildOpenAIMessage(message Message) map[string]any {
	if len(message.ContentParts) == 0 {
		return map[string]any{
			"role":    message.Role,
			"content": message.Content,
		}
	}

	parts := make([]map[string]any, 0, len(message.ContentParts))
	for _, part := range message.ContentParts {
		switch part.Type {
		case "image_url":
			if part.ImageURL == nil || strings.TrimSpace(part.ImageURL.URL) == "" {
				continue
			}
			image := map[string]any{"url": strings.TrimSpace(part.ImageURL.URL)}
			if detail := strings.TrimSpace(part.ImageURL.Detail); detail != "" {
				image["detail"] = detail
			}
			parts = append(parts, map[string]any{
				"type":      "image_url",
				"image_url": image,
			})
		default:
			text := strings.TrimSpace(part.Text)
			if text == "" {
				continue
			}
			parts = append(parts, map[string]any{
				"type": "text",
				"text": text,
			})
		}
	}

	if len(parts) == 0 {
		return map[string]any{
			"role":    message.Role,
			"content": message.Content,
		}
	}

	return map[string]any{
		"role":    message.Role,
		"content": parts,
	}
}

func (p *OpenAIProvider) usesMaxCompletionTokens(modelName string) bool {
	model := strings.ToLower(strings.TrimSpace(modelName))
	return strings.HasPrefix(model, "gpt-5")
}

func (p *OpenAIProvider) CreateEmbeddings(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	if p == nil {
		return nil, fmt.Errorf("openai provider is not configured")
	}
	if len(req.Inputs) == 0 {
		return &EmbeddingResponse{Vectors: [][]float32{}}, nil
	}

	modelName := req.Model
	if modelName == "" {
		modelName = "text-embedding-3-small"
	}

	body := map[string]interface{}{
		"model": modelName,
		"input": req.Inputs,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal embeddings request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/embeddings", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create embeddings request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai embeddings request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxOpenAIEmbeddingsResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read embeddings response: %w", err)
	}
	if len(respBody) > maxOpenAIEmbeddingsResponseBytes {
		return nil, fmt.Errorf("embeddings response exceeded %d byte limit", maxOpenAIEmbeddingsResponseBytes)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai embeddings API error (status %d): %s", resp.StatusCode, string(respBody))
	}
	if len(bytes.TrimSpace(respBody)) == 0 {
		return nil, fmt.Errorf("openai embeddings API returned an empty response")
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse embeddings response (%d bytes): %w", len(respBody), err)
	}

	vectors := make([][]float32, 0, len(result.Data))
	for _, item := range result.Data {
		vectors = append(vectors, item.Embedding)
	}

	return &EmbeddingResponse{Vectors: vectors}, nil
}
