package temporalapp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

func (a *AgentRunActivities) persistHumanInteractionArtifacts(ctx context.Context, state *resolvedRunState, result *workerpkg.ExecutionResult, assistantMessage *model.AgentRunMessage) error {
	if (a.artifactRepo == nil && a.interactionRepo == nil) || state == nil || state.run == nil || result == nil || assistantMessage == nil {
		return nil
	}

	metadata := buildAssistantSequenceArtifactMetadata(assistantMessage.SequenceNo)

	if inputRequest := latestHumanInputRequestFromResult(result); inputRequest != nil {
		inputMetadata := mergeArtifactMetadata(metadata, result.HumanInputMetadata)
		artifactPayload := humanInputArtifactFromWorker(inputRequest)
		if a.artifactRepo != nil {
			if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeHumanInputRequest, "json", artifactPayload, inputMetadata); err != nil {
				return err
			}
		}
		interaction, err := a.persistHumanInputInteraction(ctx, state, inputRequest, inputMetadata, assistantMessage.SequenceNo)
		if err != nil {
			return err
		}
		if interaction != nil {
			a.maybeNotifyAgentAttentionRequired(ctx, state, interaction)
			a.publishCodingSessionInteractionEvent(state.run, interaction)
		} else {
			a.publishCodingSessionEvent(state.run, "input.requested", map[string]any{
				"content": artifactPayload,
			})
		}
	}

	if reviewRequest := latestHumanReviewCheckpointRequestFromResult(result); reviewRequest != nil {
		approvalMetadata := mergeArtifactMetadata(metadata, result.HumanApprovalMetadata)
		if a.artifactRepo != nil {
			if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeHumanApprovalRequest, "json", reviewRequest, approvalMetadata); err != nil {
				return err
			}
			if reviewFindings := reviewFindingsArtifactFromReviewCheckpointRequest(reviewRequest, assistantMessage.SequenceNo); reviewFindings != nil {
				if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeReviewFindings, "json", reviewFindings, approvalMetadata); err != nil {
					return err
				}
			}
		}
		interaction, err := a.persistHumanApprovalInteraction(ctx, state, model.AgentRunInteractionKindReviewCheckpoint, reviewRequest.Title, reviewRequest.Summary, reviewRequest, approvalMetadata, assistantMessage.SequenceNo)
		if err != nil {
			return err
		}
		if interaction != nil {
			a.maybeNotifyAgentAttentionRequired(ctx, state, interaction)
			a.publishCodingSessionInteractionEvent(state.run, interaction)
		} else {
			a.publishCodingSessionEvent(state.run, "approval.requested", map[string]any{
				"content": reviewRequest,
			})
		}
	}

	if approvalRequest := latestHumanApprovalRequestFromResult(result); approvalRequest != nil {
		approvalMetadata := mergeArtifactMetadata(metadata, result.HumanApprovalMetadata)
		if a.artifactRepo != nil {
			if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeHumanApprovalRequest, "json", approvalRequest, approvalMetadata); err != nil {
				return err
			}
		}
		interaction, err := a.persistHumanApprovalInteraction(ctx, state, model.AgentRunInteractionKindApprovalRequest, approvalRequest.Title, approvalRequest.Summary, approvalRequest, approvalMetadata, assistantMessage.SequenceNo)
		if err != nil {
			return err
		}
		if interaction != nil {
			a.maybeNotifyAgentAttentionRequired(ctx, state, interaction)
			a.publishCodingSessionInteractionEvent(state.run, interaction)
		} else {
			a.publishCodingSessionEvent(state.run, "approval.requested", map[string]any{
				"content": approvalRequest,
			})
		}
	}

	if authState := latestCodexAuthStateFromResult(result); authState != nil {
		authMetadata := mergeArtifactMetadata(metadata, result.CodexAuthMetadata)
		if a.artifactRepo != nil {
			if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeCodexAuthState, "json", authState, authMetadata); err != nil {
				return err
			}
		}
		a.publishCodingSessionEvent(state.run, "auth.updated", map[string]any{
			"content": authState,
		})
	}

	if runPlan := latestRunPlanFromResult(result); runPlan != nil {
		runPlanMetadata := mergeArtifactMetadata(metadata, result.RunPlanMetadata)
		if a.artifactRepo != nil {
			if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeRunPlan, "json", runPlan, runPlanMetadata); err != nil {
				return err
			}
		}
		a.publishCodingSessionEvent(state.run, "activity.updated", map[string]any{
			"content": runPlan,
		})
	}

	return nil
}

