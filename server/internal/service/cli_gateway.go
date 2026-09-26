package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func (s *CLIService) modelRoute(ctx context.Context, run *model.AgentRun) (sdk.RunModel, *sdk.ModelCredential, error) {
	var input model.AgentRunInputPayload
	if err := json.Unmarshal(run.Input, &input); err != nil {
		return sdk.RunModel{}, nil, err
	}
	if input.AISelection == nil || s.agents.aiProfiles == nil {
		return sdk.RunModel{}, nil, ErrCLIInvalid
	}
	route := input.AISelection.Route.Model
	if route.Provider != "openrouter" || route.Endpoint != nil {
		return route, nil, ErrCLIInvalid
	}
	credential, err := s.agents.aiProfiles.Restore(ctx, run.WorkspaceID, derefString(run.TriggeredByUserID), input.AISelection)
	if err != nil {
		return route, nil, err
	}
	if credential == nil || credential.Type != "api_key" || credential.APIKey == "" {
		return route, nil, ErrCLIInvalid
	}
	return route, credential, nil
}
func (s *CLIService) generateNative(ctx context.Context, run *model.AgentRun, req model.CLINativeRequest) (*model.CLINativeResponse, error) {
	route, credential, err := s.modelRoute(ctx, run)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return generateCLIChat(ctx, route, credential, req, client)
}

func generateCLIChat(ctx context.Context, route sdk.RunModel, credential *sdk.ModelCredential, req model.CLINativeRequest, client *http.Client) (*model.CLINativeResponse, error) {
	messages := []map[string]any{{"role": "system", "content": req.SystemPrompt}}
	for _, m := range req.Messages {
		if !slices.Contains([]string{"user", "assistant", "tool"}, m.Role) {
			return nil, ErrCLIInvalid
		}
		text := m.Content
		var calls []map[string]any
		var results []map[string]any
		for _, b := range m.Blocks {
			switch b.Type {
			case "text":
				text += b.Text
			case "tool_call":
				calls = append(calls, map[string]any{"id": b.ToolCallID, "type": "function", "function": map[string]any{"name": b.ToolName, "arguments": string(b.Input)}})
			case "tool_result":
				results = append(results, map[string]any{"role": "tool", "tool_call_id": b.ToolCallID, "content": b.Output})
			default:
				return nil, ErrCLIInvalid
			}
		}
		if text != "" || len(calls) > 0 {
			entry := map[string]any{"role": m.Role, "content": text}
			if len(calls) > 0 {
				entry["tool_calls"] = calls
			}
			if len(m.ProviderState) > 0 {
				var state struct {
					Provider  string          `json:"provider"`
					Reasoning json.RawMessage `json:"reasoning"`
				}
				if err := json.Unmarshal(m.ProviderState, &state); err != nil || state.Provider != "cli-openrouter" {
					return nil, ErrCLIInvalid
				}
				entry["reasoning_details"] = state.Reasoning
			}
			if m.ReasoningContent != "" {
				entry["reasoning_content"] = m.ReasoningContent
			}
			messages = append(messages, entry)
		}
		messages = append(messages, results...)
	}
	defs := make([]map[string]any, 0, len(req.Tools))
	for _, d := range req.Tools {
		defs = append(defs, map[string]any{"type": "function", "function": map[string]any{"name": d.Name, "description": d.Description, "parameters": d.InputSchema}})
	}
	body := map[string]any{"model": route.Model, "messages": messages, "max_tokens": 8192, "stream": false}
	if len(defs) > 0 {
		body["tools"] = defs
	}
	if route.Controls != nil {
		raw, err := json.Marshal(route.Controls)
		if err != nil {
			return nil, err
		}
		var controls map[string]any
		if err = json.Unmarshal(raw, &controls); err != nil {
			return nil, err
		}
		for _, key := range []string{"reasoning_effort", "service_tier"} {
			if v, ok := controls[key]; ok {
				if key == "reasoning_effort" {
					body["reasoning"] = map[string]any{"effort": v}
				} else {
					tier, _ := v.(string)
					switch tier {
					case "standard":
						tier = "default"
					case "fast":
						tier = "priority"
					}
					body[key] = tier
				}
			}
		}
		if route.Provider == "openrouter" {
			if options, ok := controls["openrouter"].(map[string]any); ok {
				for k, v := range options {
					body[k] = v
				}
			}
		}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	endpoint := "https://openrouter.ai/api/v1/chat/completions"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+credential.APIKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return nil, errors.New("model provider request did not complete; inspect this attempt before retrying")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, errors.New("model provider rejected the request; inspect provider configuration and credits")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (8<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 8<<20 {
		return nil, ErrCLIInvalid
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content          string          `json:"content"`
				Reasoning        string          `json:"reasoning_content"`
				ReasoningDetails json.RawMessage `json:"reasoning_details"`
				ToolCalls        []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			Input  int64 `json:"prompt_tokens"`
			Output int64 `json:"completion_tokens"`
			Cached struct {
				Tokens int64 `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
			Reasoning struct {
				Tokens int64 `json:"reasoning_tokens"`
			} `json:"completion_tokens_details"`
		} `json:"usage"`
	}
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if len(result.Choices) != 1 {
		return nil, ErrCLIInvalid
	}
	choice := result.Choices[0]
	if result.Usage.Input+result.Usage.Output <= 0 {
		return nil, errors.New("model provider omitted usage telemetry")
	}
	output := &model.CLINativeResponse{Incomplete: choice.FinishReason == "length", Message: model.CLINativeMessage{Role: "assistant", Content: choice.Message.Content, ReasoningContent: choice.Message.Reasoning}, Usage: model.CLIUsage{InputTokens: result.Usage.Input, OutputTokens: result.Usage.Output, CachedInputTokens: result.Usage.Cached.Tokens, ReasoningOutputTokens: result.Usage.Reasoning.Tokens}}
	if len(choice.Message.ReasoningDetails) > 0 && string(choice.Message.ReasoningDetails) != "null" {
		state, err := json.Marshal(map[string]any{"provider": "cli-openrouter", "reasoning": choice.Message.ReasoningDetails})
		if err != nil {
			return nil, err
		}
		output.Message.ProviderState = state
	}
	for _, call := range choice.Message.ToolCalls {
		if !json.Valid([]byte(call.Function.Arguments)) {
			// Truncated tool JSON must not discard already observed usage or execute.
			output.Incomplete = true
			output.Message.Blocks = nil
			break
		}
		output.Message.Blocks = append(output.Message.Blocks, model.CLINativeBlock{Type: "tool_call", ToolCallID: call.ID, ToolName: call.Function.Name, Input: json.RawMessage(call.Function.Arguments)})
	}
	if output.Usage.InputTokens < 0 || output.Usage.OutputTokens < 0 || output.Usage.CachedInputTokens < 0 || output.Usage.CachedInputTokens > output.Usage.InputTokens || output.Usage.ReasoningOutputTokens < 0 || output.Usage.ReasoningOutputTokens > output.Usage.OutputTokens {
		return nil, ErrCLIInvalid
	}
	return output, nil
}

func cliRequestKey(parts ...string) string { return mcpHash(strings.Join(parts, "\x00")) }
