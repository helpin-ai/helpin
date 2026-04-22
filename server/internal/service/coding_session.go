package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	"github.com/helpin-ai/helpin/server/internal/worker"
)

func (s *AgentService) GetCodingSession(ctx context.Context, workspaceID, sessionID string) (*model.CodingSession, error) {
	run, err := s.GetAgentRun(ctx, workspaceID, sessionID)
	if err != nil {
		return nil, err
	}
	return s.buildCodingSession(ctx, run)
}

func (s *AgentService) ListCodingSessionEvents(ctx context.Context, workspaceID, sessionID string, after int) (*model.CodingSessionEventListResponse, error) {
	run, err := s.GetAgentRun(ctx, workspaceID, sessionID)
	if err != nil {
		return nil, err
	}

	messages, err := s.ListRunMessages(ctx, workspaceID, sessionID)
	if err != nil {
		return nil, err
	}
	artifacts, err := s.ListRunArtifacts(ctx, workspaceID, sessionID)
	if err != nil {
		return nil, err
	}
	var interactions []model.AgentRunInteraction
	if s.interactionRepo != nil {
		interactions, err = s.interactionRepo.ListByRun(ctx, workspaceID, sessionID)
		if err != nil {
			return nil, err
		}
	}

	type pendingEvent struct {
		at      time.Time
		weight  int
		eventID string
		event   model.CodingSessionEvent
	}

	pending := make([]pendingEvent, 0, len(messages)+len(artifacts)+len(interactions)+1)
	for _, message := range messages {
		eventType := "user.message.completed"
		switch strings.TrimSpace(message.Role) {
		case "assistant":
			eventType = "assistant.message.completed"
		case "tool":
			eventType = "tool.call.completed"
		}
		payload := map[string]any{
			"message_id":       message.ID,
			"role":             message.Role,
			"message_type":     message.MessageType,
			"content":          message.Content,
			"sequence_no":      message.SequenceNo,
			"content_blocks":   json.RawMessage(message.ContentBlocks),
			"turn_segments":    json.RawMessage(message.TurnSegments),
			"tool_invocations": json.RawMessage(message.ToolInvocations),
		}
		pending = append(pending, pendingEvent{
			at:      message.CreatedAt.UTC(),
			weight:  10,
			eventID: "msg:" + message.ID,
			event: model.CodingSessionEvent{
				ID:          "msg:" + message.ID,
				SessionID:   run.ID,
				RunID:       run.ID,
				Timestamp:   message.CreatedAt.UTC(),
				Type:        eventType,
				RuntimeKind: run.RuntimeKind,
				Payload:     payload,
				RuntimeMetadata: map[string]any{
					"source": "agent_run_message",
				},
			},
		})
	}

	useArtifactInteractionFallback := shouldUseArtifactInteractionFallback(interactions)
	interactionDedupKeys := make(map[string]struct{}, len(interactions))
	for _, interaction := range interactions {
		if dedupKey := codingSessionDedupKeyForInteraction(interaction); dedupKey != "" {
			interactionDedupKeys[dedupKey] = struct{}{}
		}
		for _, interactionEvent := range codingSessionEventsFromInteraction(interaction) {
			pending = append(pending, pendingEvent{
				at:      interactionEvent.timestamp,
				weight:  20,
				eventID: interactionEvent.id,
				event: model.CodingSessionEvent{
					ID:              interactionEvent.id,
					SessionID:       run.ID,
					RunID:           run.ID,
					Timestamp:       interactionEvent.timestamp,
					Type:            interactionEvent.eventType,
					RuntimeKind:     firstNonEmptyString(interaction.RuntimeKind, run.RuntimeKind),
					Payload:         interactionEvent.payload,
					RuntimeMetadata: interactionEvent.runtimeMetadata,
				},
			})
		}
	}

	for _, artifact := range artifacts {
		if isLegacyArtifactInteractionRequestType(artifact.ArtifactType) && !useArtifactInteractionFallback {
			continue
		}
		if dedupKey := codingSessionDedupKeyForArtifact(artifact); dedupKey != "" {
			if _, exists := interactionDedupKeys[dedupKey]; exists {
				continue
			}
		}
		eventType, payload := codingSessionEventFromArtifact(artifact)
		if eventType == "" {
			continue
		}
		pending = append(pending, pendingEvent{
			at:      artifact.CreatedAt.UTC(),
			weight:  20,
			eventID: "artifact:" + artifact.ID,
			event: model.CodingSessionEvent{
				ID:          "artifact:" + artifact.ID,
				SessionID:   run.ID,
				RunID:       run.ID,
				Timestamp:   artifact.CreatedAt.UTC(),
				Type:        eventType,
				RuntimeKind: run.RuntimeKind,
				Payload:     payload,
				RuntimeMetadata: map[string]any{
					"source":        "agent_run_artifact",
					"artifact_type": artifact.ArtifactType,
				},
			},
		})
	}

	status, pauseReason := model.NormalizeAgentRunStatus(run.Status, run.PauseReason, run.ApprovalState, run.ExecutionStage)
	pending = append(pending, pendingEvent{
		at:      run.UpdatedAt.UTC(),
		weight:  30,
		eventID: "run:" + run.ID,
		event: model.CodingSessionEvent{
			ID:          "run:" + run.ID,
			SessionID:   run.ID,
			RunID:       run.ID,
			Timestamp:   run.UpdatedAt.UTC(),
			Type:        "session.updated",
			RuntimeKind: run.RuntimeKind,
			Payload: map[string]any{
				"status":          status,
				"pause_reason":    pauseReason,
				"execution_stage": strings.TrimSpace(derefString(run.ExecutionStage)),
				"approval_state":  run.ApprovalState,
				"error_message":   strings.TrimSpace(derefString(run.ErrorMessage)),
			},
			RuntimeMetadata: map[string]any{
				"source": "agent_run",
			},
		},
	})

	sort.SliceStable(pending, func(i, j int) bool {
		if !pending[i].at.Equal(pending[j].at) {
			return pending[i].at.Before(pending[j].at)
		}
		if pending[i].weight != pending[j].weight {
			return pending[i].weight < pending[j].weight
		}
		return pending[i].eventID < pending[j].eventID
	})

	events := make([]model.CodingSessionEvent, 0, len(pending))
	nextSequenceNo := 0
	for index, item := range pending {
		item.event.SequenceNo = index + 1
		nextSequenceNo = item.event.SequenceNo
		if item.event.SequenceNo <= after {
			continue
		}
		events = append(events, item.event)
	}

	return &model.CodingSessionEventListResponse{
		Events:         events,
		NextSequenceNo: nextSequenceNo,
	}, nil
}

