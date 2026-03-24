package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	agenticopenai "github.com/cloudwego/eino-ext/components/model/agenticopenai"
	einoclaude "github.com/cloudwego/eino-ext/components/model/claude"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	appmodel "github.com/helpin-ai/helpin/server/internal/model"
)

const (
	ExecutionBlockTypeText       = "text"
	ExecutionBlockTypeToolCall   = "tool_call"
	ExecutionBlockTypeToolResult = "tool_result"
)

var (
	ErrMaxToolStepsReached    = errors.New("agent reached max tool steps")
	ErrInitialResponseTimeout = errors.New("initial_response_timeout")
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
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type ExecutionEvent struct {
	Type          string
	Text          string
	ToolCallID    string
	ToolName      string
	ToolInput     string
	OutputSummary string
	DurationMs    int64
	Error         string
}

type ExecutionResult struct {
	Messages             []ExecutionMessage
	AssistantBlocks      []ExecutionBlock
	AssistantText        string
	ToolInvocations      []appmodel.ToolInvocation
	Usage                ExecutionUsage
	ProviderContinuation *ProviderContinuation
	MaxStepsReached      bool
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

func (f *EinoModelFactory) ResolveAgentic(ctx context.Context, agent *appmodel.Agent, tools []ToolDefinition) (einomodel.AgenticModel, string, error) {
	provider, modelName := resolveProviderAndModel(agent)
	baseModel, err := f.resolveAgenticBaseModel(ctx, provider, modelName)
	if err != nil {
		return nil, "", err
	}
	toolInfos, err := toEinoToolInfos(tools)
	if err != nil {
		return nil, "", err
	}
	if len(toolInfos) == 0 {
		return baseModel, modelName, nil
	}
	withTools, err := baseModel.WithTools(toolInfos)
	if err != nil {
		return nil, "", err
	}
	return withTools, modelName, nil
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
			MaxTokens: defaultMaxTokensForProvider(provider),
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
		return agenticopenai.New(ctx, &agenticopenai.Config{
			APIKey:  f.OpenAIAPIKey,
			BaseURL: strings.TrimSpace(f.OpenAIBaseURL),
			Model:   modelName,
		})
	case appmodel.AgentModelProviderOpenRouter, appmodel.AgentModelProviderOpenRouterResponses:
		if strings.TrimSpace(f.OpenRouterKey) == "" {
			return nil, fmt.Errorf("openrouter API key is not configured")
		}
		baseURL := strings.TrimSpace(f.OpenRouterURL)
		if baseURL == "" {
			baseURL = "https://openrouter.ai/api/v1"
		}
		return agenticopenai.New(ctx, &agenticopenai.Config{
			APIKey:  f.OpenRouterKey,
			BaseURL: baseURL,
			Model:   modelName,
		})
	default:
		return nil, fmt.Errorf("provider %q does not support the Responses-based agentic model path", provider)
	}
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
	return strings.TrimSpace(provider) == appmodel.AgentModelProviderOpenAI
}

func defaultModelForProvider(provider string) string {
	switch provider {
	case appmodel.AgentModelProviderOpenAI:
		return "gpt-4.1"
	case appmodel.AgentModelProviderOpenRouter, appmodel.AgentModelProviderOpenRouterResponses:
		return "openai/gpt-4.1"
	default:
		return "claude-sonnet-4-20250514"
	}
}

func defaultMaxTokensForProvider(provider string) int {
	switch provider {
	case "anthropic":
		return 4096
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
	if providerUsesAgenticResponses(provider) {
		return executeWithEinoAgentic(ctx, factory, agent, systemPrompt, history, tools, execCtx, registry, maxSteps, onEvent)
	}

	modelWithTools, _, err := factory.Resolve(ctx, agent, tools)
	if err != nil {
		return nil, err
	}

	messages, err := toSchemaMessages(systemPrompt, history)
	if err != nil {
		return nil, err
	}

	result := &ExecutionResult{
		Messages: append([]ExecutionMessage(nil), history...),
	}

	for step := 0; step < maxSteps; step++ {
		assistantMsg, assistantBlocks, usage, err := streamAssistantMessage(ctx, modelWithTools, messages, onEvent)
		if err != nil {
			return nil, err
		}
		result.Usage.InputTokens += usage.InputTokens
		result.Usage.OutputTokens += usage.OutputTokens
		result.AssistantBlocks = assistantBlocks
		result.AssistantText = extractTextFromExecutionBlocks(assistantBlocks)
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
		for _, toolCall := range assistantMsg.ToolCalls {
			argsJSON := normalizeToolArguments(toolCall.Function.Arguments)
			toolName := toolCall.Function.Name

			if onEvent != nil {
				onEvent(ExecutionEvent{
					Type:       "tool_call_started",
					ToolCallID: toolCall.ID,
					ToolName:   toolName,
					ToolInput:  toolInputForEvent(toolName, argsJSON),
				})
			}

			start := time.Now()
			output, toolErr := registry.ExecuteAllowed(execCtx, toolName, argsJSON)
			duration := time.Since(start)
			isError := toolErr != nil
			if toolErr != nil {
				output = toolErr.Error()
			}

			summary := truncate(output, 500)
			result.ToolInvocations = append(result.ToolInvocations, appmodel.ToolInvocation{
				ToolName:      toolName,
				Input:         argsJSON,
				OutputSummary: summary,
				DurationMs:    duration.Milliseconds(),
			})
			if onEvent != nil {
				onEvent(ExecutionEvent{
					Type:          "tool_call_finished",
					ToolCallID:    toolCall.ID,
					ToolName:      toolName,
					OutputSummary: summary,
					DurationMs:    duration.Milliseconds(),
				})
			}

			result.Messages = append(result.Messages, ExecutionMessage{
				Role:    "tool",
				Content: output,
				Blocks: []ExecutionBlock{{
					Type:       ExecutionBlockTypeToolResult,
					ToolCallID: toolCall.ID,
					ToolName:   toolName,
					Input:      argsJSON,
					Output:     output,
					IsError:    isError,
				}},
			})
			messages = append(messages, schema.ToolMessage(output, toolCall.ID, schema.WithToolName(toolName)))
			if IsHumanInteractionTool(toolName) {
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
) (*ExecutionResult, error) {
	modelWithTools, _, err := factory.ResolveAgentic(ctx, agent, tools)
	if err != nil {
		return nil, err
	}

	provider, _ := resolveProviderAndModel(agent)
	effectiveSystemPrompt := systemPrompt
	effectiveHistory := history
	agenticOpts := make([]einomodel.Option, 0, 1)
	if continuation := execCtx.ProviderContinuation; continuation != nil && strings.TrimSpace(continuation.ResponseID) != "" && ProviderSupportsResponseContinuation(provider) {
		effectiveSystemPrompt = ""
		effectiveHistory = filterExecutionHistoryAfterSequence(history, continuation.AfterSequenceNo)
		agenticOpts = append(agenticOpts, agenticopenai.WithExtraFields(map[string]any{
			"previous_response_id": strings.TrimSpace(continuation.ResponseID),
		}))
	}

	messages, err := toAgenticMessages(effectiveSystemPrompt, effectiveHistory)
	if err != nil {
		return nil, err
	}

	result := &ExecutionResult{
		Messages: append([]ExecutionMessage(nil), history...),
	}

	for step := 0; step < maxSteps; step++ {
		assistantMsg, assistantBlocks, usage, continuation, err := generateAssistantAgenticMessage(ctx, modelWithTools, messages, onEvent, agenticOpts...)
		if err != nil {
			return nil, err
		}
		result.Usage.InputTokens += usage.InputTokens
		result.Usage.OutputTokens += usage.OutputTokens
		result.AssistantBlocks = assistantBlocks
		result.AssistantText = extractTextFromExecutionBlocks(assistantBlocks)
		result.ProviderContinuation = continuation
		result.Messages = append(result.Messages, ExecutionMessage{
			Role:    "assistant",
			Content: result.AssistantText,
			Blocks:  assistantBlocks,
		})
		messages = append(messages, assistantMsg)

		toolCalls := executionToolCalls(assistantBlocks)
		if len(toolCalls) == 0 {
			return result, nil
		}

		stopAfterToolRound := false
		for _, toolCall := range toolCalls {
			argsJSON := normalizeToolArguments(string(toolCall.Input))
			toolName := toolCall.ToolName

			if onEvent != nil {
				onEvent(ExecutionEvent{
					Type:       "tool_call_started",
					ToolCallID: toolCall.ToolCallID,
					ToolName:   toolName,
					ToolInput:  toolInputForEvent(toolName, argsJSON),
				})
			}

			start := time.Now()
			output, toolErr := registry.ExecuteAllowed(execCtx, toolName, argsJSON)
			duration := time.Since(start)
			isError := toolErr != nil
			if toolErr != nil {
				output = toolErr.Error()
			}

			summary := truncate(output, 500)
			result.ToolInvocations = append(result.ToolInvocations, appmodel.ToolInvocation{
				ToolName:      toolName,
				Input:         argsJSON,
				OutputSummary: summary,
				DurationMs:    duration.Milliseconds(),
			})
			if onEvent != nil {
				onEvent(ExecutionEvent{
					Type:          "tool_call_finished",
					ToolCallID:    toolCall.ToolCallID,
					ToolName:      toolName,
					OutputSummary: summary,
					DurationMs:    duration.Milliseconds(),
				})
			}

			result.Messages = append(result.Messages, ExecutionMessage{
				Role:    "tool",
				Content: output,
				Blocks: []ExecutionBlock{{
					Type:       ExecutionBlockTypeToolResult,
					ToolCallID: toolCall.ToolCallID,
					ToolName:   toolName,
					Input:      argsJSON,
					Output:     output,
					IsError:    isError,
				}},
			})
			messages = append(messages, schema.FunctionToolResultAgenticMessage(toolCall.ToolCallID, toolName, output))
			if IsHumanInteractionTool(toolName) {
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

func streamAssistantMessage(
	ctx context.Context,
	model einomodel.BaseChatModel,
	messages []*schema.Message,
	onEvent func(ExecutionEvent),
) (*schema.Message, []ExecutionBlock, ExecutionUsage, error) {
	reader, err := model.Stream(ctx, messages)
	if err != nil {
		return nil, nil, ExecutionUsage{}, err
	}
	defer reader.Close()

	var chunks []*schema.Message
	started := false

	for {
		chunk, recvErr := reader.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			return nil, nil, ExecutionUsage{}, recvErr
		}
		if chunk == nil {
			continue
		}
		chunks = append(chunks, chunk)
		if !started {
			started = true
			if onEvent != nil {
				onEvent(ExecutionEvent{Type: "assistant_message_started"})
			}
		}
		if text := chunkTextDelta(chunk); text != "" && onEvent != nil {
			onEvent(ExecutionEvent{
				Type: "assistant_message_delta",
				Text: text,
			})
		}
	}

	if len(chunks) == 0 {
		return schema.AssistantMessage("", nil), nil, ExecutionUsage{}, nil
	}

	finalMsg, err := schema.ConcatMessages(chunks)
	if err != nil {
		return nil, nil, ExecutionUsage{}, err
	}
	blocks := fromSchemaAssistantMessage(finalMsg)
	if onEvent != nil {
		onEvent(ExecutionEvent{
			Type: "assistant_message_completed",
			Text: extractTextFromExecutionBlocks(blocks),
		})
	}

	usage := ExecutionUsage{}
	if finalMsg.ResponseMeta != nil && finalMsg.ResponseMeta.Usage != nil {
		usage.InputTokens = finalMsg.ResponseMeta.Usage.PromptTokens
		usage.OutputTokens = finalMsg.ResponseMeta.Usage.CompletionTokens
	}
	return finalMsg, blocks, usage, nil
}

func generateAssistantAgenticMessage(
	ctx context.Context,
	model einomodel.AgenticModel,
	messages []*schema.AgenticMessage,
	onEvent func(ExecutionEvent),
	opts ...einomodel.Option,
) (*schema.AgenticMessage, []ExecutionBlock, ExecutionUsage, *ProviderContinuation, error) {
	if onEvent != nil {
		onEvent(ExecutionEvent{Type: "assistant_message_started"})
	}
	finalMsg, err := model.Generate(ctx, messages, opts...)
	if err != nil {
		return nil, nil, ExecutionUsage{}, nil, err
	}
	if finalMsg == nil {
		finalMsg = &schema.AgenticMessage{Role: schema.AgenticRoleTypeAssistant}
	}
	blocks := fromSchemaAgenticAssistantMessage(finalMsg)
	if onEvent != nil {
		onEvent(ExecutionEvent{
			Type: "assistant_message_completed",
			Text: extractTextFromExecutionBlocks(blocks),
		})
	}

	usage := ExecutionUsage{}
	if finalMsg.ResponseMeta != nil && finalMsg.ResponseMeta.TokenUsage != nil {
		usage.InputTokens = finalMsg.ResponseMeta.TokenUsage.PromptTokens
		usage.OutputTokens = finalMsg.ResponseMeta.TokenUsage.CompletionTokens
	}
	return finalMsg, blocks, usage, providerContinuationFromAgenticMessage(finalMsg), nil
}

func toSchemaMessages(systemPrompt string, history []ExecutionMessage) ([]*schema.Message, error) {
	messages := make([]*schema.Message, 0, len(history)+1)
	if strings.TrimSpace(systemPrompt) != "" {
		messages = append(messages, schema.SystemMessage(systemPrompt))
	}
	for _, msg := range history {
		switch msg.Role {
		case "user":
			messages = append(messages, schema.UserMessage(nonEmptyText(msg.Content, extractTextFromExecutionBlocks(msg.Blocks))))
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
					assistant.ToolCalls = append(assistant.ToolCalls, schema.ToolCall{
						ID:   block.ToolCallID,
						Type: "function",
						Function: schema.FunctionCall{
							Name:      block.ToolName,
							Arguments: string(block.Input),
						},
					})
				}
			}
			messages = append(messages, assistant)
		case "tool":
			if len(msg.Blocks) == 0 {
				messages = append(messages, schema.ToolMessage(msg.Content, "", schema.WithToolName("")))
				continue
			}
			for _, block := range msg.Blocks {
				if block.Type != ExecutionBlockTypeToolResult {
					continue
				}
				messages = append(messages, schema.ToolMessage(block.Output, block.ToolCallID, schema.WithToolName(block.ToolName)))
			}
		default:
			return nil, fmt.Errorf("unsupported execution message role %q", msg.Role)
		}
	}
	return messages, nil
}

func toAgenticMessages(systemPrompt string, history []ExecutionMessage) ([]*schema.AgenticMessage, error) {
	messages := make([]*schema.AgenticMessage, 0, len(history)+1)
	if strings.TrimSpace(systemPrompt) != "" {
		messages = append(messages, schema.SystemAgenticMessage(systemPrompt))
	}
	for _, msg := range history {
		switch msg.Role {
		case "user":
			messages = append(messages, schema.UserAgenticMessage(nonEmptyText(msg.Content, extractTextFromExecutionBlocks(msg.Blocks))))
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
					assistant.ContentBlocks = append(assistant.ContentBlocks, schema.NewContentBlock(&schema.FunctionToolCall{
						CallID:    block.ToolCallID,
						Name:      block.ToolName,
						Arguments: string(block.Input),
					}))
				}
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
				messages = append(messages, schema.FunctionToolResultAgenticMessage(block.ToolCallID, block.ToolName, block.Output))
			}
		default:
			return nil, fmt.Errorf("unsupported execution message role %q", msg.Role)
		}
	}
	return messages, nil
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

func extractTextFromExecutionBlocks(blocks []ExecutionBlock) string {
	var parts []string
	for _, block := range blocks {
		if block.Type == ExecutionBlockTypeText && strings.TrimSpace(block.Text) != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
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
		return json.RawMessage(trimmed)
	}
	encoded, _ := json.Marshal(map[string]string{"raw": trimmed})
	return json.RawMessage(encoded)
}

func summarizeToolInput(input json.RawMessage) string {
	return truncate(strings.TrimSpace(string(input)), 200)
}

func toolInputForEvent(toolName string, input json.RawMessage) string {
	if strings.TrimSpace(toolName) == ToolPublishPreview {
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
	requiredSet := map[string]bool{}
	if required, ok := schemaMap["required"].([]string); ok {
		for _, name := range required {
			requiredSet[name] = true
		}
	} else if required, ok := schemaMap["required"].([]interface{}); ok {
		for _, entry := range required {
			if name, ok := entry.(string); ok {
				requiredSet[name] = true
			}
		}
	}
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
			requiredSet := map[string]bool{}
			if required, ok := raw["required"].([]interface{}); ok {
				for _, item := range required {
					if name, ok := item.(string); ok {
						requiredSet[name] = true
					}
				}
			}
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
