package service

import (
	"context"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestConnectionPolicyViewCommunityNeedsNoTariff(t *testing.T) {
	view, err := connectionPolicyView(context.Background(), nil, "workspace", &model.AIConnection{WorkspaceID: "workspace", Funding: "customer"})
	if err != nil || !view.Allowed || view.Pricing.Mode != "community" || view.Pricing.FlatTariff != nil {
		t.Fatalf("community policy: %+v, %v", view, err)
	}
}

func TestConnectionPolicyViewReportsDisabledSelections(t *testing.T) {
	policy := connectionPolicyFunc(func(context.Context, string, *model.AIConnection) (*model.AIExecutionPolicySnapshot, error) {
		return nil, ErrAIConnectionPolicyUnavailable
	})
	view, err := connectionPolicyView(context.Background(), policy, "workspace", &model.AIConnection{WorkspaceID: "workspace"})
	if err != nil || view.Allowed || view.Pricing != nil || view.Message == "" {
		t.Fatalf("disabled policy: %+v, %v", view, err)
	}
}

func TestConnectionPolicyViewKeepsInfrastructureFailuresAsErrors(t *testing.T) {
	failure := errors.New("policy store unavailable")
	policy := connectionPolicyFunc(func(context.Context, string, *model.AIConnection) (*model.AIExecutionPolicySnapshot, error) {
		return nil, failure
	})
	view, err := connectionPolicyView(context.Background(), policy, "workspace", &model.AIConnection{WorkspaceID: "workspace"})
	if !errors.Is(err, failure) || view != nil {
		t.Fatalf("infrastructure failure became availability: %+v, %v", view, err)
	}
}

func TestConnectionPolicyViewRejectsMissingPolicyResult(t *testing.T) {
	policy := connectionPolicyFunc(func(context.Context, string, *model.AIConnection) (*model.AIExecutionPolicySnapshot, error) {
		return nil, nil
	})
	if _, err := connectionPolicyView(context.Background(), policy, "workspace", &model.AIConnection{WorkspaceID: "workspace"}); err == nil {
		t.Fatal("accepted a missing policy result")
	}
}

func TestConnectionPolicyViewDoesNotDiscloseAnotherWorkspacePolicy(t *testing.T) {
	policy := connectionPolicyFunc(func(context.Context, string, *model.AIConnection) (*model.AIExecutionPolicySnapshot, error) {
		t.Fatal("cross-workspace policy read")
		return nil, nil
	})
	view, err := connectionPolicyView(context.Background(), policy, "workspace", &model.AIConnection{WorkspaceID: "other"})
	if err != nil || view.Allowed || view.Pricing != nil {
		t.Fatalf("cross-workspace policy: %+v, %v", view, err)
	}
}

func TestAIProfileListShowsIndependentFundingWithoutPersistingPresentation(t *testing.T) {
	s, primary, fallback := setupAIProfileTest(t)
	ctx := context.Background()
	profile, err := s.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Team", Scope: "workspace", Primary: primary, Fallback: &fallback})
	if err != nil {
		t.Fatal(err)
	}
	s.SetAdmissionPolicy(connectionPolicyFunc(func(_ context.Context, _ string, c *model.AIConnection) (*model.AIExecutionPolicySnapshot, error) {
		if c.ID == primary.ConnectionID {
			return &model.AIExecutionPolicySnapshot{Mode: "ee", FundingMode: aiusage.FundingHelpinHosted}, nil
		}
		return &model.AIExecutionPolicySnapshot{Mode: "ee", FundingMode: aiusage.FundingCustomerFlat, FlatTariff: testFlatTariff(0)}, nil
	}))
	profiles, err := s.List(ctx, "workspace", "owner")
	if err != nil || len(profiles) != 1 {
		t.Fatalf("list profiles: %v", err)
	}
	p := profiles[0]
	if !p.PrimaryPolicy.Allowed || p.PrimaryPolicy.Pricing.FundingMode != aiusage.FundingHelpinHosted {
		t.Fatalf("primary: %+v", p.PrimaryPolicy)
	}
	if !p.FallbackPolicy.Allowed || p.FallbackPolicy.Pricing.FlatTariff.MicrousdPerMillion == nil || *p.FallbackPolicy.Pricing.FlatTariff.MicrousdPerMillion != 0 {
		t.Fatalf("fallback: %+v", p.FallbackPolicy)
	}
	stored, err := s.repo.Get(ctx, "workspace", profile.ID)
	if err != nil || stored.PrimaryPolicy != nil || stored.FallbackPolicy != nil {
		t.Fatalf("presentation was persisted: %+v, %v", stored, err)
	}
}
