package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrWebhookEndpointNotFound means the endpoint does not exist in the workspace.
var ErrWebhookEndpointNotFound = errors.New("webhook endpoint not found")

// ErrWebhookDeliveryNotFound means the delivery does not exist for the endpoint.
var ErrWebhookDeliveryNotFound = errors.New("webhook delivery not found")

// WebhookRepository stores webhook endpoints, the event outbox and deliveries.
type WebhookRepository struct {
	db *gorm.DB
}

func NewWebhookRepository(db *gorm.DB) *WebhookRepository {
	return &WebhookRepository{db: db}
}

func (r *WebhookRepository) postgres() bool { return r.db.Dialector.Name() == "postgres" }

// ListEndpoints returns the workspace's endpoints, oldest first.
func (r *WebhookRepository) ListEndpoints(ctx context.Context, workspaceID string) ([]model.WebhookEndpoint, error) {
	var endpoints []model.WebhookEndpoint
	if err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Order("created_at, id").Find(&endpoints).Error; err != nil {
		return nil, fmt.Errorf("list webhook endpoints: %w", err)
	}
	return endpoints, nil
}

// GetEndpoint returns one endpoint.
func (r *WebhookRepository) GetEndpoint(ctx context.Context, workspaceID, endpointID string) (*model.WebhookEndpoint, error) {
	var endpoint model.WebhookEndpoint
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, endpointID).Take(&endpoint).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrWebhookEndpointNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get webhook endpoint: %w", err)
	}
	return &endpoint, nil
}

// CountEndpoints returns how many endpoints the workspace has.
func (r *WebhookRepository) CountEndpoints(ctx context.Context, workspaceID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.WebhookEndpoint{}).Where("workspace_id = ?", workspaceID).Count(&count).Error
	return count, err
}

// CreateEndpoint inserts an endpoint.
func (r *WebhookRepository) CreateEndpoint(ctx context.Context, endpoint *model.WebhookEndpoint) error {
	if err := r.db.WithContext(ctx).Create(endpoint).Error; err != nil {
		return fmt.Errorf("create webhook endpoint: %w", err)
	}
	return nil
}

// UpdateEndpoint writes the given fields of an endpoint.
func (r *WebhookRepository) UpdateEndpoint(ctx context.Context, workspaceID, endpointID string, fields map[string]any) error {
	fields["updated_at"] = time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&model.WebhookEndpoint{}).Where("workspace_id = ? AND id = ?", workspaceID, endpointID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update webhook endpoint: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrWebhookEndpointNotFound
	}
	return nil
}

// DeleteEndpoint removes an endpoint and its delivery log.
func (r *WebhookRepository) DeleteEndpoint(ctx context.Context, workspaceID, endpointID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("workspace_id = ? AND id = ?", workspaceID, endpointID).Delete(&model.WebhookEndpoint{})
		if result.Error != nil {
			return fmt.Errorf("delete webhook endpoint: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrWebhookEndpointNotFound
		}
		if err := tx.Where("endpoint_id = ?", endpointID).Delete(&model.WebhookDelivery{}).Error; err != nil {
			return fmt.Errorf("delete webhook deliveries: %w", err)
		}
		return nil
	})
}

// EnabledEndpoints returns the workspace's enabled endpoints.
func (r *WebhookRepository) EnabledEndpoints(ctx context.Context, workspaceID string) ([]model.WebhookEndpoint, error) {
	var endpoints []model.WebhookEndpoint
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND enabled = ?", workspaceID, true).Order("created_at, id").Find(&endpoints).Error; err != nil {
		return nil, fmt.Errorf("list enabled webhook endpoints: %w", err)
	}
	return endpoints, nil
}

// InsertEvent records an outbox event.
func (r *WebhookRepository) InsertEvent(ctx context.Context, event *model.WebhookEvent) error {
	if err := r.db.WithContext(ctx).Create(event).Error; err != nil {
		return fmt.Errorf("record webhook event: %w", err)
	}
	return nil
}

