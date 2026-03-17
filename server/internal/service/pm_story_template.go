package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// PMStoryTemplateService contains story template business logic.
type PMStoryTemplateService struct {
	templateRepo *repository.PMStoryTemplateRepository
}

// NewPMStoryTemplateService creates a new PMStoryTemplateService.
func NewPMStoryTemplateService(templateRepo *repository.PMStoryTemplateRepository) *PMStoryTemplateService {
	return &PMStoryTemplateService{templateRepo: templateRepo}
}

// ListByWorkspace lists story templates by workspace.
func (s *PMStoryTemplateService) ListByWorkspace(ctx context.Context, workspaceID string, teamID *string, includeShared bool, archived *bool) ([]model.PMStoryTemplate, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.templateRepo.ListByWorkspace(ctx, workspaceID, repository.PMStoryTemplateListOptions{
		TeamID:        teamID,
		IncludeShared: includeShared,
		Archived:      archived,
	})
}

// GetByID returns a story template by ID.
func (s *PMStoryTemplateService) GetByID(ctx context.Context, id string) (*model.PMStoryTemplate, error) {
	tmpl, err := s.templateRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tmpl == nil {
		return nil, fmt.Errorf("story template not found")
	}
	return tmpl, nil
}

// Create creates a story template after uniqueness validation.
func (s *PMStoryTemplateService) Create(ctx context.Context, req model.CreateStoryTemplateRequest) (*model.PMStoryTemplate, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}
	name := strings.TrimSpace(req.Name)
	teamID := normalizeOptionalID(req.TeamID)
	existing, err := s.templateRepo.GetByName(ctx, req.WorkspaceID, teamID, name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("template name already exists in this scope")
	}

	tmpl := &model.PMStoryTemplate{
		WorkspaceID:    req.WorkspaceID,
		TeamID:         teamID,
		Name:           name,
		Description:    req.Description,
		StoryType:      req.StoryType,
		Priority:       req.Priority,
		Severity:       req.Severity,
		Estimate:       req.Estimate,
		LabelIDs:       req.LabelIDs,
		OwnerMemberID:  normalizeOptionalID(req.OwnerMemberID),
		EpicID:         normalizeOptionalID(req.EpicID),
		SprintID:       normalizeOptionalID(req.SprintID),
		Deadline:       req.Deadline,
		ChecklistItems: req.ChecklistItems,
		ExternalLinks:  req.ExternalLinks,
	}
	if err := s.templateRepo.Create(ctx, tmpl); err != nil {
		return nil, err
	}
	return tmpl, nil
}

// Update updates a story template.
func (s *PMStoryTemplateService) Update(ctx context.Context, id string, req model.UpdateStoryTemplateRequest) (*model.PMStoryTemplate, error) {
	tmpl, err := s.templateRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tmpl == nil {
		return nil, fmt.Errorf("story template not found")
	}

	if req.TeamID != nil {
		tmpl.TeamID = normalizeOptionalID(req.TeamID)
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		tmpl.Name = name
	}
	if req.TeamID != nil || req.Name != nil {
		existing, err := s.templateRepo.GetByName(ctx, tmpl.WorkspaceID, tmpl.TeamID, tmpl.Name)
		if err != nil {
			return nil, err
		}
		if existing != nil && existing.ID != tmpl.ID {
			return nil, fmt.Errorf("template name already exists in this scope")
		}
	}
	if req.Description != nil {
		tmpl.Description = req.Description
	}
	if req.StoryType != nil {
		tmpl.StoryType = req.StoryType
	}
	if req.Priority != nil {
		tmpl.Priority = req.Priority
	}
	if req.Severity != nil {
		tmpl.Severity = req.Severity
	}
	if req.Estimate != nil {
		tmpl.Estimate = req.Estimate
	}
	if req.LabelIDs != nil {
		tmpl.LabelIDs = req.LabelIDs
	}
	if req.OwnerMemberID != nil {
		tmpl.OwnerMemberID = normalizeOptionalID(req.OwnerMemberID)
	}
	if req.EpicID != nil {
		tmpl.EpicID = normalizeOptionalID(req.EpicID)
	}
	if req.SprintID != nil {
		tmpl.SprintID = normalizeOptionalID(req.SprintID)
	}
	if req.Deadline != nil {
		tmpl.Deadline = req.Deadline
	}
	if req.ChecklistItems != nil {
		tmpl.ChecklistItems = req.ChecklistItems
	}
	if req.ExternalLinks != nil {
		tmpl.ExternalLinks = req.ExternalLinks
	}
	if req.Archived != nil {
		tmpl.Archived = *req.Archived
	}

	if err := s.templateRepo.Update(ctx, tmpl); err != nil {
		return nil, err
	}
	return tmpl, nil
}

// Delete deletes a story template.
func (s *PMStoryTemplateService) Delete(ctx context.Context, id string) error {
	return s.templateRepo.Delete(ctx, id)
}