func (a *AgentRunActivities) maybeNotifyAgentAttentionRequired(ctx context.Context, state *resolvedRunState, interaction *model.AgentRunInteraction) {
	if a == nil || a.notificationEmitter == nil || state == nil || state.run == nil || interaction == nil {
		return
	}
	if strings.TrimSpace(interaction.Status) != model.AgentRunInteractionStatusPending {
		return
	}
	if strings.TrimSpace(state.run.TargetType) != "task" || strings.TrimSpace(state.run.TargetID) == "" {
		return
	}
	task, recipients, err := a.loadTaskAgentAttentionNotificationTarget(ctx, state.run.TargetID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to resolve task attention notification target",
			"error", err,
			"workspace_id", state.run.WorkspaceID,
			"run_id", state.run.ID,
			"task_id", state.run.TargetID,
		)
		return
	}
	if task == nil || len(recipients) == 0 {
		return
	}

	attentionType := "input"
	switch strings.TrimSpace(interaction.InteractionKind) {
	case model.AgentRunInteractionKindCommandExecutionApproval,
		model.AgentRunInteractionKindFileChangeApproval,
		model.AgentRunInteractionKindPermissionsApproval,
		model.AgentRunInteractionKindReviewCheckpoint:
		attentionType = "approval"
	}

	title := fmt.Sprintf("%s is waiting for %s on %s", a.agentAttentionAgentName(state), attentionType, strings.TrimSpace(task.Name))
	body := strings.TrimSpace(derefString(interaction.Summary))
	if body == "" {
		body = fmt.Sprintf("Open the task run to respond to the pending %s request.", attentionType)
	}

	teamID := ""
	if task.TeamID != nil {
		teamID = strings.TrimSpace(*task.TeamID)
	}

	if err := a.notificationEmitter.Emit(ctx, model.NotificationEventInput{
		WorkspaceID: state.run.WorkspaceID,
		EventType:   "task.agent_attention_required",
		EntityType:  "agent_run",
		EntityID:    state.run.ID,
		Title:       title,
		Body:        body,
		Category:    model.NotifCategoryAgentAttention,
		Priority:    "high",
		TeamID:      teamID,
		Metadata: model.JSONB{
			"run_id":           state.run.ID,
			"task_id":          task.ID,
			"target_type":      state.run.TargetType,
			"target_id":        state.run.TargetID,
			"interaction_id":   interaction.ID,
			"interaction_kind": interaction.InteractionKind,
			"pause_reason":     agentAttentionPauseReason(interaction),
			"agent_id":         strings.TrimSpace(state.run.AgentID),
			"agent_name":       a.agentAttentionAgentName(state),
		},
		EntitySnapshot: model.JSONB{
			"title":      task.Name,
			"identifier": taskAttentionDisplayIdentifier(task),
			"type":       "task",
		},
		ParentEntitySnapshot: model.JSONB{
			"type":       "task",
			"id":         task.ID,
			"title":      task.Name,
			"identifier": taskAttentionDisplayIdentifier(task),
		},
		ExplicitRecipients: recipients,
		SkipFollowers:      true,
		SkipEmailDelivery:  true,
	}); err != nil {
		slog.ErrorContext(ctx, "failed to emit task agent attention notification",
			"error", err,
			"workspace_id", state.run.WorkspaceID,
			"run_id", state.run.ID,
			"task_id", task.ID,
			"recipient_count", len(recipients),
		)
	}
}

