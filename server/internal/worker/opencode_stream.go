package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	appmodel "github.com/helpin-ai/helpin/server/internal/model"
)

type openCodeParsedJSONEvent struct {
	EventType    string
	PartType     string
	DisplayLine  string
	ResponseText string
	TokensUsed   int
	Usage        ExecutionUsage
	EventError   string
	Properties   map[string]any
	Part         map[string]any
	DeltaField   string
	Delta        string
}

type openCodeToolCallState struct {
	ToolCallID      string
	PartID          string
	ToolName        string
	ParentMessageID string
	ToolInput       string
	ArgsText        string
	StartedAt       time.Time
	Started         bool
	Finished        bool
	ResultMessageID string
}

type openCodeStreamCollector struct {
	runID   string
	writer  *openCodeArtifactWriter
	onEvent func(ExecutionEvent)
	mu      sync.Mutex

	stdoutFull  strings.Builder
	stderrFull  strings.Builder
	stdoutChunk strings.Builder
	stderrChunk strings.Builder
	response    strings.Builder
	eventError  string
	tokensUsed  int

	usage              ExecutionUsage
	assistantMessageID string
	assistantStarted   bool
	assistantCompleted bool
	reasoningMessageID string
	reasoningStarted   bool
	reasoningCompleted bool
	reasoningText      strings.Builder
	assistantPartText  map[string]string
	reasoningPartText  map[string]string
	toolPartToCallID   map[string]string
	liveTools          map[string]*openCodeToolCallState
	toolInvocations    []appmodel.ToolInvocation
	legacyToolUses     int
	currentActivityID  string
}

func newOpenCodeStreamCollector(runID string, writer *openCodeArtifactWriter, onEvent func(ExecutionEvent)) *openCodeStreamCollector {
	return &openCodeStreamCollector{
		runID:             runID,
		writer:            writer,
		onEvent:           onEvent,
		assistantPartText: map[string]string{},
		reasoningPartText: map[string]string{},
		toolPartToCallID:  map[string]string{},
		liveTools:         map[string]*openCodeToolCallState{},
	}
}

func (c *openCodeStreamCollector) AppendLine(ctx context.Context, stream, line string) {
	logLine := strings.TrimSpace(line)
	if logLine != "" {
		slog.DebugContext(ctx, "opencode stream line",
			"run_id", shortRunID(c.runID),
			"stream", stream,
			"line", truncate(logLine, 1000),
		)
	}

	c.mu.Lock()
	full, chunk := c.buffersFor(stream)
	full.WriteString(line)
	full.WriteByte('\n')
	chunk.WriteString(line)
	chunk.WriteByte('\n')
	shouldFlush := chunk.Len() >= openCodeChunkFlushBytes
	c.mu.Unlock()

	if shouldFlush {
		c.FlushStream(ctx, stream, true)
	}
}

func (c *openCodeStreamCollector) AppendResponseText(text string) {
	if text == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.response.WriteString(text)
}

func (c *openCodeStreamCollector) AddTokens(tokens int) {
	if tokens <= 0 {
		return
	}
	c.mu.Lock()
	c.tokensUsed += tokens
	c.mu.Unlock()
}

func (c *openCodeStreamCollector) SetUsage(usage ExecutionUsage) {
	if usage.InputTokens <= 0 && usage.OutputTokens <= 0 {
		return
	}
	c.usage = usage
}

func (c *openCodeStreamCollector) SetEventError(message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}

	c.mu.Lock()
	if c.eventError == "" {
		c.eventError = message
	}
	c.mu.Unlock()
}

func (c *openCodeStreamCollector) FlushLoop(ctx context.Context, done <-chan struct{}) {
	ticker := time.NewTicker(openCodeChunkFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.FlushPending(ctx, true)
		case <-done:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (c *openCodeStreamCollector) FlushPending(ctx context.Context, notify bool) {
	c.FlushStream(ctx, "stdout", notify)
	c.FlushStream(ctx, "stderr", notify)
}

func (c *openCodeStreamCollector) FlushStream(ctx context.Context, stream string, notify bool) {
	c.mu.Lock()
	_, chunk := c.buffersFor(stream)
	content := chunk.String()
	chunk.Reset()
	c.mu.Unlock()

	if content == "" {
		return
	}
	c.writer.Save(ctx, "opencode_"+stream+"_chunk", "text", content, notify)
}

func (c *openCodeStreamCollector) Outputs() (string, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stdoutFull.String(), c.stderrFull.String()
}

func (c *openCodeStreamCollector) ResponseText() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.response.String()
}

func (c *openCodeStreamCollector) EventError() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.eventError
}

