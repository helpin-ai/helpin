package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestListCodingSessionEventsSkipsLegacyInteractionArtifactsWhenInteractionsExist(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)

	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	interactionRepo := repository.NewAgentRunInteractionRepository(db)

	now := time.Now().UTC()
	run := &model.AgentRun{
		ID:             "run-events-1",
		WorkspaceID:    "ws-1",
		AgentID:        "agent-1",
		TargetType:     "story",
		TargetID:       "story-1",
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeInteractive,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonHumanApproval,
		Status:         model.AgentRunStatusPaused,
		Input:          json.RawMessage(`{}`),
		OutputSummary:  json.RawMessage(`{}`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	legacyApproval := `{"phase":"prd","title":"Approve PRD","summary":"Review the latest draft."}`
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            "artifact-legacy-approval",
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  model.AgentRunArtifactTypeHumanApprovalRequest,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &legacyApproval,
		Metadata:      json.RawMessage(`{}`),
		SequenceNo:    1,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("create artifact: %v", err)
	}

	if err := interactionRepo.Create(context.Background(), &model.AgentRunInteraction{
		ID:                   "interaction-1",
		WorkspaceID:          run.WorkspaceID,
		RunID:                run.ID,
		RuntimeKind:          "native_sdk",
		InteractionKind:      model.AgentRunInteractionKindReviewCheckpoint,
		Status:               model.AgentRunInteractionStatusPending,
		RequestSchemaVersion: model.AgentRunInteractionSchemaVersionHelpinV1,
		RequestPayload:       json.RawMessage(`{"phase":"prd","title":"Approve PRD","summary":"Review the latest draft."}`),
		RuntimeMetadata:      json.RawMessage(`{"runtime_kind":"native_sdk"}`),
		Title:                strPtr("Approve PRD"),
		Summary:              strPtr("Review the latest draft."),
		CreatedAt:            now.Add(time.Millisecond),
		UpdatedAt:            now.Add(time.Millisecond),
	}); err != nil {
		t.Fatalf("create interaction: %v", err)
	}

	svc := &AgentService{
		runRepo:         runRepo,
		runMessageRepo:  runMessageRepo,
		artifactRepo:    artifactRepo,
		interactionRepo: interactionRepo,
	}

	events, err := svc.ListCodingSessionEvents(context.Background(), run.WorkspaceID, run.ID, 0)
	if err != nil {
		t.Fatalf("ListCodingSessionEvents returned error: %v", err)
	}

	if codingSessionEventTypes(events.Events, "interaction.requested") != 1 {
		t.Fatalf("expected one interaction.requested event, got %#v", events.Events)
	}
	if codingSessionEventTypes(events.Events, "approval.requested") != 0 {
		t.Fatalf("expected legacy approval.requested artifact event to be suppressed, got %#v", events.Events)
	}
}

func TestListCodingSessionEventsFallsBackToLegacyInteractionArtifactsWithoutInteractionRecords(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)

	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)

	now := time.Now().UTC()
	run := &model.AgentRun{
		ID:             "run-events-legacy",
		WorkspaceID:    "ws-1",
		AgentID:        "agent-1",
		TargetType:     "story",
		TargetID:       "story-1",
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeInteractive,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonHumanInput,
		Status:         model.AgentRunStatusPaused,
		Input:          json.RawMessage(`{}`),
		OutputSummary:  json.RawMessage(`{}`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	legacyInput := `{"questions":[{"id":"q1","type":"single_select","text":"Who owns this?","options":[{"value":"sales","label":"Sales"}]}]}`
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            "artifact-legacy-input",
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  model.AgentRunArtifactTypeHumanInputRequest,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &legacyInput,
		Metadata:      json.RawMessage(`{}`),
		SequenceNo:    1,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("create artifact: %v", err)
	}

	svc := &AgentService{
		runRepo:        runRepo,
		runMessageRepo: runMessageRepo,
		artifactRepo:   artifactRepo,
	}

	events, err := svc.ListCodingSessionEvents(context.Background(), run.WorkspaceID, run.ID, 0)
	if err != nil {
		t.Fatalf("ListCodingSessionEvents returned error: %v", err)
	}

	if codingSessionEventTypes(events.Events, "input.requested") != 1 {
		t.Fatalf("expected one legacy input.requested event, got %#v", events.Events)
	}
}

