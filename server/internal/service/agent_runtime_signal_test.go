package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type fakeAgentRuntimeSignalClient struct {
	resumeCalls          []fakeAgentRuntimeResumeCall
	cancelCalls          []string
	startAuthCalls       []string
	cancelAuthCalls      []string
	upsertAgents         []AgentRuntimeAgent
	upsertResult         *AgentRuntimeAgent
	startRunCalls        []AgentRuntimeStartRunRequest
	appID                string
	getCalls             []string
	listMessageCalls     []string
	listArtifactCalls    []string
	listInteractionCalls []string
	listToolCallCalls    []string
	listV2EventCalls     []int64
	listV2EventPageSizes []int
	getRuns              map[string]*AgentRuntimeRun
	messages             map[string][]AgentRuntimeMessage
	artifacts            map[string][]AgentRuntimeArtifact
	interactions         map[string][]AgentRuntimeInteraction
	toolCalls            map[string][]AgentRuntimeToolCall
	v2Events             map[string][]AgentRuntimeEventEnvelope
	getErr               error
	listErr              error
	upsertErr            error
	startRunErr          error
	startRunErrs         []error
	startRunHook         func(AgentRuntimeStartRunRequest)
	resumeErr            error
	cancelErr            error
	pauseErr             error
	pauseCalls           []string
	startAuthErr         error
	cancelAuthErr        error
}

type fakeAgentRuntimeResumeCall struct {
	runID      string
	req        AgentRuntimeResumeRunRequest
	provenance string
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
	if c.upsertResult != nil {
		result := *c.upsertResult
		return &result, nil
	}
	return &agent, nil
}

