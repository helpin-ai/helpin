package service

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// AIConnectionAdmissionPolicy is supplied by the edition, not by a caller.
// An error is terminal and must never trigger a fallback.
type AIConnectionAdmissionPolicy interface {
	ResolveConnectionPolicy(context.Context, string, *model.AIConnection) (*model.AIExecutionPolicySnapshot, error)
}

func (s *AIProfileService) SetAdmissionPolicy(policy AIConnectionAdmissionPolicy) *AIProfileService {
	s.admissionPolicy = policy
	if s.connections != nil {
		s.connections.admissionPolicy = policy
	}
	return s
}

func (s *AIProfileService) selectionPolicy(ctx context.Context, workspace string, route model.AIProfileRoute) (*model.AIExecutionPolicySnapshot, error) {
	c, err := s.connections.repo.Get(ctx, route.ConnectionID)
	if err != nil {
		return nil, err
	}
	if c == nil || c.WorkspaceID != workspace || c.Provider != route.Model.Provider {
		return nil, ErrAIConnection
	}
	if s.admissionPolicy != nil {
		return s.admissionPolicy.ResolveConnectionPolicy(ctx, workspace, c)
	}
	return &model.AIExecutionPolicySnapshot{Mode: "community", FundingMode: aiusage.FundingCustomerUnbilled}, nil
}

// AdmitReviewedSelection freezes current edition policy on a copy of the
// reviewed route. Billing configuration is accepted at launch, not at review.
func (s *AIProfileService) AdmitReviewedSelection(ctx context.Context, workspace string, reviewed *model.AIExecutionSelection) (*model.AIExecutionSelection, error) {
	if reviewed == nil {
		return nil, ErrAIConnection
	}
	selection := *reviewed
	policy, err := s.selectionPolicy(ctx, workspace, selection.Route)
	if err != nil {
		return nil, err
	}
	if err := s.checkExecutionRoute(ctx, selection.Route); err != nil {
		return nil, err
	}
	selection.Policy = policy
	return &selection, nil
}