func (s *AgentService) GetCodingSessionRepo(ctx context.Context, workspaceID, sessionID string) (*model.CodingSessionRepoState, error) {
	run, err := s.GetAgentRun(ctx, workspaceID, sessionID)
	if err != nil {
		return nil, err
	}
	repoState, err := s.resolveCodingSessionRepoState(ctx, run)
	if err != nil {
		return nil, err
	}
	return &repoState, nil
}

func (s *AgentService) GetCodingSessionDiff(ctx context.Context, workspaceID, sessionID, path string) (*model.CodingSessionDiff, error) {
	run, err := s.GetAgentRun(ctx, workspaceID, sessionID)
	if err != nil {
		return nil, err
	}

	workDir := worker.PersistentWorkspacePathForRun(run.ID)
	if info, statErr := os.Stat(workDir); statErr == nil && info.IsDir() {
		args := []string{"diff", "--no-ext-diff", "--"}
		if strings.TrimSpace(path) != "" {
			args = append(args, strings.TrimSpace(path))
		}
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = workDir
		out, err := cmd.CombinedOutput()
		if err == nil {
			diffPath := strings.TrimSpace(path)
			return &model.CodingSessionDiff{
				Path:        stringPtrIfNotEmpty(diffPath),
				Diff:        string(out),
				IsTruncated: false,
			}, nil
		}
	}

	artifacts, err := s.ListRunArtifacts(ctx, workspaceID, sessionID)
	if err != nil {
		return nil, err
	}
	for index := len(artifacts) - 1; index >= 0; index-- {
		artifact := artifacts[index]
		if artifact.InlineContent == nil {
			continue
		}
		switch strings.TrimSpace(artifact.ArtifactType) {
		case "codex_diff", "diff":
			diffPath := strings.TrimSpace(path)
			return &model.CodingSessionDiff{
				Path:        stringPtrIfNotEmpty(diffPath),
				Diff:        *artifact.InlineContent,
				IsTruncated: false,
			}, nil
		}
	}

	return &model.CodingSessionDiff{
		Path:        stringPtrIfNotEmpty(strings.TrimSpace(path)),
		Diff:        "",
		IsTruncated: false,
	}, nil
}

func (s *AgentService) ResolveCodingSessionInteraction(ctx context.Context, workspaceID, sessionID, interactionID, actorID string, req model.ResolveAgentRunInteractionRequest) (*model.AgentRunInteraction, error) {
	if s == nil || s.interactionRepo == nil {
		return nil, fmt.Errorf("run interactions are not configured")
	}
	if len(req.ResponsePayload) == 0 || strings.TrimSpace(string(req.ResponsePayload)) == "" || strings.TrimSpace(string(req.ResponsePayload)) == "null" {
		return nil, fmt.Errorf("response_payload is required")
	}

	run, err := s.GetAgentRun(ctx, workspaceID, sessionID)
	if err != nil {
		return nil, err
	}
	interaction, err := s.interactionRepo.GetByID(ctx, workspaceID, sessionID, interactionID)
	if err != nil {
		return nil, err
	}
	if interaction == nil {
		return nil, fmt.Errorf("interaction not found")
	}
	if strings.TrimSpace(interaction.Status) != model.AgentRunInteractionStatusPending {
		return nil, fmt.Errorf("interaction is not pending")
	}

	responsePayload := append(json.RawMessage(nil), req.ResponsePayload...)
	responseSchemaVersion := strings.TrimSpace(interaction.RequestSchemaVersion)
	if responseSchemaVersion == "" {
		responseSchemaVersion = model.AgentRunInteractionSchemaVersionHelpinV1
	}
	followupMessage := strings.TrimSpace(derefString(req.FollowupMessage))
	liveCodexPause, err := s.shouldUseLiveCodexPausePath(ctx, run)
	if err != nil {
		return nil, err
	}

	previousInteraction := *interaction
	if err := s.markInteractionResolved(ctx, interaction, actorID, responsePayload, responseSchemaVersion); err != nil {
		return nil, err
	}

	if liveCodexPause && strings.TrimSpace(run.RuntimeKind) == "codex" && interactionUsesNativeCodexResume(interaction) {
		if followupMessage != "" {
			if _, err := s.createRunMessage(ctx, run, "user", interactionMessageTypeForIntent(resolveIntentForInteraction(interaction, responsePayload)), followupMessage); err != nil {
				_ = s.restorePendingInteraction(ctx, &previousInteraction)
				return nil, err
			}
		}
		if err := s.persistResolvedInteractionArtifacts(ctx, run, interaction, actorID); err != nil {
			slog.ErrorContext(ctx, "failed to persist resolved interaction artifacts",
				"error", err,
				"workspace_id", run.WorkspaceID,
				"run_id", run.ID,
				"interaction_id", interaction.ID,
				"interaction_kind", interaction.InteractionKind,
			)
		}
		s.clearAgentAttentionNotification(ctx, run)
		s.publishResolvedInteractionEvent(run, interaction, actorID)
		return interaction, nil
	}

	resumeReq, err := resumeRequestForResolvedInteraction(interaction, responsePayload, followupMessage)
	if err != nil {
		_ = s.restorePendingInteraction(ctx, &previousInteraction)
		return nil, err
	}
	if _, _, err := s.resumeRunWithIntent(ctx, workspaceID, sessionID, actorID, resumeReq); err != nil {
		_ = s.restorePendingInteraction(ctx, &previousInteraction)
		return nil, err
	}
	if err := s.persistResolvedInteractionArtifacts(ctx, run, interaction, actorID); err != nil {
		slog.ErrorContext(ctx, "failed to persist resolved interaction artifacts",
			"error", err,
			"workspace_id", run.WorkspaceID,
			"run_id", run.ID,
			"interaction_id", interaction.ID,
			"interaction_kind", interaction.InteractionKind,
		)
	}
	s.publishResolvedInteractionEvent(run, interaction, actorID)
	return interaction, nil
}

