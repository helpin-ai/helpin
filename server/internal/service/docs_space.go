package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
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

// withTeams enriches a space with its team IDs.
func (s *DocsSpaceService) withTeams(ctx context.Context, space *model.DocsSpace) (*model.DocsSpaceWithTeams, error) {
	teamIDs, err := s.spaceRepo.GetTeamIDs(ctx, space.ID)
	if err != nil {
		return nil, err
	}
	if teamIDs == nil {
		teamIDs = []string{}
	}
	return &model.DocsSpaceWithTeams{DocsSpace: *space, TeamIDs: teamIDs}, nil
}

// Create creates a new space.
func (s *DocsSpaceService) Create(ctx context.Context, workspaceID string, req model.CreateDocsSpaceRequest, userID string) (*model.DocsSpaceWithTeams, error) {
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
		DefaultReviewDays: req.DefaultReviewDays,
		CreatedBy:         userID,
	}
	created, err := s.spaceRepo.Create(ctx, space)
	if err != nil {
		return nil, err
	}

	// Set team associations
	if len(req.TeamIDs) > 0 {
		if err := s.spaceRepo.SetTeamIDs(ctx, created.ID, req.TeamIDs); err != nil {
			return nil, err
		}
	}

	return s.withTeams(ctx, created)
}

// GetBySlug returns a space by workspace ID + slug (no auth check, for public use).
func (s *DocsSpaceService) GetBySlug(ctx context.Context, workspaceID, slug string) (*model.DocsSpace, error) {
	return s.spaceRepo.GetBySlug(ctx, workspaceID, slug)
}

// Get returns a space by ID, checking team access for the actor.
func (s *DocsSpaceService) Get(ctx context.Context, id string, actor *authorization.Actor) (*model.DocsSpaceWithTeams, error) {
	space, err := s.spaceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if space == nil {
		return nil, nil
	}

	sw, err := s.withTeams(ctx, space)
	if err != nil {
		return nil, err
	}

	// Check access unless admin/owner
	if actor != nil && actor.Role != "admin" && actor.Role != "owner" {
		if space.Visibility == model.SpaceVisibilityTeamOnly && len(sw.TeamIDs) > 0 {
			hasAccess := false
			for _, tid := range sw.TeamIDs {
				for _, tm := range actor.TeamMemberships {
					if tm.TeamID == tid {
						hasAccess = true
						break
					}
				}
				if hasAccess {
					break
				}
			}
			if !hasAccess {
				return nil, nil // No access — treat as not found
			}
		}
	}

	return sw, nil
}

// GetUnfiltered returns a space by ID without access checks (for internal use).
func (s *DocsSpaceService) GetUnfiltered(ctx context.Context, id string) (*model.DocsSpaceWithTeams, error) {
	space, err := s.spaceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if space == nil {
		return nil, nil
	}
	return s.withTeams(ctx, space)
}

// AccessibleSpaceIDs returns the IDs of spaces the actor can access.
func (s *DocsSpaceService) AccessibleSpaceIDs(ctx context.Context, workspaceID string, actor *authorization.Actor) ([]string, error) {
	if actor != nil && (actor.Role == "admin" || actor.Role == "owner") {
		// Admin/owner can access all spaces — return nil to signal no filtering needed
		return nil, nil
	}

	var teamIDs []string
	if actor != nil {
		for _, tm := range actor.TeamMemberships {
			teamIDs = append(teamIDs, tm.TeamID)
		}
	}
	return s.spaceRepo.AccessibleSpaceIDs(ctx, workspaceID, teamIDs)
}

