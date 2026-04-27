package temporalapp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

func (a *AgentRunActivities) createRunMessage(ctx context.Context, run *model.AgentRun, role, messageType, content string, blocks, turnSegments, toolInvocations, tokenUsage json.RawMessage) (*model.AgentRunMessage, error) {
	if a.runMessageRepo == nil {
		return nil, nil
	}
	sequenceNo, err := a.runMessageRepo.NextSequence(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return nil, err
	}
	message := &model.AgentRunMessage{
		WorkspaceID:     run.WorkspaceID,
		RunID:           run.ID,
		Role:            role,
		Content:         content,
		MessageType:     messageType,
		ContentBlocks:   blocks,
		TurnSegments:    turnSegments,
		ToolInvocations: toolInvocations,
		TokenUsage:      tokenUsage,
		SequenceNo:      sequenceNo,
	}
	if err := a.runMessageRepo.Create(ctx, message); err != nil {
		return nil, err
	}
	if shouldClearCodingSessionStreamSnapshot(role, messageType) {
		if err := a.clearCodingSessionStreamSnapshot(ctx, run); err != nil {
			slog.WarnContext(ctx, "clear coding session stream snapshot failed",
				"run_id", run.ID,
				"workspace_id", run.WorkspaceID,
				"error", err,
			)
		}
	}
	a.publishRunMessageEvent(run, message)
	return message, nil
}

func (a *AgentRunActivities) publishRunMessageEvent(run *model.AgentRun, message *model.AgentRunMessage) {
	if a.wsPublisher == nil || run == nil || message == nil {
		return
	}
	data, _ := json.Marshal(message)
	a.wsPublisher.Publish(websocket.Event{
		Action:      "created",
		Entity:      "agent_run_message",
		EntityID:    message.ID,
		WorkspaceID: run.WorkspaceID,
		ParentType:  "agent_run",
		ParentID:    run.ID,
		Data:        data,
	})
	eventType := "user.message.completed"
	switch strings.TrimSpace(message.Role) {
	case "assistant":
		eventType = "assistant.message.completed"
	case "tool":
		eventType = "tool.call.completed"
	}
	a.publishCodingSessionEvent(run, eventType, map[string]any{
		"message_id":       message.ID,
		"role":             message.Role,
		"message_type":     message.MessageType,
		"content":          message.Content,
		"sequence_no":      message.SequenceNo,
		"content_blocks":   json.RawMessage(message.ContentBlocks),
		"turn_segments":    json.RawMessage(message.TurnSegments),
		"tool_invocations": json.RawMessage(message.ToolInvocations),
	})
}

func (a *AgentRunActivities) publishRunStreamEvent(run *model.AgentRun, event workerpkg.ExecutionEvent) {
	if run == nil {
		return
	}

	sentAt := time.Now().UTC()
	codingEventType := codingSessionEventTypeFromExecutionEvent(event)
	codingPayload := map[string]any{
		"message_id":        strings.TrimSpace(event.MessageID),
		"parent_message_id": strings.TrimSpace(event.ParentMessageID),
		"result_message_id": strings.TrimSpace(event.ResultMessageID),
		"text":              event.Text,
		"content":           coalesceRaw(event.Content, event.Text),
		"tool_call_id":      event.ToolCallID,
		"tool_name":         event.ToolName,
		"tool_input":        event.ToolInput,
		"args_delta":        event.ArgsDelta,
		"args_text":         event.ArgsText,
		"activity_id":       event.ActivityID,
		"activity_type":     event.ActivityType,
		"encrypted_value":   event.EncryptedValue,
		"output_summary":    event.OutputSummary,
		"duration_ms":       event.DurationMs,
		"error":             event.Error,
	}
	if codingEventType != "" {
		a.persistCodingSessionStreamSnapshot(context.Background(), run, codingEventType, codingPayload, sentAt)
	}
	if a.wsPublisher == nil {
		return
	}

	payload, err := json.Marshal(model.AgentRunStreamEvent{
		SentAt:          sentAt,
		Type:            event.Type,
		RunID:           run.ID,
		MessageID:       event.MessageID,
		ParentMessageID: event.ParentMessageID,
		ResultMessageID: event.ResultMessageID,
		Text:            event.Text,
		Content:         event.Content,
		ToolCallID:      event.ToolCallID,
		ToolName:        event.ToolName,
		ToolInput:       event.ToolInput,
		ArgsDelta:       event.ArgsDelta,
		ArgsText:        event.ArgsText,
		ActivityID:      event.ActivityID,
		ActivityType:    event.ActivityType,
		EncryptedValue:  event.EncryptedValue,
		OutputSummary:   event.OutputSummary,
		DurationMs:      event.DurationMs,
		Error:           event.Error,
	})
	if err != nil {
		return
	}

	a.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "agent_run_stream",
		EntityID:    run.ID,
		WorkspaceID: run.WorkspaceID,
		ParentType:  "agent_run",
		ParentID:    run.ID,
		Data:        payload,
	})
	if codingEventType != "" {
		a.publishCodingSessionEvent(run, codingEventType, codingPayload)
	}
}

