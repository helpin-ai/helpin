package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	einoclaude "github.com/cloudwego/eino-ext/components/model/claude"
	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	appmodel "github.com/helpin-ai/helpin/server/internal/model"
)

const (
	ExecutionBlockTypeText       = "text"
	ExecutionBlockTypeToolCall   = "tool_call"
	ExecutionBlockTypeToolResult = "tool_result"
)

var ErrMaxToolStepsReached = errors.New("agent reached max tool steps")

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
	Role    string           `json:"role"`
	Content string           `json:"content,omitempty"`
	Blocks  []ExecutionBlock `json:"blocks,omitempty"`
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
	Messages        []ExecutionMessage
	AssistantBlocks []ExecutionBlock
	AssistantText   string
	ToolInvocations []appmodel.ToolInvocation
	Usage           ExecutionUsage
	MaxStepsReached bool
}

type EinoModelFactory struct {
	AnthropicAPIKey string
	OpenAIAPIKey    string
	OpenAIBaseURL   string
	OpenRouterKey   string
	OpenRouterURL   string
}

func (f *EinoModelFactory) Resolve(ctx context.Context, agent *appmodel.Agent, tools []ToolDefinition) (einomodel.ToolCallingChatModel, string, error) {
	provider := "anthropic"
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

func (f *EinoModelFactory) resolveBaseModel(ctx context.Context, provider, modelName string) (einomodel.ToolCallingChatModel, error) {
	switch provider {
	case "anthropic":
		if strings.TrimSpace(f.AnthropicAPIKey) == "" {
			return nil, fmt.Errorf("anthropic API key is not configured")
		}
		return einoclaude.NewChatModel(ctx, &einoclaude.Config{
			APIKey: f.AnthropicAPIKey,
			Model:  modelName,
		})
	case "openai":
		if strings.TrimSpace(f.OpenAIAPIKey) == "" {
			return nil, fmt.Errorf("openai API key is not configured")
		}
		return einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{
			APIKey:  f.OpenAIAPIKey,
			BaseURL: strings.TrimSpace(f.OpenAIBaseURL),
			Model:   modelName,
		})
	case "openrouter":
		if strings.TrimSpace(f.OpenRouterKey) == "" {
			return nil, fmt.Errorf("openrouter API key is not configured")
		}
		baseURL := strings.TrimSpace(f.OpenRouterURL)
		if baseURL == "" {
			baseURL = "https://openrouter.ai/api/v1"
		}
		return einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{
			APIKey:  f.OpenRouterKey,
			BaseURL: baseURL,
			Model:   modelName,
		})
	default:
		return nil, fmt.Errorf("unsupported provider %q", provider)
	}
}

func defaultModelForProvider(provider string) string {
	switch provider {
	case "openai":
		return "gpt-4.1"
	case "openrouter":
		return "openai/gpt-4.1"
	default:
		return "claude-sonnet-4-20250514"
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

	modelWithTools, _, err := factory.Resolve(ctx, agent, tools)
	if err != nil {
		return nil, err
	}

	toolDefsByName := make(map[string]ToolDefinition, len(tools))
	for _, def := range tools {
		toolDefsByName[def.Name] = def
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

		for _, toolCall := range assistantMsg.ToolCalls {
			argsJSON := normalizeToolArguments(toolCall.Function.Arguments)
			toolName := toolCall.Function.Name
			if _, ok := toolDefsByName[toolName]; !ok {
				return nil, fmt.Errorf("model requested unknown tool %q", toolName)
			}

			if onEvent != nil {
				onEvent(ExecutionEvent{
					Type:       "tool_call_started",
					ToolCallID: toolCall.ID,
					ToolName:   toolName,
					ToolInput:  summarizeToolInput(argsJSON),
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
