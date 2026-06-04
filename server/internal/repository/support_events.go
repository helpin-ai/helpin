package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// SupportEventRepository provides data access for the shared
// support_events append-only ledger.
type SupportEventRepository struct {
	db *gorm.DB
}

// NewSupportEventRepository creates a new SupportEventRepository.
func NewSupportEventRepository(db *gorm.DB) *SupportEventRepository {
	return &SupportEventRepository{db: db}
}

// Create inserts a new support event. Append-only — no updates.
func (r *SupportEventRepository) Create(ctx context.Context, event *model.SupportEvent) error {
	if event.WorkspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if event.EventType == "" {
		return fmt.Errorf("event_type is required")
	}
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Metadata == nil {
		event.Metadata = []byte("{}")
	}
	if err := r.db.WithContext(ctx).Create(event).Error; err != nil {
		return fmt.Errorf("create support event: %w", err)
	}
	return nil
}

// List returns events for a workspace with optional filters. Scoped
// to workspace_id always. This is for internal/diagnostic use — the
// Coverage UI reads from gap/evidence/snapshot tables, not events.
func (r *SupportEventRepository) List(ctx context.Context, workspaceID string, filter model.SupportEventFilter) ([]model.SupportEvent, error) {
	q := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("occurred_at DESC")

	if filter.EventType != "" {
		q = q.Where("event_type = ?", filter.EventType)
	}
	if filter.ConversationID != "" {
		q = q.Where("conversation_id = ?", filter.ConversationID)
	}

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q = q.Limit(limit)

	var events []model.SupportEvent
	if err := q.Find(&events).Error; err != nil {
		return nil, fmt.Errorf("list support events: %w", err)
	}
	return events, nil
}
