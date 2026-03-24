package worker

import (
	"encoding/json"
	"testing"

	"github.com/cloudwego/eino/schema"
	openaischema "github.com/cloudwego/eino/schema/openai"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestProviderUsesAgenticResponses(t *testing.T) {
	for _, provider := range []string{
		model.AgentModelProviderOpenAI,
		model.AgentModelProviderOpenRouter,
		model.AgentModelProviderOpenRouterResponses,
	} {
		if !providerUsesAgenticResponses(provider) {
			t.Fatalf("expected provider %q to use agentic responses", provider)
		}
	}
	if providerUsesAgenticResponses(model.AgentModelProviderAnthropic) {
		t.Fatalf("did not expect anthropic to use agentic responses")
	}
}

func TestProviderContinuationFromAgenticMessage(t *testing.T) {
	msg := &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeAssistant,
		ResponseMeta: &schema.AgenticResponseMeta{
			OpenAIExtension: &openaischema.ResponseMetaExtension{
				ID:                 "resp_123",
				PreviousResponseID: "resp_122",
			},
		},
	}

	continuation := providerContinuationFromAgenticMessage(msg)
	if continuation == nil {
		t.Fatal("expected continuation metadata")
	}
	if continuation.Provider != model.AgentModelProviderOpenAI {
		t.Fatalf("expected provider %q, got %q", model.AgentModelProviderOpenAI, continuation.Provider)
	}
	if continuation.ResponseID != "resp_123" || continuation.PreviousResponseID != "resp_122" {
		t.Fatalf("unexpected continuation %#v", continuation)
	}
}

func TestProviderContinuationFromAgenticMessageRequiresResponseID(t *testing.T) {
	msg := &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeAssistant,
		ResponseMeta: &schema.AgenticResponseMeta{
			OpenAIExtension: &openaischema.ResponseMetaExtension{},
		},
	}
	if continuation := providerContinuationFromAgenticMessage(msg); continuation != nil {
		t.Fatalf("expected nil continuation, got %#v", continuation)
	}
}

func TestFilterExecutionHistoryAfterSequence(t *testing.T) {
	history := []ExecutionMessage{
		{SequenceNo: 1, Role: "user", Content: "one"},
		{SequenceNo: 2, Role: "assistant", Content: "two"},
		{SequenceNo: 3, Role: "tool", Content: "three"},
	}

	filtered := filterExecutionHistoryAfterSequence(history, 2)
	if len(filtered) != 1 || filtered[0].SequenceNo != 3 {
		t.Fatalf("unexpected filtered history %#v", filtered)
	}
}

