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

func TestDefaultNativeMaxTokensSupportsLargePlannerToolCalls(t *testing.T) {
	for _, provider := range []string{
		model.AgentModelProviderAnthropic,
		model.AgentModelProviderOpenAI,
		model.AgentModelProviderOpenRouter,
		model.AgentModelProviderOpenRouterResponses,
	} {
		if got := defaultNativeMaxTokensForProvider(provider); got < 16000 {
			t.Fatalf("expected large native output budget for %q, got %d", provider, got)
		}
	}
}

func TestResolveOpenAIResponsesBaseURLDefaultsToOpenAIAPI(t *testing.T) {
	if got := resolveOpenAIResponsesBaseURL(""); got != defaultOpenAIResponsesBaseURL {
		t.Fatalf("expected default openai responses base url %q, got %q", defaultOpenAIResponsesBaseURL, got)
	}
	if got := resolveOpenAIResponsesBaseURL("  "); got != defaultOpenAIResponsesBaseURL {
		t.Fatalf("expected blank openai responses base url to fall back to %q, got %q", defaultOpenAIResponsesBaseURL, got)
	}
}

func TestResolveOpenAIResponsesBaseURLPreservesExplicitValue(t *testing.T) {
	want := "https://proxy.example/v1"
	if got := resolveOpenAIResponsesBaseURL("  " + want + "  "); got != want {
		t.Fatalf("expected explicit openai responses base url %q, got %q", want, got)
	}
}