func (a *AgentRunActivities) loadTaskAgentAttentionNotificationTarget(ctx context.Context, taskID string) (*model.PMTask, []string, error) {
	if a == nil || a.taskRepo == nil {
		return nil, nil, nil
	}
	task, err := a.taskRepo.GetRawByID(ctx, taskID)
	if err != nil || task == nil {
		return task, nil, err
	}

	if ownerID := strings.TrimSpace(derefString(task.OwnerID)); ownerID != "" {
		return task, []string{ownerID}, nil
	}

	ownerUserIDs, err := a.taskRepo.ListOwnerUserIDs(ctx, task.ID)
	if err != nil {
		return nil, nil, err
	}
	if len(ownerUserIDs) > 0 {
		return task, ownerUserIDs, nil
	}

	teamID := strings.TrimSpace(derefString(task.TeamID))
	if teamID == "" || a.workspaceRepo == nil {
		return task, nil, nil
	}

	teamUserIDs, err := a.workspaceRepo.ListActiveTeamUserIDs(ctx, task.WorkspaceID, teamID)
	if err != nil {
		return nil, nil, err
	}
	return task, teamUserIDs, nil
}

func (a *AgentRunActivities) agentAttentionAgentName(state *resolvedRunState) string {
	if state != nil && state.agent != nil {
		if name := strings.TrimSpace(state.agent.Name); name != "" {
			return name
		}
	}
	return "Agent"
}

func agentAttentionPauseReason(interaction *model.AgentRunInteraction) string {
	if interaction == nil {
		return model.AgentRunPauseReasonHumanInput
	}
	switch strings.TrimSpace(interaction.InteractionKind) {
	case model.AgentRunInteractionKindCommandExecutionApproval,
		model.AgentRunInteractionKindFileChangeApproval,
		model.AgentRunInteractionKindPermissionsApproval,
		model.AgentRunInteractionKindReviewCheckpoint:
		return model.AgentRunPauseReasonHumanApproval
	default:
		return model.AgentRunPauseReasonHumanInput
	}
}

func taskAttentionDisplayIdentifier(task *model.PMTask) string {
	if task == nil || task.DisplayID <= 0 {
		return ""
	}
	return "#" + strconv.Itoa(task.DisplayID)
}

type interactionRuntimeMetadata struct {
	AssistantMessageSequenceNo int             `json:"assistant_message_sequence_no,omitempty"`
	RuntimeKind                string          `json:"runtime_kind,omitempty"`
	CodexRequestKind           string          `json:"codex_request_kind,omitempty"`
	CodexRequestID             string          `json:"codex_request_id,omitempty"`
	CodexThreadID              string          `json:"codex_thread_id,omitempty"`
	CodexTurnID                string          `json:"codex_turn_id,omitempty"`
	CodexItemID                string          `json:"codex_item_id,omitempty"`
	CodexApprovalID            *string         `json:"codex_approval_id,omitempty"`
	CodexRequestPayload        json.RawMessage `json:"codex_request_payload,omitempty"`
}

func (a *AgentRunActivities) persistHumanInputInteraction(ctx context.Context, state *resolvedRunState, inputRequest *workerpkg.UserInputRequest, metadata json.RawMessage, assistantSequenceNo int) (*model.AgentRunInteraction, error) {
	if a.interactionRepo == nil || state == nil || state.run == nil || inputRequest == nil {
		return nil, nil
	}

	runtimeMetadata := decodeInteractionRuntimeMetadata(metadata)
	interaction := &model.AgentRunInteraction{
		WorkspaceID:                state.run.WorkspaceID,
		RunID:                      state.run.ID,
		RuntimeKind:                firstNonEmptyString(strings.TrimSpace(runtimeMetadata.RuntimeKind), executionRuntimeKind(state), strings.TrimSpace(state.run.RuntimeKind)),
		InteractionKind:            model.AgentRunInteractionKindRequestUserInput,
		Status:                     model.AgentRunInteractionStatusPending,
		RequestSchemaVersion:       model.AgentRunInteractionSchemaVersionCodexV2,
		RequestPayload:             mustMarshalJSON(inputRequest),
		RuntimeMetadata:            defaultInteractionRuntimeMetadata(metadata),
		AssistantMessageSequenceNo: intPtrIfPositive(firstPositiveInt(runtimeMetadata.AssistantMessageSequenceNo, assistantSequenceNo)),
		Title:                      strPtr("User input required"),
	}

	if interaction.RuntimeKind == "codex" && len(runtimeMetadata.CodexRequestPayload) > 0 {
		interaction.RequestSchemaVersion = model.AgentRunInteractionSchemaVersionCodexV2
		interaction.RequestPayload = copyRawJSON(runtimeMetadata.CodexRequestPayload)
		interaction.RequestID = strPtrIfNotEmpty(runtimeMetadata.CodexRequestID)
		interaction.ThreadID = strPtrIfNotEmpty(runtimeMetadata.CodexThreadID)
		interaction.TurnID = strPtrIfNotEmpty(runtimeMetadata.CodexTurnID)
		interaction.ItemID = strPtrIfNotEmpty(runtimeMetadata.CodexItemID)
	}

	interaction.Summary = strPtrIfNotEmpty(workerpkg.UserInputSummary(inputRequest))

	if err := a.appendRunInteraction(ctx, interaction); err != nil {
		return nil, err
	}
	return interaction, nil
}