func (c *openCodeStreamCollector) TokensUsed() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tokensUsed
}

func (c *openCodeStreamCollector) Usage() ExecutionUsage {
	return c.usage
}

func (c *openCodeStreamCollector) ToolInvocations() []appmodel.ToolInvocation {
	if len(c.toolInvocations) == 0 {
		return nil
	}
	invocations := make([]appmodel.ToolInvocation, len(c.toolInvocations))
	copy(invocations, c.toolInvocations)
	return invocations
}

func (c *openCodeStreamCollector) buffersFor(stream string) (*strings.Builder, *strings.Builder) {
	if stream == "stderr" {
		return &c.stderrFull, &c.stderrChunk
	}
	return &c.stdoutFull, &c.stdoutChunk
}

func consumeOpenCodeTextStream(reader io.Reader, stream string, ctx context.Context, collector *openCodeStreamCollector, wg *sync.WaitGroup, errCh chan<- error) {
	defer wg.Done()

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), openCodeScannerBufferSize)
	for scanner.Scan() {
		collector.AppendLine(ctx, stream, stripANSI(scanner.Text()))
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		errCh <- fmt.Errorf("read %s: %w", stream, err)
		return
	}
	errCh <- nil
}

func consumeOpenCodeJSONStream(reader io.Reader, ctx context.Context, collector *openCodeStreamCollector, wg *sync.WaitGroup, errCh chan<- error) {
	defer wg.Done()

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), openCodeScannerBufferSize)
	for scanner.Scan() {
		line := strings.TrimSpace(stripANSI(scanner.Text()))
		if line == "" {
			continue
		}

		parsed, err := parseOpenCodeJSONEvent(line)
		if err != nil {
			slog.DebugContext(ctx, "treating non-json opencode stdout line as plain text",
				"error", err,
				"run_id", shortRunID(collector.runID),
			)
			collector.AppendLine(ctx, "stdout", line)
			continue
		}
		if strings.TrimSpace(parsed.DisplayLine) != "" {
			collector.AppendLine(ctx, "stdout", parsed.DisplayLine)
		}
		if parsed.TokensUsed > 0 {
			collector.AddTokens(parsed.TokensUsed)
		}
		if parsed.Usage.InputTokens > 0 || parsed.Usage.OutputTokens > 0 {
			collector.SetUsage(parsed.Usage)
		}
		if parsed.EventError != "" {
			collector.SetEventError(parsed.EventError)
		}
		collector.ApplyParsedJSONEvent(parsed)
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		errCh <- fmt.Errorf("read stdout: %w", err)
		return
	}
	collector.FinalizeExecutionEvents()
	errCh <- nil
}

func parseOpenCodeJSONEvent(line string) (openCodeParsedJSONEvent, error) {
	var event map[string]any
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return openCodeParsedJSONEvent{}, fmt.Errorf("unmarshal event line %q: %w", truncate(line, 200), err)
	}

	eventType := lookupString(event, "type")
	properties, _ := event["properties"].(map[string]any)
	part := openCodeEventPart(event, properties)
	parsed := openCodeParsedJSONEvent{
		EventType:  eventType,
		PartType:   lookupString(part, "type"),
		Properties: properties,
		Part:       part,
	}

	switch eventType {
	case "text":
		parsed.ResponseText = lookupString(part, "text")
		parsed.DisplayLine = parsed.ResponseText
		return parsed, nil
	case "tool_use":
		parsed.DisplayLine = renderOpenCodeToolUse(part)
		return parsed, nil
	case "step_start":
		parsed.DisplayLine = renderOpenCodeStepStart(part)
		return parsed, nil
	case "step_finish":
		parsed.TokensUsed = openCodeTokensFromPart(part)
		parsed.Usage = openCodeUsageFromPart(part)
		parsed.DisplayLine = renderOpenCodeStepFinish(part, parsed.TokensUsed)
		return parsed, nil
	case "error":
		parsed.EventError = firstNonEmptyText(
			lookupString(event, "error"),
			lookupString(event, "message"),
			lookupString(part, "error"),
			lookupString(part, "message"),
			renderOpenCodeRawPart(part),
		)
		parsed.DisplayLine = strings.TrimSpace(firstNonEmptyText("Error: "+strings.TrimSpace(parsed.EventError), parsed.EventError))
		parsed.EventError = strings.TrimSpace(parsed.EventError)
		return parsed, nil
	case "message.part.updated":
		switch parsed.PartType {
		case "text":
			parsed.ResponseText = lookupString(part, "text")
		case "tool":
			parsed.DisplayLine = renderOpenCodeToolPart(part)
		case "step-start":
			parsed.DisplayLine = renderOpenCodeStepStart(part)
		case "step-finish":
			parsed.TokensUsed = openCodeTokensFromPart(part)
			parsed.Usage = openCodeUsageFromPart(part)
			parsed.DisplayLine = renderOpenCodeStepFinish(part, parsed.TokensUsed)
		}
		return parsed, nil
	case "message.part.delta":
		parsed.DeltaField = lookupString(properties, "field")
		parsed.Delta = lookupString(properties, "delta")
		if strings.HasSuffix(parsed.DeltaField, "text") {
			parsed.ResponseText = parsed.Delta
		}
		return parsed, nil
	default:
		parsed.DisplayLine = renderOpenCodeGenericEvent(eventType, part)
		return parsed, nil
	}
}

