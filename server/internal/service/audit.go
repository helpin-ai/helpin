package service

import (
	"context"

	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/repository"
)

// AuditService handles bonus audit log business logic.
type AuditService struct {
	bonusRepo *repository.BonusRepository
}

// NewAuditService creates a new AuditService.
func NewAuditService(bonusRepo *repository.BonusRepository) *AuditService {
	return &AuditService{bonusRepo: bonusRepo}
}

// List returns all audit entries for a workspace/quarter.
func (s *AuditService) List(ctx context.Context, workspaceID, quarterID string) ([]model.BonusAuditLog, error) {
	return s.bonusRepo.ListAuditEntries(ctx, workspaceID, quarterID)
}
