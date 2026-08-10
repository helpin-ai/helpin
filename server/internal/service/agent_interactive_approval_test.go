package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func seedApprovalArtifacts(t *testing.T, artifactRepo *repository.AgentRunArtifactRepository, workspaceID, runID string, assistantSequenceNo int, now time.Time, previewContent string) {
	t.Helper()

	runPreviewContent := fmt.Sprintf(`{"panel_key":"prd_draft","title":"PRD Draft","format":"markdown","content":%q,"replace":true}`, previewContent)
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            fmt.Sprintf("artifact-run-preview-%s-%d", runID, assistantSequenceNo),
		WorkspaceID:   workspaceID,
		RunID:         runID,
		ArtifactType:  agentcontract.RunPreviewArtifactType,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &runPreviewContent,
		Metadata:      json.RawMessage(fmt.Sprintf(`{"assistant_message_sequence_no":%d}`, assistantSequenceNo)),
		SequenceNo:    assistantSequenceNo*2 - 1,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("create run preview artifact: %v", err)
	}

	approvalContent := `{"phase":"prd","title":"Approve PRD","summary":"Review the current draft"}`
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            fmt.Sprintf("artifact-approval-%s-%d", runID, assistantSequenceNo),
		WorkspaceID:   workspaceID,
		RunID:         runID,
		ArtifactType:  model.AgentRunArtifactTypeHumanApprovalRequest,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &approvalContent,
		Metadata:      json.RawMessage(fmt.Sprintf(`{"assistant_message_sequence_no":%d}`, assistantSequenceNo)),
		SequenceNo:    assistantSequenceNo * 2,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("create approval artifact: %v", err)
	}
}

func seedCodexPendingSessionState(t *testing.T, artifactRepo *repository.AgentRunArtifactRepository, workspaceID, runID string, now time.Time) {
	t.Helper()

	content := `{"thread_id":"thread-1","pending_request":{"kind":"command_execution","request_id":"7","request_id_raw":7,"turn_id":"turn-1","item_id":"item-1","payload":{"command":"git commit"}}}`
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            fmt.Sprintf("artifact-codex-session-%s", runID),
		WorkspaceID:   workspaceID,
		RunID:         runID,
		ArtifactType:  "codex_session_state",
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &content,
		Metadata:      json.RawMessage(`{"internal":true}`),
		SequenceNo:    1,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("create codex session artifact: %v", err)
	}
}

func TestSendRunMessageTreatsExplicitApprovalAsNormalUserReply(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Epic Planner", model.AgentPresetEpicPlanner, "Planner", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:                "run-1",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_reply_approve"),
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		TargetType:        "epic",
		TargetID:          "epic-1",
		RuntimeKind:       "native_sdk",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     "not_required",
		PauseReason:       model.AgentRunPauseReasonHumanInput,
		Status:            model.AgentRunStatusPaused,
		OutputSummary:     []byte(`{"status":"waiting"}`),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := runMessageRepo.Create(context.Background(), &model.AgentRunMessage{
		WorkspaceID: "ws-1",
		RunID:       run.ID,
		Role:        "assistant",
		Content:     "Please review the latest PRD draft in the preview pane.",
		MessageType: "assistant_turn",
		ToolInvocations: mustMarshalTestJSON(t, []model.ToolInvocation{
			{
				ToolName: agentcontract.ToolPublishPreview,
				Input:    json.RawMessage(`{"panel_key":"prd_draft","title":"PRD Draft","format":"markdown","content":"# Problem\n\nDraft body"}`),
			},
			{
				ToolName: agentcontract.ToolRequestHumanApproval,
				Input:    json.RawMessage(`{"phase":"prd","title":"Approve PRD","summary":"Review the current draft"}`),
			},
		}),
		SequenceNo: 1,
	}); err != nil {
		t.Fatalf("create message: %v", err)
	}
	seedApprovalArtifacts(t, artifactRepo, "ws-1", run.ID, 1, now, "# Problem\n\nDraft body")

	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		artifactRepo:       artifactRepo,
		agentRuntimeClient: &fakeAgentRuntimeSignalClient{},
	}

	message, err := svc.SendRunMessage(context.Background(), "ws-1", run.ID, "user-1", model.SendAgentRunMessageRequest{
		Content: "good to go",
	})
	if err != nil {
		t.Fatalf("SendRunMessage returned error: %v", err)
	}
	if message.MessageType != "user_reply" {
		t.Fatalf("expected user_reply message type, got %q", message.MessageType)
	}

	runtimeClient := svc.agentRuntimeClient.(*fakeAgentRuntimeSignalClient)
	if len(runtimeClient.resumeCalls) != 1 {
		t.Fatalf("expected one agent runtime resume call, got %#v", runtimeClient.resumeCalls)
	}
	resumeCall := runtimeClient.resumeCalls[0]
	if resumeCall.runID != "run_rt_reply_approve" {
		t.Fatalf("expected runtime run id run_rt_reply_approve, got %q", resumeCall.runID)
	}
	if resumeCall.req.Intent != model.AgentRunResumeIntentReply || resumeCall.req.Content != "good to go" {
		t.Fatalf("expected reply resume request, got %#v", resumeCall.req)
	}

	updated, err := runRepo.GetByID(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	// Status stays paused locally: the runtime event projection owns the
	// status/pause flip for delegated runs.
	if updated.Status != model.AgentRunStatusPaused {
		t.Fatalf("expected run status %q, got %q", model.AgentRunStatusPaused, updated.Status)
	}
	if updated.ApprovalState != "not_required" {
		t.Fatalf("expected approval_state not_required, got %q", updated.ApprovalState)
	}
	if updated.ExecutionStage == nil || *updated.ExecutionStage != "resuming" {
		t.Fatalf("expected execution stage resuming, got %#v", updated.ExecutionStage)
	}
	if string(updated.OutputSummary) != `{"status":"waiting"}` {
		t.Fatalf("expected output_summary to remain unchanged, got %s", string(updated.OutputSummary))
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	var approvedArtifact *model.AgentRunArtifact
	for i := range artifacts {
		if artifacts[i].ArtifactType == model.AgentRunArtifactTypeApprovedPreview {
			approvedArtifact = &artifacts[i]
			break
		}
	}
	if approvedArtifact == nil {
		t.Fatalf("expected approved preview artifact, got %#v", artifacts)
	}
	var approved model.ApprovedRunPreview
	if err := json.Unmarshal([]byte(derefString(approvedArtifact.InlineContent)), &approved); err != nil {
		t.Fatalf("unmarshal approved preview artifact: %v", err)
	}
	if approved.Phase != "prd" || approved.PanelKey != "prd_draft" || approved.Format != agentcontract.PreviewFormatMarkdown {
		t.Fatalf("unexpected approved preview payload: %#v", approved)
	}
}

func TestSendRunMessageResolvesLatestPendingCodexInputInteraction(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	interactionRepo := repository.NewAgentRunInteractionRepository(db)

	now := time.Now().UTC()
	run := &model.AgentRun{
		ID:                "run-codex-input",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_codex_input"),
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		TargetType:        "story",
		TargetID:          "story-1",
		RuntimeKind:       "codex",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     "not_required",
		PauseReason:       model.AgentRunPauseReasonHumanInput,
		Status:            model.AgentRunStatusPaused,
		LastHeartbeatAt:   &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	seedCodexPendingSessionState(t, artifactRepo, run.WorkspaceID, run.ID, now)

	requestPayload := json.RawMessage(`{
		"threadId":"thread-1",
		"turnId":"turn-1",
		"itemId":"item-1",
		"questions":[{"id":"tier","header":"Confirm","question":"Which tier should we use?","isOther":false,"isSecret":false,"options":[{"label":"Enterprise","description":"Use enterprise tier"}]}]
	}`)
	if err := interactionRepo.Create(context.Background(), &model.AgentRunInteraction{
		ID:                         "interaction-1",
		WorkspaceID:                run.WorkspaceID,
		RunID:                      run.ID,
		RuntimeKind:                "codex",
		InteractionKind:            model.AgentRunInteractionKindRequestUserInput,
		Status:                     model.AgentRunInteractionStatusPending,
		RequestSchemaVersion:       model.AgentRunInteractionSchemaVersionCodexV2,
		RequestID:                  strPtr("7"),
		ThreadID:                   strPtr("thread-1"),
		TurnID:                     strPtr("turn-1"),
		ItemID:                     strPtr("item-1"),
		AssistantMessageSequenceNo: intPtr(7),
		Title:                      strPtr("User input required"),
		RequestPayload:             requestPayload,
		RuntimeMetadata:            json.RawMessage(`{"runtime_kind":"codex"}`),
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}); err != nil {
		t.Fatalf("create interaction: %v", err)
	}

	svc := &AgentService{
		agentRepo:          repository.NewAgentRepository(db),
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		artifactRepo:       artifactRepo,
		interactionRepo:    interactionRepo,
		agentRuntimeClient: &fakeAgentRuntimeSignalClient{},
	}

	message, err := svc.SendRunMessage(context.Background(), run.WorkspaceID, run.ID, "user-1", model.SendAgentRunMessageRequest{
		Content: "enterprise",
	})
	if err != nil {
		t.Fatalf("SendRunMessage returned error: %v", err)
	}
	if message == nil || message.MessageType != "user_reply" {
		t.Fatalf("expected user_reply message, got %#v", message)
	}

	interaction, err := interactionRepo.GetLatestPendingByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("get latest pending interaction: %v", err)
	}
	if interaction != nil {
		t.Fatalf("expected no pending interactions, got %#v", interaction)
	}

	interactions, err := interactionRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list interactions: %v", err)
	}
	if len(interactions) != 1 {
		t.Fatalf("expected one interaction, got %#v", interactions)
	}
	resolved := interactions[0]
	if resolved.Status != model.AgentRunInteractionStatusResolved {
		t.Fatalf("expected resolved status, got %#v", resolved)
	}
	if resolved.ResponseSchemaVersion == nil || *resolved.ResponseSchemaVersion != model.AgentRunInteractionSchemaVersionCodexV2 {
		t.Fatalf("expected codex response schema version, got %#v", resolved.ResponseSchemaVersion)
	}
	var response struct {
		Answers map[string]struct {
			Answers []string `json:"answers"`
		} `json:"answers"`
	}
	if err := json.Unmarshal(resolved.ResponsePayload, &response); err != nil {
		t.Fatalf("unmarshal response payload: %v", err)
	}
	if got := response.Answers["tier"].Answers; len(got) != 1 || got[0] != "enterprise" {
		t.Fatalf("unexpected codex input response %#v", response)
	}
}