func interactionUsesNativeCodexResume(interaction *model.AgentRunInteraction) bool {
	if interaction == nil {
		return false
	}
	if strings.TrimSpace(interaction.RuntimeKind) != "codex" {
		return false
	}
	switch strings.TrimSpace(interaction.InteractionKind) {
	case model.AgentRunInteractionKindRequestUserInput,
		model.AgentRunInteractionKindCommandExecutionApproval,
		model.AgentRunInteractionKindFileChangeApproval,
		model.AgentRunInteractionKindPermissionsApproval:
		return true
	default:
		return false
	}
}

func (s *AgentService) buildCodingSession(ctx context.Context, run *model.AgentRun) (*model.CodingSession, error) {
	if run == nil {
		return nil, fmt.Errorf("agent run not found")
	}
	repoState, err := s.resolveCodingSessionRepoState(ctx, run)
	if err != nil {
		return nil, err
	}

	var title string
	if agent, err := s.agentRepo.GetByID(ctx, run.WorkspaceID, run.AgentID); err == nil && agent != nil && strings.TrimSpace(agent.Name) != "" {
		title = strings.TrimSpace(agent.Name)
	}
	if title == "" {
		title = "Coding Session"
	}

	var summary *string
	if strings.TrimSpace(derefString(run.ExecutionStage)) != "" {
		stage := strings.TrimSpace(derefString(run.ExecutionStage))
		summary = &stage
	}

	artifacts, err := s.ListRunArtifacts(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return nil, err
	}

	var streamSnapshot *model.CodingSessionStreamSnapshot
	if s.sessionSnapshotRepo != nil && run.Status != model.AgentRunStatusCompleted && run.Status != model.AgentRunStatusFailed && run.Status != model.AgentRunStatusCancelled {
		snapshotRecord, err := s.sessionSnapshotRepo.GetByRun(ctx, run.WorkspaceID, run.ID)
		if err != nil {
			return nil, err
		}
		if snapshotRecord != nil {
			streamSnapshot, err = model.DecodeCodingSessionStreamSnapshot(snapshotRecord.SnapshotPayload)
			if err != nil {
				return nil, err
			}
		}
	}

	var triggeredBy *model.CodingSessionActor
	if run.TriggeredByUserID != nil && s.userRepo != nil {
		user, userErr := s.userRepo.GetByID(ctx, *run.TriggeredByUserID)
		if userErr != nil {
			return nil, userErr
		}
		if user != nil {
			triggeredBy = &model.CodingSessionActor{
				ID:        user.ID,
				Email:     user.Email,
				FullName:  user.FullName,
				AvatarURL: user.AvatarURL,
			}
		}
	}

	session := &model.CodingSession{
		ID:                  run.ID,
		RunID:               run.ID,
		ParentRunID:         run.ParentRunID,
		WorkspaceID:         run.WorkspaceID,
		TargetType:          run.TargetType,
		TargetID:            run.TargetID,
		AgentID:             run.AgentID,
		RuntimeKind:         run.RuntimeKind,
		InvocationMode:      run.InvocationMode,
		Status:              run.Status,
		PauseReason:         run.PauseReason,
		ErrorMessage:        run.ErrorMessage,
		Title:               title,
		Summary:             summary,
		Capabilities:        codingSessionCapabilitiesForRun(run),
		Repo:                repoState,
		CachedInputTokens:   run.CachedInputTokens,
		InputTokens:         run.InputTokens,
		OutputTokens:        run.OutputTokens,
		TokensUsed:          run.TokensUsed,
		AuthState:           latestCodexAuthArtifact(artifacts),
		StreamStateSnapshot: streamSnapshot,
		TriggeredByUser:     triggeredBy,
		CreatedAt:           run.CreatedAt,
		UpdatedAt:           run.UpdatedAt,
	}
	model.NormalizeAgentRunPauseState(run)
	session.Status = run.Status
	session.PauseReason = run.PauseReason
	return session, nil
}

func (s *AgentService) markInteractionResolved(ctx context.Context, interaction *model.AgentRunInteraction, actorID string, responsePayload json.RawMessage, responseSchemaVersion string) error {
	if s == nil || s.interactionRepo == nil || interaction == nil {
		return nil
	}
	now := time.Now().UTC()
	interaction.Status = model.AgentRunInteractionStatusResolved
	interaction.ResponsePayload = append(json.RawMessage(nil), responsePayload...)
	interaction.ResponseSchemaVersion = stringPtrIfNotEmpty(responseSchemaVersion)
	interaction.ResolvedBy = stringPtrIfNotEmpty(actorID)
	interaction.ResolvedAt = &now
	return s.interactionRepo.Update(ctx, interaction)
}

