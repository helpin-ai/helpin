package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// fakeRunGroupClient serves one group's completions like Agent Runtime:
// in sequence order, after the requested cursor.
type fakeRunGroupClient struct {
	agentRuntimeSignalClient
	available   bool
	createErr   error
	created     []string
	completions []CommandBarRunGroupCompletion
	listCalls   int
}

func (f *fakeRunGroupClient) RunGroupsAvailable(context.Context) bool { return f.available }

func (f *fakeRunGroupClient) CreateRunGroup(_ context.Context, key string) (string, error) {
	if f.createErr != nil {
		return "", f.createErr
	}
	f.created = append(f.created, key)
	return "rungroup-1", nil
}

func (f *fakeRunGroupClient) ListRunGroupCompletions(_ context.Context, _ string, after int64, limit int) ([]CommandBarRunGroupCompletion, error) {
	f.listCalls++
	var out []CommandBarRunGroupCompletion
	for _, completion := range f.completions {
		if completion.Seq > after && len(out) < limit {
			out = append(out, completion)
		}
	}
	return out, nil
}

func setupRunGroupPlanTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupCommandBarPlanTestDB(t)
	if err := db.Exec(`CREATE TABLE command_bar_plan_run_groups (
		plan_id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, group_id TEXT NOT NULL,
		completion_cursor INTEGER NOT NULL DEFAULT 0, created_at DATETIME, updated_at DATETIME)`).Error; err != nil {
		t.Fatalf("create run group table: %v", err)
	}
	return db
}

const (
	runGroupTestWorkspace = "11111111-1111-1111-1111-111111111111"
	runGroupTestPlan      = "22222222-2222-2222-2222-222222222222"
	runGroupTestAgent     = "33333333-3333-3333-3333-333333333333"
)

// seedTwoStepPlan creates a pipeline plan whose first step completed and
// whose second step's run is still stored as running, as it is when the
// runtime has settled it but the projection has not caught up.
func seedTwoStepPlan(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	ctx := context.Background()
	runRepo := repository.NewAgentRunRepository(db)
	planRepo := repository.NewCommandBarPlanRepository(db)
	runIDs := []string{"44444444-4444-4444-4444-444444444444", "55555555-5555-5555-5555-555555555555"}
	pageContext := model.CommandBarPageContext{EntityType: "epic", EntityID: "epic-1", DisplayTitle: "Epic 1"}
	target := model.CommandBarPageContext{EntityType: "task", EntityID: "task-1", DisplayTitle: "Task 1"}
	steps := []model.CommandBarPlanStep{
		{AgentID: runGroupTestAgent, AgentName: "Forge", PlanKind: model.CommandBarPlanKindTaskPipeline, Target: target, Instructions: "Build."},
		{AgentID: runGroupTestAgent, AgentName: "Lens", PlanKind: model.CommandBarPlanKindTaskPipeline, Target: target, Instructions: "Review.", DependsOnStepIndexes: []int{0}},
	}
	plan, err := newCommandBarPlanRecord(runGroupTestWorkspace, "", runGroupTestPlan, "build then review", pageContext, steps)
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	plan.RunIDsByStep, _ = json.Marshal(map[int]string{0: runIDs[0], 1: runIDs[1]})
	plan.RunCount = 2
	if err := planRepo.Create(ctx, plan); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	for index, status := range []string{model.AgentRunStatusCompleted, model.AgentRunStatusRunning} {
		trigger, err := buildCommandBarTriggerContext("build then review", pageContext, steps, index, runGroupTestPlan)
		if err != nil {
			t.Fatalf("trigger: %v", err)
		}
		input, _ := json.Marshal(model.AgentRunInputPayload{Trigger: trigger})
		runtimeID := "runtime-" + runIDs[index]
		if err := runRepo.Create(ctx, &model.AgentRun{
			ID: runIDs[index], WorkspaceID: runGroupTestWorkspace, AgentID: runGroupTestAgent, TargetType: "task", TargetID: "task-1",
			RuntimeKind: "native_sdk", InvocationMode: model.InvocationModeAutonomous, ApprovalState: "not_required",
			PauseReason: model.AgentRunPauseReasonNone, Status: status, Input: input, OutputSummary: json.RawMessage("{}"),
			ExternalRuntime: strPtr(agentRuntimeName), ExternalRuntimeID: &runtimeID,
		}); err != nil {
			t.Fatalf("create run: %v", err)
		}
	}
	return runIDs
}

