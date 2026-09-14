package service

import (
	"context"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type connectionPolicyFunc func(context.Context, string, *model.AIConnection) (*model.AIExecutionPolicySnapshot, error)

func (f connectionPolicyFunc) ResolveConnectionPolicy(ctx context.Context, ws string, c *model.AIConnection) (*model.AIExecutionPolicySnapshot, error) {
	return f(ctx, ws, c)
}

func TestAIProfilePolicyRejectionNeverTriggersFallback(t *testing.T) {
	s, primary, fallback := setupAIProfileTest(t)
	ctx := context.Background()
	p, err := s.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{
		Name: "Team", Scope: "workspace", Primary: primary, Fallback: &fallback})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.connections.repo.WithLocked(ctx, primary.ConnectionID, func(c *model.AIConnection) error {
		c.Status = "disconnected"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	denied := errors.New("BYOK disabled")
	var visited []string
	s.SetAdmissionPolicy(connectionPolicyFunc(func(_ context.Context, _ string, c *model.AIConnection) (*model.AIExecutionPolicySnapshot, error) {
		visited = append(visited, c.ID)
		return nil, denied
	}))
	_, _, err = s.Resolve(ctx, "workspace", "owner", AIProfileSelectionRequest{ProfileID: p.ID})
	if !errors.Is(err, denied) || len(visited) != 1 || visited[0] != primary.ConnectionID {
		t.Fatalf("policy rejection attempted fallback: %v, %v", err, visited)
	}
}

func TestAIProfileFallbackSnapshotsActualFundingAndRestoreKeepsIt(t *testing.T) {
	for _, fallbackFunding := range []aiusage.FundingMode{aiusage.FundingHelpinHosted, aiusage.FundingCustomerFlat} {
		t.Run(string(fallbackFunding), func(t *testing.T) {
			s, primary, fallback := setupAIProfileTest(t)
			ctx := context.Background()
			p, err := s.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{
				Name: "Team", Scope: "workspace", Primary: primary, Fallback: &fallback})
			if err != nil {
				t.Fatal(err)
			}
			if err := s.connections.repo.WithLocked(ctx, primary.ConnectionID, func(c *model.AIConnection) error {
				c.Status = "disconnected"
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			calls := 0
			s.SetAdmissionPolicy(connectionPolicyFunc(func(_ context.Context, _ string, c *model.AIConnection) (*model.AIExecutionPolicySnapshot, error) {
				calls++
				funding := aiusage.FundingCustomerFlat
				if fallbackFunding == funding {
					funding = aiusage.FundingHelpinHosted
				}
				if c.ID == fallback.ConnectionID {
					funding = fallbackFunding
				}
				return &model.AIExecutionPolicySnapshot{Mode: "ee", FundingMode: funding, FlatTariff: testFlatTariff(1_000_000)}, nil
			}))
			selection, credential, err := s.Resolve(ctx, "workspace", "owner", AIProfileSelectionRequest{ProfileID: p.ID})
			if err != nil {
				t.Fatal(err)
			}
			if calls != 2 || selection.Policy.FundingMode != fallbackFunding || credential.APIKey != "Fallback-key" {
				t.Fatalf("incorrect fallback policy: %+v, calls %d", selection.Policy, calls)
			}
			s.SetAdmissionPolicy(connectionPolicyFunc(func(context.Context, string, *model.AIConnection) (*model.AIExecutionPolicySnapshot, error) {
				t.Fatal("restore re-evaluated admission policy")
				return nil, nil
			}))
			if _, err := s.Restore(ctx, "workspace", "owner", selection); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAIProfileReviewDoesNotFreezeBillingBeforeLaunch(t *testing.T) {
	s, primary, _ := setupAIProfileTest(t)
	ctx := context.Background()
	p, err := s.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Team", Scope: "workspace", Primary: primary})
	if err != nil {
		t.Fatal(err)
	}
	reviewed, err := s.ReviewSharedSelection(ctx, "workspace", p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reviewed.Policy != nil {
		t.Fatal("review prematurely froze billing")
	}
	s.SetAdmissionPolicy(connectionPolicyFunc(func(context.Context, string, *model.AIConnection) (*model.AIExecutionPolicySnapshot, error) {
		return &model.AIExecutionPolicySnapshot{Mode: "ee", FundingMode: aiusage.FundingCustomerFlat, FlatTariff: testFlatTariff(500_000)}, nil
	}))
	admitted, err := s.AdmitReviewedSelection(ctx, "workspace", reviewed)
	if err != nil {
		t.Fatal(err)
	}
	if reviewed.Policy != nil || admitted.Policy == nil || admitted.Route.ConnectionID != reviewed.Route.ConnectionID {
		t.Fatal("launch mutated reviewed route or omitted its policy")
	}
}