func (s *AgentService) restorePendingInteraction(ctx context.Context, interaction *model.AgentRunInteraction) error {
	if s == nil || s.interactionRepo == nil || interaction == nil {
		return nil
	}
	interaction.Status = model.AgentRunInteractionStatusPending
	interaction.ResponsePayload = nil
	interaction.ResponseSchemaVersion = nil
	interaction.ResolvedBy = nil
	interaction.ResolvedAt = nil
	return s.interactionRepo.Update(ctx, interaction)
}

func (s *AgentService) clearAgentAttentionNotification(ctx context.Context, run *model.AgentRun) {
	if s == nil || s.notificationService == nil || run == nil {
		return
	}
	if err := s.notificationService.MarkAgentAttentionResolved(ctx, run.WorkspaceID, run.ID); err != nil {
		slog.ErrorContext(ctx, "failed to clear agent attention notification",
			"error", err,
			"workspace_id", run.WorkspaceID,
			"run_id", run.ID,
		)
	}
}

func (s *AgentService) publishResolvedInteractionEvent(run *model.AgentRun, interaction *model.AgentRunInteraction, actorID string) {
	if s == nil || run == nil || interaction == nil {
		return
	}
	eventType, payload, _ := codingSessionEventFromInteraction(*interaction)
	if strings.TrimSpace(eventType) == "" {
		return
	}
	s.publishCodingSessionEvent(run, eventType, payload, actorID)
}

func resumeRequestForResolvedInteraction(interaction *model.AgentRunInteraction, responsePayload json.RawMessage, followupMessage string) (model.ResumeAgentRunRequest, error) {
	intent := resolveIntentForInteraction(interaction, responsePayload)
	switch strings.TrimSpace(interaction.InteractionKind) {
	case model.AgentRunInteractionKindRequestUserInput:
		content := requestUserInputResumeContent(interaction, responsePayload)
		if strings.TrimSpace(content) == "" {
			return model.ResumeAgentRunRequest{}, fmt.Errorf("request_user_input response is missing content")
		}
		return model.ResumeAgentRunRequest{
			Intent:          model.AgentRunResumeIntentReply,
			Content:         content,
			ResponsePayload: append(json.RawMessage(nil), responsePayload...),
		}, nil
	case model.AgentRunInteractionKindApprovalRequest:
		content := firstNonEmptyString(followupMessage, approvalRequestResumeContent(interaction.RequestPayload, responsePayload, intent))
		req := model.ResumeAgentRunRequest{
			Intent:          intent,
			ResponsePayload: append(json.RawMessage(nil), responsePayload...),
		}
		if intent == model.AgentRunResumeIntentApprove {
			if content != "" {
				req.Content = content
				req.SendMessage = true
			}
			return req, nil
		}
		req.Content = firstNonEmptyString(content, "Please revise and continue.")
		return req, nil
	case model.AgentRunInteractionKindReviewCheckpoint:
		content := firstNonEmptyString(followupMessage, reviewCheckpointResumeContent(interaction.RequestPayload, responsePayload, intent))
		req := model.ResumeAgentRunRequest{
			Intent:          intent,
			ResponsePayload: append(json.RawMessage(nil), responsePayload...),
		}
		if intent == model.AgentRunResumeIntentApprove {
			if content != "" {
				req.Content = content
				req.SendMessage = true
			}
			return req, nil
		}
		req.Content = firstNonEmptyString(content, "Please revise and continue.")
		return req, nil
	default:
		req := model.ResumeAgentRunRequest{
			Intent:          intent,
			ResponsePayload: append(json.RawMessage(nil), responsePayload...),
		}
		if intent == model.AgentRunResumeIntentApprove {
			if followupMessage != "" {
				req.Content = followupMessage
				req.SendMessage = true
			}
			return req, nil
		}
		req.Content = firstNonEmptyString(followupMessage, "Please revise and continue.")
		return req, nil
	}
}

type codingSessionInteractionEvent struct {
	id              string
	eventType       string
	timestamp       time.Time
	payload         map[string]any
	runtimeMetadata map[string]any
}

func codingSessionEventsFromInteraction(interaction model.AgentRunInteraction) []codingSessionInteractionEvent {
	status := strings.TrimSpace(interaction.Status)
	if status == "" {
		status = model.AgentRunInteractionStatusPending
	}

	statuses := []string{model.AgentRunInteractionStatusPending}
	if status != model.AgentRunInteractionStatusPending {
		statuses = append(statuses, status)
	}

	events := make([]codingSessionInteractionEvent, 0, len(statuses))
	for _, eventStatus := range statuses {
		eventType, payload, runtimeMetadata := codingSessionEventFromInteractionWithStatus(interaction, eventStatus)
		if eventType == "" {
			continue
		}
		events = append(events, codingSessionInteractionEvent{
			id:              fmt.Sprintf("interaction:%s:%s", interaction.ID, eventStatus),
			eventType:       eventType,
			timestamp:       codingSessionInteractionEventTimestamp(interaction, eventStatus),
			payload:         payload,
			runtimeMetadata: runtimeMetadata,
		})
	}
	return events
}

func codingSessionInteractionEventTimestamp(interaction model.AgentRunInteraction, status string) time.Time {
	switch strings.TrimSpace(status) {
	case model.AgentRunInteractionStatusPending:
		return interaction.CreatedAt.UTC()
	default:
		createdAt := interaction.CreatedAt.UTC()
		latest := createdAt
		if interaction.UpdatedAt.After(latest) {
			latest = interaction.UpdatedAt.UTC()
		}
		if interaction.ResolvedAt != nil && interaction.ResolvedAt.After(latest) {
			latest = interaction.ResolvedAt.UTC()
		}
		if !latest.After(createdAt) {
			latest = createdAt.Add(time.Nanosecond)
		}
		return latest
	}
}