func (c *openCodeStreamCollector) ApplyParsedJSONEvent(parsed openCodeParsedJSONEvent) {
	switch parsed.EventType {
	case "text":
		messageID := c.ensureAssistantMessageID(firstNonEmptyText(
			lookupString(parsed.Part, "messageID"),
			lookupString(parsed.Part, "id"),
		))
		partID := firstNonEmptyText(lookupString(parsed.Part, "id"), messageID)
		c.applyAssistantTextSnapshot(messageID, partID, parsed.ResponseText)
	case "tool_use":
		c.applyLegacyToolUse(parsed.Part)
	case "step_start":
		c.emitActivitySnapshot(parsed.Part, parsed.DisplayLine)
	case "step_finish":
		c.emitActivityDelta(parsed.Part, parsed.DisplayLine)
		c.completeReasoningStream()
		c.completeAssistantStream()
	case "message.part.updated":
		c.applyUpdatedPart(parsed)
	case "message.part.delta":
		c.applyPartDelta(parsed)
	}
}

func (c *openCodeStreamCollector) FinalizeExecutionEvents() {
	for _, tool := range c.liveTools {
		if tool != nil && !tool.Finished {
			c.finishToolCall(tool, "", "", time.Since(tool.StartedAt).Milliseconds())
		}
	}
	c.completeReasoningStream()
	if strings.TrimSpace(c.assistantMessageID) != "" && !c.assistantCompleted {
		c.completeAssistantStream()
	}
}

func (c *openCodeStreamCollector) applyUpdatedPart(parsed openCodeParsedJSONEvent) {
	switch parsed.PartType {
	case "text":
		messageID := c.ensureAssistantMessageID(lookupString(parsed.Part, "messageID"))
		partID := firstNonEmptyText(lookupString(parsed.Part, "id"), messageID)
		c.applyAssistantTextSnapshot(messageID, partID, parsed.ResponseText)
		if openCodePartHasEnded(parsed.Part) {
			c.completeAssistantStream()
		}
	case "reasoning":
		messageID := c.ensureReasoningMessageID(firstNonEmptyText(lookupString(parsed.Part, "id"), lookupString(parsed.Part, "messageID")))
		partID := firstNonEmptyText(lookupString(parsed.Part, "id"), messageID)
		c.applyReasoningSnapshot(messageID, partID, lookupString(parsed.Part, "text"))
		if openCodePartHasEnded(parsed.Part) {
			c.completeReasoningStream()
		}
	case "tool":
		c.applyToolPart(parsed.Part)
	case "step-start":
		c.emitActivitySnapshot(parsed.Part, parsed.DisplayLine)
	case "step-finish":
		c.emitActivityDelta(parsed.Part, parsed.DisplayLine)
		c.completeReasoningStream()
		c.completeAssistantStream()
	}
}

