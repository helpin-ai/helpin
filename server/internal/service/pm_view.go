package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// PMViewService contains view business logic.
type PMViewService struct {
	viewRepo *repository.PMViewRepository
}

// NewPMViewService creates a new PMViewService.
func NewPMViewService(viewRepo *repository.PMViewRepository) *PMViewService {
	return &PMViewService{viewRepo: viewRepo}
}

// List lists views visible to the given user.
func (s *PMViewService) List(ctx context.Context, workspaceID, userID string) ([]model.PMView, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.viewRepo.ListByWorkspace(ctx, workspaceID, userID)
}

// Create creates a new view.
func (s *PMViewService) Create(ctx context.Context, workspaceID, userID string, req model.CreateViewRequest) (*model.PMView, error) {
	if workspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}
	view := &model.PMView{
		WorkspaceID: workspaceID,
		Name:        strings.TrimSpace(req.Name),
		Filters:     req.Filters,
		IsShared:    req.IsShared,
		IsPinned:    req.IsPinned,
		CreatedBy:   userID,
	}
	if view.Filters == nil {
		view.Filters = model.ViewFilters{}
	}
	if err := s.viewRepo.Create(ctx, view); err != nil {
		return nil, err
	}
	return view, nil
}

// Update updates a view after ownership check.
func (s *PMViewService) Update(ctx context.Context, id, userID string, req model.UpdateViewRequest) (*model.PMView, error) {
	view, err := s.viewRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if view == nil {
		return nil, fmt.Errorf("view not found")
	}
	if view.CreatedBy != userID {
		return nil, fmt.Errorf("forbidden: you can only update your own views")
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		view.Name = name
	}
	if req.Filters != nil {
		view.Filters = *req.Filters
	}
	if req.IsShared != nil {
		view.IsShared = *req.IsShared
	}
	if req.IsPinned != nil {
		view.IsPinned = *req.IsPinned
	}
	if req.Position != nil {
		view.Position = *req.Position
	}

	if err := s.viewRepo.Update(ctx, view); err != nil {
		return nil, err
	}
	return view, nil
}

// Delete deletes a view after ownership check.
func (s *PMViewService) Delete(ctx context.Context, id, userID string) error {
	view, err := s.viewRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("view not found")
	}
	if view.CreatedBy != userID {
		return fmt.Errorf("forbidden: you can only delete your own views")
	}
	return s.viewRepo.Delete(ctx, id)
}
