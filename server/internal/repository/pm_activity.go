package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PMActivityRepository handles DB operations for activity log entries.
type PMActivityRepository struct {
	db                  *gorm.DB
	eventTypeColumnOnce sync.Once
	hasEventTypeColumn  bool
}

// NewPMActivityRepository creates a new PMActivityRepository.
func NewPMActivityRepository(db *gorm.DB) *PMActivityRepository {
	return &PMActivityRepository{db: db}
}

// List returns activity for a specific entity.
func (r *PMActivityRepository) List(ctx context.Context, entityType, entityID string, pagination model.PMPagination) ([]model.ActivityLogEntry, int64, error) {
	query := r.db.WithContext(ctx).
		Model(&model.PMActivityLog{}).
		Where("entity_type = ? AND entity_id = ?", entityType, entityID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count entity activity: %w", err)
	}

	page := pagination.Page
	perPage := pagination.PerPage
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 50
	}

	var rows []model.PMActivityLog
	if err := query.Order("created_at DESC").Offset((page - 1) * perPage).Limit(perPage).Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list entity activity: %w", err)
	}

	entries, err := r.enrichActors(ctx, rows)
	if err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

// ListByWorkspace returns workspace-wide activity feed.
func (r *PMActivityRepository) ListByWorkspace(ctx context.Context, workspaceID string, filters model.PMActivityFilters, pagination model.PMPagination) ([]model.ActivityLogEntry, int64, error) {
	query := r.db.WithContext(ctx).
		Model(&model.PMActivityLog{}).
		Where("workspace_id = ?", workspaceID)

	if filters.ActorID != nil && *filters.ActorID != "" {
		query = query.Where("actor_id = ?", *filters.ActorID)
	}
	if filters.EntityType != nil && *filters.EntityType != "" {
		query = query.Where("entity_type = ?", *filters.EntityType)
	}
	if filters.Action != nil && *filters.Action != "" {
		query = query.Where("action = ?", *filters.Action)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count workspace activity: %w", err)
	}

	page := pagination.Page
	perPage := pagination.PerPage
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 50
	}

	var rows []model.PMActivityLog
	if err := query.Order("created_at DESC").Offset((page - 1) * perPage).Limit(perPage).Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list workspace activity: %w", err)
	}

	entries, err := r.enrichActors(ctx, rows)
	if err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

// Create inserts an activity log entry.
func (r *PMActivityRepository) Create(ctx context.Context, entry *model.PMActivityLog) error {
	db := r.db.WithContext(ctx)
	r.eventTypeColumnOnce.Do(func() {
		r.hasEventTypeColumn = r.db.Migrator().HasColumn(&model.PMActivityLog{}, "event_type")
	})
	if !r.hasEventTypeColumn {
		if entry.EventType != nil {
			metadata := map[string]interface{}{}
			if len(entry.Metadata) > 0 {
				_ = json.Unmarshal(entry.Metadata, &metadata)
			}
			metadata["event_type"] = *entry.EventType
			if encoded, err := json.Marshal(metadata); err == nil {
				entry.Metadata = encoded
			}
		}
		db = db.Omit("event_type")
	}
	if err := db.Create(entry).Error; err != nil {
		return fmt.Errorf("create activity entry: %w", err)
	}
	return nil
}

func (r *PMActivityRepository) enrichActors(ctx context.Context, rows []model.PMActivityLog) ([]model.ActivityLogEntry, error) {
	entries := make([]model.ActivityLogEntry, 0, len(rows))
	for _, row := range rows {
		var actor *model.User
		if row.ActorID != nil {
			var user model.User
			if err := r.db.WithContext(ctx).Where("id = ?", *row.ActorID).First(&user).Error; err == nil {
				actor = &user
			}
		}
		entries = append(entries, model.ActivityLogEntry{Activity: row, Actor: actor})
	}
	return entries, nil
}
