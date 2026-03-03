package service

import (
	"context"
	"fmt"

	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/repository"
)

// DraftService handles goal draft business logic.
type DraftService struct {
	draftRepo *repository.DraftRepository
}

// NewDraftService creates a new DraftService.
func NewDraftService(draftRepo *repository.DraftRepository) *DraftService {
	return &DraftService{draftRepo: draftRepo}
}

// Create creates a new goal draft.
func (s *DraftService) Create(ctx context.Context, req model.CreateDraftRequest, createdBy string) (*model.GoalDraft, error) {
	if req.WorkspaceID == "" || req.QuarterID == "" {
		return nil, fmt.Errorf("workspace_id and quarter_id are required")
	}
	return s.draftRepo.Create(ctx, req.WorkspaceID, req.QuarterID, createdBy, req.DraftData)
}

// List returns all drafts for a workspace/quarter.
func (s *DraftService) List(ctx context.Context, workspaceID, quarterID string) ([]model.GoalDraft, error) {
	return s.draftRepo.List(ctx, workspaceID, quarterID)
}

// Get returns a single draft.
func (s *DraftService) Get(ctx context.Context, id string) (*model.GoalDraft, error) {
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
func (s *DraftService) Update(ctx context.Context, id string, req model.UpdateDraftRequest) (*model.GoalDraft, error) {
	return s.draftRepo.Update(ctx, id, req.DraftData, req.Status)
}

// Delete removes a draft.
func (s *DraftService) Delete(ctx context.Context, id string) error {
	return s.draftRepo.Delete(ctx, id)
}
