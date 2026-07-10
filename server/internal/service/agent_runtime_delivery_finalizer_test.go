package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type fakeFinalizerRepositoryDelivery struct {
	calls  []AgentRunRepositoryDelivery
	result *AgentRunRepositoryDeliveryResult
	err    error
}

func (f *fakeFinalizerRepositoryDelivery) FinalizeDelegatedRunDelivery(_ context.Context, _ *model.AgentRun, delivery AgentRunRepositoryDelivery) (*AgentRunRepositoryDeliveryResult, error) {
	f.calls = append(f.calls, delivery)
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func TestAgentRuntimeFinalizersRepositoryDeliveryFiresOnPushedSummary(t *testing.T) {
	run := newDelegatedTerminalTestRun("helpin-run-delivery", "run_runtime_delivery")
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_delivery": run},
	}
	runtimeClient := &fakeAgentRuntimeSignalClient{
		getRuns: map[string]*AgentRuntimeRun{
			"run_runtime_delivery": {
				ID:            "run_runtime_delivery",
				HostRunID:     run.ID,
				Status:        model.AgentRunStatusCompleted,
				OutputSummary: json.RawMessage(`{"repository":{"changed":true,"pushed":true,"branch":"hel-31-fix","commit":"abc1234"}}`),
			},
		},
	}
	delivery := &fakeFinalizerRepositoryDelivery{
		result: &AgentRunRepositoryDeliveryResult{Provider: "github", Number: 5, Title: "Fix", URL: "https://github.test/pr/5"},
	}
	finalizers := &AgentRunFinalizerService{
		runRepo:            runRepo,
		repositoryDelivery: delivery,
	}
	svc := &AgentRuntimeProjectionService{
		runRepo:            runRepo,
		agentRuntimeClient: runtimeClient,
		runFinalizers:      finalizers,
		now:                time.Now,
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_delivery",
		Type:  "run.completed",
	}); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(delivery.calls) != 1 {
		t.Fatalf("expected one delivery call, got %d", len(delivery.calls))
	}
	if delivery.calls[0].Branch != "hel-31-fix" || delivery.calls[0].CommitSHA != "abc1234" {
		t.Fatalf("unexpected delivery payload: %#v", delivery.calls[0])
	}
	if !runOutputSummaryFlag(run.OutputSummary, agentRuntimeFinalizerRepositoryDeliverySummaryKey) {
		t.Fatalf("expected repository delivery marker, got %s", string(run.OutputSummary))
	}

	// Crash-replay while still non-terminal: the marker skips the delivery.
	finalizers.FinalizeTerminalRun(context.Background(), run, true)
	if len(delivery.calls) != 1 {
		t.Fatalf("expected delivery replay no-op, got %d calls", len(delivery.calls))
	}
}

