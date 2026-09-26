package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// WithRecipientEmailLock serializes email decisions and delivery records across API/worker replicas.
func (r *NotificationRepository) WithRecipientEmailLock(ctx context.Context, recipientID string, fn func(*NotificationRepository) error) error {
	if r.db.Dialector.Name() != "postgres" {
		return fn(r)
	}
	busy := errors.New("notification email lock busy")
	for {
		err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var acquired bool
			if err := tx.Raw("SELECT pg_try_advisory_xact_lock(hashtextextended(?, 0))", "notification-email:"+recipientID).Scan(&acquired).Error; err != nil {
				return err
			}
			if !acquired {
				return busy
			}
			return fn(NewNotificationRepository(tx))
		})
		if !errors.Is(err, busy) {
			return err
		}
		// Release the connection while waiting; the sender may need other repositories.
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

}

// CanSendIndividualEmail must be called under WithRecipientEmailLock.
func (r *NotificationRepository) CanSendIndividualEmail(ctx context.Context, recipientID, workspaceID, entityType, entityID string, now time.Time) (bool, error) {
	base := func() *gorm.DB {
		return r.db.WithContext(ctx).Table("notification_deliveries nd").Joins("JOIN notification_events ne ON ne.id = nd.notification_event_id").Joins("JOIN notifications n ON n.id = ne.notification_id").Where("n.recipient_id = ? AND nd.channel IN ? AND nd.status = ? AND nd.delivered_at > ?", recipientID, []string{"email", "support_reply_email"}, "delivered", now.Add(-time.Hour))
	}
	var count int64
	if err := base().Count(&count).Error; err != nil {
		return false, err
	}
	if count >= 5 {
		return false, nil
	}
	if err := base().Where("n.workspace_id = ? AND n.entity_type = ? AND n.entity_id = ? AND nd.delivered_at > ?", workspaceID, entityType, entityID, now.Add(-15*time.Minute)).Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}

func (r *NotificationRepository) MoveDeliveryToOverflow(ctx context.Context, deliveryID string) error {
	return r.db.WithContext(ctx).Table("notification_deliveries").Where("id = ? AND status = ?", deliveryID, "pending").Updates(map[string]any{"channel": "email_overflow", "error": "grouped to limit notification email volume"}).Error
}

func (r *NotificationRepository) HasDigestSince(ctx context.Context, recipientID string, cutoff time.Time) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("notification_deliveries nd").Joins("JOIN notification_events ne ON ne.id = nd.notification_event_id").Joins("JOIN notifications n ON n.id = ne.notification_id").Where("n.recipient_id = ? AND nd.channel IN ? AND nd.status = ? AND nd.delivered_at >= ?", recipientID, []string{"digest", "email_overflow"}, "delivered", cutoff).Count(&count).Error
	return count > 0, err
}
