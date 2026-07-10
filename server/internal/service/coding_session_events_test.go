package service

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/worker"
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

func TestListCodingSessionEventsIncludesStructuredReviewFindingsArtifact(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)

	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)

	now := time.Now().UTC()
	run := &model.AgentRun{
		ID:             "run-events-review-findings",
		WorkspaceID:    "ws-1",
		AgentID:        "agent-1",
		TargetType:     "story",
		TargetID:       "story-1",
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeInteractive,
		ApprovalState:  "pending",
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

	payload := `{"phase":"review_findings","title":"Lens review findings","findings":[{"title":"Regression","body":"The sprint picker loses state.","priority":"P1"}],"overall_correctness":"incorrect"}`
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            "artifact-review-findings",
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  model.AgentRunArtifactTypeReviewFindings,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &payload,
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
	if codingSessionEventTypes(events.Events, "review.findings.updated") != 1 {
		t.Fatalf("expected one review.findings.updated event, got %#v", events.Events)
	}
}

func TestListCodingSessionEventsIncludesStructuredReviewDecisionArtifact(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)

	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)

	now := time.Now().UTC()
	run := &model.AgentRun{
		ID:             "run-events-review-decision",
		WorkspaceID:    "ws-1",
		AgentID:        "agent-1",
		TargetType:     "story",
		TargetID:       "story-1",
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeInteractive,
		ApprovalState:  "approved",
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

	payload := `{"title":"Lens review findings","decision":"approve","findings":[{"id":"finding_1","title":"Regression","status":"approved"}]}`
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            "artifact-review-decision",
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  model.AgentRunArtifactTypeReviewDecision,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &payload,
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
	if codingSessionEventTypes(events.Events, "review.decision.recorded") != 1 {
		t.Fatalf("expected one review.decision.recorded event, got %#v", events.Events)
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
	if session.ApprovalState != "not_required" {
		t.Fatalf("approval state = %q", session.ApprovalState)
	}
}

func TestListCodingSessionEventsIncludesLiveStreamSnapshotForActiveRuns(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)

	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	snapshotRepo := repository.NewCodingSessionStateSnapshotRepository(db)

	now := time.Now().UTC()
	run := &model.AgentRun{
		ID:             "run-event-snapshot",
		WorkspaceID:    "ws-1",
		AgentID:        "agent-1",
		TargetType:     "story",
		TargetID:       "story-1",
		RuntimeKind:    "native_sdk",
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
		LiveTurnSegments: []model.CodingSessionLiveTurnSegment{{
			SegmentID: "tool-1",
			Kind:      "tool_call",
			ToolCall: &model.CodingSessionLiveToolCall{
				ToolCallID: "tool-1",
				ToolName:   "read_file",
				Status:     "running",
			},
		}},
	})
	if err != nil {
		t.Fatalf("encode snapshot: %v", err)
	}
	if err := snapshotRepo.Upsert(context.Background(), &model.CodingSessionStateSnapshot{
		ID:              "snapshot-event-1",
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
		sessionSnapshotRepo: snapshotRepo,
	}

	events, err := svc.ListCodingSessionEvents(context.Background(), run.WorkspaceID, run.ID, 0)
	if err != nil {
		t.Fatalf("ListCodingSessionEvents returned error: %v", err)
	}
	if events.StreamStateSnapshot == nil || len(events.StreamStateSnapshot.LiveTurnSegments) != 1 {
		t.Fatalf("expected stream snapshot on event list, got %#v", events.StreamStateSnapshot)
	}
	if got := events.StreamStateSnapshot.LiveTurnSegments[0].ToolCall.ToolName; got != "read_file" {
		t.Fatalf("unexpected snapshot tool name %q", got)
	}
}