func resolveIntentForInteraction(interaction *model.AgentRunInteraction, responsePayload json.RawMessage) string {
	if interaction == nil {
		return model.AgentRunResumeIntentReply
	}

	switch strings.TrimSpace(interaction.InteractionKind) {
	case model.AgentRunInteractionKindRequestUserInput:
		return model.AgentRunResumeIntentReply
	case model.AgentRunInteractionKindApprovalRequest:
		var payload struct {
			Decision string `json:"decision"`
		}
		if err := json.Unmarshal(responsePayload, &payload); err == nil && strings.TrimSpace(payload.Decision) == "approve" {
			return model.AgentRunResumeIntentApprove
		}
		return model.AgentRunResumeIntentRequestChanges
	case model.AgentRunInteractionKindReviewCheckpoint:
		var payload struct {
			Decision string `json:"decision"`
		}
		if err := json.Unmarshal(responsePayload, &payload); err == nil && strings.TrimSpace(payload.Decision) == "approve" {
			return model.AgentRunResumeIntentApprove
		}
		return model.AgentRunResumeIntentRequestChanges
	case model.AgentRunInteractionKindPermissionsApproval:
		var payload struct {
			Permissions map[string]any `json:"permissions"`
		}
		if err := json.Unmarshal(responsePayload, &payload); err == nil && len(payload.Permissions) > 0 {
			return model.AgentRunResumeIntentApprove
		}
		return model.AgentRunResumeIntentRequestChanges
	case model.AgentRunInteractionKindCommandExecutionApproval, model.AgentRunInteractionKindFileChangeApproval:
		var payload struct {
			Decision string `json:"decision"`
		}
		if err := json.Unmarshal(responsePayload, &payload); err == nil {
			switch strings.TrimSpace(payload.Decision) {
			case "accept", "acceptForSession", "acceptWithExecpolicyAmendment", "applyNetworkPolicyAmendment":
				return model.AgentRunResumeIntentApprove
			}
		}
		return model.AgentRunResumeIntentRequestChanges
	default:
		return model.AgentRunResumeIntentReply
	}
}

func interactionMessageTypeForIntent(intent string) string {
	switch strings.TrimSpace(intent) {
	case model.AgentRunResumeIntentApprove:
		return "approval"
	case model.AgentRunResumeIntentRequestChanges:
		return "request_changes"
	default:
		return "user_reply"
	}
}

func requestUserInputResumeContent(interaction *model.AgentRunInteraction, responsePayload json.RawMessage) string {
	if interaction == nil {
		return ""
	}
	if strings.TrimSpace(interaction.RequestSchemaVersion) == model.AgentRunInteractionSchemaVersionHelpinV1 {
		var payload struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(responsePayload, &payload); err == nil {
			return strings.TrimSpace(payload.Content)
		}
	}
	if content := codexUserInputResumeContent(interaction.RequestPayload, responsePayload); content != "" {
		return content
	}
	return strings.TrimSpace(string(responsePayload))
}

func reviewCheckpointResumeContent(requestPayload, responsePayload json.RawMessage, intent string) string {
	var response model.ReviewCheckpointResponse
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return ""
	}
	response.Message = strings.TrimSpace(response.Message)
	response.SelectionMode = strings.ToLower(strings.TrimSpace(response.SelectionMode))
	for i := range response.SelectedFindingIDs {
		response.SelectedFindingIDs[i] = strings.TrimSpace(response.SelectedFindingIDs[i])
	}

	var request model.ReviewCheckpointRequest
	if err := json.Unmarshal(requestPayload, &request); err != nil {
		return response.Message
	}
	selected := selectReviewFindings(request.Findings, response.SelectionMode, response.SelectedFindingIDs)

	switch strings.TrimSpace(intent) {
	case model.AgentRunResumeIntentApprove:
		if len(selected) == 0 {
			return response.Message
		}
		lines := []string{"Approved review findings for implementation:"}
		lines = append(lines, formatReviewFindingLines(selected)...)
		lines = append(lines, "Implement only these approved findings in the same branch. Run focused validation and summarize what changed.")
		if response.Message != "" {
			lines = append(lines, "Human note: "+response.Message)
		}
		return strings.Join(lines, "\n")
	case model.AgentRunResumeIntentRequestChanges:
		if len(selected) == 0 {
			return response.Message
		}
		lines := []string{"Revise or discuss the review using this selected scope:"}
		lines = append(lines, formatReviewFindingLines(selected)...)
		if response.Message != "" {
			lines = append(lines, "Human feedback: "+response.Message)
		}
		return strings.Join(lines, "\n")
	default:
		return response.Message
	}
}

func approvalRequestResumeContent(requestPayload, responsePayload json.RawMessage, intent string) string {
	var response model.ApprovalResponse
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return ""
	}
	response.Message = strings.TrimSpace(response.Message)
	if strings.TrimSpace(intent) == model.AgentRunResumeIntentApprove {
		if response.Message != "" {
			return response.Message
		}
		var request model.ApprovalRequest
		if err := json.Unmarshal(requestPayload, &request); err != nil {
			return "Approved. Continue."
		}
		switch strings.ToLower(strings.TrimSpace(request.Phase)) {
		case "prd":
			return "Approved PRD. Continue to task planning."
		case "tasks", "stories":
			return "Approved task plan. Apply it and create tasks."
		case "task_doc", "story_doc":
			return "Approved task planning document. Persist it and finish."
		default:
			return "Approved. Continue."
		}
	}
	return response.Message
}

