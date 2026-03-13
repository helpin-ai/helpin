package service

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// AutomationHealthObserver records built-in automation health observations.
type AutomationHealthObserver interface {
	ObserveSuccess(ctx context.Context, workspaceID, catalogID, scopeType, scopeID string, metrics model.JSONB) error
	ObserveFailure(ctx context.Context, workspaceID, catalogID, scopeType, scopeID, message string, metrics model.JSONB) error
}

// AutomationHealthService is the shared health snapshot writer for built-in automations.
type AutomationHealthService struct {
	repo *repository.AutomationHealthRepository
}

func NewAutomationHealthService(repo *repository.AutomationHealthRepository) *AutomationHealthService {
	return &AutomationHealthService{repo: repo}
}

func (s *AutomationHealthService) ObserveSuccess(ctx context.Context, workspaceID, catalogID, scopeType, scopeID string, metrics model.JSONB) error {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.ObserveSuccess(ctx, workspaceID, catalogID, scopeType, scopeID, metrics)
}

func (s *AutomationHealthService) ObserveFailure(ctx context.Context, workspaceID, catalogID, scopeType, scopeID, message string, metrics model.JSONB) error {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.ObserveFailure(ctx, workspaceID, catalogID, scopeType, scopeID, message, metrics)
}
