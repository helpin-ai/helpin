package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

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

func TestCompactToolOutputForModelCompactsLargeReadResults(t *testing.T) {
	output := strings.Repeat("line of file contents\n", 500)

	compacted := compactToolOutputForModel("read_file", output)

	if compacted == output {
		t.Fatal("expected large read_file output to be compacted")
	}
	if !strings.Contains(compacted, "truncated") {
		t.Fatalf("expected compaction marker, got %q", compacted)
	}
	if len([]rune(compacted)) >= len([]rune(output)) {
		t.Fatalf("expected compacted output to be shorter than original")
	}
}

func TestAnalyzeToolOutputForModelTracksCompactionMetadata(t *testing.T) {
	output := strings.Repeat("line of file contents\n", 500)

	analysis := analyzeToolOutputForModel("read_file", output)

	if !analysis.Compacted {
		t.Fatal("expected large read_file output to be marked compacted")
	}
	if analysis.OriginalRunes <= analysis.VisibleRunes {
		t.Fatalf("expected visible output to be smaller, got %#v", analysis)
	}
	if analysis.OriginalLines <= 0 || analysis.VisibleLines <= 0 {
		t.Fatalf("expected positive line counts, got %#v", analysis)
	}
	if !strings.Contains(analysis.Content, "truncated") {
		t.Fatalf("expected compaction marker in output, got %q", analysis.Content)
	}
}

func TestToSchemaMessagesCompactsLargeToolResultsForModelHistory(t *testing.T) {
	output := strings.Repeat("line of file contents\n", 500)
	history := []ExecutionMessage{
		{
			Role: "tool",
			Blocks: []ExecutionBlock{
				{Type: ExecutionBlockTypeToolResult, ToolCallID: "call-1", ToolName: "read_file", Output: output},
			},
		},
	}

	msgs, err := toSchemaMessages("system prompt", history)
	if err != nil {
		t.Fatalf("toSchemaMessages returned error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected system plus one tool message, got %#v", msgs)
	}
	if !strings.Contains(msgs[1].Content, "truncated") {
		t.Fatalf("expected compacted tool output marker, got %q", msgs[1].Content)
	}
	if msgs[1].Content == output {
		t.Fatal("expected tool output to be compacted for model history")
	}
}