func (c *openCodeStreamCollector) applyPartDelta(parsed openCodeParsedJSONEvent) {
	if parsed.Delta == "" {
		return
	}
	messageID := firstNonEmptyText(
		lookupString(parsed.Properties, "messageID"),
		c.assistantMessageID,
	)
	partID := lookupString(parsed.Properties, "partID")
	switch {
	case strings.HasSuffix(parsed.DeltaField, "text"):
		if _, ok := c.reasoningPartText[partID]; ok {
			reasoningID := c.ensureReasoningMessageID(firstNonEmptyText(c.reasoningMessageID, partID, messageID))
			c.applyReasoningDelta(reasoningID, partID, parsed.Delta)
			return
		}
		assistantID := c.ensureAssistantMessageID(messageID)
		c.applyAssistantDelta(assistantID, partID, parsed.Delta)
	case strings.Contains(parsed.DeltaField, "state.input"), strings.Contains(parsed.DeltaField, "state.raw"), strings.HasSuffix(parsed.DeltaField, "input"):
		c.applyToolArgsDelta(messageID, partID, parsed.Delta)
	}
}

func (c *openCodeStreamCollector) ensureAssistantMessageID(preferred string) string {
	preferred = strings.TrimSpace(preferred)
	if preferred != "" && preferred != c.assistantMessageID {
		if c.assistantStarted && !c.assistantCompleted {
			c.completeAssistantStream()
		}
		c.assistantMessageID = preferred
		c.assistantCompleted = false
	}
	if strings.TrimSpace(c.assistantMessageID) == "" {
		c.assistantMessageID = uuid.NewString()
	}
	return c.assistantMessageID
}

func (c *openCodeStreamCollector) ensureReasoningMessageID(preferred string) string {
	preferred = strings.TrimSpace(preferred)
	if preferred != "" && preferred != c.reasoningMessageID {
		if c.reasoningStarted && !c.reasoningCompleted {
			c.completeReasoningStream()
		}
		c.reasoningMessageID = preferred
		c.reasoningCompleted = false
	}
	if strings.TrimSpace(c.reasoningMessageID) == "" {
		c.reasoningMessageID = uuid.NewString()
	}
	return c.reasoningMessageID
}

func (c *openCodeStreamCollector) applyAssistantTextSnapshot(messageID, partID, text string) {
	if partID == "" {
		partID = messageID
	}
	previous := c.assistantPartText[partID]
	c.assistantPartText[partID] = text
	c.applyAssistantDelta(messageID, partID, openCodeDeltaFromSnapshot(previous, text))
}

func (c *openCodeStreamCollector) applyAssistantDelta(messageID, partID, delta string) {
	if delta == "" {
		return
	}
	messageID = c.ensureAssistantMessageID(messageID)
	if !c.assistantStarted {
		c.assistantStarted = true
		c.emitEvent(ExecutionEvent{Type: "assistant_message_started", MessageID: messageID})
	}
	c.AppendResponseText(delta)
	c.emitEvent(ExecutionEvent{
		Type:      "assistant_message_delta",
		MessageID: messageID,
		Text:      delta,
		Content:   delta,
	})
}

func (c *openCodeStreamCollector) completeAssistantStream() {
	if strings.TrimSpace(c.assistantMessageID) == "" || c.assistantCompleted {
		return
	}
	c.assistantCompleted = true
	c.emitEvent(ExecutionEvent{
		Type:      "assistant_message_completed",
		MessageID: c.assistantMessageID,
		Text:      c.ResponseText(),
		Content:   c.ResponseText(),
	})
}

func (c *openCodeStreamCollector) applyReasoningSnapshot(messageID, partID, text string) {
	if partID == "" {
		partID = messageID
	}
	previous := c.reasoningPartText[partID]
	c.reasoningPartText[partID] = text
	c.applyReasoningDelta(messageID, partID, openCodeDeltaFromSnapshot(previous, text))
}

func (c *openCodeStreamCollector) applyReasoningDelta(messageID, partID, delta string) {
	if delta == "" {
		return
	}
	messageID = c.ensureReasoningMessageID(messageID)
	if !c.reasoningStarted {
		c.reasoningStarted = true
		c.emitEvent(ExecutionEvent{Type: "reasoning_message_started", MessageID: messageID})
	}
	c.reasoningText.WriteString(delta)
	c.emitEvent(ExecutionEvent{
		Type:      "reasoning_message_delta",
		MessageID: messageID,
		Text:      delta,
		Content:   delta,
	})
	if partID != "" {
		c.reasoningPartText[partID] = c.reasoningPartText[partID]
	}
}