// DispatchEvents locks up to limit available events, lets build turn them
// into deliveries, then stores the deliveries and removes the events in one
// transaction, so an event is delivered once even with several workers.
func (r *WebhookRepository) DispatchEvents(ctx context.Context, now time.Time, limit int,
	build func(events []model.WebhookEvent) ([]model.WebhookDelivery, error)) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	processed := 0
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Where("available_at <= ?", now).Order("created_at, id").Limit(limit)
		if r.postgres() {
			query = query.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}
		var events []model.WebhookEvent
		if err := query.Find(&events).Error; err != nil {
			return fmt.Errorf("claim webhook events: %w", err)
		}
		if len(events) == 0 {
			return nil
		}
		deliveries, err := build(events)
		if err != nil {
			return err
		}
		if len(deliveries) > 0 {
			if err := tx.CreateInBatches(deliveries, 200).Error; err != nil {
				return fmt.Errorf("create webhook deliveries: %w", err)
			}
		}
		ids := make([]string, 0, len(events))
		for _, event := range events {
			ids = append(ids, event.ID)
		}
		if err := tx.Where("id IN ?", ids).Delete(&model.WebhookEvent{}).Error; err != nil {
			return fmt.Errorf("remove dispatched webhook events: %w", err)
		}
		processed = len(events)
		return nil
	})
	return processed, err
}

// CreateDelivery inserts a delivery, such as a test event or a redelivery.
func (r *WebhookRepository) CreateDelivery(ctx context.Context, delivery *model.WebhookDelivery) error {
	if err := r.db.WithContext(ctx).Create(delivery).Error; err != nil {
		return fmt.Errorf("create webhook delivery: %w", err)
	}
	return nil
}

// ClaimDeliveries leases up to limit pending deliveries that are due, hiding
// them from other workers until leaseUntil. A crashed worker's deliveries
// return when the lease ends.
func (r *WebhookRepository) ClaimDeliveries(ctx context.Context, now, leaseUntil time.Time, limit int) ([]model.WebhookDelivery, error) {
	if limit <= 0 {
		limit = 50
	}
	leaseUntil = leaseUntil.UTC().Truncate(time.Microsecond)
	var claimed []model.WebhookDelivery
	if r.postgres() {
		err := r.db.WithContext(ctx).Raw(`
			UPDATE webhook_deliveries SET next_attempt_at = ?
			WHERE id IN (
				SELECT id FROM webhook_deliveries
				WHERE status = 'pending' AND next_attempt_at <= ?
				ORDER BY next_attempt_at
				LIMIT ?
				FOR UPDATE SKIP LOCKED
			)
			RETURNING *`, leaseUntil, now, limit).Scan(&claimed).Error
		if err != nil {
			return nil, fmt.Errorf("claim webhook deliveries: %w", err)
		}
		return claimed, nil
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("status = ? AND next_attempt_at <= ?", model.WebhookDeliveryPending, now).
			Order("next_attempt_at").Limit(limit).Find(&claimed).Error; err != nil {
			return err
		}
		ids := make([]string, 0, len(claimed))
		for i := range claimed {
			ids = append(ids, claimed[i].ID)
			claimed[i].NextAttemptAt = leaseUntil
		}
		if len(ids) == 0 {
			return nil
		}
		return tx.Model(&model.WebhookDelivery{}).Where("id IN ?", ids).Update("next_attempt_at", leaseUntil).Error
	})
	if err != nil {
		return nil, fmt.Errorf("claim webhook deliveries: %w", err)
	}
	return claimed, nil
}

// WebhookAttempt is the outcome of one delivery attempt.
type WebhookAttempt struct {
	StatusCode    *int
	Error         string
	At            time.Time
	DurationMS    int
	Succeeded     bool
	NextAttemptAt *time.Time // nil when no retry follows
}

