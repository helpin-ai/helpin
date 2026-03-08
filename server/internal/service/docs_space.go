package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// DocsSpaceService handles business logic for docs spaces.
type DocsSpaceService struct {
	spaceRepo *repository.DocsSpaceRepository
}

// NewDocsSpaceService creates a new DocsSpaceService.
func NewDocsSpaceService(spaceRepo *repository.DocsSpaceRepository) *DocsSpaceService {
	return &DocsSpaceService{spaceRepo: spaceRepo}
}

// Create creates a new space.
func (s *DocsSpaceService) Create(ctx context.Context, workspaceID string, req model.CreateDocsSpaceRequest, userID string) (*model.DocsSpace, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if req.Slug == "" {
		req.Slug = slugify(req.Name)
	}
	if req.Visibility == "" {
		req.Visibility = model.SpaceVisibilityWorkspaceWide
	}
	if req.Type == "" {
		req.Type = model.SpaceTypeInternal
	}
	if req.Type != model.SpaceTypeInternal && req.Type != model.SpaceTypeExternalCapable {
		return nil, fmt.Errorf("invalid space type: %s", req.Type)
	}

	space := &model.DocsSpace{
		WorkspaceID:       workspaceID,
		TeamID:            req.TeamID,
		Name:              req.Name,
		Slug:              req.Slug,
		Icon:              req.Icon,
		Visibility:        req.Visibility,
		Type:              req.Type,
		RestrictToOwners:  req.RestrictToOwners,
		DefaultReviewDays: req.DefaultReviewDays,
		CreatedBy:         userID,
	}
	return s.spaceRepo.Create(ctx, space)
}

// Get returns a space by ID.
func (s *DocsSpaceService) Get(ctx context.Context, id string) (*model.DocsSpace, error) {
	return s.spaceRepo.GetByID(ctx, id)
}

// List returns all spaces for a workspace.
func (s *DocsSpaceService) List(ctx context.Context, workspaceID string) ([]model.DocsSpace, error) {
	return s.spaceRepo.ListByWorkspace(ctx, workspaceID)
}

// Update updates a space.
func (s *DocsSpaceService) Update(ctx context.Context, id string, req model.UpdateDocsSpaceRequest) (*model.DocsSpace, error) {
	space, err := s.spaceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if space == nil {
		return nil, fmt.Errorf("space not found")
	}
	if space.IsSystem {
		// System spaces can update name, icon, visibility, restrict_to_owners but not slug or type.
		if req.Slug != nil {
			return nil, fmt.Errorf("cannot change slug of a system space")
		}
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Slug != nil {
		updates["slug"] = *req.Slug
	}
	if req.Icon != nil {
		updates["icon"] = *req.Icon
	}
	if req.Visibility != nil {
		updates["visibility"] = *req.Visibility
	}
	if req.RestrictToOwners != nil {
		updates["restrict_to_owners"] = *req.RestrictToOwners
	}
	if req.DefaultReviewDays != nil {
		updates["default_review_days"] = *req.DefaultReviewDays
	}
	if len(updates) == 0 {
		return space, nil
	}
	return s.spaceRepo.Update(ctx, id, updates)
}

// Delete soft-deletes a space.
func (s *DocsSpaceService) Delete(ctx context.Context, id string) error {
	space, err := s.spaceRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if space == nil {
		return fmt.Errorf("space not found")
	}
	if space.IsSystem {
		return fmt.Errorf("cannot delete a system space")
	}
	return s.spaceRepo.Delete(ctx, id)
}

// Restore restores a soft-deleted space.
func (s *DocsSpaceService) Restore(ctx context.Context, id string) (*model.DocsSpace, error) {
	return s.spaceRepo.Restore(ctx, id)
}

// SeedDefaultSpaces creates the default spaces for a workspace (idempotent).
func (s *DocsSpaceService) SeedDefaultSpaces(ctx context.Context, workspaceID, userID string) error {
	existing, err := s.spaceRepo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil // Already seeded.
	}

	defaults := []struct {
		Name             string
		Slug             string
		Type             string
		RestrictToOwners bool
		Position         int
	}{
		{"Company Wiki", "company-wiki", model.SpaceTypeInternal, false, 0},
		{"Engineering", "engineering", model.SpaceTypeInternal, false, 1},
		{"Support Knowledge", "support-knowledge", model.SpaceTypeInternal, false, 2},
		{"Marketing", "marketing", model.SpaceTypeInternal, false, 3},
		{"Sales", "sales", model.SpaceTypeInternal, false, 4},
		{"HR & People", "hr-people", model.SpaceTypeInternal, true, 5},
		{"Operations", "operations", model.SpaceTypeInternal, false, 6},
		{"Help Center", "help-center", model.SpaceTypeExternalCapable, false, 7},
		{"Developer / API Docs", "developer-api-docs", model.SpaceTypeExternalCapable, false, 8},
	}

	for _, d := range defaults {
		space := &model.DocsSpace{
			WorkspaceID:      workspaceID,
			Name:             d.Name,
			Slug:             d.Slug,
			Visibility:       model.SpaceVisibilityWorkspaceWide,
			Type:             d.Type,
			RestrictToOwners: d.RestrictToOwners,
			IsSystem:         true,
			Position:         d.Position,
			CreatedBy:        userID,
		}
		if _, err := s.spaceRepo.Create(ctx, space); err != nil {
			return fmt.Errorf("seed space %s: %w", d.Name, err)
		}
	}
	return nil
}

// slugify creates a URL-safe slug from a name.
func slugify(name string) string {
	s := strings.ToLower(name)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == ' ' {
			return r
		}
		return -1
	}, s)
	s = strings.Join(strings.Fields(s), "-")
	return s
}