// A completion advances the plan as soon as the consumer reads it, records
// the cursor, and is applied once however often it is read again.
func TestCommandBarRunGroupConsumerAdvancesPlanOnce(t *testing.T) {
	db := setupRunGroupPlanTestDB(t)
	ctx := context.Background()
	runIDs := seedTwoStepPlan(t, db)
	groups := repository.NewCommandBarPlanRunGroupRepository(db)
	if err := groups.Create(ctx, &model.CommandBarPlanRunGroup{PlanID: runGroupTestPlan, WorkspaceID: runGroupTestWorkspace, GroupID: "rungroup-1"}); err != nil {
		t.Fatalf("create link: %v", err)
	}
	client := &fakeRunGroupClient{available: true, completions: []CommandBarRunGroupCompletion{
		{Seq: 1, RuntimeRunID: "runtime-" + runIDs[0], Status: model.AgentRunStatusCompleted},
		{Seq: 2, RuntimeRunID: "runtime-" + runIDs[1], Status: model.AgentRunStatusCompleted},
	}}
	service := &AgentService{runRepo: repository.NewAgentRunRepository(db), commandBarPlanRepo: repository.NewCommandBarPlanRepository(db), agentRuntimeClient: client}
	service.SetCommandBarRunGroups(true, groups)

	if err := service.ConsumeCommandBarRunGroupCompletions(ctx); err != nil {
		t.Fatalf("consume: %v", err)
	}
	plan, err := service.commandBarPlanRepo.GetByID(ctx, runGroupTestWorkspace, runGroupTestPlan)
	if err != nil || plan.Status != model.CommandBarPlanStatusCompleted {
		t.Fatalf("plan = %+v, %v; want completed without waiting for the sweep", plan, err)
	}
	link, err := groups.GetByPlan(ctx, runGroupTestWorkspace, runGroupTestPlan)
	if err != nil || link.CompletionCursor != 2 {
		t.Fatalf("cursor = %+v, %v", link, err)
	}
	// A replica or retry that read the same completions before the cursor
	// moved does not apply them again.
	for _, seq := range []int64{1, 2} {
		owned, err := groups.AdvanceCursor(ctx, runGroupTestWorkspace, runGroupTestPlan, seq)
		if err != nil || owned {
			t.Fatalf("replay of seq %d owned=%v err=%v", seq, owned, err)
		}
	}
	// The child's own row is left to the projection, so its finalizers
	// still see the terminal transition.
	child, err := service.runRepo.GetByID(ctx, runGroupTestWorkspace, runIDs[1])
	if err != nil || child.Status != model.AgentRunStatusRunning {
		t.Fatalf("child row = %+v, %v", child, err)
	}
}

func TestCommandBarRunGroupConsumerFailsPlanOnFailedChild(t *testing.T) {
	db := setupRunGroupPlanTestDB(t)
	ctx := context.Background()
	runIDs := seedTwoStepPlan(t, db)
	groups := repository.NewCommandBarPlanRunGroupRepository(db)
	_ = groups.Create(ctx, &model.CommandBarPlanRunGroup{PlanID: runGroupTestPlan, WorkspaceID: runGroupTestWorkspace, GroupID: "rungroup-1"})
	client := &fakeRunGroupClient{available: true, completions: []CommandBarRunGroupCompletion{
		{Seq: 1, RuntimeRunID: "runtime-" + runIDs[1], Status: model.AgentRunStatusFailed},
	}}
	service := &AgentService{runRepo: repository.NewAgentRunRepository(db), commandBarPlanRepo: repository.NewCommandBarPlanRepository(db), agentRuntimeClient: client}
	service.SetCommandBarRunGroups(true, groups)
	if err := service.ConsumeCommandBarRunGroupCompletions(ctx); err != nil {
		t.Fatalf("consume: %v", err)
	}
	plan, _ := service.commandBarPlanRepo.GetByID(ctx, runGroupTestWorkspace, runGroupTestPlan)
	if plan.Status != model.CommandBarPlanStatusFailed {
		t.Fatalf("plan status = %q, want failed", plan.Status)
	}
}

func TestEnsureCommandBarPlanRunGroup(t *testing.T) {
	tests := []struct {
		name      string
		enabled   bool
		available bool
		createErr error
		wantLink  bool
	}{
		{name: "enabled and supported", enabled: true, available: true, wantLink: true},
		{name: "flag off keeps the legacy path", enabled: false, available: true},
		{name: "runtime without run groups keeps the legacy path", enabled: true, available: false},
		{name: "creation failure keeps the legacy path", enabled: true, available: true, createErr: errors.New("boom")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupRunGroupPlanTestDB(t)
			ctx := context.Background()
			seedTwoStepPlan(t, db)
			groups := repository.NewCommandBarPlanRunGroupRepository(db)
			client := &fakeRunGroupClient{available: tt.available, createErr: tt.createErr}
			service := &AgentService{runRepo: repository.NewAgentRunRepository(db), commandBarPlanRepo: repository.NewCommandBarPlanRepository(db), agentRuntimeClient: client}
			service.SetCommandBarRunGroups(tt.enabled, groups)
			service.ensureCommandBarPlanRunGroup(ctx, runGroupTestWorkspace, runGroupTestPlan)
			link, err := groups.GetByPlan(ctx, runGroupTestWorkspace, runGroupTestPlan)
			if err != nil {
				t.Fatalf("get link: %v", err)
			}
			if (link != nil) != tt.wantLink {
				t.Fatalf("link = %+v, want link %v", link, tt.wantLink)
			}
			stepCtx := service.commandBarPlanStepContext(ctx, runGroupTestWorkspace, runGroupTestPlan)
			if got := commandBarRunGroupFromContext(stepCtx); (got == "rungroup-1") != tt.wantLink {
				t.Fatalf("step joins group %q, want link %v", got, tt.wantLink)
			}
			if tt.wantLink && (len(client.created) != 1 || client.created[0] != "command_bar_plan:"+runGroupTestPlan) {
				t.Fatalf("creation keys = %v", client.created)
			}
		})
	}
}
