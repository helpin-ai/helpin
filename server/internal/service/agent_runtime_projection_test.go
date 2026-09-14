package service

import (
	"context"
	"encoding/json"
	"errors"
	agentruntime "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	"strings"
	"testing"
	"time"
)

type fakeAgentRuntimeProjectionRunRepo struct {
	byID           map[string]*model.AgentRun
	byExternal     map[string]*model.AgentRun
	active         []model.AgentRun
	updates        int
	summaryUpdates int
	notifications  int
}

func (r *fakeAgentRuntimeProjectionRunRepo) GetByIDAny(_ context.Context, id string) (*model.AgentRun, error) {
	if r.byID == nil {
		return nil, nil
	}
	return r.byID[id], nil
}

func (r *fakeAgentRuntimeProjectionRunRepo) GetByExternalRuntimeID(_ context.Context, externalRuntime, externalRuntimeID string) (*model.AgentRun, error) {
	if r.byExternal == nil {
		return nil, nil
	}
	return r.byExternal[externalRuntime+"|"+externalRuntimeID], nil
}

func (r *fakeAgentRuntimeProjectionRunRepo) ListActiveByExternalRuntime(_ context.Context, _ string, _ time.Time, _ int) ([]model.AgentRun, error) {
	return r.active, nil
}

func (r *fakeAgentRuntimeProjectionRunRepo) Update(_ context.Context, run *model.AgentRun) error {
	r.updates++
	if r.byID == nil {
		r.byID = map[string]*model.AgentRun{}
	}
	r.byID[run.ID] = run
	if run.ExternalRuntime != nil && run.ExternalRuntimeID != nil {
		if r.byExternal == nil {
			r.byExternal = map[string]*model.AgentRun{}
		}
		r.byExternal[*run.ExternalRuntime+"|"+*run.ExternalRuntimeID] = run
	}
	for index := range r.active {
		if r.active[index].ID == run.ID {
			r.active[index] = *run
			break
		}
	}
	return nil
}

func (r *fakeAgentRuntimeProjectionRunRepo) UpdateRuntimeProjection(ctx context.Context, run *model.AgentRun) error {
	return r.Update(ctx, run)
}

func (r *fakeAgentRuntimeProjectionRunRepo) UpdateOutputSummary(_ context.Context, runID string, outputSummary json.RawMessage) error {
	r.summaryUpdates++
	if r.byID != nil && r.byID[runID] != nil {
		r.byID[runID].OutputSummary = append(json.RawMessage(nil), outputSummary...)
	}
	for index := range r.active {
		if r.active[index].ID == runID {
			r.active[index].OutputSummary = append(json.RawMessage(nil), outputSummary...)
			break
		}
	}
	return nil
}

func (r *fakeAgentRuntimeProjectionRunRepo) UpdateRuntimeSummaryMarker(ctx context.Context, workspaceID, runID, key string, value json.RawMessage) error {
	body := map[string]json.RawMessage{}
	if run := r.byID[runID]; run != nil && len(run.OutputSummary) > 0 {
		if err := json.Unmarshal(run.OutputSummary, &body); err != nil {
			return err
		}
	}
	if body == nil {
		body = map[string]json.RawMessage{}
	}
	body[key] = value
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return r.UpdateOutputSummary(ctx, runID, encoded)
}

func (r *fakeAgentRuntimeProjectionRunRepo) Notify(_ context.Context, _ *model.AgentRun) {
	r.notifications++
}

type fakeAgentRuntimeProjectionAgentRepo struct {
	agent *model.Agent
	err   error
}

func (r *fakeAgentRuntimeProjectionAgentRepo) GetByID(_ context.Context, _, _ string) (*model.Agent, error) {
	return r.agent, r.err
}

type fakeAgentRuntimeProjectionUsageConsumer struct {
	preflightErr    error
	preflightInputs []BillingCreditPreflight
	consumeErr      error
	consumeInputs   []BillingCreditConsumption
}

func (c *fakeAgentRuntimeProjectionUsageConsumer) PreflightCredits(_ context.Context, input BillingCreditPreflight) error {
	c.preflightInputs = append(c.preflightInputs, input)
	return c.preflightErr
}

func (c *fakeAgentRuntimeProjectionUsageConsumer) ConsumeCredits(_ context.Context, input BillingCreditConsumption) (*BillingSummary, error) {
	c.consumeInputs = append(c.consumeInputs, input)
	if c.consumeErr != nil {
		return nil, c.consumeErr
	}
	return nil, nil
}

type fakeAgentRuntimeProjectionMessageRepo struct {
	messages []model.AgentRunMessage
	creates  int
	updates  int
}

func (r *fakeAgentRuntimeProjectionMessageRepo) ListByRun(_ context.Context, _, _ string) ([]model.AgentRunMessage, error) {
	return append([]model.AgentRunMessage(nil), r.messages...), nil
}

func (r *fakeAgentRuntimeProjectionMessageRepo) NextSequence(_ context.Context, _, _ string) (int, error) {
	return len(r.messages) + 1, nil
}

func (r *fakeAgentRuntimeProjectionMessageRepo) Create(_ context.Context, message *model.AgentRunMessage) error {
	r.creates++
	r.messages = append(r.messages, *message)
	return nil
}

func (r *fakeAgentRuntimeProjectionMessageRepo) Update(_ context.Context, message *model.AgentRunMessage) error {
	r.updates++
	for index := range r.messages {
		if (message.ID != "" && r.messages[index].ID == message.ID) || (r.messages[index].RunID == message.RunID && r.messages[index].SequenceNo == message.SequenceNo) {
			r.messages[index] = *message
			return nil
		}
	}
	r.messages = append(r.messages, *message)
	return nil
}

type fakeAgentRuntimeProjectionArtifactRepo struct {
	artifacts []model.AgentRunArtifact
	creates   int
}

func (r *fakeAgentRuntimeProjectionArtifactRepo) ListByRun(_ context.Context, _, _ string) ([]model.AgentRunArtifact, error) {
	return append([]model.AgentRunArtifact(nil), r.artifacts...), nil
}

func (r *fakeAgentRuntimeProjectionArtifactRepo) NextSequence(_ context.Context, _, _ string) (int, error) {
	return len(r.artifacts) + 1, nil
}

func (r *fakeAgentRuntimeProjectionArtifactRepo) Create(_ context.Context, artifact *model.AgentRunArtifact) error {
	r.creates++
	r.artifacts = append(r.artifacts, *artifact)
	return nil
}

type fakeAgentRuntimeProjectionInteractionRepo struct {
	interactions []model.AgentRunInteraction
	creates      int
	updates      int
}

func (r *fakeAgentRuntimeProjectionInteractionRepo) ListByRun(_ context.Context, _, _ string) ([]model.AgentRunInteraction, error) {
	return append([]model.AgentRunInteraction(nil), r.interactions...), nil
}

func (r *fakeAgentRuntimeProjectionInteractionRepo) Create(_ context.Context, interaction *model.AgentRunInteraction) error {
	r.creates++
	r.interactions = append(r.interactions, *interaction)
	return nil
}

func (r *fakeAgentRuntimeProjectionInteractionRepo) Update(_ context.Context, interaction *model.AgentRunInteraction) error {
	r.updates++
	for index := range r.interactions {
		if r.interactions[index].ID == interaction.ID {
			r.interactions[index] = *interaction
			return nil
		}
	}
	r.interactions = append(r.interactions, *interaction)
	return nil
}

type fakeAgentRuntimeProjectionSessionSnapshotRepo struct {
	record        *model.CodingSessionStateSnapshot
	upserts       int
	deletes       int
	upsertStarted chan<- struct{}
	releaseUpsert <-chan struct{}
}

type fakeAgentRuntimeProjectionPublisher struct {
	events []websocket.Event
}

func (p *fakeAgentRuntimeProjectionPublisher) Publish(event websocket.Event) {
	p.events = append(p.events, event)
}

func (r *fakeAgentRuntimeProjectionSessionSnapshotRepo) GetByRun(_ context.Context, _, _ string) (*model.CodingSessionStateSnapshot, error) {
	if r.record == nil {
		return nil, nil
	}
	copy := *r.record
	return &copy, nil
}

func (r *fakeAgentRuntimeProjectionSessionSnapshotRepo) UpsertIfNewer(
	_ context.Context,
	snapshot *model.CodingSessionStateSnapshot,
) (bool, error) {
	r.upserts++
	if r.upsertStarted != nil {
		r.upsertStarted <- struct{}{}
	}
	if r.releaseUpsert != nil {
		<-r.releaseUpsert
	}
	if snapshot == nil {
		r.record = nil
		return false, nil
	}
	if r.record != nil && snapshot.ThroughSequence > 0 &&
		r.record.ThroughSequence >= snapshot.ThroughSequence {
		return false, nil
	}
	copy := *snapshot
	r.record = &copy
	return true, nil
}

func (r *fakeAgentRuntimeProjectionSessionSnapshotRepo) DeleteByRun(_ context.Context, _, _ string) error {
	r.deletes++
	r.record = nil
	return nil
}

func TestAgentRuntimeProjectionMapsLifecycleByHostRunID(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	sentAt := now.Add(-2 * time.Minute)
	run := &model.AgentRun{
		ID:          "helpin-run-1",
		Status:      model.AgentRunStatusQueued,
		PauseReason: model.AgentRunPauseReasonNone,
	}
	repo := &fakeAgentRuntimeProjectionRunRepo{byID: map[string]*model.AgentRun{run.ID: run}}
	svc := &AgentRuntimeProjectionService{
		runRepo: repo,
		now:     func() time.Time { return now },
	}

	err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID:     "run_runtime_1",
		HostRunID: run.ID,
		Type:      "run.started",
		SentAt:    sentAt,
	})
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if run.Status != model.AgentRunStatusRunning || run.PauseReason != model.AgentRunPauseReasonNone {
		t.Fatalf("expected running/none, got %s/%s", run.Status, run.PauseReason)
	}
	if run.ExternalRuntime == nil || *run.ExternalRuntime != agentRuntimeName {
		t.Fatalf("expected external runtime to be set, got %#v", run.ExternalRuntime)
	}
	if run.ExternalRuntimeID == nil || *run.ExternalRuntimeID != "run_runtime_1" {
		t.Fatalf("expected external runtime id, got %#v", run.ExternalRuntimeID)
	}
	if run.StartedAt == nil || !run.StartedAt.Equal(sentAt) {
		t.Fatalf("expected started_at %s, got %#v", sentAt, run.StartedAt)
	}
	if repo.updates != 1 || repo.notifications != 1 {
		t.Fatalf("expected one update/notify, got %d/%d", repo.updates, repo.notifications)
	}

	err = svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_1",
		Type:  "run.paused",
		Data:  map[string]any{"pause_reason": model.AgentRunPauseReasonHumanApproval},
	})
	if err != nil {
		t.Fatalf("ApplyEvent paused returned error: %v", err)
	}
	if run.Status != model.AgentRunStatusPaused || run.PauseReason != model.AgentRunPauseReasonHumanApproval {
		t.Fatalf("expected paused/human_approval, got %s/%s", run.Status, run.PauseReason)
	}

	err = svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_1",
		Type:  "run.paused",
		Data:  map[string]any{"pause_reason": model.AgentRunPauseReasonUserMessage},
	})
	if err != nil {
		t.Fatalf("ApplyEvent awaiting user message returned error: %v", err)
	}
	if run.Status != model.AgentRunStatusPaused || run.PauseReason != model.AgentRunPauseReasonUserMessage {
		t.Fatalf("expected paused/awaiting_user_message, got %s/%s", run.Status, run.PauseReason)
	}
}

