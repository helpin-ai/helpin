package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// AutomationHealthRepository stores health observations for built-in automations.
type AutomationHealthRepository struct {
	db *gorm.DB
}

func NewAutomationHealthRepository(db *gorm.DB) *AutomationHealthRepository {
	return &AutomationHealthRepository{db: db}
}

func (r *AutomationHealthRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.AutomationHealthSnapshot, error) {
	var snapshots []model.AutomationHealthSnapshot
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("catalog_id ASC, scope_type ASC, scope_id ASC").
		Find(&snapshots).Error; err != nil {
		return nil, fmt.Errorf("list automation health snapshots: %w", err)
	}
	return snapshots, nil
}

func (r *AutomationHealthRepository) UpsertSnapshot(ctx context.Context, snapshot *model.AutomationHealthSnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("snapshot is required")
	}

	if snapshot.Metrics == nil {
		snapshot.Metrics = model.JSONB{}
	}

	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "workspace_id"},
			{Name: "catalog_id"},
			{Name: "scope_type"},
			{Name: "scope_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"status",
			"last_seen_at",
			"last_success_at",
			"last_error_at",
			"last_error_message",
			"metrics",
			"updated_at",
		}),
	}).Create(snapshot).Error; err != nil {
		return fmt.Errorf("upsert automation health snapshot: %w", err)
	}
	return nil
}

func (r *AutomationHealthRepository) ObserveSuccess(ctx context.Context, workspaceID, catalogID, scopeType, scopeID string, metrics model.JSONB) error {
	now := time.Now().UTC()
	snapshot := &model.AutomationHealthSnapshot{
		WorkspaceID:   workspaceID,
		CatalogID:     catalogID,
		ScopeType:     scopeType,
		ScopeID:       scopeID,
		Status:        model.AutomationHealthHealthy,
		LastSeenAt:    &now,
		LastSuccessAt: &now,
		Metrics:       metrics,
		UpdatedAt:     now,
	}
	return r.UpsertSnapshot(ctx, snapshot)
}

func (r *AutomationHealthRepository) ObserveFailure(ctx context.Context, workspaceID, catalogID, scopeType, scopeID, message string, metrics model.JSONB) error {
	now := time.Now().UTC()
	msg := message
	snapshot := &model.AutomationHealthSnapshot{
		WorkspaceID:      workspaceID,
		CatalogID:        catalogID,
		ScopeType:        scopeType,
		ScopeID:          scopeID,
		Status:           model.AutomationHealthError,
		LastSeenAt:       &now,
		LastErrorAt:      &now,
		LastErrorMessage: &msg,
		Metrics:          metrics,
		UpdatedAt:        now,
	}
	return r.UpsertSnapshot(ctx, snapshot)
}