func (a *AgentRunActivities) persistHumanApprovalInteraction(ctx context.Context, state *resolvedRunState, interactionKind, title, summary string, requestPayload any, metadata json.RawMessage, assistantSequenceNo int) (*model.AgentRunInteraction, error) {
	if a.interactionRepo == nil || state == nil || state.run == nil || requestPayload == nil {
		return nil, nil
	}

	runtimeMetadata := decodeInteractionRuntimeMetadata(metadata)
	interaction := &model.AgentRunInteraction{
		WorkspaceID:                state.run.WorkspaceID,
		RunID:                      state.run.ID,
		RuntimeKind:                firstNonEmptyString(strings.TrimSpace(runtimeMetadata.RuntimeKind), executionRuntimeKind(state), strings.TrimSpace(state.run.RuntimeKind)),
		InteractionKind:            strings.TrimSpace(interactionKind),
		Status:                     model.AgentRunInteractionStatusPending,
		RequestSchemaVersion:       model.AgentRunInteractionSchemaVersionHelpinV1,
		RequestPayload:             mustMarshalJSON(requestPayload),
		RuntimeMetadata:            defaultInteractionRuntimeMetadata(metadata),
		AssistantMessageSequenceNo: intPtrIfPositive(firstPositiveInt(runtimeMetadata.AssistantMessageSequenceNo, assistantSequenceNo)),
		Title:                      strPtrIfNotEmpty(strings.TrimSpace(title)),
		Summary:                    strPtrIfNotEmpty(strings.TrimSpace(summary)),
	}

	if interaction.RuntimeKind == "codex" && len(runtimeMetadata.CodexRequestPayload) > 0 {
		interaction.RequestSchemaVersion = model.AgentRunInteractionSchemaVersionCodexV2
		interaction.RequestPayload = copyRawJSON(runtimeMetadata.CodexRequestPayload)
		interaction.RequestID = strPtrIfNotEmpty(runtimeMetadata.CodexRequestID)
		interaction.ThreadID = strPtrIfNotEmpty(runtimeMetadata.CodexThreadID)
		interaction.TurnID = strPtrIfNotEmpty(runtimeMetadata.CodexTurnID)
		interaction.ItemID = strPtrIfNotEmpty(runtimeMetadata.CodexItemID)
		interaction.ApprovalID = runtimeMetadata.CodexApprovalID
		switch strings.TrimSpace(runtimeMetadata.CodexRequestKind) {
		case "command_execution":
			interaction.InteractionKind = model.AgentRunInteractionKindCommandExecutionApproval
		case "file_change":
			interaction.InteractionKind = model.AgentRunInteractionKindFileChangeApproval
		case "permissions":
			interaction.InteractionKind = model.AgentRunInteractionKindPermissionsApproval
		}
	}

	if err := a.appendRunInteraction(ctx, interaction); err != nil {
		return nil, err
	}
	return interaction, nil
}