func TestAgentRuntimeProjectionRejectsProseOnlyCustomAgentCoverageCompletion(t *testing.T) {
	run := &model.AgentRun{
		ID: "run-gap", WorkspaceID: "workspace-1", AgentID: "agent-custom",
		TargetType: "support_coverage_gap", TargetID: "gap-1",
		Status: model.AgentRunStatusRunning,
	}
	repo := &fakeAgentRuntimeProjectionRunRepo{byID: map[string]*model.AgentRun{run.ID: run}}
	svc := &AgentRuntimeProjectionService{
		runRepo: repo,
		now:     time.Now,
	}

	err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "runtime-gap", HostRunID: run.ID, Type: agentruntime.EventRunCompleted,
	})
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if run.Status != model.AgentRunStatusFailed {
		t.Fatalf("expected prose-only coverage run to fail, got %q", run.Status)
	}
	if run.ErrorMessage == nil || !strings.Contains(*run.ErrorMessage, "complete_support_coverage_gap") {
		t.Fatalf("expected repair-oriented completion error, got %#v", run.ErrorMessage)
	}
}

func TestAgentRuntimeProjectionAcceptsDurableCustomAgentCoverageCompletion(t *testing.T) {
	outcome := supportCoverageGapOutcomeSummary{
		Outcome: SupportCoverageAgentOutcomeReviewReady, Action: SupportCoverageAgentActionProposalSubmitted,
		SourceStatus: SupportCoverageAgentSourceVerified, DocumentID: "doc-1", ProposalID: "proposal-1",
		DocumentationEvidence: "No existing command bar article was found.",
		SourceEvidence:        "Verified the command bar implementation.",
		Summary:               "Proposed a verified command bar help article.",
	}
	summary, err := json.Marshal(map[string]any{supportCoverageGapOutcomeSummaryKey: outcome})
	if err != nil {
		t.Fatal(err)
	}
	run := &model.AgentRun{
		ID: "run-gap", WorkspaceID: "workspace-1", AgentID: "agent-custom",
		TargetType: "support_coverage_gap", TargetID: "gap-1",
		Status: model.AgentRunStatusRunning, OutputSummary: summary,
	}
	repo := &fakeAgentRuntimeProjectionRunRepo{byID: map[string]*model.AgentRun{run.ID: run}}
	svc := &AgentRuntimeProjectionService{
		runRepo: repo,
		now:     time.Now,
	}

	err = svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "runtime-gap", HostRunID: run.ID, Type: agentruntime.EventRunCompleted,
	})
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if run.Status != model.AgentRunStatusCompleted {
		t.Fatalf("expected durable coverage run to complete, got %q (%v)", run.Status, run.ErrorMessage)
	}
}

func TestHasDurableSupportCoverageGapOutcomeAcceptsLegacyProposal(t *testing.T) {
	summary := json.RawMessage(`{"support_coverage_gap_outcome":{"outcome":"proposal_submitted","document_id":"doc-legacy","proposal_id":"proposal-legacy","summary":"Submitted for review."}}`)
	if !hasDurableSupportCoverageGapOutcome(summary) {
		t.Fatal("expected an in-flight legacy proposal outcome to remain valid")
	}
}

func TestAgentRuntimeProjectionRecoversReviewReadyCoverageDocumentAcrossRuntimeAuditFormats(t *testing.T) {
	tests := []struct {
		name   string
		output json.RawMessage
	}{
		{
			name:   "native sdk output envelope",
			output: json.RawMessage(`{"runtime_kind":"native_sdk","output":"{\"id\":\"doc-created\",\"status\":\"draft\"}"}`),
		},
		{
			name:   "codex dynamic tool result envelope",
			output: json.RawMessage(`{"runtime_kind":"codex","result":{"success":true,"contentItems":[{"type":"inputText","text":"{\"id\":\"doc-created\",\"status\":\"draft\"}"}]}}`),
		},
		{
			name:   "codex mcp result envelope",
			output: json.RawMessage(`{"runtime_kind":"codex","result":{"id":"doc-created","status":"draft"}}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := &model.AgentRun{
				ID: "run-gap", WorkspaceID: "workspace-1", AgentID: "agent-custom",
				TargetType: "support_coverage_gap", TargetID: "gap-1",
				Status: model.AgentRunStatusRunning,
			}
			repo := &fakeAgentRuntimeProjectionRunRepo{byID: map[string]*model.AgentRun{run.ID: run}}
			runtimeClient := &fakeAgentRuntimeSignalClient{toolCalls: map[string][]AgentRuntimeToolCall{
				"runtime-gap": {{
					ToolName: "mcp__helpin__create_document",
					Output:   tt.output,
				}},
			}}
			svc := &AgentRuntimeProjectionService{
				runRepo:            repo,
				agentRuntimeClient: runtimeClient,
				now:                time.Now,
			}

			err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
				RunID: "runtime-gap", HostRunID: run.ID, Type: agentruntime.EventRunCompleted,
			})
			if err != nil {
				t.Fatalf("ApplyEvent returned error: %v", err)
			}
			if run.Status != model.AgentRunStatusCompleted {
				t.Fatalf("expected durable document recovery to complete the run, got %q (%v)", run.Status, run.ErrorMessage)
			}
			var body map[string]json.RawMessage
			if err := json.Unmarshal(run.OutputSummary, &body); err != nil {
				t.Fatalf("decode recovered summary: %v", err)
			}
			var outcome supportCoverageGapOutcomeSummary
			if err := json.Unmarshal(body[supportCoverageGapOutcomeSummaryKey], &outcome); err != nil {
				t.Fatalf("decode recovered outcome: %v", err)
			}
			if outcome.Outcome != SupportCoverageAgentOutcomeReviewReady || outcome.Action != SupportCoverageAgentActionDocumentCreated || outcome.DocumentID != "doc-created" || !outcome.Recovered {
				t.Fatalf("unexpected recovered outcome: %#v", outcome)
			}
		})
	}
}

func TestFirstRuntimeToolCallStringReadsNativeAndCodexInputs(t *testing.T) {
	tests := []struct {
		name string
		raw  json.RawMessage
	}{
		{name: "native sdk direct input", raw: json.RawMessage(`{"document_id":"doc-native"}`)},
		{name: "codex arguments envelope", raw: json.RawMessage(`{"runtime_kind":"codex","arguments":{"document_id":"doc-native"}}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstRuntimeToolCallString(tt.raw, "document_id"); got != "doc-native" {
				t.Fatalf("document_id = %q, want doc-native", got)
			}
		})
	}
}

func TestAgentRuntimeProjectionStoresWorkspacePreparedBranchSyncSummary(t *testing.T) {
	run := &model.AgentRun{
		ID:            "run-helpin",
		WorkspaceID:   "ws-1",
		AgentID:       "agent-1",
		Status:        model.AgentRunStatusQueued,
		PauseReason:   model.AgentRunPauseReasonNone,
		OutputSummary: json.RawMessage(`{"existing":true}`),
	}
	repo := &fakeAgentRuntimeProjectionRunRepo{byID: map[string]*model.AgentRun{run.ID: run}}
	svc := &AgentRuntimeProjectionService{runRepo: repo, now: time.Now}

	err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		Type:      "workspace.prepared",
		RunID:     "run-runtime",
		HostRunID: run.ID,
		Data: map[string]any{
			"metadata": map[string]any{
				"repository_id":              "repo-1",
				"repo_full_name":             "usermaven/events-pipeline",
				"base_branch":                "main",
				"work_branch":                "use-100",
				"branch_sync_status":         "conflicted",
				"branch_sync_conflict_files": []any{"Dockerfile"},
			},
		},
	})
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	var summary map[string]any
	if err := json.Unmarshal(run.OutputSummary, &summary); err != nil {
		t.Fatalf("decode output summary: %v", err)
	}
	repository, _ := summary["repository"].(map[string]any)
	if repository["repo_full_name"] != "usermaven/events-pipeline" || repository["work_branch"] != "use-100" {
		t.Fatalf("repository summary not projected: %s", string(run.OutputSummary))
	}
	branchSync, _ := repository["branch_sync"].(map[string]any)
	if branchSync["branch_sync_status"] != "conflicted" {
		t.Fatalf("branch sync summary not projected: %s", string(run.OutputSummary))
	}
	if repo.updates != 1 || repo.notifications != 1 {
		t.Fatalf("expected update and notify, got updates=%d notifications=%d", repo.updates, repo.notifications)
	}
}

func TestAgentRuntimeProjectionPausedRunBackfillsAndPublishesPendingInteraction(t *testing.T) {
	now := time.Date(2026, 7, 3, 8, 0, 0, 0, time.UTC)
	run := &model.AgentRun{
		ID:                "helpin-run-approval",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		RuntimeKind:       "native_sdk",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_approval"),
	}
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_approval": run},
	}
	interactionRepo := &fakeAgentRuntimeProjectionInteractionRepo{}
	publisher := &fakeAgentRuntimeProjectionPublisher{}
	runtimeClient := &fakeAgentRuntimeSignalClient{
		interactions: map[string][]AgentRuntimeInteraction{
			"run_runtime_approval": {{
				ID:              "runtime-interaction-1",
				RuntimeKind:     "native_sdk",
				InteractionKind: "approval_request",
				Status:          model.AgentRunInteractionStatusPending,
				Title:           "Approve tool call",
				Summary:         "Approve fetch_branch for this agent run.",
				RequestPayload:  json.RawMessage(`{"request_schema":"approval_request_v1","title":"Approve tool call","summary":"Approve fetch_branch for this agent run."}`),
				CreatedAt:       now,
				UpdatedAt:       now,
			}},
		},
	}
	svc := &AgentRuntimeProjectionService{
		runRepo:            runRepo,
		interactionRepo:    interactionRepo,
		agentRuntimeClient: runtimeClient,
		wsPublisher:        publisher,
		now:                func() time.Time { return now },
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID:  "run_runtime_approval",
		Type:   "run.paused",
		SentAt: now,
		Data:   map[string]any{"pause_reason": model.AgentRunPauseReasonHumanApproval},
	}); err != nil {
		t.Fatalf("ApplyEvent paused returned error: %v", err)
	}

	if run.Status != model.AgentRunStatusPaused || run.PauseReason != model.AgentRunPauseReasonHumanApproval {
		t.Fatalf("expected paused/human_approval, got %s/%s", run.Status, run.PauseReason)
	}
	if len(runtimeClient.listInteractionCalls) != 1 || runtimeClient.listInteractionCalls[0] != "run_runtime_approval" {
		t.Fatalf("expected paused event to backfill runtime interactions, got %#v", runtimeClient.listInteractionCalls)
	}
	if interactionRepo.creates != 1 || len(interactionRepo.interactions) != 1 {
		t.Fatalf("expected pending interaction mirror, creates=%d interactions=%#v", interactionRepo.creates, interactionRepo.interactions)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("expected live interaction websocket event, got %#v", publisher.events)
	}
	var event model.CodingSessionEvent
	if err := json.Unmarshal(publisher.events[0].Data, &event); err != nil {
		t.Fatalf("decode interaction websocket event: %v", err)
	}
	if event.Type != "interaction.requested" || event.Payload["interaction_kind"] != model.AgentRunInteractionKindApprovalRequest {
		t.Fatalf("unexpected interaction event: %#v", event)
	}
}

