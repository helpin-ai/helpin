package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// PMTaskTemplateService contains story template business logic.
type PMTaskTemplateService struct {
	templateRepo   *repository.PMTaskTemplateRepository
	attachmentRepo *repository.PMAttachmentRepository
	wsPublisher    *websocket.Publisher
	logger         *slog.Logger
}

// NewPMTaskTemplateService creates a new PMTaskTemplateService.
func NewPMTaskTemplateService(templateRepo *repository.PMTaskTemplateRepository, wsPublisher *websocket.Publisher) *PMTaskTemplateService {
	return &PMTaskTemplateService{templateRepo: templateRepo, wsPublisher: wsPublisher, logger: slog.Default().With("service", "pm_task_template")}
}

// SetAttachmentRepository sets the attachment repository used by template inline uploads.
func (s *PMTaskTemplateService) SetAttachmentRepository(repo *repository.PMAttachmentRepository) {
	s.attachmentRepo = repo
}

// ListByWorkspace lists story templates by workspace.
func (s *PMTaskTemplateService) ListByWorkspace(ctx context.Context, workspaceID string, teamID *string, includeShared bool, archived *bool) ([]model.PMTaskTemplate, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	templates, err := s.templateRepo.ListByWorkspace(ctx, workspaceID, repository.PMTaskTemplateListOptions{
		TeamID:        teamID,
		IncludeShared: includeShared,
		Archived:      archived,
	})
	if err != nil {
		return nil, err
	}
	return filterVisibleTaskTemplates(ctx, templates), nil
}

// GetByID returns a story template by ID.
func (s *PMTaskTemplateService) GetByID(ctx context.Context, id string) (*model.PMTaskTemplate, error) {
	tmpl, err := s.templateRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tmpl == nil {
		return nil, fmt.Errorf("story template not found")
	}
	if !canViewTaskTemplate(ctx, tmpl.TeamID) {
		return nil, &model.ErrForbidden{Message: "you do not have access to this template"}
	}
	return tmpl, nil
}

// Create creates a story template after uniqueness validation.
func (s *PMTaskTemplateService) Create(ctx context.Context, req model.CreateTaskTemplateRequest) (*model.PMTaskTemplate, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}
	name := strings.TrimSpace(req.Name)
	teamID := normalizeOptionalID(req.TeamID)
	if err := requireCanManage(ctx, teamID); err != nil {
		return nil, err
	}
	existing, err := s.templateRepo.GetByName(ctx, req.WorkspaceID, teamID, name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("template name already exists in this scope")
	}

	tmpl := &model.PMTaskTemplate{
		WorkspaceID:     req.WorkspaceID,
		TeamID:          teamID,
		Name:            name,
		Description:     req.Description,
		TaskType:        req.TaskType,
		Priority:        req.Priority,
		Severity:        req.Severity,
		Estimate:        req.Estimate,
		LabelIDs:        req.LabelIDs,
		OwnerMemberID:   normalizeOptionalID(req.OwnerMemberID),
		OwnerMemberIDs:  req.OwnerMemberIDs,
		EpicID:          normalizeOptionalID(req.EpicID),
		SprintID:        normalizeOptionalID(req.SprintID),
		WorkflowStateID: normalizeOptionalID(req.WorkflowStateID),
		Deadline:        req.Deadline,
		ChecklistItems:  req.ChecklistItems,
		ExternalLinks:   req.ExternalLinks,
	}
	if err := s.templateRepo.Create(ctx, tmpl); err != nil {
		return nil, err
	}
	if len(req.AttachmentIDs) > 0 && s.attachmentRepo != nil {
		if err := s.attachmentRepo.ReassignToEntity(ctx, req.AttachmentIDs, "task_template", tmpl.ID); err != nil {
			s.logger.ErrorContext(ctx, "failed to reassign attachments to task template", "error", err, "template_id", tmpl.ID, "attachment_ids", req.AttachmentIDs)
		}
	}
	publishWorkspaceEvent(s.wsPublisher, "created", "story_template", tmpl.ID, req.WorkspaceID, "")
	return tmpl, nil
}

// Update updates a story template.
func (s *PMTaskTemplateService) Update(ctx context.Context, id string, req model.UpdateTaskTemplateRequest) (*model.PMTaskTemplate, error) {
	tmpl, err := s.templateRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tmpl == nil {
		return nil, fmt.Errorf("story template not found")
	}
	if err := requireCanManage(ctx, tmpl.TeamID); err != nil {
		return nil, err
	}
	if req.TeamID != nil {
		if err := requireCanManage(ctx, normalizeOptionalID(req.TeamID)); err != nil {
			return nil, err
		}
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
	if req.OwnerMemberIDs != nil {
		tmpl.OwnerMemberIDs = req.OwnerMemberIDs
	}
	if req.EpicID != nil {
		tmpl.EpicID = normalizeOptionalID(req.EpicID)
	}
	if req.SprintID != nil {
		tmpl.SprintID = normalizeOptionalID(req.SprintID)
	}
	if req.WorkflowStateID != nil {
		tmpl.WorkflowStateID = normalizeOptionalID(req.WorkflowStateID)
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
	if len(req.AttachmentIDs) > 0 && s.attachmentRepo != nil {
		if err := s.attachmentRepo.ReassignToEntity(ctx, req.AttachmentIDs, "task_template", tmpl.ID); err != nil {
			s.logger.ErrorContext(ctx, "failed to reassign attachments to task template", "error", err, "template_id", tmpl.ID, "attachment_ids", req.AttachmentIDs)
		}
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
	if err := requireCanManage(ctx, tmpl.TeamID); err != nil {
		return err
	}
	if err := s.templateRepo.Delete(ctx, id); err != nil {
		return err
	}
	publishWorkspaceEvent(s.wsPublisher, "deleted", "story_template", id, tmpl.WorkspaceID, "")
	return nil
}

func filterVisibleTaskTemplates(ctx context.Context, templates []model.PMTaskTemplate) []model.PMTaskTemplate {
	filtered := templates[:0]
	for _, tmpl := range templates {
		if canViewTaskTemplate(ctx, tmpl.TeamID) {
			filtered = append(filtered, tmpl)
		}
	}
	return filtered
}

func canViewTaskTemplate(ctx context.Context, teamID *string) bool {
	if teamID == nil || strings.TrimSpace(*teamID) == "" {
		return true
	}
	return canAccessTeam(ctx, teamID)
}
