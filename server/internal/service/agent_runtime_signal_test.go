package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
)

type fakeAgentRuntimeSignalClient struct {
	resumeCalls []fakeAgentRuntimeResumeCall
	cancelCalls []string
	resumeErr   error
	cancelErr   error
}

type fakeAgentRuntimeResumeCall struct {
	runID string
	req   AgentRuntimeResumeRunRequest
}

func (c *fakeAgentRuntimeSignalClient) ResumeRun(_ context.Context, runtimeRunID string, req AgentRuntimeResumeRunRequest) (*AgentRuntimeRun, error) {
	c.resumeCalls = append(c.resumeCalls, fakeAgentRuntimeResumeCall{runID: runtimeRunID, req: req})
	if c.resumeErr != nil {
		return nil, c.resumeErr
	}
	return &AgentRuntimeRun{ID: runtimeRunID, Status: model.AgentRunStatusRunning}, nil
}

func (c *fakeAgentRuntimeSignalClient) CancelRun(_ context.Context, runtimeRunID string) (*AgentRuntimeRun, error) {
	c.cancelCalls = append(c.cancelCalls, runtimeRunID)
	if c.cancelErr != nil {
		return nil, c.cancelErr
	}
	return &AgentRuntimeRun{ID: runtimeRunID, Status: model.AgentRunStatusCancelled}, nil
}

func TestResumeRunForAgentRuntimeRunSignalsRuntimeAndKeepsLocalSideEffects(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	now := time.Now().UTC()
	seedAgentRuntimeSignalAgent(t, db, now)
	run := seedAgentRuntimeSignalRun(t, runRepo, model.AgentRunStatusPaused, model.AgentRunPauseReasonHumanInput, "not_required", now)
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		runEngine:          &temporalapp.RunEngine{},
		agentRuntimeClient: runtimeClient,
	}

	updated, err := svc.ResumeRun(context.Background(), "ws-1", run.ID, "user-1", model.ResumeAgentRunRequest{
		Intent:          model.AgentRunResumeIntentReply,
		Content:         "continue with the smaller scope",
		ResponsePayload: json.RawMessage(`{"answer":"ok"}`),
	})
	if err != nil {
		t.Fatalf("ResumeRun returned error: %v", err)
	}
	if len(runtimeClient.resumeCalls) != 1 {
		t.Fatalf("expected one runtime resume call, got %d", len(runtimeClient.resumeCalls))
	}
	call := runtimeClient.resumeCalls[0]
	if call.runID != "run_runtime_1" {
		t.Fatalf("expected runtime run id, got %q", call.runID)
	}
	if call.req.Intent != model.AgentRunResumeIntentReply || call.req.Content != "continue with the smaller scope" || call.req.ExternalActorID != "user-1" {
		t.Fatalf("unexpected runtime resume request: %#v", call.req)
	}
	if string(call.req.ResponsePayload) != `{"answer":"ok"}` {
		t.Fatalf("expected response payload to forward, got %s", string(call.req.ResponsePayload))
	}
	if updated.Status != model.AgentRunStatusPaused || updated.PauseReason != model.AgentRunPauseReasonHumanInput {
		t.Fatalf("expected projection-owned status to remain paused/human_input, got %s/%s", updated.Status, updated.PauseReason)
	}
	messages, err := runMessageRepo.ListByRun(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 1 || messages[0].Role != "user" || messages[0].Content != "continue with the smaller scope" {
		t.Fatalf("expected local user message, got %#v", messages)
	}
}