func TestAgentRuntimeFinalizersRepositoryDeliverySkipsWhenNotPushed(t *testing.T) {
	tests := []struct {
		name    string
		summary string
	}{
		{name: "pushed false", summary: `{"repository":{"changed":true,"pushed":false,"branch":"hel-31-fix"}}`},
		{name: "repository contract absent", summary: `{"status":"success"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := newDelegatedTerminalTestRun("helpin-run-nopush", "run_runtime_nopush")
			run.OutputSummary = json.RawMessage(tt.summary)
			runRepo := &fakeAgentRuntimeProjectionRunRepo{
				byID: map[string]*model.AgentRun{run.ID: run},
			}
			delivery := &fakeFinalizerRepositoryDelivery{}
			finalizers := &AgentRunFinalizerService{
				runRepo:            runRepo,
				repositoryDelivery: delivery,
			}

			run.Status = model.AgentRunStatusCompleted
			finalizers.FinalizeTerminalRun(context.Background(), run, true)
			if len(delivery.calls) != 0 {
				t.Fatalf("expected no delivery calls, got %d", len(delivery.calls))
			}
			if runOutputSummaryFlag(run.OutputSummary, agentRuntimeFinalizerRepositoryDeliverySummaryKey) {
				t.Fatalf("skip must not record a marker, got %s", string(run.OutputSummary))
			}
		})
	}
}

func TestAgentRuntimeFinalizersRepositoryDeliverySkipsTargetsWithoutDeliverySemantics(t *testing.T) {
	run := newDelegatedTerminalTestRun("helpin-run-nontarget", "run_runtime_nontarget")
	run.TargetType = "support_conversation"
	run.TargetID = "conv-1"
	run.TaskID = nil
	run.Status = model.AgentRunStatusCompleted
	run.OutputSummary = json.RawMessage(`{"repository":{"changed":true,"pushed":true,"branch":"b"}}`)
	runRepo := &fakeAgentRuntimeProjectionRunRepo{byID: map[string]*model.AgentRun{run.ID: run}}
	delivery := &fakeFinalizerRepositoryDelivery{}
	finalizers := &AgentRunFinalizerService{
		runRepo:            runRepo,
		repositoryDelivery: delivery,
	}

	finalizers.FinalizeTerminalRun(context.Background(), run, true)
	if len(delivery.calls) != 0 {
		t.Fatalf("expected no delivery calls for support target, got %d", len(delivery.calls))
	}
}

func TestAgentRuntimeFinalizersRepositoryDeliveryFailureDoesNotBlockOthers(t *testing.T) {
	run := newDelegatedTerminalTestRun("helpin-run-delivery-fail", "run_runtime_delivery_fail")
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_delivery_fail": run},
	}
	runtimeClient := &fakeAgentRuntimeSignalClient{
		getRuns: map[string]*AgentRuntimeRun{
			"run_runtime_delivery_fail": {
				ID:            "run_runtime_delivery_fail",
				HostRunID:     run.ID,
				Status:        model.AgentRunStatusCompleted,
				OutputSummary: json.RawMessage(`{"repository":{"changed":true,"pushed":true,"branch":"hel-31-fix","commit":"abc1234"}}`),
			},
		},
	}
	agentRepo := &fakeFinalizerAgentRepo{agent: &model.Agent{ID: "agent-1", Status: "working"}}
	delivery := &fakeFinalizerRepositoryDelivery{err: errors.New("github unreachable")}
	svc := &AgentRuntimeProjectionService{
		runRepo:            runRepo,
		agentRuntimeClient: runtimeClient,
		runFinalizers: &AgentRunFinalizerService{
			runRepo:            runRepo,
			agentRepo:          agentRepo,
			repositoryDelivery: delivery,
		},
		now: time.Now,
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_delivery_fail",
		Type:  "run.completed",
	}); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(delivery.calls) != 1 {
		t.Fatalf("expected one delivery attempt, got %d", len(delivery.calls))
	}
	if agentRepo.updates != 1 || agentRepo.agent.Status != "idle" {
		t.Fatalf("expected agent idle finalizer to run despite delivery failure, updates=%d", agentRepo.updates)
	}
	if run.Status != model.AgentRunStatusCompleted || runRepo.updates != 1 {
		t.Fatalf("expected status projection despite delivery failure: status=%s updates=%d", run.Status, runRepo.updates)
	}
	if runOutputSummaryFlag(run.OutputSummary, agentRuntimeFinalizerRepositoryDeliverySummaryKey) {
		t.Fatalf("failed delivery must not record its marker, got %s", string(run.OutputSummary))
	}
}

// fakeDeliveryGitHubClient wraps fakeGitHubAppClient to simulate the provider
// returning an already-open pull request for the branch pair (the natural
// idempotency of the EnsurePullRequest lookup-then-create contract).
type fakeDeliveryGitHubClient struct {
	fakeGitHubAppClient
	existing *githubapp.PullRequest
}

func (f *fakeDeliveryGitHubClient) EnsurePullRequest(ctx context.Context, installationID, owner, repo string, input githubapp.EnsurePullRequestInput) (*githubapp.PullRequest, error) {
	if f.existing != nil {
		f.ensurePRs = append(f.ensurePRs, input)
		return f.existing, nil
	}
	return f.fakeGitHubAppClient.EnsurePullRequest(ctx, installationID, owner, repo, input)
}

func newDelegatedDeliveryTestRun() *model.AgentRun {
	return &model.AgentRun{
		ID:            "run-delivery-1",
		WorkspaceID:   "ws-1",
		AgentID:       "agent-1",
		TargetType:    "task",
		TargetID:      "task-1",
		TaskID:        stringPointer("task-1"),
		Status:        model.AgentRunStatusCompleted,
		RepositoryID:  stringPointer("repo-1"),
		RepoFullName:  stringPointer("acme/api"),
		BaseBranch:    stringPointer("main"),
		WorkingBranch: stringPointer("hel-31-fix-merge-status"),
	}
}

func TestFinalizeDelegatedRunDeliveryCreatesGitHubPullRequest(t *testing.T) {
	db := newTestDB(t)
	seedGitDeliveryStatusFixture(t, db)
	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO workspaces (
		id, name, slug, workspace_key, owner_id, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"ws-1", "Demo Workspace", "demo", "HEL", "user-1", now, now)

	app := &fakeGitHubAppClient{}
	svc := newGitDeliveryStatusService(db, app)
	run := newDelegatedDeliveryTestRun()

	result, err := svc.FinalizeDelegatedRunDelivery(context.Background(), run, AgentRunRepositoryDelivery{
		Branch:    "hel-31-fix-merge-status",
		CommitSHA: "abc1234",
	})
	if err != nil {
		t.Fatalf("FinalizeDelegatedRunDelivery returned error: %v", err)
	}
	if result == nil || result.Provider != "github" || result.Number != 1 || result.URL != "https://github.test/pr/1" {
		t.Fatalf("unexpected delivery result: %#v", result)
	}
	if len(app.ensurePRs) != 1 {
		t.Fatalf("ensure PR calls = %d, want 1", len(app.ensurePRs))
	}
	prCall := app.ensurePRs[0]
	if prCall.Head != "hel-31-fix-merge-status" || prCall.Base != "main" {
		t.Fatalf("unexpected ensure PR call: %#v", prCall)
	}
	if prCall.Title != "HEL-31: Fix merge status" {
		t.Fatalf("PR title = %q, want task key + name", prCall.Title)
	}

	var target model.TaskDeliveryTarget
	if err := db.Where("task_id = ?", "task-1").First(&target).Error; err != nil {
		t.Fatalf("load delivery target: %v", err)
	}
	if target.DeliveryState != "pr_open" || target.ActivePRNumber == nil || *target.ActivePRNumber != 1 {
		t.Fatalf("delivery target not updated: state=%q pr=%#v", target.DeliveryState, target.ActivePRNumber)
	}
	if target.LastCommitSHA == nil || *target.LastCommitSHA != "abc1234" {
		t.Fatalf("delivery target commit sha = %#v, want abc1234", target.LastCommitSHA)
	}
	if target.LastRunID == nil || *target.LastRunID != run.ID {
		t.Fatalf("delivery target run id = %#v, want %q", target.LastRunID, run.ID)
	}

	var link model.TaskGitLink
	if err := db.Where("workspace_id = ? AND repo = ? AND branch = ?", "ws-1", "acme/api", "hel-31-fix-merge-status").First(&link).Error; err != nil {
		t.Fatalf("load git link: %v", err)
	}
	if link.PRNumber == nil || *link.PRNumber != 1 || link.RunID == nil || *link.RunID != run.ID {
		t.Fatalf("git link not refreshed: %#v", link)
	}
	if link.CommitSHA == nil || *link.CommitSHA != "abc1234" {
		t.Fatalf("git link commit sha = %#v, want abc1234", link.CommitSHA)
	}
}

func TestFinalizeDelegatedRunDeliveryReusesExistingPullRequest(t *testing.T) {
	db := newTestDB(t)
	seedGitDeliveryStatusFixture(t, db)

	app := &fakeDeliveryGitHubClient{
		existing: &githubapp.PullRequest{
			Number:  42,
			Title:   "Fix merge status",
			HTMLURL: "https://github.test/acme/api/pull/42",
			State:   "open",
			HeadRef: "hel-31-fix-merge-status",
			BaseRef: "main",
		},
	}
	svc := newGitDeliveryStatusService(db, app)
	run := newDelegatedDeliveryTestRun()

	for replay := 0; replay < 2; replay++ {
		result, err := svc.FinalizeDelegatedRunDelivery(context.Background(), run, AgentRunRepositoryDelivery{
			Branch:    "hel-31-fix-merge-status",
			CommitSHA: "abc1234",
		})
		if err != nil {
			t.Fatalf("FinalizeDelegatedRunDelivery replay %d returned error: %v", replay, err)
		}
		if result == nil || result.Number != 42 || result.URL != "https://github.test/acme/api/pull/42" {
			t.Fatalf("replay %d: expected existing PR reused, got %#v", replay, result)
		}
	}

	var target model.TaskDeliveryTarget
	if err := db.Where("task_id = ?", "task-1").First(&target).Error; err != nil {
		t.Fatalf("load delivery target: %v", err)
	}
	if target.DeliveryState != "pr_open" || target.ActivePRNumber == nil || *target.ActivePRNumber != 42 {
		t.Fatalf("delivery target should keep existing PR: state=%q pr=%#v", target.DeliveryState, target.ActivePRNumber)
	}

	var linkCount int64
	if err := db.Table("task_git_links").Where("workspace_id = ? AND repo = ? AND branch = ?", "ws-1", "acme/api", "hel-31-fix-merge-status").Count(&linkCount).Error; err != nil {
		t.Fatalf("count git links: %v", err)
	}
	if linkCount != 1 {
		t.Fatalf("expected a single git link for the branch, got %d", linkCount)
	}
}

func TestFinalizeDelegatedRunDeliverySkipsWhenBranchEqualsBase(t *testing.T) {
	db := newTestDB(t)
	seedGitDeliveryStatusFixture(t, db)

	app := &fakeGitHubAppClient{}
	svc := newGitDeliveryStatusService(db, app)
	run := newDelegatedDeliveryTestRun()
	run.WorkingBranch = stringPointer("main")

	result, err := svc.FinalizeDelegatedRunDelivery(context.Background(), run, AgentRunRepositoryDelivery{Branch: "main"})
	if err != nil {
		t.Fatalf("FinalizeDelegatedRunDelivery returned error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected skip for branch == base, got %#v", result)
	}
	if len(app.ensurePRs) != 0 {
		t.Fatalf("expected no ensure PR calls, got %d", len(app.ensurePRs))
	}
}

func TestFinalizeDelegatedRunDeliveryMarksPRFailedOnProviderError(t *testing.T) {
	db := newTestDB(t)
	seedGitDeliveryStatusFixture(t, db)

	// No installation ID -> the GitHub ensure path fails before any API call.
	mustExec(t, db, `UPDATE git_integrations SET installation_id = NULL WHERE id = ?`, "gi-1")

	svc := newGitDeliveryStatusService(db, &fakeGitHubAppClient{})
	run := newDelegatedDeliveryTestRun()

	if _, err := svc.FinalizeDelegatedRunDelivery(context.Background(), run, AgentRunRepositoryDelivery{
		Branch:    "hel-31-fix-merge-status",
		CommitSHA: "abc1234",
	}); err == nil {
		t.Fatalf("expected provider error")
	}

	var target model.TaskDeliveryTarget
	if err := db.Where("task_id = ?", "task-1").First(&target).Error; err != nil {
		t.Fatalf("load delivery target: %v", err)
	}
	if target.DeliveryState != "pr_failed" {
		t.Fatalf("delivery state = %q, want pr_failed", target.DeliveryState)
	}
	if target.LastRunID == nil || *target.LastRunID != run.ID {
		t.Fatalf("delivery target run id = %#v, want %q", target.LastRunID, run.ID)
	}
}