func shouldClearCodingSessionStreamSnapshot(role, messageType string) bool {
	return strings.TrimSpace(role) == "assistant" && strings.TrimSpace(messageType) == "assistant_turn"
}

func (a *AgentRunActivities) clearCodingSessionStreamSnapshot(ctx context.Context, run *model.AgentRun) error {
	if a == nil || a.sessionSnapshotRepo == nil || run == nil {
		return nil
	}
	return a.sessionSnapshotRepo.DeleteByRun(ctx, run.WorkspaceID, run.ID)
}

func (a *AgentRunActivities) loadCodingSessionStreamSnapshot(ctx context.Context, run *model.AgentRun) (*model.CodingSessionStreamSnapshot, error) {
	if a == nil || a.sessionSnapshotRepo == nil || run == nil {
		return nil, nil
	}
	record, err := a.sessionSnapshotRepo.GetByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil || record == nil {
		return nil, err
	}
	return model.DecodeCodingSessionStreamSnapshot(record.SnapshotPayload)
}

func (a *AgentRunActivities) persistCodingSessionStreamSnapshot(ctx context.Context, run *model.AgentRun, eventType string, payload map[string]any, timestamp time.Time) {
	if a == nil || a.sessionSnapshotRepo == nil || run == nil || strings.TrimSpace(eventType) == "" {
		return
	}

	record, err := a.sessionSnapshotRepo.GetByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		slog.WarnContext(ctx, "load coding session stream snapshot failed",
			"run_id", run.ID,
			"workspace_id", run.WorkspaceID,
			"error", err,
		)
		return
	}

	var snapshot *model.CodingSessionStreamSnapshot
	if record != nil {
		snapshot, err = model.DecodeCodingSessionStreamSnapshot(record.SnapshotPayload)
		if err != nil {
			slog.WarnContext(ctx, "decode coding session stream snapshot failed",
				"run_id", run.ID,
				"workspace_id", run.WorkspaceID,
				"error", err,
			)
			record = nil
		}
	}

	snapshot = model.ApplyCodingSessionStreamEvent(snapshot, eventType, payload, timestamp)
	if snapshot == nil || snapshot.IsEmpty() {
		if record != nil {
			if err := a.sessionSnapshotRepo.DeleteByRun(ctx, run.WorkspaceID, run.ID); err != nil {
				slog.WarnContext(ctx, "delete empty coding session stream snapshot failed",
					"run_id", run.ID,
					"workspace_id", run.WorkspaceID,
					"error", err,
				)
			}
		}
		return
	}

	encoded, err := model.EncodeCodingSessionStreamSnapshot(snapshot)
	if err != nil {
		slog.WarnContext(ctx, "encode coding session stream snapshot failed",
			"run_id", run.ID,
			"workspace_id", run.WorkspaceID,
			"error", err,
		)
		return
	}

	nextRecord := &model.CodingSessionStateSnapshot{
		WorkspaceID:     run.WorkspaceID,
		RunID:           run.ID,
		SchemaVersion:   model.CodingSessionStateSnapshotSchemaVersionV1,
		SnapshotPayload: encoded,
	}
	if record != nil {
		nextRecord.ID = record.ID
		nextRecord.CreatedAt = record.CreatedAt
	}
	if err := a.sessionSnapshotRepo.Upsert(ctx, nextRecord); err != nil {
		slog.WarnContext(ctx, "persist coding session stream snapshot failed",
			"run_id", run.ID,
			"workspace_id", run.WorkspaceID,
			"error", err,
		)
	}
}

func (a *AgentRunActivities) publishCodingSessionEvent(run *model.AgentRun, eventType string, payload map[string]any) {
	if a.wsPublisher == nil || run == nil || strings.TrimSpace(eventType) == "" {
		return
	}
	envelope, _ := json.Marshal(model.CodingSessionEvent{
		ID:          fmt.Sprintf("%s:%d", run.ID, time.Now().UTC().UnixNano()),
		SessionID:   run.ID,
		RunID:       run.ID,
		SequenceNo:  int(time.Now().UTC().UnixMilli()),
		Timestamp:   time.Now().UTC(),
		Type:        eventType,
		RuntimeKind: run.RuntimeKind,
		Payload:     payload,
	})
	a.wsPublisher.Publish(websocket.Event{
		Action:      "created",
		Entity:      "coding_session_event",
		EntityID:    fmt.Sprintf("%s:%d", run.ID, time.Now().UTC().UnixNano()),
		WorkspaceID: run.WorkspaceID,
		ParentType:  "coding_session",
		ParentID:    run.ID,
		Data:        envelope,
	})
}