func (c *openCodeStreamCollector) completeReasoningStream() {
	if strings.TrimSpace(c.reasoningMessageID) == "" || c.reasoningCompleted {
		return
	}
	c.reasoningCompleted = true
	content := c.reasoningText.String()
	c.emitEvent(ExecutionEvent{
		Type:      "reasoning_message_completed",
		MessageID: c.reasoningMessageID,
		Text:      content,
		Content:   content,
	})
}

func (c *openCodeStreamCollector) applyLegacyToolUse(part map[string]any) {
	toolCallID := firstNonEmptyText(lookupString(part, "callID"), lookupString(part, "id"))
	if toolCallID == "" {
		c.legacyToolUses++
		toolCallID = fmt.Sprintf("%s:legacy-tool:%d", shortRunID(c.runID), c.legacyToolUses)
	}
	parentMessageID := c.ensureAssistantMessageID(lookupString(part, "messageID"))
	toolName := openCodeToolName(part, nil)
	if toolName == "" {
		toolName = "tool"
	}
	toolInput := openCodeToolInputForState(part, nil)
	argsText := firstNonEmptyText(openCodeNormalizedToolInput(toolInput), renderOpenCodeToolUse(part))
	tool := &openCodeToolCallState{
		ToolCallID:      toolCallID,
		PartID:          lookupString(part, "id"),
		ToolName:        toolName,
		ParentMessageID: parentMessageID,
		ToolInput:       openCodeNormalizedToolInput(toolInput),
		ArgsText:        argsText,
		StartedAt:       time.Now(),
	}
	c.startToolCall(tool)
	c.finishToolCall(tool, "", "", 0)
}

func (c *openCodeStreamCollector) applyToolPart(part map[string]any) {
	state, _ := part["state"].(map[string]any)
	toolCallID := firstNonEmptyText(lookupString(part, "callID"), lookupString(part, "id"))
	if toolCallID == "" {
		toolCallID = uuid.NewString()
	}
	partID := lookupString(part, "id")
	if partID == "" {
		partID = toolCallID
	}
	parentMessageID := c.ensureAssistantMessageID(lookupString(part, "messageID"))
	tool := c.liveTools[toolCallID]
	if tool == nil {
		tool = &openCodeToolCallState{
			ToolCallID:      toolCallID,
			PartID:          partID,
			StartedAt:       time.Now(),
			ResultMessageID: uuid.NewString(),
		}
		c.liveTools[toolCallID] = tool
	}
	tool.PartID = partID
	tool.ParentMessageID = parentMessageID
	tool.ToolName = firstNonEmptyText(openCodeToolName(part, state), tool.ToolName, "tool")
	tool.ToolInput = openCodeNormalizedToolInput(openCodeToolInputForState(part, state))
	tool.ArgsText = firstNonEmptyText(tool.ToolInput, openCodeToolArgsText(part, state), tool.ArgsText)
	c.toolPartToCallID[partID] = toolCallID

	if !tool.Started {
		c.startToolCall(tool)
	}

	status := strings.TrimSpace(lookupString(state, "status"))
	switch status {
	case "pending", "running":
		if tool.ArgsText != "" {
			c.emitToolArgsDelta(tool, tool.ArgsText, true)
		}
	case "completed":
		if tool.ArgsText != "" {
			c.emitToolArgsDelta(tool, tool.ArgsText, true)
		}
		c.finishToolCall(tool, lookupString(state, "output"), "", openCodeToolDurationMs(state))
	case "error":
		if tool.ArgsText != "" {
			c.emitToolArgsDelta(tool, tool.ArgsText, true)
		}
		errorText := firstNonEmptyText(lookupString(state, "error"), lookupString(state, "output"))
		c.finishToolCall(tool, errorText, errorText, openCodeToolDurationMs(state))
	}
}

func (c *openCodeStreamCollector) applyToolArgsDelta(messageID, partID, delta string) {
	if delta == "" {
		return
	}
	toolCallID := firstNonEmptyText(c.toolPartToCallID[partID], partID)
	if toolCallID == "" {
		return
	}
	tool := c.liveTools[toolCallID]
	if tool == nil {
		tool = &openCodeToolCallState{
			ToolCallID:      toolCallID,
			PartID:          partID,
			ToolName:        "tool",
			ParentMessageID: c.ensureAssistantMessageID(messageID),
			StartedAt:       time.Now(),
			ResultMessageID: uuid.NewString(),
		}
		c.liveTools[toolCallID] = tool
	}
	if !tool.Started {
		c.startToolCall(tool)
	}
	c.emitToolArgsDelta(tool, delta, false)
}

