package model

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

const CodingSessionStateSnapshotSchemaVersionV1 = "helpin.coding_session.stream.v1"

// CodingSessionStateSnapshot stores the latest in-progress live stream state for a run.
type CodingSessionStateSnapshot struct {
	ID              string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string          `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_coding_session_state_snapshots_run"`
	RunID           string          `json:"run_id" gorm:"type:uuid;not null;uniqueIndex:idx_coding_session_state_snapshots_run"`
	SchemaVersion   string          `json:"schema_version" gorm:"not null;default:'helpin.coding_session.stream.v1'"`
	ThroughSequence int64           `json:"through_sequence" gorm:"not null;default:0"`
	SnapshotPayload json.RawMessage `json:"snapshot_payload" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt       time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CodingSessionStateSnapshot) TableName() string { return "coding_session_state_snapshots" }

func DecodeCodingSessionStreamSnapshot(raw json.RawMessage) (*CodingSessionStreamSnapshot, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" || trimmed == "{}" {
		return nil, nil
	}

	var snapshot CodingSessionStreamSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, err
	}
	if snapshot.IsEmpty() {
		return nil, nil
	}
	return &snapshot, nil
}

func EncodeCodingSessionStreamSnapshot(snapshot *CodingSessionStreamSnapshot) (json.RawMessage, error) {
	if snapshot == nil || snapshot.IsEmpty() {
		return json.RawMessage(`{}`), nil
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func (s *CodingSessionStreamSnapshot) IsEmpty() bool {
	return s == nil || (s.LiveAssistantMessage == nil && s.LiveReasoningMessage == nil && len(s.LiveTurnSegments) == 0 && s.CurrentPlan == nil)
}

func ApplyCodingSessionStreamEvent(snapshot *CodingSessionStreamSnapshot, eventType string, payload map[string]any, timestamp time.Time) *CodingSessionStreamSnapshot {
	if strings.TrimSpace(eventType) == "" {
		return snapshot
	}
	if snapshot == nil {
		snapshot = &CodingSessionStreamSnapshot{}
	}
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}

	switch strings.TrimSpace(eventType) {
	case "assistant.message.started":
		messageID := firstNonEmptySnapshotValue(trimmedSnapshotString(payload["message_id"]), "assistant:"+timestamp.UTC().Format(time.RFC3339Nano))
		snapshot.LiveAssistantMessage = ensureCodingSessionAssistantMessage(snapshot.LiveAssistantMessage, messageID, timestamp.UTC())

	case "assistant.message.delta":
		messageID := firstNonEmptySnapshotValue(
			trimmedSnapshotString(payload["message_id"]),
			snapshotMessageID(snapshot.LiveAssistantMessage),
			"assistant:"+timestamp.UTC().Format(time.RFC3339Nano),
		)
		assistant := ensureCodingSessionAssistantMessage(snapshot.LiveAssistantMessage, messageID, timestamp.UTC())
		deltaText := firstNonEmptySnapshotRawString(payload["content"], payload["text"])
		deltaText = codingSessionStreamDelta(assistant.Content, deltaText)
		assistant.Content += deltaText
		assistant.Status = "streaming"
		snapshot.LiveAssistantMessage = assistant
		appendCodingSessionAssistantSegment(snapshot, messageID, deltaText, timestamp.UTC())

	case "assistant.message.completed":
		messageID := firstNonEmptySnapshotValue(
			trimmedSnapshotString(payload["message_id"]),
			snapshotMessageID(snapshot.LiveAssistantMessage),
			"assistant:"+timestamp.UTC().Format(time.RFC3339Nano),
		)
		assistant := ensureCodingSessionAssistantMessage(snapshot.LiveAssistantMessage, messageID, timestamp.UTC())
		previousContent := assistant.Content
		content := firstNonEmptySnapshotRawString(payload["content"], payload["text"])
		if content != "" {
			assistant.Content = content
		}
		assistant.Status = "completed"
		assistant.CompletedAt = timePtr(timestamp.UTC())
		snapshot.LiveAssistantMessage = assistant
		appendOrMarkCompletedCodingSessionAssistantSegment(snapshot, messageID, previousContent, assistant.Content, timestamp.UTC())
		if messageType := trimmedSnapshotString(payload["message_type"]); messageType != "" {
			assistant.MessageType = messageType
			for _, segment := range snapshot.LiveTurnSegments {
				if segment.AssistantMessage != nil && segment.AssistantMessage.MessageID == messageID {
					segment.AssistantMessage.MessageType = messageType
				}
			}
		}

	case "reasoning.message.started":
		messageID := firstNonEmptySnapshotValue(trimmedSnapshotString(payload["message_id"]), "reasoning:"+timestamp.UTC().Format(time.RFC3339Nano))
		reasoning := ensureCodingSessionReasoningMessage(snapshot.LiveReasoningMessage, messageID, timestamp.UTC())
		if encrypted := trimmedSnapshotString(payload["encrypted_value"]); encrypted != "" {
			reasoning.EncryptedValue = snapshotStringPtr(encrypted)
		}
		snapshot.LiveReasoningMessage = reasoning

	case "reasoning.message.delta":
		messageID := firstNonEmptySnapshotValue(
			trimmedSnapshotString(payload["message_id"]),
			snapshotMessageID(snapshot.LiveReasoningMessage),
			"reasoning:"+timestamp.UTC().Format(time.RFC3339Nano),
		)
		reasoning := ensureCodingSessionReasoningMessage(snapshot.LiveReasoningMessage, messageID, timestamp.UTC())
		reasoning.Content += codingSessionStreamDelta(reasoning.Content, firstNonEmptySnapshotRawString(payload["content"], payload["text"]))
		if encrypted := trimmedSnapshotString(payload["encrypted_value"]); encrypted != "" {
			reasoning.EncryptedValue = snapshotStringPtr(encrypted)
		}
		reasoning.Status = "streaming"
		snapshot.LiveReasoningMessage = reasoning

	case "reasoning.message.completed":
		messageID := firstNonEmptySnapshotValue(
			trimmedSnapshotString(payload["message_id"]),
			snapshotMessageID(snapshot.LiveReasoningMessage),
			"reasoning:"+timestamp.UTC().Format(time.RFC3339Nano),
		)
		reasoning := ensureCodingSessionReasoningMessage(snapshot.LiveReasoningMessage, messageID, timestamp.UTC())
		content := firstNonEmptySnapshotRawString(payload["content"], payload["text"])
		if content != "" && len(content) >= len(reasoning.Content) {
			reasoning.Content = content
		}
		if encrypted := trimmedSnapshotString(payload["encrypted_value"]); encrypted != "" {
			reasoning.EncryptedValue = snapshotStringPtr(encrypted)
		}
		reasoning.Status = "completed"
		reasoning.CompletedAt = timePtr(timestamp.UTC())
		snapshot.LiveReasoningMessage = reasoning

	case "tool.call.started":
		assistant := ensureCodingSessionAssistantMessage(
			snapshot.LiveAssistantMessage,
			firstNonEmptySnapshotValue(
				trimmedSnapshotString(payload["parent_message_id"]),
				snapshotMessageID(snapshot.LiveAssistantMessage),
				"assistant:"+timestamp.UTC().Format(time.RFC3339Nano),
			),
			timestamp.UTC(),
		)
		toolCall := ensureCodingSessionToolCall(assistant, payload, timestamp.UTC())
		toolCall.ArgsText = firstNonEmptySnapshotValue(rawSnapshotString(payload["args_text"]), rawSnapshotString(payload["tool_input"]), toolCall.ArgsText)
		toolCall.Status = "running"
		snapshot.LiveAssistantMessage = assistant
		segmentToolCall := ensureCodingSessionToolCallSegment(snapshot, payload, timestamp.UTC(), assistant.MessageID)
		segmentToolCall.ArgsText = firstNonEmptySnapshotValue(rawSnapshotString(payload["args_text"]), rawSnapshotString(payload["tool_input"]), segmentToolCall.ArgsText)
		segmentToolCall.Status = "running"

	case "tool.call.args.delta":
		assistant := ensureCodingSessionAssistantMessage(
			snapshot.LiveAssistantMessage,
			firstNonEmptySnapshotValue(
				trimmedSnapshotString(payload["parent_message_id"]),
				snapshotMessageID(snapshot.LiveAssistantMessage),
				"assistant:"+timestamp.UTC().Format(time.RFC3339Nano),
			),
			timestamp.UTC(),
		)
		toolCall := ensureCodingSessionToolCall(assistant, payload, timestamp.UTC())
		toolCall.ArgsText = snapshotMergeToolArgs(toolCall.ArgsText, payload)
		snapshot.LiveAssistantMessage = assistant
		segmentToolCall := ensureCodingSessionToolCallSegment(snapshot, payload, timestamp.UTC(), assistant.MessageID)
		segmentToolCall.ArgsText = snapshotMergeToolArgs(segmentToolCall.ArgsText, payload)

	case "tool.call.result":
		assistant := ensureCodingSessionAssistantMessage(
			snapshot.LiveAssistantMessage,
			firstNonEmptySnapshotValue(
				trimmedSnapshotString(payload["parent_message_id"]),
				snapshotMessageID(snapshot.LiveAssistantMessage),
				"assistant:"+timestamp.UTC().Format(time.RFC3339Nano),
			),
			timestamp.UTC(),
		)
		toolCall := ensureCodingSessionToolCall(assistant, payload, timestamp.UTC())
		result := &CodingSessionLiveToolResult{
			MessageID: trimmedSnapshotString(payload["result_message_id"]),
			Content: firstNonEmptySnapshotValue(
				rawSnapshotString(payload["content"]),
				rawSnapshotString(payload["output_summary"]),
			),
		}
		if summary := rawSnapshotString(payload["output_summary"]); summary != "" {
			result.OutputSummary = snapshotStringPtr(summary)
		}
		if errText := rawSnapshotString(payload["error"]); errText != "" {
			result.Error = snapshotStringPtr(errText)
		}
		toolCall.Result = result
		snapshot.LiveAssistantMessage = assistant
		segmentToolCall := ensureCodingSessionToolCallSegment(snapshot, payload, timestamp.UTC(), assistant.MessageID)
		segmentResult := &CodingSessionLiveToolResult{
			MessageID: trimmedSnapshotString(payload["result_message_id"]),
			Content: firstNonEmptySnapshotValue(
				rawSnapshotString(payload["content"]),
				rawSnapshotString(payload["output_summary"]),
			),
		}
		if summary := rawSnapshotString(payload["output_summary"]); summary != "" {
			segmentResult.OutputSummary = snapshotStringPtr(summary)
		}
		if errText := rawSnapshotString(payload["error"]); errText != "" {
			segmentResult.Error = snapshotStringPtr(errText)
		}
		segmentToolCall.Result = segmentResult

	case "tool.call.completed", "tool.call.failed":
		assistant := ensureCodingSessionAssistantMessage(
			snapshot.LiveAssistantMessage,
			firstNonEmptySnapshotValue(
				trimmedSnapshotString(payload["parent_message_id"]),
				snapshotMessageID(snapshot.LiveAssistantMessage),
				"assistant:"+timestamp.UTC().Format(time.RFC3339Nano),
			),
			timestamp.UTC(),
		)
		toolCall := ensureCodingSessionToolCall(assistant, payload, timestamp.UTC())
		if strings.TrimSpace(eventType) == "tool.call.failed" {
			toolCall.Status = "failed"
		} else {
			toolCall.Status = "completed"
		}
		if duration := snapshotInt64(payload["duration_ms"]); duration > 0 {
			toolCall.DurationMs = int64Ptr(duration)
		}
		toolCall.CompletedAt = timePtr(timestamp.UTC())

		result := toolCall.Result
		if result == nil {
			result = &CodingSessionLiveToolResult{}
		}
		if result.MessageID == "" {
			result.MessageID = trimmedSnapshotString(payload["result_message_id"])
		}
		if content := firstNonEmptySnapshotValue(rawSnapshotString(payload["content"]), rawSnapshotString(payload["output_summary"]), result.Content); content != "" {
			result.Content = content
		}
		if summary := firstNonEmptySnapshotValue(rawSnapshotString(payload["output_summary"]), derefString(result.OutputSummary)); summary != "" {
			result.OutputSummary = snapshotStringPtr(summary)
		}
		if errText := firstNonEmptySnapshotValue(rawSnapshotString(payload["error"]), derefString(result.Error)); errText != "" {
			result.Error = snapshotStringPtr(errText)
		}
		toolCall.Result = result
		snapshot.LiveAssistantMessage = assistant
		segmentToolCall := ensureCodingSessionToolCallSegment(snapshot, payload, timestamp.UTC(), assistant.MessageID)
		if strings.TrimSpace(eventType) == "tool.call.failed" {
			segmentToolCall.Status = "failed"
		} else {
			segmentToolCall.Status = "completed"
		}
		if duration := snapshotInt64(payload["duration_ms"]); duration > 0 {
			segmentToolCall.DurationMs = int64Ptr(duration)
		}
		segmentToolCall.CompletedAt = timePtr(timestamp.UTC())
		segmentResult := segmentToolCall.Result
		if segmentResult == nil {
			segmentResult = &CodingSessionLiveToolResult{}
		}
		if segmentResult.MessageID == "" {
			segmentResult.MessageID = trimmedSnapshotString(payload["result_message_id"])
		}
		if content := firstNonEmptySnapshotValue(rawSnapshotString(payload["content"]), rawSnapshotString(payload["output_summary"]), segmentResult.Content); content != "" {
			segmentResult.Content = content
		}
		if summary := firstNonEmptySnapshotValue(rawSnapshotString(payload["output_summary"]), derefString(segmentResult.OutputSummary)); summary != "" {
			segmentResult.OutputSummary = snapshotStringPtr(summary)
		}
		if errText := firstNonEmptySnapshotValue(rawSnapshotString(payload["error"]), derefString(segmentResult.Error)); errText != "" {
			segmentResult.Error = snapshotStringPtr(errText)
		}
		segmentToolCall.Result = segmentResult

	case "plan.updated":
		plan := parseCodingSessionRunPlan(payload["content"])
		if plan != nil {
			snapshot.CurrentPlan = plan
		}
	}

	return normalizeCodingSessionStreamSnapshot(snapshot)
}

func parseCodingSessionRunPlan(value any) *CodingSessionRunPlan {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil
		}
		var plan CodingSessionRunPlan
		if err := json.Unmarshal([]byte(trimmed), &plan); err != nil {
			return nil
		}
		if !isValidCodingSessionRunPlan(&plan) {
			return nil
		}
		return &plan
	case map[string]any:
		payload, err := json.Marshal(typed)
		if err != nil {
			return nil
		}
		var plan CodingSessionRunPlan
		if err := json.Unmarshal(payload, &plan); err != nil {
			return nil
		}
		if !isValidCodingSessionRunPlan(&plan) {
			return nil
		}
		return &plan
	default:
		return nil
	}
}

func isValidCodingSessionRunPlan(plan *CodingSessionRunPlan) bool {
	if plan == nil || len(plan.Plan) == 0 {
		return false
	}
	plan.Note = strings.TrimSpace(plan.Note)
	for i := range plan.Plan {
		plan.Plan[i].Step = strings.TrimSpace(plan.Plan[i].Step)
		plan.Plan[i].Status = strings.TrimSpace(plan.Plan[i].Status)
		if plan.Plan[i].Step == "" {
			return false
		}
		switch plan.Plan[i].Status {
		case "pending", "in_progress", "completed":
		default:
			return false
		}
	}
	return true
}

func ensureCodingSessionAssistantMessage(current *CodingSessionLiveAssistantMessage, messageID string, timestamp time.Time) *CodingSessionLiveAssistantMessage {
	if current != nil && current.MessageID == messageID {
		if current.StartedAt == nil {
			current.StartedAt = timePtr(timestamp)
		}
		return current
	}
	return &CodingSessionLiveAssistantMessage{
		MessageID: messageID,
		Content:   "",
		StartedAt: timePtr(timestamp),
		Status:    "streaming",
		ToolCalls: []CodingSessionLiveToolCall{},
	}
}

func ensureCodingSessionReasoningMessage(current *CodingSessionLiveReasoningMessage, messageID string, timestamp time.Time) *CodingSessionLiveReasoningMessage {
	if current != nil && current.MessageID == messageID {
		if current.StartedAt == nil {
			current.StartedAt = timePtr(timestamp)
		}
		return current
	}
	return &CodingSessionLiveReasoningMessage{
		MessageID: messageID,
		Content:   "",
		StartedAt: timePtr(timestamp),
		Status:    "streaming",
	}
}

func ensureCodingSessionToolCall(assistant *CodingSessionLiveAssistantMessage, payload map[string]any, timestamp time.Time) *CodingSessionLiveToolCall {
	if assistant == nil {
		return nil
	}

	toolCallID := firstNonEmptySnapshotValue(trimmedSnapshotString(payload["tool_call_id"]), assistant.MessageID+":tool:1")
	for index := range assistant.ToolCalls {
		if assistant.ToolCalls[index].ToolCallID != toolCallID {
			continue
		}
		if assistant.ToolCalls[index].StartedAt == nil {
			assistant.ToolCalls[index].StartedAt = timePtr(timestamp)
		}
		if toolName := trimmedSnapshotString(payload["tool_name"]); toolName != "" {
			assistant.ToolCalls[index].ToolName = toolName
		}
		if parentMessageID := trimmedSnapshotString(payload["parent_message_id"]); parentMessageID != "" {
			assistant.ToolCalls[index].ParentMessageID = parentMessageID
		}
		return &assistant.ToolCalls[index]
	}

	assistant.ToolCalls = append(assistant.ToolCalls, CodingSessionLiveToolCall{
		ToolCallID:      toolCallID,
		ParentMessageID: firstNonEmptySnapshotValue(trimmedSnapshotString(payload["parent_message_id"]), assistant.MessageID),
		ToolName:        firstNonEmptySnapshotValue(trimmedSnapshotString(payload["tool_name"]), "tool"),
		ArgsText:        firstNonEmptySnapshotValue(rawSnapshotString(payload["args_text"]), rawSnapshotString(payload["tool_input"])),
		Status:          "running",
		StartedAt:       timePtr(timestamp),
	})
	return &assistant.ToolCalls[len(assistant.ToolCalls)-1]
}

func appendCodingSessionAssistantSegment(snapshot *CodingSessionStreamSnapshot, messageID, content string, timestamp time.Time) *CodingSessionLiveAssistantMessage {
	if snapshot == nil || strings.TrimSpace(messageID) == "" || content == "" {
		return nil
	}
	for index := len(snapshot.LiveTurnSegments) - 1; index >= 0; index-- {
		segment := &snapshot.LiveTurnSegments[index]
		if segment.Kind != "assistant_message" || segment.AssistantMessage == nil {
			break
		}
		if segment.AssistantMessage.MessageID != messageID {
			break
		}
		if segment.AssistantMessage.StartedAt == nil {
			segment.AssistantMessage.StartedAt = timePtr(timestamp)
		}
		segment.AssistantMessage.Content += content
		segment.AssistantMessage.Status = "streaming"
		return segment.AssistantMessage
	}

	assistant := &CodingSessionLiveAssistantMessage{
		MessageID: messageID,
		Content:   content,
		StartedAt: timePtr(timestamp),
		Status:    "streaming",
		ToolCalls: []CodingSessionLiveToolCall{},
	}
	snapshot.LiveTurnSegments = append(snapshot.LiveTurnSegments, CodingSessionLiveTurnSegment{
		SegmentID:        nextCodingSessionAssistantSegmentID(snapshot.LiveTurnSegments, messageID),
		Kind:             "assistant_message",
		AssistantMessage: assistant,
	})
	return snapshot.LiveTurnSegments[len(snapshot.LiveTurnSegments)-1].AssistantMessage
}

func appendOrMarkCompletedCodingSessionAssistantSegment(snapshot *CodingSessionStreamSnapshot, messageID, previousContent, fullContent string, timestamp time.Time) {
	if snapshot == nil || strings.TrimSpace(messageID) == "" {
		return
	}
	if suffix, ok := deriveCodingSessionAssistantSegmentDelta(previousContent, fullContent); ok && suffix != "" {
		segment := appendCodingSessionAssistantSegment(snapshot, messageID, suffix, timestamp)
		if segment != nil {
			segment.Status = "completed"
			segment.CompletedAt = timePtr(timestamp)
		}
		return
	}
	if strings.TrimSpace(fullContent) != "" && fullContent != previousContent {
		if segment := firstCodingSessionAssistantSegment(snapshot.LiveTurnSegments, messageID); segment != nil {
			segment.Content = fullContent
			segment.Status = "completed"
			segment.CompletedAt = timePtr(timestamp)
			return
		}
	}
	if segment := latestCodingSessionAssistantSegment(snapshot.LiveTurnSegments, messageID); segment != nil {
		segment.Status = "completed"
		segment.CompletedAt = timePtr(timestamp)
		return
	}
	if strings.TrimSpace(fullContent) == "" {
		return
	}
	segment := appendCodingSessionAssistantSegment(snapshot, messageID, fullContent, timestamp)
	if segment != nil {
		segment.Status = "completed"
		segment.CompletedAt = timePtr(timestamp)
	}
}

func firstCodingSessionAssistantSegment(segments []CodingSessionLiveTurnSegment, messageID string) *CodingSessionLiveAssistantMessage {
	for index := range segments {
		segment := &segments[index]
		if segment.Kind != "assistant_message" || segment.AssistantMessage == nil {
			continue
		}
		if segment.AssistantMessage.MessageID == messageID {
			return segment.AssistantMessage
		}
	}
	return nil
}

func ensureCodingSessionToolCallSegment(snapshot *CodingSessionStreamSnapshot, payload map[string]any, timestamp time.Time, assistantMessageID string) *CodingSessionLiveToolCall {
	if snapshot == nil {
		return nil
	}

	toolCallID := firstNonEmptySnapshotValue(trimmedSnapshotString(payload["tool_call_id"]), assistantMessageID+":tool:1")
	for index := range snapshot.LiveTurnSegments {
		segment := &snapshot.LiveTurnSegments[index]
		if segment.Kind != "tool_call" || segment.ToolCall == nil || segment.ToolCall.ToolCallID != toolCallID {
			continue
		}
		if segment.ToolCall.StartedAt == nil {
			segment.ToolCall.StartedAt = timePtr(timestamp)
		}
		if toolName := trimmedSnapshotString(payload["tool_name"]); toolName != "" {
			segment.ToolCall.ToolName = toolName
		}
		if parentMessageID := firstNonEmptySnapshotValue(trimmedSnapshotString(payload["parent_message_id"]), assistantMessageID); parentMessageID != "" {
			segment.ToolCall.ParentMessageID = parentMessageID
		}
		return segment.ToolCall
	}

	toolCall := &CodingSessionLiveToolCall{
		ToolCallID:      toolCallID,
		ParentMessageID: firstNonEmptySnapshotValue(trimmedSnapshotString(payload["parent_message_id"]), assistantMessageID),
		ToolName:        firstNonEmptySnapshotValue(trimmedSnapshotString(payload["tool_name"]), "tool"),
		ArgsText:        firstNonEmptySnapshotValue(rawSnapshotString(payload["args_text"]), rawSnapshotString(payload["tool_input"])),
		Status:          "running",
		StartedAt:       timePtr(timestamp),
	}
	snapshot.LiveTurnSegments = append(snapshot.LiveTurnSegments, CodingSessionLiveTurnSegment{
		SegmentID: toolCallID,
		Kind:      "tool_call",
		ToolCall:  toolCall,
	})
	return snapshot.LiveTurnSegments[len(snapshot.LiveTurnSegments)-1].ToolCall
}

func normalizeCodingSessionStreamSnapshot(snapshot *CodingSessionStreamSnapshot) *CodingSessionStreamSnapshot {
	if snapshot == nil {
		return nil
	}
	if snapshot.LiveAssistantMessage != nil &&
		snapshot.LiveAssistantMessage.Status == "completed" &&
		strings.TrimSpace(snapshot.LiveAssistantMessage.Content) == "" &&
		len(snapshot.LiveAssistantMessage.ToolCalls) == 0 {
		snapshot.LiveAssistantMessage = nil
	}
	if snapshot.LiveReasoningMessage != nil &&
		snapshot.LiveReasoningMessage.Status == "completed" &&
		strings.TrimSpace(snapshot.LiveReasoningMessage.Content) == "" &&
		derefString(snapshot.LiveReasoningMessage.EncryptedValue) == "" {
		snapshot.LiveReasoningMessage = nil
	}
	if len(snapshot.LiveTurnSegments) > 0 {
		normalizedSegments := snapshot.LiveTurnSegments[:0]
		for _, segment := range snapshot.LiveTurnSegments {
			switch segment.Kind {
			case "assistant_message":
				if segment.AssistantMessage == nil {
					continue
				}
				if segment.AssistantMessage.Status == "completed" && strings.TrimSpace(segment.AssistantMessage.Content) == "" {
					continue
				}
			case "tool_call":
				if segment.ToolCall == nil {
					continue
				}
			default:
				continue
			}
			normalizedSegments = append(normalizedSegments, segment)
		}
		snapshot.LiveTurnSegments = normalizedSegments
	}
	if snapshot.IsEmpty() {
		return nil
	}
	return snapshot
}

func deriveCodingSessionAssistantSegmentDelta(previousContent, fullContent string) (string, bool) {
	if fullContent == "" {
		return "", false
	}
	if previousContent == "" {
		return fullContent, true
	}
	if strings.HasPrefix(fullContent, previousContent) {
		return fullContent[len(previousContent):], true
	}
	return "", false
}

func codingSessionStreamDelta(current, incoming string) string {
	if incoming == "" {
		return ""
	}
	if current != "" && strings.HasPrefix(incoming, current) {
		return incoming[len(current):]
	}
	return incoming
}

func latestCodingSessionAssistantSegment(segments []CodingSessionLiveTurnSegment, messageID string) *CodingSessionLiveAssistantMessage {
	for index := len(segments) - 1; index >= 0; index-- {
		segment := &segments[index]
		if segment.Kind != "assistant_message" || segment.AssistantMessage == nil {
			continue
		}
		if segment.AssistantMessage.MessageID == messageID {
			return segment.AssistantMessage
		}
	}
	return nil
}

func nextCodingSessionAssistantSegmentID(segments []CodingSessionLiveTurnSegment, messageID string) string {
	count := 0
	for _, segment := range segments {
		if segment.Kind == "assistant_message" && segment.AssistantMessage != nil && segment.AssistantMessage.MessageID == messageID {
			count++
		}
	}
	return firstNonEmptySnapshotValue(messageID, "assistant") + ":segment:" + strconv.Itoa(count+1)
}

func snapshotMergeToolArgs(current string, payload map[string]any) string {
	if argsText := rawSnapshotString(payload["args_text"]); argsText != "" {
		return argsText
	}
	if delta := rawSnapshotString(payload["args_delta"]); delta != "" {
		return current + delta
	}
	return current
}

func rawSnapshotString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case *string:
		return derefString(typed)
	default:
		return ""
	}
}

func trimmedSnapshotString(value any) string {
	return strings.TrimSpace(rawSnapshotString(value))
}

func snapshotInt64(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case *int:
		if typed != nil {
			return int64(*typed)
		}
	case *int64:
		if typed != nil {
			return *typed
		}
	}
	return 0
}

func firstNonEmptySnapshotValue(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptySnapshotRawString(values ...any) string {
	for _, value := range values {
		if raw := rawSnapshotString(value); raw != "" {
			return raw
		}
	}
	return ""
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func int64Ptr(value int64) *int64 {
	if value == 0 {
		return nil
	}
	return &value
}

func snapshotStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	copied := value
	return &copied
}

func snapshotMessageID(message any) string {
	switch typed := message.(type) {
	case *CodingSessionLiveAssistantMessage:
		if typed == nil {
			return ""
		}
		return strings.TrimSpace(typed.MessageID)
	case *CodingSessionLiveReasoningMessage:
		if typed == nil {
			return ""
		}
		return strings.TrimSpace(typed.MessageID)
	default:
		return ""
	}
}
