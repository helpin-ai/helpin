package service

import (
	"context"
	"log/slog"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// RewardBonusService handles bonus calculation business logic.
type RewardBonusService struct {
	bonusRepo   *repository.RewardBonusRepository
	scoringRepo *repository.RewardScoringRepository
	logger      *slog.Logger
}

// NewRewardBonusService creates a new RewardBonusService.
func NewRewardBonusService(bonusRepo *repository.RewardBonusRepository, scoringRepo *repository.RewardScoringRepository) *RewardBonusService {
	return &RewardBonusService{
		bonusRepo:   bonusRepo,
		scoringRepo: scoringRepo,
		logger:      slog.Default().With("service", "bonus"),
	}
}

// GetCalculations returns bonus calculations for a workspace/quarter.
func (s *RewardBonusService) GetCalculations(ctx context.Context, workspaceID, quarterID string) ([]model.RewardBonusCalculation, error) {
	return s.bonusRepo.GetCalculations(ctx, workspaceID, quarterID)
}

// SaveCalculations saves a batch of bonus calculations.
func (s *RewardBonusService) SaveCalculations(ctx context.Context, calcs []model.RewardBonusCalculation) error {
	if err := s.bonusRepo.SaveCalculations(ctx, calcs); err != nil {
		s.logger.ErrorContext(ctx, "failed to save bonus calculations", "error", err, "count", len(calcs))
		return err
	}
	workspaceID, quarterID := "", ""
	if len(calcs) > 0 {
		workspaceID = calcs[0].WorkspaceID
		quarterID = calcs[0].QuarterID
	}
	s.logger.InfoContext(ctx, "bonus calculations saved", "workspace_id", workspaceID, "quarter_id", quarterID, "count", len(calcs))
	return nil
}

// Lock locks all bonus calculations for a workspace/quarter.
func (s *RewardBonusService) Lock(ctx context.Context, workspaceID, quarterID string) error {
	if err := s.bonusRepo.Lock(ctx, workspaceID, quarterID); err != nil {
		s.logger.ErrorContext(ctx, "failed to lock bonus calculations", "error", err, "workspace_id", workspaceID, "quarter_id", quarterID)
		return err
	}
	s.logger.InfoContext(ctx, "bonus calculations locked", "workspace_id", workspaceID, "quarter_id", quarterID)
	return nil
}

// Unlock unlocks all bonus calculations for a workspace/quarter.
func (s *RewardBonusService) Unlock(ctx context.Context, workspaceID, quarterID string) error {
	if err := s.bonusRepo.Unlock(ctx, workspaceID, quarterID); err != nil {
		s.logger.ErrorContext(ctx, "failed to unlock bonus calculations", "error", err, "workspace_id", workspaceID, "quarter_id", quarterID)
		return err
	}
	s.logger.InfoContext(ctx, "bonus calculations unlocked", "workspace_id", workspaceID, "quarter_id", quarterID)
	return nil
}

// GetFinance returns the quarterly finance settings.
func (s *RewardBonusService) GetFinance(ctx context.Context, workspaceID, quarterID string) (*model.RewardFinanceSettings, error) {
	return s.bonusRepo.GetFinance(ctx, workspaceID, quarterID)
}

// UpsertFinance creates or updates quarterly finance settings.
func (s *RewardBonusService) UpsertFinance(ctx context.Context, req model.UpsertRewardFinanceRequest) (*model.RewardFinanceSettings, error) {
	result, err := s.bonusRepo.UpsertFinance(ctx, req)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to upsert finance settings", "error", err, "workspace_id", req.WorkspaceID, "quarter_id", req.QuarterID)
		return nil, err
	}
	s.logger.InfoContext(ctx, "finance settings upserted", "workspace_id", req.WorkspaceID, "quarter_id", req.QuarterID)
	return result, nil
}

// GetTeamSprintData returns individual checks for a sprint (used in bonus calc context).
func (s *RewardBonusService) GetTeamSprintData(ctx context.Context, sprintID string) ([]model.RewardIndividualCheck, error) {
	return s.scoringRepo.GetChecksBySprint(ctx, sprintID)
}
