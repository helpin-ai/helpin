package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// NotificationRepository handles notification CRUD operations.
type NotificationRepository struct {
	db *gorm.DB
}

// NewNotificationRepository creates a new notification repository.
func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

// Upsert creates or updates a notification for a recipient+entity pair.
func (r *NotificationRepository) Upsert(ctx context.Context, notif *model.Notification) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "recipient_id"},
				{Name: "entity_type"},
				{Name: "entity_id"},
				{Name: "workspace_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"actor_id", "event_type", "title", "body", "metadata",
				"latest_event_category", "actor_snapshot", "entity_snapshot",
				"parent_entity_snapshot", "event_count", "last_event_at",
				"status", "priority", "read_at", "snoozed_until", "archived_at",
				"updated_at",
			}),
		}).
		Create(notif).Error
}

// GetByID fetches a notification by ID.
func (r *NotificationRepository) GetByID(ctx context.Context, id string) (*model.Notification, error) {
	var notif model.Notification
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&notif).Error; err != nil {
		return nil, fmt.Errorf("get notification: %w", err)
	}
	return &notif, nil
}

// GetExisting fetches an existing notification for a recipient+entity pair.
func (r *NotificationRepository) GetExisting(ctx context.Context, recipientID, entityType, entityID, workspaceID string) (*model.Notification, error) {
	var notif model.Notification
	err := r.db.WithContext(ctx).
		Where("recipient_id = ? AND entity_type = ? AND entity_id = ? AND workspace_id = ?",
			recipientID, entityType, entityID, workspaceID).
		First(&notif).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get existing notification: %w", err)
	}
	return &notif, nil
}

// List fetches paginated notifications for a user in a workspace.
func (r *NotificationRepository) List(ctx context.Context, recipientID, workspaceID string, status string, filter string, limit int, cursor *time.Time) ([]model.Notification, error) {
	q := r.db.WithContext(ctx).
		Where("recipient_id = ? AND workspace_id = ?", recipientID, workspaceID)

	if status != "" {
		q = q.Where("status = ?", status)
	} else {
		// Default: show unread and read, not archived
		q = q.Where("status IN ('unread', 'read')")
	}

	// Tab filters
	switch filter {
	case "mentions":
		q = q.Where("event_type LIKE '%.mentioned'")
	case "assigned":
		q = q.Where("event_type LIKE '%.assigned'")
	}

	// Handle snoozed: hide snoozed unless they've expired
	q = q.Where("(snoozed_until IS NULL OR snoozed_until <= ?)", time.Now())

	if cursor != nil {
		q = q.Where("last_event_at < ?", *cursor)
	}

	if limit <= 0 || limit > 50 {
		limit = 20
	}

	var notifs []model.Notification
	if err := q.Order("last_event_at DESC").Limit(limit).Find(&notifs).Error; err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	return notifs, nil
}

// UnreadCount returns the number of unread notifications for a user.
func (r *NotificationRepository) UnreadCount(ctx context.Context, recipientID, workspaceID string) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("recipient_id = ? AND workspace_id = ? AND status = 'unread'", recipientID, workspaceID).
		Where("(snoozed_until IS NULL OR snoozed_until <= ?)", time.Now()).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("unread count: %w", err)
	}
	return int(count), nil
}

// MarkAsRead marks a single notification as read.
func (r *NotificationRepository) MarkAsRead(ctx context.Context, id, recipientID string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("id = ? AND recipient_id = ?", id, recipientID).
		Updates(map[string]interface{}{
			"status":  "read",
			"read_at": now,
		}).Error
}

// MarkAllAsRead marks all unread notifications as read for a user.
func (r *NotificationRepository) MarkAllAsRead(ctx context.Context, recipientID, workspaceID string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("recipient_id = ? AND workspace_id = ? AND status = 'unread'", recipientID, workspaceID).
		Updates(map[string]interface{}{
			"status":  "read",
			"read_at": now,
		}).Error
}

// ArchiveAllRead archives all read notifications for a user.
func (r *NotificationRepository) ArchiveAllRead(ctx context.Context, recipientID, workspaceID string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("recipient_id = ? AND workspace_id = ? AND status = 'read'", recipientID, workspaceID).
		Updates(map[string]interface{}{
			"status":      "archived",
			"archived_at": now,
		}).Error
}

// Update updates a notification's status/snooze fields.
func (r *NotificationRepository) Update(ctx context.Context, id, recipientID string, updates map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("id = ? AND recipient_id = ?", id, recipientID).
		Updates(updates).Error
}

// Delete deletes a notification.
func (r *NotificationRepository) Delete(ctx context.Context, id, recipientID string) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND recipient_id = ?", id, recipientID).
		Delete(&model.Notification{}).Error
}

// CreateEvent creates a notification event.
func (r *NotificationRepository) CreateEvent(ctx context.Context, event *model.NotificationEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

// CreateDelivery creates a notification delivery record.
func (r *NotificationRepository) CreateDelivery(ctx context.Context, delivery *model.NotificationDelivery) error {
	return r.db.WithContext(ctx).Create(delivery).Error
}

// WakeExpiredSnoozes finds and wakes snoozed notifications whose snooze has expired.
func (r *NotificationRepository) WakeExpiredSnoozes(ctx context.Context) ([]model.Notification, error) {
	var notifs []model.Notification
	now := time.Now()

	err := r.db.WithContext(ctx).
		Where("snoozed_until IS NOT NULL AND snoozed_until <= ? AND status = 'snoozed'", now).
		Find(&notifs).Error
	if err != nil {
		return nil, fmt.Errorf("wake expired snoozes: %w", err)
	}

	if len(notifs) > 0 {
		r.db.WithContext(ctx).Model(&model.Notification{}).
			Where("snoozed_until IS NOT NULL AND snoozed_until <= ? AND status = 'snoozed'", now).
			Updates(map[string]interface{}{
				"status":        "unread",
				"snoozed_until": nil,
			})
	}

	return notifs, nil
}