func TestResolveCodingSessionInteractionPreservesNativeCodexApprovalPayload(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	interactionRepo := repository.NewAgentRunInteractionRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Code Builder", model.AgentPresetCodeBuilder, "Builder", "idle", "codex",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)
	run := &model.AgentRun{
		ID:                "run-codex-approval",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_codex_approval"),
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		TargetType:        "story",
		TargetID:          "story-1",
		RuntimeKind:       "codex",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     "pending",
		PauseReason:       model.AgentRunPauseReasonHumanApproval,
		Status:            model.AgentRunStatusPaused,
		LastHeartbeatAt:   &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	seedCodexPendingSessionState(t, artifactRepo, run.WorkspaceID, run.ID, now)

	requestPayload := json.RawMessage(`{
		"threadId":"thread-1",
		"turnId":"turn-1",
		"itemId":"item-1",
		"approvalId":"approval-1",
		"command":"git commit",
		"cwd":"/workspace",
		"availableDecisions":["accept","acceptForSession","decline","cancel"]
	}`)
	if err := interactionRepo.Create(context.Background(), &model.AgentRunInteraction{
		ID:                         "interaction-approval-1",
		WorkspaceID:                run.WorkspaceID,
		RunID:                      run.ID,
		RuntimeKind:                "codex",
		InteractionKind:            model.AgentRunInteractionKindCommandExecutionApproval,
		Status:                     model.AgentRunInteractionStatusPending,
		RequestSchemaVersion:       model.AgentRunInteractionSchemaVersionCodexV2,
		RequestID:                  strPtr("7"),
		ThreadID:                   strPtr("thread-1"),
		TurnID:                     strPtr("turn-1"),
		ItemID:                     strPtr("item-1"),
		ApprovalID:                 strPtr("approval-1"),
		AssistantMessageSequenceNo: intPtr(8),
		Title:                      strPtr("Approve command execution"),
		RequestPayload:             requestPayload,
		RuntimeMetadata:            json.RawMessage(`{"runtime_kind":"codex"}`),
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}); err != nil {
		t.Fatalf("create interaction: %v", err)
	}

	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		artifactRepo:       artifactRepo,
		interactionRepo:    interactionRepo,
		agentRuntimeClient: runtimeClient,
	}

	responsePayload := json.RawMessage(`{"decision":"acceptForSession"}`)
	interaction, err := svc.ResolveCodingSessionInteraction(context.Background(), run.WorkspaceID, run.ID, "interaction-approval-1", "user-1", model.ResolveAgentRunInteractionRequest{
		ResponsePayload: responsePayload,
	})
	if err != nil {
		t.Fatalf("ResolveCodingSessionInteraction returned error: %v", err)
	}
	if interaction == nil || interaction.Status != model.AgentRunInteractionStatusResolved {
		t.Fatalf("expected resolved interaction, got %#v", interaction)
	}
	if interaction.ResponseSchemaVersion == nil || *interaction.ResponseSchemaVersion != model.AgentRunInteractionSchemaVersionCodexV2 {
		t.Fatalf("expected codex response schema version, got %#v", interaction.ResponseSchemaVersion)
	}
	if got := string(interaction.ResponsePayload); got != string(responsePayload) {
		t.Fatalf("expected native response payload to be preserved, got %s", got)
	}
	if len(runtimeClient.resumeCalls) != 1 {
		t.Fatalf("expected one agent runtime resume call, got %#v", runtimeClient.resumeCalls)
	}
	if got := string(runtimeClient.resumeCalls[0].req.ResponsePayload); got != string(responsePayload) {
		t.Fatalf("expected native response payload on runtime resume, got %s", got)
	}
	if interaction.ResolvedBy == nil || *interaction.ResolvedBy != "user-1" {
		t.Fatalf("expected resolved_by to be set, got %#v", interaction.ResolvedBy)
	}

	updatedRun, err := runRepo.GetByID(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if updatedRun.Status != model.AgentRunStatusPaused || updatedRun.PauseReason != model.AgentRunPauseReasonHumanApproval {
		t.Fatalf("expected codex run to remain paused until runtime projection resumes it, got %#v", updatedRun)
	}

	messages, err := runMessageRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("expected no follow-up messages for a straight approval, got %#v", messages)
	}
}

func TestResolveCodingSessionInteractionSignalsNativeCodexApprovalPayloadForStaleResume(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	interactionRepo := repository.NewAgentRunInteractionRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Code Builder", model.AgentPresetCodeBuilder, "Builder", "idle", "codex",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	staleHeartbeat := now.Add(-10 * time.Minute)
	run := &model.AgentRun{
		ID:                "run-codex-stale-resume",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_stale_resume"),
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		TargetType:        "story",
		TargetID:          "story-1",
		RuntimeKind:       "codex",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     "pending",
		PauseReason:       model.AgentRunPauseReasonHumanApproval,
		Status:            model.AgentRunStatusPaused,
		WorkflowID:        strPtr("workflow-run-codex-stale"),
		LastHeartbeatAt:   &staleHeartbeat,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	requestPayload := json.RawMessage(`{
		"threadId":"thread-1",
		"turnId":"turn-1",
		"itemId":"item-1",
		"reason":"Need broader access",
		"permissions":{"network":{"enabled":true}}
	}`)
	if err := interactionRepo.Create(context.Background(), &model.AgentRunInteraction{
		ID:                   "interaction-stale-codex-1",
		WorkspaceID:          run.WorkspaceID,
		RunID:                run.ID,
		RuntimeKind:          "codex",
		InteractionKind:      model.AgentRunInteractionKindPermissionsApproval,
		Status:               model.AgentRunInteractionStatusPending,
		RequestSchemaVersion: model.AgentRunInteractionSchemaVersionCodexV2,
		RequestID:            strPtr("7"),
		ThreadID:             strPtr("thread-1"),
		TurnID:               strPtr("turn-1"),
		ItemID:               strPtr("item-1"),
		Title:                strPtr("Approve additional permissions"),
		RequestPayload:       requestPayload,
		RuntimeMetadata:      json.RawMessage(`{"runtime_kind":"codex"}`),
		CreatedAt:            now,
		UpdatedAt:            now,
	}); err != nil {
		t.Fatalf("create interaction: %v", err)
	}

	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		interactionRepo:    interactionRepo,
		agentRuntimeClient: runtimeClient,
	}

	responsePayload := json.RawMessage(`{"permissions":{"network":{"enabled":true}},"scope":"session"}`)
	interaction, err := svc.ResolveCodingSessionInteraction(context.Background(), run.WorkspaceID, run.ID, "interaction-stale-codex-1", "user-1", model.ResolveAgentRunInteractionRequest{
		ResponsePayload: responsePayload,
	})
	if err != nil {
		t.Fatalf("ResolveCodingSessionInteraction returned error: %v", err)
	}
	if interaction == nil || interaction.Status != model.AgentRunInteractionStatusResolved {
		t.Fatalf("expected resolved interaction, got %#v", interaction)
	}
	if len(runtimeClient.resumeCalls) != 1 {
		t.Fatalf("expected one agent runtime resume call, got %#v", runtimeClient.resumeCalls)
	}
	resumeCall := runtimeClient.resumeCalls[0]
	if resumeCall.runID != "run_rt_stale_resume" {
		t.Fatalf("expected runtime run id run_rt_stale_resume, got %q", resumeCall.runID)
	}
	if resumeCall.req.Intent != model.AgentRunResumeIntentApprove {
		t.Fatalf("expected approve intent, got %#v", resumeCall.req)
	}
	if got := string(resumeCall.req.ResponsePayload); got != string(responsePayload) {
		t.Fatalf("expected exact native response payload on runtime resume, got %s", got)
	}
	if resumeCall.req.Content != "" {
		t.Fatalf("expected empty content for approve without message, got %#v", resumeCall.req)
	}
}

