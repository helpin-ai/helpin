package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAuthorizeCommandBarDispatchAllowsMixedTargetsForEditor(t *testing.T) {
	req := httptest.NewRequest("POST", "/command-bar/plans/dispatch", nil)
	actor := &authorization.Actor{Role: model.RoleMember}
	req = req.WithContext(authorization.WithActor(req.Context(), actor))

	err := authorizeCommandBarDispatch(req, model.CommandBarDispatchRequest{
		PageContext: model.CommandBarPageContext{EntityType: "task"},
		Steps: []model.CommandBarPlanStep{
			{Target: model.CommandBarPageContext{EntityType: "task"}},
			{Target: model.CommandBarPageContext{EntityType: "document"}},
			{Target: model.CommandBarPageContext{EntityType: "crm_deal"}},
		},
	})
	if err != nil {
		t.Fatalf("expected mixed target dispatch to be authorized: %v", err)
	}
}

func TestAuthorizeCommandBarDispatchAllowsRepositoryTarget(t *testing.T) {
	req := httptest.NewRequest("POST", "/command-bar/plans/dispatch", nil)
	actor := &authorization.Actor{Role: model.RoleMember}
	req = req.WithContext(authorization.WithActor(req.Context(), actor))

	err := authorizeCommandBarDispatch(req, model.CommandBarDispatchRequest{
		PageContext: model.CommandBarPageContext{EntityType: "repository"},
		Steps: []model.CommandBarPlanStep{
			{Target: model.CommandBarPageContext{EntityType: "repository"}},
		},
	})
	if err != nil {
		t.Fatalf("expected repository target dispatch to be authorized: %v", err)
	}
}

func TestAuthorizeCommandBarDispatchRejectsMissingTargetPermission(t *testing.T) {
	req := httptest.NewRequest("POST", "/command-bar/plans/dispatch", nil)
	actor := &authorization.Actor{Role: model.RoleViewer}
	req = req.WithContext(authorization.WithActor(req.Context(), actor))

	err := authorizeCommandBarDispatch(req, model.CommandBarDispatchRequest{
		PageContext: model.CommandBarPageContext{EntityType: "crm_contact"},
		Steps: []model.CommandBarPlanStep{
			{Target: model.CommandBarPageContext{EntityType: "crm_contact"}},
		},
	})
	if err == nil {
		t.Fatal("expected viewer dispatch to be rejected")
	}
}

func TestAuthorizeCommandBarDispatchFallsBackToPageContextTarget(t *testing.T) {
	req := httptest.NewRequest("POST", "/command-bar/plans/dispatch", nil)
	actor := &authorization.Actor{Role: model.RoleMember}
	req = req.WithContext(authorization.WithActor(req.Context(), actor))

	err := authorizeCommandBarDispatch(req, model.CommandBarDispatchRequest{
		PageContext: model.CommandBarPageContext{EntityType: "document"},
		Steps: []model.CommandBarPlanStep{
			{Target: model.CommandBarPageContext{}},
		},
	})
	if err != nil {
		t.Fatalf("expected page-context fallback target to be authorized: %v", err)
	}
}

func TestAuthorizeCommandBarStepsUsesPersistedRetryStepOffset(t *testing.T) {
	req := httptest.NewRequest("POST", "/command-bar/plans/plan-1/retry", nil)
	actor := &authorization.Actor{Role: model.RoleViewer}
	req = req.WithContext(authorization.WithActor(req.Context(), actor))

	err := authorizeCommandBarSteps(req, model.CommandBarPageContext{EntityType: "task"}, []model.CommandBarPlanStep{
		{Target: model.CommandBarPageContext{EntityType: "task"}},
	}, 2)
	if err == nil {
		t.Fatal("expected viewer retry to be rejected")
	}
	if got := err.Error(); got != "insufficient permission for task target in step 3: requires pm.edit" {
		t.Fatalf("unexpected retry auth error: %q", got)
	}
}