func codexUserInputResumeContent(requestPayload, responsePayload json.RawMessage) string {
	var request struct {
		Questions []struct {
			ID       string `json:"id"`
			Header   string `json:"header"`
			Question string `json:"question"`
		} `json:"questions"`
	}
	var response struct {
		Answers map[string]struct {
			Answers []string `json:"answers"`
		} `json:"answers"`
	}
	if err := json.Unmarshal(requestPayload, &request); err != nil {
		return ""
	}
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return ""
	}
	if len(response.Answers) == 0 {
		return ""
	}

	lines := make([]string, 0, len(response.Answers))
	seen := make(map[string]struct{}, len(response.Answers))
	for _, question := range request.Questions {
		questionID := strings.TrimSpace(question.ID)
		answer, ok := response.Answers[questionID]
		if !ok || len(answer.Answers) == 0 {
			continue
		}
		value := strings.TrimSpace(answer.Answers[0])
		if value == "" {
			continue
		}
		prompt := strings.TrimSpace(question.Question)
		header := strings.TrimSpace(question.Header)
		switch {
		case header != "" && prompt != "" && !strings.EqualFold(header, prompt):
			prompt = header + ": " + prompt
		case prompt == "":
			prompt = firstNonEmptyString(header, questionID, "Question")
		}
		lines = append(lines, fmt.Sprintf("- %s -> %s", prompt, value))
		seen[questionID] = struct{}{}
	}

	if len(lines) < len(response.Answers) {
		keys := make([]string, 0, len(response.Answers))
		for key := range response.Answers {
			if _, exists := seen[key]; exists {
				continue
			}
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			answer := response.Answers[key]
			if len(answer.Answers) == 0 {
				continue
			}
			value := strings.TrimSpace(answer.Answers[0])
			if value == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("- %s -> %s", strings.TrimSpace(key), value))
		}
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func selectReviewFindings(findings []model.ReviewFinding, selectionMode string, selectedIDs []string) []model.ReviewFinding {
	if len(findings) == 0 {
		return nil
	}
	if selectionMode == "" || selectionMode == "all" {
		return append([]model.ReviewFinding(nil), findings...)
	}
	if selectionMode == "selected" && len(selectedIDs) == 0 {
		return nil
	}
	selectedSet := make(map[string]struct{}, len(selectedIDs))
	for _, id := range selectedIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		selectedSet[id] = struct{}{}
	}
	if len(selectedSet) == 0 {
		return nil
	}
	selected := make([]model.ReviewFinding, 0, len(selectedSet))
	for _, finding := range findings {
		if _, ok := selectedSet[strings.TrimSpace(finding.ID)]; ok {
			selected = append(selected, finding)
		}
	}
	return selected
}

func formatReviewFindingLines(findings []model.ReviewFinding) []string {
	lines := make([]string, 0, len(findings))
	for _, finding := range findings {
		line := "- "
		if priority := strings.TrimSpace(finding.Priority); priority != "" {
			line += priority + " "
		}
		line += strings.TrimSpace(finding.Title)
		if location := strings.TrimSpace(finding.CodeLocation); location != "" {
			line += " (" + location + ")"
		}
		if body := strings.TrimSpace(finding.Body); body != "" {
			line += ": " + body
		}
		lines = append(lines, line)
	}
	return lines
}

func (s *AgentService) resolveCodingSessionRepoState(ctx context.Context, run *model.AgentRun) (model.CodingSessionRepoState, error) {
	state := model.CodingSessionRepoState{
		RepoName: stringPtrIfNotEmpty(strings.TrimSpace(derefString(run.RepoFullName))),
		Branch:   stringPtrIfNotEmpty(strings.TrimSpace(derefString(run.WorkingBranch))),
	}
	if state.Branch == nil {
		state.Branch = stringPtrIfNotEmpty(strings.TrimSpace(derefString(run.BaseBranch)))
	}

	workDir := worker.PersistentWorkspacePathForRun(run.ID)
	if info, err := os.Stat(workDir); err == nil && info.IsDir() {
		if repoName := strings.TrimSpace(derefString(run.RepoFullName)); repoName != "" {
			state.RepoName = &repoName
		}
		if branch := gitCurrentBranch(ctx, workDir); branch != "" {
			state.Branch = &branch
		}
		files := gitChangedFiles(ctx, workDir)
		state.ChangedFiles = files
		state.ChangedFileCount = len(files)
		state.IsDirty = len(files) > 0
		return state, nil
	}

	if summary := parseChangedFilesFromOutputSummary(run.OutputSummary); len(summary) > 0 {
		state.ChangedFiles = summary
		state.ChangedFileCount = len(summary)
		state.IsDirty = len(summary) > 0
	}
	return state, nil
}

func codingSessionCapabilitiesForRun(run *model.AgentRun) model.CodingSessionCapabilities {
	capabilities := model.CodingSessionCapabilities{
		LiveTextStreaming: true,
		ToolStreaming:     true,
		RepoDiffStreaming: true,
		PlanStreaming:     true,
		Approvals:         true,
		HumanInput:        true,
		Authentication:    false,
		Previews:          true,
		TerminalOutput:    true,
		Checkpoints:       true,
	}
	switch strings.TrimSpace(run.RuntimeKind) {
	case "codex":
		capabilities.Authentication = true
	case "opencode":
		capabilities.Authentication = false
	case "native_sdk":
		capabilities.Authentication = false
		capabilities.RepoDiffStreaming = strings.TrimSpace(run.InvocationMode) == model.InvocationModeInteractive
	}
	if strings.TrimSpace(run.InvocationMode) != model.InvocationModeInteractive {
		capabilities.Approvals = false
		capabilities.HumanInput = false
		capabilities.Authentication = false
	}
	return capabilities
}

func latestCodexAuthArtifact(artifacts []model.AgentRunArtifact) *model.CodexAuthState {
	for index := len(artifacts) - 1; index >= 0; index-- {
		artifact := artifacts[index]
		if strings.TrimSpace(artifact.ArtifactType) != model.AgentRunArtifactTypeCodexAuthState || artifact.InlineContent == nil {
			continue
		}
		var state model.CodexAuthState
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &state); err != nil {
			continue
		}
		return &state
	}
	return nil
}