func TestResolveCodingSessionInteractionReviewCheckpointResumesEvenWhenLiveCodexPauseIsPresent(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	interactionRepo := repository.NewAgentRunInteractionRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Epic Planner", model.AgentPresetEpicPlanner, "Planner", "idle", "codex",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:                "run-codex-review-checkpoint",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_review_checkpoint"),
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		TargetType:        "epic",
		TargetID:          "epic-1",
		RuntimeKind:       "codex",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     "pending",
		PauseReason:       model.AgentRunPauseReasonHumanApproval,
		Status:            model.AgentRunStatusPaused,
		WorkflowID:        strPtr("workflow-run-codex-review"),
		LastHeartbeatAt:   &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	// Simulate an unrelated live codex pending-request snapshot still being present.
	seedCodexPendingSessionState(t, artifactRepo, run.WorkspaceID, run.ID, now)

	requestPayload := json.RawMessage(`{
		"phase":"prd",
		"title":"PRD Review: Increase Kafka throughput",
		"summary":"Review the current PRD draft.",
		"findings":[
			{"id":"finding_1","title":"Missing acceptance coverage","body":"The draft does not cover rollback behavior.","priority":"P1"}
		]
	}`)
	if err := interactionRepo.Create(context.Background(), &model.AgentRunInteraction{
		ID:                         "interaction-review-1",
		WorkspaceID:                run.WorkspaceID,
		RunID:                      run.ID,
		RuntimeKind:                "codex",
		InteractionKind:            model.AgentRunInteractionKindReviewCheckpoint,
		Status:                     model.AgentRunInteractionStatusPending,
		RequestSchemaVersion:       model.AgentRunInteractionSchemaVersionHelpinV1,
		AssistantMessageSequenceNo: intPtr(3),
		Title:                      strPtr("PRD Review"),
		Summary:                    strPtr("Review the current PRD draft."),
		RequestPayload:             requestPayload,
		RuntimeMetadata:            json.RawMessage(`{"runtime_kind":"codex"}`),
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}); err != nil {
		t.Fatalf("create interaction: %v", err)
	}

	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		artifactRepo:       artifactRepo,
		interactionRepo:    interactionRepo,
		agentRuntimeClient: runtimeClient,
	}

	responsePayload := json.RawMessage(`{"decision":"approve"}`)
	interaction, err := svc.ResolveCodingSessionInteraction(context.Background(), run.WorkspaceID, run.ID, "interaction-review-1", "user-1", model.ResolveAgentRunInteractionRequest{
		ResponsePayload: responsePayload,
	})
	if err != nil {
		t.Fatalf("ResolveCodingSessionInteraction returned error: %v", err)
	}
	if interaction == nil || interaction.Status != model.AgentRunInteractionStatusResolved {
		t.Fatalf("expected resolved interaction, got %#v", interaction)
	}
	if len(runtimeClient.resumeCalls) != 1 {
		t.Fatalf("expected review checkpoint to resume via the agent runtime even with a live codex pause, got %#v", runtimeClient.resumeCalls)
	}
	if runtimeClient.resumeCalls[0].req.Intent != model.AgentRunResumeIntentApprove {
		t.Fatalf("expected approve intent on runtime resume, got %#v", runtimeClient.resumeCalls[0].req)
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	foundDecisionArtifact := false
	for _, artifact := range artifacts {
		if artifact.ArtifactType != model.AgentRunArtifactTypeReviewDecision || artifact.InlineContent == nil {
			continue
		}
		foundDecisionArtifact = true
		if !strings.Contains(*artifact.InlineContent, `"decision":"approve"`) {
			t.Fatalf("expected persisted review decision artifact to capture approval, got %s", *artifact.InlineContent)
		}
	}
	if !foundDecisionArtifact {
		t.Fatal("expected resolved review checkpoint to persist a review decision artifact")
	}
}

func TestResolveCodingSessionInteractionReviewCheckpointPersistsDecisionForCleanReview(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	interactionRepo := repository.NewAgentRunInteractionRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Lens", model.AgentPresetReviewAgent, "Reviewer", "idle", "codex",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:                "run-codex-review-clean",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_review_clean"),
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		TargetType:        "epic",
		TargetID:          "epic-1",
		RuntimeKind:       "codex",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     "pending",
		PauseReason:       model.AgentRunPauseReasonHumanApproval,
		Status:            model.AgentRunStatusPaused,
		WorkflowID:        strPtr("workflow-run-codex-review-clean"),
		LastHeartbeatAt:   &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	requestPayload := json.RawMessage(`{
		"phase":"review_findings",
		"title":"Lens review findings",
		"summary":"No issues found in the reviewed diff.",
		"findings":[],
		"overall_correctness":"correct",
		"overall_explanation":"I did not find correctness issues in this pass."
	}`)
	if err := interactionRepo.Create(context.Background(), &model.AgentRunInteraction{
		ID:                         "interaction-review-clean-1",
		WorkspaceID:                run.WorkspaceID,
		RunID:                      run.ID,
		RuntimeKind:                "codex",
		InteractionKind:            model.AgentRunInteractionKindReviewCheckpoint,
		Status:                     model.AgentRunInteractionStatusPending,
		RequestSchemaVersion:       model.AgentRunInteractionSchemaVersionHelpinV1,
		AssistantMessageSequenceNo: intPtr(5),
		Title:                      strPtr("Lens review findings"),
		Summary:                    strPtr("No issues found in the reviewed diff."),
		RequestPayload:             requestPayload,
		RuntimeMetadata:            json.RawMessage(`{"runtime_kind":"codex"}`),
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}); err != nil {
		t.Fatalf("create interaction: %v", err)
	}

	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		artifactRepo:       artifactRepo,
		interactionRepo:    interactionRepo,
		agentRuntimeClient: &fakeAgentRuntimeSignalClient{},
	}

	responsePayload := json.RawMessage(`{"decision":"approve","message":"Looks good."}`)
	interaction, err := svc.ResolveCodingSessionInteraction(context.Background(), run.WorkspaceID, run.ID, "interaction-review-clean-1", "user-1", model.ResolveAgentRunInteractionRequest{
		ResponsePayload: responsePayload,
	})
	if err != nil {
		t.Fatalf("ResolveCodingSessionInteraction returned error: %v", err)
	}
	if interaction == nil || interaction.Status != model.AgentRunInteractionStatusResolved {
		t.Fatalf("expected resolved interaction, got %#v", interaction)
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}

	foundDecisionArtifact := false
	for _, artifact := range artifacts {
		if artifact.ArtifactType != model.AgentRunArtifactTypeReviewDecision || artifact.InlineContent == nil {
			continue
		}
		foundDecisionArtifact = true

		var payload model.ReviewDecisionArtifact
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &payload); err != nil {
			t.Fatalf("unmarshal decision artifact: %v", err)
		}
		if payload.Decision != "approve" {
			t.Fatalf("expected approve decision, got %#v", payload)
		}
		if len(payload.Findings) != 0 {
			t.Fatalf("expected clean review artifact to preserve zero findings, got %#v", payload)
		}
	}
	if !foundDecisionArtifact {
		t.Fatal("expected clean review resolution to persist a review decision artifact")
	}
}

func TestResolveCodingSessionInteractionReviewCheckpointPersistsOnlySelectedFindings(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	interactionRepo := repository.NewAgentRunInteractionRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Lens", model.AgentPresetReviewAgent, "Reviewer", "idle", "codex",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:                "run-codex-review-selected",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_review_selected"),
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		TargetType:        "epic",
		TargetID:          "epic-1",
		RuntimeKind:       "codex",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     "pending",
		PauseReason:       model.AgentRunPauseReasonHumanApproval,
		Status:            model.AgentRunStatusPaused,
		WorkflowID:        strPtr("workflow-run-codex-review-selected"),
		LastHeartbeatAt:   &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	requestPayload := json.RawMessage(`{
		"phase":"review_findings",
		"title":"Lens review findings",
		"summary":"Two findings need triage.",
		"findings":[
			{"id":"finding_1","title":"Regression A","body":"Breaks filter state.","priority":"P1"},
			{"id":"finding_2","title":"Regression B","body":"Drops sort order.","priority":"P2"}
		]
	}`)
	if err := interactionRepo.Create(context.Background(), &model.AgentRunInteraction{
		ID:                         "interaction-review-selected-1",
		WorkspaceID:                run.WorkspaceID,
		RunID:                      run.ID,
		RuntimeKind:                "codex",
		InteractionKind:            model.AgentRunInteractionKindReviewCheckpoint,
		Status:                     model.AgentRunInteractionStatusPending,
		RequestSchemaVersion:       model.AgentRunInteractionSchemaVersionHelpinV1,
		AssistantMessageSequenceNo: intPtr(7),
		Title:                      strPtr("Lens review findings"),
		Summary:                    strPtr("Two findings need triage."),
		RequestPayload:             requestPayload,
		RuntimeMetadata:            json.RawMessage(`{"runtime_kind":"codex"}`),
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}); err != nil {
		t.Fatalf("create interaction: %v", err)
	}

	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		artifactRepo:       artifactRepo,
		interactionRepo:    interactionRepo,
		agentRuntimeClient: &fakeAgentRuntimeSignalClient{},
	}

	responsePayload := json.RawMessage(`{
		"decision":"approve",
		"selection_mode":"selected",
		"selected_finding_ids":["finding_2"]
	}`)
	interaction, err := svc.ResolveCodingSessionInteraction(context.Background(), run.WorkspaceID, run.ID, "interaction-review-selected-1", "user-1", model.ResolveAgentRunInteractionRequest{
		ResponsePayload: responsePayload,
	})
	if err != nil {
		t.Fatalf("ResolveCodingSessionInteraction returned error: %v", err)
	}
	if interaction == nil || interaction.Status != model.AgentRunInteractionStatusResolved {
		t.Fatalf("expected resolved interaction, got %#v", interaction)
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}

	for _, artifact := range artifacts {
		if artifact.ArtifactType != model.AgentRunArtifactTypeReviewDecision || artifact.InlineContent == nil {
			continue
		}

		var payload model.ReviewDecisionArtifact
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &payload); err != nil {
			t.Fatalf("unmarshal decision artifact: %v", err)
		}
		if payload.SelectionMode != "selected" {
			t.Fatalf("expected selected scope, got %#v", payload)
		}
		if len(payload.Findings) != 1 || payload.Findings[0].ID != "finding_2" {
			t.Fatalf("expected only the selected finding to be persisted, got %#v", payload)
		}
		return
	}

	t.Fatal("expected selected review resolution to persist a review decision artifact")
}

