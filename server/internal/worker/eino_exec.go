package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	agenticopenai "github.com/cloudwego/eino-ext/components/model/agenticopenai"
	einoclaude "github.com/cloudwego/eino-ext/components/model/claude"
	openaiacl "github.com/cloudwego/eino-ext/libs/acl/openai"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	appmodel "github.com/helpin-ai/helpin/server/internal/model"
)

const (
	ExecutionBlockTypeText       = "text"
	ExecutionBlockTypeToolCall   = "tool_call"
	ExecutionBlockTypeToolResult = "tool_result"

	modelVisibleToolOutputMaxRunes     = 4_000
	modelVisibleToolOutputHeadRunes    = 2_700
	modelVisibleToolOutputTailRunes    = 800
	modelVisibleToolOutputMaxSmallTool = 8_000

	modelVisibleFileReadOutputMaxRunes  = 2_800
	modelVisibleFileReadOutputHeadRunes = 1_800
	modelVisibleFileReadOutputTailRunes = 600

	modelVisibleReadRangeOutputMaxRunes  = 2_600
	modelVisibleReadRangeOutputHeadRunes = 1_700
	modelVisibleReadRangeOutputTailRunes = 500

	modelVisibleRipgrepOutputMaxRunes  = 3_000
	modelVisibleRipgrepOutputHeadRunes = 2_100
	modelVisibleRipgrepOutputTailRunes = 500

	boundedToolOutputCompactionMaxRunes = 30_000

	toolResultNoOutputPlaceholder = "[tool returned no output]"
)

var (
	ErrMaxToolStepsReached    = errors.New("agent reached max tool steps")
	ErrInitialResponseTimeout = errors.New("initial_response_timeout")
)

const (
	defaultOpenAIResponsesBaseURL = "https://api.openai.com/v1"
	defaultOpenRouterBaseURL      = "https://openrouter.ai/api/v1"
)