func TestAgentRuntimeProjectionPreservesCodexNativeApprovalKinds(t *testing.T) {
	tests := []struct {
		name             string
		runtimeKind      string
		responseMetadata string
		wantKind         string
	}{
		{name: "legacy command", runtimeKind: "human_approval", responseMetadata: `{"codex_request_kind":"command_execution"}`, wantKind: model.AgentRunInteractionKindCommandExecutionApproval},
		{name: "legacy file change", runtimeKind: "human_approval", responseMetadata: `{"codex_request_kind":"file_change"}`, wantKind: model.AgentRunInteractionKindFileChangeApproval},
		{name: "legacy permissions", runtimeKind: "human_approval", responseMetadata: `{"codex_request_kind":"permissions"}`, wantKind: model.AgentRunInteractionKindPermissionsApproval},
		{name: "typed file change", runtimeKind: "file_change_approval", responseMetadata: `{}`, wantKind: model.AgentRunInteractionKindFileChangeApproval},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := &model.AgentRun{ID: "run-1", WorkspaceID: "ws-1", RuntimeKind: "codex"}
			repo := &fakeAgentRuntimeProjectionInteractionRepo{}
			svc := &AgentRuntimeProjectionService{interactionRepo: repo, runRepo: &fakeAgentRuntimeProjectionRunRepo{}}
			runtimeInteraction := AgentRuntimeInteraction{
				ID:              "runtime-interaction-1",
				RuntimeKind:     "codex",
				InteractionKind: tt.runtimeKind,
				Status:          model.AgentRunInteractionStatusPending,
				RequestPayload:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1"}`),
				ResponsePayload: json.RawMessage(tt.responseMetadata),
			}
			if err := svc.upsertRuntimeInteraction(context.Background(), run, runtimeInteraction); err != nil {
				t.Fatalf("upsert pending interaction: %v", err)
			}
			if len(repo.interactions) != 1 {
				t.Fatalf("expected one interaction, got %#v", repo.interactions)
			}
			projected := repo.interactions[0]
			if projected.InteractionKind != tt.wantKind || projected.RequestSchemaVersion != model.AgentRunInteractionSchemaVersionCodexV2 {
				t.Fatalf("unexpected projected contract: %#v", projected)
			}

			runtimeInteraction.Status = model.AgentRunInteractionStatusResolved
			runtimeInteraction.ResponsePayload = json.RawMessage(`{"decision":"accept"}`)
			if err := svc.upsertRuntimeInteraction(context.Background(), run, runtimeInteraction); err != nil {
				t.Fatalf("upsert resolved interaction: %v", err)
			}
			projected = repo.interactions[0]
			if projected.InteractionKind != tt.wantKind || projected.RequestSchemaVersion != model.AgentRunInteractionSchemaVersionCodexV2 {
				t.Fatalf("resolved projection lost native contract: %#v", projected)
			}
		})
	}
}

func TestAgentRuntimeProjectionCancelsPendingInteractionsOnTerminalEvent(t *testing.T) {
	now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	run := &model.AgentRun{
		ID:                "helpin-run-terminal-interaction",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_terminal_interaction"),
	}
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_terminal_interaction": run},
	}
	interactionRepo := &fakeAgentRuntimeProjectionInteractionRepo{
		interactions: []model.AgentRunInteraction{{
			ID:                   "interaction-1",
			WorkspaceID:          run.WorkspaceID,
			RunID:                run.ID,
			RuntimeKind:          "codex",
			InteractionKind:      model.AgentRunInteractionKindCommandExecutionApproval,
			Status:               model.AgentRunInteractionStatusPending,
			RequestSchemaVersion: model.AgentRunInteractionSchemaVersionCodexV2,
			RequestPayload:       json.RawMessage(`{"command":"git push"}`),
			RuntimeMetadata:      json.RawMessage(`{}`),
			CreatedAt:            now.Add(-time.Minute),
			UpdatedAt:            now.Add(-time.Minute),
		}},
	}
	publisher := &fakeAgentRuntimeProjectionPublisher{}
	svc := &AgentRuntimeProjectionService{
		runRepo:         runRepo,
		interactionRepo: interactionRepo,
		wsPublisher:     publisher,
		now:             func() time.Time { return now },
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID:  "run_runtime_terminal_interaction",
		Type:   agentruntime.EventRunCompleted,
		SentAt: now,
		Data:   map[string]any{},
	}); err != nil {
		t.Fatalf("ApplyEvent completed returned error: %v", err)
	}

	if got := interactionRepo.interactions[0].Status; got != model.AgentRunInteractionStatusCancelled {
		t.Fatalf("interaction status = %q, want cancelled", got)
	}
	if interactionRepo.interactions[0].ResolvedAt == nil {
		t.Fatalf("expected cancelled interaction resolved_at timestamp")
	}
	var cancelledEvent *model.CodingSessionEvent
	for index := range publisher.events {
		var event model.CodingSessionEvent
		if err := json.Unmarshal(publisher.events[index].Data, &event); err != nil {
			t.Fatalf("decode interaction websocket event: %v", err)
		}
		if event.Type == "interaction.cancelled" {
			cancelledEvent = &event
			break
		}
	}
	if cancelledEvent == nil || cancelledEvent.Payload["status"] != model.AgentRunInteractionStatusCancelled {
		t.Fatalf("missing cancelled interaction event: %#v", publisher.events)
	}
}

func TestAgentRuntimeProjectionRetainsDurableTerminalStreamSnapshots(t *testing.T) {
	for _, eventType := range []string{agentruntime.EventRunCompleted, agentruntime.EventRunCancelled} {
		t.Run(eventType, func(t *testing.T) {
			now := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
			run := &model.AgentRun{
				ID:          "helpin-run-terminal-snapshot-" + strings.ReplaceAll(eventType, ".", "-"),
				WorkspaceID: "ws-1",
				AgentID:     "agent-1",
				Status:      model.AgentRunStatusRunning,
				PauseReason: model.AgentRunPauseReasonNone,
			}
			runRepo := &fakeAgentRuntimeProjectionRunRepo{byID: map[string]*model.AgentRun{run.ID: run}}
			snapshotRepo := &fakeAgentRuntimeProjectionSessionSnapshotRepo{
				record: &model.CodingSessionStateSnapshot{
					ID:          "snapshot-1",
					WorkspaceID: run.WorkspaceID,
					RunID:       run.ID,
				},
			}
			svc := &AgentRuntimeProjectionService{
				runRepo:             runRepo,
				sessionSnapshotRepo: snapshotRepo,
				eventProtocol:       "v2",
				now:                 func() time.Time { return now },
			}

			if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
				RunID:     "runtime-terminal-snapshot",
				HostRunID: run.ID,
				Type:      eventType,
				SentAt:    now,
				Data:      map[string]any{},
			}); err != nil {
				t.Fatalf("ApplyEvent(%s) returned error: %v", eventType, err)
			}

			if snapshotRepo.deletes != 0 || snapshotRepo.record == nil {
				t.Fatalf("terminal snapshot retention = deletes:%d record:%#v, want no delete and a durable record", snapshotRepo.deletes, snapshotRepo.record)
			}
		})
	}
}

func TestAgentRuntimeProjectionRetainsFailedStreamSnapshotForRecovery(t *testing.T) {
	now := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	run := &model.AgentRun{
		ID:          "helpin-run-failed-snapshot",
		WorkspaceID: "ws-1",
		AgentID:     "agent-1",
		Status:      model.AgentRunStatusRunning,
		PauseReason: model.AgentRunPauseReasonNone,
	}
	snapshotRepo := &fakeAgentRuntimeProjectionSessionSnapshotRepo{
		record: &model.CodingSessionStateSnapshot{
			ID:          "snapshot-failed",
			WorkspaceID: run.WorkspaceID,
			RunID:       run.ID,
		},
	}
	svc := &AgentRuntimeProjectionService{
		runRepo: &fakeAgentRuntimeProjectionRunRepo{
			byID: map[string]*model.AgentRun{run.ID: run},
		},
		sessionSnapshotRepo: snapshotRepo,
		now:                 func() time.Time { return now },
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID:     "runtime-failed-snapshot",
		HostRunID: run.ID,
		Type:      agentruntime.EventRunFailed,
		SentAt:    now,
		Data:      map[string]any{"error": "provider disconnected"},
	}); err != nil {
		t.Fatalf("ApplyEvent failed returned error: %v", err)
	}

	if snapshotRepo.deletes != 0 || snapshotRepo.record == nil {
		t.Fatalf("failed snapshot should be retained, got deletes:%d record:%#v", snapshotRepo.deletes, snapshotRepo.record)
	}
}

func TestAgentRuntimeProjectionMirrorsAssistantMessageCompletedIdempotently(t *testing.T) {
	run := &model.AgentRun{
		ID:                "helpin-run-message",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_message"),
	}
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byID:       map[string]*model.AgentRun{run.ID: run},
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_message": run},
	}
	messageRepo := &fakeAgentRuntimeProjectionMessageRepo{}
	svc := &AgentRuntimeProjectionService{
		runRepo:        runRepo,
		runMessageRepo: messageRepo,
		now:            time.Now,
	}
	event := AgentRuntimeEventEnvelope{
		RunID:     "run_runtime_message",
		HostRunID: run.ID,
		Type:      "assistant_message_completed",
		Data: map[string]any{
			"message_id": "runtime-message-1",
			"content":    "Done with the task.",
		},
	}
	if err := svc.ApplyEvent(context.Background(), event); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if err := svc.ApplyEvent(context.Background(), event); err != nil {
		t.Fatalf("ApplyEvent redelivery returned error: %v", err)
	}
	if messageRepo.creates != 1 || len(messageRepo.messages) != 1 {
		t.Fatalf("expected exactly one mirrored message, creates=%d messages=%#v", messageRepo.creates, messageRepo.messages)
	}
	message := messageRepo.messages[0]
	if message.Role != "assistant" || message.MessageType != "assistant_turn" || message.Content != "Done with the task." || message.SequenceNo != 1 {
		t.Fatalf("unexpected mirrored message: %#v", message)
	}
	if !agentRunMessageHasRuntimeMessageID(message, "runtime-message-1") {
		t.Fatalf("mirrored message missing runtime id in content blocks: %s", string(message.ContentBlocks))
	}
}

func TestAgentRuntimeProjectionMirrorsAssistantMessageCompletedWithEventTimestamp(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	sentAt := now.Add(-45 * time.Minute)
	run := &model.AgentRun{
		ID:                "helpin-run-message-time",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_message_time"),
	}
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byID:       map[string]*model.AgentRun{run.ID: run},
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_message_time": run},
	}
	messageRepo := &fakeAgentRuntimeProjectionMessageRepo{}
	svc := &AgentRuntimeProjectionService{
		runRepo:        runRepo,
		runMessageRepo: messageRepo,
		now:            func() time.Time { return now },
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID:     "run_runtime_message_time",
		HostRunID: run.ID,
		Type:      "assistant_message_completed",
		SentAt:    sentAt,
		Data: map[string]any{
			"message_id": "runtime-message-time-1",
			"content":    "Replayed reply.",
		},
	}); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(messageRepo.messages) != 1 {
		t.Fatalf("expected one mirrored message, got %d", len(messageRepo.messages))
	}
	if got := messageRepo.messages[0].CreatedAt; !got.Equal(sentAt) {
		t.Fatalf("mirrored message CreatedAt = %v, want event sent_at %v", got, sentAt)
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID:     "run_runtime_message_time",
		HostRunID: run.ID,
		Type:      "assistant_message_completed",
		Data: map[string]any{
			"message_id": "runtime-message-time-2",
			"content":    "Live reply.",
		},
	}); err != nil {
		t.Fatalf("ApplyEvent without sent_at returned error: %v", err)
	}
	if len(messageRepo.messages) != 2 {
		t.Fatalf("expected two mirrored messages, got %d", len(messageRepo.messages))
	}
	if got := messageRepo.messages[1].CreatedAt; !got.Equal(now) {
		t.Fatalf("fallback CreatedAt = %v, want projection now %v", got, now)
	}
}

