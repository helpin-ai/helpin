package service

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// RewardAuditService handles reward audit log business logic.
type RewardAuditService struct {
	bonusRepo *repository.RewardBonusRepository
}

// NewRewardAuditService creates a new RewardAuditService.
func NewRewardAuditService(bonusRepo *repository.RewardBonusRepository) *RewardAuditService {
	return &RewardAuditService{bonusRepo: bonusRepo}
}

// List returns all audit entries for a workspace/quarter.
func (s *RewardAuditService) List(ctx context.Context, workspaceID, quarterID string) ([]model.RewardAuditLog, error) {
	return s.bonusRepo.ListAuditEntries(ctx, workspaceID, quarterID)
}