func TestResolveOpenRouterBaseURLDefaultsToOpenRouterAPI(t *testing.T) {
	if got := resolveOpenRouterBaseURL(""); got != defaultOpenRouterBaseURL {
		t.Fatalf("expected default openrouter base url %q, got %q", defaultOpenRouterBaseURL, got)
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

func TestContinuationAgenticOptionsEmptyWithoutResponseID(t *testing.T) {
	if opts := continuationAgenticOptions(""); len(opts) != 0 {
		t.Fatalf("expected no continuation options without response id, got %#v", opts)
	}
}

func TestNextAgenticStepStateUsesIncrementalToolResultsForContinuation(t *testing.T) {
	currentMessages := []*schema.AgenticMessage{
		schema.UserAgenticMessage("latest human reply"),
	}
	assistantMsg := &schema.AgenticMessage{Role: schema.AgenticRoleTypeAssistant}
	toolResultMessages := []*schema.AgenticMessage{
		functionToolResultAgenticMessage("call-1", "list_documents", "{\"ok\":true}"),
	}

	nextResponseID, nextMessages := nextAgenticStepState(
		currentMessages,
		assistantMsg,
		toolResultMessages,
		"resp_old",
		&ProviderContinuation{ResponseID: "resp_new"},
	)

	if nextResponseID != "resp_new" {
		t.Fatalf("expected continuation response id to advance, got %q", nextResponseID)
	}
	if len(nextMessages) != 1 {
		t.Fatalf("expected only incremental tool-result messages, got %#v", nextMessages)
	}
	if nextMessages[0].Role != schema.AgenticRoleTypeUser {
		t.Fatalf("expected tool result message role, got %#v", nextMessages[0])
	}
}

func TestNextAgenticStepStateAppendsAssistantAndToolsWithoutContinuation(t *testing.T) {
	currentMessages := []*schema.AgenticMessage{
		schema.UserAgenticMessage("latest human reply"),
	}
	assistantMsg := &schema.AgenticMessage{Role: schema.AgenticRoleTypeAssistant}
	toolResultMessages := []*schema.AgenticMessage{
		functionToolResultAgenticMessage("call-1", "list_documents", "{\"ok\":true}"),
	}

	nextResponseID, nextMessages := nextAgenticStepState(
		currentMessages,
		assistantMsg,
		toolResultMessages,
		"",
		nil,
	)

	if nextResponseID != "" {
		t.Fatalf("expected no continuation response id, got %q", nextResponseID)
	}
	if len(nextMessages) != 3 {
		t.Fatalf("expected transcript-style append behavior, got %#v", nextMessages)
	}
}

func TestNextAgenticStepStateDoesNotDuplicateAssistantWithoutContinuation(t *testing.T) {
	currentMessages := []*schema.AgenticMessage{
		schema.UserAgenticMessage("latest human reply"),
	}
	assistantMsg := &schema.AgenticMessage{Role: schema.AgenticRoleTypeAssistant}
	toolResultMessages := []*schema.AgenticMessage{
		functionToolResultAgenticMessage("call-1", "list_documents", "{\"ok\":true}"),
	}

	_, nextMessages := nextAgenticStepState(
		currentMessages,
		assistantMsg,
		toolResultMessages,
		"",
		nil,
	)

	assistantCount := 0
	for _, msg := range nextMessages {
		if msg.Role == schema.AgenticRoleTypeAssistant {
			assistantCount++
		}
	}
	if assistantCount != 1 {
		t.Fatalf("expected exactly one assistant message, got %d in %#v", assistantCount, nextMessages)
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

func TestBuildAgenticReplayMessagesUsesOpenAIContinuationDeltaOnly(t *testing.T) {
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
		{Role: "user", Content: "continue with the implementation"},
	}

	msgs, err := buildAgenticReplayMessages(model.AgentModelProviderOpenAI, "", history, "repair and continue", true)
	if err != nil {
		t.Fatalf("buildAgenticReplayMessages returned error: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected turn-local message plus tool result and user delta, got %#v", msgs)
	}
	if msgs[0].Role != schema.AgenticRoleTypeUser {
		t.Fatalf("expected turn-local instructions as user message, got %#v", msgs[0])
	}
	if msgs[1].Role != schema.AgenticRoleTypeUser || len(msgs[1].ContentBlocks) != 1 || msgs[1].ContentBlocks[0].FunctionToolResult == nil {
		t.Fatalf("expected tool result delta message, got %#v", msgs[1])
	}
	if msgs[2].Role != schema.AgenticRoleTypeUser {
		t.Fatalf("expected latest human reply to remain, got %#v", msgs[2])
	}
}

func TestBuildAgenticReplayMessagesKeepsGenericReplayForOpenRouter(t *testing.T) {
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

	msgs, err := buildAgenticReplayMessages(model.AgentModelProviderOpenRouter, "", history, "", true)
	if err != nil {
		t.Fatalf("buildAgenticReplayMessages returned error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected assistant/tool replay pair for non-OpenAI continuation path, got %#v", msgs)
	}
	if msgs[0].Role != schema.AgenticRoleTypeAssistant {
		t.Fatalf("expected assistant replay message, got %#v", msgs[0])
	}
	if msgs[1].Role != schema.AgenticRoleTypeUser || len(msgs[1].ContentBlocks) != 1 || msgs[1].ContentBlocks[0].FunctionToolResult == nil {
		t.Fatalf("expected tool result replay message, got %#v", msgs[1])
	}
}

func TestToSchemaMessagesIncludesTurnLocalInstructionsWithoutMutatingHistory(t *testing.T) {
	history := []ExecutionMessage{
		{Role: "user", Content: "human reply"},
	}

	msgs, err := toSchemaMessagesWithTurnLocalInstructions("system prompt", history, "use the active contract only")
	if err != nil {
		t.Fatalf("toSchemaMessagesWithTurnLocalInstructions returned error: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected system prompt, execution-local message, and history message, got %#v", msgs)
	}
	if msgs[1].Role != schema.User || !strings.Contains(msgs[1].Content, "Execution-local instructions for this turn only") {
		t.Fatalf("expected execution-local user message, got %#v", msgs[1])
	}
	if msgs[2].Role != schema.User || msgs[2].Content != "human reply" {
		t.Fatalf("expected original history message to remain after execution-local instructions, got %#v", msgs[2])
	}
	if len(history) != 1 || history[0].Content != "human reply" {
		t.Fatalf("expected original history slice to remain unchanged, got %#v", history)
	}
}

func TestToAgenticMessagesIncludesTurnLocalInstructionsWithoutMutatingHistory(t *testing.T) {
	history := []ExecutionMessage{
		{Role: "user", Content: "human reply"},
	}

	msgs, err := toAgenticMessagesWithTurnLocalInstructions("system prompt", history, "repair the failed contract and continue")
	if err != nil {
		t.Fatalf("toAgenticMessagesWithTurnLocalInstructions returned error: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected system prompt, execution-local message, and history message, got %#v", msgs)
	}
	if msgs[1].Role != schema.AgenticRoleTypeUser {
		t.Fatalf("expected execution-local user agentic message, got %#v", msgs[1])
	}
	executionLocalJSON, err := json.Marshal(msgs[1])
	if err != nil {
		t.Fatalf("marshal execution-local agentic message: %v", err)
	}
	if !strings.Contains(string(executionLocalJSON), "Execution-local instructions for this turn only") {
		t.Fatalf("expected execution-local instruction marker, got %s", string(executionLocalJSON))
	}
	if msgs[2].Role != schema.AgenticRoleTypeUser {
		t.Fatalf("expected original history message to remain after execution-local instructions, got %#v", msgs[2])
	}
	historyJSON, err := json.Marshal(msgs[2])
	if err != nil {
		t.Fatalf("marshal history agentic message: %v", err)
	}
	if !strings.Contains(string(historyJSON), "human reply") {
		t.Fatalf("expected original history text to remain after execution-local instructions, got %s", string(historyJSON))
	}
	if len(history) != 1 || history[0].Content != "human reply" {
		t.Fatalf("expected original history slice to remain unchanged, got %#v", history)
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

func TestAnalyzeToolOutputForModelHonorsBoundedJSONCompactionExemption(t *testing.T) {
	output := `{"_helpin_compaction":{"exempt":true,"max_runes":30000,"mode":"bounded_index"},"rows":["` + strings.Repeat("x", 9000) + `"]}`

	analysis := analyzeToolOutputForModel("scan_trivy", output)

	if analysis.Compacted {
		t.Fatalf("expected bounded JSON to avoid compaction, got %#v", analysis)
	}
	if analysis.Content != output {
		t.Fatal("expected original output to be preserved")
	}
}

func TestAnalyzeToolOutputForModelIgnoresOverCapCompactionExemption(t *testing.T) {
	output := `{"_helpin_compaction":{"exempt":true,"max_runes":30000,"mode":"bounded_index"},"rows":["` + strings.Repeat("x", 31000) + `"]}`

	analysis := analyzeToolOutputForModel("scan_trivy", output)

	if !analysis.Compacted {
		t.Fatalf("expected over-cap bounded JSON to compact, got %#v", analysis)
	}
	if !strings.Contains(analysis.Content, "truncated") {
		t.Fatalf("expected compaction marker, got %q", analysis.Content)
	}
}

func TestAnalyzeToolOutputForModelCompactsModerateReadFileOutputs(t *testing.T) {
	output := strings.Repeat("0123456789abcdef\n", 210)

	analysis := analyzeToolOutputForModel("read_file", output)

	if !analysis.Compacted {
		t.Fatalf("expected moderate read_file output to compact under the tighter threshold, got %#v", analysis)
	}
	if analysis.OriginalRunes <= analysis.VisibleRunes {
		t.Fatalf("expected visible output to be smaller, got %#v", analysis)
	}
}

func TestAnalyzeToolOutputForModelCompactsModerateReadFileRangeOutputs(t *testing.T) {
	output := strings.Repeat("0123456789abcdef\n", 180)

	analysis := analyzeToolOutputForModel("read_file_range", output)

	if !analysis.Compacted {
		t.Fatalf("expected moderate read_file_range output to compact under the tighter threshold, got %#v", analysis)
	}
	if analysis.OriginalRunes <= analysis.VisibleRunes {
		t.Fatalf("expected visible output to be smaller, got %#v", analysis)
	}
}

func TestAnalyzeToolOutputForModelCompactsLargeRipgrepOutputs(t *testing.T) {
	output := strings.Repeat("path/file.rs:123: matched text here\n", 100)

	analysis := analyzeToolOutputForModel("ripgrep", output)

	if !analysis.Compacted {
		t.Fatalf("expected 3k+ ripgrep output to compact, got %#v", analysis)
	}
	if analysis.OriginalRunes <= analysis.VisibleRunes {
		t.Fatalf("expected visible output to be smaller, got %#v", analysis)
	}
}

func TestPrepareToolResultForModelUsesPlaceholderForEmptyOutput(t *testing.T) {
	analysis := prepareToolResultForModel("run_command", "", false)

	if analysis.Content != toolResultNoOutputPlaceholder {
		t.Fatalf("expected no-output placeholder, got %q", analysis.Content)
	}
	if analysis.VisibleRunes == 0 || analysis.VisibleLines == 0 {
		t.Fatalf("expected visible metrics for placeholder, got %#v", analysis)
	}
}

func TestToSchemaMessagesCompactsLargeToolResultsForModelHistory(t *testing.T) {
	output := strings.Repeat("line of file contents\n", 500)
	history := []ExecutionMessage{
		{
			Role: "assistant",
			Blocks: []ExecutionBlock{
				{Type: ExecutionBlockTypeToolCall, ToolCallID: "call-1", ToolName: "read_file", Input: json.RawMessage(`{"path":"a.go"}`)},
			},
		},
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
	if len(msgs) != 3 {
		t.Fatalf("expected system plus assistant/tool replay messages, got %#v", msgs)
	}
	if !strings.Contains(msgs[2].Content, "truncated") {
		t.Fatalf("expected compacted tool output marker, got %q", msgs[2].Content)
	}
	if msgs[2].Content == output {
		t.Fatal("expected tool output to be compacted for model history")
	}
}

func TestToAgenticMessagesCompactsLargeToolResultsForModelHistory(t *testing.T) {
	output := strings.Repeat("line of file contents\n", 500)
	history := []ExecutionMessage{
		{
			Role: "assistant",
			Blocks: []ExecutionBlock{
				{Type: ExecutionBlockTypeToolCall, ToolCallID: "call-1", ToolName: "read_file", Input: json.RawMessage(`{"path":"a.go"}`)},
			},
		},
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
	if len(msgs) != 3 {
		t.Fatalf("expected system plus assistant/tool replay messages, got %#v", msgs)
	}
	if len(msgs[2].ContentBlocks) != 1 || msgs[2].ContentBlocks[0].FunctionToolResult == nil {
		t.Fatalf("expected function tool result block, got %#v", msgs[2])
	}
	resultBlocks := msgs[2].ContentBlocks[0].FunctionToolResult.Content
	if len(resultBlocks) != 1 || resultBlocks[0].Text == nil {
		t.Fatalf("expected text function tool result content, got %#v", resultBlocks)
	}
	result := resultBlocks[0].Text.Text
	if !strings.Contains(result, "truncated") {
		t.Fatalf("expected compacted tool output marker, got %q", result)
	}
	if result == output {
		t.Fatal("expected tool output to be compacted for agentic model history")
	}
}

func TestToSchemaMessagesPreservesEmptyToolResultsWithPlaceholder(t *testing.T) {
	history := []ExecutionMessage{
		{
			Role: "assistant",
			Blocks: []ExecutionBlock{
				{Type: ExecutionBlockTypeToolCall, ToolCallID: "call-1", ToolName: "run_command", Input: json.RawMessage(`{"command":"mkdir -p tmp"}`)},
			},
		},
		{
			Role: "tool",
			Blocks: []ExecutionBlock{
				{Type: ExecutionBlockTypeToolResult, ToolCallID: "call-1", ToolName: "run_command", Output: ""},
			},
		},
	}

	msgs, err := toSchemaMessages("system prompt", history)
	if err != nil {
		t.Fatalf("toSchemaMessages returned error: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected system plus assistant/tool replay messages, got %#v", msgs)
	}
	if msgs[2].Content != toolResultNoOutputPlaceholder {
		t.Fatalf("expected placeholder tool content, got %#v", msgs[2])
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

func TestToSchemaMessagesDropsOrphanToolResultsFromReplayHistory(t *testing.T) {
	history := []ExecutionMessage{
		{Role: "user", Content: "Resume context from earlier turns:\nSummary"},
		{
			Role: "tool",
			Blocks: []ExecutionBlock{
				{Type: ExecutionBlockTypeToolResult, ToolCallID: "call-1", ToolName: "read_file", Output: "package main"},
			},
		},
	}

	msgs, err := toSchemaMessages("system prompt", history)
	if err != nil {
		t.Fatalf("toSchemaMessages returned error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected system plus summary user message after dropping orphan tool result, got %#v", msgs)
	}
}

func TestToSchemaMessagesKeepsMatchedToolReplayPairs(t *testing.T) {
	history := []ExecutionMessage{
		{Role: "user", Content: "Resume context from earlier turns:\nSummary"},
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

	msgs, err := toSchemaMessages("system prompt", history)
	if err != nil {
		t.Fatalf("toSchemaMessages returned error: %v", err)
	}
	if len(msgs) != 4 {
		t.Fatalf("expected system, summary, assistant, and tool result messages, got %#v", msgs)
	}
	if len(msgs[2].ToolCalls) != 1 || msgs[2].ToolCalls[0].ID != "call-1" {
		t.Fatalf("expected matched tool call to remain, got %#v", msgs[2])
	}
}

func TestSummarizeNativeToolPressureAggregatesLargeReads(t *testing.T) {
	messages := []ExecutionMessage{
		{
			Role: "tool",
			Blocks: []ExecutionBlock{
				{Type: ExecutionBlockTypeToolResult, ToolCallID: "call-1", ToolName: "read_file", Output: strings.Repeat("abc\n", 1600)},
			},
		},
		{
			Role: "tool",
			Blocks: []ExecutionBlock{
				{Type: ExecutionBlockTypeToolResult, ToolCallID: "call-2", ToolName: "run_command", Output: "ok\n"},
			},
		},
	}

	summary := summarizeNativeToolPressure(messages)
	if summary.ToolCalls != 2 {
		t.Fatalf("expected 2 tool calls, got %#v", summary)
	}
	if summary.CompactedResults != 1 {
		t.Fatalf("expected one compacted result, got %#v", summary)
	}
	if summary.OutputChars <= summary.ModelVisibleChars {
		t.Fatalf("expected visible chars to be smaller after compaction, got %#v", summary)
	}
	if len(summary.TopToolsByPressure) == 0 || !strings.Contains(summary.TopToolsByPressure[0], "read_file") {
		t.Fatalf("expected read_file to be the top noisy tool, got %#v", summary)
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

func TestToEinoToolInfosPreservesNestedRequiredFields(t *testing.T) {
	registry := NewToolRegistry(nil)
	defs := registry.DefinitionsFor(map[string]bool{ToolPublishTaskPlan: true})
	infos, err := toEinoToolInfos(defs)
	if err != nil {
		t.Fatalf("toEinoToolInfos returned error: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected one tool info, got %d", len(infos))
	}

	toolSchema, err := infos[0].ParamsOneOf.ToJSONSchema()
	if err != nil {
		t.Fatalf("ToJSONSchema returned error: %v", err)
	}
	encoded, _ := json.Marshal(toolSchema)
	schemaJSON := string(encoded)
	for _, snippet := range []string{
		`"required":["content"]`,
		`"required":["proposed_tasks","summary"]`,
		`"required":["acceptance_criteria","dependency_refs","description","name","task_type"]`,
	} {
		if !strings.Contains(schemaJSON, snippet) {
			t.Fatalf("expected converted schema to contain %s, got %s", snippet, schemaJSON)
		}
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