func TestAgentRuntimeProjectionPersistsCodingSessionStreamSnapshot(t *testing.T) {
	run := &model.AgentRun{
		ID:                "helpin-run-stream",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_stream"),
	}
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_stream": run},
	}
	snapshotRepo := &fakeAgentRuntimeProjectionSessionSnapshotRepo{}
	publisher := &fakeAgentRuntimeProjectionPublisher{}
	svc := &AgentRuntimeProjectionService{
		runRepo:             runRepo,
		sessionSnapshotRepo: snapshotRepo,
		wsPublisher:         publisher,
		eventProtocol:       "v2",
		now:                 time.Now,
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		EventID:    "event-assistant-started",
		RunID:      "run_runtime_stream",
		SequenceNo: 1,
		Type:       "assistant_message_started",
		Data:       map[string]any{"message_id": "runtime-message-1"},
	}); err != nil {
		t.Fatalf("ApplyEvent started returned error: %v", err)
	}
	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		EventID:    "event-assistant-delta",
		RunID:      "run_runtime_stream",
		SequenceNo: 2,
		Type:       "assistant_message_delta",
		Data: map[string]any{
			"message_id": "runtime-message-1",
			"content":    "Working through the task.",
		},
	}); err != nil {
		t.Fatalf("ApplyEvent delta returned error: %v", err)
	}
	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		EventID:    "event-assistant-delta",
		RunID:      "run_runtime_stream",
		SequenceNo: 2,
		Type:       "assistant_message_delta",
		Data: map[string]any{
			"message_id": "runtime-message-1",
			"content":    "Working through the task.",
		},
	}); err != nil {
		t.Fatalf("ApplyEvent duplicate delta returned error: %v", err)
	}

	if snapshotRepo.upserts != 2 || snapshotRepo.record == nil {
		t.Fatalf("expected two snapshot upserts, got upserts=%d record=%#v", snapshotRepo.upserts, snapshotRepo.record)
	}
	snapshot, err := model.DecodeCodingSessionStreamSnapshot(snapshotRepo.record.SnapshotPayload)
	if err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if snapshot.LiveAssistantMessage == nil || snapshot.LiveAssistantMessage.Content != "Working through the task." || snapshot.LiveAssistantMessage.Status != "streaming" {
		t.Fatalf("unexpected live assistant message: %#v", snapshot.LiveAssistantMessage)
	}
	if snapshot.ThroughSequence != 2 {
		t.Fatalf("unexpected v2 snapshot watermark: %d", snapshot.ThroughSequence)
	}
	if len(snapshot.LiveTurnSegments) != 1 || snapshot.LiveTurnSegments[0].AssistantMessage == nil || snapshot.LiveTurnSegments[0].AssistantMessage.Content != "Working through the task." {
		t.Fatalf("unexpected live turn segments: %#v", snapshot.LiveTurnSegments)
	}
	if runRepo.notifications != 0 {
		t.Fatalf("expected snapshot updates to avoid generic run notifications, got %d", runRepo.notifications)
	}
	if len(publisher.events) != 2 {
		t.Fatalf("expected live websocket events, got %#v", publisher.events)
	}
	if publisher.events[1].Entity != "coding_session_event" || publisher.events[1].ParentID != run.ID {
		t.Fatalf("unexpected websocket event: %#v", publisher.events[1])
	}
	var liveEvent model.CodingSessionEvent
	if err := json.Unmarshal(publisher.events[1].Data, &liveEvent); err != nil {
		t.Fatalf("decode live websocket event: %v", err)
	}
	if liveEvent.Type != "assistant.message.delta" || liveEvent.Payload["content"] != "Working through the task." {
		t.Fatalf("unexpected live event payload: %#v", liveEvent)
	}
	if liveEvent.RuntimeMetadata["source"] != "agent-runtime-v2" {
		t.Fatalf("unexpected live event source: %#v", liveEvent.RuntimeMetadata)
	}
}

func TestAgentRuntimeProjectionDoesNotPublishStaleStreamEvent(t *testing.T) {
	run := &model.AgentRun{
		ID:                "helpin-run-stale-stream",
		WorkspaceID:       "ws-1",
		Status:            model.AgentRunStatusRunning,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_stale_stream"),
	}
	payload, err := model.EncodeCodingSessionStreamSnapshot(&model.CodingSessionStreamSnapshot{
		ThroughSequence: 10,
		LiveAssistantMessage: &model.CodingSessionLiveAssistantMessage{
			MessageID: "assistant-1",
			Content:   "newer state",
			Status:    "streaming",
		},
	})
	if err != nil {
		t.Fatalf("encode snapshot: %v", err)
	}
	snapshotRepo := &fakeAgentRuntimeProjectionSessionSnapshotRepo{
		record: &model.CodingSessionStateSnapshot{
			WorkspaceID:     run.WorkspaceID,
			RunID:           run.ID,
			ThroughSequence: 10,
			SnapshotPayload: payload,
		},
	}
	publisher := &fakeAgentRuntimeProjectionPublisher{}
	svc := &AgentRuntimeProjectionService{
		runRepo: &fakeAgentRuntimeProjectionRunRepo{
			byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_stale_stream": run},
		},
		sessionSnapshotRepo: snapshotRepo,
		wsPublisher:         publisher,
		eventProtocol:       "v2",
		now:                 time.Now,
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		EventID:    "stale-event",
		RunID:      "run_runtime_stale_stream",
		SequenceNo: 9,
		Type:       "assistant_message_delta",
		Data:       map[string]any{"message_id": "assistant-1", "content": " stale"},
	}); err != nil {
		t.Fatalf("ApplyEvent stale delta returned error: %v", err)
	}
	if snapshotRepo.upserts != 0 {
		t.Fatalf("stale event caused %d snapshot upserts", snapshotRepo.upserts)
	}
	if len(publisher.events) != 0 {
		t.Fatalf("stale event was published: %#v", publisher.events)
	}
}

func TestAgentRuntimeProjectionSerializesConcurrentEventsForOneRun(t *testing.T) {
	run := &model.AgentRun{
		ID:                "helpin-run-concurrent-stream",
		WorkspaceID:       "ws-1",
		Status:            model.AgentRunStatusRunning,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_concurrent_stream"),
	}
	upsertStarted := make(chan struct{}, 1)
	releaseUpsert := make(chan struct{})
	snapshotRepo := &fakeAgentRuntimeProjectionSessionSnapshotRepo{
		upsertStarted: upsertStarted,
		releaseUpsert: releaseUpsert,
	}
	publisher := &fakeAgentRuntimeProjectionPublisher{}
	svc := &AgentRuntimeProjectionService{
		runRepo: &fakeAgentRuntimeProjectionRunRepo{
			byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_concurrent_stream": run},
		},
		sessionSnapshotRepo: snapshotRepo,
		wsPublisher:         publisher,
		eventProtocol:       "v2",
		now:                 time.Now,
	}

	firstDone := make(chan error, 1)
	go func() {
		firstDone <- svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
			EventID:    "event-1",
			RunID:      "run_runtime_concurrent_stream",
			SequenceNo: 1,
			Type:       "assistant_message_started",
			Data:       map[string]any{"message_id": "assistant-1"},
		})
	}()
	select {
	case <-upsertStarted:
	case <-time.After(time.Second):
		t.Fatal("first event did not reach snapshot upsert")
	}

	secondDone := make(chan error, 1)
	go func() {
		secondDone <- svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
			EventID:    "event-2",
			RunID:      "run_runtime_concurrent_stream",
			SequenceNo: 2,
			Type:       "assistant_message_delta",
			Data:       map[string]any{"message_id": "assistant-1", "content": "hello"},
		})
	}()
	select {
	case <-upsertStarted:
		t.Fatal("second event entered snapshot upsert while the first event was still applying")
	case <-time.After(40 * time.Millisecond):
	}
	close(releaseUpsert)
	if err := <-firstDone; err != nil {
		t.Fatalf("first ApplyEvent returned error: %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second ApplyEvent returned error: %v", err)
	}

	if len(publisher.events) != 2 {
		t.Fatalf("published events = %#v, want two", publisher.events)
	}
	for index, published := range publisher.events {
		var event model.CodingSessionEvent
		if err := json.Unmarshal(published.Data, &event); err != nil {
			t.Fatalf("decode published event %d: %v", index, err)
		}
		if event.SequenceNo != index+1 {
			t.Fatalf("published sequence at index %d = %d, want %d", index, event.SequenceNo, index+1)
		}
	}
	snapshot, err := model.DecodeCodingSessionStreamSnapshot(snapshotRepo.record.SnapshotPayload)
	if err != nil {
		t.Fatalf("decode final snapshot: %v", err)
	}
	if snapshot.ThroughSequence != 2 || snapshot.LiveAssistantMessage == nil ||
		snapshot.LiveAssistantMessage.Content != "hello" {
		t.Fatalf("final snapshot = %#v, want ordered sequence 2 state", snapshot)
	}
}

func TestRuntimeCodingSessionEventPayloadPreservesDeltaWhitespace(t *testing.T) {
	tests := []struct {
		name    string
		data    map[string]any
		wantRaw string
	}{
		{name: "leading space", data: map[string]any{"text": " world", "content": " world"}, wantRaw: " world"},
		{name: "whitespace only", data: map[string]any{"text": " ", "content": " "}, wantRaw: " "},
		{name: "content fallback", data: map[string]any{"content": " next"}, wantRaw: " next"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := runtimeCodingSessionEventPayload(AgentRuntimeEventEnvelope{Data: tt.data})
			if payload["text"] != tt.wantRaw || payload["content"] != tt.wantRaw {
				t.Fatalf("delta whitespace changed: text=%q content=%q", payload["text"], payload["content"])
			}
		})
	}
}

func TestAgentRuntimeProjectionClearsResumingStageOnRuntimeWork(t *testing.T) {
	stage := "resuming"
	run := &model.AgentRun{
		ID:                "helpin-run-resuming-work",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExecutionStage:    &stage,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_resuming_work"),
	}
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_resuming_work": run},
	}
	snapshotRepo := &fakeAgentRuntimeProjectionSessionSnapshotRepo{}
	svc := &AgentRuntimeProjectionService{
		runRepo:             runRepo,
		sessionSnapshotRepo: snapshotRepo,
		now:                 time.Now,
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_resuming_work",
		Type:  "tool_call_started",
		Data: map[string]any{
			"tool_call_id":      "tool-1",
			"tool_name":         "run_command",
			"parent_message_id": "assistant-1",
			"args_text":         `{"cmd":"git status --short"}`,
		},
	}); err != nil {
		t.Fatalf("ApplyEvent tool_call_started returned error: %v", err)
	}
	if run.ExecutionStage != nil {
		t.Fatalf("expected resuming stage to clear on runtime work, got %#v", run.ExecutionStage)
	}
	if runRepo.updates != 1 || runRepo.notifications != 1 {
		t.Fatalf("expected lifecycle update notification only, got updates=%d notifications=%d", runRepo.updates, runRepo.notifications)
	}
	if snapshotRepo.upserts != 1 {
		t.Fatalf("expected snapshot upsert, got %d", snapshotRepo.upserts)
	}
}

func TestAgentRuntimeProjectionPublishesReasoningStreamEvent(t *testing.T) {
	run := &model.AgentRun{
		ID:                "helpin-run-reasoning",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_reasoning"),
	}
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_reasoning": run},
	}
	snapshotRepo := &fakeAgentRuntimeProjectionSessionSnapshotRepo{}
	publisher := &fakeAgentRuntimeProjectionPublisher{}
	svc := &AgentRuntimeProjectionService{
		runRepo:             runRepo,
		sessionSnapshotRepo: snapshotRepo,
		wsPublisher:         publisher,
		now:                 time.Now,
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		EventID: "event-reasoning-delta",
		RunID:   "run_runtime_reasoning",
		Type:    "reasoning_message_delta",
		Data: map[string]any{
			"message_id": "reasoning-1",
			"content":    "Checking repo state.",
		},
	}); err != nil {
		t.Fatalf("ApplyEvent reasoning returned error: %v", err)
	}
	if snapshotRepo.upserts != 1 || snapshotRepo.record == nil {
		t.Fatalf("expected reasoning snapshot upsert, got upserts=%d record=%#v", snapshotRepo.upserts, snapshotRepo.record)
	}
	snapshot, err := model.DecodeCodingSessionStreamSnapshot(snapshotRepo.record.SnapshotPayload)
	if err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if snapshot.LiveReasoningMessage == nil || snapshot.LiveReasoningMessage.Content != "Checking repo state." {
		t.Fatalf("unexpected reasoning snapshot: %#v", snapshot.LiveReasoningMessage)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("expected one reasoning websocket event, got %#v", publisher.events)
	}
	var liveEvent model.CodingSessionEvent
	if err := json.Unmarshal(publisher.events[0].Data, &liveEvent); err != nil {
		t.Fatalf("decode reasoning websocket event: %v", err)
	}
	if liveEvent.Type != "reasoning.message.delta" || liveEvent.Payload["content"] != "Checking repo state." {
		t.Fatalf("unexpected reasoning event: %#v", liveEvent)
	}
}

