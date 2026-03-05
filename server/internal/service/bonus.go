package service

import (
	"context"

	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/repository"
)

// RewardBonusService handles bonus calculation business logic.
type RewardBonusService struct {
	bonusRepo   *repository.RewardBonusRepository
	scoringRepo *repository.RewardScoringRepository
}

// NewRewardBonusService creates a new RewardBonusService.
func NewRewardBonusService(bonusRepo *repository.RewardBonusRepository, scoringRepo *repository.RewardScoringRepository) *RewardBonusService {
	return &RewardBonusService{bonusRepo: bonusRepo, scoringRepo: scoringRepo}
}

// GetCalculations returns bonus calculations for a workspace/quarter.
func (s *RewardBonusService) GetCalculations(ctx context.Context, workspaceID, quarterID string) ([]model.RewardBonusCalculation, error) {
	return s.bonusRepo.GetCalculations(ctx, workspaceID, quarterID)
}

// SaveCalculations saves a batch of bonus calculations.
func (s *RewardBonusService) SaveCalculations(ctx context.Context, calcs []model.RewardBonusCalculation) error {
	return s.bonusRepo.SaveCalculations(ctx, calcs)
}

// Lock locks all bonus calculations for a workspace/quarter.
func (s *RewardBonusService) Lock(ctx context.Context, workspaceID, quarterID string) error {
	return s.bonusRepo.Lock(ctx, workspaceID, quarterID)
}

// Unlock unlocks all bonus calculations for a workspace/quarter.
func (s *RewardBonusService) Unlock(ctx context.Context, workspaceID, quarterID string) error {
	return s.bonusRepo.Unlock(ctx, workspaceID, quarterID)
}

// GetFinance returns the quarterly finance settings.
func (s *RewardBonusService) GetFinance(ctx context.Context, workspaceID, quarterID string) (*model.RewardFinanceSettings, error) {
	return s.bonusRepo.GetFinance(ctx, workspaceID, quarterID)
}

// UpsertFinance creates or updates quarterly finance settings.
func (s *RewardBonusService) UpsertFinance(ctx context.Context, req model.UpsertRewardFinanceRequest) (*model.RewardFinanceSettings, error) {
	return s.bonusRepo.UpsertFinance(ctx, req)
}

// GetTeamSprintData returns individual checks for a sprint (used in bonus calc context).
func (s *RewardBonusService) GetTeamSprintData(ctx context.Context, sprintID string) ([]model.RewardIndividualCheck, error) {
	return s.scoringRepo.GetChecksBySprint(ctx, sprintID)
}