func (c *openCodeStreamCollector) startToolCall(tool *openCodeToolCallState) {
	if tool == nil || tool.Started {
		return
	}
	tool.Started = true
	if strings.TrimSpace(tool.ResultMessageID) == "" {
		tool.ResultMessageID = uuid.NewString()
	}
	c.emitEvent(ExecutionEvent{
		Type:            "tool_call_started",
		ToolCallID:      tool.ToolCallID,
		ToolName:        tool.ToolName,
		ToolInput:       tool.ToolInput,
		ParentMessageID: tool.ParentMessageID,
		ArgsText:        tool.ArgsText,
	})
}

func (c *openCodeStreamCollector) emitToolArgsDelta(tool *openCodeToolCallState, value string, replace bool) {
	if tool == nil || strings.TrimSpace(value) == "" {
		return
	}
	delta := value
	if replace {
		if tool.ArgsText == value {
			return
		}
		delta = openCodeDeltaFromSnapshot(tool.ArgsText, value)
		tool.ArgsText = value
	} else {
		tool.ArgsText += value
	}
	if delta == "" {
		return
	}
	c.emitEvent(ExecutionEvent{
		Type:            "tool_call_args_delta",
		ToolCallID:      tool.ToolCallID,
		ToolName:        tool.ToolName,
		ParentMessageID: tool.ParentMessageID,
		ArgsDelta:       delta,
		ArgsText:        tool.ArgsText,
	})
}

func (c *openCodeStreamCollector) finishToolCall(tool *openCodeToolCallState, outputSummary, errorText string, durationMs int64) {
	if tool == nil || tool.Finished {
		return
	}
	tool.Finished = true
	outputSummary = strings.TrimSpace(outputSummary)
	errorText = strings.TrimSpace(errorText)
	if durationMs == 0 && !tool.StartedAt.IsZero() {
		durationMs = time.Since(tool.StartedAt).Milliseconds()
	}
	if outputSummary != "" || errorText != "" {
		c.emitEvent(ExecutionEvent{
			Type:            "tool_call_result",
			ToolCallID:      tool.ToolCallID,
			ToolName:        tool.ToolName,
			ParentMessageID: tool.ParentMessageID,
			ResultMessageID: tool.ResultMessageID,
			Content:         firstNonEmptyText(outputSummary, errorText),
			OutputSummary:   firstNonEmptyText(outputSummary, errorText),
			Error:           errorText,
		})
	}
	c.emitEvent(ExecutionEvent{
		Type:            "tool_call_finished",
		ToolCallID:      tool.ToolCallID,
		ToolName:        tool.ToolName,
		ParentMessageID: tool.ParentMessageID,
		ResultMessageID: tool.ResultMessageID,
		Content:         firstNonEmptyText(outputSummary, errorText),
		OutputSummary:   firstNonEmptyText(outputSummary, errorText),
		DurationMs:      durationMs,
		Error:           errorText,
	})
	c.toolInvocations = append(c.toolInvocations, appmodel.ToolInvocation{
		ToolName:      tool.ToolName,
		Input:         json.RawMessage(firstNonEmptyText(tool.ToolInput, `{}`)),
		OutputSummary: firstNonEmptyText(outputSummary, errorText),
		DurationMs:    durationMs,
	})
	delete(c.liveTools, tool.ToolCallID)
	if tool.PartID != "" {
		delete(c.toolPartToCallID, tool.PartID)
	}
}

func (c *openCodeStreamCollector) emitActivitySnapshot(part map[string]any, content string) {
	activityID := firstNonEmptyText(lookupString(part, "stepID"), lookupString(part, "id"))
	if activityID == "" {
		activityID = uuid.NewString()
	}
	c.currentActivityID = activityID
	c.emitEvent(ExecutionEvent{
		Type:         "activity_snapshot",
		ActivityID:   activityID,
		ActivityType: "step",
		Content:      content,
		Text:         content,
	})
}

