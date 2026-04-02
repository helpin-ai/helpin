package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// PMTaskTemplateService contains story template business logic.
type PMTaskTemplateService struct {
	templateRepo *repository.PMTaskTemplateRepository
	wsPublisher  *websocket.Publisher
}

// NewPMTaskTemplateService creates a new PMTaskTemplateService.
func NewPMTaskTemplateService(templateRepo *repository.PMTaskTemplateRepository, wsPublisher *websocket.Publisher) *PMTaskTemplateService {
	return &PMTaskTemplateService{templateRepo: templateRepo, wsPublisher: wsPublisher}
}

// ListByWorkspace lists story templates by workspace.
func (s *PMTaskTemplateService) ListByWorkspace(ctx context.Context, workspaceID string, teamID *string, includeShared bool, archived *bool) ([]model.PMStoryTemplate, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.templateRepo.ListByWorkspace(ctx, workspaceID, repository.PMTaskTemplateListOptions{
		TeamID:        teamID,
		IncludeShared: includeShared,
		Archived:      archived,
	})
}

// GetByID returns a story template by ID.
func (s *PMTaskTemplateService) GetByID(ctx context.Context, id string) (*model.PMStoryTemplate, error) {
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
func (s *PMTaskTemplateService) Create(ctx context.Context, req model.CreateStoryTemplateRequest) (*model.PMStoryTemplate, error) {
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
		TaskType:      req.TaskType,
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
	publishWorkspaceEvent(s.wsPublisher, "created", "story_template", tmpl.ID, req.WorkspaceID, "")
	return tmpl, nil
}

// Update updates a story template.
func (s *PMTaskTemplateService) Update(ctx context.Context, id string, req model.UpdateStoryTemplateRequest) (*model.PMStoryTemplate, error) {
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
	if req.TaskType != nil {
		tmpl.TaskType = req.TaskType
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
	publishWorkspaceEvent(s.wsPublisher, "updated", "story_template", tmpl.ID, tmpl.WorkspaceID, "")
	return tmpl, nil
}

// Delete deletes a story template.
func (s *PMTaskTemplateService) Delete(ctx context.Context, id string) error {
	tmpl, err := s.templateRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if tmpl == nil {
		return fmt.Errorf("story template not found")
	}
	if err := s.templateRepo.Delete(ctx, id); err != nil {
		return err
	}
	publishWorkspaceEvent(s.wsPublisher, "deleted", "story_template", id, tmpl.WorkspaceID, "")
	return nil
}