func TestSendRunMessageTreatsLongApprovalPhraseAsNormalUserReply(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Epic Planner", model.AgentPresetEpicPlanner, "Planner", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:                "run-long-approve",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_long_approve"),
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		TargetType:        "epic",
		TargetID:          "epic-1",
		RuntimeKind:       "native_sdk",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     "not_required",
		PauseReason:       model.AgentRunPauseReasonHumanInput,
		Status:            model.AgentRunStatusPaused,
		OutputSummary:     []byte(`{"status":"waiting"}`),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := runMessageRepo.Create(context.Background(), &model.AgentRunMessage{
		WorkspaceID: "ws-1",
		RunID:       run.ID,
		Role:        "assistant",
		Content:     "Please review the latest PRD draft in the preview pane.",
		MessageType: "assistant_turn",
		ToolInvocations: mustMarshalTestJSON(t, []model.ToolInvocation{
			{
				ToolName: agentcontract.ToolPublishPreview,
				Input:    json.RawMessage(`{"panel_key":"prd_draft","title":"PRD Draft","format":"markdown","content":"# Problem\n\nDraft body"}`),
			},
			{
				ToolName: agentcontract.ToolRequestHumanApproval,
				Input:    json.RawMessage(`{"phase":"prd","title":"Approve PRD","summary":"Review the current draft"}`),
			},
		}),
		SequenceNo: 1,
	}); err != nil {
		t.Fatalf("create message: %v", err)
	}
	seedApprovalArtifacts(t, artifactRepo, "ws-1", run.ID, 1, now, "# Problem\n\nDraft body")

	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		artifactRepo:       artifactRepo,
		agentRuntimeClient: &fakeAgentRuntimeSignalClient{},
	}

	message, err := svc.SendRunMessage(context.Background(), "ws-1", run.ID, "user-1", model.SendAgentRunMessageRequest{
		Content: "approve and create the PRD doc",
	})
	if err != nil {
		t.Fatalf("SendRunMessage returned error: %v", err)
	}
	if message.MessageType != "user_reply" {
		t.Fatalf("expected user_reply message type, got %q", message.MessageType)
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	found := false
	for _, artifact := range artifacts {
		if artifact.ArtifactType == model.AgentRunArtifactTypeApprovedPreview {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected approved preview artifact, got %#v", artifacts)
	}
}

func TestSendRunMessageApprovalPrefersAssistantLinkedPreviewArtifact(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Epic Planner", model.AgentPresetEpicPlanner, "Planner", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:                "run-artifact-preferred",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_artifact_preferred"),
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		TargetType:        "epic",
		TargetID:          "epic-1",
		RuntimeKind:       "native_sdk",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     "not_required",
		PauseReason:       model.AgentRunPauseReasonHumanInput,
		Status:            model.AgentRunStatusPaused,
		OutputSummary:     []byte(`{"status":"waiting"}`),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := runMessageRepo.Create(context.Background(), &model.AgentRunMessage{
		WorkspaceID: "ws-1",
		RunID:       run.ID,
		Role:        "assistant",
		Content:     "Please approve the latest PRD draft.",
		MessageType: "assistant_turn",
		ToolInvocations: mustMarshalTestJSON(t, []model.ToolInvocation{
			{
				ToolName: agentcontract.ToolPublishPreview,
				Input:    json.RawMessage(`{"panel_key":"prd_draft","title":"PRD Draft","format":"markdown","content":"# Problem\n\nStale invocation draft"}`),
			},
			{
				ToolName: agentcontract.ToolRequestHumanApproval,
				Input:    json.RawMessage(`{"phase":"prd","title":"Approve PRD","summary":"Review the current draft"}`),
			},
		}),
		SequenceNo: 3,
	}); err != nil {
		t.Fatalf("create message: %v", err)
	}

	runPreviewContent := `{"panel_key":"prd_draft","title":"PRD Draft","format":"markdown","content":"# Problem\n\nArtifact-backed draft","replace":true}`
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            "artifact-run-preview-linked",
		WorkspaceID:   "ws-1",
		RunID:         run.ID,
		ArtifactType:  agentcontract.RunPreviewArtifactType,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &runPreviewContent,
		Metadata:      json.RawMessage(`{"assistant_message_sequence_no":3}`),
		SequenceNo:    1,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("create run preview artifact: %v", err)
	}
	approvalContent := `{"phase":"prd","title":"Approve PRD","summary":"Review the current draft"}`
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            "artifact-approval-linked-preferred",
		WorkspaceID:   "ws-1",
		RunID:         run.ID,
		ArtifactType:  model.AgentRunArtifactTypeHumanApprovalRequest,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &approvalContent,
		Metadata:      json.RawMessage(`{"assistant_message_sequence_no":3}`),
		SequenceNo:    2,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("create approval artifact: %v", err)
	}

	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		artifactRepo:       artifactRepo,
		agentRuntimeClient: &fakeAgentRuntimeSignalClient{},
	}

	if _, err := svc.SendRunMessage(context.Background(), "ws-1", run.ID, "user-1", model.SendAgentRunMessageRequest{
		Content: "approve",
	}); err != nil {
		t.Fatalf("SendRunMessage returned error: %v", err)
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	var approvedArtifact *model.AgentRunArtifact
	for i := range artifacts {
		if artifacts[i].ArtifactType == model.AgentRunArtifactTypeApprovedPreview {
			approvedArtifact = &artifacts[i]
			break
		}
	}
	if approvedArtifact == nil {
		t.Fatalf("expected approved preview artifact, got %#v", artifacts)
	}
	var approved model.ApprovedRunPreview
	if err := json.Unmarshal([]byte(derefString(approvedArtifact.InlineContent)), &approved); err != nil {
		t.Fatalf("unmarshal approved preview: %v", err)
	}
	var approvedContent string
	if err := json.Unmarshal(approved.Content, &approvedContent); err != nil {
		t.Fatalf("unmarshal approved preview content: %v", err)
	}
	if approvedContent != "# Problem\n\nArtifact-backed draft" {
		t.Fatalf("expected approved preview to use linked artifact content, got %q", approvedContent)
	}
}

func TestSendRunMessageApprovalUsesApprovalArtifactWithoutToolInvocations(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Epic Planner", model.AgentPresetEpicPlanner, "Planner", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:                "run-approval-artifact-only",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_approval_artifact_only"),
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		TargetType:        "epic",
		TargetID:          "epic-1",
		RuntimeKind:       "native_sdk",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     "not_required",
		PauseReason:       model.AgentRunPauseReasonHumanInput,
		Status:            model.AgentRunStatusPaused,
		OutputSummary:     []byte(`{"status":"waiting"}`),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := runMessageRepo.Create(context.Background(), &model.AgentRunMessage{
		WorkspaceID: "ws-1",
		RunID:       run.ID,
		Role:        "assistant",
		Content:     "Please approve the latest PRD draft.",
		MessageType: "assistant_turn",
		SequenceNo:  5,
	}); err != nil {
		t.Fatalf("create message: %v", err)
	}

	runPreviewContent := `{"panel_key":"prd_draft","title":"PRD Draft","format":"markdown","content":"# Problem\n\nArtifact-backed draft","replace":true}`
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            "artifact-run-preview-linked-2",
		WorkspaceID:   "ws-1",
		RunID:         run.ID,
		ArtifactType:  agentcontract.RunPreviewArtifactType,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &runPreviewContent,
		Metadata:      json.RawMessage(`{"assistant_message_sequence_no":5}`),
		SequenceNo:    1,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("create run preview artifact: %v", err)
	}

	approvalContent := `{"phase":"prd","title":"Approve PRD","summary":"Review the current draft"}`
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            "artifact-approval-linked",
		WorkspaceID:   "ws-1",
		RunID:         run.ID,
		ArtifactType:  model.AgentRunArtifactTypeHumanApprovalRequest,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &approvalContent,
		Metadata:      json.RawMessage(`{"assistant_message_sequence_no":5}`),
		SequenceNo:    2,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("create approval artifact: %v", err)
	}

	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		artifactRepo:       artifactRepo,
		agentRuntimeClient: &fakeAgentRuntimeSignalClient{},
	}

	if _, err := svc.SendRunMessage(context.Background(), "ws-1", run.ID, "user-1", model.SendAgentRunMessageRequest{
		Content: "approve",
	}); err != nil {
		t.Fatalf("SendRunMessage returned error: %v", err)
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	var approvedArtifact *model.AgentRunArtifact
	for i := range artifacts {
		if artifacts[i].ArtifactType == model.AgentRunArtifactTypeApprovedPreview {
			approvedArtifact = &artifacts[i]
			break
		}
	}
	if approvedArtifact == nil {
		t.Fatalf("expected approved preview artifact, got %#v", artifacts)
	}
}

func TestMaybePersistApprovedInteractivePreviewWithoutAssistantMessageRow(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Epic Planner", model.AgentPresetEpicPlanner, "Planner", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:             "run-approval-artifact-tool-only",
		WorkspaceID:    "ws-1",
		AgentID:        "agent-1",
		TargetType:     "epic",
		TargetID:       "epic-1",
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeInteractive,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonHumanInput,
		Status:         model.AgentRunStatusPaused,
		OutputSummary:  []byte(`{"status":"waiting"}`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	runPreviewContent := `{"panel_key":"prd_draft","title":"PRD Draft","format":"markdown","content":"# Problem\n\nTool-only draft","replace":true}`
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            "artifact-run-preview-tool-only",
		WorkspaceID:   "ws-1",
		RunID:         run.ID,
		ArtifactType:  agentcontract.RunPreviewArtifactType,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &runPreviewContent,
		Metadata:      json.RawMessage(`{"assistant_message_sequence_no":11}`),
		SequenceNo:    1,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("create run preview artifact: %v", err)
	}

	approvalContent := `{"phase":"prd","title":"Approve PRD","summary":"Review the current draft"}`
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            "artifact-approval-tool-only",
		WorkspaceID:   "ws-1",
		RunID:         run.ID,
		ArtifactType:  model.AgentRunArtifactTypeHumanApprovalRequest,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &approvalContent,
		Metadata:      json.RawMessage(`{"assistant_message_sequence_no":11}`),
		SequenceNo:    2,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("create approval artifact: %v", err)
	}

	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		artifactRepo:       artifactRepo,
		agentRuntimeClient: &fakeAgentRuntimeSignalClient{},
	}

	if err := svc.maybePersistApprovedInteractivePreview(context.Background(), run, "user-1", "approve"); err != nil {
		t.Fatalf("maybePersistApprovedInteractivePreview returned error: %v", err)
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	var approvedArtifact *model.AgentRunArtifact
	for i := range artifacts {
		if artifacts[i].ArtifactType == model.AgentRunArtifactTypeApprovedPreview {
			approvedArtifact = &artifacts[i]
			break
		}
	}
	if approvedArtifact == nil {
		t.Fatalf("expected approved preview artifact, got %#v", artifacts)
	}
	var approved model.ApprovedRunPreview
	if err := json.Unmarshal([]byte(derefString(approvedArtifact.InlineContent)), &approved); err != nil {
		t.Fatalf("unmarshal approved preview: %v", err)
	}
	if approved.SourceMessageID != "" {
		t.Fatalf("expected empty source_message_id for tool-only approval turn, got %q", approved.SourceMessageID)
	}
	if approved.AssistantMessageSequenceNo != 11 {
		t.Fatalf("expected assistant_message_sequence_no 11, got %d", approved.AssistantMessageSequenceNo)
	}
}

