package service

import (
	"context"

	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/repository"
)

// BonusService handles bonus calculation business logic.
type BonusService struct {
	bonusRepo   *repository.BonusRepository
	scoringRepo *repository.ScoringRepository
}

// NewBonusService creates a new BonusService.
func NewBonusService(bonusRepo *repository.BonusRepository, scoringRepo *repository.ScoringRepository) *BonusService {
	return &BonusService{bonusRepo: bonusRepo, scoringRepo: scoringRepo}
}

// GetCalculations returns bonus calculations for a workspace/quarter.
func (s *BonusService) GetCalculations(ctx context.Context, workspaceID, quarterID string) ([]model.BonusCalculation, error) {
	return s.bonusRepo.GetCalculations(ctx, workspaceID, quarterID)
}

// SaveCalculations saves a batch of bonus calculations.
func (s *BonusService) SaveCalculations(ctx context.Context, calcs []model.BonusCalculation) error {
	return s.bonusRepo.SaveCalculations(ctx, calcs)
}

// Lock locks all bonus calculations for a workspace/quarter.
func (s *BonusService) Lock(ctx context.Context, workspaceID, quarterID string) error {
	return s.bonusRepo.Lock(ctx, workspaceID, quarterID)
}

// Unlock unlocks all bonus calculations for a workspace/quarter.
func (s *BonusService) Unlock(ctx context.Context, workspaceID, quarterID string) error {
	return s.bonusRepo.Unlock(ctx, workspaceID, quarterID)
}

// GetFinance returns the quarterly finance settings.
func (s *BonusService) GetFinance(ctx context.Context, workspaceID, quarterID string) (*model.FinanceSettings, error) {
	return s.bonusRepo.GetFinance(ctx, workspaceID, quarterID)
}

// UpsertFinance creates or updates quarterly finance settings.
func (s *BonusService) UpsertFinance(ctx context.Context, req model.UpsertFinanceRequest) (*model.FinanceSettings, error) {
	return s.bonusRepo.UpsertFinance(ctx, req)
}

// GetTeamSprintData returns individual checks for a sprint (used in bonus calc context).
func (s *BonusService) GetTeamSprintData(ctx context.Context, sprintID string) ([]model.IndividualCheck, error) {
	return s.scoringRepo.GetChecksBySprint(ctx, sprintID)
}
