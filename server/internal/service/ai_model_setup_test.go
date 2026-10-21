package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAIModelVisibilityPreservesDefaultsAndRoutes(t *testing.T) {
	s, primary, fallback := setupAIProfileTest(t)
	ctx := context.Background()
	p, err := s.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Existing variant", Scope: "workspace", Primary: primary, Fallback: &fallback})
	if err != nil {
		t.Fatal(err)
	}
	if p.HiddenFromAskAgent {
		t.Fatal("existing models must start visible")
	}
	if err := s.SetDefault(ctx, "workspace", "owner", p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetVisibility(ctx, "workspace", "teammate", p.ID, p.Revision, true); err == nil {
		t.Fatal("member changed shared visibility")
	}
	updated, err := s.SetVisibility(ctx, "workspace", "owner", p.ID, p.Revision, true)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.HiddenFromAskAgent || updated.Fallback == nil || updated.Name != p.Name {
		t.Fatalf("configuration lost: %+v", updated)
	}
	if _, err := s.SetVisibility(ctx, "workspace", "owner", p.ID, p.Revision, false); err == nil {
		t.Fatal("stale edit accepted")
	}
	selection, _, err := s.Resolve(ctx, "workspace", "", AIProfileSelectionRequest{Unattended: true})
	if err != nil || selection.ProfileID != p.ID || selection.Route.ConnectionID != primary.ConnectionID {
		t.Fatalf("hidden model stopped default runs: %v", err)
	}
}

func TestEnableAIModelIsIdempotentAndPreservesExistingVariant(t *testing.T) {
	s, primary, fallback := setupAIProfileTest(t)
	ctx := context.Background()
	existing, err := s.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Custom variant", Scope: "workspace", Primary: primary, Fallback: &fallback})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetVisibility(ctx, "workspace", "owner", existing.ID, existing.Revision, true); err != nil {
		t.Fatal(err)
	}
	request := model.EnableAIModelRequest{ConnectionID: primary.ConnectionID, Model: primary.Model.Model, Name: "Provider label"}
	enabled, err := s.EnableModel(ctx, "workspace", "owner", request)
	if err != nil || enabled.ID != existing.ID || enabled.HiddenFromAskAgent || enabled.Name != "Custom variant" || enabled.Fallback == nil {
		t.Fatalf("existing variant not preserved: %+v %v", enabled, err)
	}
	if _, err := s.EnableModel(ctx, "workspace", "teammate", request); err == nil {
		t.Fatal("member enabled workspace model")
	}
	request.Model = "new-model"
	created, err := s.EnableModel(ctx, "workspace", "owner", request)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.EnableModel(ctx, "workspace", "owner", request)
	if err != nil || again.ID != created.ID {
		t.Fatalf("duplicate model created: %+v %v", again, err)
	}
}