func TestSendRunMessageKeepsInteractiveRunResumingOnFeedback(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Epic Planner", model.AgentPresetEpicPlanner, "Planner", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:                "run-2",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_feedback"),
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		TargetType:        "epic",
		TargetID:          "epic-1",
		RuntimeKind:       "native_sdk",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     "not_required",
		PauseReason:       model.AgentRunPauseReasonHumanInput,
		Status:            model.AgentRunStatusPaused,
		OutputSummary:     []byte(`{"status":"waiting"}`),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := runMessageRepo.Create(context.Background(), &model.AgentRunMessage{
		WorkspaceID: "ws-1",
		RunID:       run.ID,
		Role:        "assistant",
		Content:     "Please review the latest PRD draft in the preview pane.",
		MessageType: "assistant_turn",
		ToolInvocations: mustMarshalTestJSON(t, []model.ToolInvocation{
			{
				ToolName: agentcontract.ToolPublishPreview,
				Input:    json.RawMessage(`{"panel_key":"prd_draft","title":"PRD Draft","format":"markdown","content":"# Problem\n\nDraft body"}`),
			},
			{
				ToolName: agentcontract.ToolRequestHumanApproval,
				Input:    json.RawMessage(`{"phase":"prd","title":"Approve PRD","summary":"Review the current draft"}`),
			},
		}),
		SequenceNo: 1,
	}); err != nil {
		t.Fatalf("create message: %v", err)
	}
	seedApprovalArtifacts(t, artifactRepo, "ws-1", run.ID, 1, now, "# Problem\n\nDraft body")

	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		artifactRepo:       artifactRepo,
		agentRuntimeClient: &fakeAgentRuntimeSignalClient{},
	}

	message, err := svc.SendRunMessage(context.Background(), "ws-1", run.ID, "user-1", model.SendAgentRunMessageRequest{
		Content: "add one more edge case before approval",
	})
	if err != nil {
		t.Fatalf("SendRunMessage returned error: %v", err)
	}
	if message.MessageType != "user_reply" {
		t.Fatalf("expected user_reply message type, got %q", message.MessageType)
	}

	updated, err := runRepo.GetByID(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if updated.ExecutionStage == nil || *updated.ExecutionStage != "resuming" {
		t.Fatalf("expected execution stage resuming, got %#v", updated.ExecutionStage)
	}
	if string(updated.OutputSummary) != `{"status":"waiting"}` {
		t.Fatalf("expected output_summary to remain unchanged, got %s", string(updated.OutputSummary))
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	for _, artifact := range artifacts {
		if artifact.ArtifactType == model.AgentRunArtifactTypeApprovedPreview {
			t.Fatalf("expected no approved preview artifacts for feedback, got %#v", artifacts)
		}
	}
}

func TestSendRunMessageAllowsAwaitingApprovalRuns(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Epic Planner", model.AgentPresetEpicPlanner, "Planner", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:                "run-awaiting-approval-message",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_awaiting_approval"),
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		TargetType:        "epic",
		TargetID:          "epic-1",
		RuntimeKind:       "native_sdk",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     "pending",
		PauseReason:       model.AgentRunPauseReasonHumanApproval,
		Status:            model.AgentRunStatusPaused,
		OutputSummary:     []byte(`{"status":"waiting"}`),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		artifactRepo:       artifactRepo,
		agentRuntimeClient: &fakeAgentRuntimeSignalClient{},
	}

	message, err := svc.SendRunMessage(context.Background(), "ws-1", run.ID, "user-1", model.SendAgentRunMessageRequest{
		Content: "Please revise the migration rollback section.",
	})
	if err != nil {
		t.Fatalf("SendRunMessage returned error: %v", err)
	}
	if message.MessageType != "user_reply" {
		t.Fatalf("expected user_reply message type, got %q", message.MessageType)
	}

	updated, err := runRepo.GetByID(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	// Status stays paused locally: the runtime event projection owns the
	// status/pause flip for delegated runs.
	if updated.Status != model.AgentRunStatusPaused {
		t.Fatalf("expected run status %q, got %q", model.AgentRunStatusPaused, updated.Status)
	}
	if updated.ApprovalState != "rejected" {
		t.Fatalf("expected approval_state rejected for feedback reply, got %q", updated.ApprovalState)
	}

	runtimeClient := svc.agentRuntimeClient.(*fakeAgentRuntimeSignalClient)
	if len(runtimeClient.resumeCalls) != 1 {
		t.Fatalf("expected one agent runtime resume call, got %#v", runtimeClient.resumeCalls)
	}
	if runtimeClient.resumeCalls[0].req.Intent != model.AgentRunResumeIntentRequestChanges {
		t.Fatalf("expected request_changes intent for feedback reply, got %#v", runtimeClient.resumeCalls[0].req)
	}
}

func TestResumeRunAllowsPausedApprovalRuns(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Epic Planner", model.AgentPresetEpicPlanner, "Planner", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:                "run-paused-approval",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_paused_approval"),
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		TargetType:        "epic",
		TargetID:          "epic-1",
		RuntimeKind:       "native_sdk",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     "pending",
		PauseReason:       model.AgentRunPauseReasonHumanApproval,
		Status:            model.AgentRunStatusPaused,
		OutputSummary:     []byte(`{"status":"waiting_approval"}`),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		agentRuntimeClient: &fakeAgentRuntimeSignalClient{},
	}

	updated, err := svc.ResumeRun(context.Background(), "ws-1", run.ID, "user-1", model.ResumeAgentRunRequest{
		Intent:  model.AgentRunResumeIntentRequestChanges,
		Content: "Please tighten the requirements section.",
	})
	if err != nil {
		t.Fatalf("ResumeRun returned error: %v", err)
	}
	// Status and pause reason stay untouched locally: the runtime event
	// projection owns the status/pause flip for delegated runs.
	if updated.Status != model.AgentRunStatusPaused {
		t.Fatalf("expected run status paused, got %q", updated.Status)
	}
	if updated.PauseReason != model.AgentRunPauseReasonHumanApproval {
		t.Fatalf("expected pause_reason human_approval until projection resumes, got %q", updated.PauseReason)
	}
	if updated.ApprovalState != "rejected" {
		t.Fatalf("expected approval_state rejected, got %q", updated.ApprovalState)
	}

	runtimeClient := svc.agentRuntimeClient.(*fakeAgentRuntimeSignalClient)
	if len(runtimeClient.resumeCalls) != 1 {
		t.Fatalf("expected one agent runtime resume call, got %#v", runtimeClient.resumeCalls)
	}
	resumeCall := runtimeClient.resumeCalls[0]
	if resumeCall.runID != "run_rt_paused_approval" {
		t.Fatalf("expected runtime run id run_rt_paused_approval, got %q", resumeCall.runID)
	}
	if resumeCall.req.Intent != model.AgentRunResumeIntentRequestChanges || resumeCall.req.Content != "Please tighten the requirements section." {
		t.Fatalf("expected request_changes resume request, got %#v", resumeCall.req)
	}
}

func TestGetAgentRunPreservesPausedApprovalState(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Epic Planner", model.AgentPresetEpicPlanner, "Planner", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:             "run-paused-approval-read",
		WorkspaceID:    "ws-1",
		AgentID:        "agent-1",
		TargetType:     "epic",
		TargetID:       "epic-1",
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeInteractive,
		ApprovalState:  "pending",
		PauseReason:    model.AgentRunPauseReasonHumanApproval,
		Status:         model.AgentRunStatusPaused,
		OutputSummary:  []byte(`{"status":"waiting_approval"}`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	svc := &AgentService{
		agentRepo: agentRepo,
		runRepo:   runRepo,
	}

	loaded, err := svc.GetAgentRun(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("GetAgentRun returned error: %v", err)
	}
	if loaded.Status != model.AgentRunStatusPaused {
		t.Fatalf("expected paused status, got %q", loaded.Status)
	}
	if loaded.PauseReason != model.AgentRunPauseReasonHumanApproval {
		t.Fatalf("expected human_approval pause_reason, got %q", loaded.PauseReason)
	}
}

func TestResumeRunApproveCreatesApprovalMessageAndApprovesRun(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Epic Planner", model.AgentPresetEpicPlanner, "Planner", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:                "run-resume-approve",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_resume_approve"),
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		TargetType:        "epic",
		TargetID:          "epic-1",
		RuntimeKind:       "native_sdk",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     "pending",
		PauseReason:       model.AgentRunPauseReasonHumanApproval,
		Status:            model.AgentRunStatusPaused,
		OutputSummary:     []byte(`{"status":"waiting_approval"}`),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		agentRuntimeClient: &fakeAgentRuntimeSignalClient{},
	}

	updated, err := svc.ResumeRun(context.Background(), "ws-1", run.ID, "user-1", model.ResumeAgentRunRequest{
		Intent:      model.AgentRunResumeIntentApprove,
		SendMessage: true,
	})
	if err != nil {
		t.Fatalf("ResumeRun returned error: %v", err)
	}
	if updated.ApprovalState != "approved" {
		t.Fatalf("expected approval_state approved, got %q", updated.ApprovalState)
	}
	// Status stays paused locally: the runtime event projection owns the
	// status/pause flip for delegated runs.
	if updated.Status != model.AgentRunStatusPaused {
		t.Fatalf("expected run status paused, got %q", updated.Status)
	}

	runtimeClient := svc.agentRuntimeClient.(*fakeAgentRuntimeSignalClient)
	if len(runtimeClient.resumeCalls) != 1 {
		t.Fatalf("expected one agent runtime resume call, got %#v", runtimeClient.resumeCalls)
	}
	resumeCall := runtimeClient.resumeCalls[0]
	if resumeCall.runID != "run_rt_resume_approve" {
		t.Fatalf("expected runtime run id run_rt_resume_approve, got %q", resumeCall.runID)
	}
	if resumeCall.req.Intent != model.AgentRunResumeIntentApprove || resumeCall.req.Content != "approve" {
		t.Fatalf("expected approve resume request with default content, got %#v", resumeCall.req)
	}

	messages, err := runMessageRepo.ListByRun(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 approval message, got %d", len(messages))
	}
	if messages[0].MessageType != "approval" || messages[0].Content != "approve" {
		t.Fatalf("unexpected approval message: %#v", messages[0])
	}
}

