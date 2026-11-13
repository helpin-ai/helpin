package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// TestAdvanceTaskPipelinePlanUsesSettledRun covers the finalizer path: the
// terminal status is applied to the plan before the projection persists it,
// so the stored child row still reads as running. The plan must advance on
// the settled in-memory run instead of waiting for the stalled-plan sweep.
func TestAdvanceTaskPipelinePlanUsesSettledRun(t *testing.T) {
	tests := []struct {
		name    string
		settled string
		want    string
	}{
		{name: "completed child completes the plan now", settled: model.AgentRunStatusCompleted, want: model.CommandBarPlanStatusCompleted},
		{name: "failed child fails the plan now", settled: model.AgentRunStatusFailed, want: model.CommandBarPlanStatusFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupCommandBarPlanTestDB(t)
			runRepo := repository.NewAgentRunRepository(db)
			planRepo := repository.NewCommandBarPlanRepository(db)
			service := &AgentService{runRepo: runRepo, commandBarPlanRepo: planRepo}
			ctx := context.Background()
			workspaceID := "11111111-1111-1111-1111-111111111111"
			planID := "22222222-2222-2222-2222-222222222222"
			agentID := "33333333-3333-3333-3333-333333333333"
			runIDs := []string{"44444444-4444-4444-4444-444444444444", "55555555-5555-5555-5555-555555555555"}
			pageContext := model.CommandBarPageContext{EntityType: "epic", EntityID: "epic-1", DisplayTitle: "Epic 1"}
			target := model.CommandBarPageContext{EntityType: "task", EntityID: "task-1", DisplayTitle: "Task 1"}
			steps := []model.CommandBarPlanStep{
				{AgentID: agentID, AgentName: "Forge", PlanKind: model.CommandBarPlanKindTaskPipeline, Target: target, Instructions: "Build task 1."},
				{AgentID: agentID, AgentName: "Lens", PlanKind: model.CommandBarPlanKindTaskPipeline, Target: target, Instructions: "Review task 1.", DependsOnStepIndexes: []int{0}},
			}
			plan, err := newCommandBarPlanRecord(workspaceID, "", planID, "run forge then lens", pageContext, steps)
			if err != nil {
				t.Fatalf("build plan record: %v", err)
			}
			plan.RunIDsByStep, _ = json.Marshal(map[int]string{0: runIDs[0], 1: runIDs[1]})
			plan.RunCount = 2
			if err := planRepo.Create(ctx, plan); err != nil {
				t.Fatalf("create plan: %v", err)
			}
			var settled model.AgentRun
			for index, status := range []string{model.AgentRunStatusCompleted, model.AgentRunStatusRunning} {
				trigger, err := buildCommandBarTriggerContext("run forge then lens", pageContext, steps, index, planID)
				if err != nil {
					t.Fatalf("build trigger %d: %v", index, err)
				}
				input, _ := json.Marshal(model.AgentRunInputPayload{Trigger: trigger})
				run := model.AgentRun{
					ID: runIDs[index], WorkspaceID: workspaceID, AgentID: agentID, TargetType: "task", TargetID: "task-1",
					RuntimeKind: "native_sdk", InvocationMode: model.InvocationModeAutonomous, ApprovalState: "not_required",
					PauseReason: model.AgentRunPauseReasonNone, Status: status, Input: input, OutputSummary: json.RawMessage("{}"),
				}
				if err := runRepo.Create(ctx, &run); err != nil {
					t.Fatalf("create run %d: %v", index, err)
				}
				settled = run
			}
			settled.Status = tt.settled

			if _, err := service.AdvanceCommandBarPlanForDelegatedRun(ctx, &settled); err != nil {
				t.Fatalf("advance: %v", err)
			}
			updated, err := planRepo.GetByID(ctx, workspaceID, planID)
			if err != nil {
				t.Fatalf("get plan: %v", err)
			}
			if updated.Status != tt.want {
				t.Fatalf("plan status = %q, want %q", updated.Status, tt.want)
			}
		})
	}
}
