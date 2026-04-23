package temporalapp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

func (a *AgentRunActivities) ensureRunBootstrapStatusMessage(ctx context.Context, run *model.AgentRun, content string) error {
	if a.runMessageRepo == nil || run == nil || strings.TrimSpace(content) == "" {
		return nil
	}
	messages, err := a.runMessageRepo.ListByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return err
	}
	if len(messages) > 0 {
		return nil
	}
	_, err = a.createRunMessage(ctx, run, "assistant", "status", strings.TrimSpace(content), nil, nil, nil, nil)
	return err
}

func (a *AgentRunActivities) persistAssistantRunMessage(ctx context.Context, state *resolvedRunState, execCtx *workerpkg.ExecutionContext) (*model.AgentRunMessage, error) {
	if a.runMessageRepo == nil || execCtx == nil || execCtx.LastExecutionResult == nil {
		return nil, nil
	}
	result := execCtx.LastExecutionResult
	snapshot, err := a.loadCodingSessionStreamSnapshot(ctx, state.run)
	if err != nil {
		return nil, err
	}
	assistantMessageInput, err := buildPersistedAssistantRunMessage(result, snapshot)
	if err != nil {
		return nil, err
	}
	if assistantMessageInput == nil {
		return nil, nil
	}

	assistantMessage, err := a.createRunMessage(
		ctx,
		state.run,
		assistantMessageInput.Role,
		assistantMessageInput.MessageType,
		assistantMessageInput.Content,
		assistantMessageInput.ContentBlocks,
		assistantMessageInput.TurnSegments,
		assistantMessageInput.ToolInvocations,
		assistantMessageInput.TokenUsage,
	)
	if err != nil {
		return nil, err
	}
	if err := a.persistProviderResponseCheckpoint(ctx, state, result, assistantMessage); err != nil {
		return nil, err
	}
	if err := a.persistHumanInteractionArtifacts(ctx, state, result, assistantMessage); err != nil {
		return nil, err
	}
	if err := a.persistNativeTurnDebugArtifact(ctx, state, execCtx, assistantMessage); err != nil {
		return nil, err
	}
	if err := a.persistNativeRepairStateArtifact(ctx, state, execCtx, assistantMessage); err != nil {
		return nil, err
	}

	for _, toolMessage := range buildPersistedToolResultMessages(result.Messages) {
		if _, err := a.createRunMessage(ctx, state.run, toolMessage.Role, toolMessage.MessageType, toolMessage.Content, toolMessage.ContentBlocks, nil, nil, nil); err != nil {
			return nil, err
		}
	}

	return assistantMessage, nil
}

type persistedRunMessageInput struct {
	Role            string
	MessageType     string
	Content         string
	ContentBlocks   json.RawMessage
	TurnSegments    json.RawMessage
	ToolInvocations json.RawMessage
	TokenUsage      json.RawMessage
}

func buildPersistedAssistantRunMessage(result *workerpkg.ExecutionResult, snapshot *model.CodingSessionStreamSnapshot) (*persistedRunMessageInput, error) {
	if result == nil {
		return nil, nil
	}
	content := persistedMessageContent(strings.TrimSpace(result.AssistantText), result.AssistantBlocks)
	if content == "" && len(result.AssistantBlocks) == 0 {
		return nil, nil
	}

	blocks, err := marshalExecutionBlocks(result.AssistantBlocks)
	if err != nil {
		return nil, fmt.Errorf("marshal assistant blocks: %w", err)
	}
	turnSegments, err := marshalCodingSessionTurnSegments(snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal turn segments: %w", err)
	}
	invocations, err := marshalToolInvocations(result.ToolInvocations)
	if err != nil {
		return nil, fmt.Errorf("marshal tool invocations: %w", err)
	}
	usagePayload, err := marshalTokenUsage(result.Usage)
	if err != nil {
		return nil, fmt.Errorf("marshal usage: %w", err)
	}

	return &persistedRunMessageInput{
		Role:            "assistant",
		MessageType:     "assistant_turn",
		Content:         content,
		ContentBlocks:   blocks,
		TurnSegments:    turnSegments,
		ToolInvocations: invocations,
		TokenUsage:      usagePayload,
	}, nil
}

func buildPersistedToolResultMessages(messages []workerpkg.ExecutionMessage) []persistedRunMessageInput {
	toolMessages := finalRoundToolMessages(messages)
	results := make([]persistedRunMessageInput, 0, len(toolMessages))
	for _, toolMessage := range toolMessages {
		results = append(results, persistedRunMessageInput{
			Role:          "tool",
			MessageType:   "tool_result",
			Content:       persistedMessageContent(strings.TrimSpace(toolMessage.Content), toolMessage.Blocks),
			ContentBlocks: mustMarshalExecutionBlocks(toolMessage.Blocks),
		})
	}
	return results
}