func TestSendRunMessagePersistsApprovedPreviewFromRunArtifactWhenToolsSplitAcrossMessages(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Epic Planner", model.AgentPresetEpicPlanner, "Planner", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:                "run-split-tools",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_split_tools"),
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		TargetType:        "epic",
		TargetID:          "epic-1",
		RuntimeKind:       "native_sdk",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     "not_required",
		PauseReason:       model.AgentRunPauseReasonHumanInput,
		Status:            model.AgentRunStatusPaused,
		OutputSummary:     []byte(`{"status":"waiting"}`),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	if err := runMessageRepo.Create(context.Background(), &model.AgentRunMessage{
		WorkspaceID: "ws-1",
		RunID:       run.ID,
		Role:        "assistant",
		Content:     "Preview published.",
		MessageType: "assistant_turn",
		ToolInvocations: mustMarshalTestJSON(t, []model.ToolInvocation{
			{
				ToolName: agentcontract.ToolPublishPreview,
				Input:    json.RawMessage(`{"panel_key":"prd_draft","title":"PRD Draft","format":"markdown","content":"# Problem\n\nDraft body"}`),
			},
		}),
		SequenceNo: 1,
	}); err != nil {
		t.Fatalf("create preview message: %v", err)
	}

	if err := runMessageRepo.Create(context.Background(), &model.AgentRunMessage{
		WorkspaceID: "ws-1",
		RunID:       run.ID,
		Role:        "assistant",
		Content:     "Please approve this PRD.",
		MessageType: "assistant_turn",
		ToolInvocations: mustMarshalTestJSON(t, []model.ToolInvocation{
			{
				ToolName: agentcontract.ToolRequestHumanApproval,
				Input:    json.RawMessage(`{"phase":"prd","title":"Approve PRD","summary":"Review draft"}`),
			},
		}),
		SequenceNo: 2,
	}); err != nil {
		t.Fatalf("create approval message: %v", err)
	}

	runPreviewContent := `{"panel_key":"prd_draft","title":"PRD Draft","format":"markdown","content":"# Problem\n\nDraft body","replace":true}`
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            "artifact-run-preview",
		WorkspaceID:   "ws-1",
		RunID:         run.ID,
		ArtifactType:  agentcontract.RunPreviewArtifactType,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &runPreviewContent,
		Metadata:      json.RawMessage(`{}`),
		SequenceNo:    1,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("create run preview artifact: %v", err)
	}
	approvalContent := `{"phase":"prd","title":"Approve PRD","summary":"Review draft"}`
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            "artifact-approval-split-tools",
		WorkspaceID:   "ws-1",
		RunID:         run.ID,
		ArtifactType:  model.AgentRunArtifactTypeHumanApprovalRequest,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &approvalContent,
		Metadata:      json.RawMessage(`{"assistant_message_sequence_no":2}`),
		SequenceNo:    2,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("create approval artifact: %v", err)
	}

	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		artifactRepo:       artifactRepo,
		agentRuntimeClient: &fakeAgentRuntimeSignalClient{},
	}

	if _, err := svc.SendRunMessage(context.Background(), "ws-1", run.ID, "user-1", model.SendAgentRunMessageRequest{
		Content: "I approve",
	}); err != nil {
		t.Fatalf("SendRunMessage returned error: %v", err)
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	var approvedArtifact *model.AgentRunArtifact
	for i := range artifacts {
		if artifacts[i].ArtifactType == model.AgentRunArtifactTypeApprovedPreview {
			approvedArtifact = &artifacts[i]
			break
		}
	}
	if approvedArtifact == nil {
		t.Fatalf("expected approved_preview artifact, got %#v", artifacts)
	}
	var approved model.ApprovedRunPreview
	if err := json.Unmarshal([]byte(derefString(approvedArtifact.InlineContent)), &approved); err != nil {
		t.Fatalf("unmarshal approved preview: %v", err)
	}
	if approved.PanelKey != "prd_draft" || approved.Format != agentcontract.PreviewFormatMarkdown {
		t.Fatalf("unexpected approved preview payload: %#v", approved)
	}
}

func TestSendRunMessagePersistsApprovedTaskDocPreviewWhenApprovalUsesPublishToolKey(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-task-doc", "ws-1", true, "Scribe", model.AgentPresetTaskPlanner, "Planner", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:                "run-task-doc-tool-key",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_task_doc_tool_key"),
		WorkspaceID:       "ws-1",
		AgentID:           "agent-task-doc",
		TargetType:        "task",
		TargetID:          "task-1",
		RuntimeKind:       "native_sdk",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     "pending",
		PauseReason:       model.AgentRunPauseReasonHumanApproval,
		Status:            model.AgentRunStatusPaused,
		OutputSummary:     []byte(`{"status":"waiting_approval"}`),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	assistantSequenceNo := 3
	if err := runMessageRepo.Create(context.Background(), &model.AgentRunMessage{
		WorkspaceID: "ws-1",
		RunID:       run.ID,
		Role:        "assistant",
		Content:     "Please approve the task planning document.",
		MessageType: "assistant_turn",
		ToolInvocations: mustMarshalTestJSON(t, []model.ToolInvocation{
			{
				ToolName: agentcontract.ToolPublishTaskPlanDoc,
				Input:    json.RawMessage(`{"content":"# Plan\n\nInstrument Kafka producer metrics."}`),
			},
			{
				ToolName: agentcontract.ToolRequestApproval,
				Input:    json.RawMessage(`{"phase":"task_doc","preview_panel_key":"publish_task_plan_doc","title":"Approve task planning document","summary":"Review it"}`),
			},
		}),
		SequenceNo: assistantSequenceNo,
	}); err != nil {
		t.Fatalf("create assistant message: %v", err)
	}

	runPreviewContent := `{"panel_key":"task_plan_doc","title":"Task Planning Document","format":"markdown","content":"# Plan\n\nInstrument Kafka producer metrics.","replace":true}`
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            "artifact-task-doc-preview",
		WorkspaceID:   "ws-1",
		RunID:         run.ID,
		ArtifactType:  agentcontract.RunPreviewArtifactType,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &runPreviewContent,
		Metadata:      json.RawMessage(`{"assistant_message_sequence_no":3}`),
		SequenceNo:    1,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("create run preview artifact: %v", err)
	}
	approvalContent := `{"phase":"task_doc","preview_panel_key":"publish_task_plan_doc","title":"Approve task planning document","summary":"Review it"}`
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            "artifact-task-doc-approval",
		WorkspaceID:   "ws-1",
		RunID:         run.ID,
		ArtifactType:  model.AgentRunArtifactTypeHumanApprovalRequest,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &approvalContent,
		Metadata:      json.RawMessage(`{"assistant_message_sequence_no":3}`),
		SequenceNo:    2,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("create approval artifact: %v", err)
	}

	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		artifactRepo:       artifactRepo,
		agentRuntimeClient: &fakeAgentRuntimeSignalClient{},
	}

	if _, err := svc.SendRunMessage(context.Background(), "ws-1", run.ID, "user-1", model.SendAgentRunMessageRequest{
		Content: "Approved task planning document. Persist it and finish.",
	}); err != nil {
		t.Fatalf("SendRunMessage returned error: %v", err)
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	var approvedArtifact *model.AgentRunArtifact
	for i := range artifacts {
		if artifacts[i].ArtifactType == model.AgentRunArtifactTypeApprovedPreview {
			approvedArtifact = &artifacts[i]
			break
		}
	}
	if approvedArtifact == nil {
		t.Fatalf("expected approved_preview artifact, got %#v", artifacts)
	}
	var approved model.ApprovedRunPreview
	if err := json.Unmarshal([]byte(derefString(approvedArtifact.InlineContent)), &approved); err != nil {
		t.Fatalf("unmarshal approved preview: %v", err)
	}
	if approved.Phase != "task_doc" || approved.PanelKey != "task_plan_doc" || approved.Format != agentcontract.PreviewFormatMarkdown {
		t.Fatalf("unexpected approved preview payload: %#v", approved)
	}
	if string(approved.Content) != `"# Plan\n\nInstrument Kafka producer metrics."` {
		t.Fatalf("unexpected approved preview content: %s", string(approved.Content))
	}
}