func TestGetCodingSessionIncludesLiveStreamSnapshotForActiveRuns(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)

	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	snapshotRepo := repository.NewCodingSessionStateSnapshotRepository(db)

	now := time.Now().UTC()
	run := &model.AgentRun{
		ID:             "run-session-snapshot",
		WorkspaceID:    "ws-1",
		AgentID:        "agent-1",
		TargetType:     "story",
		TargetID:       "story-1",
		RuntimeKind:    "codex",
		InvocationMode: model.InvocationModeInteractive,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusRunning,
		Input:          json.RawMessage(`{}`),
		OutputSummary:  json.RawMessage(`{}`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	payload, err := model.EncodeCodingSessionStreamSnapshot(&model.CodingSessionStreamSnapshot{
		LiveAssistantMessage: &model.CodingSessionLiveAssistantMessage{
			MessageID: "assistant-live-1",
			Content:   "Inspecting workspace",
			Status:    "streaming",
		},
	})
	if err != nil {
		t.Fatalf("encode snapshot: %v", err)
	}
	if err := snapshotRepo.Upsert(context.Background(), &model.CodingSessionStateSnapshot{
		ID:              "snapshot-1",
		WorkspaceID:     run.WorkspaceID,
		RunID:           run.ID,
		SchemaVersion:   model.CodingSessionStateSnapshotSchemaVersionV1,
		SnapshotPayload: payload,
		CreatedAt:       now,
		UpdatedAt:       now,
	}); err != nil {
		t.Fatalf("upsert snapshot: %v", err)
	}

	svc := &AgentService{
		runRepo:             runRepo,
		runMessageRepo:      runMessageRepo,
		artifactRepo:        artifactRepo,
		agentRepo:           agentRepo,
		sessionSnapshotRepo: snapshotRepo,
	}

	session, err := svc.GetCodingSession(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("GetCodingSession returned error: %v", err)
	}
	if session.StreamStateSnapshot == nil || session.StreamStateSnapshot.LiveAssistantMessage == nil {
		t.Fatalf("expected stream snapshot on coding session, got %#v", session)
	}
	if session.StreamStateSnapshot.LiveAssistantMessage.Content != "Inspecting workspace" {
		t.Fatalf("unexpected snapshot content %#v", session.StreamStateSnapshot.LiveAssistantMessage)
	}
}

func TestListCodingSessionEventsIncludesPersistedTurnSegmentsOnAssistantMessages(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)

	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)

	now := time.Now().UTC()
	run := &model.AgentRun{
		ID:             "run-events-turn-segments",
		WorkspaceID:    "ws-1",
		AgentID:        "agent-1",
		TargetType:     "story",
		TargetID:       "story-1",
		RuntimeKind:    "codex",
		InvocationMode: model.InvocationModeInteractive,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusRunning,
		Input:          json.RawMessage(`{}`),
		OutputSummary:  json.RawMessage(`{}`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	turnSegments, err := json.Marshal([]model.CodingSessionLiveTurnSegment{
		{
			SegmentID: "assistant-live-1:segment:1",
			Kind:      "assistant_message",
			AssistantMessage: &model.CodingSessionLiveAssistantMessage{
				MessageID: "assistant-live-1",
				Content:   "Inspecting files.\n",
				Status:    "completed",
			},
		},
		{
			SegmentID: "tool-1",
			Kind:      "tool_call",
			ToolCall: &model.CodingSessionLiveToolCall{
				ToolCallID: "tool-1",
				ToolName:   "read_file",
				Status:     "completed",
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal turn segments: %v", err)
	}

	if err := runMessageRepo.Create(context.Background(), &model.AgentRunMessage{
		ID:           "msg-assistant-turn-1",
		WorkspaceID:  run.WorkspaceID,
		RunID:        run.ID,
		Role:         "assistant",
		Content:      "Inspecting files.\nPatched the call site.",
		MessageType:  "assistant_turn",
		TurnSegments: turnSegments,
		SequenceNo:   1,
		CreatedAt:    now.Add(time.Second),
	}); err != nil {
		t.Fatalf("create assistant message: %v", err)
	}

	svc := &AgentService{
		runRepo:        runRepo,
		runMessageRepo: runMessageRepo,
		artifactRepo:   artifactRepo,
	}

	events, err := svc.ListCodingSessionEvents(context.Background(), run.WorkspaceID, run.ID, 0)
	if err != nil {
		t.Fatalf("ListCodingSessionEvents returned error: %v", err)
	}

	var assistantEvent *model.CodingSessionEvent
	for index := range events.Events {
		if events.Events[index].Type == "assistant.message.completed" {
			assistantEvent = &events.Events[index]
			break
		}
	}
	if assistantEvent == nil {
		t.Fatalf("expected assistant.message.completed event, got %#v", events.Events)
	}
	rawSegments, ok := assistantEvent.Payload["turn_segments"].(json.RawMessage)
	if !ok || len(rawSegments) == 0 {
		t.Fatalf("expected persisted turn_segments on assistant event, got %#v", assistantEvent.Payload)
	}
	var decoded []model.CodingSessionLiveTurnSegment
	if err := json.Unmarshal(rawSegments, &decoded); err != nil {
		t.Fatalf("unmarshal turn_segments: %v", err)
	}
	if len(decoded) != 2 || decoded[0].AssistantMessage == nil || decoded[0].AssistantMessage.Content != "Inspecting files.\n" {
		t.Fatalf("unexpected decoded turn segments %#v", decoded)
	}
}

func codingSessionEventTypes(events []model.CodingSessionEvent, eventType string) int {
	count := 0
	for _, event := range events {
		if event.Type == eventType {
			count++
		}
	}
	return count
}
