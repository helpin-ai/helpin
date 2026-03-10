package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func eventTypesForCategory(category string) []string {
	eventTypes := make([]string, 0)
	for eventType, mappedCategory := range model.EventTypeToCategory {
		if mappedCategory == category {
			eventTypes = append(eventTypes, eventType)
		}
	}
	return eventTypes
}

// NotificationRepository handles notification CRUD operations.
type NotificationRepository struct {
	db *gorm.DB
}

// PendingDigestDelivery is a pending digest delivery joined with the current notification state.
type PendingDigestDelivery struct {
	DeliveryID         string
	RecipientID        string
	NotificationID     string
	WorkspaceID        string
	NotificationStatus string
	SnoozedUntil       *time.Time
	EventType          string
	EventTitle         string
	CreatedAt          time.Time
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
		mentionTypes := eventTypesForCategory(model.NotifCategoryMentions)
		if len(mentionTypes) == 0 {
			q = q.Where("1 = 0")
		} else {
			q = q.Where("event_type IN ?", mentionTypes)
		}
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
func (r *NotificationRepository) UnreadCount(ctx context.Context, recipientID, workspaceID, badgeMode string) (int, error) {
	if badgeMode == "none" {
		return 0, nil
	}

	var count int64
	q := r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("recipient_id = ? AND workspace_id = ? AND status = 'unread'", recipientID, workspaceID).
		Where("(snoozed_until IS NULL OR snoozed_until <= ?)", time.Now())

	if badgeMode == "mentions_only" {
		mentionTypes := eventTypesForCategory(model.NotifCategoryMentions)
		if len(mentionTypes) == 0 {
			return 0, nil
		}
		q = q.Where("event_type IN ?", mentionTypes)
	}

	err := q.Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("unread count: %w", err)
	}
	return int(count), nil
}

// ListPendingDigestDeliveries returns pending digest deliveries joined with the current notification state.
func (r *NotificationRepository) ListPendingDigestDeliveries(ctx context.Context) ([]PendingDigestDelivery, error) {
	var deliveries []PendingDigestDelivery
	err := r.db.WithContext(ctx).
		Table("notification_deliveries nd").
		Select(`
			nd.id AS delivery_id,
			n.recipient_id,
			n.id AS notification_id,
			n.workspace_id,
			n.status AS notification_status,
			n.snoozed_until,
			ne.event_type,
			ne.title AS event_title,
			nd.created_at
		`).
		Joins("JOIN notification_events ne ON ne.id = nd.notification_event_id").
		Joins("JOIN notifications n ON n.id = ne.notification_id").
		Where("nd.channel = ? AND nd.status = ?", "digest", "pending").
		Order("nd.created_at ASC").
		Scan(&deliveries).Error
	if err != nil {
		return nil, fmt.Errorf("list pending digest deliveries: %w", err)
	}
	return deliveries, nil
}

// UpdateDeliveryStatus updates a batch of delivery rows to the same status.
func (r *NotificationRepository) UpdateDeliveryStatus(ctx context.Context, deliveryIDs []string, status string, deliveredAt *time.Time, errorMessage *string) error {
	if len(deliveryIDs) == 0 {
		return nil
	}

	updates := map[string]any{
		"status": status,
		"error":  errorMessage,
	}
	if deliveredAt != nil {
		updates["delivered_at"] = *deliveredAt
	} else {
		updates["delivered_at"] = nil
	}

	if err := r.db.WithContext(ctx).
		Model(&model.NotificationDelivery{}).
		Where("id IN ?", deliveryIDs).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("update delivery status: %w", err)
	}
	return nil
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

// DeleteArchivedOlderThan removes archived notifications older than the given cutoff.
// Returns the number of rows deleted.
func (r *NotificationRepository) DeleteArchivedOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tx := r.db.WithContext(ctx).
		Where("status = 'archived' AND updated_at < ?", cutoff).
		Delete(&model.Notification{})
	if tx.Error != nil {
		return 0, fmt.Errorf("delete archived notifications: %w", tx.Error)
	}
	return tx.RowsAffected, nil
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
