package service

import (
	"context"
	"fmt"

	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/repository"
)

// WorkspaceService handles workspace business logic.
type WorkspaceService struct {
	workspaceRepo *repository.WorkspaceRepository
}

// NewWorkspaceService creates a new WorkspaceService.
func NewWorkspaceService(workspaceRepo *repository.WorkspaceRepository) *WorkspaceService {
	return &WorkspaceService{workspaceRepo: workspaceRepo}
}

// Create creates a workspace and adds the creator as the owner member.
func (s *WorkspaceService) Create(ctx context.Context, req model.CreateWorkspaceRequest, ownerID string) (*model.WorkspaceWithRole, error) {
	if req.Name == "" || req.Slug == "" {
		return nil, fmt.Errorf("name and slug are required")
	}

	ws, err := s.workspaceRepo.Create(ctx, req.Name, req.Slug, ownerID, req.Description)
	if err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}

	_, err = s.workspaceRepo.AddMember(ctx, ws.ID, ownerID, "owner")
	if err != nil {
		return nil, fmt.Errorf("add owner as member: %w", err)
	}

	return &model.WorkspaceWithRole{
		Workspace: *ws,
		Role:      "owner",
	}, nil
}

// List returns all workspaces the user belongs to.
func (s *WorkspaceService) List(ctx context.Context, userID string) ([]model.WorkspaceWithRole, error) {
	return s.workspaceRepo.List(ctx, userID)
}

// GetBySlug returns a workspace by its slug.
func (s *WorkspaceService) GetBySlug(ctx context.Context, slug string) (*model.Workspace, error) {
	ws, err := s.workspaceRepo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if ws == nil {
		return nil, fmt.Errorf("workspace not found")
	}
	return ws, nil
}

// GetByID returns a workspace by its ID.
func (s *WorkspaceService) GetByID(ctx context.Context, id string) (*model.Workspace, error) {
	ws, err := s.workspaceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ws == nil {
		return nil, fmt.Errorf("workspace not found")
	}
	return ws, nil
}

// Update modifies a workspace.
func (s *WorkspaceService) Update(ctx context.Context, id string, req model.UpdateWorkspaceRequest) (*model.Workspace, error) {
	return s.workspaceRepo.Update(ctx, id, req.Name, req.Description)
}

// Delete removes a workspace.
func (s *WorkspaceService) Delete(ctx context.Context, id string) error {
	return s.workspaceRepo.Delete(ctx, id)
}

// GetMyRole returns the user's role in a workspace.
func (s *WorkspaceService) GetMyRole(ctx context.Context, workspaceID, userID string) (string, error) {
	return s.workspaceRepo.GetMemberRole(ctx, workspaceID, userID)
}

// GetMyMembership returns the user's full membership in a workspace.
func (s *WorkspaceService) GetMyMembership(ctx context.Context, workspaceID, userID string) (*model.WorkspaceMember, error) {
	m, err := s.workspaceRepo.GetMembership(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("membership not found")
	}
	return m, nil
}