func buildAssistantSequenceArtifactMetadata(sequenceNo int) json.RawMessage {
	if sequenceNo <= 0 {
		return json.RawMessage(`{}`)
	}
	payload, err := json.Marshal(map[string]any{
		"assistant_message_sequence_no": sequenceNo,
	})
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return payload
}

func artifactAssistantMessageSequenceNo(artifact model.AgentRunArtifact) int {
	if len(artifact.Metadata) == 0 || string(artifact.Metadata) == "null" {
		return 0
	}
	var metadata struct {
		AssistantMessageSequenceNo int `json:"assistant_message_sequence_no"`
	}
	if err := json.Unmarshal(artifact.Metadata, &metadata); err != nil {
		return 0
	}
	return metadata.AssistantMessageSequenceNo
}

func mergeArtifactMetadata(parts ...json.RawMessage) json.RawMessage {
	merged := map[string]any{}
	for _, part := range parts {
		if len(part) == 0 || string(part) == "null" {
			continue
		}
		var decoded map[string]any
		if err := json.Unmarshal(part, &decoded); err != nil {
			continue
		}
		for key, value := range decoded {
			merged[key] = value
		}
	}
	if len(merged) == 0 {
		return json.RawMessage(`{}`)
	}
	payload, err := json.Marshal(merged)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return payload
}

func marshalExecutionBlocks(blocks []workerpkg.ExecutionBlock) (json.RawMessage, error) {
	if len(blocks) == 0 {
		return nil, nil
	}
	payload, err := json.Marshal(workerpkg.NormalizeExecutionBlocks(blocks))
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func marshalCodingSessionTurnSegments(snapshot *model.CodingSessionStreamSnapshot) (json.RawMessage, error) {
	if snapshot == nil || len(snapshot.LiveTurnSegments) == 0 {
		return nil, nil
	}
	payload, err := json.Marshal(snapshot.LiveTurnSegments)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func mustMarshalExecutionBlocks(blocks []workerpkg.ExecutionBlock) json.RawMessage {
	payload, err := marshalExecutionBlocks(blocks)
	if err != nil {
		return nil
	}
	return payload
}

func marshalToolInvocations(invocations []model.ToolInvocation) (json.RawMessage, error) {
	if len(invocations) == 0 {
		return nil, nil
	}
	payload, err := json.Marshal(invocations)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func marshalTokenUsage(usage workerpkg.ExecutionUsage) (json.RawMessage, error) {
	return json.Marshal(map[string]int{
		"cached_input_tokens": usage.CachedInputTokens,
		"input_tokens":        usage.InputTokens,
		"output_tokens":       usage.OutputTokens,
	})
}

func persistedMessageContent(fallback string, blocks []workerpkg.ExecutionBlock) string {
	if strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback)
	}
	text := strings.TrimSpace(workerpkg.ExtractPersistedContentFromExecutionBlocks(blocks))
	if text != "" {
		return text
	}
	return ""
}

func finalRoundToolMessages(messages []workerpkg.ExecutionMessage) []workerpkg.ExecutionMessage {
	lastAssistantIndex := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" {
			lastAssistantIndex = i
			break
		}
	}
	if lastAssistantIndex == -1 || lastAssistantIndex >= len(messages)-1 {
		return nil
	}

	results := make([]workerpkg.ExecutionMessage, 0, len(messages)-lastAssistantIndex-1)
	for _, message := range messages[lastAssistantIndex+1:] {
		if message.Role != "tool" {
			continue
		}
		results = append(results, message)
	}
	return results
}

func (a *AgentRunActivities) persistProviderResponseCheckpoint(ctx context.Context, state *resolvedRunState, result *workerpkg.ExecutionResult, assistantMessage *model.AgentRunMessage) error {
	if a.artifactRepo == nil || state == nil || state.run == nil || state.agent == nil || result == nil || assistantMessage == nil || result.ProviderContinuation == nil {
		return nil
	}
	if strings.TrimSpace(result.ProviderContinuation.ResponseID) == "" {
		return nil
	}

	provider := strings.TrimSpace(derefString(state.agent.Provider))
	if provider == "" {
		provider = strings.TrimSpace(result.ProviderContinuation.Provider)
	}

	_, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeProviderResponseCheckpoint, "json", model.ProviderResponseCheckpoint{
		Provider:              provider,
		ResponseID:            strings.TrimSpace(result.ProviderContinuation.ResponseID),
		PreviousResponseID:    strings.TrimSpace(result.ProviderContinuation.PreviousResponseID),
		AssistantMessageSeqNo: assistantMessage.SequenceNo,
		RecordedAt:            time.Now().UTC(),
	}, buildAssistantSequenceArtifactMetadata(assistantMessage.SequenceNo))
	if err == nil {
		slog.InfoContext(ctx, "saved provider continuation checkpoint",
			"workspace_id", state.run.WorkspaceID,
			"run_id", state.run.ID,
			"provider", provider,
			"response_id", strings.TrimSpace(result.ProviderContinuation.ResponseID),
			"assistant_sequence_no", assistantMessage.SequenceNo,
		)
	}
	return err
}