func TestToAgenticMessagesCompactsLargeToolResultsForModelHistory(t *testing.T) {
	output := strings.Repeat("line of file contents\n", 500)
	history := []ExecutionMessage{
		{
			Role: "tool",
			Blocks: []ExecutionBlock{
				{Type: ExecutionBlockTypeToolResult, ToolCallID: "call-1", ToolName: "read_file", Output: output},
			},
		},
	}

	msgs, err := toAgenticMessages("system prompt", history)
	if err != nil {
		t.Fatalf("toAgenticMessages returned error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected system plus one tool result message, got %#v", msgs)
	}
	if len(msgs[1].ContentBlocks) != 1 || msgs[1].ContentBlocks[0].FunctionToolResult == nil {
		t.Fatalf("expected function tool result block, got %#v", msgs[1])
	}
	result := msgs[1].ContentBlocks[0].FunctionToolResult.Result
	if !strings.Contains(result, "truncated") {
		t.Fatalf("expected compacted tool output marker, got %q", result)
	}
	if result == output {
		t.Fatal("expected tool output to be compacted for agentic model history")
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

func TestToSchemaMessagesSkipsEmptyHistoryMessages(t *testing.T) {
	history := []ExecutionMessage{
		{Role: "user"},
		{Role: "assistant"},
		{Role: "tool"},
		{Role: "assistant", Blocks: []ExecutionBlock{{Type: ExecutionBlockTypeToolCall, ToolCallID: "call-1", ToolName: "read_file", Input: json.RawMessage(`{"path":"a.go"}`)}}},
	}

	msgs, err := toSchemaMessages("system prompt", history)
	if err != nil {
		t.Fatalf("toSchemaMessages returned error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected system plus one non-empty assistant message, got %#v", msgs)
	}
	if len(msgs[1].ToolCalls) != 1 || msgs[1].ToolCalls[0].Function.Name != "read_file" {
		t.Fatalf("expected non-empty assistant tool call to remain, got %#v", msgs[1])
	}
}

func TestToAgenticMessagesSkipsEmptyHistoryMessages(t *testing.T) {
	history := []ExecutionMessage{
		{Role: "user"},
		{Role: "assistant"},
		{Role: "tool"},
		{Role: "assistant", Blocks: []ExecutionBlock{{Type: ExecutionBlockTypeToolCall, ToolCallID: "call-1", ToolName: "read_file", Input: json.RawMessage(`{"path":"a.go"}`)}}},
	}

	msgs, err := toAgenticMessages("system prompt", history)
	if err != nil {
		t.Fatalf("toAgenticMessages returned error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected system plus one non-empty assistant message, got %#v", msgs)
	}
	if len(msgs[1].ContentBlocks) != 1 || msgs[1].ContentBlocks[0].FunctionToolCall == nil || msgs[1].ContentBlocks[0].FunctionToolCall.Name != "read_file" {
		t.Fatalf("expected non-empty assistant tool call to remain, got %#v", msgs[1])
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

func TestExecuteToolCallsForRoundRunsSafeReadsInParallel(t *testing.T) {
	registry := NewToolRegistry(nil)
	registry.tools["read_file"] = func(ctx *ExecutionContext, input json.RawMessage) (string, error) {
		time.Sleep(200 * time.Millisecond)
		return "read-one", nil
	}
	registry.tools["list_directory"] = func(ctx *ExecutionContext, input json.RawMessage) (string, error) {
		time.Sleep(200 * time.Millisecond)
		return "read-two", nil
	}

	execCtx := &ExecutionContext{Context: context.Background()}
	toolCalls := []ExecutionBlock{
		{Type: ExecutionBlockTypeToolCall, ToolCallID: "call-1", ToolName: "read_file", Input: json.RawMessage(`{"path":"a.go"}`)},
		{Type: ExecutionBlockTypeToolCall, ToolCallID: "call-2", ToolName: "list_directory", Input: json.RawMessage(`{"path":"."}`)},
	}

	start := time.Now()
	results := executeToolCallsForRound(execCtx, registry, toolCalls, "", nil)
	elapsed := time.Since(start)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if elapsed >= 350*time.Millisecond {
		t.Fatalf("expected parallel execution, took %v", elapsed)
	}
}

func TestCanExecuteToolCallsInParallelRejectsMutations(t *testing.T) {
	if canExecuteToolCallsInParallel([]ExecutionBlock{
		{ToolName: "read_file"},
		{ToolName: "write_file"},
	}) {
		t.Fatal("expected mixed read/write tool calls not to run in parallel")
	}
}

func TestExecuteToolCallsForRoundEmitsSequentialToolEventsWhenParallelDisabled(t *testing.T) {
	registry := NewToolRegistry(nil)
	registry.tools["read_file"] = func(ctx *ExecutionContext, input json.RawMessage) (string, error) {
		time.Sleep(25 * time.Millisecond)
		return "read-one", nil
	}
	registry.tools["write_file"] = func(ctx *ExecutionContext, input json.RawMessage) (string, error) {
		return "write-one", nil
	}

	execCtx := &ExecutionContext{Context: context.Background()}
	toolCalls := []ExecutionBlock{
		{Type: ExecutionBlockTypeToolCall, ToolCallID: "call-1", ToolName: "read_file", Input: json.RawMessage(`{"path":"a.go"}`)},
		{Type: ExecutionBlockTypeToolCall, ToolCallID: "call-2", ToolName: "write_file", Input: json.RawMessage(`{"path":"a.go","content":"updated"}`)},
	}

	var events []string
	results := executeToolCallsForRound(execCtx, registry, toolCalls, "assistant-1", func(event ExecutionEvent) {
		events = append(events, fmt.Sprintf("%s:%s", event.Type, event.ToolCallID))
	})

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	want := []string{
		"tool_call_started:call-1",
		"tool_call_args_delta:call-1",
		"tool_call_result:call-1",
		"tool_call_finished:call-1",
		"tool_call_started:call-2",
		"tool_call_args_delta:call-2",
		"tool_call_result:call-2",
		"tool_call_finished:call-2",
	}
	if len(events) != len(want) {
		t.Fatalf("unexpected event count %v", events)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("unexpected event order %v", events)
		}
	}
}

func TestExecuteToolCallsForRoundStopsAfterHumanInteractionTool(t *testing.T) {
	registry := NewToolRegistry(nil)
	registry.tools[ToolRequestReviewCheckpoint] = func(ctx *ExecutionContext, input json.RawMessage) (string, error) {
		return `{"status":"paused","pause_reason":"human_approval"}`, nil
	}
	registry.tools["write_file"] = func(ctx *ExecutionContext, input json.RawMessage) (string, error) {
		t.Fatal("write_file should not execute after request_review_checkpoint in the same round")
		return "", nil
	}

	execCtx := &ExecutionContext{Context: context.Background()}
	toolCalls := []ExecutionBlock{
		{Type: ExecutionBlockTypeToolCall, ToolCallID: "call-1", ToolName: ToolRequestReviewCheckpoint, Input: json.RawMessage(`{"phase":"prd","title":"PRD Review"}`)},
		{Type: ExecutionBlockTypeToolCall, ToolCallID: "call-2", ToolName: "write_file", Input: json.RawMessage(`{"path":"a.go","content":"updated"}`)},
	}

	results := executeToolCallsForRound(execCtx, registry, toolCalls, "assistant-1", nil)
	if len(results) != 1 {
		t.Fatalf("expected execution to stop after the human interaction tool, got %#v", results)
	}
	if results[0].ToolName != ToolRequestReviewCheckpoint {
		t.Fatalf("expected only request_review_checkpoint to execute, got %#v", results)
	}
}

func TestChunkReasoningDeltaReadsReasoningPartsAndSignature(t *testing.T) {
	chunk := &schema.Message{
		Role:             schema.Assistant,
		ReasoningContent: "Think ",
		AssistantGenMultiContent: []schema.MessageOutputPart{
			{
				Type: schema.ChatMessagePartTypeReasoning,
				Reasoning: &schema.MessageOutputReasoning{
					Text:      "more.",
					Signature: "sig_123",
				},
			},
		},
	}

	reasoning := chunkReasoningDelta(chunk)
	if reasoning.Text != "Think more." {
		t.Fatalf("expected reasoning text to be combined, got %#v", reasoning)
	}
	if reasoning.EncryptedValue != "sig_123" {
		t.Fatalf("expected reasoning signature, got %#v", reasoning)
	}
}

func TestExtractReasoningFromAgenticMessageReadsReasoningBlocks(t *testing.T) {
	msg := &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeAssistant,
		ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlock(&schema.Reasoning{Text: "Step 1: ", Signature: "sig_a"}),
			schema.NewContentBlock(&schema.Reasoning{Text: "analyze."}),
			schema.NewContentBlock(&schema.AssistantGenText{Text: "Final answer"}),
		},
	}

	reasoning := extractReasoningFromAgenticMessage(msg)
	if reasoning.Text != "Step 1: analyze." {
		t.Fatalf("expected reasoning text, got %#v", reasoning)
	}
	if reasoning.EncryptedValue != "sig_a" {
		t.Fatalf("expected reasoning signature, got %#v", reasoning)
	}
}

func TestSchemaResponseUsageCarriesCachedInputTokens(t *testing.T) {
	msg := &schema.Message{
		ResponseMeta: &schema.ResponseMeta{
			Usage: &schema.TokenUsage{
				PromptTokens: 12,
				PromptTokenDetails: schema.PromptTokenDetails{
					CachedTokens: 5,
				},
				CompletionTokens: 7,
			},
		},
	}

	usage := ExecutionUsage{}
	if msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
		usage.CachedInputTokens = msg.ResponseMeta.Usage.PromptTokenDetails.CachedTokens
		usage.InputTokens = msg.ResponseMeta.Usage.PromptTokens
		usage.OutputTokens = msg.ResponseMeta.Usage.CompletionTokens
	}

	if usage.CachedInputTokens != 5 || usage.InputTokens != 12 || usage.OutputTokens != 7 {
		t.Fatalf("unexpected usage %+v", usage)
	}
}

func TestAgenticResponseUsageCarriesCachedInputTokens(t *testing.T) {
	msg := &schema.AgenticMessage{
		ResponseMeta: &schema.AgenticResponseMeta{
			TokenUsage: &schema.TokenUsage{
				PromptTokens: 18,
				PromptTokenDetails: schema.PromptTokenDetails{
					CachedTokens: 9,
				},
				CompletionTokens: 6,
			},
		},
	}

	usage := ExecutionUsage{}
	if msg.ResponseMeta != nil && msg.ResponseMeta.TokenUsage != nil {
		usage.CachedInputTokens = msg.ResponseMeta.TokenUsage.PromptTokenDetails.CachedTokens
		usage.InputTokens = msg.ResponseMeta.TokenUsage.PromptTokens
		usage.OutputTokens = msg.ResponseMeta.TokenUsage.CompletionTokens
	}

	if usage.CachedInputTokens != 9 || usage.InputTokens != 18 || usage.OutputTokens != 6 {
		t.Fatalf("unexpected usage %+v", usage)
	}
}
