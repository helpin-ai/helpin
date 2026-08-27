package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// EventProjectRepository resolves internal workspaces to canonical and legacy projects.
type EventProjectRepository struct {
	db *gorm.DB
}

// NewEventProjectRepository creates an EventProjectRepository.
func NewEventProjectRepository(db *gorm.DB) *EventProjectRepository {
	return &EventProjectRepository{db: db}
}

// ResolveProjectSet returns the canonical workspace UUID plus verified aliases.
func (r *EventProjectRepository) ResolveProjectSet(ctx context.Context, workspaceID string) ([]string, error) {
	canonical := strings.ToLower(strings.TrimSpace(workspaceID))
	if canonical == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	var aliases []model.WorkspaceEventProjectAlias
	if err := r.db.WithContext(ctx).
		Select("project_id").
		Where("workspace_id = ?", canonical).
		Order("created_at ASC").
		Find(&aliases).Error; err != nil {
		return nil, fmt.Errorf("resolve event project aliases: %w", err)
	}

	projects := make([]string, 0, len(aliases)+1)
	projects = append(projects, canonical)
	seen := map[string]struct{}{canonical: {}}
	for _, alias := range aliases {
		projectID := strings.TrimSpace(alias.ProjectID)
		if projectID == "" {
			continue
		}
		if _, ok := seen[projectID]; ok {
			continue
		}
		seen[projectID] = struct{}{}
		projects = append(projects, projectID)
	}
	return projects, nil
}

// ListActiveEventWorkspaceIDs returns a bounded set of recently configured event tenants.
func (r *EventProjectRepository) ListActiveEventWorkspaceIDs(ctx context.Context, limit int) ([]string, error) {
	if limit < 1 || limit > 1000 {
		limit = 500
	}
	var workspaceIDs []string
	if err := r.db.WithContext(ctx).Model(&model.SupportWidgetInstallation{}).
		Distinct("workspace_id").Where("active = ?", true).
		Order("workspace_id ASC").Limit(limit).Pluck("workspace_id", &workspaceIDs).Error; err != nil {
		return nil, fmt.Errorf("list active event workspaces: %w", err)
	}
	return workspaceIDs, nil
}