// List returns spaces for a workspace, filtered by the actor's team access.
// Owners and admins see all spaces. Other roles see workspace_wide spaces
// plus team_only spaces where they are a member of an associated team.
func (s *DocsSpaceService) List(ctx context.Context, workspaceID string, actor *authorization.Actor) ([]model.DocsSpaceWithTeams, error) {
	spaces, err := s.spaceRepo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	// Batch-load team IDs
	spaceIDs := make([]string, len(spaces))
	for i, sp := range spaces {
		spaceIDs[i] = sp.ID
	}
	teamMap, err := s.spaceRepo.GetTeamIDsForSpaces(ctx, spaceIDs)
	if err != nil {
		return nil, err
	}

	// Build accessible set — filter unless admin/owner
	bypassFilter := actor != nil && (actor.Role == "admin" || actor.Role == "owner")

	var actorTeamIDs map[string]bool
	if !bypassFilter && actor != nil {
		actorTeamIDs = make(map[string]bool, len(actor.TeamMemberships))
		for _, tm := range actor.TeamMemberships {
			actorTeamIDs[tm.TeamID] = true
		}
	}

	var result []model.DocsSpaceWithTeams
	for _, sp := range spaces {
		tids := teamMap[sp.ID]
		if tids == nil {
			tids = []string{}
		}

		// Check access
		if !bypassFilter {
			if sp.Visibility == model.SpaceVisibilityTeamOnly && len(tids) > 0 {
				// Space is team_only — user needs membership in at least one team
				hasAccess := false
				for _, tid := range tids {
					if actorTeamIDs[tid] {
						hasAccess = true
						break
					}
				}
				if !hasAccess {
					continue
				}
			}
			// workspace_wide or team_only with no teams set → accessible to all
		}

		result = append(result, model.DocsSpaceWithTeams{DocsSpace: sp, TeamIDs: tids})
	}
	if result == nil {
		result = []model.DocsSpaceWithTeams{}
	}
	return result, nil
}

// Update updates a space.
func (s *DocsSpaceService) Update(ctx context.Context, id string, req model.UpdateDocsSpaceRequest) (*model.DocsSpaceWithTeams, error) {
	space, err := s.spaceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if space == nil {
		return nil, fmt.Errorf("space not found")
	}
	if space.IsSystem {
		// System spaces can update name, icon, visibility, type but not slug.
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
	if req.Type != nil {
		updates["type"] = *req.Type
	}
	if req.Visibility != nil {
		updates["visibility"] = *req.Visibility
	}
	if req.DefaultReviewDays != nil {
		updates["default_review_days"] = *req.DefaultReviewDays
	}
	if len(updates) > 0 {
		space, err = s.spaceRepo.Update(ctx, id, updates)
		if err != nil {
			return nil, err
		}
	}

	// Update team associations if explicitly set
	if req.SetTeamIDs {
		if err := s.spaceRepo.SetTeamIDs(ctx, id, req.TeamIDs); err != nil {
			return nil, err
		}
	}

	return s.withTeams(ctx, space)
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
func (s *DocsSpaceService) Restore(ctx context.Context, id string) (*model.DocsSpaceWithTeams, error) {
	space, err := s.spaceRepo.Restore(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.withTeams(ctx, space)
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
		Name     string
		Slug     string
		Type     string
		Position int
	}{
		{"Company Wiki", "company-wiki", model.SpaceTypeInternal, 0},
		{"Engineering", "engineering", model.SpaceTypeInternal, 1},
		{"Product Specs", "product-specs", model.SpaceTypeInternal, 2},
		{"Support Knowledge", "support-knowledge", model.SpaceTypeInternal, 3},
		{"Marketing", "marketing", model.SpaceTypeInternal, 4},
		{"Sales", "sales", model.SpaceTypeInternal, 5},
		{"HR & People", "hr-people", model.SpaceTypeInternal, 6},
		{"Operations", "operations", model.SpaceTypeInternal, 7},
		{"Help Center", "help-center", model.SpaceTypeExternalCapable, 8},
		{"Developer / API Docs", "developer-api-docs", model.SpaceTypeExternalCapable, 9},
	}

	for _, d := range defaults {
		space := &model.DocsSpace{
			WorkspaceID: workspaceID,
			Name:        d.Name,
			Slug:        d.Slug,
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        d.Type,
			IsSystem:    true,
			Position:    d.Position,
			CreatedBy:   userID,
		}
		if _, err := s.spaceRepo.Create(ctx, space); err != nil {
			return fmt.Errorf("seed space %s: %w", d.Name, err)
		}
	}
	return nil
}

// EnsureSystemSpace returns a system space by slug, creating it if needed.
func (s *DocsSpaceService) EnsureSystemSpace(ctx context.Context, workspaceID, userID, slug, name, spaceType string, position int) (*model.DocsSpace, error) {
	existing, err := s.spaceRepo.GetBySlug(ctx, workspaceID, slug)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	space := &model.DocsSpace{
		WorkspaceID: workspaceID,
		Name:        name,
		Slug:        slug,
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		Type:        spaceType,
		IsSystem:    true,
		Position:    position,
		CreatedBy:   userID,
	}
	return s.spaceRepo.Create(ctx, space)
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