func TestAgentRuntimeProjectionCancelsOnCumulativeUsageOverage(t *testing.T) {
	run := &model.AgentRun{
		ID:                "helpin-run-overage",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_overage"),
	}
	repo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_overage": run},
	}
	usageConsumer := &fakeAgentRuntimeProjectionUsageConsumer{preflightErr: model.ErrAIUsageExhausted}
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := &AgentRuntimeProjectionService{
		runRepo: repo,
		agentRepo: &fakeAgentRuntimeProjectionAgentRepo{agent: &model.Agent{
			ID:        "agent-1",
			PresetKey: model.AgentPresetCodeBuilder,
			IsSystem:  true,
		}},
		usageMeter:         &AIUsageMeter{consumer: usageConsumer},
		agentRuntimeClient: runtimeClient,
		now:                time.Now,
	}

	err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_overage",
		Type:  "usage.checkpoint",
		Data: map[string]any{
			"usage": map[string]any{
				"total_tokens":        float64(12000),
				"input_tokens":        float64(10000),
				"output_tokens":       float64(2000),
				"cached_input_tokens": float64(0),
			},
			"usage_semantic": "cumulative",
		},
	})
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(runtimeClient.cancelCalls) != 1 || runtimeClient.cancelCalls[0] != "run_runtime_overage" {
		t.Fatalf("expected runtime cancel call, got %#v", runtimeClient.cancelCalls)
	}
	if run.ExecutionStage == nil || *run.ExecutionStage != agentRuntimeExecutionStageUsageOverageCancel {
		t.Fatalf("expected overage cancel stage, got %#v", run.ExecutionStage)
	}
	if run.Status != model.AgentRunStatusRunning {
		t.Fatalf("expected status to remain projection-owned running, got %q", run.Status)
	}
	if len(usageConsumer.preflightInputs) != 1 || usageConsumer.preflightInputs[0].FeatureKey != BillingFeatureForgeRun {
		t.Fatalf("expected Forge preflight input, got %#v", usageConsumer.preflightInputs)
	}
	if repo.updates != 1 || repo.notifications != 1 {
		t.Fatalf("expected one update/notify, got %d/%d", repo.updates, repo.notifications)
	}
}

func TestAgentRuntimeProjectionDoesNotCancelOverageForNonCumulativeUsage(t *testing.T) {
	run := &model.AgentRun{
		ID:                "helpin-run-delta",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_delta"),
	}
	repo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_delta": run},
	}
	usageConsumer := &fakeAgentRuntimeProjectionUsageConsumer{preflightErr: model.ErrAIUsageExhausted}
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := &AgentRuntimeProjectionService{
		runRepo:            repo,
		agentRepo:          &fakeAgentRuntimeProjectionAgentRepo{agent: &model.Agent{ID: "agent-1"}},
		usageMeter:         &AIUsageMeter{consumer: usageConsumer},
		agentRuntimeClient: runtimeClient,
		now:                time.Now,
	}

	err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_delta",
		Type:  "usage.checkpoint",
		Data: map[string]any{
			"usage": map[string]any{
				"input_tokens":  float64(10000),
				"output_tokens": float64(2000),
			},
			"usage_semantic": "delta",
		},
	})
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(runtimeClient.cancelCalls) != 0 {
		t.Fatalf("expected no runtime cancel calls, got %#v", runtimeClient.cancelCalls)
	}
	if len(usageConsumer.preflightInputs) != 0 {
		t.Fatalf("expected no preflight for non-cumulative checkpoint, got %#v", usageConsumer.preflightInputs)
	}
}

func TestAgentRuntimeProjectionConsumesTerminalUsageOnce(t *testing.T) {
	completedAt := time.Date(2026, 7, 2, 14, 0, 0, 0, time.UTC)
	run := &model.AgentRun{
		ID:                "helpin-run-consume",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_consume"),
		OutputSummary:     json.RawMessage(`{}`),
	}
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_consume": run},
	}
	usageConsumer := &fakeAgentRuntimeProjectionUsageConsumer{}
	svc := &AgentRuntimeProjectionService{
		runRepo: runRepo,
		agentRepo: &fakeAgentRuntimeProjectionAgentRepo{agent: &model.Agent{
			ID:        "agent-1",
			PresetKey: model.AgentPresetCodeBuilder,
			IsSystem:  true,
		}},
		usageMeter: &AIUsageMeter{consumer: usageConsumer},
		now:        func() time.Time { return completedAt },
	}
	event := AgentRuntimeEventEnvelope{
		RunID:  "run_runtime_consume",
		Type:   "run.completed",
		SentAt: completedAt,
		Data: map[string]any{
			"usage": map[string]any{
				"total_tokens":            float64(30),
				"input_tokens":            float64(10),
				"cached_input_tokens":     float64(2),
				"output_tokens":           float64(4),
				"reasoning_output_tokens": float64(1),
			},
			"usage_semantic": "cumulative",
		},
	}
	if err := svc.ApplyEvent(context.Background(), event); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if err := svc.ApplyEvent(context.Background(), event); err != nil {
		t.Fatalf("ApplyEvent redelivery returned error: %v", err)
	}
	if len(usageConsumer.consumeInputs) != 1 {
		t.Fatalf("expected one terminal usage consumption, got %#v", usageConsumer.consumeInputs)
	}
	input := usageConsumer.consumeInputs[0]
	if input.WorkspaceID != "ws-1" || input.FeatureKey != BillingFeatureForgeRun || input.IdempotencyKey != "ws-1:agent-runtime:helpin-run-consume:terminal-usage" {
		t.Fatalf("unexpected consumption input: %#v", input)
	}
	if !runtimeUsageAlreadyConsumed(run.OutputSummary) {
		t.Fatalf("expected output summary marker, got %s", string(run.OutputSummary))
	}
}

func TestAgentRuntimeProjectionTerminalUsageFailureDoesNotBlockStatusProjection(t *testing.T) {
	completedAt := time.Date(2026, 7, 2, 14, 30, 0, 0, time.UTC)
	run := &model.AgentRun{
		ID:                "helpin-run-consume-failure",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_consume_failure"),
		OutputSummary:     json.RawMessage(`{}`),
	}
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_consume_failure": run},
	}
	usageConsumer := &fakeAgentRuntimeProjectionUsageConsumer{consumeErr: model.ErrBillingWorkspaceLocked}
	svc := &AgentRuntimeProjectionService{
		runRepo: runRepo,
		agentRepo: &fakeAgentRuntimeProjectionAgentRepo{agent: &model.Agent{
			ID:        "agent-1",
			PresetKey: model.AgentPresetCodeBuilder,
			IsSystem:  true,
		}},
		usageMeter: &AIUsageMeter{consumer: usageConsumer},
		now:        func() time.Time { return completedAt },
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID:  "run_runtime_consume_failure",
		Type:   "run.completed",
		SentAt: completedAt,
		Data: map[string]any{
			"usage": map[string]any{
				"total_tokens":  float64(18),
				"input_tokens":  float64(12),
				"output_tokens": float64(6),
			},
			"usage_semantic": "cumulative",
		},
	}); !errors.Is(err, model.ErrBillingWorkspaceLocked) {
		t.Fatalf("expected retryable settlement error, got: %v", err)
	}
	if run.Status != model.AgentRunStatusCompleted || run.CompletedAt == nil || !run.CompletedAt.Equal(completedAt) {
		t.Fatalf("terminal status not projected after consume failure: status=%s completed_at=%v", run.Status, run.CompletedAt)
	}
	if run.InputTokens != 12 || run.OutputTokens != 6 || run.TokensUsed != 18 {
		t.Fatalf("usage counters not projected after consume failure: input=%d output=%d total=%d", run.InputTokens, run.OutputTokens, run.TokensUsed)
	}
	if runtimeUsageAlreadyConsumed(run.OutputSummary) {
		t.Fatalf("consume marker should remain unset after failed consume, got %s", string(run.OutputSummary))
	}
	if runRepo.updates != 1 || runRepo.notifications != 1 {
		t.Fatalf("expected status update/notify despite consume failure, got %d/%d", runRepo.updates, runRepo.notifications)
	}
}

func TestAgentRuntimeProjectionDoesNotRegressTerminalRunOnLatePreTerminalEvent(t *testing.T) {
	completedAt := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	run := &model.AgentRun{
		ID:                "helpin-run-terminal",
		Status:            model.AgentRunStatusCompleted,
		PauseReason:       model.AgentRunPauseReasonNone,
		CompletedAt:       &completedAt,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_terminal"),
	}
	repo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_terminal": run},
	}
	svc := &AgentRuntimeProjectionService{runRepo: repo, now: time.Now}

	err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID:  "run_runtime_terminal",
		Type:   "run.started",
		SentAt: completedAt.Add(-time.Minute),
	})
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if run.Status != model.AgentRunStatusCompleted || run.CompletedAt == nil || !run.CompletedAt.Equal(completedAt) {
		t.Fatalf("terminal run regressed: status=%s completed_at=%v", run.Status, run.CompletedAt)
	}
	if repo.updates != 0 || repo.notifications != 0 {
		t.Fatalf("expected no update/notify for late pre-terminal event, got %d/%d", repo.updates, repo.notifications)
	}

	err = svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID:  "run_runtime_terminal",
		Type:   "run.resumed",
		SentAt: completedAt.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("ApplyEvent resumed returned error: %v", err)
	}
	if run.Status != model.AgentRunStatusCompleted || run.CompletedAt == nil || !run.CompletedAt.Equal(completedAt) {
		t.Fatalf("terminal run regressed after run.resumed: status=%s completed_at=%v", run.Status, run.CompletedAt)
	}
	if repo.updates != 0 || repo.notifications != 0 {
		t.Fatalf("expected no update/notify for late run.resumed, got %d/%d", repo.updates, repo.notifications)
	}
}

func TestAgentRuntimeProjectionAppliesCumulativeUsage(t *testing.T) {
	run := &model.AgentRun{
		ID:                "helpin-run-2",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_2"),
	}
	repo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_2": run},
	}
	svc := &AgentRuntimeProjectionService{
		runRepo: repo,
		now:     time.Now,
	}

	err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_2",
		Type:  "usage.checkpoint",
		Data: map[string]any{
			"usage": map[string]any{
				"total_tokens":            float64(25),
				"input_tokens":            float64(12),
				"cached_input_tokens":     float64(4),
				"output_tokens":           float64(8),
				"reasoning_output_tokens": float64(5),
			},
			"usage_semantic": "cumulative",
		},
	})
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if run.InputTokens != 12 || run.CachedInputTokens != 4 || run.OutputTokens != 8 || run.TokensUsed != 25 {
		t.Fatalf("usage not projected: input=%d cached=%d output=%d total=%d", run.InputTokens, run.CachedInputTokens, run.OutputTokens, run.TokensUsed)
	}
	if repo.updates != 1 || repo.notifications != 1 {
		t.Fatalf("expected one update/notify, got %d/%d", repo.updates, repo.notifications)
	}
}

func TestAgentRuntimeProjectionUsageFallbackDoesNotDoubleCountUsageDetails(t *testing.T) {
	run := &model.AgentRun{
		ID:                "helpin-run-usage",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_usage"),
	}
	repo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_usage": run},
	}
	svc := &AgentRuntimeProjectionService{runRepo: repo, now: time.Now}

	err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_usage",
		Type:  "usage.checkpoint",
		Data: map[string]any{
			"usage": map[string]any{
				"input_tokens":            float64(12),
				"cached_input_tokens":     float64(4),
				"output_tokens":           float64(8),
				"reasoning_output_tokens": float64(5),
			},
			"usage_semantic": "cumulative",
		},
	})
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if run.TokensUsed != 20 {
		t.Fatalf("expected fallback total to use input plus output, got %d", run.TokensUsed)
	}
}