func TestResumeRunForAgentRuntimeRunRollsBackLocalStateWhenRuntimeSignalFails(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	now := time.Now().UTC()
	seedAgentRuntimeSignalAgent(t, db, now)
	run := seedAgentRuntimeSignalRun(t, runRepo, model.AgentRunStatusPaused, model.AgentRunPauseReasonHumanInput, "not_required", now)
	runtimeClient := &fakeAgentRuntimeSignalClient{resumeErr: errors.New("runtime down")}
	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		runEngine:          &temporalapp.RunEngine{},
		agentRuntimeClient: runtimeClient,
	}

	_, err := svc.ResumeRun(context.Background(), "ws-1", run.ID, "user-1", model.ResumeAgentRunRequest{
		Intent:  model.AgentRunResumeIntentReply,
		Content: "continue",
	})
	if err == nil {
		t.Fatal("expected resume error")
	}
	reloaded, err := runRepo.GetByID(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("reload run: %v", err)
	}
	if reloaded.Status != model.AgentRunStatusPaused || reloaded.PauseReason != model.AgentRunPauseReasonHumanInput {
		t.Fatalf("expected local run rollback to paused/human_input, got %s/%s", reloaded.Status, reloaded.PauseReason)
	}
	messages, err := runMessageRepo.ListByRun(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("expected no local message after failed runtime signal, got %#v", messages)
	}
}

func TestApproveRunForAgentRuntimeRunWithoutMessageDoesNotSendSyntheticContent(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	now := time.Now().UTC()
	seedAgentRuntimeSignalAgent(t, db, now)
	run := seedAgentRuntimeSignalRun(t, runRepo, model.AgentRunStatusPaused, model.AgentRunPauseReasonHumanApproval, "pending", now)
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     runMessageRepo,
		runEngine:          &temporalapp.RunEngine{},
		agentRuntimeClient: runtimeClient,
	}

	_, err := svc.ApproveRun(context.Background(), "ws-1", run.ID, "user-1", model.ApproveAgentRunRequest{})
	if err != nil {
		t.Fatalf("ApproveRun returned error: %v", err)
	}
	if len(runtimeClient.resumeCalls) != 1 {
		t.Fatalf("expected one runtime resume call, got %d", len(runtimeClient.resumeCalls))
	}
	call := runtimeClient.resumeCalls[0]
	if call.req.Intent != model.AgentRunResumeIntentApprove {
		t.Fatalf("expected approve intent, got %#v", call.req)
	}
	if call.req.Content != "" {
		t.Fatalf("expected no synthetic runtime approval content, got %q", call.req.Content)
	}
	messages, err := runMessageRepo.ListByRun(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("expected no local approval message, got %#v", messages)
	}
}

func TestCancelRunForAgentRuntimeRunSignalsRuntimeBeforeLocalCancel(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	now := time.Now().UTC()
	seedAgentRuntimeSignalAgent(t, db, now)
	run := seedAgentRuntimeSignalRun(t, runRepo, model.AgentRunStatusRunning, model.AgentRunPauseReasonNone, "not_required", now)
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runEngine:          &temporalapp.RunEngine{},
		agentRuntimeClient: runtimeClient,
	}

	updated, err := svc.CancelRun(context.Background(), "ws-1", run.ID, "user-1")
	if err != nil {
		t.Fatalf("CancelRun returned error: %v", err)
	}
	if len(runtimeClient.cancelCalls) != 1 || runtimeClient.cancelCalls[0] != "run_runtime_1" {
		t.Fatalf("expected runtime cancel call, got %#v", runtimeClient.cancelCalls)
	}
	if updated.Status != model.AgentRunStatusRunning || updated.CompletedAt != nil {
		t.Fatalf("expected projection-owned status to remain running until event projection, got status=%s completed_at=%v", updated.Status, updated.CompletedAt)
	}
}

func seedAgentRuntimeSignalAgent(t *testing.T, db *gorm.DB, now time.Time) {
	t.Helper()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", false, "Delegated Agent", "Executor", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)
}

func seedAgentRuntimeSignalRun(t *testing.T, runRepo *repository.AgentRunRepository, status, pauseReason, approvalState string, now time.Time) *model.AgentRun {
	t.Helper()
	run := &model.AgentRun{
		ID:                "run-1",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		TargetType:        "workspace",
		TargetID:          "ws-1",
		RuntimeKind:       "native_sdk",
		InvocationMode:    model.InvocationModeInteractive,
		ApprovalState:     approvalState,
		PauseReason:       pauseReason,
		Status:            status,
		ExternalRuntime:   strPtr(agentRuntimeName),
		ExternalRuntimeID: strPtr("run_runtime_1"),
		OutputSummary:     []byte(`{}`),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	return run
}