type ExecutionBlock struct {
	Type       string          `json:"type"`
	Text       string          `json:"text,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	ToolName   string          `json:"tool_name,omitempty"`
	Input      json.RawMessage `json:"input,omitempty"`
	Output     string          `json:"output,omitempty"`
	IsError    bool            `json:"is_error,omitempty"`
}

type ExecutionMessage struct {
	SequenceNo int              `json:"sequence_no,omitempty"`
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	Blocks     []ExecutionBlock `json:"blocks,omitempty"`
}

type ExecutionUsage struct {
	// CachedInputTokens is a provider-reported subset of InputTokens. Providers
	// that do not expose prompt caching should leave this at zero.
	CachedInputTokens int `json:"cached_input_tokens"`
	InputTokens       int `json:"input_tokens"`
	OutputTokens      int `json:"output_tokens"`
}

type ExecutionEvent struct {
	Type            string
	MessageID       string
	ParentMessageID string
	Text            string
	Content         string
	ToolCallID      string
	ToolName        string
	ToolInput       string
	ArgsDelta       string
	ArgsText        string
	ResultMessageID string
	ActivityID      string
	ActivityType    string
	EncryptedValue  string
	OutputSummary   string
	DurationMs      int64
	Error           string
}

type executionReasoning struct {
	Text           string
	EncryptedValue string
}

type ExecutionResult struct {
	Messages              []ExecutionMessage
	AssistantBlocks       []ExecutionBlock
	AssistantText         string
	ToolInvocations       []appmodel.ToolInvocation
	CodexAuthState        *appmodel.CodexAuthState
	Usage                 ExecutionUsage
	ProviderContinuation  *ProviderContinuation
	HumanInputMetadata    json.RawMessage
	HumanApprovalMetadata json.RawMessage
	CodexAuthMetadata     json.RawMessage
	RunPlanMetadata       json.RawMessage
	MaxStepsReached       bool
}

type modelVisibleToolOutput struct {
	Content       string
	OriginalRunes int
	VisibleRunes  int
	OriginalLines int
	VisibleLines  int
	Compacted     bool
}

type helpinCompactionHint struct {
	Exempt   bool   `json:"exempt"`
	MaxRunes int    `json:"max_runes,omitempty"`
	Mode     string `json:"mode,omitempty"`
}

type executedToolCall struct {
	ToolCallID string
	ToolName   string
	Input      json.RawMessage
	Output     string
	Duration   time.Duration
	IsError    bool
}

type EinoModelFactory struct {
	AnthropicAPIKey string
	OpenAIAPIKey    string
	OpenAIBaseURL   string
	OpenRouterKey   string
	OpenRouterURL   string
}

func (f *EinoModelFactory) Resolve(ctx context.Context, agent *appmodel.Agent, tools []ToolDefinition) (einomodel.ToolCallingChatModel, string, error) {
	provider, modelName := resolveProviderAndModel(agent)
	baseModel, err := f.resolveBaseModel(ctx, provider, modelName)
	if err != nil {
		return nil, "", err
	}
	toolInfos, err := toEinoToolInfos(tools)
	if err != nil {
		return nil, "", err
	}
	if len(toolInfos) == 0 {
		withTools, err := baseModel.WithTools(nil)
		return withTools, modelName, err
	}
	withTools, err := baseModel.WithTools(toolInfos)
	if err != nil {
		return nil, "", err
	}
	return withTools, modelName, nil
}

func (f *EinoModelFactory) ResolveAgentic(ctx context.Context, agent *appmodel.Agent, tools []ToolDefinition) (einomodel.AgenticModel, string, []einomodel.Option, error) {
	provider, modelName := resolveProviderAndModel(agent)
	baseModel, err := f.resolveAgenticBaseModel(ctx, provider, modelName)
	if err != nil {
		return nil, "", nil, err
	}
	toolInfos, err := toEinoToolInfos(tools)
	if err != nil {
		return nil, "", nil, err
	}
	if len(toolInfos) == 0 {
		return baseModel, modelName, nil, nil
	}
	return baseModel, modelName, []einomodel.Option{einomodel.WithTools(toolInfos)}, nil
}

func (f *EinoModelFactory) resolveBaseModel(ctx context.Context, provider, modelName string) (einomodel.ToolCallingChatModel, error) {
	switch provider {
	case "anthropic":
		if strings.TrimSpace(f.AnthropicAPIKey) == "" {
			return nil, fmt.Errorf("anthropic API key is not configured")
		}
		return einoclaude.NewChatModel(ctx, &einoclaude.Config{
			APIKey:    f.AnthropicAPIKey,
			Model:     modelName,
			MaxTokens: defaultNativeMaxTokensForProvider(provider),
		})
	default:
		if providerUsesAgenticResponses(provider) {
			return nil, fmt.Errorf("provider %q must use the Responses-based agentic model path", provider)
		}
		return nil, fmt.Errorf("unsupported provider %q", provider)
	}
}

func (f *EinoModelFactory) resolveAgenticBaseModel(ctx context.Context, provider, modelName string) (einomodel.AgenticModel, error) {
	switch provider {
	case appmodel.AgentModelProviderOpenAI:
		if strings.TrimSpace(f.OpenAIAPIKey) == "" {
			return nil, fmt.Errorf("openai API key is not configured")
		}
		maxTokens := defaultNativeMaxTokensForProvider(provider)
		return agenticopenai.NewResponsesModel(ctx, &agenticopenai.ResponsesConfig{
			APIKey:    f.OpenAIAPIKey,
			BaseURL:   resolveOpenAIResponsesBaseURL(f.OpenAIBaseURL),
			Model:     modelName,
			MaxTokens: &maxTokens,
		})
	case appmodel.AgentModelProviderOpenRouter, appmodel.AgentModelProviderOpenRouterResponses:
		if strings.TrimSpace(f.OpenRouterKey) == "" {
			return nil, fmt.Errorf("openrouter API key is not configured")
		}
		maxTokens := defaultNativeMaxTokensForProvider(provider)
		return agenticopenai.NewResponsesModel(ctx, &agenticopenai.ResponsesConfig{
			APIKey:    f.OpenRouterKey,
			BaseURL:   resolveOpenRouterBaseURL(f.OpenRouterURL),
			Model:     modelName,
			MaxTokens: &maxTokens,
		})
	default:
		return nil, fmt.Errorf("provider %q does not support the Responses-based agentic model path", provider)
	}
}

func resolveOpenAIResponsesBaseURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return defaultOpenAIResponsesBaseURL
	}
	return baseURL
}

func resolveOpenRouterBaseURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return defaultOpenRouterBaseURL
	}
	return baseURL
}

func resolveProviderAndModel(agent *appmodel.Agent) (string, string) {
	provider := appmodel.AgentModelProviderAnthropic
	if agent != nil && agent.Provider != nil && strings.TrimSpace(*agent.Provider) != "" {
		provider = strings.TrimSpace(*agent.Provider)
	}
	modelName := ""
	if agent != nil && agent.Model != nil {
		modelName = strings.TrimSpace(*agent.Model)
	}
	if modelName == "" {
		modelName = defaultModelForProvider(provider)
	}
	return provider, modelName
}

func providerUsesAgenticResponses(provider string) bool {
	switch strings.TrimSpace(provider) {
	case appmodel.AgentModelProviderOpenAI, appmodel.AgentModelProviderOpenRouter, appmodel.AgentModelProviderOpenRouterResponses:
		return true
	default:
		return false
	}
}

func ProviderSupportsResponseContinuation(provider string) bool {
	// OpenAI Responses continuation is stricter than OpenRouter-compatible
	// replay: when previous_response_id is used, Eino can preserve provider
	// output item IDs on self-generated blocks, and OpenAI rejects those as
	// duplicate input items on resumed/tool-followup turns. Keep OpenAI on full
	// sanitized transcript replay until the adapter exposes a clean delta-only
	// continuation transport.
	return false
}

func defaultModelForProvider(provider string) string {
	switch provider {
	case appmodel.AgentModelProviderOpenAI:
		return "gpt-4.1"
	case appmodel.AgentModelProviderOpenRouter, appmodel.AgentModelProviderOpenRouterResponses:
		return "openai/gpt-4.1"
	default:
		return "claude-sonnet-4-6"
	}
}

func defaultNativeMaxTokensForProvider(provider string) int {
	switch provider {
	case appmodel.AgentModelProviderAnthropic, appmodel.AgentModelProviderOpenAI, appmodel.AgentModelProviderOpenRouter, appmodel.AgentModelProviderOpenRouterResponses:
		return 16384
	default:
		return 0
	}
}

func ExecuteWithEino(
	ctx context.Context,
	factory *EinoModelFactory,
	agent *appmodel.Agent,
	systemPrompt string,
	history []ExecutionMessage,
	tools []ToolDefinition,
	execCtx *ExecutionContext,
	registry *ToolRegistry,
	maxSteps int,
	onEvent func(ExecutionEvent),
	turnLocalInstructions string,
) (*ExecutionResult, error) {
	if factory == nil {
		return nil, fmt.Errorf("eino model factory is not configured")
	}
	if registry == nil {
		return nil, fmt.Errorf("tool registry is not configured")
	}
	if maxSteps <= 0 {
		maxSteps = 25
	}

	provider, _ := resolveProviderAndModel(agent)
	slog.InfoContext(ctx, "native runtime provider route selected",
		"provider", provider,
		"agentic_responses", providerUsesAgenticResponses(provider),
		"history_messages", len(history),
		"tool_count", len(tools),
	)
	if providerUsesAgenticResponses(provider) {
		return executeWithEinoAgentic(ctx, factory, agent, systemPrompt, history, tools, execCtx, registry, maxSteps, onEvent, turnLocalInstructions)
	}

	modelWithTools, _, err := factory.Resolve(ctx, agent, tools)
	if err != nil {
		return nil, err
	}

	messages, err := toSchemaMessagesWithTurnLocalInstructions(systemPrompt, history, turnLocalInstructions)
	if err != nil {
		return nil, err
	}

	result := &ExecutionResult{
		Messages: append([]ExecutionMessage(nil), history...),
	}

	for step := 0; step < maxSteps; step++ {
		assistantMsg, assistantBlocks, usage, assistantMessageID, err := streamAssistantMessage(ctx, modelWithTools, messages, onEvent)
		if err != nil {
			return nil, err
		}
		result.Usage.InputTokens += usage.InputTokens
		result.Usage.OutputTokens += usage.OutputTokens
		result.AssistantBlocks = assistantBlocks
		result.AssistantText = extractTextFromExecutionBlocks(assistantBlocks)
		execCtx.CurrentAssistantText = result.AssistantText
		result.Messages = append(result.Messages, ExecutionMessage{
			Role:    "assistant",
			Content: result.AssistantText,
			Blocks:  assistantBlocks,
		})
		messages = append(messages, assistantMsg)

		if len(assistantMsg.ToolCalls) == 0 {
			return result, nil
		}

		stopAfterToolRound := false
		toolCalls := make([]ExecutionBlock, 0, len(assistantMsg.ToolCalls))
		for _, toolCall := range assistantMsg.ToolCalls {
			toolCalls = append(toolCalls, ExecutionBlock{
				Type:       ExecutionBlockTypeToolCall,
				ToolCallID: toolCall.ID,
				ToolName:   toolCall.Function.Name,
				Input:      normalizeToolArguments(toolCall.Function.Arguments),
			})
		}

		for _, executed := range executeToolCallsForRound(execCtx, registry, toolCalls, assistantMessageID, onEvent) {
			summary := truncate(executed.Output, 500)
			result.ToolInvocations = append(result.ToolInvocations, appmodel.ToolInvocation{
				ToolName:      executed.ToolName,
				Input:         executed.Input,
				OutputSummary: summary,
				DurationMs:    executed.Duration.Milliseconds(),
			})

			result.Messages = append(result.Messages, ExecutionMessage{
				Role:    "tool",
				Content: executed.Output,
				Blocks: []ExecutionBlock{{
					Type:       ExecutionBlockTypeToolResult,
					ToolCallID: executed.ToolCallID,
					ToolName:   executed.ToolName,
					Input:      executed.Input,
					Output:     executed.Output,
					IsError:    executed.IsError,
				}},
			})
			modelVisibleOutput := prepareToolResultForModel(executed.ToolName, executed.Output, executed.IsError)
			logNativeToolResultForModel(ctx, execCtx, executed, modelVisibleOutput)
			messages = append(messages, schema.ToolMessage(modelVisibleOutput.Content, executed.ToolCallID, schema.WithToolName(executed.ToolName)))
			if IsHumanInteractionTool(executed.ToolName) {
				stopAfterToolRound = true
			}
		}
		if stopAfterToolRound {
			return result, nil
		}
	}

	result.MaxStepsReached = true
	return result, ErrMaxToolStepsReached
}

func executeWithEinoAgentic(
	ctx context.Context,
	factory *EinoModelFactory,
	agent *appmodel.Agent,
	systemPrompt string,
	history []ExecutionMessage,
	tools []ToolDefinition,
	execCtx *ExecutionContext,
	registry *ToolRegistry,
	maxSteps int,
	onEvent func(ExecutionEvent),
	turnLocalInstructions string,
) (*ExecutionResult, error) {
	agenticModel, _, agenticOptions, err := factory.ResolveAgentic(ctx, agent, tools)
	if err != nil {
		return nil, err
	}

	provider, _ := resolveProviderAndModel(agent)
	effectiveSystemPrompt := systemPrompt
	effectiveHistory := history
	continuationResponseID := ""
	if continuation := execCtx.ProviderContinuation; continuation != nil && strings.TrimSpace(continuation.ResponseID) != "" && ProviderSupportsResponseContinuation(provider) {
		effectiveSystemPrompt = ""
		effectiveHistory = filterExecutionHistoryAfterSequence(history, continuation.AfterSequenceNo)
		continuationResponseID = strings.TrimSpace(continuation.ResponseID)
		slog.InfoContext(ctx, "native runtime using provider continuation",
			"provider", provider,
			"response_id", continuationResponseID,
			"after_sequence_no", continuation.AfterSequenceNo,
			"effective_history_messages", len(effectiveHistory),
		)
	}

	messages, err := buildAgenticReplayMessages(provider, effectiveSystemPrompt, effectiveHistory, turnLocalInstructions, continuationResponseID != "")
	if err != nil {
		return nil, err
	}

	result := &ExecutionResult{
		Messages: append([]ExecutionMessage(nil), history...),
	}

	for step := 0; step < maxSteps; step++ {
		modelOptions := append([]einomodel.Option{}, agenticOptions...)
		modelOptions = append(modelOptions, continuationAgenticOptions(continuationResponseID)...)
		assistantMsg, assistantBlocks, usage, continuation, assistantMessageID, err := generateAssistantAgenticMessage(ctx, agenticModel, messages, onEvent, modelOptions...)
		if err != nil {
			return nil, err
		}
		result.Usage.InputTokens += usage.InputTokens
		result.Usage.OutputTokens += usage.OutputTokens
		result.AssistantBlocks = assistantBlocks
		result.AssistantText = extractTextFromExecutionBlocks(assistantBlocks)
		execCtx.CurrentAssistantText = result.AssistantText
		result.ProviderContinuation = continuation
		result.Messages = append(result.Messages, ExecutionMessage{
			Role:    "assistant",
			Content: result.AssistantText,
			Blocks:  assistantBlocks,
		})

		toolCalls := executionToolCalls(assistantBlocks)
		if len(toolCalls) == 0 {
			return result, nil
		}

		stopAfterToolRound := false
		toolResultMessages := make([]*schema.AgenticMessage, 0, len(toolCalls))
		for _, executed := range executeToolCallsForRound(execCtx, registry, toolCalls, assistantMessageID, onEvent) {
			summary := truncate(executed.Output, 500)
			result.ToolInvocations = append(result.ToolInvocations, appmodel.ToolInvocation{
				ToolName:      executed.ToolName,
				Input:         executed.Input,
				OutputSummary: summary,
				DurationMs:    executed.Duration.Milliseconds(),
			})

			result.Messages = append(result.Messages, ExecutionMessage{
				Role:    "tool",
				Content: executed.Output,
				Blocks: []ExecutionBlock{{
					Type:       ExecutionBlockTypeToolResult,
					ToolCallID: executed.ToolCallID,
					ToolName:   executed.ToolName,
					Input:      executed.Input,
					Output:     executed.Output,
					IsError:    executed.IsError,
				}},
			})
			modelVisibleOutput := prepareToolResultForModel(executed.ToolName, executed.Output, executed.IsError)
			logNativeToolResultForModel(ctx, execCtx, executed, modelVisibleOutput)
			toolResultMessages = append(toolResultMessages, functionToolResultAgenticMessage(executed.ToolCallID, executed.ToolName, modelVisibleOutput.Content))
			if IsHumanInteractionTool(executed.ToolName) {
				stopAfterToolRound = true
			}
		}
		continuationResponseID, messages = nextAgenticStepState(messages, assistantMsg, toolResultMessages, continuationResponseID, continuation)
		if stopAfterToolRound {
			return result, nil
		}
	}

	result.MaxStepsReached = true
	return result, ErrMaxToolStepsReached
}

func continuationAgenticOptions(responseID string) []einomodel.Option {
	responseID = strings.TrimSpace(responseID)
	if responseID == "" {
		return nil
	}
	return []einomodel.Option{
		agenticopenai.WithExtraFields(map[string]any{
			"previous_response_id": responseID,
		}),
	}
}

func functionToolResultAgenticMessage(callID, name, content string) *schema.AgenticMessage {
	return &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeUser,
		ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlock(&schema.FunctionToolResult{
				CallID: callID,
				Name:   name,
				Content: []*schema.FunctionToolResultContentBlock{{
					Type: schema.FunctionToolResultContentBlockTypeText,
					Text: &schema.UserInputText{Text: content},
				}},
			}),
		},
	}
}

func nextAgenticStepState(
	currentMessages []*schema.AgenticMessage,
	assistantMsg *schema.AgenticMessage,
	toolResultMessages []*schema.AgenticMessage,
	currentContinuationResponseID string,
	continuation *ProviderContinuation,
) (string, []*schema.AgenticMessage) {
	if strings.TrimSpace(currentContinuationResponseID) == "" {
		nextMessages := append(currentMessages, assistantMsg)
		nextMessages = append(nextMessages, toolResultMessages...)
		return "", nextMessages
	}

	nextResponseID := strings.TrimSpace(currentContinuationResponseID)
	if continuation != nil && strings.TrimSpace(continuation.ResponseID) != "" {
		nextResponseID = strings.TrimSpace(continuation.ResponseID)
	}
	return nextResponseID, append([]*schema.AgenticMessage(nil), toolResultMessages...)
}

func streamAssistantMessage(
	ctx context.Context,
	model einomodel.BaseChatModel,
	messages []*schema.Message,
	onEvent func(ExecutionEvent),
) (*schema.Message, []ExecutionBlock, ExecutionUsage, string, error) {
	reader, err := model.Stream(ctx, messages)
	if err != nil {
		return nil, nil, ExecutionUsage{}, "", err
	}
	defer reader.Close()

	var chunks []*schema.Message
	started := false
	assistantMessageID := uuid.NewString()
	reasoningMessageID := uuid.NewString()
	reasoningStarted := false

	for {
		chunk, recvErr := reader.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			return nil, nil, ExecutionUsage{}, "", recvErr
		}
		if chunk == nil {
			continue
		}
		chunks = append(chunks, chunk)
		if !started {
			started = true
			if onEvent != nil {
				onEvent(ExecutionEvent{Type: "assistant_message_started", MessageID: assistantMessageID})
			}
		}
		if text := chunkTextDelta(chunk); text != "" && onEvent != nil {
			onEvent(ExecutionEvent{
				Type:      "assistant_message_delta",
				MessageID: assistantMessageID,
				Text:      text,
				Content:   text,
			})
		}
		if onEvent != nil {
			reasoningDelta := chunkReasoningDelta(chunk)
			if reasoningDelta.Text != "" || reasoningDelta.EncryptedValue != "" {
				if !reasoningStarted {
					reasoningStarted = true
					onEvent(ExecutionEvent{Type: "reasoning_message_started", MessageID: reasoningMessageID})
				}
				onEvent(ExecutionEvent{
					Type:           "reasoning_message_delta",
					MessageID:      reasoningMessageID,
					Text:           reasoningDelta.Text,
					Content:        reasoningDelta.Text,
					EncryptedValue: reasoningDelta.EncryptedValue,
				})
			}
		}
	}

	if len(chunks) == 0 {
		return schema.AssistantMessage("", nil), nil, ExecutionUsage{}, assistantMessageID, nil
	}

	finalMsg, err := schema.ConcatMessages(chunks)
	if err != nil {
		return nil, nil, ExecutionUsage{}, "", err
	}
	sanitizeSchemaMessageToolCalls(finalMsg)
	blocks := fromSchemaAssistantMessage(finalMsg)
	finalReasoning := extractReasoningFromSchemaMessage(finalMsg)
	if onEvent != nil && (reasoningStarted || finalReasoning.Text != "" || finalReasoning.EncryptedValue != "") {
		if !reasoningStarted {
			onEvent(ExecutionEvent{Type: "reasoning_message_started", MessageID: reasoningMessageID})
		}
		onEvent(ExecutionEvent{
			Type:           "reasoning_message_completed",
			MessageID:      reasoningMessageID,
			Text:           finalReasoning.Text,
			Content:        finalReasoning.Text,
			EncryptedValue: finalReasoning.EncryptedValue,
		})
	}
	if onEvent != nil {
		onEvent(ExecutionEvent{
			Type:      "assistant_message_completed",
			MessageID: assistantMessageID,
			Text:      extractTextFromExecutionBlocks(blocks),
			Content:   extractTextFromExecutionBlocks(blocks),
		})
	}

	usage := ExecutionUsage{}
	if finalMsg.ResponseMeta != nil && finalMsg.ResponseMeta.Usage != nil {
		usage.CachedInputTokens = finalMsg.ResponseMeta.Usage.PromptTokenDetails.CachedTokens
		usage.InputTokens = finalMsg.ResponseMeta.Usage.PromptTokens
		usage.OutputTokens = finalMsg.ResponseMeta.Usage.CompletionTokens
	}
	return finalMsg, blocks, usage, assistantMessageID, nil
}

func generateAssistantAgenticMessage(
	ctx context.Context,
	model einomodel.AgenticModel,
	messages []*schema.AgenticMessage,
	onEvent func(ExecutionEvent),
	opts ...einomodel.Option,
) (*schema.AgenticMessage, []ExecutionBlock, ExecutionUsage, *ProviderContinuation, string, error) {
	assistantMessageID := uuid.NewString()
	if onEvent != nil {
		onEvent(ExecutionEvent{Type: "assistant_message_started", MessageID: assistantMessageID})
	}
	finalMsg, err := model.Generate(ctx, messages, opts...)
	if err != nil {
		return nil, nil, ExecutionUsage{}, nil, "", err
	}
	if finalMsg == nil {
		finalMsg = &schema.AgenticMessage{Role: schema.AgenticRoleTypeAssistant}
	}
	blocks := fromSchemaAgenticAssistantMessage(finalMsg)
	reasoning := extractReasoningFromAgenticMessage(finalMsg)
	if onEvent != nil && (reasoning.Text != "" || reasoning.EncryptedValue != "") {
		reasoningMessageID := uuid.NewString()
		onEvent(ExecutionEvent{Type: "reasoning_message_started", MessageID: reasoningMessageID})
		onEvent(ExecutionEvent{
			Type:           "reasoning_message_completed",
			MessageID:      reasoningMessageID,
			Text:           reasoning.Text,
			Content:        reasoning.Text,
			EncryptedValue: reasoning.EncryptedValue,
		})
	}
	if onEvent != nil {
		onEvent(ExecutionEvent{
			Type:      "assistant_message_completed",
			MessageID: assistantMessageID,
			Text:      extractTextFromExecutionBlocks(blocks),
			Content:   extractTextFromExecutionBlocks(blocks),
		})
	}

	usage := ExecutionUsage{}
	if finalMsg.ResponseMeta != nil && finalMsg.ResponseMeta.TokenUsage != nil {
		usage.CachedInputTokens = finalMsg.ResponseMeta.TokenUsage.PromptTokenDetails.CachedTokens
		usage.InputTokens = finalMsg.ResponseMeta.TokenUsage.PromptTokens
		usage.OutputTokens = finalMsg.ResponseMeta.TokenUsage.CompletionTokens
	}
	return finalMsg, blocks, usage, providerContinuationFromAgenticMessage(finalMsg), assistantMessageID, nil
}

func buildAgenticReplayMessages(
	provider string,
	systemPrompt string,
	history []ExecutionMessage,
	turnLocalInstructions string,
	continuationMode bool,
) ([]*schema.AgenticMessage, error) {
	if continuationMode && strings.TrimSpace(provider) == appmodel.AgentModelProviderOpenAI {
		return toOpenAIContinuationAgenticMessages(history, turnLocalInstructions)
	}
	return toAgenticMessagesWithTurnLocalInstructions(systemPrompt, history, turnLocalInstructions)
}

func toSchemaMessages(systemPrompt string, history []ExecutionMessage) ([]*schema.Message, error) {
	return toSchemaMessagesWithTurnLocalInstructions(systemPrompt, history, "")
}

func toSchemaMessagesWithTurnLocalInstructions(systemPrompt string, history []ExecutionMessage, turnLocalInstructions string) ([]*schema.Message, error) {
	history = sanitizeExecutionHistoryForReplay(history)
	messages := make([]*schema.Message, 0, len(history)+2)
	if strings.TrimSpace(systemPrompt) != "" {
		messages = append(messages, schema.SystemMessage(systemPrompt))
	}
	if instructionMessage := buildTurnLocalInstructionMessage(turnLocalInstructions); instructionMessage != "" {
		messages = append(messages, schema.UserMessage(instructionMessage))
	}
	for _, msg := range history {
		switch msg.Role {
		case "user":
			content := nonEmptyText(msg.Content, extractTextFromExecutionBlocks(msg.Blocks))
			if strings.TrimSpace(content) == "" {
				continue
			}
			messages = append(messages, schema.UserMessage(content))
		case "assistant":
			assistant := &schema.Message{
				Role:    schema.Assistant,
				Content: nonEmptyText(msg.Content, extractTextFromExecutionBlocks(msg.Blocks)),
			}
			for _, block := range msg.Blocks {
				switch block.Type {
				case ExecutionBlockTypeText:
					if strings.TrimSpace(block.Text) != "" {
						assistant.AssistantGenMultiContent = append(assistant.AssistantGenMultiContent, schema.MessageOutputPart{
							Type: schema.ChatMessagePartTypeText,
							Text: block.Text,
						})
					}
				case ExecutionBlockTypeToolCall:
					argsJSON := normalizeExecutionBlockToolInput(block.Input)
					assistant.ToolCalls = append(assistant.ToolCalls, schema.ToolCall{
						ID:   block.ToolCallID,
						Type: "function",
						Function: schema.FunctionCall{
							Name:      block.ToolName,
							Arguments: string(argsJSON),
						},
					})
				}
			}
			if strings.TrimSpace(assistant.Content) == "" && len(assistant.AssistantGenMultiContent) == 0 && len(assistant.ToolCalls) == 0 {
				continue
			}
			messages = append(messages, assistant)
		case "tool":
			if len(msg.Blocks) == 0 {
				modelVisibleOutput := prepareToolResultForModel("", msg.Content, false)
				if strings.TrimSpace(modelVisibleOutput.Content) == "" {
					continue
				}
				messages = append(messages, schema.ToolMessage(modelVisibleOutput.Content, "", schema.WithToolName("")))
				continue
			}
			for _, block := range msg.Blocks {
				if block.Type != ExecutionBlockTypeToolResult {
					continue
				}
				modelVisibleOutput := prepareToolResultForModel(block.ToolName, block.Output, block.IsError)
				if strings.TrimSpace(modelVisibleOutput.Content) == "" {
					continue
				}
				messages = append(messages, schema.ToolMessage(modelVisibleOutput.Content, block.ToolCallID, schema.WithToolName(block.ToolName)))
			}
		default:
			return nil, fmt.Errorf("unsupported execution message role %q", msg.Role)
		}
	}
	return messages, nil
}

func toAgenticMessages(systemPrompt string, history []ExecutionMessage) ([]*schema.AgenticMessage, error) {
	return toAgenticMessagesWithTurnLocalInstructions(systemPrompt, history, "")
}

func toOpenAIContinuationAgenticMessages(history []ExecutionMessage, turnLocalInstructions string) ([]*schema.AgenticMessage, error) {
	messages := make([]*schema.AgenticMessage, 0, len(history)+1)
	if instructionMessage := buildTurnLocalInstructionMessage(turnLocalInstructions); instructionMessage != "" {
		messages = append(messages, schema.UserAgenticMessage(instructionMessage))
	}
	for _, msg := range history {
		switch msg.Role {
		case "user":
			content := nonEmptyText(msg.Content, extractTextFromExecutionBlocks(msg.Blocks))
			if strings.TrimSpace(content) == "" {
				continue
			}
			messages = append(messages, schema.UserAgenticMessage(content))
		case "tool":
			if len(msg.Blocks) == 0 {
				continue
			}
			for _, block := range msg.Blocks {
				if block.Type != ExecutionBlockTypeToolResult {
					continue
				}
				modelVisibleOutput := prepareToolResultForModel(block.ToolName, block.Output, block.IsError)
				if strings.TrimSpace(modelVisibleOutput.Content) == "" {
					continue
				}
				messages = append(messages, functionToolResultAgenticMessage(block.ToolCallID, block.ToolName, modelVisibleOutput.Content))
			}
		case "assistant":
			// With previous_response_id, OpenAI already has prior assistant output in
			// the referenced response chain. Replaying assistant tool-call items here
			// can duplicate provider-owned function_call items.
			continue
		default:
			return nil, fmt.Errorf("unsupported execution message role %q", msg.Role)
		}
	}
	return messages, nil
}

func toAgenticMessagesWithTurnLocalInstructions(systemPrompt string, history []ExecutionMessage, turnLocalInstructions string) ([]*schema.AgenticMessage, error) {
	history = sanitizeExecutionHistoryForReplay(history)
	messages := make([]*schema.AgenticMessage, 0, len(history)+2)
	if strings.TrimSpace(systemPrompt) != "" {
		messages = append(messages, schema.SystemAgenticMessage(systemPrompt))
	}
	if instructionMessage := buildTurnLocalInstructionMessage(turnLocalInstructions); instructionMessage != "" {
		messages = append(messages, schema.UserAgenticMessage(instructionMessage))
	}
	for _, msg := range history {
		switch msg.Role {
		case "user":
			content := nonEmptyText(msg.Content, extractTextFromExecutionBlocks(msg.Blocks))
			if strings.TrimSpace(content) == "" {
				continue
			}
			messages = append(messages, schema.UserAgenticMessage(content))
		case "assistant":
			assistant := &schema.AgenticMessage{
				Role:          schema.AgenticRoleTypeAssistant,
				ContentBlocks: make([]*schema.ContentBlock, 0, len(msg.Blocks)+1),
			}
			if len(msg.Blocks) == 0 && strings.TrimSpace(msg.Content) != "" {
				assistant.ContentBlocks = append(assistant.ContentBlocks, schema.NewContentBlock(&schema.AssistantGenText{Text: msg.Content}))
			}
			for _, block := range msg.Blocks {
				switch block.Type {
				case ExecutionBlockTypeText:
					if strings.TrimSpace(block.Text) != "" {
						assistant.ContentBlocks = append(assistant.ContentBlocks, schema.NewContentBlock(&schema.AssistantGenText{Text: block.Text}))
					}
				case ExecutionBlockTypeToolCall:
					argsJSON := normalizeExecutionBlockToolInput(block.Input)
					assistant.ContentBlocks = append(assistant.ContentBlocks, schema.NewContentBlock(&schema.FunctionToolCall{
						CallID:    block.ToolCallID,
						Name:      block.ToolName,
						Arguments: string(argsJSON),
					}))
				}
			}
			if len(assistant.ContentBlocks) == 0 {
				continue
			}
			messages = append(messages, assistant)
		case "tool":
			if len(msg.Blocks) == 0 {
				continue
			}
			for _, block := range msg.Blocks {
				if block.Type != ExecutionBlockTypeToolResult {
					continue
				}
				modelVisibleOutput := prepareToolResultForModel(block.ToolName, block.Output, block.IsError)
				if strings.TrimSpace(modelVisibleOutput.Content) == "" {
					continue
				}
				messages = append(messages, functionToolResultAgenticMessage(block.ToolCallID, block.ToolName, modelVisibleOutput.Content))
			}
		default:
			return nil, fmt.Errorf("unsupported execution message role %q", msg.Role)
		}
	}
	return messages, nil
}

func buildTurnLocalInstructionMessage(turnLocalInstructions string) string {
	turnLocalInstructions = strings.TrimSpace(turnLocalInstructions)
	if turnLocalInstructions == "" {
		return ""
	}
	return "Execution-local instructions for this turn only:\n" + turnLocalInstructions
}

func compactToolOutputForModel(toolName, output string) string {
	return analyzeToolOutputForModel(toolName, output).Content
}

func prepareToolResultForModel(toolName, output string, isError bool) modelVisibleToolOutput {
	analysis := analyzeToolOutputForModel(toolName, output)
	if strings.TrimSpace(analysis.Content) != "" {
		return analysis
	}

	placeholder := toolResultNoOutputPlaceholder
	if isError {
		placeholder = "[tool returned no output; tool reported an error]"
	}
	analysis.Content = placeholder
	analysis.VisibleRunes = len([]rune(placeholder))
	analysis.VisibleLines = countToolOutputLines(placeholder)
	return analysis
}

func analyzeToolOutputForModel(toolName, output string) modelVisibleToolOutput {
	if strings.TrimSpace(output) == "" {
		return modelVisibleToolOutput{Content: output}
	}

	runes := []rune(output)
	maxRunes, headRunes, tailRunes := toolOutputCompactionLimits(toolName)
	analysis := modelVisibleToolOutput{
		Content:       output,
		OriginalRunes: len(runes),
		VisibleRunes:  len(runes),
		OriginalLines: countToolOutputLines(output),
		VisibleLines:  countToolOutputLines(output),
		Compacted:     false,
	}
	if outputHasCompactionExemption(output, len(runes)) {
		return analysis
	}
	if len(runes) <= maxRunes {
		return analysis
	}

	if headRunes+tailRunes >= len(runes) {
		return analysis
	}

	omittedRunes := len(runes) - headRunes - tailRunes
	head := string(runes[:headRunes])
	tail := string(runes[len(runes)-tailRunes:])
	label := strings.TrimSpace(toolName)
	if label == "" {
		label = "tool"
	}

	compacted := head + fmt.Sprintf(
		"\n\n[helpin truncated %d characters from previous %s output to reduce model token usage. Re-run the tool if you need the omitted section.]\n\n",
		omittedRunes,
		label,
	) + tail
	return modelVisibleToolOutput{
		Content:       compacted,
		OriginalRunes: len(runes),
		VisibleRunes:  len([]rune(compacted)),
		OriginalLines: countToolOutputLines(output),
		VisibleLines:  countToolOutputLines(compacted),
		Compacted:     true,
	}
}

func outputHasCompactionExemption(output string, runeCount int) bool {
	if !strings.Contains(output, "_helpin_compaction") {
		return false
	}
	var envelope struct {
		Hint *helpinCompactionHint `json:"_helpin_compaction"`
	}
	if err := json.Unmarshal([]byte(output), &envelope); err != nil || envelope.Hint == nil || !envelope.Hint.Exempt {
		return false
	}
	maxRunes := envelope.Hint.MaxRunes
	if maxRunes <= 0 || maxRunes > boundedToolOutputCompactionMaxRunes {
		maxRunes = boundedToolOutputCompactionMaxRunes
	}
	return runeCount <= maxRunes
}

func toolOutputCompactionLimits(toolName string) (maxRunes, headRunes, tailRunes int) {
	switch strings.TrimSpace(toolName) {
	case "read_file", "read_files":
		return modelVisibleFileReadOutputMaxRunes, modelVisibleFileReadOutputHeadRunes, modelVisibleFileReadOutputTailRunes
	case "read_file_range":
		return modelVisibleReadRangeOutputMaxRunes, modelVisibleReadRangeOutputHeadRunes, modelVisibleReadRangeOutputTailRunes
	case "ripgrep":
		return modelVisibleRipgrepOutputMaxRunes, modelVisibleRipgrepOutputHeadRunes, modelVisibleRipgrepOutputTailRunes
	default:
		if isHighVolumeToolOutput(toolName) {
			return modelVisibleToolOutputMaxRunes, modelVisibleToolOutputHeadRunes, modelVisibleToolOutputTailRunes
		}
		return modelVisibleToolOutputMaxSmallTool, modelVisibleToolOutputMaxSmallTool - 800, 400
	}
}

func isHighVolumeToolOutput(toolName string) bool {
	switch strings.TrimSpace(toolName) {
	case "read_file",
		"read_files",
		"read_file_range",
		"list_directory",
		"search_files",
		"ripgrep",
		"grep",
		"list_symbols",
		"run_command",
		"read_document",
		"get_document_blocks",
		"get_release_context",
		"get_task_context",
		"search_documents",
		"find_tasks_for_git_changes",
		ToolScanSemgrep,
		ToolScanTrivy,
		ToolScanGitleaks,
		"web_search_brave",
		"web_search_exa",
		"fetch_url",
		"crawl_url":
		return true
	default:
		return false
	}
}

func countToolOutputLines(value string) int {
	if value == "" {
		return 0
	}
	return strings.Count(value, "\n") + 1
}

func logNativeToolResultForModel(
	ctx context.Context,
	execCtx *ExecutionContext,
	executed executedToolCall,
	modelVisible modelVisibleToolOutput,
) {
	slog.DebugContext(ctx, "native runtime tool result prepared for model",
		"workspace_id", execCtx.WorkspaceID,
		"run_id", execCtx.RunID,
		"agent_id", execCtx.AgentID,
		"tool_name", executed.ToolName,
		"tool_call_id", executed.ToolCallID,
		"duration_ms", executed.Duration.Milliseconds(),
		"is_error", executed.IsError,
		"output_chars", modelVisible.OriginalRunes,
		"output_lines", modelVisible.OriginalLines,
		"model_visible_chars", modelVisible.VisibleRunes,
		"model_visible_lines", modelVisible.VisibleLines,
		"compacted_for_model", modelVisible.Compacted,
	)
}

func sanitizeExecutionHistoryForReplay(history []ExecutionMessage) []ExecutionMessage {
	if len(history) == 0 {
		return nil
	}

	sanitized := make([]ExecutionMessage, 0, len(history))
	for i := 0; i < len(history); i++ {
		msg := history[i]
		switch msg.Role {
		case "assistant":
			if !messageHasToolCalls(msg) {
				sanitized = append(sanitized, msg)
				continue
			}

			groupEnd := i + 1
			for groupEnd < len(history) && history[groupEnd].Role == "tool" {
				groupEnd++
			}
			if groupEnd == i+1 {
				sanitized = append(sanitized, msg)
				continue
			}

			matchedCalls := matchedAssistantToolCallIDs(msg, history[i+1:groupEnd])
			sanitizedAssistant, keepAssistant := sanitizeAssistantReplayMessage(msg, matchedCalls)
			if keepAssistant {
				sanitized = append(sanitized, sanitizedAssistant)
			}
			for _, toolMsg := range history[i+1 : groupEnd] {
				sanitizedTool, keepTool := sanitizeToolReplayMessage(toolMsg, matchedCalls)
				if keepTool {
					sanitized = append(sanitized, sanitizedTool)
				}
			}
			i = groupEnd - 1
		case "tool":
			// Replay tool results only when they are paired with the immediately
			// preceding assistant tool calls. Orphaned results break Anthropic.
			continue
		default:
			sanitized = append(sanitized, msg)
		}
	}
	return sanitized
}

func messageHasToolCalls(msg ExecutionMessage) bool {
	for _, block := range msg.Blocks {
		if block.Type == ExecutionBlockTypeToolCall && strings.TrimSpace(block.ToolCallID) != "" {
			return true
		}
	}
	return false
}

func matchedAssistantToolCallIDs(assistant ExecutionMessage, toolMessages []ExecutionMessage) map[string]bool {
	allowed := make(map[string]bool)
	for _, block := range assistant.Blocks {
		if block.Type != ExecutionBlockTypeToolCall || strings.TrimSpace(block.ToolCallID) == "" {
			continue
		}
		allowed[strings.TrimSpace(block.ToolCallID)] = false
	}
	for _, toolMsg := range toolMessages {
		for _, block := range toolMsg.Blocks {
			if block.Type != ExecutionBlockTypeToolResult {
				continue
			}
			toolCallID := strings.TrimSpace(block.ToolCallID)
			if toolCallID == "" {
				continue
			}
			if _, ok := allowed[toolCallID]; ok {
				allowed[toolCallID] = true
			}
		}
	}

	matched := make(map[string]bool)
	for toolCallID, ok := range allowed {
		if ok {
			matched[toolCallID] = true
		}
	}
	return matched
}

func sanitizeAssistantReplayMessage(msg ExecutionMessage, matchedCalls map[string]bool) (ExecutionMessage, bool) {
	if len(matchedCalls) == 0 {
		textOnly := msg
		textOnly.Blocks = filterExecutionBlocksForReplay(msg.Blocks, nil)
		if strings.TrimSpace(textOnly.Content) == "" && len(textOnly.Blocks) == 0 {
			return ExecutionMessage{}, false
		}
		return textOnly, true
	}

	sanitized := msg
	sanitized.Blocks = filterExecutionBlocksForReplay(msg.Blocks, matchedCalls)
	if strings.TrimSpace(sanitized.Content) == "" && len(sanitized.Blocks) == 0 {
		return ExecutionMessage{}, false
	}
	return sanitized, true
}

func sanitizeToolReplayMessage(msg ExecutionMessage, matchedCalls map[string]bool) (ExecutionMessage, bool) {
	if len(matchedCalls) == 0 {
		return ExecutionMessage{}, false
	}

	sanitized := msg
	sanitized.Blocks = filterExecutionBlocksForReplay(msg.Blocks, matchedCalls)
	if len(sanitized.Blocks) == 0 {
		return ExecutionMessage{}, false
	}
	sanitized.Content = ExtractPersistedContentFromExecutionBlocks(sanitized.Blocks)
	return sanitized, true
}

func filterExecutionBlocksForReplay(blocks []ExecutionBlock, matchedCalls map[string]bool) []ExecutionBlock {
	if len(blocks) == 0 {
		return nil
	}
	filtered := make([]ExecutionBlock, 0, len(blocks))
	for _, block := range blocks {
		switch block.Type {
		case ExecutionBlockTypeText:
			filtered = append(filtered, block)
		case ExecutionBlockTypeToolCall, ExecutionBlockTypeToolResult:
			if matchedCalls[strings.TrimSpace(block.ToolCallID)] {
				filtered = append(filtered, block)
			}
		}
	}
	return filtered
}

func fromSchemaAssistantMessage(msg *schema.Message) []ExecutionBlock {
	if msg == nil {
		return nil
	}
	blocks := make([]ExecutionBlock, 0, len(msg.AssistantGenMultiContent)+len(msg.ToolCalls)+1)
	if strings.TrimSpace(msg.Content) != "" {
		blocks = append(blocks, ExecutionBlock{
			Type: ExecutionBlockTypeText,
			Text: msg.Content,
		})
	}
	for _, part := range msg.AssistantGenMultiContent {
		if part.Type == schema.ChatMessagePartTypeText && strings.TrimSpace(part.Text) != "" {
			blocks = append(blocks, ExecutionBlock{
				Type: ExecutionBlockTypeText,
				Text: part.Text,
			})
		}
	}
	for _, toolCall := range msg.ToolCalls {
		blocks = append(blocks, ExecutionBlock{
			Type:       ExecutionBlockTypeToolCall,
			ToolCallID: toolCall.ID,
			ToolName:   toolCall.Function.Name,
			Input:      normalizeToolArguments(toolCall.Function.Arguments),
		})
	}
	return dedupeAdjacentTextBlocks(blocks)
}

func chunkReasoningDelta(chunk *schema.Message) executionReasoning {
	if chunk == nil {
		return executionReasoning{}
	}

	reasoning := executionReasoning{
		Text:           chunk.ReasoningContent,
		EncryptedValue: "",
	}
	for _, part := range chunk.AssistantGenMultiContent {
		if part.Type != schema.ChatMessagePartTypeReasoning || part.Reasoning == nil {
			continue
		}
		reasoning.Text += part.Reasoning.Text
		if strings.TrimSpace(part.Reasoning.Signature) != "" {
			reasoning.EncryptedValue = strings.TrimSpace(part.Reasoning.Signature)
		}
	}
	if strings.TrimSpace(reasoning.Text) == "" {
		if text, ok := openaiacl.GetReasoningContent(chunk); ok {
			reasoning.Text = text
		}
	}
	return reasoning
}

func extractReasoningFromSchemaMessage(msg *schema.Message) executionReasoning {
	if msg == nil {
		return executionReasoning{}
	}

	reasoning := executionReasoning{
		Text:           msg.ReasoningContent,
		EncryptedValue: "",
	}
	for _, part := range msg.AssistantGenMultiContent {
		if part.Type != schema.ChatMessagePartTypeReasoning || part.Reasoning == nil {
			continue
		}
		reasoning.Text += part.Reasoning.Text
		if strings.TrimSpace(part.Reasoning.Signature) != "" {
			reasoning.EncryptedValue = strings.TrimSpace(part.Reasoning.Signature)
		}
	}
	if strings.TrimSpace(reasoning.Text) == "" {
		if text, ok := openaiacl.GetReasoningContent(msg); ok {
			reasoning.Text = text
		}
	}
	return reasoning
}

func fromSchemaAgenticAssistantMessage(msg *schema.AgenticMessage) []ExecutionBlock {
	if msg == nil {
		return nil
	}
	blocks := make([]ExecutionBlock, 0, len(msg.ContentBlocks))
	for _, block := range msg.ContentBlocks {
		if block == nil {
			continue
		}
		switch block.Type {
		case schema.ContentBlockTypeAssistantGenText:
			if block.AssistantGenText != nil && strings.TrimSpace(block.AssistantGenText.Text) != "" {
				blocks = append(blocks, ExecutionBlock{
					Type: ExecutionBlockTypeText,
					Text: block.AssistantGenText.Text,
				})
			}
		case schema.ContentBlockTypeFunctionToolCall:
			if block.FunctionToolCall != nil {
				blocks = append(blocks, ExecutionBlock{
					Type:       ExecutionBlockTypeToolCall,
					ToolCallID: block.FunctionToolCall.CallID,
					ToolName:   block.FunctionToolCall.Name,
					Input:      normalizeToolArguments(block.FunctionToolCall.Arguments),
				})
			}
		}
	}
	return dedupeAdjacentTextBlocks(blocks)
}

func extractReasoningFromAgenticMessage(msg *schema.AgenticMessage) executionReasoning {
	if msg == nil {
		return executionReasoning{}
	}

	var reasoning executionReasoning
	for _, block := range msg.ContentBlocks {
		if block == nil || block.Type != schema.ContentBlockTypeReasoning || block.Reasoning == nil {
			continue
		}
		reasoning.Text += block.Reasoning.Text
		if strings.TrimSpace(block.Reasoning.Signature) != "" {
			reasoning.EncryptedValue = strings.TrimSpace(block.Reasoning.Signature)
		}
	}
	return reasoning
}

func extractTextFromExecutionBlocks(blocks []ExecutionBlock) string {
	var parts []string
	for _, block := range blocks {
		if block.Type == ExecutionBlockTypeText && strings.TrimSpace(block.Text) != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func ExtractPersistedContentFromExecutionBlocks(blocks []ExecutionBlock) string {
	if text := strings.TrimSpace(extractTextFromExecutionBlocks(blocks)); text != "" {
		return text
	}
	for _, block := range blocks {
		if block.Type == ExecutionBlockTypeToolResult && strings.TrimSpace(block.Output) != "" {
			return strings.TrimSpace(block.Output)
		}
	}
	return ""
}

func chunkTextDelta(chunk *schema.Message) string {
	if chunk == nil {
		return ""
	}
	if strings.TrimSpace(chunk.Content) != "" {
		return chunk.Content
	}
	var parts []string
	for _, part := range chunk.AssistantGenMultiContent {
		if part.Type == schema.ChatMessagePartTypeText && strings.TrimSpace(part.Text) != "" {
			parts = append(parts, part.Text)
		}
	}
	return strings.Join(parts, "")
}

func chunkAgenticTextDelta(chunk *schema.AgenticMessage) string {
	if chunk == nil {
		return ""
	}
	var parts []string
	for _, block := range chunk.ContentBlocks {
		if block == nil || block.Type != schema.ContentBlockTypeAssistantGenText || block.AssistantGenText == nil {
			continue
		}
		if strings.TrimSpace(block.AssistantGenText.Text) != "" {
			parts = append(parts, block.AssistantGenText.Text)
		}
	}
	return strings.Join(parts, "")
}

func normalizeToolArguments(raw string) json.RawMessage {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return json.RawMessage(`{}`)
	}
	if json.Valid([]byte(trimmed)) {
		switch trimmed[0] {
		case '{':
			return json.RawMessage(trimmed)
		case '"':
			var text string
			if err := json.Unmarshal([]byte(trimmed), &text); err == nil {
				text = strings.TrimSpace(text)
				if text == "" {
					return json.RawMessage(`{}`)
				}
				encoded, _ := json.Marshal(map[string]string{"raw": text})
				return json.RawMessage(encoded)
			}
		}
	}
	encoded, _ := json.Marshal(map[string]string{"raw": trimmed})
	return json.RawMessage(encoded)
}

func normalizeExecutionBlockToolInput(raw json.RawMessage) json.RawMessage {
	return normalizeToolArguments(string(raw))
}

func NormalizeExecutionBlocks(blocks []ExecutionBlock) []ExecutionBlock {
	if len(blocks) == 0 {
		return blocks
	}
	normalized := make([]ExecutionBlock, len(blocks))
	copy(normalized, blocks)
	for i := range normalized {
		if normalized[i].Type == ExecutionBlockTypeToolCall {
			normalized[i].Input = normalizeExecutionBlockToolInput(normalized[i].Input)
		}
	}
	return normalized
}

func sanitizeSchemaMessageToolCalls(msg *schema.Message) {
	if msg == nil || len(msg.ToolCalls) == 0 {
		return
	}
	for i := range msg.ToolCalls {
		msg.ToolCalls[i].Function.Arguments = string(normalizeToolArguments(msg.ToolCalls[i].Function.Arguments))
	}
}

func summarizeToolInput(input json.RawMessage) string {
	return truncate(strings.TrimSpace(string(input)), 200)
}

func toolInputForEvent(toolName string, input json.RawMessage) string {
	if isPreviewToolName(toolName) {
		return strings.TrimSpace(string(input))
	}
	return summarizeToolInput(input)
}

func executionToolCalls(blocks []ExecutionBlock) []ExecutionBlock {
	toolCalls := make([]ExecutionBlock, 0, len(blocks))
	for _, block := range blocks {
		if block.Type == ExecutionBlockTypeToolCall {
			toolCalls = append(toolCalls, block)
		}
	}
	return toolCalls
}

func providerContinuationFromAgenticMessage(msg *schema.AgenticMessage) *ProviderContinuation {
	if msg == nil || msg.ResponseMeta == nil || msg.ResponseMeta.OpenAIExtension == nil {
		return nil
	}
	ext := msg.ResponseMeta.OpenAIExtension
	if strings.TrimSpace(ext.ID) == "" {
		return nil
	}
	return &ProviderContinuation{
		Provider:           appmodel.AgentModelProviderOpenAI,
		ResponseID:         strings.TrimSpace(ext.ID),
		PreviousResponseID: strings.TrimSpace(ext.PreviousResponseID),
	}
}

func executeToolCallsForRound(
	execCtx *ExecutionContext,
	registry *ToolRegistry,
	toolCalls []ExecutionBlock,
	parentMessageID string,
	onEvent func(ExecutionEvent),
) []executedToolCall {
	if len(toolCalls) == 0 {
		return nil
	}

	emitStarted := func(toolCall ExecutionBlock) {
		if onEvent == nil {
			return
		}
		onEvent(ExecutionEvent{
			Type:            "tool_call_started",
			ToolCallID:      toolCall.ToolCallID,
			ToolName:        toolCall.ToolName,
			ToolInput:       toolInputForEvent(toolCall.ToolName, toolCall.Input),
			ParentMessageID: strings.TrimSpace(parentMessageID),
			ArgsText:        strings.TrimSpace(string(normalizeExecutionBlockToolInput(toolCall.Input))),
		})
		argsText := strings.TrimSpace(string(normalizeExecutionBlockToolInput(toolCall.Input)))
		if argsText != "" {
			onEvent(ExecutionEvent{
				Type:            "tool_call_args_delta",
				ToolCallID:      toolCall.ToolCallID,
				ToolName:        toolCall.ToolName,
				ParentMessageID: strings.TrimSpace(parentMessageID),
				ArgsDelta:       argsText,
				ArgsText:        argsText,
			})
		}
	}
	emitFinished := func(executed executedToolCall) {
		if onEvent == nil {
			return
		}
		resultMessageID := uuid.NewString()
		errorText := ""
		if executed.IsError {
			errorText = truncate(executed.Output, 500)
		}
		onEvent(ExecutionEvent{
			Type:            "tool_call_result",
			ToolCallID:      executed.ToolCallID,
			ToolName:        executed.ToolName,
			ParentMessageID: strings.TrimSpace(parentMessageID),
			ResultMessageID: resultMessageID,
			Content:         truncate(executed.Output, 2000),
			OutputSummary:   truncate(executed.Output, 500),
			Error:           errorText,
		})
		onEvent(ExecutionEvent{
			Type:            "tool_call_finished",
			ToolCallID:      executed.ToolCallID,
			ToolName:        executed.ToolName,
			ParentMessageID: strings.TrimSpace(parentMessageID),
			ResultMessageID: resultMessageID,
			OutputSummary:   truncate(executed.Output, 500),
			Content:         truncate(executed.Output, 2000),
			DurationMs:      executed.Duration.Milliseconds(),
			Error:           errorText,
		})
	}

	results := make([]executedToolCall, len(toolCalls))
	if !canExecuteToolCallsInParallel(toolCalls) {
		for i, toolCall := range toolCalls {
			emitStarted(toolCall)
			results[i] = executeSingleToolCall(execCtx, registry, toolCall)
			emitFinished(results[i])
			if IsHumanInteractionTool(results[i].ToolName) {
				return results[:i+1]
			}
		}
		return results
	}

	var wg sync.WaitGroup
	var eventMu sync.Mutex
	wg.Add(len(toolCalls))
	for i, toolCall := range toolCalls {
		go func(index int, pending ExecutionBlock) {
			defer wg.Done()

			eventMu.Lock()
			emitStarted(pending)
			eventMu.Unlock()

			executed := executeSingleToolCall(execCtx, registry, pending)
			results[index] = executed

			eventMu.Lock()
			emitFinished(executed)
			eventMu.Unlock()
		}(i, toolCall)
	}
	wg.Wait()
	return results
}

func executeSingleToolCall(execCtx *ExecutionContext, registry *ToolRegistry, toolCall ExecutionBlock) executedToolCall {
	start := time.Now()
	output, toolErr := registry.ExecuteAllowed(execCtx, toolCall.ToolName, toolCall.Input)
	duration := time.Since(start)
	isError := toolErr != nil
	if toolErr != nil {
		output = toolErr.Error()
	}
	return executedToolCall{
		ToolCallID: toolCall.ToolCallID,
		ToolName:   toolCall.ToolName,
		Input:      toolCall.Input,
		Output:     output,
		Duration:   duration,
		IsError:    isError,
	}
}

func canExecuteToolCallsInParallel(toolCalls []ExecutionBlock) bool {
	if len(toolCalls) < 2 {
		return false
	}
	for _, toolCall := range toolCalls {
		if !isParallelSafeTool(toolCall.ToolName) {
			return false
		}
	}
	return true
}

func isParallelSafeTool(name string) bool {
	switch CanonicalToolName(name) {
	case "read_file", "read_files", "read_file_range", "list_directory", "search_files", "ripgrep", "grep", "list_symbols",
		"web_search_brave", "web_search_exa", "fetch_url", "crawl_url",
		"list_task_checklist", "list_workspace_teams", "list_team_workflows_with_stages", "list_conversation_messages",
		"list_deals", "list_contacts", "list_buyer_signals",
		"list_documents", "list_collections", "read_document", "get_document_blocks", "search_documents", "list_epic_tasks",
		"get_release_context", "find_tasks_for_git_changes", "get_task_context":
		return true
	default:
		return false
	}
}

func filterExecutionHistoryAfterSequence(history []ExecutionMessage, afterSequenceNo int) []ExecutionMessage {
	if afterSequenceNo <= 0 {
		return append([]ExecutionMessage(nil), history...)
	}
	filtered := make([]ExecutionMessage, 0, len(history))
	for _, message := range history {
		if message.SequenceNo <= afterSequenceNo {
			continue
		}
		filtered = append(filtered, message)
	}
	return filtered
}

func nonEmptyText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func dedupeAdjacentTextBlocks(blocks []ExecutionBlock) []ExecutionBlock {
	if len(blocks) == 0 {
		return nil
	}
	result := make([]ExecutionBlock, 0, len(blocks))
	for _, block := range blocks {
		if block.Type != ExecutionBlockTypeText {
			result = append(result, block)
			continue
		}
		if len(result) == 0 || result[len(result)-1].Type != ExecutionBlockTypeText {
			result = append(result, block)
			continue
		}
		if strings.TrimSpace(block.Text) == "" {
			continue
		}
		if strings.TrimSpace(result[len(result)-1].Text) == "" {
			result[len(result)-1].Text = block.Text
			continue
		}
		result[len(result)-1].Text = result[len(result)-1].Text + block.Text
	}
	return result
}

func toEinoToolInfos(defs []ToolDefinition) ([]*schema.ToolInfo, error) {
	if len(defs) == 0 {
		return nil, nil
	}
	result := make([]*schema.ToolInfo, 0, len(defs))
	for _, def := range defs {
		params, err := toToolParams(def.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("convert tool %q schema: %w", def.Name, err)
		}
		result = append(result, &schema.ToolInfo{
			Name:        def.Name,
			Desc:        def.Description,
			ParamsOneOf: schema.NewParamsOneOfByParams(params),
		})
	}
	return result, nil
}

func toToolParams(inputSchema any) (map[string]*schema.ParameterInfo, error) {
	schemaMap, ok := inputSchema.(map[string]interface{})
	if !ok {
		return nil, nil
	}
	props, _ := schemaMap["properties"].(map[string]interface{})
	requiredSet := jsonSchemaRequiredSet(schemaMap)
	params := make(map[string]*schema.ParameterInfo, len(props))
	for name, raw := range props {
		propMap, _ := raw.(map[string]interface{})
		param, err := toParameterInfo(propMap)
		if err != nil {
			return nil, fmt.Errorf("property %q: %w", name, err)
		}
		param.Required = requiredSet[name]
		params[name] = param
	}
	return params, nil
}

func toParameterInfo(raw map[string]interface{}) (*schema.ParameterInfo, error) {
	typeName, _ := raw["type"].(string)
	info := &schema.ParameterInfo{
		Type: schema.String,
		Desc: stringValue(raw["description"]),
	}
	switch typeName {
	case "object":
		info.Type = schema.Object
		props, _ := raw["properties"].(map[string]interface{})
		if len(props) > 0 {
			requiredSet := jsonSchemaRequiredSet(raw)
			info.SubParams = make(map[string]*schema.ParameterInfo, len(props))
			for name, value := range props {
				childMap, _ := value.(map[string]interface{})
				child, err := toParameterInfo(childMap)
				if err != nil {
					return nil, err
				}
				child.Required = requiredSet[name]
				info.SubParams[name] = child
			}
		}
	case "array":
		info.Type = schema.Array
		if itemMap, ok := raw["items"].(map[string]interface{}); ok {
			elem, err := toParameterInfo(itemMap)
			if err != nil {
				return nil, err
			}
			info.ElemInfo = elem
		} else {
			info.ElemInfo = &schema.ParameterInfo{Type: schema.String}
		}
	case "integer":
		info.Type = schema.Integer
	case "number":
		info.Type = schema.Number
	case "boolean":
		info.Type = schema.Boolean
	case "null":
		info.Type = schema.Null
	default:
		info.Type = schema.String
	}
	if enumValues, ok := raw["enum"].([]interface{}); ok {
		for _, value := range enumValues {
			if text, ok := value.(string); ok {
				info.Enum = append(info.Enum, text)
			}
		}
	}
	return info, nil
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func jsonSchemaRequiredSet(raw map[string]interface{}) map[string]bool {
	requiredSet := map[string]bool{}
	switch required := raw["required"].(type) {
	case []string:
		for _, name := range required {
			requiredSet[name] = true
		}
	case []interface{}:
		for _, entry := range required {
			if name, ok := entry.(string); ok {
				requiredSet[name] = true
			}
		}
	}
	return requiredSet
}