// RecordAttempt stores an attempt's outcome on the delivery and its
// endpoint and returns the endpoint's consecutive failure count.
func (r *WebhookRepository) RecordAttempt(ctx context.Context, delivery model.WebhookDelivery, attempt WebhookAttempt) (int, error) {
	failures := 0
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		fields := map[string]any{
			"attempts": gorm.Expr("attempts + 1"), "last_status_code": attempt.StatusCode, "last_error": attempt.Error,
			"last_attempt_at": attempt.At, "duration_ms": attempt.DurationMS,
		}
		switch {
		case attempt.Succeeded:
			fields["status"], fields["delivered_at"] = model.WebhookDeliverySucceeded, attempt.At
		case attempt.NextAttemptAt != nil:
			fields["next_attempt_at"] = *attempt.NextAttemptAt
		default:
			fields["status"] = model.WebhookDeliveryFailed
		}
		if err := tx.Model(&model.WebhookDelivery{}).Where("id = ?", delivery.ID).Updates(fields).Error; err != nil {
			return fmt.Errorf("record webhook attempt: %w", err)
		}
		endpointFields := map[string]any{"last_success_at": attempt.At, "failure_count": 0}
		if !attempt.Succeeded {
			endpointFields = map[string]any{"last_failure_at": attempt.At, "failure_count": gorm.Expr("failure_count + 1")}
		}
		if err := tx.Model(&model.WebhookEndpoint{}).Where("id = ?", delivery.EndpointID).Updates(endpointFields).Error; err != nil {
			return fmt.Errorf("record webhook endpoint health: %w", err)
		}
		var health struct{ FailureCount int }
		if err := tx.Model(&model.WebhookEndpoint{}).Select("failure_count").Where("id = ?", delivery.EndpointID).Scan(&health).Error; err != nil {
			return fmt.Errorf("read webhook endpoint health: %w", err)
		}
		failures = health.FailureCount
		return nil
	})
	return failures, err
}

// DisableEndpoint turns an endpoint off with a reason and fails its pending
// deliveries.
func (r *WebhookRepository) DisableEndpoint(ctx context.Context, endpointID, reason string, at time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.WebhookEndpoint{}).Where("id = ?", endpointID).
			Updates(map[string]any{"enabled": false, "disabled_reason": reason, "updated_at": at}).Error; err != nil {
			return fmt.Errorf("disable webhook endpoint: %w", err)
		}
		return r.failPending(tx, endpointID, "The endpoint was turned off.")
	})
}

// FailPendingDeliveries ends the endpoint's pending deliveries.
func (r *WebhookRepository) FailPendingDeliveries(ctx context.Context, endpointID, reason string) error {
	return r.failPending(r.db.WithContext(ctx), endpointID, reason)
}

func (r *WebhookRepository) failPending(db *gorm.DB, endpointID, reason string) error {
	if err := db.Model(&model.WebhookDelivery{}).Where("endpoint_id = ? AND status = ?", endpointID, model.WebhookDeliveryPending).
		Updates(map[string]any{"status": model.WebhookDeliveryFailed, "last_error": reason}).Error; err != nil {
		return fmt.Errorf("fail pending webhook deliveries: %w", err)
	}
	return nil
}

// ListDeliveries returns an endpoint's most recent deliveries.
func (r *WebhookRepository) ListDeliveries(ctx context.Context, workspaceID, endpointID string, limit int) ([]model.WebhookDelivery, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var deliveries []model.WebhookDelivery
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND endpoint_id = ?", workspaceID, endpointID).
		Order("created_at DESC, id DESC").Limit(limit).Find(&deliveries).Error; err != nil {
		return nil, fmt.Errorf("list webhook deliveries: %w", err)
	}
	return deliveries, nil
}

// GetDelivery returns one delivery of an endpoint.
func (r *WebhookRepository) GetDelivery(ctx context.Context, workspaceID, endpointID, deliveryID string) (*model.WebhookDelivery, error) {
	var delivery model.WebhookDelivery
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND endpoint_id = ? AND id = ?", workspaceID, endpointID, deliveryID).Take(&delivery).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrWebhookDeliveryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get webhook delivery: %w", err)
	}
	return &delivery, nil
}

// PurgeDeliveries removes finished deliveries created before the cutoff.
func (r *WebhookRepository) PurgeDeliveries(ctx context.Context, before time.Time) error {
	if err := r.db.WithContext(ctx).Where("created_at < ? AND status <> ?", before, model.WebhookDeliveryPending).
		Delete(&model.WebhookDelivery{}).Error; err != nil {
		return fmt.Errorf("purge webhook deliveries: %w", err)
	}
	return nil
}
