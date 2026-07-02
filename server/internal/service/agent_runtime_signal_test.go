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
	resumeCalls          []fakeAgentRuntimeResumeCall
	cancelCalls          []string
	startAuthCalls       []string
	cancelAuthCalls      []string
	upsertAgents         []AgentRuntimeAgent
	startRunCalls        []AgentRuntimeStartRunRequest
	appID                string
	getCalls             []string
	listMessageCalls     []string
	listArtifactCalls    []string
	listInteractionCalls []string
	getRuns              map[string]*AgentRuntimeRun
	messages             map[string][]AgentRuntimeMessage
	artifacts            map[string][]AgentRuntimeArtifact
	interactions         map[string][]AgentRuntimeInteraction
	getErr               error
	listErr              error
	upsertErr            error
	startRunErr          error
	resumeErr            error
	cancelErr            error
	startAuthState       *model.CodexAuthState
	cancelAuthState      *model.CodexAuthState
	startAuthErr         error
	cancelAuthErr        error
}

type fakeAgentRuntimeResumeCall struct {
	runID string
	req   AgentRuntimeResumeRunRequest
}

func (c *fakeAgentRuntimeSignalClient) AppID() string {
	if c.appID == "" {
		return "helpin"
	}
	return c.appID
}

func (c *fakeAgentRuntimeSignalClient) UpsertAgent(_ context.Context, agent AgentRuntimeAgent) (*AgentRuntimeAgent, error) {
	c.upsertAgents = append(c.upsertAgents, agent)
	if c.upsertErr != nil {
		return nil, c.upsertErr
	}
	return &agent, nil
}

func (c *fakeAgentRuntimeSignalClient) StartRun(_ context.Context, req AgentRuntimeStartRunRequest) (*AgentRuntimeRun, error) {
	c.startRunCalls = append(c.startRunCalls, req)
	if c.startRunErr != nil {
		return nil, c.startRunErr
	}
	return &AgentRuntimeRun{ID: "run_runtime_1", HostRunID: req.HostRunID, AgentID: req.AgentID, Status: model.AgentRunStatusQueued}, nil
}

func (c *fakeAgentRuntimeSignalClient) GetRun(_ context.Context, runtimeRunID string) (*AgentRuntimeRun, error) {
	c.getCalls = append(c.getCalls, runtimeRunID)
	if c.getErr != nil {
		return nil, c.getErr
	}
	if c.getRuns == nil {
		return nil, nil
	}
	return c.getRuns[runtimeRunID], nil
}

func (c *fakeAgentRuntimeSignalClient) ListMessages(_ context.Context, runtimeRunID string) ([]AgentRuntimeMessage, error) {
	c.listMessageCalls = append(c.listMessageCalls, runtimeRunID)
	if c.listErr != nil {
		return nil, c.listErr
	}
	return append([]AgentRuntimeMessage(nil), c.messages[runtimeRunID]...), nil
}

func (c *fakeAgentRuntimeSignalClient) ListArtifacts(_ context.Context, runtimeRunID string) ([]AgentRuntimeArtifact, error) {
	c.listArtifactCalls = append(c.listArtifactCalls, runtimeRunID)
	if c.listErr != nil {
		return nil, c.listErr
	}
	return append([]AgentRuntimeArtifact(nil), c.artifacts[runtimeRunID]...), nil
}

