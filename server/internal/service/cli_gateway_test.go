package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type cliTransport func(*http.Request) (*http.Response, error)

func (f cliTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestCLIOpenRouterWirePreservesToolsReasoningAndUsage(t *testing.T) {
	client := &http.Client{Transport: cliTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://openrouter.ai/api/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer fixture-secret" {
			t.Fatal("wrong provider destination or credentials")
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if string(body["model"]) != `"fixture/model"` || string(body["max_tokens"]) != "8192" {
			t.Fatalf("wrong route: %s", body["model"])
		}
		if !strings.Contains(string(body["messages"]), "tool_call_id") || !strings.Contains(string(body["messages"]), "reasoning_details") {
			t.Fatal("tool history lost")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"tool_calls","message":{"content":"","reasoning_details":[{"type":"reasoning.encrypted","data":"opaque"}],"tool_calls":[{"id":"next","function":{"name":"read_files","arguments":"{\"files\":[]}"}}]}}],"usage":{"prompt_tokens":100,"completion_tokens":30,"prompt_tokens_details":{"cached_tokens":25},"completion_tokens_details":{"reasoning_tokens":10}}}`)), Header: make(http.Header)}, nil
	})}
	req := model.CLINativeRequest{SystemPrompt: "Host policy", Messages: []model.CLINativeMessage{
		{Role: "assistant", ProviderState: json.RawMessage(`{"provider":"cli-openrouter","reasoning":[{"type":"reasoning.encrypted","data":"prior"}]}`), Blocks: []model.CLINativeBlock{{Type: "tool_call", ToolCallID: "call-1", ToolName: "read_files", Input: json.RawMessage(`{}`)}}},
		{Role: "tool", Blocks: []model.CLINativeBlock{{Type: "tool_result", ToolCallID: "call-1", Output: "contents"}}},
	}, Tools: []model.CLIToolDefinition{{Name: "read_files", InputSchema: map[string]any{"type": "object"}}}}
	result, err := generateCLIChat(context.Background(), sdk.RunModel{Provider: "openrouter", Model: "fixture/model"}, &sdk.ModelCredential{APIKey: "fixture-secret"}, req, client)
	if err != nil {
		t.Fatal(err)
	}
	if result.Usage.InputTokens != 100 || result.Usage.ReasoningOutputTokens != 10 || len(result.Message.ProviderState) == 0 || len(result.Message.Blocks) != 1 {
		t.Fatalf("incomplete native response: %+v", result)
	}
}

func TestCLITruncatedToolRetainsProviderUsage(t *testing.T) {
	client := &http.Client{Transport: cliTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"length","message":{"tool_calls":[{"id":"cut","function":{"name":"write_file","arguments":"{broken"}}]}}],"usage":{"prompt_tokens":100,"completion_tokens":8192}}`))}, nil
	})}
	result, err := generateCLIChat(context.Background(), sdk.RunModel{Provider: "openrouter", Model: "fixture/model"}, &sdk.ModelCredential{APIKey: "fixture"}, model.CLINativeRequest{}, client)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Incomplete || len(result.Message.Blocks) != 0 || result.Usage.OutputTokens != 8192 {
		t.Fatalf("truncated response lost usage or retained a tool: %+v", result)
	}
}
