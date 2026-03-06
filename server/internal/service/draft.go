package service

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// RewardDraftService handles goal draft business logic.
type RewardDraftService struct {
	draftRepo *repository.RewardDraftRepository
}

// NewRewardDraftService creates a new RewardDraftService.
func NewRewardDraftService(draftRepo *repository.RewardDraftRepository) *RewardDraftService {
	return &RewardDraftService{draftRepo: draftRepo}
}

// Create creates a new goal draft.
func (s *RewardDraftService) Create(ctx context.Context, req model.CreateRewardDraftRequest, createdBy string) (*model.RewardGoalDraft, error) {
	if req.WorkspaceID == "" || req.QuarterID == "" {
		return nil, fmt.Errorf("workspace_id and quarter_id are required")
	}
	return s.draftRepo.Create(ctx, req.WorkspaceID, req.QuarterID, createdBy, req.DraftData)
}

// List returns all drafts for a workspace/quarter.
func (s *RewardDraftService) List(ctx context.Context, workspaceID, quarterID string) ([]model.RewardGoalDraft, error) {
	return s.draftRepo.List(ctx, workspaceID, quarterID)
}

// Get returns a single draft.
func (s *RewardDraftService) Get(ctx context.Context, id string) (*model.RewardGoalDraft, error) {
	d, err := s.draftRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, fmt.Errorf("draft not found")
	}
	return d, nil
}

// Update modifies a draft.
func (s *RewardDraftService) Update(ctx context.Context, id string, req model.UpdateRewardDraftRequest) (*model.RewardGoalDraft, error) {
	return s.draftRepo.Update(ctx, id, req.DraftData, req.Status)
}

// Delete removes a draft.
func (s *RewardDraftService) Delete(ctx context.Context, id string) error {
	return s.draftRepo.Delete(ctx, id)
}
