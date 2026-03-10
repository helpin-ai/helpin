package service

import (
	"context"
	"log/slog"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// RewardAuditService handles reward audit log business logic.
type RewardAuditService struct {
	bonusRepo *repository.RewardBonusRepository
	logger    *slog.Logger
}

// NewRewardAuditService creates a new RewardAuditService.
func NewRewardAuditService(bonusRepo *repository.RewardBonusRepository) *RewardAuditService {
	return &RewardAuditService{
		bonusRepo: bonusRepo,
		logger:    slog.Default().With("service", "audit"),
	}
}

// List returns all audit entries for a workspace/quarter.
func (s *RewardAuditService) List(ctx context.Context, workspaceID, quarterID string) ([]model.RewardAuditLog, error) {
	entries, err := s.bonusRepo.ListAuditEntries(ctx, workspaceID, quarterID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to list audit entries", "error", err, "workspace_id", workspaceID, "quarter_id", quarterID)
		return nil, err
	}
	s.logger.DebugContext(ctx, "audit entries listed", "workspace_id", workspaceID, "quarter_id", quarterID, "count", len(entries))
	return entries, nil
}
