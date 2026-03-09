package service

import (
	"context"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestResolvePlanningWebSearchConfigAppliesOverrides(t *testing.T) {
	current := &model.WorkspaceSettings{
		PlanningWebSearchEnabled:  false,
		PlanningWebSearchProvider: model.PlanningWebSearchProviderBrave,
	}
	enabled := true
	provider := "brave"

	gotEnabled, gotProvider := resolvePlanningWebSearchConfig(current, model.UpdateSystemSettingsRequest{
		PlanningWebSearchEnabled:  &enabled,
		PlanningWebSearchProvider: &provider,
	})
	if !gotEnabled {
		t.Fatal("expected planning web search to be enabled")
	}
	if gotProvider != model.PlanningWebSearchProviderBrave {
		t.Fatalf("expected provider brave, got %q", gotProvider)
	}
}

func TestValidatePlanningWebSearchSettingsRejectsBraveWithoutAPIKey(t *testing.T) {
	service := &SettingsService{
		braveSearchAPIKey: "",
	}
	enabled := true
	err := service.validatePlanningWebSearchSettings(context.Background(), "ws-1", model.UpdateSystemSettingsRequest{
		PlanningWebSearchEnabled: &enabled,
	})
	if err == nil {
		t.Fatal("expected missing Brave API key error")
	}
	if !strings.Contains(err.Error(), "BRAVE_SEARCH_API_KEY") {
		t.Fatalf("expected missing key error, got %v", err)
	}
}
