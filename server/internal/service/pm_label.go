package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/repository"
)

// PMLabelService contains label business logic.
type PMLabelService struct {
	labelRepo *repository.PMLabelRepository
}

// NewPMLabelService creates a new PMLabelService.
func NewPMLabelService(labelRepo *repository.PMLabelRepository) *PMLabelService {
	return &PMLabelService{labelRepo: labelRepo}
}

// ListByWorkspace lists labels by workspace.
func (s *PMLabelService) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.PMLabel, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.labelRepo.ListByWorkspace(ctx, workspaceID)
}

// ListWithStats returns labels with story/epic completion stats.
func (s *PMLabelService) ListWithStats(ctx context.Context, workspaceID string, archived *bool) ([]model.LabelWithStats, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.labelRepo.ListWithStats(ctx, workspaceID, archived)
}

// Create creates a label after uniqueness validation.
func (s *PMLabelService) Create(ctx context.Context, req model.CreateLabelRequest) (*model.PMLabel, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}
	name := strings.TrimSpace(req.Name)
	existing, err := s.labelRepo.GetByName(ctx, req.WorkspaceID, name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("label name already exists in workspace")
	}

	label := &model.PMLabel{
		WorkspaceID: req.WorkspaceID,
		Name:        name,
		Description: req.Description,
		Color:       req.Color,
	}
	if err := s.labelRepo.Create(ctx, label); err != nil {
		return nil, err
	}
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

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		if !strings.EqualFold(name, label.Name) {
			existing, err := s.labelRepo.GetByName(ctx, label.WorkspaceID, name)
			if err != nil {
				return nil, err
			}
			if existing != nil && existing.ID != label.ID {
				return nil, fmt.Errorf("label name already exists in workspace")
			}
		}
		label.Name = name
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
		return nil, err
	}
	return label, nil
}

// Delete deletes a label.
func (s *PMLabelService) Delete(ctx context.Context, id string) error {
	return s.labelRepo.Delete(ctx, id)
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
		existing, err := s.labelRepo.GetByName(ctx, workspaceID, def.Name)
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