func TestToAgenticMessagesIncludesToolResults(t *testing.T) {
	history := []ExecutionMessage{
		{
			Role: "assistant",
			Blocks: []ExecutionBlock{
				{Type: ExecutionBlockTypeText, Text: "Need a tool"},
				{Type: ExecutionBlockTypeToolCall, ToolCallID: "call-1", ToolName: "read_file", Input: json.RawMessage(`{"path":"a.go"}`)},
			},
		},
		{
			Role: "tool",
			Blocks: []ExecutionBlock{
				{Type: ExecutionBlockTypeToolResult, ToolCallID: "call-1", ToolName: "read_file", Output: "package main"},
			},
		},
	}

	msgs, err := toAgenticMessages("system prompt", history)
	if err != nil {
		t.Fatalf("toAgenticMessages returned error: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if msgs[1].Role != schema.AgenticRoleTypeAssistant {
		t.Fatalf("expected assistant message, got %q", msgs[1].Role)
	}
	if len(msgs[1].ContentBlocks) != 2 {
		t.Fatalf("expected assistant content blocks, got %#v", msgs[1].ContentBlocks)
	}
	if msgs[2].Role != schema.AgenticRoleTypeUser {
		t.Fatalf("expected tool result to be encoded as a user-side agentic message, got role %q", msgs[2].Role)
	}
}

func TestToAgenticMessagesAllowsUserSummaryFirst(t *testing.T) {
	history := []ExecutionMessage{
		{Role: "user", Content: "Resume context from earlier turns:\nSummary"},
		{Role: "assistant", Content: "Need a tool", Blocks: []ExecutionBlock{{Type: ExecutionBlockTypeToolCall, ToolCallID: "call-1", ToolName: "read_file", Input: json.RawMessage(`{"path":"a.go"}`)}}},
	}

	msgs, err := toAgenticMessages("system prompt", history)
	if err != nil {
		t.Fatalf("toAgenticMessages returned error: %v", err)
	}
	if len(msgs) < 2 {
		t.Fatalf("expected system plus history messages, got %#v", msgs)
	}
	if msgs[1].Role != schema.AgenticRoleTypeUser {
		t.Fatalf("expected first non-system message to be user, got %q", msgs[1].Role)
	}
}

func TestToSchemaMessagesNormalizesEmptyToolCallInput(t *testing.T) {
	history := []ExecutionMessage{
		{
			Role: "assistant",
			Blocks: []ExecutionBlock{
				{Type: ExecutionBlockTypeToolCall, ToolCallID: "call-1", ToolName: "read_file"},
			},
		},
	}

	msgs, err := toSchemaMessages("system prompt", history)
	if err != nil {
		t.Fatalf("toSchemaMessages returned error: %v", err)
	}
	if got := msgs[1].ToolCalls[0].Function.Arguments; got != "{}" {
		t.Fatalf("expected normalized empty tool args, got %q", got)
	}
}

func TestToAgenticMessagesNormalizesEmptyToolCallInput(t *testing.T) {
	history := []ExecutionMessage{
		{
			Role: "assistant",
			Blocks: []ExecutionBlock{
				{Type: ExecutionBlockTypeToolCall, ToolCallID: "call-1", ToolName: "read_file"},
			},
		},
	}

	msgs, err := toAgenticMessages("system prompt", history)
	if err != nil {
		t.Fatalf("toAgenticMessages returned error: %v", err)
	}
	if len(msgs) < 2 || len(msgs[1].ContentBlocks) != 1 || msgs[1].ContentBlocks[0].FunctionToolCall == nil {
		t.Fatalf("unexpected agentic messages %#v", msgs)
	}
	if got := msgs[1].ContentBlocks[0].FunctionToolCall.Arguments; got != "{}" {
		t.Fatalf("expected normalized empty tool args, got %q", got)
	}
}

func TestFromSchemaAgenticAssistantMessageIncludesTextAndToolCalls(t *testing.T) {
	msg := &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeAssistant,
		ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlock(&schema.AssistantGenText{Text: "hello"}),
			schema.NewContentBlock(&schema.FunctionToolCall{CallID: "call-1", Name: "list_directory", Arguments: `{"path":"."}`}),
		},
	}

	blocks := fromSchemaAgenticAssistantMessage(msg)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 execution blocks, got %#v", blocks)
	}
	if blocks[0].Type != ExecutionBlockTypeText || blocks[0].Text != "hello" {
		t.Fatalf("unexpected text block %#v", blocks[0])
	}
	if blocks[1].Type != ExecutionBlockTypeToolCall || blocks[1].ToolCallID != "call-1" || blocks[1].ToolName != "list_directory" {
		t.Fatalf("unexpected tool block %#v", blocks[1])
	}
}

func TestSanitizeSchemaMessageToolCallsNormalizesInvalidArguments(t *testing.T) {
	msg := &schema.Message{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{
			{
				ID:   "call-1",
				Type: "function",
				Function: schema.FunctionCall{
					Name:      "read_file",
					Arguments: "",
				},
			},
		},
	}

	sanitizeSchemaMessageToolCalls(msg)

	if got := msg.ToolCalls[0].Function.Arguments; got != "{}" {
		t.Fatalf("expected normalized tool args, got %q", got)
	}
}

func TestNormalizeToolArgumentsConvertsJSONStringsToObjects(t *testing.T) {
	if got := string(normalizeToolArguments(`""`)); got != "{}" {
		t.Fatalf("expected empty JSON string args to normalize to object, got %q", got)
	}
	if got := string(normalizeToolArguments(`"read this file"`)); got != `{"raw":"read this file"}` {
		t.Fatalf("expected JSON string args to be wrapped, got %q", got)
	}
}