func TestAgentRuntimeProjectionReconcileMappedRunsAppliesFetchedRuntimeState(t *testing.T) {
	completedAt := time.Date(2026, 7, 2, 12, 30, 0, 0, time.UTC)
	run := &model.AgentRun{
		ID:                "helpin-run-reconcile",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_reconcile"),
	}
	repo := &fakeAgentRuntimeProjectionRunRepo{
		byID:       map[string]*model.AgentRun{run.ID: run},
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_reconcile": run},
		active:     []model.AgentRun{*run},
	}
	runtimeClient := &fakeAgentRuntimeSignalClient{
		getRuns: map[string]*AgentRuntimeRun{
			"run_runtime_reconcile": {
				ID:            "run_runtime_reconcile",
				AppID:         "helpin",
				HostRunID:     run.ID,
				Status:        model.AgentRunStatusCompleted,
				CompletedAt:   &completedAt,
				OutputSummary: json.RawMessage(`{"input_tokens":12,"cached_input_tokens":3,"output_tokens":8}`),
			},
		},
	}
	svc := &AgentRuntimeProjectionService{
		runRepo:            repo,
		agentRuntimeClient: runtimeClient,
		now:                func() time.Time { return completedAt.Add(time.Minute) },
	}

	if err := svc.ReconcileMappedRuns(context.Background(), time.Minute, 10); err != nil {
		t.Fatalf("ReconcileMappedRuns returned error: %v", err)
	}
	if len(runtimeClient.getCalls) != 1 || runtimeClient.getCalls[0] != "run_runtime_reconcile" {
		t.Fatalf("expected runtime get call, got %#v", runtimeClient.getCalls)
	}
	if run.Status != model.AgentRunStatusCompleted || run.CompletedAt == nil || !run.CompletedAt.Equal(completedAt) {
		t.Fatalf("expected completed run from reconciliation, got status=%s completed_at=%v", run.Status, run.CompletedAt)
	}
	if run.InputTokens != 12 || run.CachedInputTokens != 3 || run.OutputTokens != 8 || run.TokensUsed != 20 {
		t.Fatalf("expected usage from runtime summary, got input=%d cached=%d output=%d total=%d", run.InputTokens, run.CachedInputTokens, run.OutputTokens, run.TokensUsed)
	}
}

func TestAgentRuntimeProjectionReconcileRetriesStaleCancellation(t *testing.T) {
	now := time.Date(2026, 8, 14, 15, 10, 0, 0, time.UTC)
	run := &model.AgentRun{
		ID:                "helpin-run-cancelling",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExecutionStage:    stringPointer("cancelling"),
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_cancelling"),
	}
	repo := &fakeAgentRuntimeProjectionRunRepo{
		byID:       map[string]*model.AgentRun{run.ID: run},
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_cancelling": run},
		active:     []model.AgentRun{*run},
	}
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := &AgentRuntimeProjectionService{
		runRepo:            repo,
		agentRuntimeClient: runtimeClient,
		now:                func() time.Time { return now },
	}

	if err := svc.ReconcileMappedRuns(context.Background(), time.Minute, 10); err != nil {
		t.Fatalf("ReconcileMappedRuns returned error: %v", err)
	}
	if len(runtimeClient.cancelCalls) != 1 || runtimeClient.cancelCalls[0] != "run_runtime_cancelling" {
		t.Fatalf("expected stale cancellation to be retried, got %#v", runtimeClient.cancelCalls)
	}
	if len(runtimeClient.getCalls) != 0 {
		t.Fatalf("expected cancellation reconciliation to skip the ordinary status fetch, got %#v", runtimeClient.getCalls)
	}
	if run.Status != model.AgentRunStatusCancelled || run.CompletedAt == nil {
		t.Fatalf("expected stale cancelling run to become cancelled, got status=%s completed_at=%v", run.Status, run.CompletedAt)
	}
}

func TestAgentRuntimeProjectionReconcileReplaysMissedV2EventsFromCursor(t *testing.T) {
	now := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	run := &model.AgentRun{
		ID:                "helpin-run-v2-replay",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		Status:            model.AgentRunStatusQueued,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("runtime-run-v2-replay"),
	}
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byID:       map[string]*model.AgentRun{run.ID: run},
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|runtime-run-v2-replay": run},
		active:     []model.AgentRun{*run},
	}
	runtimeClient := &fakeAgentRuntimeSignalClient{
		getRuns: map[string]*AgentRuntimeRun{
			"runtime-run-v2-replay": {
				ID:        "runtime-run-v2-replay",
				AppID:     "helpin",
				HostRunID: run.ID,
				Status:    model.AgentRunStatusRunning,
				UpdatedAt: now,
			},
		},
		v2Events: map[string][]AgentRuntimeEventEnvelope{
			"runtime-run-v2-replay": {
				{AppID: "helpin", RunID: "runtime-run-v2-replay", HostRunID: run.ID, Type: agentruntime.EventRunStarted, SequenceNo: 1, SentAt: now},
				{AppID: "helpin", RunID: "runtime-run-v2-replay", HostRunID: run.ID, Type: agentruntime.EventAssistantMessageStarted, SequenceNo: 2, SentAt: now, Data: map[string]any{"message_id": "assistant-1"}},
				{AppID: "helpin", RunID: "runtime-run-v2-replay", HostRunID: run.ID, Type: agentruntime.EventAssistantMessageDelta, SequenceNo: 3, SentAt: now, Data: map[string]any{"message_id": "assistant-1", "text": "hello"}},
			},
		},
	}
	snapshotRepo := &fakeAgentRuntimeProjectionSessionSnapshotRepo{}
	svc := &AgentRuntimeProjectionService{
		runRepo:             runRepo,
		agentRuntimeClient:  runtimeClient,
		sessionSnapshotRepo: snapshotRepo,
		eventProtocol:       "v2",
		now:                 func() time.Time { return now.Add(time.Minute) },
	}

	if err := svc.ReconcileMappedRuns(context.Background(), time.Second, 10); err != nil {
		t.Fatalf("first ReconcileMappedRuns returned error: %v", err)
	}
	if runtimeV2ReplayCursor(run.OutputSummary) != 3 {
		t.Fatalf("expected durable replay cursor 3, summary=%s", string(run.OutputSummary))
	}
	restartedSvc := &AgentRuntimeProjectionService{
		runRepo:             runRepo,
		agentRuntimeClient:  runtimeClient,
		sessionSnapshotRepo: snapshotRepo,
		eventProtocol:       "v2",
		now:                 func() time.Time { return now.Add(time.Minute) },
	}
	if err := restartedSvc.ReconcileMappedRuns(context.Background(), time.Second, 10); err != nil {
		t.Fatalf("second ReconcileMappedRuns returned error: %v", err)
	}
	if run.Status != model.AgentRunStatusRunning {
		t.Fatalf("expected replay to advance queued run, got %s", run.Status)
	}
	if len(runtimeClient.listV2EventCalls) != 2 || runtimeClient.listV2EventCalls[0] != 0 || runtimeClient.listV2EventCalls[1] != 3 {
		t.Fatalf("expected incremental replay cursors [0 3], got %#v", runtimeClient.listV2EventCalls)
	}
	if snapshotRepo.record == nil {
		t.Fatal("expected replayed stream snapshot")
	}
	snapshot, err := model.DecodeCodingSessionStreamSnapshot(snapshotRepo.record.SnapshotPayload)
	if err != nil {
		t.Fatalf("decode replayed snapshot: %v", err)
	}
	if snapshot == nil || snapshot.LiveAssistantMessage == nil || snapshot.LiveAssistantMessage.Content != "hello" || snapshot.ThroughSequence != 3 {
		t.Fatalf("unexpected replayed snapshot: %#v", snapshot)
	}
}

func TestAgentRuntimeProjectionReplaysV2EventsInBoundedPages(t *testing.T) {
	run := &model.AgentRun{
		ID:                "helpin-run-v2-pages",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		Status:            model.AgentRunStatusRunning,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("runtime-run-v2-pages"),
	}
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byID:       map[string]*model.AgentRun{run.ID: run},
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|runtime-run-v2-pages": run},
	}
	events := make([]AgentRuntimeEventEnvelope, agentRuntimeV2ReplayPageSize+1)
	for index := range events {
		events[index] = AgentRuntimeEventEnvelope{
			AppID:      "helpin",
			RunID:      "runtime-run-v2-pages",
			HostRunID:  run.ID,
			Type:       "runtime.replay_checkpoint",
			SequenceNo: int64(index + 1),
		}
	}
	runtimeClient := &fakeAgentRuntimeSignalClient{
		v2Events: map[string][]AgentRuntimeEventEnvelope{"runtime-run-v2-pages": events},
	}
	svc := &AgentRuntimeProjectionService{
		runRepo:            runRepo,
		agentRuntimeClient: runtimeClient,
		eventProtocol:      "v2",
		now:                time.Now,
	}

	if err := svc.replayV2Events(context.Background(), run, "runtime-run-v2-pages"); err != nil {
		t.Fatalf("replayV2Events returned error: %v", err)
	}
	if len(runtimeClient.listV2EventCalls) != 2 || runtimeClient.listV2EventCalls[0] != 0 || runtimeClient.listV2EventCalls[1] != agentRuntimeV2ReplayPageSize {
		t.Fatalf("expected two bounded pages, cursors=%#v", runtimeClient.listV2EventCalls)
	}
	if runtimeV2ReplayCursor(run.OutputSummary) != int64(len(events)) {
		t.Fatalf("expected final durable cursor %d, summary=%s", len(events), string(run.OutputSummary))
	}
	if runRepo.summaryUpdates != 2 {
		t.Fatalf("expected one durable cursor update per page, got %d", runRepo.summaryUpdates)
	}
}