func (c *openCodeStreamCollector) emitActivityDelta(part map[string]any, content string) {
	activityID := firstNonEmptyText(lookupString(part, "stepID"), lookupString(part, "id"), c.currentActivityID)
	if activityID == "" {
		activityID = uuid.NewString()
	}
	c.currentActivityID = activityID
	c.emitEvent(ExecutionEvent{
		Type:         "activity_delta",
		ActivityID:   activityID,
		ActivityType: "step",
		Content:      content,
		Text:         content,
	})
}

func (c *openCodeStreamCollector) emitEvent(event ExecutionEvent) {
	if c.onEvent == nil {
		return
	}
	c.onEvent(event)
}

func openCodeEventPart(event map[string]any, properties map[string]any) map[string]any {
	if part, ok := event["part"].(map[string]any); ok {
		return part
	}
	if part, ok := properties["part"].(map[string]any); ok {
		return part
	}
	return nil
}

func openCodeUsageFromPart(part map[string]any) ExecutionUsage {
	tokens, _ := part["tokens"].(map[string]any)
	if len(tokens) == 0 {
		return ExecutionUsage{}
	}
	cache, _ := tokens["cache"].(map[string]any)
	return ExecutionUsage{
		InputTokens:  lookupInt(tokens, "input") + lookupInt(cache, "read") + lookupInt(cache, "write"),
		OutputTokens: lookupInt(tokens, "output") + lookupInt(tokens, "reasoning"),
	}
}

func openCodeDeltaFromSnapshot(previous, current string) string {
	if current == "" || current == previous {
		return ""
	}
	if previous == "" {
		return current
	}
	if strings.HasPrefix(current, previous) {
		return current[len(previous):]
	}
	return current
}

func openCodePartHasEnded(part map[string]any) bool {
	timeMap, _ := part["time"].(map[string]any)
	if len(timeMap) == 0 {
		return false
	}
	_, ok := timeMap["end"]
	return ok && timeMap["end"] != nil
}

func openCodeToolDurationMs(state map[string]any) int64 {
	timeMap, _ := state["time"].(map[string]any)
	if len(timeMap) == 0 {
		return 0
	}
	start := lookupInt64(timeMap, "start")
	end := lookupInt64(timeMap, "end")
	if start <= 0 || end <= 0 || end < start {
		return 0
	}
	return end - start
}

func openCodeToolName(part map[string]any, state map[string]any) string {
	metadata, _ := part["metadata"].(map[string]any)
	stateMetadata, _ := state["metadata"].(map[string]any)
	return firstNonEmptyText(
		lookupString(part, "tool"),
		lookupString(part, "name"),
		lookupString(state, "title"),
		lookupString(part, "title"),
		lookupString(stateMetadata, "command"),
		lookupString(metadata, "command"),
		lookupString(stateMetadata, "description"),
		lookupString(metadata, "description"),
	)
}

func openCodeToolArgsText(part map[string]any, state map[string]any) string {
	return firstNonEmptyText(
		openCodeNormalizedToolInput(openCodeToolInputForState(part, state)),
		lookupString(state, "title"),
		lookupString(part, "title"),
	)
}

func openCodeToolInputForState(part map[string]any, state map[string]any) string {
	if raw := firstNonEmptyText(
		lookupString(state, "raw"),
		lookupJSONString(state, "input"),
		lookupJSONString(part, "input"),
	); raw != "" {
		return raw
	}
	stateMetadata, _ := state["metadata"].(map[string]any)
	if raw := firstNonEmptyText(
		lookupString(stateMetadata, "command"),
		lookupString(stateMetadata, "path"),
		lookupString(part, "title"),
	); raw != "" {
		return raw
	}
	return ""
}

func openCodeNormalizedToolInput(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	return strings.TrimSpace(string(normalizeToolArguments(raw)))
}

func renderOpenCodeToolPart(part map[string]any) string {
	state, _ := part["state"].(map[string]any)
	if state == nil {
		return renderOpenCodeToolUse(part)
	}
	name := openCodeToolName(part, state)
	if name == "" {
		name = renderOpenCodeToolUse(part)
	}
	status := firstNonEmptyText(lookupString(state, "status"), "running")
	switch status {
	case "completed":
		return "Tool completed: " + name
	case "error":
		return "Tool failed: " + name
	default:
		return "Tool: " + name
	}
}