func TestGetCodingSessionIncludesStreamSnapshotForFailedRuns(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)

	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	snapshotRepo := repository.NewCodingSessionStateSnapshotRepository(db)

	now := time.Now().UTC()
	run := &model.AgentRun{
		ID:             "run-session-failed-snapshot",
		WorkspaceID:    "ws-1",
		AgentID:        "agent-1",
		TargetType:     "story",
		TargetID:       "story-1",
		RuntimeKind:    "codex",
		InvocationMode: model.InvocationModeInteractive,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusFailed,
		Input:          json.RawMessage(`{}`),
		OutputSummary:  json.RawMessage(`{}`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	payload, err := model.EncodeCodingSessionStreamSnapshot(&model.CodingSessionStreamSnapshot{
		CurrentPlan: &model.CodingSessionRunPlan{
			Plan: []model.CodingSessionRunPlanStep{{
				Step:   "Draft release note",
				Status: "in_progress",
			}},
		},
	})
	if err != nil {
		t.Fatalf("encode snapshot: %v", err)
	}
	if err := snapshotRepo.Upsert(context.Background(), &model.CodingSessionStateSnapshot{
		ID:              "snapshot-failed-1",
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
	if session.StreamStateSnapshot == nil || session.StreamStateSnapshot.CurrentPlan == nil {
		t.Fatalf("expected failed session stream snapshot, got %#v", session.StreamStateSnapshot)
	}
	if got := session.StreamStateSnapshot.CurrentPlan.Plan[0].Step; got != "Draft release note" {
		t.Fatalf("unexpected recovered plan step %q", got)
	}
}

func TestGetCodingSessionIncludesRunErrorMessage(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)

	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	agentRepo := repository.NewAgentRepository(db)

	now := time.Now().UTC()
	errMsg := "agent reached max tool steps after 300 tool-call rounds; start another run to continue"
	parentRunID := "run-parent-1"
	run := &model.AgentRun{
		ID:             "run-session-error",
		WorkspaceID:    "ws-1",
		AgentID:        "agent-1",
		TargetType:     "story",
		TargetID:       "story-1",
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeInteractive,
		ParentRunID:    &parentRunID,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusFailed,
		ErrorMessage:   &errMsg,
		Input:          json.RawMessage(`{}`),
		OutputSummary:  json.RawMessage(`{}`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	svc := &AgentService{
		runRepo:        runRepo,
		runMessageRepo: runMessageRepo,
		artifactRepo:   artifactRepo,
		agentRepo:      agentRepo,
	}

	session, err := svc.GetCodingSession(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("GetCodingSession returned error: %v", err)
	}
	if session.ErrorMessage == nil || *session.ErrorMessage != errMsg {
		t.Fatalf("expected coding session error_message %q, got %#v", errMsg, session.ErrorMessage)
	}
	if session.ParentRunID == nil || *session.ParentRunID != parentRunID {
		t.Fatalf("expected coding session parent_run_id %q, got %#v", parentRunID, session.ParentRunID)
	}
}

func TestGetCodingSessionIncludesAgentSystemPrompt(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)

	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	agentRepo := repository.NewAgentRepository(db)

	now := time.Now().UTC()
	systemPrompt := "Research configured competitors and file the changelog tracking report task."
	agent := &model.Agent{
		ID:                    "agent-session-prompt",
		WorkspaceID:           "ws-1",
		Name:                  "Competitive digest",
		Status:                "idle",
		RuntimeKind:           "native_sdk",
		Skills:                model.AgentSkillRefs{},
		TriggerMode:           "manual",
		ExecutionConfig:       model.JSONBlob(`{}`),
		SystemPrompt:          &systemPrompt,
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		AllowedTargets:        json.RawMessage(`[]`),
		ApprovalMode:          "never",
		MaxConcurrentRuns:     1,
		DefaultInvocationMode: model.InvocationModeAutonomous,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := agentRepo.Create(context.Background(), agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	run := &model.AgentRun{
		ID:             "run-session-system-prompt",
		WorkspaceID:    agent.WorkspaceID,
		AgentID:        agent.ID,
		TargetType:     "workspace",
		TargetID:       agent.WorkspaceID,
		RuntimeKind:    "native_sdk",
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

	svc := &AgentService{
		runRepo:        runRepo,
		runMessageRepo: runMessageRepo,
		artifactRepo:   artifactRepo,
		agentRepo:      agentRepo,
	}

	session, err := svc.GetCodingSession(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("GetCodingSession returned error: %v", err)
	}
	if session.SystemPrompt == nil || *session.SystemPrompt != systemPrompt {
		t.Fatalf("expected coding session system_prompt %q, got %#v", systemPrompt, session.SystemPrompt)
	}
	if session.Title != agent.Name {
		t.Fatalf("expected coding session title %q, got %q", agent.Name, session.Title)
	}
}

func TestBuildContinuationAdditionalContextIncludesFailureReasonAndHumanFollowup(t *testing.T) {
	errMsg := "agent reached max tool steps after 300 tool-call rounds; start another run to continue"
	run := &model.AgentRun{
		ID:           "run-prev-1",
		TargetType:   "epic",
		Status:       model.AgentRunStatusFailed,
		ErrorMessage: &errMsg,
	}

	context := buildContinuationAdditionalContext(run, "Focus on keeping the task breakdown intact.")
	if !strings.Contains(context, "Previous run ID: run-prev-1") {
		t.Fatalf("expected previous run id in continuation context, got %q", context)
	}
	if !strings.Contains(context, errMsg) {
		t.Fatalf("expected failure reason in continuation context, got %q", context)
	}
	if !strings.Contains(context, "Focus on keeping the task breakdown intact.") {
		t.Fatalf("expected human follow-up in continuation context, got %q", context)
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

func TestListCodingSessionEventsIncludesRuntimeToolCallArtifacts(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)

	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)

	now := time.Now().UTC()
	run := &model.AgentRun{
		ID:             "run-events-runtime-tool",
		WorkspaceID:    "ws-1",
		AgentID:        "agent-1",
		TargetType:     "workspace",
		TargetID:       "ws-1",
		RuntimeKind:    "native_sdk",
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

	started := `{"tool_call_id":"tool-1","tool_name":"fetch_url","args_text":"{\"url\":\"https://example.com\"}","parent_message_id":"assistant-1"}`
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            "artifact-tool-started",
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  model.AgentRunArtifactTypeToolCall,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &started,
		Metadata:      json.RawMessage(`{"runtime_event_type":"tool_call_started"}`),
		SequenceNo:    1,
		CreatedAt:     now.Add(time.Second),
	}); err != nil {
		t.Fatalf("create started artifact: %v", err)
	}
	completed := `{"tool_call_id":"tool-1","tool_name":"fetch_url","output_summary":"Fetched page","duration_ms":42,"parent_message_id":"assistant-1"}`
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            "artifact-tool-completed",
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  model.AgentRunArtifactTypeToolCall,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &completed,
		Metadata:      json.RawMessage(`{"runtime_event_type":"tool_call_finished"}`),
		SequenceNo:    2,
		CreatedAt:     now.Add(2 * time.Second),
	}); err != nil {
		t.Fatalf("create completed artifact: %v", err)
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

	var startedEvent, completedEvent *model.CodingSessionEvent
	for index := range events.Events {
		switch events.Events[index].Type {
		case "tool.call.started":
			startedEvent = &events.Events[index]
		case "tool.call.completed":
			completedEvent = &events.Events[index]
		}
	}
	if startedEvent == nil || completedEvent == nil {
		t.Fatalf("expected tool call events, got %#v", events.Events)
	}
	if startedEvent.Payload["tool_call_id"] != "tool-1" || startedEvent.Payload["tool_name"] != "fetch_url" {
		t.Fatalf("expected flattened started payload, got %#v", startedEvent.Payload)
	}
	if completedEvent.Payload["output_summary"] != "Fetched page" {
		t.Fatalf("expected flattened completed payload, got %#v", completedEvent.Payload)
	}
}

func TestListCodingSessionEventsEmitsResolvedInteractionAfterPreviousSequence(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)

	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	interactionRepo := repository.NewAgentRunInteractionRepository(db)

	now := time.Now().UTC()
	run := &model.AgentRun{
		ID:             "run-events-interaction-update",
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

	interaction := &model.AgentRunInteraction{
		ID:                   "interaction-lifecycle-1",
		WorkspaceID:          run.WorkspaceID,
		RunID:                run.ID,
		RuntimeKind:          "native_sdk",
		InteractionKind:      model.AgentRunInteractionKindReviewCheckpoint,
		Status:               model.AgentRunInteractionStatusPending,
		RequestSchemaVersion: model.AgentRunInteractionSchemaVersionHelpinV1,
		RequestPayload:       json.RawMessage(`{"phase":"prd","title":"Approve PRD","summary":"Review the draft."}`),
		RuntimeMetadata:      json.RawMessage(`{"runtime_kind":"native_sdk"}`),
		Title:                strPtr("Approve PRD"),
		Summary:              strPtr("Review the draft."),
		CreatedAt:            now.Add(time.Millisecond),
		UpdatedAt:            now.Add(time.Millisecond),
	}
	if err := interactionRepo.Create(context.Background(), interaction); err != nil {
		t.Fatalf("create interaction: %v", err)
	}

	svc := &AgentService{
		runRepo:         runRepo,
		runMessageRepo:  runMessageRepo,
		artifactRepo:    artifactRepo,
		interactionRepo: interactionRepo,
	}

	initialEvents, err := svc.ListCodingSessionEvents(context.Background(), run.WorkspaceID, run.ID, 0)
	if err != nil {
		t.Fatalf("ListCodingSessionEvents returned error: %v", err)
	}
	if codingSessionEventTypes(initialEvents.Events, "interaction.requested") != 1 {
		t.Fatalf("expected one interaction.requested event, got %#v", initialEvents.Events)
	}

	resolvedAt := now.Add(2 * time.Second)
	responseSchemaVersion := model.AgentRunInteractionSchemaVersionHelpinV1
	interaction.Status = model.AgentRunInteractionStatusResolved
	interaction.ResponseSchemaVersion = &responseSchemaVersion
	interaction.ResponsePayload = json.RawMessage(`{"decision":"approve"}`)
	interaction.ResolvedBy = strPtr("user-1")
	interaction.ResolvedAt = &resolvedAt
	interaction.UpdatedAt = resolvedAt
	if err := interactionRepo.Update(context.Background(), interaction); err != nil {
		t.Fatalf("update interaction: %v", err)
	}

	nextEvents, err := svc.ListCodingSessionEvents(context.Background(), run.WorkspaceID, run.ID, initialEvents.NextSequenceNo)
	if err != nil {
		t.Fatalf("ListCodingSessionEvents after resolved update returned error: %v", err)
	}
	if len(nextEvents.Events) != 1 || nextEvents.Events[0].Type != "interaction.resolved" {
		t.Fatalf("expected only interaction.resolved after previous sequence, got %#v", nextEvents.Events)
	}
}

func TestGetCodingSessionDiffUsesRunWorkspace(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)

	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)

	now := time.Now().UTC()
	run := &model.AgentRun{
		ID:             "run-diff-session-workdir",
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

	defer func() {
		_ = worker.CleanupWorkspaceForRun(run.ID)
	}()
	workDir := worker.PersistentWorkspacePathForRun(run.ID)
	_ = os.RemoveAll(filepath.Dir(workDir))
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir workdir: %v", err)
	}
	runGitCommand(t, workDir, "init")
	filePath := filepath.Join(workDir, "session.txt")
	if err := os.WriteFile(filePath, []byte("before\n"), 0o644); err != nil {
		t.Fatalf("write initial file: %v", err)
	}
	runGitCommand(t, workDir, "add", "session.txt")
	runGitCommand(t, workDir, "-c", "user.name=Test Runner", "-c", "user.email=test@example.com", "commit", "-m", "init")
	if err := os.WriteFile(filePath, []byte("after\n"), 0o644); err != nil {
		t.Fatalf("write modified file: %v", err)
	}

	svc := &AgentService{
		runRepo:        runRepo,
		runMessageRepo: runMessageRepo,
		artifactRepo:   artifactRepo,
	}

	diff, err := svc.GetCodingSessionDiff(context.Background(), run.WorkspaceID, run.ID, "session.txt")
	if err != nil {
		t.Fatalf("GetCodingSessionDiff returned error: %v", err)
	}
	if !strings.Contains(diff.Diff, "session.txt") || !strings.Contains(diff.Diff, "-before") || !strings.Contains(diff.Diff, "+after") {
		t.Fatalf("expected diff from session checkout, got %q", diff.Diff)
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

func runGitCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}
