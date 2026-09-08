package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

var (
	// ErrAutomationEventConflict rejects changed intent under an existing event key.
	ErrAutomationEventConflict = errors.New("scheduled event identity conflicts")
	// ErrAutomationEventLeaseLost rejects cancelled, expired or replaced delivery claims.
	ErrAutomationEventLeaseLost = errors.New("scheduled event lease is no longer active")
)

// AutomationScheduledEventRepository owns durable product-event delivery state.
type AutomationScheduledEventRepository struct{ db *gorm.DB }

// NewAutomationScheduledEventRepository binds the shared Automation outbox.
func NewAutomationScheduledEventRepository(db *gorm.DB) *AutomationScheduledEventRepository {
	return &AutomationScheduledEventRepository{db: db}
}

// Enqueue inserts an immutable event, preserving the original receipt on retry.
// Call with a transaction-bound repository to commit alongside the originating change.
func (r *AutomationScheduledEventRepository) Enqueue(ctx context.Context, event model.AutomationScheduledEvent) error {
	if event.WorkspaceID == "" || event.EventKey == "" || len(event.EventKey) > 200 || event.Kind == "" ||
		event.TargetType == "" || event.TargetID == "" || event.ExpectedRevision < 1 || event.DueAt.IsZero() {
		return fmt.Errorf("invalid scheduled event")
	}
	event.ID, event.Status = uuid.NewString(), model.AutomationEventScheduled
	event.DueAt = event.DueAt.UTC().Truncate(time.Microsecond)
	event.AvailableAt = event.DueAt
	event.Attempts, event.MaxAttempts = 0, 5
	event.LeaseToken, event.LeaseUntil, event.CompletedAt = nil, nil, nil
	event.ResultCode = ""
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "workspace_id"}, {Name: "event_key"}}, DoNothing: true,
	}).Create(&event)
	if result.Error != nil {
		return fmt.Errorf("enqueue scheduled event: %w", result.Error)
	}
	if result.RowsAffected == 1 {
		return nil
	}
	var stored model.AutomationScheduledEvent
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND event_key = ?", event.WorkspaceID, event.EventKey).Take(&stored).Error; err != nil {
		return err
	}
	if stored.Kind != event.Kind || stored.TargetType != event.TargetType || stored.TargetID != event.TargetID ||
		stored.ExpectedRevision != event.ExpectedRevision || !stored.DueAt.Equal(event.DueAt) {
		return ErrAutomationEventConflict
	}
	return nil
}

// CancelTarget revokes pending delivery, including claimed events, under the
// caller's domain lock. It does not claim to cancel an already executed action.
func (r *AutomationScheduledEventRepository) CancelTarget(ctx context.Context, ws, kind, targetType, id string, now time.Time) error {
	return r.db.WithContext(ctx).Model(&model.AutomationScheduledEvent{}).
		Where("workspace_id = ? AND kind = ? AND target_type = ? AND target_id = ?", ws, kind, targetType, id).
		Where("status IN ?", []string{model.AutomationEventScheduled, model.AutomationEventProcessing}).
		Updates(map[string]any{"status": model.AutomationEventCancelled, "result_code": "superseded",
			"lease_token": nil, "lease_until": nil, "completed_at": now, "updated_at": now}).Error
}

// ClaimNext leases one due event. PostgreSQL workers skip each other's locks;
// expired leases preserve the same event identity and consume a bounded attempt.
func (r *AutomationScheduledEventRepository) ClaimNext(ctx context.Context, now time.Time, lease time.Duration) (*model.AutomationScheduledEvent, error) {
	if lease <= 0 {
		return nil, fmt.Errorf("scheduled event lease must be positive")
	}
	var claimed *model.AutomationScheduledEvent
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var event model.AutomationScheduledEvent
		err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("due_at <= ? AND available_at <= ?", now, now).
			Where("status = ? OR (status = ? AND lease_until <= ?)", model.AutomationEventScheduled, model.AutomationEventProcessing, now).
			Order("available_at ASC, due_at ASC, id ASC").Take(&event).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if event.Attempts >= event.MaxAttempts {
			if err := tx.Model(&event).Updates(map[string]any{"status": model.AutomationEventFailed, "result_code": "delivery_exhausted",
				"lease_token": nil, "lease_until": nil, "completed_at": now, "updated_at": now}).Error; err != nil {
				return err
			}
			event.Status = model.AutomationEventFailed
			claimed = &event // Let the dispatcher keep draining instead of starving later events.
			return nil
		}
		token, until := uuid.NewString(), now.Add(lease)
		event.Status, event.Attempts = model.AutomationEventProcessing, event.Attempts+1
		event.LeaseToken, event.LeaseUntil, event.UpdatedAt = &token, &until, now
		if err := tx.Model(&event).Updates(map[string]any{"status": event.Status, "attempts": event.Attempts,
			"lease_token": token, "lease_until": until, "updated_at": now}).Error; err != nil {
			return err
		}
		claimed = &event
		return nil
	})
	return claimed, err
}