func TestSendRunMessagePersistsApprovedTaskDocPreviewWhenApprovalUsesUnknownPanelKeyWithSinglePreview(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-task-doc-uuid", "ws-1", true, "Scribe", model.AgentPresetTaskPlanner, "Planner", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:                "run-task-doc-uuid-key",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_task_doc_uuid_key"),
		WorkspaceID:       "ws-1",
		AgentID:           "agent-task-doc-uuid",
		TargetType:        "task",
		TargetID:          "task-1",
		RuntimeKind:       "native_sdk",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     "pending",
		PauseReason:       model.AgentRunPauseReasonHumanApproval,
		Status:            model.AgentRunStatusPaused,
		OutputSummary:     []byte(`{"status":"waiting_approval"}`),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	runPreviewContent := `{"panel_key":"task_plan_doc","title":"Task Planning Document","format":"markdown","content":"# Plan\n\nRegister histogram buckets.","replace":true}`
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            "artifact-task-doc-uuid-preview",
		WorkspaceID:   "ws-1",
		RunID:         run.ID,
		ArtifactType:  agentcontract.RunPreviewArtifactType,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &runPreviewContent,
		Metadata:      json.RawMessage(`{"assistant_message_sequence_no":3}`),
		SequenceNo:    1,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("create run preview artifact: %v", err)
	}
	approvalContent := `{"phase":"task_doc","preview_panel_key":"db1e88e9-2538-426a-a787-12a17f108bcd","title":"Approve task planning document","summary":"Review it"}`
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            "artifact-task-doc-uuid-approval",
		WorkspaceID:   "ws-1",
		RunID:         run.ID,
		ArtifactType:  model.AgentRunArtifactTypeHumanApprovalRequest,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &approvalContent,
		Metadata:      json.RawMessage(`{"assistant_message_sequence_no":3}`),
		SequenceNo:    2,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("create approval artifact: %v", err)
	}

	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		artifactRepo:       artifactRepo,
		agentRuntimeClient: &fakeAgentRuntimeSignalClient{},
	}

	if _, err := svc.SendRunMessage(context.Background(), "ws-1", run.ID, "user-1", model.SendAgentRunMessageRequest{
		Content: "Approved task planning document. Persist it and finish.",
	}); err != nil {
		t.Fatalf("SendRunMessage returned error: %v", err)
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	var approvedArtifact *model.AgentRunArtifact
	for i := range artifacts {
		if artifacts[i].ArtifactType == model.AgentRunArtifactTypeApprovedPreview {
			approvedArtifact = &artifacts[i]
			break
		}
	}
	if approvedArtifact == nil {
		t.Fatalf("expected approved_preview artifact, got %#v", artifacts)
	}
	var approved model.ApprovedRunPreview
	if err := json.Unmarshal([]byte(derefString(approvedArtifact.InlineContent)), &approved); err != nil {
		t.Fatalf("unmarshal approved preview: %v", err)
	}
	if approved.PanelKey != "task_plan_doc" || approved.Phase != "task_doc" {
		t.Fatalf("unexpected approved preview payload: %#v", approved)
	}
}

func TestApproveRunRecoversAwaitingApprovalWithStaleApprovalState(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Epic Planner", model.AgentPresetEpicPlanner, "Planner", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:                "run-awaiting-approve",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_awaiting_approve"),
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		TargetType:        "epic",
		TargetID:          "epic-1",
		RuntimeKind:       "native_sdk",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     "not_required",
		PauseReason:       model.AgentRunPauseReasonHumanApproval,
		Status:            model.AgentRunStatusPaused,
		OutputSummary:     []byte(`{"status":"waiting_approval"}`),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		agentRuntimeClient: &fakeAgentRuntimeSignalClient{},
	}

	updated, err := svc.ApproveRun(context.Background(), "ws-1", run.ID, "user-1", model.ApproveAgentRunRequest{})
	if err != nil {
		t.Fatalf("ApproveRun returned error: %v", err)
	}
	if updated.ApprovalState != "approved" {
		t.Fatalf("expected approval_state approved, got %q", updated.ApprovalState)
	}
	// Status stays paused locally: the runtime event projection owns the
	// status/pause flip for delegated runs.
	if updated.Status != model.AgentRunStatusPaused {
		t.Fatalf("expected run status paused, got %q", updated.Status)
	}
	if updated.ExecutionStage == nil || *updated.ExecutionStage != "approved" {
		t.Fatalf("expected execution stage approved, got %#v", updated.ExecutionStage)
	}

	runtimeClient := svc.agentRuntimeClient.(*fakeAgentRuntimeSignalClient)
	if len(runtimeClient.resumeCalls) != 1 {
		t.Fatalf("expected one agent runtime resume call, got %#v", runtimeClient.resumeCalls)
	}
	resumeCall := runtimeClient.resumeCalls[0]
	if resumeCall.req.Intent != model.AgentRunResumeIntentApprove || resumeCall.req.Content != "" {
		t.Fatalf("expected approve resume request with empty content, got %#v", resumeCall.req)
	}
}

func TestApproveRunKeepsPausedStateForLiveCodexSession(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-codex", "ws-1", false, "Forge", model.AgentPresetCodeBuilder, "Engineer", "idle", "codex",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:                "run-live-codex-approve",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_live_codex_approve"),
		WorkspaceID:       "ws-1",
		AgentID:           "agent-codex",
		TargetType:        "story",
		TargetID:          "story-1",
		RuntimeKind:       "codex",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     "pending",
		PauseReason:       model.AgentRunPauseReasonHumanApproval,
		Status:            model.AgentRunStatusPaused,
		LastHeartbeatAt:   &now,
		OutputSummary:     []byte(`{"status":"waiting_approval"}`),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	seedCodexPendingSessionState(t, artifactRepo, "ws-1", run.ID, now)

	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		artifactRepo:       artifactRepo,
		agentRuntimeClient: &fakeAgentRuntimeSignalClient{},
	}

	updated, err := svc.ApproveRun(context.Background(), "ws-1", run.ID, "user-1", model.ApproveAgentRunRequest{SendMessage: true})
	if err != nil {
		t.Fatalf("ApproveRun returned error: %v", err)
	}
	if updated.ApprovalState != "approved" {
		t.Fatalf("expected approval_state approved, got %q", updated.ApprovalState)
	}
	if updated.Status != model.AgentRunStatusPaused {
		t.Fatalf("expected live codex run to remain paused until the worker consumes approval, got %q", updated.Status)
	}
	if updated.PauseReason != model.AgentRunPauseReasonHumanApproval {
		t.Fatalf("expected pause reason human_approval, got %q", updated.PauseReason)
	}

	messages, err := runMessageRepo.ListByRun(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 1 || messages[0].MessageType != "approval" {
		t.Fatalf("expected one persisted approval message, got %#v", messages)
	}

	runtimeClient := svc.agentRuntimeClient.(*fakeAgentRuntimeSignalClient)
	if len(runtimeClient.resumeCalls) != 1 {
		t.Fatalf("expected one agent runtime resume call, got %#v", runtimeClient.resumeCalls)
	}
	if runtimeClient.resumeCalls[0].req.Intent != model.AgentRunResumeIntentApprove {
		t.Fatalf("expected approve intent, got %#v", runtimeClient.resumeCalls[0].req)
	}
}

func TestApproveRunPersistsProvidedApprovalContent(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Epic Planner", model.AgentPresetEpicPlanner, "Planner", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:                "run-approve-content",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_approve_content"),
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		TargetType:        "epic",
		TargetID:          "epic-1",
		RuntimeKind:       "native_sdk",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     "pending",
		PauseReason:       model.AgentRunPauseReasonHumanApproval,
		Status:            model.AgentRunStatusPaused,
		OutputSummary:     []byte(`{"status":"waiting_approval"}`),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		agentRuntimeClient: &fakeAgentRuntimeSignalClient{},
	}

	updated, err := svc.ApproveRun(context.Background(), "ws-1", run.ID, "user-1", model.ApproveAgentRunRequest{
		Content:     "PRD approved. Continue to tasks.",
		SendMessage: true,
	})
	if err != nil {
		t.Fatalf("ApproveRun returned error: %v", err)
	}
	if updated.ApprovalState != "approved" {
		t.Fatalf("expected approval_state approved, got %q", updated.ApprovalState)
	}

	messages, err := runMessageRepo.ListByRun(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 approval message, got %d", len(messages))
	}
	if messages[0].MessageType != "approval" || messages[0].Content != "PRD approved. Continue to tasks." {
		t.Fatalf("unexpected approval message: %#v", messages[0])
	}
}