func TestAgentRuntimeProjectionReconcileMirrorsRuntimeTranscriptCollections(t *testing.T) {
	now := time.Date(2026, 7, 2, 13, 0, 0, 0, time.UTC)
	run := &model.AgentRun{
		ID:                "helpin-run-transcript",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		RuntimeKind:       "native_sdk",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_transcript"),
	}
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byID:       map[string]*model.AgentRun{run.ID: run},
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_transcript": run},
		active:     []model.AgentRun{*run},
	}
	messageRepo := &fakeAgentRuntimeProjectionMessageRepo{}
	artifactRepo := &fakeAgentRuntimeProjectionArtifactRepo{}
	interactionRepo := &fakeAgentRuntimeProjectionInteractionRepo{}
	resolvedAt := now.Add(time.Minute)
	runtimeClient := &fakeAgentRuntimeSignalClient{
		getRuns: map[string]*AgentRuntimeRun{
			"run_runtime_transcript": {
				ID:        "run_runtime_transcript",
				AppID:     "helpin",
				HostRunID: run.ID,
				Status:    model.AgentRunStatusRunning,
				UpdatedAt: now,
			},
		},
		messages: map[string][]AgentRuntimeMessage{
			"run_runtime_transcript": {{
				ID:          "msg-runtime-1",
				Role:        "assistant",
				Content:     "I checked the repository.",
				MessageType: "assistant_turn",
				CreatedAt:   now,
			}},
		},
		artifacts: map[string][]AgentRuntimeArtifact{
			"run_runtime_transcript": {{
				ID:            "art-runtime-1",
				ArtifactType:  model.AgentRunArtifactTypeRunPlan,
				Format:        "json",
				StorageMode:   "inline",
				InlineContent: `{"plan":[]}`,
				SequenceNo:    1,
				CreatedAt:     now,
			}},
		},
		interactions: map[string][]AgentRuntimeInteraction{
			"run_runtime_transcript": {{
				ID:                   "int-runtime-1",
				RuntimeKind:          "native_sdk",
				InteractionKind:      "human_input",
				Status:               model.AgentRunInteractionStatusResolved,
				Title:                "Input requested",
				Summary:              "Pick one",
				RequestPayload:       json.RawMessage(`{"prompt":"Pick one"}`),
				ResponsePayload:      json.RawMessage(`{"answer":"A"}`),
				ResolvedByExternalID: "user-1",
				ResolvedAt:           &resolvedAt,
				CreatedAt:            now,
				UpdatedAt:            resolvedAt,
			}},
		},
	}
	svc := &AgentRuntimeProjectionService{
		runRepo:            runRepo,
		runMessageRepo:     messageRepo,
		artifactRepo:       artifactRepo,
		interactionRepo:    interactionRepo,
		agentRuntimeClient: runtimeClient,
		now:                func() time.Time { return now.Add(2 * time.Minute) },
	}

	if err := svc.ReconcileMappedRuns(context.Background(), time.Minute, 10); err != nil {
		t.Fatalf("ReconcileMappedRuns returned error: %v", err)
	}
	if err := svc.ReconcileMappedRuns(context.Background(), time.Minute, 10); err != nil {
		t.Fatalf("second ReconcileMappedRuns returned error: %v", err)
	}
	if len(runtimeClient.getCalls) != 2 {
		t.Fatalf("expected runtime state to be fetched on each sweep, got %#v", runtimeClient.getCalls)
	}
	if len(runtimeClient.listMessageCalls) != 1 || len(runtimeClient.listArtifactCalls) != 1 || len(runtimeClient.listInteractionCalls) != 1 {
		t.Fatalf("expected transcript collections to be listed only once, messages=%#v artifacts=%#v interactions=%#v", runtimeClient.listMessageCalls, runtimeClient.listArtifactCalls, runtimeClient.listInteractionCalls)
	}
	if runRepo.summaryUpdates != 1 {
		t.Fatalf("expected one output-summary marker update, got %d", runRepo.summaryUpdates)
	}
	if messageRepo.creates != 1 || len(messageRepo.messages) != 1 {
		t.Fatalf("expected one mirrored message, creates=%d messages=%#v", messageRepo.creates, messageRepo.messages)
	}
	if !agentRunMessageHasRuntimeMessageID(messageRepo.messages[0], "msg-runtime-1") {
		t.Fatalf("message missing runtime id: %s", string(messageRepo.messages[0].ContentBlocks))
	}
	if artifactRepo.creates != 1 || len(artifactRepo.artifacts) != 1 {
		t.Fatalf("expected one mirrored artifact, creates=%d artifacts=%#v", artifactRepo.creates, artifactRepo.artifacts)
	}
	if !agentRunArtifactHasRuntimeArtifactID(artifactRepo.artifacts[0], "art-runtime-1") {
		t.Fatalf("artifact missing runtime id: %s", string(artifactRepo.artifacts[0].Metadata))
	}
	if interactionRepo.creates != 1 || interactionRepo.updates != 0 || len(interactionRepo.interactions) != 1 {
		t.Fatalf("expected one mirrored interaction and no second update, creates=%d updates=%d interactions=%#v", interactionRepo.creates, interactionRepo.updates, interactionRepo.interactions)
	}
	interaction := interactionRepo.interactions[0]
	if interaction.InteractionKind != model.AgentRunInteractionKindRequestUserInput || interaction.Status != model.AgentRunInteractionStatusResolved {
		t.Fatalf("unexpected interaction projection: %#v", interaction)
	}
	if !agentRunInteractionHasRuntimeInteractionID(interaction, "int-runtime-1") {
		t.Fatalf("interaction missing runtime id: %s", string(interaction.RuntimeMetadata))
	}
}

func TestAgentRuntimeProjectionTerminalEventBackfillsRuntimeTranscript(t *testing.T) {
	completedAt := time.Date(2026, 7, 2, 15, 0, 0, 0, time.UTC)
	run := &model.AgentRun{
		ID:                "helpin-run-terminal-transcript",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		RuntimeKind:       "native_sdk",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_terminal_transcript"),
	}
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_terminal_transcript": run},
	}
	messageRepo := &fakeAgentRuntimeProjectionMessageRepo{}
	runtimeClient := &fakeAgentRuntimeSignalClient{
		messages: map[string][]AgentRuntimeMessage{
			"run_runtime_terminal_transcript": {{
				ID:          "msg-runtime-terminal",
				Role:        "assistant",
				Content:     "Terminal transcript backfill.",
				MessageType: "assistant_turn",
				CreatedAt:   completedAt,
			}},
		},
	}
	svc := &AgentRuntimeProjectionService{
		runRepo:            runRepo,
		runMessageRepo:     messageRepo,
		agentRuntimeClient: runtimeClient,
		now:                func() time.Time { return completedAt },
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID:  "run_runtime_terminal_transcript",
		Type:   "run.completed",
		SentAt: completedAt,
	}); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(runtimeClient.listMessageCalls) != 1 || runtimeClient.listMessageCalls[0] != "run_runtime_terminal_transcript" {
		t.Fatalf("expected transcript list on terminal event, got %#v", runtimeClient.listMessageCalls)
	}
	if messageRepo.creates != 1 || len(messageRepo.messages) != 1 {
		t.Fatalf("expected one mirrored terminal message, creates=%d messages=%#v", messageRepo.creates, messageRepo.messages)
	}
	if !agentRunMessageHasRuntimeMessageID(messageRepo.messages[0], "msg-runtime-terminal") {
		t.Fatalf("message missing runtime id: %s", string(messageRepo.messages[0].ContentBlocks))
	}
}

func TestAgentRuntimeProjectionTerminalBackfillUpdatesChangedLiveAssistantMessageByRuntimeMessageID(t *testing.T) {
	completedAt := time.Date(2026, 7, 2, 15, 15, 0, 0, time.UTC)
	run := &model.AgentRun{
		ID:                "helpin-run-terminal-live-dedupe",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		RuntimeKind:       "codex",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_terminal_live_dedupe"),
	}
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_terminal_live_dedupe": run},
	}
	messageRepo := &fakeAgentRuntimeProjectionMessageRepo{}
	runtimeClient := &fakeAgentRuntimeSignalClient{
		messages: map[string][]AgentRuntimeMessage{
			"run_runtime_terminal_live_dedupe": {{
				ID:               "store-msg-1",
				RuntimeMessageID: "event-msg-1",
				Role:             "assistant",
				Content:          "Final answer from the completed turn.",
				MessageType:      "message",
				CreatedAt:        completedAt,
			}},
		},
	}
	svc := &AgentRuntimeProjectionService{
		runRepo:            runRepo,
		runMessageRepo:     messageRepo,
		agentRuntimeClient: runtimeClient,
		now:                func() time.Time { return completedAt },
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_terminal_live_dedupe",
		Type:  "assistant_message_completed",
		Data: map[string]any{
			"message_id": "event-msg-1",
			"content":    "Opening progress message.",
		},
	}); err != nil {
		t.Fatalf("ApplyEvent assistant returned error: %v", err)
	}
	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID:  "run_runtime_terminal_live_dedupe",
		Type:   "run.completed",
		SentAt: completedAt,
	}); err != nil {
		t.Fatalf("ApplyEvent completed returned error: %v", err)
	}
	if messageRepo.creates != 1 || len(messageRepo.messages) != 1 {
		t.Fatalf("expected terminal backfill to retain one live message, creates=%d messages=%#v", messageRepo.creates, messageRepo.messages)
	}
	if messageRepo.updates != 1 {
		t.Fatalf("expected terminal backfill to update changed live message, updates=%d", messageRepo.updates)
	}
	if messageRepo.messages[0].Content != "Final answer from the completed turn." {
		t.Fatalf("terminal backfill content = %q", messageRepo.messages[0].Content)
	}
	if messageRepo.messages[0].MessageType != "assistant_turn" {
		t.Fatalf("terminal backfill message type = %q", messageRepo.messages[0].MessageType)
	}
	if !agentRunMessageHasRuntimeMessageID(messageRepo.messages[0], "event-msg-1") {
		t.Fatalf("message missing event runtime id: %s", string(messageRepo.messages[0].ContentBlocks))
	}
}

func TestAgentRuntimeProjectionTerminalBackfillEnrichesLiveCodexTranscript(t *testing.T) {
	completedAt := time.Date(2026, 7, 2, 15, 30, 0, 0, time.UTC)
	run := &model.AgentRun{
		ID:                "helpin-run-terminal-live-enrich",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		RuntimeKind:       "codex",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_terminal_live_enrich"),
	}
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_terminal_live_enrich": run},
	}
	messageRepo := &fakeAgentRuntimeProjectionMessageRepo{}
	runtimeClient := &fakeAgentRuntimeSignalClient{messages: map[string][]AgentRuntimeMessage{}}
	svc := &AgentRuntimeProjectionService{
		runRepo:            runRepo,
		runMessageRepo:     messageRepo,
		agentRuntimeClient: runtimeClient,
		now:                func() time.Time { return completedAt },
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_terminal_live_enrich",
		Type:  "assistant_message_completed",
		Data: map[string]any{
			"message_id": "event-msg-enrich",
			"content":    "Done.",
		},
	}); err != nil {
		t.Fatalf("ApplyEvent assistant returned error: %v", err)
	}
	if len(messageRepo.messages) != 1 || len(messageRepo.messages[0].ToolInvocations) != 0 {
		t.Fatalf("expected initial content-only live mirror, got %#v", messageRepo.messages)
	}

	runtimeClient.messages["run_runtime_terminal_live_enrich"] = []AgentRuntimeMessage{{
		ID:               "store-msg-enrich",
		RuntimeMessageID: "event-msg-enrich",
		Role:             "assistant",
		Content:          "Done.",
		MessageType:      "assistant_turn",
		ToolInvocations:  json.RawMessage(`[{"tool_name":"run_command","input":{"command":"go test ./..."},"output_summary":"ok","duration_ms":12}]`),
		CreatedAt:        completedAt,
	}}
	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID:  "run_runtime_terminal_live_enrich",
		Type:   "run.completed",
		SentAt: completedAt,
	}); err != nil {
		t.Fatalf("ApplyEvent completed returned error: %v", err)
	}
	if messageRepo.creates != 1 || messageRepo.updates != 1 || len(messageRepo.messages) != 1 {
		t.Fatalf("expected terminal reconciliation to enrich in place, creates=%d updates=%d messages=%#v", messageRepo.creates, messageRepo.updates, messageRepo.messages)
	}
	if len(messageRepo.messages[0].ToolInvocations) == 0 || len(messageRepo.messages[0].TurnSegments) == 0 {
		t.Fatalf("expected durable Codex tool transcript, got %#v", messageRepo.messages[0])
	}
}

func TestAgentRuntimeProjectionEmptyTerminalBackfillPreservesLiveCodexTranscript(t *testing.T) {
	run := &model.AgentRun{
		ID:          "helpin-run-terminal-live-preserve",
		WorkspaceID: "ws-1",
		RuntimeKind: "codex",
	}
	toolInvocations := json.RawMessage(`[{"tool_name":"publish_task_plan_doc","input":{"content":"# Plan"},"output_summary":"published"}]`)
	messageRepo := &fakeAgentRuntimeProjectionMessageRepo{messages: []model.AgentRunMessage{{
		WorkspaceID:      run.WorkspaceID,
		RunID:            run.ID,
		RuntimeMessageID: "event-msg-preserve",
		Role:             "assistant",
		Content:          "The planning document is ready for approval.",
		MessageType:      "assistant_turn",
		ToolInvocations:  toolInvocations,
		TurnSegments:     runtimeMessageTurnSegments("event-msg-preserve", "The planning document is ready for approval.", toolInvocations),
	}}}
	svc := &AgentRuntimeProjectionService{
		runRepo:        &fakeAgentRuntimeProjectionRunRepo{},
		runMessageRepo: messageRepo,
	}

	if err := svc.createRuntimeMessage(context.Background(), run, AgentRuntimeMessage{
		RuntimeMessageID: "event-msg-preserve",
	}); err != nil {
		t.Fatalf("createRuntimeMessage returned error: %v", err)
	}
	if len(messageRepo.messages) != 1 {
		t.Fatalf("expected one preserved message, got %#v", messageRepo.messages)
	}
	message := messageRepo.messages[0]
	if message.Content != "The planning document is ready for approval." {
		t.Fatalf("empty terminal backfill erased content: %q", message.Content)
	}
	if len(message.ToolInvocations) == 0 || len(message.TurnSegments) == 0 {
		t.Fatalf("empty terminal backfill erased tool transcript: %#v", message)
	}
}