// LockClaim loads the canonical, live claim for an idempotent domain consumer.
// It must be called inside a transaction, after any relevant domain locks.
func (r *AutomationScheduledEventRepository) LockClaim(ctx context.Context, claim model.AutomationScheduledEvent, now time.Time) (*model.AutomationScheduledEvent, error) {
	if claim.LeaseToken == nil {
		return nil, ErrAutomationEventLeaseLost
	}
	var event model.AutomationScheduledEvent
	err := activeAutomationClaim(r.db.WithContext(ctx), claim, now).
		Clauses(clause.Locking{Strength: "UPDATE"}).Take(&event).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAutomationEventLeaseLost
	}
	return &event, err
}

// Complete acknowledges a fenced claim. A domain consumer commits this receipt
// and any permitted local effect together; external effects need their own action identity.
func (r *AutomationScheduledEventRepository) Complete(ctx context.Context, claim model.AutomationScheduledEvent, code string, now time.Time) error {
	if code == "" || len(code) > 100 {
		return fmt.Errorf("invalid scheduled event result")
	}
	return updateAutomationClaim(activeAutomationClaim(r.db.WithContext(ctx), claim, now), map[string]any{
		"status": model.AutomationEventDelivered, "result_code": code, "completed_at": now,
		"lease_token": nil, "lease_until": nil, "updated_at": now,
	})
}

// DeliveryFinished verifies that a consumer acknowledged the event or lost its
// claim. Returning nil without committing a receipt cannot count as delivery.
func (r *AutomationScheduledEventRepository) DeliveryFinished(ctx context.Context, claim model.AutomationScheduledEvent) (bool, error) {
	var event model.AutomationScheduledEvent
	err := r.db.WithContext(ctx).Select("status", "lease_token").
		Where("workspace_id = ? AND id = ?", claim.WorkspaceID, claim.ID).Take(&event).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return event.Status != model.AutomationEventProcessing || event.LeaseToken == nil || claim.LeaseToken == nil || *event.LeaseToken != *claim.LeaseToken, nil
}

// Retry records a safe error code without persisting potentially sensitive error text.
func (r *AutomationScheduledEventRepository) Retry(ctx context.Context, claim model.AutomationScheduledEvent, code string, permanent bool, now time.Time) error {
	if code == "" || len(code) > 100 {
		return fmt.Errorf("invalid scheduled event failure code")
	}
	updates := map[string]any{"status": model.AutomationEventScheduled, "result_code": code,
		"lease_token": nil, "lease_until": nil, "updated_at": now,
		"available_at": now.Add(time.Minute * time.Duration(1<<min(max(claim.Attempts-1, 0), 8)))}
	if permanent || claim.Attempts >= claim.MaxAttempts {
		updates["status"], updates["completed_at"] = model.AutomationEventFailed, now
	}
	return updateAutomationClaim(activeAutomationClaim(r.db.WithContext(ctx), claim, now), updates)
}

func activeAutomationClaim(db *gorm.DB, claim model.AutomationScheduledEvent, now time.Time) *gorm.DB {
	return db.Model(&model.AutomationScheduledEvent{}).
		Where("workspace_id = ? AND id = ? AND status = ? AND lease_token = ? AND lease_until > ?", claim.WorkspaceID, claim.ID, model.AutomationEventProcessing, claim.LeaseToken, now)
}

func updateAutomationClaim(query *gorm.DB, updates map[string]any) error {
	result := query.Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrAutomationEventLeaseLost
	}
	return nil
}