func (a *AgentRunActivities) publishCodingSessionInteractionEvent(run *model.AgentRun, interaction *model.AgentRunInteraction) {
	if interaction == nil {
		return
	}
	eventType, payload := codingSessionInteractionEventPayload(interaction)
	a.publishCodingSessionEvent(run, eventType, payload)
}

func codingSessionInteractionEventPayload(interaction *model.AgentRunInteraction) (string, map[string]any) {
	if interaction == nil {
		return "", nil
	}

	eventType := "interaction.updated"
	switch strings.TrimSpace(interaction.Status) {
	case model.AgentRunInteractionStatusPending:
		eventType = "interaction.requested"
	case model.AgentRunInteractionStatusResolved:
		eventType = "interaction.resolved"
	case model.AgentRunInteractionStatusCancelled:
		eventType = "interaction.cancelled"
	}

	payload := map[string]any{
		"interaction_id":         interaction.ID,
		"interaction_kind":       interaction.InteractionKind,
		"status":                 interaction.Status,
		"request_schema_version": interaction.RequestSchemaVersion,
		"request_payload":        json.RawMessage(interaction.RequestPayload),
		"title":                  derefString(interaction.Title),
		"summary":                derefString(interaction.Summary),
		"request_id":             derefString(interaction.RequestID),
		"thread_id":              derefString(interaction.ThreadID),
		"turn_id":                derefString(interaction.TurnID),
		"item_id":                derefString(interaction.ItemID),
		"approval_id":            derefString(interaction.ApprovalID),
	}
	if interaction.AssistantMessageSequenceNo != nil {
		payload["assistant_message_sequence_no"] = *interaction.AssistantMessageSequenceNo
	}
	if interaction.ResponseSchemaVersion != nil && strings.TrimSpace(*interaction.ResponseSchemaVersion) != "" {
		payload["response_schema_version"] = strings.TrimSpace(*interaction.ResponseSchemaVersion)
	}
	if len(interaction.ResponsePayload) > 0 && string(interaction.ResponsePayload) != "null" {
		payload["response_payload"] = json.RawMessage(interaction.ResponsePayload)
	}
	if interaction.ResolvedAt != nil {
		payload["resolved_at"] = interaction.ResolvedAt.UTC()
	}
	if interaction.ResolvedBy != nil && strings.TrimSpace(*interaction.ResolvedBy) != "" {
		payload["resolved_by"] = strings.TrimSpace(*interaction.ResolvedBy)
	}

	return eventType, payload
}

func codingSessionEventTypeFromExecutionEvent(event workerpkg.ExecutionEvent) string {
	switch strings.TrimSpace(event.Type) {
	case "assistant_message_started":
		return "assistant.message.started"
	case "assistant_message_delta":
		return "assistant.message.delta"
	case "assistant_message_completed":
		return "assistant.message.completed"
	case "reasoning_message_started":
		return "reasoning.message.started"
	case "reasoning_message_delta":
		return "reasoning.message.delta"
	case "reasoning_message_completed":
		return "reasoning.message.completed"
	case "tool_call_started":
		return "tool.call.started"
	case "tool_call_args_delta":
		return "tool.call.args.delta"
	case "tool_call_result":
		return "tool.call.result"
	case "tool_call_finished":
		if strings.TrimSpace(event.Error) != "" {
			return "tool.call.failed"
		}
		return "tool.call.completed"
	case "activity_snapshot":
		return "activity.snapshot"
	case "activity_delta":
		return "activity.delta"
	case "plan_updated":
		return "plan.updated"
	default:
		return ""
	}
}

func latestExecutionApprovalRequest(execCtx *workerpkg.ExecutionContext) *model.ApprovalRequest {
	if execCtx == nil || execCtx.LastExecutionResult == nil {
		return nil
	}
	return workerpkg.ExtractLatestApprovalRequest(execCtx.LastExecutionResult.ToolInvocations)
}

func latestExecutionReviewCheckpointRequest(execCtx *workerpkg.ExecutionContext) *model.ReviewCheckpointRequest {
	if execCtx == nil || execCtx.LastExecutionResult == nil {
		return nil
	}
	return workerpkg.ExtractLatestReviewCheckpointRequest(execCtx.LastExecutionResult.ToolInvocations)
}

func latestExecutionHumanInputRequest(execCtx *workerpkg.ExecutionContext) *workerpkg.UserInputRequest {
	if execCtx == nil || execCtx.LastExecutionResult == nil {
		return nil
	}
	return workerpkg.ExtractLatestHumanInputRequest(execCtx.LastExecutionResult.ToolInvocations)
}

func latestExecutionCodexAuthState(execCtx *workerpkg.ExecutionContext) *model.CodexAuthState {
	if execCtx == nil || execCtx.LastExecutionResult == nil {
		return nil
	}
	return execCtx.LastExecutionResult.CodexAuthState
}
