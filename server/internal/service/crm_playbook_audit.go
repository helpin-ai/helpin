package service

import (
	"context"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func (s *CRMPlaybookExecutionService) AgentUsage(ctx context.Context, ws, id string) ([]model.CRMPlaybookAgentUsage, error) {
	if _, err := s.playbooks.authorize(ctx, ws, authorization.PermCRMRead); err != nil {
		return nil, err
	}
	if !validSituationID(id) {
		return nil, ErrCRMPlaybookInput
	}
	store, ok := s.store.(interface {
		AgentUsage(context.Context, string, string) ([]model.CRMPlaybookAgentUsage, error)
	})
	if !ok {
		return nil, ErrCRMPlaybookRuntimeUnavailable
	}
	return store.AgentUsage(ctx, ws, id)
}

func (s *CRMPlaybookExecutionService) AutomationActivity(ctx context.Context, ws, id string, page int) (*model.CRMPlaybookAutomationActivityPage, error) {
	if _, err := s.playbooks.Get(ctx, ws, id); err != nil {
		return nil, err
	}
	if page < 1 || page > 10000 {
		return nil, ErrCRMPlaybookInput
	}
	store, ok := s.store.(interface {
		AutomationActivity(context.Context, string, string, int) (*model.CRMPlaybookAutomationActivityPage, error)
	})
	if !ok {
		return nil, ErrCRMPlaybookRuntimeUnavailable
	}
	return store.AutomationActivity(ctx, ws, id, page)
}
