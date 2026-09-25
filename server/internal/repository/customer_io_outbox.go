package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const customerIOLastErrorMaxBytes = 2 * 1024

var _customerIOSQLiteClaimMutex sync.Mutex

// CustomerIOLifecycleOutboxRepository persists Customer.io lifecycle delivery work.
type CustomerIOLifecycleOutboxRepository struct {
	db      *gorm.DB
	claimMu *sync.Mutex
}

// CustomerIOLifecycleEventInput is the durable workspace-event payload to enqueue.
type CustomerIOLifecycleEventInput struct {
	SemanticKey string
	WorkspaceID string
	EventName   string
	OccurredAt  time.Time
	Attributes  map[string]any
}

// NewCustomerIOLifecycleOutboxRepository creates a Customer.io lifecycle outbox repository.
func NewCustomerIOLifecycleOutboxRepository(db *gorm.DB) *CustomerIOLifecycleOutboxRepository {
	repository := &CustomerIOLifecycleOutboxRepository{db: db}
	if db != nil && db.Dialector != nil && db.Dialector.Name() == "sqlite" {
		repository.claimMu = &_customerIOSQLiteClaimMutex
	}
	return repository
}

// EnqueueCustomerIOLifecycleEventTx snapshots recipients and enqueues an event
// inside the caller transaction, so retries cannot consume an uncommitted event.
func EnqueueCustomerIOLifecycleEventTx(
	ctx context.Context,
	tx *gorm.DB,
	input CustomerIOLifecycleEventInput,
) (*model.CustomerIOOutbox, error) {
	attributes, err := json.Marshal(input.Attributes)
	if err != nil {
		return nil, fmt.Errorf("marshal Customer.io lifecycle attributes: %w", err)
	}
	var recipients []model.CustomerIOOutboxRecipient
	if err := tx.WithContext(ctx).
		Table("workspace_members").
		Select("user_id, role AS workspace_role").
		Where("workspace_id = ? AND status = ? AND user_id IS NOT NULL", input.WorkspaceID, model.WorkspaceMemberStatusActive).
		Order("user_id ASC").
		Scan(&recipients).Error; err != nil {
		return nil, fmt.Errorf("snapshot Customer.io lifecycle recipients: %w", err)
	}
	recipientSnapshot, err := json.Marshal(recipients)
	if err != nil {
		return nil, fmt.Errorf("marshal Customer.io lifecycle recipients: %w", err)
	}
	workspaceID := input.WorkspaceID
	event := &model.CustomerIOOutbox{
		ID:                uuid.NewString(),
		SemanticKey:       input.SemanticKey,
		WorkspaceID:       &workspaceID,
		EventName:         input.EventName,
		OccurredAt:        input.OccurredAt.UTC(),
		Attributes:        attributes,
		RecipientSnapshot: recipientSnapshot,
		Status:            model.CustomerIOOutboxStatusPending,
		NextAttemptAt:     input.OccurredAt.UTC(),
	}
	if err := tx.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "semantic_key"}}, DoNothing: true}).
		Create(event).Error; err != nil {
		return nil, fmt.Errorf("enqueue Customer.io lifecycle event %q: %w", input.SemanticKey, err)
	}
	return event, nil
}

// ClaimDue exclusively leases due Customer.io lifecycle events for delivery.
func (r *CustomerIOLifecycleOutboxRepository) ClaimDue(
	ctx context.Context,
	now time.Time,
	leaseDuration time.Duration,
	limit int,
) ([]model.CustomerIOOutbox, error) {
	if limit <= 0 {
		return nil, nil
	}
	if r.claimMu != nil {
		r.claimMu.Lock()
		defer r.claimMu.Unlock()
	}

	var claimed []model.CustomerIOOutbox
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Where(
			"(status = ? AND next_attempt_at <= ?) OR "+
				"(status = ? AND lease_expires_at <= ?)",
			model.CustomerIOOutboxStatusPending,
			now,
			model.CustomerIOOutboxStatusProcessing,
			now,
		).Order("next_attempt_at ASC, created_at ASC").Limit(limit)
		if locking := customerIOClaimLockingClause(tx.Dialector.Name()); locking != nil {
			query = query.Clauses(locking)
		}

		var due []model.CustomerIOOutbox
		if err := query.Find(&due).Error; err != nil {
			return fmt.Errorf("select due Customer.io lifecycle events: %w", err)
		}

		leaseExpiresAt := now.Add(leaseDuration)
		for i := range due {
			token := uuid.NewString()
			result := tx.Model(&model.CustomerIOOutbox{}).
				Where("id = ?", due[i].ID).
				Where(
					"(status = ? AND next_attempt_at <= ?) OR "+
						"(status = ? AND lease_expires_at <= ?)",
					model.CustomerIOOutboxStatusPending,
					now,
					model.CustomerIOOutboxStatusProcessing,
					now,
				).
				Updates(map[string]any{
					"status":           model.CustomerIOOutboxStatusProcessing,
					"claim_token":      token,
					"claimed_at":       now,
					"lease_expires_at": leaseExpiresAt,
					"attempts":         gorm.Expr("attempts + 1"),
				})
			if result.Error != nil {
				return fmt.Errorf("claim Customer.io lifecycle event %q: %w", due[i].ID, result.Error)
			}
			if result.RowsAffected == 0 {
				continue
			}
			due[i].Status = model.CustomerIOOutboxStatusProcessing
			due[i].ClaimToken = &token
			due[i].ClaimedAt = &now
			due[i].LeaseExpiresAt = &leaseExpiresAt
			due[i].Attempts++
			claimed = append(claimed, due[i])
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("claim due Customer.io lifecycle events: %w", err)
	}
	return claimed, nil
}