func (c *fakeAgentRuntimeSignalClient) ListInteractions(_ context.Context, runtimeRunID string) ([]AgentRuntimeInteraction, error) {
	c.listInteractionCalls = append(c.listInteractionCalls, runtimeRunID)
	if c.listErr != nil {
		return nil, c.listErr
	}
	return append([]AgentRuntimeInteraction(nil), c.interactions[runtimeRunID]...), nil
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

func (c *fakeAgentRuntimeSignalClient) StartCodexDeviceCodeAuth(_ context.Context, runtimeRunID string) (*model.CodexAuthState, error) {
	c.startAuthCalls = append(c.startAuthCalls, runtimeRunID)
	if c.startAuthErr != nil {
		return nil, c.startAuthErr
	}
	if c.startAuthState != nil {
		return c.startAuthState, nil
	}
	return &model.CodexAuthState{State: model.CodexAuthStatePending, Provider: "openai", AuthMode: "chatgpt_device_code"}, nil
}

func (c *fakeAgentRuntimeSignalClient) CancelCodexDeviceCodeAuth(_ context.Context, runtimeRunID string) (*model.CodexAuthState, error) {
	c.cancelAuthCalls = append(c.cancelAuthCalls, runtimeRunID)
	if c.cancelAuthErr != nil {
		return nil, c.cancelAuthErr
	}
	if c.cancelAuthState != nil {
		return c.cancelAuthState, nil
	}
	return &model.CodexAuthState{State: model.CodexAuthStateCancelled, Provider: "openai", AuthMode: "chatgpt_device_code"}, nil
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

func TestReplyToApprovalPausedAgentRuntimeRunForwardsRequestChanges(t *testing.T) {
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

	updated, err := svc.ResumeRun(context.Background(), "ws-1", run.ID, "user-1", model.ResumeAgentRunRequest{
		Intent:  model.AgentRunResumeIntentReply,
		Content: "please change the title before approval",
	})
	if err != nil {
		t.Fatalf("ResumeRun returned error: %v", err)
	}
	if len(runtimeClient.resumeCalls) != 1 {
		t.Fatalf("expected one runtime resume call, got %d", len(runtimeClient.resumeCalls))
	}
	call := runtimeClient.resumeCalls[0]
	if call.req.Intent != model.AgentRunResumeIntentRequestChanges {
		t.Fatalf("expected request_changes runtime intent, got %#v", call.req)
	}
	if call.req.Content != "please change the title before approval" {
		t.Fatalf("expected feedback content to forward, got %q", call.req.Content)
	}
	if updated.ApprovalState != "rejected" {
		t.Fatalf("expected local approval state rejected, got %q", updated.ApprovalState)
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

func TestDelegatedCodexAuthConnectedResumesRuntimeWithoutLocalStatusClobber(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	now := time.Now().UTC()
	seedAgentRuntimeSignalAgent(t, db, now)
	run := seedAgentRuntimeSignalRun(t, runRepo, model.AgentRunStatusPaused, model.AgentRunPauseReasonAuthentication, "not_required", now)
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := &AgentService{
		runRepo:            runRepo,
		agentRuntimeClient: runtimeClient,
	}

	err := svc.applyCodexAuthState(context.Background(), "ws-1", run.ID, "user-1", &model.CodexAuthState{
		State:     model.CodexAuthStateConnected,
		Provider:  "openai",
		AuthMode:  "chatgpt_device_code",
		UpdatedAt: now,
	}, true)
	if err != nil {
		t.Fatalf("applyCodexAuthState returned error: %v", err)
	}
	if len(runtimeClient.resumeCalls) != 1 {
		t.Fatalf("expected one runtime resume call, got %d", len(runtimeClient.resumeCalls))
	}
	call := runtimeClient.resumeCalls[0]
	if call.runID != "run_runtime_1" {
		t.Fatalf("expected runtime run id, got %q", call.runID)
	}
	if call.req.Intent != model.AgentRunResumeIntentAuthCompleted || call.req.ExternalActorID != "user-1" {
		t.Fatalf("unexpected runtime auth resume request: %#v", call.req)
	}
	reloaded, err := runRepo.GetByID(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("reload run: %v", err)
	}
	if reloaded.Status != model.AgentRunStatusPaused || reloaded.PauseReason != model.AgentRunPauseReasonAuthentication {
		t.Fatalf("expected projection-owned status to remain paused/authentication, got %s/%s", reloaded.Status, reloaded.PauseReason)
	}
	if reloaded.ExecutionStage == nil || *reloaded.ExecutionStage != "auth_completed" {
		t.Fatalf("expected auth_completed stage, got %#v", reloaded.ExecutionStage)
	}
}

func TestStartCodexDeviceCodeAuthForAgentRuntimeRunUsesRuntimeAuthManager(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	now := time.Now().UTC()
	seedAgentRuntimeSignalAgent(t, db, now)
	run := seedAgentRuntimeSignalRun(t, runRepo, model.AgentRunStatusPaused, model.AgentRunPauseReasonAuthentication, "not_required", now)
	mustExec(t, db, `UPDATE agents SET runtime_kind = ?, provider = ? WHERE id = ?`, "codex", "openai", "agent-1")
	mustExec(t, db, `UPDATE agent_runs SET runtime_kind = ? WHERE id = ?`, "codex", run.ID)
	runtimeClient := &fakeAgentRuntimeSignalClient{startAuthState: &model.CodexAuthState{
		State:     model.CodexAuthStateConnected,
		Provider:  "openai",
		AuthMode:  "chatgpt_device_code",
		PlanType:  strPtr("pro"),
		UpdatedAt: now,
	}}
	svc := &AgentService{
		agentRepo:                agentRepo,
		runRepo:                  runRepo,
		agentRuntimeClient:       runtimeClient,
		codexOpenAIAuthMode:      "chatgpt_device_code",
		codexChatGPTOAuthEnabled: true,
	}

	state, err := svc.StartCodexDeviceCodeAuth(context.Background(), "ws-1", run.ID, "user-1")
	if err != nil {
		t.Fatalf("StartCodexDeviceCodeAuth returned error: %v", err)
	}
	if state == nil || state.State != model.CodexAuthStateConnected {
		t.Fatalf("expected connected auth state, got %#v", state)
	}
	if len(runtimeClient.startAuthCalls) != 1 || runtimeClient.startAuthCalls[0] != "run_runtime_1" {
		t.Fatalf("expected runtime auth start call, got %#v", runtimeClient.startAuthCalls)
	}
	if len(runtimeClient.resumeCalls) != 1 || runtimeClient.resumeCalls[0].req.Intent != model.AgentRunResumeIntentAuthCompleted {
		t.Fatalf("expected runtime auth_completed resume, got %#v", runtimeClient.resumeCalls)
	}
	reloaded, err := runRepo.GetByID(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("reload run: %v", err)
	}
	if reloaded.Status != model.AgentRunStatusPaused || reloaded.PauseReason != model.AgentRunPauseReasonAuthentication {
		t.Fatalf("expected projection-owned status to remain paused/authentication, got %s/%s", reloaded.Status, reloaded.PauseReason)
	}
	if reloaded.ExecutionStage == nil || *reloaded.ExecutionStage != "auth_completed" {
		t.Fatalf("expected auth_completed stage, got %#v", reloaded.ExecutionStage)
	}
}

func TestCancelCodexDeviceCodeAuthForAgentRuntimeRunUsesRuntimeAuthManager(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	now := time.Now().UTC()
	seedAgentRuntimeSignalAgent(t, db, now)
	run := seedAgentRuntimeSignalRun(t, runRepo, model.AgentRunStatusPaused, model.AgentRunPauseReasonAuthentication, "not_required", now)
	mustExec(t, db, `UPDATE agents SET runtime_kind = ?, provider = ? WHERE id = ?`, "codex", "openai", "agent-1")
	mustExec(t, db, `UPDATE agent_runs SET runtime_kind = ? WHERE id = ?`, "codex", run.ID)
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := &AgentService{
		agentRepo:                agentRepo,
		runRepo:                  runRepo,
		agentRuntimeClient:       runtimeClient,
		codexOpenAIAuthMode:      "chatgpt_device_code",
		codexChatGPTOAuthEnabled: true,
	}

	state, err := svc.CancelCodexDeviceCodeAuth(context.Background(), "ws-1", run.ID, "user-1")
	if err != nil {
		t.Fatalf("CancelCodexDeviceCodeAuth returned error: %v", err)
	}
	if state == nil || state.State != model.CodexAuthStateCancelled {
		t.Fatalf("expected cancelled auth state, got %#v", state)
	}
	if len(runtimeClient.cancelAuthCalls) != 1 || runtimeClient.cancelAuthCalls[0] != "run_runtime_1" {
		t.Fatalf("expected runtime auth cancel call, got %#v", runtimeClient.cancelAuthCalls)
	}
	if len(runtimeClient.resumeCalls) != 0 {
		t.Fatalf("did not expect runtime resume, got %#v", runtimeClient.resumeCalls)
	}
}

func TestDelegatedCodexAuthPendingDoesNotResumeRuntime(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	now := time.Now().UTC()
	seedAgentRuntimeSignalAgent(t, db, now)
	run := seedAgentRuntimeSignalRun(t, runRepo, model.AgentRunStatusPaused, model.AgentRunPauseReasonAuthentication, "not_required", now)
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := &AgentService{
		runRepo:            runRepo,
		agentRuntimeClient: runtimeClient,
	}

	err := svc.applyCodexAuthState(context.Background(), "ws-1", run.ID, "user-1", &model.CodexAuthState{
		State:           model.CodexAuthStatePending,
		Provider:        "openai",
		AuthMode:        "chatgpt_device_code",
		VerificationURL: strPtr("https://example.test/device"),
		UserCode:        strPtr("ABCD-EFGH"),
		UpdatedAt:       now,
	}, true)
	if err != nil {
		t.Fatalf("applyCodexAuthState returned error: %v", err)
	}
	if len(runtimeClient.resumeCalls) != 0 {
		t.Fatalf("expected no runtime resume call, got %#v", runtimeClient.resumeCalls)
	}
	reloaded, err := runRepo.GetByID(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("reload run: %v", err)
	}
	if reloaded.Status != model.AgentRunStatusPaused || reloaded.PauseReason != model.AgentRunPauseReasonAuthentication {
		t.Fatalf("expected projection-owned status to remain paused/authentication, got %s/%s", reloaded.Status, reloaded.PauseReason)
	}
	if reloaded.ExecutionStage == nil || *reloaded.ExecutionStage != "awaiting_auth" {
		t.Fatalf("expected awaiting_auth stage, got %#v", reloaded.ExecutionStage)
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