func TestAgentRuntimeProjectionReconcileDedupesLocalUserResumeMessage(t *testing.T) {
	run := &model.AgentRun{
		ID:          "helpin-run-user-reply-dedupe",
		WorkspaceID: "ws-1",
	}
	messageRepo := &fakeAgentRuntimeProjectionMessageRepo{
		messages: []model.AgentRunMessage{{
			WorkspaceID: "ws-1",
			RunID:       run.ID,
			Role:        "user",
			Content:     "Please make that change.",
			MessageType: "request_changes",
			SequenceNo:  1,
		}},
	}
	svc := &AgentRuntimeProjectionService{
		runMessageRepo: messageRepo,
	}

	err := svc.createRuntimeMessage(context.Background(), run, AgentRuntimeMessage{
		ID:          "runtime-user-message-1",
		Role:        "user",
		Content:     "Please make that change.",
		MessageType: "message",
	})
	if err != nil {
		t.Fatalf("createRuntimeMessage returned error: %v", err)
	}
	if messageRepo.creates != 0 || len(messageRepo.messages) != 1 {
		t.Fatalf("expected runtime user message to dedupe against local request_changes, creates=%d messages=%#v", messageRepo.creates, messageRepo.messages)
	}
}

func TestAgentRuntimeProjectionSkipsToolCallArgsDeltaArtifacts(t *testing.T) {
	run := &model.AgentRun{
		ID:                "helpin-run-tool-delta",
		WorkspaceID:       "ws-1",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_tool_delta"),
	}
	artifactRepo := &fakeAgentRuntimeProjectionArtifactRepo{}
	svc := &AgentRuntimeProjectionService{
		runRepo: &fakeAgentRuntimeProjectionRunRepo{
			byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_tool_delta": run},
		},
		artifactRepo: artifactRepo,
		now:          time.Now,
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_tool_delta",
		Type:  "tool_call_args_delta",
		Data: map[string]any{
			"tool_call_id": "tool-1",
			"args_delta":   "{\"path\"",
		},
	}); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if artifactRepo.creates != 0 || len(artifactRepo.artifacts) != 0 {
		t.Fatalf("expected no artifact for args delta, creates=%d artifacts=%#v", artifactRepo.creates, artifactRepo.artifacts)
	}
}

func TestAgentRuntimeProjectionReturnsNotFoundForUnmappedRun(t *testing.T) {
	svc := &AgentRuntimeProjectionService{
		runRepo: &fakeAgentRuntimeProjectionRunRepo{},
		now:     time.Now,
	}
	err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_missing",
		Type:  "run.completed",
	})
	if !errors.Is(err, errAgentRuntimeProjectionRunNotFound) {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestAgentRuntimeProjectionSubjectIsScopedToApp(t *testing.T) {
	if got := agentruntime.AppEventSubject("helpin"); got != "agent-runtime.events.helpin.>" {
		t.Fatalf("expected app-scoped subject, got %q", got)
	}
	if got := agentruntime.AppEventSubject("helpin.stage"); got != "agent-runtime.events.helpin_stage.>" {
		t.Fatalf("expected dotted app IDs to be scoped as one token, got %q", got)
	}
}

func TestAgentRuntimeProjectionStreamUsesSDKRuntimeEventDefaults(t *testing.T) {
	if agentruntime.DefaultNATSStreamName != "AGENT_RUNTIME_EVENTS" {
		t.Fatalf("unexpected runtime stream name: %q", agentruntime.DefaultNATSStreamName)
	}
	if agentruntime.DefaultNATSStreamSubject != "agent-runtime.events.>" {
		t.Fatalf("unexpected runtime stream subject: %q", agentruntime.DefaultNATSStreamSubject)
	}
}

func stringPointer(value string) *string {
	return &value
}

func TestAgentRuntimeProjectionDropsToolCallEventArtifacts(t *testing.T) {
	run := &model.AgentRun{
		ID:                "helpin-run-tool-events",
		WorkspaceID:       "ws-1",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_tool_events"),
	}
	artifactRepo := &fakeAgentRuntimeProjectionArtifactRepo{}
	svc := &AgentRuntimeProjectionService{
		runRepo: &fakeAgentRuntimeProjectionRunRepo{
			byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_tool_events": run},
		},
		artifactRepo: artifactRepo,
		now:          time.Now,
	}

	for _, eventType := range []string{"tool_call_started", "tool_call_result", "tool_call_finished"} {
		if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
			RunID: "run_runtime_tool_events",
			Type:  eventType,
			Data:  map[string]any{"tool_call_id": "tool-1"},
		}); err != nil {
			t.Fatalf("ApplyEvent(%s) returned error: %v", eventType, err)
		}
	}
	if artifactRepo.creates != 0 || len(artifactRepo.artifacts) != 0 {
		t.Fatalf("tool call events must not create artifacts, creates=%d artifacts=%#v", artifactRepo.creates, artifactRepo.artifacts)
	}
}

func TestRuntimeMessageTurnSegmentsMapsInvocations(t *testing.T) {
	invocations := []byte(`[{"tool_name":"read_file","input":{"path":"a.md"},"output_summary":"12 lines","duration_ms":40}]`)
	payload := runtimeMessageTurnSegments("msg-1", "All done.", invocations)
	if payload == nil {
		t.Fatal("expected turn segments payload")
	}
	var segments []model.CodingSessionLiveTurnSegment
	if err := json.Unmarshal(payload, &segments); err != nil {
		t.Fatalf("unmarshal segments: %v", err)
	}
	if len(segments) != 2 {
		t.Fatalf("expected tool + assistant segments, got %#v", segments)
	}
	tool := segments[0]
	if tool.Kind != "tool_call" || tool.ToolCall == nil || tool.ToolCall.ToolName != "read_file" || tool.ToolCall.Status != "completed" {
		t.Fatalf("unexpected tool segment %#v", tool)
	}
	if tool.ToolCall.Result == nil || tool.ToolCall.Result.Content != "12 lines" {
		t.Fatalf("expected tool result summary, got %#v", tool.ToolCall.Result)
	}
	if tool.ToolCall.DurationMs == nil || *tool.ToolCall.DurationMs != 40 {
		t.Fatalf("expected duration 40, got %#v", tool.ToolCall.DurationMs)
	}
	text := segments[1]
	if text.Kind != "assistant_message" || text.AssistantMessage == nil || text.AssistantMessage.Content != "All done." {
		t.Fatalf("unexpected assistant segment %#v", text)
	}
	if runtimeMessageTurnSegments("msg-2", "text only", nil) != nil {
		t.Fatal("no invocations must produce no segments")
	}
}

func TestRuntimeMessageTurnSegmentsPreservesCodexPreambleBeforeTool(t *testing.T) {
	invocations := []byte(`[{"tool_call_id":"tool-1","tool_name":"read_file","input":{"path":"a.md"},"output_summary":"12 lines","assistant_before_tool":true}]`)
	payload := runtimeMessageTurnSegments("msg-preamble", "I will inspect the file.", invocations)
	var segments []model.CodingSessionLiveTurnSegment
	if err := json.Unmarshal(payload, &segments); err != nil {
		t.Fatalf("unmarshal segments: %v", err)
	}
	if len(segments) != 2 || segments[0].Kind != "assistant_message" || segments[1].Kind != "tool_call" {
		t.Fatalf("expected assistant preamble followed by tool, got %#v", segments)
	}
	if segments[1].SegmentID != "tool-1" || segments[1].ToolCall == nil || segments[1].ToolCall.ToolCallID != "tool-1" {
		t.Fatalf("expected stable tool identity, got %#v", segments[1])
	}
}

func TestRuntimeMessageTurnSegmentsPreservesFailedToolStatus(t *testing.T) {
	invocations := []byte(`[{"tool_call_id":"tool-failed","tool_name":"read_skill","input":{"key":"missing"},"output_summary":"not found","status":"failed","error":"skill is not available"}]`)
	payload := runtimeMessageTurnSegments("msg-failed", "", invocations)
	var segments []model.CodingSessionLiveTurnSegment
	if err := json.Unmarshal(payload, &segments); err != nil {
		t.Fatalf("unmarshal segments: %v", err)
	}
	if len(segments) != 1 || segments[0].ToolCall == nil || segments[0].ToolCall.Status != "failed" {
		t.Fatalf("expected persisted failure, got %#v", segments)
	}
	if segments[0].ToolCall.Result == nil || segments[0].ToolCall.Result.Error == nil || *segments[0].ToolCall.Result.Error != "skill is not available" {
		t.Fatalf("expected persisted tool error, got %#v", segments[0].ToolCall.Result)
	}
}

func TestAgentRuntimeProjectionMirrorsAssistantMessageWithToolInvocations(t *testing.T) {
	run := &model.AgentRun{
		ID:                "helpin-run-enriched",
		WorkspaceID:       "ws-1",
		Status:            model.AgentRunStatusRunning,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_enriched"),
	}
	messageRepo := &fakeAgentRuntimeProjectionMessageRepo{}
	client := &fakeAgentRuntimeSignalClient{
		messages: map[string][]AgentRuntimeMessage{
			"run_runtime_enriched": {{
				ID:               "msg_store_1",
				RuntimeMessageID: "evt-msg-1",
				Role:             "assistant",
				Content:          "Drafts created.",
				MessageType:      "assistant_turn",
				ToolInvocations:  json.RawMessage(`[{"tool_name":"create_document","input":{},"output_summary":"doc created","duration_ms":10}]`),
			}},
		},
	}
	svc := &AgentRuntimeProjectionService{
		runRepo: &fakeAgentRuntimeProjectionRunRepo{
			byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_enriched": run},
		},
		runMessageRepo:     messageRepo,
		agentRuntimeClient: client,
		now:                time.Now,
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_enriched",
		Type:  "assistant_message_completed",
		Data:  map[string]any{"message_id": "evt-msg-1", "content": "Drafts created."},
	}); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(messageRepo.messages) != 1 {
		t.Fatalf("expected one mirrored message, got %#v", messageRepo.messages)
	}
	mirrored := messageRepo.messages[0]
	if len(mirrored.ToolInvocations) == 0 {
		t.Fatal("expected tool invocations copied from runtime store message")
	}
	if len(mirrored.TurnSegments) == 0 {
		t.Fatal("expected turn segments derived from tool invocations")
	}
	if !agentRunMessageHasRuntimeMessageID(mirrored, "evt-msg-1") {
		t.Fatal("dedupe identity annotation must survive enrichment")
	}
}

func TestAgentRuntimeProjectionMirrorsAssistantMessageWhenStoreLookupFails(t *testing.T) {
	run := &model.AgentRun{
		ID:                "helpin-run-degraded",
		WorkspaceID:       "ws-1",
		Status:            model.AgentRunStatusRunning,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_degraded"),
	}
	messageRepo := &fakeAgentRuntimeProjectionMessageRepo{}
	client := &fakeAgentRuntimeSignalClient{listErr: errors.New("runtime unreachable")}
	svc := &AgentRuntimeProjectionService{
		runRepo: &fakeAgentRuntimeProjectionRunRepo{
			byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_degraded": run},
		},
		runMessageRepo:     messageRepo,
		agentRuntimeClient: client,
		now:                time.Now,
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_degraded",
		Type:  "assistant_message_completed",
		Data:  map[string]any{"message_id": "evt-msg-2", "content": "Done."},
	}); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(messageRepo.messages) != 1 || messageRepo.messages[0].Content != "Done." {
		t.Fatalf("store lookup failure must degrade to content-only mirror, got %#v", messageRepo.messages)
	}
}