// MarkDelivered completes a Customer.io lifecycle event when the caller owns its claim.
func (r *CustomerIOLifecycleOutboxRepository) MarkDelivered(
	ctx context.Context,
	id string,
	claimToken string,
) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&model.CustomerIOOutbox{}).
		Where("id = ? AND status = ? AND claim_token = ?", id,
			model.CustomerIOOutboxStatusProcessing, claimToken).
		Updates(map[string]any{
			"status":           model.CustomerIOOutboxStatusDelivered,
			"claim_token":      nil,
			"claimed_at":       nil,
			"lease_expires_at": nil,
			"last_error":       nil,
		})
	if result.Error != nil {
		return false, fmt.Errorf("mark Customer.io lifecycle event %q delivered: %w", id, result.Error)
	}
	return result.RowsAffected == 1, nil
}

// ScheduleRetry releases an owned claim and schedules the event for another attempt.
func (r *CustomerIOLifecycleOutboxRepository) ScheduleRetry(
	ctx context.Context,
	id string,
	claimToken string,
	nextAttemptAt time.Time,
	lastError string,
) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&model.CustomerIOOutbox{}).
		Where("id = ? AND status = ? AND claim_token = ?", id,
			model.CustomerIOOutboxStatusProcessing, claimToken).
		Updates(map[string]any{
			"status":           model.CustomerIOOutboxStatusPending,
			"next_attempt_at":  nextAttemptAt,
			"claim_token":      nil,
			"claimed_at":       nil,
			"lease_expires_at": nil,
			"last_error":       truncateCustomerIOLastError(lastError),
		})
	if result.Error != nil {
		return false, fmt.Errorf("schedule Customer.io lifecycle event %q retry: %w", id, result.Error)
	}
	return result.RowsAffected == 1, nil
}

// MarkFailed terminally fails a Customer.io lifecycle event when the caller owns its claim.
func (r *CustomerIOLifecycleOutboxRepository) MarkFailed(
	ctx context.Context,
	id string,
	claimToken string,
	lastError string,
) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&model.CustomerIOOutbox{}).
		Where("id = ? AND status = ? AND claim_token = ?", id,
			model.CustomerIOOutboxStatusProcessing, claimToken).
		Updates(map[string]any{
			"status":           model.CustomerIOOutboxStatusFailed,
			"claim_token":      nil,
			"claimed_at":       nil,
			"lease_expires_at": nil,
			"last_error":       truncateCustomerIOLastError(lastError),
		})
	if result.Error != nil {
		return false, fmt.Errorf("mark Customer.io lifecycle event %q failed: %w", id, result.Error)
	}
	return result.RowsAffected == 1, nil
}

func truncateCustomerIOLastError(lastError string) string {
	if len(lastError) <= customerIOLastErrorMaxBytes {
		return lastError
	}
	lastError = lastError[:customerIOLastErrorMaxBytes]
	for !utf8.ValidString(lastError) {
		lastError = lastError[:len(lastError)-1]
	}
	return lastError
}

func customerIOClaimLockingClause(dialect string) clause.Expression {
	if dialect != "postgres" {
		return nil
	}
	return clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}
}

// Enqueue persists an event or returns the event already stored for its semantic key.
func (r *CustomerIOLifecycleOutboxRepository) Enqueue(
	ctx context.Context,
	event *model.CustomerIOOutbox,
) (*model.CustomerIOOutbox, error) {
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "semantic_key"}},
			DoNothing: true,
		}).
		Create(event).Error; err != nil {
		return nil, fmt.Errorf("enqueue Customer.io lifecycle event %q: %w", event.SemanticKey, err)
	}

	var persisted model.CustomerIOOutbox
	if err := r.db.WithContext(ctx).
		Where("semantic_key = ?", event.SemanticKey).
		First(&persisted).Error; err != nil {
		return nil, fmt.Errorf("load Customer.io lifecycle event %q: %w", event.SemanticKey, err)
	}
	return &persisted, nil
}