func TestSendRunMessageApprovalNormalizesApprovedStoryPlanPreviewContent(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Epic Planner", model.AgentPresetEpicPlanner, "Planner", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:                "run-approve-story-plan",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_approve_story_plan"),
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		TargetType:        "epic",
		TargetID:          "epic-1",
		RuntimeKind:       "native_sdk",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     "pending",
		PauseReason:       model.AgentRunPauseReasonHumanApproval,
		Status:            model.AgentRunStatusPaused,
		OutputSummary:     []byte(`{"status":"waiting"}`),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := runMessageRepo.Create(context.Background(), &model.AgentRunMessage{
		WorkspaceID: "ws-1",
		RunID:       run.ID,
		Role:        "assistant",
		Content:     "Please review the latest task plan.",
		MessageType: "assistant_turn",
		ToolInvocations: mustMarshalTestJSON(t, []model.ToolInvocation{
			{
				ToolName: agentcontract.ToolRequestHumanApproval,
				Input:    json.RawMessage(`{"phase":"tasks","title":"Approve task plan","summary":"Review the current breakdown"}`),
			},
		}),
		SequenceNo: 2,
	}); err != nil {
		t.Fatalf("create approval message: %v", err)
	}

	runPreviewContent := "{\n" +
		"  \"panel_key\": \"task_plan\",\n" +
		"  \"title\": \"Task Plan\",\n" +
		"  \"format\": \"json\",\n" +
		"  \"content\": \"Here is the plan in the required format:\\n```json\\n{\\\"summary\\\":\\\"Breakdown\\\",\\\"proposed_tasks\\\":[{\\\"ref\\\":\\\"task_1\\\",\\\"name\\\":\\\"Task A\\\",\\\"description\\\":\\\"Do A\\\",\\\"task_type\\\":\\\"feature\\\",\\\"acceptance_criteria\\\":[\\\"works\\\"]}]}\\n```\",\n" +
		"  \"replace\": true\n" +
		"}"
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            "artifact-run-preview-story-plan",
		WorkspaceID:   "ws-1",
		RunID:         run.ID,
		ArtifactType:  agentcontract.RunPreviewArtifactType,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &runPreviewContent,
		Metadata:      json.RawMessage(`{}`),
		SequenceNo:    1,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("create run preview artifact: %v", err)
	}
	approvalContent := `{"phase":"tasks","title":"Approve task plan","summary":"Review the current breakdown"}`
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            "artifact-approval-story-plan",
		WorkspaceID:   "ws-1",
		RunID:         run.ID,
		ArtifactType:  model.AgentRunArtifactTypeHumanApprovalRequest,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &approvalContent,
		Metadata:      json.RawMessage(`{"assistant_message_sequence_no":2}`),
		SequenceNo:    2,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("create approval artifact: %v", err)
	}

	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		artifactRepo:       artifactRepo,
		agentRuntimeClient: &fakeAgentRuntimeSignalClient{},
	}

	if _, err := svc.SendRunMessage(context.Background(), "ws-1", run.ID, "user-1", model.SendAgentRunMessageRequest{
		Content: "I approve",
	}); err != nil {
		t.Fatalf("SendRunMessage returned error: %v", err)
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	var approvedArtifact *model.AgentRunArtifact
	for i := range artifacts {
		if artifacts[i].ArtifactType == model.AgentRunArtifactTypeApprovedPreview {
			approvedArtifact = &artifacts[i]
			break
		}
	}
	if approvedArtifact == nil {
		t.Fatalf("expected approved_preview artifact, got %#v", artifacts)
	}
	var approved model.ApprovedRunPreview
	if err := json.Unmarshal([]byte(derefString(approvedArtifact.InlineContent)), &approved); err != nil {
		t.Fatalf("unmarshal approved preview: %v", err)
	}
	var approvedContent map[string]any
	if err := json.Unmarshal(approved.Content, &approvedContent); err != nil {
		t.Fatalf("unmarshal approved preview content: %v", err)
	}
	if got, _ := approvedContent["summary"].(string); got != "Breakdown" {
		t.Fatalf("expected normalized summary, got %#v", approvedContent)
	}
	if _, ok := approvedContent["proposed_tasks"].([]any); !ok {
		t.Fatalf("expected normalized proposed_tasks array, got %#v", approvedContent)
	}
}

func TestRequestRunChangesRecoversAwaitingApprovalWithStaleApprovalState(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Epic Planner", model.AgentPresetEpicPlanner, "Planner", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:                "run-awaiting-changes",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_awaiting_changes"),
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		TargetType:        "epic",
		TargetID:          "epic-1",
		RuntimeKind:       "native_sdk",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     "not_required",
		PauseReason:       model.AgentRunPauseReasonHumanApproval,
		Status:            model.AgentRunStatusPaused,
		OutputSummary:     []byte(`{"status":"waiting_approval"}`),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		agentRuntimeClient: &fakeAgentRuntimeSignalClient{},
	}

	updated, err := svc.RequestRunChanges(context.Background(), "ws-1", run.ID, "user-1", model.SendAgentRunRequestChangesRequest{
		Content: "Please adjust acceptance criteria.",
	})
	if err != nil {
		t.Fatalf("RequestRunChanges returned error: %v", err)
	}
	if updated.ApprovalState != "rejected" {
		t.Fatalf("expected approval_state rejected, got %q", updated.ApprovalState)
	}
	// Status stays paused locally: the runtime event projection owns the
	// status/pause flip for delegated runs.
	if updated.Status != model.AgentRunStatusPaused {
		t.Fatalf("expected run status paused, got %q", updated.Status)
	}

	runtimeClient := svc.agentRuntimeClient.(*fakeAgentRuntimeSignalClient)
	if len(runtimeClient.resumeCalls) != 1 {
		t.Fatalf("expected one agent runtime resume call, got %#v", runtimeClient.resumeCalls)
	}
	if runtimeClient.resumeCalls[0].req.Intent != model.AgentRunResumeIntentRequestChanges {
		t.Fatalf("expected request_changes intent, got %#v", runtimeClient.resumeCalls[0].req)
	}
}

func TestRequestRunChangesKeepsPausedStateForLiveCodexSession(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)

	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-codex", "ws-1", false, "Forge", model.AgentPresetCodeBuilder, "Engineer", "idle", "codex",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	run := &model.AgentRun{
		ID:                "run-live-codex-feedback",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_live_codex_feedback"),
		WorkspaceID:       "ws-1",
		AgentID:           "agent-codex",
		TargetType:        "story",
		TargetID:          "story-1",
		RuntimeKind:       "codex",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     "pending",
		PauseReason:       model.AgentRunPauseReasonHumanApproval,
		Status:            model.AgentRunStatusPaused,
		LastHeartbeatAt:   &now,
		OutputSummary:     []byte(`{"status":"waiting_approval"}`),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	seedCodexPendingSessionState(t, artifactRepo, "ws-1", run.ID, now)

	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		artifactRepo:       artifactRepo,
		agentRuntimeClient: &fakeAgentRuntimeSignalClient{},
	}

	updated, err := svc.RequestRunChanges(context.Background(), "ws-1", run.ID, "user-1", model.SendAgentRunRequestChangesRequest{
		Content: "Please split the helper from the middleware.",
	})
	if err != nil {
		t.Fatalf("RequestRunChanges returned error: %v", err)
	}
	if updated.ApprovalState != "rejected" {
		t.Fatalf("expected approval_state rejected, got %q", updated.ApprovalState)
	}
	if updated.Status != model.AgentRunStatusPaused {
		t.Fatalf("expected live codex run to remain paused until the worker consumes feedback, got %q", updated.Status)
	}
	if updated.PauseReason != model.AgentRunPauseReasonHumanApproval {
		t.Fatalf("expected pause reason human_approval, got %q", updated.PauseReason)
	}

	runtimeClient := svc.agentRuntimeClient.(*fakeAgentRuntimeSignalClient)
	if len(runtimeClient.resumeCalls) != 1 {
		t.Fatalf("expected one agent runtime resume call, got %#v", runtimeClient.resumeCalls)
	}
	if runtimeClient.resumeCalls[0].req.Intent != model.AgentRunResumeIntentRequestChanges {
		t.Fatalf("expected request_changes intent, got %#v", runtimeClient.resumeCalls[0].req)
	}
}

func newInteractiveApprovalTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:interactive-approval-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	registerAgentTestUUIDCallback(t, db)

	statements := []string{
		`CREATE TABLE agents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			icon_key TEXT NOT NULL DEFAULT '',
			preset_key TEXT,
			preset_version_key TEXT,
			source_preset_key TEXT,
			source_preset_version_key TEXT,
			source_template_id TEXT,
			source_template_key TEXT NOT NULL DEFAULT '',
			template_key TEXT,
			template_instance_id TEXT,
			template_version INTEGER,
			active_version_id TEXT,
			role TEXT,
			status TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			skills BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			trigger_mode TEXT NOT NULL,
			provider TEXT,
			model TEXT,
			execution_config BLOB NOT NULL DEFAULT x'7b7d',
			system_prompt TEXT,
			instruction_template_version TEXT NOT NULL DEFAULT '',
			planning_notes TEXT,
			tools BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			monthly_token_budget INTEGER,
			tokens_used_this_month INTEGER NOT NULL DEFAULT 0,
			active_task_id TEXT,
			team_id TEXT,
			allowed_tools BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			allowed_commands BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			allowed_targets BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			schedule TEXT,
			target_selector TEXT,
			trigger_events BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			approval_mode TEXT NOT NULL DEFAULT 'never',
			max_concurrent_runs INTEGER NOT NULL DEFAULT 1,
			default_invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_runs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			task_id TEXT,
			conversation_id TEXT,
			target_type TEXT NOT NULL,
			target_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			invocation_mode TEXT NOT NULL,
			parent_run_id TEXT,
			dock_chat_id TEXT,
			handoff_state TEXT,
			approval_state TEXT NOT NULL,
			pause_reason TEXT NOT NULL DEFAULT 'none',
			triggered_by_user_id TEXT,
			status TEXT NOT NULL,
			workflow_id TEXT,
			workflow_run_id TEXT,
			external_runtime TEXT,
			external_runtime_id TEXT,
			task_queue TEXT,
			runner_pool TEXT,
			agent_version_id TEXT,
			repository_id TEXT,
			repo_full_name TEXT,
			base_branch TEXT,
			working_branch TEXT,
			delivery_target_id TEXT,
			execution_stage TEXT,
			last_heartbeat_at DATETIME,
			input BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			output_summary BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			cached_input_tokens INTEGER NOT NULL DEFAULT 0,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			tokens_used INTEGER NOT NULL DEFAULT 0,
			error_message TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_run_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			runtime_message_id TEXT,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			message_type TEXT NOT NULL,
			content_blocks BLOB,
			turn_segments BLOB,
			tool_invocations BLOB,
			token_usage BLOB,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE agent_run_interactions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			interaction_kind TEXT NOT NULL,
			status TEXT NOT NULL,
			request_schema_version TEXT NOT NULL,
			response_schema_version TEXT,
			request_id TEXT,
			thread_id TEXT,
			turn_id TEXT,
			item_id TEXT,
			approval_id TEXT,
			assistant_message_sequence_no INTEGER,
			title TEXT,
			summary TEXT,
			request_payload BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			response_payload BLOB,
			runtime_metadata BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			resolved_by TEXT,
			resolved_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE coding_session_state_snapshots (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			schema_version TEXT NOT NULL,
			through_sequence INTEGER NOT NULL DEFAULT 0,
			snapshot_payload BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE(workspace_id, run_id)
		)`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("exec schema: %v", err)
		}
	}

	return db
}

func mustMarshalTestJSON(t *testing.T, value any) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return payload
}