func codingSessionEventFromArtifact(artifact model.AgentRunArtifact) (string, map[string]any) {
	payload := map[string]any{
		"artifact_id":   artifact.ID,
		"artifact_type": artifact.ArtifactType,
	}
	if artifact.InlineContent != nil {
		var raw any
		if json.Unmarshal([]byte(*artifact.InlineContent), &raw) == nil {
			payload["content"] = raw
		} else {
			payload["content"] = *artifact.InlineContent
		}
	}
	switch strings.TrimSpace(artifact.ArtifactType) {
	case model.AgentRunArtifactTypeHumanInputRequest:
		return "input.requested", payload
	case model.AgentRunArtifactTypeHumanApprovalRequest:
		return "approval.requested", payload
	case model.AgentRunArtifactTypeReviewFindings:
		return "review.findings.updated", payload
	case model.AgentRunArtifactTypeReviewDecision:
		return "review.decision.recorded", payload
	case model.AgentRunArtifactTypeCodexAuthState:
		return "auth.updated", payload
	case "codex_diff", "diff":
		return "repo.diff.updated", payload
	case model.AgentRunArtifactTypeRunPlan:
		return "activity.updated", payload
	case worker.RunPreviewArtifactType:
		return "preview.updated", payload
	default:
		return "", nil
	}
}

func codingSessionEventFromInteraction(interaction model.AgentRunInteraction) (string, map[string]any, map[string]any) {
	return codingSessionEventFromInteractionWithStatus(interaction, strings.TrimSpace(interaction.Status))
}

func codingSessionEventFromInteractionWithStatus(interaction model.AgentRunInteraction, status string) (string, map[string]any, map[string]any) {
	interaction = codingSessionInteractionEventView(interaction, status)
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
		"interaction_id":                interaction.ID,
		"interaction_kind":              interaction.InteractionKind,
		"status":                        interaction.Status,
		"request_schema_version":        interaction.RequestSchemaVersion,
		"request_payload":               json.RawMessage(interaction.RequestPayload),
		"title":                         derefString(interaction.Title),
		"summary":                       derefString(interaction.Summary),
		"request_id":                    derefString(interaction.RequestID),
		"thread_id":                     derefString(interaction.ThreadID),
		"turn_id":                       derefString(interaction.TurnID),
		"item_id":                       derefString(interaction.ItemID),
		"approval_id":                   derefString(interaction.ApprovalID),
		"assistant_message_sequence_no": interactionAssistantSequenceNo(interaction),
	}
	if interaction.ResponseSchemaVersion != nil && strings.TrimSpace(*interaction.ResponseSchemaVersion) != "" {
		payload["response_schema_version"] = strings.TrimSpace(*interaction.ResponseSchemaVersion)
	}
	if len(interaction.ResponsePayload) > 0 {
		payload["response_payload"] = json.RawMessage(interaction.ResponsePayload)
	}
	if interaction.ResolvedAt != nil {
		payload["resolved_at"] = interaction.ResolvedAt.UTC()
	}
	if interaction.ResolvedBy != nil && strings.TrimSpace(*interaction.ResolvedBy) != "" {
		payload["resolved_by"] = strings.TrimSpace(*interaction.ResolvedBy)
	}

	runtimeMetadata := map[string]any{
		"source":           "agent_run_interaction",
		"interaction_kind": interaction.InteractionKind,
	}
	if len(interaction.RuntimeMetadata) > 0 && string(interaction.RuntimeMetadata) != "null" {
		var metadata map[string]any
		if err := json.Unmarshal(interaction.RuntimeMetadata, &metadata); err == nil && len(metadata) > 0 {
			runtimeMetadata["interaction_metadata"] = metadata
		}
	}
	return eventType, payload, runtimeMetadata
}

func codingSessionInteractionEventView(interaction model.AgentRunInteraction, status string) model.AgentRunInteraction {
	view := interaction
	status = strings.TrimSpace(status)
	if status == "" {
		status = strings.TrimSpace(view.Status)
	}
	if status == "" {
		status = model.AgentRunInteractionStatusPending
	}
	view.Status = status
	if status == model.AgentRunInteractionStatusPending {
		view.ResponseSchemaVersion = nil
		view.ResponsePayload = nil
		view.ResolvedAt = nil
		view.ResolvedBy = nil
	}
	return view
}

func interactionAssistantSequenceNo(interaction model.AgentRunInteraction) int {
	if interaction.AssistantMessageSequenceNo == nil {
		return 0
	}
	return *interaction.AssistantMessageSequenceNo
}

func codingSessionDedupKeyForInteraction(interaction model.AgentRunInteraction) string {
	if interaction.AssistantMessageSequenceNo == nil || *interaction.AssistantMessageSequenceNo <= 0 {
		return ""
	}
	return fmt.Sprintf("%d:%s", *interaction.AssistantMessageSequenceNo, strings.TrimSpace(interaction.InteractionKind))
}

func codingSessionDedupKeyForArtifact(artifact model.AgentRunArtifact) string {
	assistantSequenceNo := artifactAssistantMessageSequenceNo(artifact)
	if assistantSequenceNo <= 0 {
		return ""
	}
	interactionKind := artifactInteractionKind(artifact)
	if interactionKind == "" {
		return ""
	}
	return fmt.Sprintf("%d:%s", assistantSequenceNo, interactionKind)
}

func shouldUseArtifactInteractionFallback(interactions []model.AgentRunInteraction) bool {
	return len(interactions) == 0
}

func isLegacyArtifactInteractionRequestType(artifactType string) bool {
	switch strings.TrimSpace(artifactType) {
	case model.AgentRunArtifactTypeHumanInputRequest, model.AgentRunArtifactTypeHumanApprovalRequest:
		return true
	default:
		return false
	}
}

