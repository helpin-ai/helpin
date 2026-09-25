package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCommandBarProfileBindingKeepsOwnerAndRoutePrivate(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	repo := repository.NewCommandBarPlanRepository(db)
	const workspaceID = "11111111-1111-1111-1111-111111111111"
	const planID = "22222222-2222-2222-2222-222222222222"
	owner := "33333333-3333-3333-3333-333333333333"
	selection := model.AIExecutionSelection{ProfileID: "44444444-4444-4444-4444-444444444444", ProfileOwnerID: &owner, ConnectionScope: "personal", OwnerID: &owner, Route: model.AIProfileRoute{ConnectionID: "private-connection"}}
	page := model.CommandBarPageContext{EntityType: "epic", EntityID: "epic-1"}
	plan, err := newCommandBarPlanRecord(workspaceID, owner, planID, "deliver epic", page, []model.CommandBarPlanStep{{AgentID: "forge", AgentName: "Forge", Target: page}})
	if err != nil {
		t.Fatal(err)
	}
	plan.ProfileBinding, _ = json.Marshal(selection)
	if err := repo.Create(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	service := &AgentService{commandBarPlanRepo: repo}
	ctx, actor, err := service.commandBarAIContext(context.Background(), workspaceID, planID, "another-user")
	if err != nil || actor != owner {
		t.Fatalf("personal owner was not preserved: actor=%q err=%v", actor, err)
	}
	bound, ok := ctx.Value(commandBarAIContextKey{}).(*model.AIExecutionSelection)
	if !ok || bound == nil || bound.Route.ConnectionID != "private-connection" {
		t.Fatalf("trusted route not loaded: %+v", bound)
	}
	if err := requireCommandBarPlanProfileOwner(*plan, "another-user"); err == nil {
		t.Fatal("another user could manually resume the personal binding")
	}
	if err := requireCommandBarPlanProfileOwner(*plan, owner); err != nil {
		t.Fatal(err)
	}
	summary := commandBarPlanSummary(*plan, nil)
	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	if summary.AIProfileID != selection.ProfileID || summary.AIProfileOwnerID != owner {
		t.Fatalf("missing safe profile metadata: %+v", summary)
	}
	if strings.Contains(string(encoded), "private-connection") {
		t.Fatal("private route leaked into plan summary")
	}
}

func TestCommandBarParentLookupPrefersRetriedActiveChild(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	repo := repository.NewAgentRunRepository(db)
	const workspaceID = "11111111-1111-1111-1111-111111111111"
	parentID := "22222222-2222-2222-2222-222222222222"
	for _, item := range []struct {
		id     string
		status string
	}{
		{"33333333-3333-3333-3333-333333333333", model.AgentRunStatusCancelled},
		{"44444444-4444-4444-4444-444444444444", model.AgentRunStatusQueued},
	} {
		if err := repo.Create(context.Background(), &model.AgentRun{
			ID: item.id, WorkspaceID: workspaceID, AgentID: "forge", TargetType: "task", TargetID: "task-1",
			ParentRunID: &parentID, RuntimeKind: "native_sdk", InvocationMode: model.InvocationModeAutonomous,
			ApprovalState: "not_required", PauseReason: model.AgentRunPauseReasonNone, Status: item.status,
			Input: json.RawMessage(`{}`), OutputSummary: json.RawMessage(`{}`),
		}); err != nil {
			t.Fatal(err)
		}
	}
	child, err := repo.FindByParentRunID(context.Background(), workspaceID, parentID)
	if err != nil {
		t.Fatal(err)
	}
	if child == nil || child.Status != model.AgentRunStatusQueued {
		t.Fatalf("returned old terminal child: %+v", child)
	}
}