func (a *AgentRunActivities) appendRunInteraction(ctx context.Context, interaction *model.AgentRunInteraction) error {
	if a.interactionRepo == nil || interaction == nil {
		return nil
	}
	if len(interaction.RequestPayload) == 0 {
		interaction.RequestPayload = json.RawMessage(`{}`)
	}
	if len(interaction.RuntimeMetadata) == 0 {
		interaction.RuntimeMetadata = json.RawMessage(`{}`)
	}
	if strings.TrimSpace(interaction.Status) == "" {
		interaction.Status = model.AgentRunInteractionStatusPending
	}
	if strings.TrimSpace(interaction.RequestSchemaVersion) == "" {
		interaction.RequestSchemaVersion = model.AgentRunInteractionSchemaVersionHelpinV1
	}
	return a.interactionRepo.Create(ctx, interaction)
}

func decodeInteractionRuntimeMetadata(raw json.RawMessage) interactionRuntimeMetadata {
	var metadata interactionRuntimeMetadata
	if len(raw) == 0 || string(raw) == "null" {
		return metadata
	}
	_ = json.Unmarshal(raw, &metadata)
	return metadata
}

func defaultInteractionRuntimeMetadata(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage(`{}`)
	}
	return copyRawJSON(raw)
}

func mustMarshalJSON(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return payload
}

func copyRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func intPtrIfPositive(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func strPtrIfNotEmpty(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func latestHumanApprovalRequestFromResult(result *workerpkg.ExecutionResult) *model.ApprovalRequest {
	if result == nil {
		return nil
	}
	return workerpkg.ExtractLatestApprovalRequest(result.ToolInvocations)
}

func latestHumanReviewCheckpointRequestFromResult(result *workerpkg.ExecutionResult) *model.ReviewCheckpointRequest {
	if result == nil {
		return nil
	}
	return workerpkg.ExtractLatestReviewCheckpointRequest(result.ToolInvocations)
}

func reviewFindingsArtifactFromReviewCheckpointRequest(reviewRequest *model.ReviewCheckpointRequest, assistantSequenceNo int) *model.ReviewFindingsArtifact {
	if reviewRequest == nil {
		return nil
	}
	if len(reviewRequest.Findings) == 0 && strings.TrimSpace(reviewRequest.OverallCorrectness) == "" && strings.TrimSpace(reviewRequest.OverallExplanation) == "" && reviewRequest.OverallConfidenceScore == nil {
		return nil
	}
	return &model.ReviewFindingsArtifact{
		Phase:                      strings.TrimSpace(reviewRequest.Phase),
		Title:                      strings.TrimSpace(reviewRequest.Title),
		Summary:                    strings.TrimSpace(reviewRequest.Summary),
		Findings:                   slices.Clone(reviewRequest.Findings),
		OverallCorrectness:         strings.TrimSpace(reviewRequest.OverallCorrectness),
		OverallExplanation:         strings.TrimSpace(reviewRequest.OverallExplanation),
		OverallConfidenceScore:     reviewRequest.OverallConfidenceScore,
		AssistantMessageSequenceNo: assistantSequenceNo,
		RecordedAt:                 time.Now().UTC(),
	}
}

func latestHumanInputRequestFromResult(result *workerpkg.ExecutionResult) *workerpkg.UserInputRequest {
	if result == nil {
		return nil
	}
	return workerpkg.ExtractLatestHumanInputRequest(result.ToolInvocations)
}

func latestCodexAuthStateFromResult(result *workerpkg.ExecutionResult) *model.CodexAuthState {
	if result == nil || result.CodexAuthState == nil {
		return nil
	}
	return result.CodexAuthState
}

func latestRunPlanFromResult(result *workerpkg.ExecutionResult) *workerpkg.RunPlanArtifact {
	if result == nil {
		return nil
	}
	return workerpkg.ExtractLatestRunPlan(result.ToolInvocations)
}

func humanInputArtifactFromWorker(req *workerpkg.UserInputRequest) model.HumanInputArtifact {
	return workerpkg.HumanInputArtifactFromUserInputRequest(req)
}