func artifactInteractionKind(artifact model.AgentRunArtifact) string {
	switch strings.TrimSpace(artifact.ArtifactType) {
	case model.AgentRunArtifactTypeHumanInputRequest:
		return model.AgentRunInteractionKindRequestUserInput
	case model.AgentRunArtifactTypeHumanApprovalRequest:
		var metadata struct {
			RuntimeKind      string `json:"runtime_kind"`
			CodexRequestKind string `json:"codex_request_kind"`
		}
		_ = json.Unmarshal(artifact.Metadata, &metadata)
		if strings.TrimSpace(metadata.RuntimeKind) == "codex" {
			switch strings.TrimSpace(metadata.CodexRequestKind) {
			case "command_execution":
				return model.AgentRunInteractionKindCommandExecutionApproval
			case "file_change":
				return model.AgentRunInteractionKindFileChangeApproval
			case "permissions":
				return model.AgentRunInteractionKindPermissionsApproval
			}
		}
		var review model.ReviewCheckpointRequest
		if artifact.InlineContent != nil && json.Unmarshal([]byte(*artifact.InlineContent), &review) == nil {
			if len(review.Findings) > 0 || strings.TrimSpace(review.OverallCorrectness) != "" || strings.TrimSpace(review.OverallExplanation) != "" || review.OverallConfidenceScore != nil {
				return model.AgentRunInteractionKindReviewCheckpoint
			}
		}
		return model.AgentRunInteractionKindApprovalRequest
	default:
		return ""
	}
}

func (s *AgentService) publishCodingSessionUpdated(run *model.AgentRun, actorID string) {
	if s.wsPublisher == nil || run == nil {
		return
	}
	status, pauseReason := model.NormalizeAgentRunStatus(run.Status, run.PauseReason, run.ApprovalState, run.ExecutionStage)
	data, _ := json.Marshal(map[string]any{
		"parent_run_id":   strings.TrimSpace(derefString(run.ParentRunID)),
		"id":              run.ID,
		"run_id":          run.ID,
		"status":          status,
		"pause_reason":    pauseReason,
		"runtime_kind":    run.RuntimeKind,
		"invocation_mode": run.InvocationMode,
		"execution_stage": strings.TrimSpace(derefString(run.ExecutionStage)),
	})
	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "coding_session",
		EntityID:    run.ID,
		WorkspaceID: run.WorkspaceID,
		ActorID:     actorID,
		ParentType:  run.TargetType,
		ParentID:    run.TargetID,
		Data:        data,
	})
}

func (s *AgentService) publishCodingSessionEvent(run *model.AgentRun, eventType string, payload map[string]any, actorID string) {
	if s.wsPublisher == nil || run == nil || strings.TrimSpace(eventType) == "" {
		return
	}
	event := model.CodingSessionEvent{
		ID:          fmt.Sprintf("%s:%d", run.ID, time.Now().UTC().UnixNano()),
		SessionID:   run.ID,
		RunID:       run.ID,
		SequenceNo:  int(time.Now().UTC().UnixMilli()),
		Timestamp:   time.Now().UTC(),
		Type:        eventType,
		RuntimeKind: run.RuntimeKind,
		Payload:     payload,
	}
	data, _ := json.Marshal(event)
	s.wsPublisher.Publish(websocket.Event{
		Action:      "created",
		Entity:      "coding_session_event",
		EntityID:    event.ID,
		WorkspaceID: run.WorkspaceID,
		ActorID:     actorID,
		ParentType:  "coding_session",
		ParentID:    run.ID,
		Data:        data,
	})
}

func parseChangedFilesFromOutputSummary(raw json.RawMessage) []model.CodingSessionRepoFile {
	if len(raw) == 0 {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	items, _ := payload["changed_files"].([]any)
	files := make([]model.CodingSessionRepoFile, 0, len(items))
	for _, item := range items {
		switch value := item.(type) {
		case string:
			path := strings.TrimSpace(value)
			if path == "" {
				continue
			}
			files = append(files, model.CodingSessionRepoFile{Path: path, Status: "modified"})
		case map[string]any:
			path := strings.TrimSpace(codingSessionStringValue(value["path"]))
			if path == "" {
				continue
			}
			status := strings.TrimSpace(codingSessionStringValue(value["status"]))
			if status == "" {
				status = "modified"
			}
			files = append(files, model.CodingSessionRepoFile{Path: path, Status: status})
		}
	}
	return files
}

func gitCurrentBranch(ctx context.Context, workDir string) string {
	out, err := exec.CommandContext(ctx, "git", "-C", workDir, "branch", "--show-current").CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitChangedFiles(ctx context.Context, workDir string) []model.CodingSessionRepoFile {
	out, err := exec.CommandContext(ctx, "git", "-C", workDir, "status", "--porcelain").CombinedOutput()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	files := make([]model.CodingSessionRepoFile, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(line) == "" || len(line) < 4 {
			continue
		}
		statusCode := strings.TrimSpace(line[:2])
		path := strings.TrimSpace(line[3:])
		if path == "" {
			continue
		}
		files = append(files, model.CodingSessionRepoFile{
			Path:   path,
			Status: gitStatusLabel(statusCode),
		})
	}
	return files
}

func gitStatusLabel(code string) string {
	switch {
	case strings.Contains(code, "A"):
		return "added"
	case strings.Contains(code, "D"):
		return "deleted"
	case strings.Contains(code, "R"):
		return "renamed"
	case strings.Contains(code, "M"):
		return "modified"
	case strings.Contains(code, "U"):
		return "unmerged"
	case strings.Contains(code, "?"):
		return "untracked"
	default:
		return "modified"
	}
}

func codingSessionStringValue(value any) string {
	text, _ := value.(string)
	return text
}

func stringPtrIfNotEmpty(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(value)
	return &trimmed
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func sessionPathLabel(path string) *string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	cleaned := filepath.Clean(path)
	return &cleaned
}