func (c *fakeAgentRuntimeSignalClient) StartRun(_ context.Context, req AgentRuntimeStartRunRequest) (*AgentRuntimeRun, error) {
	c.startRunCalls = append(c.startRunCalls, req)
	if c.startRunHook != nil {
		c.startRunHook(req)
	}
	if len(c.startRunErrs) > 0 {
		err := c.startRunErrs[0]
		c.startRunErrs = c.startRunErrs[1:]
		if err != nil {
			return nil, err
		}
	}
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

func (c *fakeAgentRuntimeSignalClient) ListV2Events(_ context.Context, runtimeRunID string, afterSequence int64) (*AgentRuntimeEventListResponse, error) {
	c.listV2EventCalls = append(c.listV2EventCalls, afterSequence)
	if c.listErr != nil {
		return nil, c.listErr
	}
	events := make([]AgentRuntimeEventEnvelope, 0)
	for _, event := range c.v2Events[runtimeRunID] {
		if event.SequenceNo > afterSequence {
			events = append(events, event)
		}
	}
	return &AgentRuntimeEventListResponse{Events: events}, nil
}

func (c *fakeAgentRuntimeSignalClient) ListV2EventPage(_ context.Context, runtimeRunID string, afterSequence int64, pageSize int) (*AgentRuntimeEventListResponse, error) {
	c.listV2EventCalls = append(c.listV2EventCalls, afterSequence)
	c.listV2EventPageSizes = append(c.listV2EventPageSizes, pageSize)
	if c.listErr != nil {
		return nil, c.listErr
	}
	events := make([]AgentRuntimeEventEnvelope, 0, pageSize)
	for _, event := range c.v2Events[runtimeRunID] {
		if event.SequenceNo <= afterSequence {
			continue
		}
		events = append(events, event)
		if len(events) == pageSize {
			break
		}
	}
	nextSequence := afterSequence
	if len(events) > 0 {
		nextSequence = events[len(events)-1].SequenceNo
	}
	return &AgentRuntimeEventListResponse{Events: events, NextSequenceNo: nextSequence}, nil
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

func (c *fakeAgentRuntimeSignalClient) ListToolCalls(_ context.Context, runtimeRunID string) ([]AgentRuntimeToolCall, error) {
	c.listToolCallCalls = append(c.listToolCallCalls, runtimeRunID)
	if c.listErr != nil {
		return nil, c.listErr
	}
	return append([]AgentRuntimeToolCall(nil), c.toolCalls[runtimeRunID]...), nil
}

func (c *fakeAgentRuntimeSignalClient) ResumeRun(_ context.Context, runtimeRunID string, req AgentRuntimeResumeRunRequest) (*AgentRuntimeRun, error) {
	c.resumeCalls = append(c.resumeCalls, fakeAgentRuntimeResumeCall{runID: runtimeRunID, req: req})
	if c.resumeErr != nil {
		return nil, c.resumeErr
	}
	return &AgentRuntimeRun{ID: runtimeRunID, Status: model.AgentRunStatusRunning}, nil
}

func (c *fakeAgentRuntimeSignalClient) ResumeRunWithProvenance(ctx context.Context, runtimeRunID string, req AgentRuntimeResumeRunRequest, provenance string) (*AgentRuntimeRun, error) {
	run, err := c.ResumeRun(ctx, runtimeRunID, req)
	c.resumeCalls[len(c.resumeCalls)-1].provenance = provenance
	return run, err
}

func (c *fakeAgentRuntimeSignalClient) CancelRun(_ context.Context, runtimeRunID string) (*AgentRuntimeRun, error) {
	c.cancelCalls = append(c.cancelCalls, runtimeRunID)
	if c.cancelErr != nil {
		return nil, c.cancelErr
	}
	return &AgentRuntimeRun{ID: runtimeRunID, Status: model.AgentRunStatusCancelled}, nil
}

func (c *fakeAgentRuntimeSignalClient) PauseRun(_ context.Context, runtimeRunID string) (*AgentRuntimeRun, error) {
	c.pauseCalls = append(c.pauseCalls, runtimeRunID)
	if c.pauseErr != nil {
		return nil, c.pauseErr
	}
	return &AgentRuntimeRun{ID: runtimeRunID, Status: model.AgentRunStatusRunning}, nil
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
		agentRepo:      agentRepo,
		runRepo:        runRepo,
		runMessageRepo: runMessageRepo,

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

func TestResumeDockAskRunSendsExplicitCompletionPolicy(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	mustExec(t, db, `CREATE TABLE dock_chats (
 coverage_gap_id TEXT, initial_context TEXT,
execution_enabled boolean NOT NULL DEFAULT false,
		id text PRIMARY KEY,
		workspace_id text NOT NULL,
		next_message_sequence integer NOT NULL DEFAULT 0,
		updated_at datetime
	)`)
	mustExec(t, db, `INSERT INTO dock_chats (id, workspace_id, next_message_sequence, updated_at) VALUES (?, ?, ?, ?)`, "chat-1", "ws-1", 0, time.Now().UTC())
	mustExec(t, db, `ALTER TABLE agent_run_messages ADD COLUMN dock_chat_id text`)
	mustExec(t, db, `ALTER TABLE agent_run_messages ADD COLUMN dock_chat_sequence integer`)
	mustExec(t, db, `ALTER TABLE agent_run_messages ADD COLUMN client_message_id text`)
	mustExec(t, db, `ALTER TABLE agent_run_messages ADD COLUMN delivery_status text NOT NULL DEFAULT 'sent'`)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	now := time.Now().UTC()
	seedAgentRuntimeSignalAgent(t, db, now)
	run := seedAgentRuntimeSignalRun(t, runRepo, model.AgentRunStatusPaused, model.AgentRunPauseReasonUserMessage, "not_required", now)
	chatID := "chat-1"
	run.DockChatID = &chatID
	if err := db.Model(&model.AgentRun{}).Where("id = ?", run.ID).Update("dock_chat_id", chatID).Error; err != nil {
		t.Fatalf("mark run as dock chat: %v", err)
	}
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := &AgentService{
		agentRepo:          agentRepo,
		runRepo:            runRepo,
		runMessageRepo:     repository.NewAgentRunMessageRepository(db),
		agentRuntimeClient: runtimeClient,
	}

	if _, err := svc.ResumeRun(context.Background(), "ws-1", run.ID, "user-1", model.ResumeAgentRunRequest{
		Intent:  model.AgentRunResumeIntentReply,
		Content: "continue",
	}); err != nil {
		t.Fatalf("resume Ask run: %v", err)
	}
	if len(runtimeClient.resumeCalls) != 1 {
		t.Fatalf("expected one runtime resume call, got %d", len(runtimeClient.resumeCalls))
	}
	policy := runtimeClient.resumeCalls[0].req.TurnPolicy
	if policy == nil || policy.Mode != agentRuntimeTurnPauseAfterAssist || policy.CompletionMode != agentRuntimeTurnCompletionExplicit || policy.MaxCompletionCorrections != askAgentCompletionCorrections {
		t.Fatalf("Ask resume did not send the guarded turn policy: %#v", policy)
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
		agentRepo:      agentRepo,
		runRepo:        runRepo,
		runMessageRepo: runMessageRepo,

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
		agentRepo:      agentRepo,
		runRepo:        runRepo,
		runMessageRepo: runMessageRepo,

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
		agentRepo:      agentRepo,
		runRepo:        runRepo,
		runMessageRepo: runMessageRepo,

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
	projection := NewAgentRuntimeProjectionService(runRepo)
	svc := &AgentService{
		agentRepo:              agentRepo,
		runRepo:                runRepo,
		agentRuntimeClient:     runtimeClient,
		agentRuntimeProjection: projection,
	}

	updated, err := svc.CancelRun(context.Background(), "ws-1", run.ID, "user-1")
	if err != nil {
		t.Fatalf("CancelRun returned error: %v", err)
	}
	if len(runtimeClient.cancelCalls) != 1 || runtimeClient.cancelCalls[0] != "run_runtime_1" {
		t.Fatalf("expected runtime cancel call, got %#v", runtimeClient.cancelCalls)
	}
	if updated.Status != model.AgentRunStatusCancelled || updated.CompletedAt == nil {
		t.Fatalf("expected runtime cancellation acknowledgement to be projected immediately, got status=%s completed_at=%v", updated.Status, updated.CompletedAt)
	}
}

func TestManualPauseAndResumeKeepsTheSameRuntimeRun(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	now := time.Now().UTC()
	seedAgentRuntimeSignalAgent(t, db, now)
	run := seedAgentRuntimeSignalRun(t, runRepo, model.AgentRunStatusRunning, model.AgentRunPauseReasonNone, "not_required", now)
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := &AgentService{runRepo: runRepo, agentRuntimeClient: runtimeClient}

	pausing, err := svc.PauseRun(context.Background(), "ws-1", run.ID, "user-1")
	if err != nil {
		t.Fatalf("PauseRun: %v", err)
	}
	if len(runtimeClient.pauseCalls) != 1 || runtimeClient.pauseCalls[0] != "run_runtime_1" {
		t.Fatalf("unexpected runtime pause calls: %#v", runtimeClient.pauseCalls)
	}
	if pausing.Status != model.AgentRunStatusRunning || pausing.ExecutionStage == nil || *pausing.ExecutionStage != "pausing" {
		t.Fatalf("pause should stay pending until runtime acknowledgement: %#v", pausing)
	}
	run.Status = model.AgentRunStatusPaused
	run.PauseReason = model.AgentRunPauseReasonManual
	if err := runRepo.Update(context.Background(), run); err != nil {
		t.Fatalf("project manual pause: %v", err)
	}
	resuming, err := svc.ResumeManuallyPausedRun(context.Background(), "ws-1", run.ID, "user-1")
	if err != nil {
		t.Fatalf("ResumeManuallyPausedRun: %v", err)
	}
	if len(runtimeClient.resumeCalls) != 1 || runtimeClient.resumeCalls[0].runID != "run_runtime_1" || runtimeClient.resumeCalls[0].req.Intent != "continue" {
		t.Fatalf("unexpected runtime resume calls: %#v", runtimeClient.resumeCalls)
	}
	if resuming.ID != run.ID || resuming.ExecutionStage == nil || *resuming.ExecutionStage != "resuming" {
		t.Fatalf("resume should target the same run: %#v", resuming)
	}
}

func TestCancelRunWithoutRuntimeMappingSignalsRuntimeByHostRunID(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	now := time.Now().UTC()
	seedAgentRuntimeSignalAgent(t, db, now)
	run := seedAgentRuntimeSignalRun(t, runRepo, model.AgentRunStatusRunning, model.AgentRunPauseReasonNone, "not_required", now)
	run.ExternalRuntime = nil
	run.ExternalRuntimeID = nil
	if err := runRepo.Update(context.Background(), run); err != nil {
		t.Fatalf("remove runtime mapping: %v", err)
	}
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	projection := NewAgentRuntimeProjectionService(runRepo)
	svc := &AgentService{
		agentRepo:              agentRepo,
		runRepo:                runRepo,
		agentRuntimeClient:     runtimeClient,
		agentRuntimeProjection: projection,
	}

	updated, err := svc.CancelRun(context.Background(), "ws-1", run.ID, "user-1")
	if err != nil {
		t.Fatalf("CancelRun returned error: %v", err)
	}
	if len(runtimeClient.cancelCalls) != 1 || runtimeClient.cancelCalls[0] != run.ID {
		t.Fatalf("expected runtime cancel by host run id %q, got %#v", run.ID, runtimeClient.cancelCalls)
	}
	if updated.Status != model.AgentRunStatusCancelled || updated.CompletedAt == nil {
		t.Fatalf("expected host-ID cancellation acknowledgement to be projected immediately, got status=%s completed_at=%v", updated.Status, updated.CompletedAt)
	}
}

// setupAgentRuntimeSupportRunTestDB extends the shared interactive-approval
// schema with the support tables that runConversationAgent's conversation
// lookup joins against.
func setupAgentRuntimeSupportRunTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newInteractiveApprovalTestDB(t)
	for _, stmt := range []string{
		`CREATE TABLE support_mailboxes (
			id text PRIMARY KEY,
			workspace_id text NOT NULL,
			name text NOT NULL,
			handle text NOT NULL,
			icon text NOT NULL DEFAULT 'inbox',
			description text,
			routing_prompt text,
			triage_eligible boolean NOT NULL DEFAULT true,
			linked_team_id text,
			visibility_mode text NOT NULL DEFAULT 'members_only',
			assignment_mode text NOT NULL DEFAULT 'manual',
			reply_time_preset text,
			reply_time_custom_minutes integer,
			position integer NOT NULL DEFAULT 0,
			active boolean NOT NULL DEFAULT true,
			created_by_id text NOT NULL DEFAULT '',
			created_at datetime,
			updated_at datetime
		)`,
		`CREATE TABLE support_conversations (
			id text PRIMARY KEY,
			workspace_id text NOT NULL,
			mailbox_id text,
			display_id integer NOT NULL,
			subject text NOT NULL,
			status text NOT NULL DEFAULT 'open',
			flow_state text,
			priority text NOT NULL DEFAULT 'medium',
			channel text NOT NULL DEFAULT 'widget',
			customer_name text,
			customer_email text,
			customer_phone text,
			opened_by_user_id text,
			assigned_user_id text,
			assigned_agent_id text,
			linked_task_id text,
			source text NOT NULL DEFAULT 'internal',
			anonymous_id text,
			crm_contact_id text,
			resolved_at datetime,
			closed_at datetime,
			team_last_seen_at datetime,
			contact_last_seen_at datetime,
			email_unsubscribed boolean NOT NULL DEFAULT false,
			ai_state text,
			ai_resolved_at datetime,
			ai_escalated_at datetime,
			ai_resolution_type text,
			ai_turn_count integer NOT NULL DEFAULT 0,
			customer_requested_human_at datetime,
			ai_active_run_id TEXT,
			human_takeover boolean DEFAULT false,
            ai_control_version BIGINT NOT NULL DEFAULT 0, ai_resumed_at timestamptz, ai_paused_at timestamptz, ai_paused_by_user_id TEXT,
			created_at datetime,
			updated_at datetime
		)`,
		`CREATE TABLE support_messages (
			id text PRIMARY KEY,
			workspace_id text NOT NULL,
			conversation_id text NOT NULL,
			sender_type text NOT NULL,
			message_type text NOT NULL DEFAULT 'reply',
			system_event_type text,
			is_internal boolean NOT NULL DEFAULT false,
			created_at datetime,
			deleted_at datetime
		)`,
		`CREATE TABLE support_widget_sessions (
			id text PRIMARY KEY,
			workspace_id text NOT NULL,
			conversation_id text,
			anonymous_id text,
			country_code text,
			country_name text,
			created_at datetime
		)`,
	} {
		mustExec(t, db, stmt)
	}
	return db
}

func seedAgentRuntimeSupportAgent(t *testing.T, db *gorm.DB, invocationMode string, now time.Time) {
	t.Helper()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", true, "Echo", model.AgentPresetSupportAgent, "Support Agent", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "always", 1, invocationMode, now, now,
	)
}

func seedAgentRuntimeSupportConversation(t *testing.T, db *gorm.DB, now time.Time) {
	t.Helper()
	mustExec(t, db, `INSERT INTO support_conversations (
		id, workspace_id, display_id, subject, status, priority, channel, assigned_agent_id, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"conv-1", "ws-1", 1, "Widget cannot load", "open", "medium", "widget", "agent-1", now, now,
	)
}

func newDelegatedSupportRunService(t *testing.T, db *gorm.DB, runtimeClient *fakeAgentRuntimeSignalClient) *AgentService {
	t.Helper()
	return (&AgentService{
		agentRepo:        repository.NewAgentRepository(db),
		runRepo:          repository.NewAgentRunRepository(db),
		runMessageRepo:   repository.NewAgentRunMessageRepository(db),
		conversationRepo: repository.NewSupportConversationRepository(db),

		agentRuntimeClient: runtimeClient,
	}).SetAgentRuntimeLaunchEnabled(true)
}

// TestRunConversationAgentDelegatesSupportRunToAgentRuntime proves the manual
// support launch chain reaches createRun's delegation branch with the
// conversation-specific input assembly intact: support_conversation target,
// conversation_id stamped, workspace_id metadata for the host target-context
// lookup, and no local Temporal workflow.
func TestRunConversationAgentDelegatesSupportRunToAgentRuntime(t *testing.T) {
	db := setupAgentRuntimeSupportRunTestDB(t)
	now := time.Now().UTC()
	seedAgentRuntimeSupportAgent(t, db, model.InvocationModeAutonomous, now)
	seedAgentRuntimeSupportConversation(t, db, now)
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := newDelegatedSupportRunService(t, db, runtimeClient)

	run, err := svc.RunConversationAgent(context.Background(), "ws-1", "conv-1", "user-1")
	if err != nil {
		t.Fatalf("RunConversationAgent returned error: %v", err)
	}
	if len(runtimeClient.upsertAgents) != 1 {
		t.Fatalf("expected one runtime agent upsert, got %d", len(runtimeClient.upsertAgents))
	}
	// System agents are record-normalized to approval_mode=never; support
	// draft approval is enforced at execution level (support skill +
	// approval interaction), exactly like the Temporal path.
	if runtimeClient.upsertAgents[0].ApprovalMode != "never" {
		t.Fatalf("system support agent record normalizes to approval_mode=never, got %q", runtimeClient.upsertAgents[0].ApprovalMode)
	}
	if len(runtimeClient.startRunCalls) != 1 {
		t.Fatalf("expected one runtime start call, got %d", len(runtimeClient.startRunCalls))
	}
	start := runtimeClient.startRunCalls[0]
	if start.HostRunID != run.ID || start.Target.Type != "support_conversation" || start.Target.ID != "conv-1" {
		t.Fatalf("unexpected runtime start target: %#v", start)
	}
	if start.Metadata["workspace_id"] != "ws-1" || start.Target.Metadata["workspace_id"] != "ws-1" {
		t.Fatalf("support target context requires workspace_id metadata, got metadata=%#v target=%#v", start.Metadata, start.Target.Metadata)
	}
	if start.ExternalActorID != "user-1" || start.Mode != model.InvocationModeAutonomous {
		t.Fatalf("unexpected actor/mode: %#v", start)
	}
	if start.TurnPolicy.Mode != "complete_on_finish" {
		t.Fatalf("autonomous support run must complete on finish so the draft finalizer fires, got %#v", start.TurnPolicy)
	}
	if start.Trigger["source"] != model.AgentRunTriggerSourceManual || start.Trigger["trigger_type"] != model.AgentRunTriggerTypeManual {
		t.Fatalf("expected manual trigger context, got %#v", start.Trigger)
	}
	if run.ConversationID == nil || *run.ConversationID != "conv-1" {
		t.Fatalf("expected conversation_id stamped on run, got %#v", run.ConversationID)
	}
	var input model.AgentRunInputPayload
	if err := json.Unmarshal(run.Input, &input); err != nil {
		t.Fatalf("parse run input: %v", err)
	}
	if input.Target == nil || input.Target.TargetType != "support_conversation" || input.Target.TargetID != "conv-1" {
		t.Fatalf("unexpected run input target: %#v", input.Target)
	}
	reloaded, err := repository.NewAgentRunRepository(db).GetByID(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("reload run: %v", err)
	}
	if reloaded.ExternalRuntime == nil || *reloaded.ExternalRuntime != agentRuntimeName || reloaded.ExternalRuntimeID == nil || *reloaded.ExternalRuntimeID != "run_runtime_1" {
		t.Fatalf("expected delegated runtime mapping, got %v/%v", reloaded.ExternalRuntime, reloaded.ExternalRuntimeID)
	}
	if reloaded.WorkflowID != nil || reloaded.WorkflowRunID != nil {
		t.Fatalf("expected no local Temporal workflow, got %v/%v", reloaded.WorkflowID, reloaded.WorkflowRunID)
	}
}

// TestStartRunFailsLoudlyWhenAgentRuntimeLaunchDisabled proves there is no
// local executor fallback: with AGENT_RUNTIME_LAUNCH_ENABLED off the run row
// is created and immediately failed with an explicit error.
func TestStartRunFailsLoudlyWhenAgentRuntimeLaunchDisabled(t *testing.T) {
	db := setupAgentRuntimeSupportRunTestDB(t)
	now := time.Now().UTC()
	seedAgentRuntimeSupportAgent(t, db, model.InvocationModeAutonomous, now)
	seedAgentRuntimeSupportConversation(t, db, now)
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := newDelegatedSupportRunService(t, db, runtimeClient)
	svc.SetAgentRuntimeLaunchEnabled(false)

	_, err := svc.RunConversationAgent(context.Background(), "ws-1", "conv-1", "user-1")
	if err == nil || !strings.Contains(err.Error(), "no execution path") {
		t.Fatalf("expected loud launch-disabled failure, got %v", err)
	}
	if len(runtimeClient.startRunCalls) != 0 {
		t.Fatalf("expected no runtime start call, got %d", len(runtimeClient.startRunCalls))
	}
	var runs []model.AgentRun
	if err := db.Where("workspace_id = ?", "ws-1").Find(&runs).Error; err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected one failed run row, got %d", len(runs))
	}
	if runs[0].Status != model.AgentRunStatusFailed {
		t.Fatalf("expected failed run, got %q", runs[0].Status)
	}
	if runs[0].ErrorMessage == nil || !strings.Contains(*runs[0].ErrorMessage, "no execution path") {
		t.Fatalf("expected explicit error message, got %v", runs[0].ErrorMessage)
	}
}

// TestRunConversationAgentAutoDelegatesWidgetAutoRunToAgentRuntime covers the
// widget auto-run trigger path (maybeAutoRunConversationAgent →
// RunConversationAgentAuto): no actor, a system support.auto trigger, and —
// for an interactive-configured support agent — the pause_after_assistant
// turn policy.
func TestRunConversationAgentAutoDelegatesWidgetAutoRunToAgentRuntime(t *testing.T) {
	db := setupAgentRuntimeSupportRunTestDB(t)
	now := time.Now().UTC()
	seedAgentRuntimeSupportAgent(t, db, model.InvocationModeInteractive, now)
	seedAgentRuntimeSupportConversation(t, db, now)
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := newDelegatedSupportRunService(t, db, runtimeClient)

	run, err := svc.RunConversationAgentAuto(context.Background(), "ws-1", "conv-1")
	if err != nil {
		t.Fatalf("RunConversationAgentAuto returned error: %v", err)
	}
	if len(runtimeClient.startRunCalls) != 1 {
		t.Fatalf("expected one runtime start call, got %d", len(runtimeClient.startRunCalls))
	}
	start := runtimeClient.startRunCalls[0]
	if start.HostRunID != run.ID || start.Target.Type != "support_conversation" || start.Target.ID != "conv-1" {
		t.Fatalf("unexpected runtime start target: %#v", start)
	}
	if start.ExternalActorID != "" {
		t.Fatalf("widget auto-run has no actor, got %q", start.ExternalActorID)
	}
	if start.Trigger["source"] != model.AgentRunTriggerSourceSystem || start.Trigger["trigger_type"] != supportAutoTriggerType {
		t.Fatalf("expected system support.auto trigger context, got %#v", start.Trigger)
	}
	if start.Mode != model.InvocationModeInteractive || start.TurnPolicy.Mode != "pause_after_assistant" {
		t.Fatalf("interactive support run must pause after assistant turns, got mode=%q policy=%#v", start.Mode, start.TurnPolicy)
	}
}

func seedDelegatedSupportRun(t *testing.T, runRepo *repository.AgentRunRepository, status, pauseReason, approvalState string, outputSummary string, now time.Time) *model.AgentRun {
	t.Helper()
	conversationID := "conv-1"
	if outputSummary == "" {
		outputSummary = "{}"
	}
	run := &model.AgentRun{
		ID:                "run-support-1",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		ConversationID:    &conversationID,
		TargetType:        "support_conversation",
		TargetID:          conversationID,
		RuntimeKind:       "native_sdk",
		InvocationMode:    model.InvocationModeAutonomous,
		ApprovalState:     approvalState,
		PauseReason:       pauseReason,
		Status:            status,
		ExternalRuntime:   strPtr(agentRuntimeName),
		ExternalRuntimeID: strPtr("run_runtime_1"),
		OutputSummary:     json.RawMessage(outputSummary),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	return run
}

const delegatedSupportDraftSummary = `{"draft_reply":{"content":"Hi, try clearing the widget cache.","is_internal":false,"approval_required":true}}`

// TestDelegatedSupportRunReplyForwardsHumanInput proves the human-input
// interaction round-trip for a delegated support run: a teammate reply on a
// human_input pause forwards a reply intent to the runtime and persists the
// local run message.
func TestDelegatedSupportRunReplyForwardsHumanInput(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	now := time.Now().UTC()
	seedAgentRuntimeSignalAgent(t, db, now)
	run := seedDelegatedSupportRun(t, runRepo, model.AgentRunStatusPaused, model.AgentRunPauseReasonHumanInput, "not_required", "", now)
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := &AgentService{
		agentRepo:      repository.NewAgentRepository(db),
		runRepo:        runRepo,
		runMessageRepo: repository.NewAgentRunMessageRepository(db),

		agentRuntimeClient: runtimeClient,
	}

	_, err := svc.ResumeRun(context.Background(), "ws-1", run.ID, "user-1", model.ResumeAgentRunRequest{
		Intent:  model.AgentRunResumeIntentReply,
		Content: "the customer is on the enterprise plan",
	})
	if err != nil {
		t.Fatalf("ResumeRun returned error: %v", err)
	}
	if len(runtimeClient.resumeCalls) != 1 {
		t.Fatalf("expected one runtime resume call, got %d", len(runtimeClient.resumeCalls))
	}
	call := runtimeClient.resumeCalls[0]
	if call.runID != "run_runtime_1" || call.req.Intent != model.AgentRunResumeIntentReply || call.req.Content != "the customer is on the enterprise plan" {
		t.Fatalf("unexpected runtime resume request: %#v", call)
	}
	if call.req.TurnPolicy != nil {
		t.Fatalf("support chat must not opt into Ask completion guard, got %#v", call.req.TurnPolicy)
	}
	messages, err := repository.NewAgentRunMessageRepository(db).ListByRun(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 1 || messages[0].MessageType != "user_reply" {
		t.Fatalf("expected one local user_reply message, got %#v", messages)
	}
}

// TestDelegatedSupportRunApproveForwardsApprovalAndKeepsDraftStaged proves the
// approval round-trip: approving a draft-paused delegated support run forwards
// the approve intent, flips the local approval state (the gate
// finalizeSupportDraft checks), and leaves the staged draft_reply summary
// intact for the terminal finalizer.
func TestDelegatedSupportRunApproveForwardsApprovalAndKeepsDraftStaged(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	now := time.Now().UTC()
	seedAgentRuntimeSignalAgent(t, db, now)
	run := seedDelegatedSupportRun(t, runRepo, model.AgentRunStatusPaused, model.AgentRunPauseReasonHumanApproval, "pending", delegatedSupportDraftSummary, now)
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := &AgentService{
		agentRepo:      repository.NewAgentRepository(db),
		runRepo:        runRepo,
		runMessageRepo: repository.NewAgentRunMessageRepository(db),

		agentRuntimeClient: runtimeClient,
	}

	updated, err := svc.ApproveRun(context.Background(), "ws-1", run.ID, "user-1", model.ApproveAgentRunRequest{})
	if err != nil {
		t.Fatalf("ApproveRun returned error: %v", err)
	}
	if len(runtimeClient.resumeCalls) != 1 {
		t.Fatalf("expected one runtime resume call, got %d", len(runtimeClient.resumeCalls))
	}
	call := runtimeClient.resumeCalls[0]
	if call.runID != "run_runtime_1" || call.req.Intent != model.AgentRunResumeIntentApprove {
		t.Fatalf("unexpected runtime resume request: %#v", call)
	}
	if updated.ApprovalState != "approved" {
		t.Fatalf("expected approved local approval state, got %q", updated.ApprovalState)
	}
	reloaded, err := runRepo.GetByID(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("reload run: %v", err)
	}
	var summary struct {
		DraftReply *struct {
			Content string `json:"content"`
		} `json:"draft_reply"`
	}
	if err := json.Unmarshal(reloaded.OutputSummary, &summary); err != nil {
		t.Fatalf("parse output summary: %v", err)
	}
	if summary.DraftReply == nil || summary.DraftReply.Content != "Hi, try clearing the widget cache." {
		t.Fatalf("staged draft_reply must survive approval for the terminal finalizer, got %s", string(reloaded.OutputSummary))
	}
}

// TestDelegatedSupportRunRequestChangesForwardsIntent proves the
// request-changes round-trip: rejecting a draft forwards request_changes with
// the feedback and marks the local run rejected so finalizeSupportDraft's
// pending gate is never bypassed with an unapproved draft.
func TestDelegatedSupportRunRequestChangesForwardsIntent(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	now := time.Now().UTC()
	seedAgentRuntimeSignalAgent(t, db, now)
	run := seedDelegatedSupportRun(t, runRepo, model.AgentRunStatusPaused, model.AgentRunPauseReasonHumanApproval, "pending", delegatedSupportDraftSummary, now)
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := &AgentService{
		agentRepo:      repository.NewAgentRepository(db),
		runRepo:        runRepo,
		runMessageRepo: repository.NewAgentRunMessageRepository(db),

		agentRuntimeClient: runtimeClient,
	}

	updated, err := svc.RequestRunChanges(context.Background(), "ws-1", run.ID, "user-1", model.SendAgentRunRequestChangesRequest{
		Content: "mention the status page link",
	})
	if err != nil {
		t.Fatalf("RequestRunChanges returned error: %v", err)
	}
	if len(runtimeClient.resumeCalls) != 1 {
		t.Fatalf("expected one runtime resume call, got %d", len(runtimeClient.resumeCalls))
	}
	call := runtimeClient.resumeCalls[0]
	if call.req.Intent != model.AgentRunResumeIntentRequestChanges || call.req.Content != "mention the status page link" {
		t.Fatalf("unexpected runtime resume request: %#v", call)
	}
	if updated.ApprovalState != "rejected" {
		t.Fatalf("expected rejected local approval state, got %q", updated.ApprovalState)
	}
	messages, err := repository.NewAgentRunMessageRepository(db).ListByRun(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 1 || messages[0].MessageType != "request_changes" {
		t.Fatalf("expected one local request_changes message, got %#v", messages)
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
