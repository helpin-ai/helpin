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

// PMLabelService contains label business logic.
type PMLabelService struct {
	labelRepo   *repository.PMLabelRepository
	wsPublisher *websocket.Publisher
	logger      *slog.Logger
}

// NewPMLabelService creates a new PMLabelService.
func NewPMLabelService(labelRepo *repository.PMLabelRepository, wsPublisher *websocket.Publisher) *PMLabelService {
	return &PMLabelService{labelRepo: labelRepo, wsPublisher: wsPublisher, logger: slog.Default().With("service", "pm_label")}
}

// ListByWorkspace lists labels by workspace.
func (s *PMLabelService) ListByWorkspace(ctx context.Context, workspaceID string, teamID *string, includeShared bool) ([]model.PMLabel, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.labelRepo.ListByWorkspace(ctx, workspaceID, repository.PMLabelListOptions{
		TeamID:        teamID,
		IncludeShared: includeShared,
	})
}

// ListWithStats returns labels with task/epic completion stats.
func (s *PMLabelService) ListWithStats(ctx context.Context, workspaceID string, teamID *string, includeShared bool, archived *bool) ([]model.LabelWithStats, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.labelRepo.ListWithStats(ctx, workspaceID, repository.PMLabelListOptions{
		TeamID:        teamID,
		IncludeShared: includeShared,
		Archived:      archived,
	})
}

// Create creates a label after uniqueness validation.
func (s *PMLabelService) Create(ctx context.Context, req model.CreateLabelRequest) (*model.PMLabel, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}
	name := strings.TrimSpace(req.Name)
	teamID := normalizeOptionalID(req.TeamID)
	existing, err := s.labelRepo.GetByName(ctx, req.WorkspaceID, teamID, name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("label name already exists in this scope")
	}

	label := &model.PMLabel{
		WorkspaceID: req.WorkspaceID,
		TeamID:      teamID,
		Name:        name,
		Description: req.Description,
		Color:       req.Color,
	}
	if err := s.labelRepo.Create(ctx, label); err != nil {
		s.logger.ErrorContext(ctx, "failed to create label", "error", err, "workspace_id", req.WorkspaceID)
		return nil, err
	}
	s.logger.InfoContext(ctx, "label created", "label_id", label.ID, "workspace_id", req.WorkspaceID, "name", label.Name)
	publishWorkspaceEvent(s.wsPublisher, "created", "label", label.ID, req.WorkspaceID, "")
	return label, nil
}

// Update updates a label.
func (s *PMLabelService) Update(ctx context.Context, id string, req model.UpdateLabelRequest) (*model.PMLabel, error) {
	label, err := s.labelRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if label == nil {
		return nil, fmt.Errorf("label not found")
	}

	if req.TeamID != nil {
		label.TeamID = normalizeOptionalID(req.TeamID)
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		label.Name = name
	}
	if req.TeamID != nil || req.Name != nil {
		existing, err := s.labelRepo.GetByName(ctx, label.WorkspaceID, label.TeamID, label.Name)
		if err != nil {
			return nil, err
		}
		if existing != nil && existing.ID != label.ID {
			return nil, fmt.Errorf("label name already exists in this scope")
		}
	}
	if req.Description != nil {
		label.Description = req.Description
	}
	if req.Color != nil {
		label.Color = req.Color
	}
	if req.Archived != nil {
		label.Archived = *req.Archived
	}

	if err := s.labelRepo.Update(ctx, label); err != nil {
		s.logger.ErrorContext(ctx, "failed to update label", "error", err, "label_id", id)
		return nil, err
	}
	s.logger.InfoContext(ctx, "label updated", "label_id", id)
	publishWorkspaceEvent(s.wsPublisher, "updated", "label", id, label.WorkspaceID, "")
	return label, nil
}

// Delete deletes a label.
func (s *PMLabelService) Delete(ctx context.Context, id string) error {
	label, err := s.labelRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if label == nil {
		return fmt.Errorf("label not found")
	}
	if err := s.labelRepo.Delete(ctx, id); err != nil {
		s.logger.ErrorContext(ctx, "failed to delete label", "error", err, "label_id", id)
		return err
	}
	s.logger.InfoContext(ctx, "label deleted", "label_id", id)
	publishWorkspaceEvent(s.wsPublisher, "deleted", "label", id, label.WorkspaceID, "")
	return nil
}

// SeedDefaults creates default labels if missing.
func (s *PMLabelService) SeedDefaults(ctx context.Context, workspaceID string) error {
	defaults := []struct {
		Name  string
		Color string
	}{
		{Name: "frontend", Color: "#3b82f6"},
		{Name: "backend", Color: "#16a34a"},
		{Name: "design", Color: "#ec4899"},
		{Name: "infrastructure", Color: "#64748b"},
	}

	for _, def := range defaults {
		existing, err := s.labelRepo.GetByName(ctx, workspaceID, nil, def.Name)
		if err != nil {
			return err
		}
		if existing != nil {
			continue
		}
		color := def.Color
		label := &model.PMLabel{
			WorkspaceID: workspaceID,
			Name:        def.Name,
			Color:       &color,
		}
		if err := s.labelRepo.Create(ctx, label); err != nil {
			return err
		}
	}
	return nil
}
