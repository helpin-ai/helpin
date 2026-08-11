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

const productAnalyticsLastErrorMaxBytes = 2 * 1024

var _productAnalyticsSQLiteClaimMutex sync.Mutex

// ProductAnalyticsEventInput is a canonical product event queued for delivery.
type ProductAnalyticsEventInput struct {
	SemanticKey string
	UserID      string
	AnonymousID string
	WorkspaceID string
	EventName   string
	Source      string
	OccurredAt  time.Time
	Attributes  map[string]any
}

// ProductAnalyticsOutboxRepository persists canonical product analytics events.
type ProductAnalyticsOutboxRepository struct {
	db      *gorm.DB
	claimMu *sync.Mutex
}

// NewProductAnalyticsOutboxRepository creates a product analytics outbox repository.
func NewProductAnalyticsOutboxRepository(db *gorm.DB) *ProductAnalyticsOutboxRepository {
	repo := &ProductAnalyticsOutboxRepository{db: db}
	if db != nil && db.Dialector != nil && db.Dialector.Name() == "sqlite" {
		repo.claimMu = &_productAnalyticsSQLiteClaimMutex
	}
	return repo
}

// Enqueue inserts an event once using its semantic key as the idempotency boundary.
func (r *ProductAnalyticsOutboxRepository) Enqueue(
	ctx context.Context,
	input ProductAnalyticsEventInput,
) (*model.ProductAnalyticsOutbox, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	attributes, err := json.Marshal(input.Attributes)
	if err != nil {
		return nil, fmt.Errorf("marshal product analytics attributes: %w", err)
	}
	occurredAt := input.OccurredAt.UTC()
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	event := &model.ProductAnalyticsOutbox{
		ID:            uuid.NewString(),
		SemanticKey:   input.SemanticKey,
		UserID:        optionalAnalyticsID(input.UserID),
		AnonymousID:   optionalAnalyticsID(input.AnonymousID),
		WorkspaceID:   optionalAnalyticsID(input.WorkspaceID),
		EventName:     input.EventName,
		Source:        input.Source,
		OccurredAt:    occurredAt,
		Attributes:    attributes,
		Status:        model.ProductAnalyticsOutboxStatusPending,
		NextAttemptAt: occurredAt,
	}
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "semantic_key"}}, DoNothing: true}).
		Create(event)
	if result.Error != nil {
		return nil, fmt.Errorf("enqueue product analytics event %q: %w", input.SemanticKey, result.Error)
	}
	if result.RowsAffected == 0 {
		var existing model.ProductAnalyticsOutbox
		if err := r.db.WithContext(ctx).Where("semantic_key = ?", input.SemanticKey).First(&existing).Error; err != nil {
			return nil, fmt.Errorf("load existing product analytics event %q: %w", input.SemanticKey, err)
		}
		return &existing, nil
	}
	return event, nil
}

// ClaimDue exclusively leases due events for delivery.
func (r *ProductAnalyticsOutboxRepository) ClaimDue(
	ctx context.Context,
	now time.Time,
	leaseDuration time.Duration,
	limit int,
) ([]model.ProductAnalyticsOutbox, error) {
	if limit <= 0 {
		return nil, nil
	}
	if r.claimMu != nil {
		r.claimMu.Lock()
		defer r.claimMu.Unlock()
	}
	var claimed []model.ProductAnalyticsOutbox
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Where(
			"(status = ? AND next_attempt_at <= ?) OR (status = ? AND lease_expires_at <= ?)",
			model.ProductAnalyticsOutboxStatusPending, now,
			model.ProductAnalyticsOutboxStatusProcessing, now,
		).Order("next_attempt_at ASC, created_at ASC").Limit(limit)
		if tx.Dialector.Name() != "sqlite" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}
		var due []model.ProductAnalyticsOutbox
		if err := query.Find(&due).Error; err != nil {
			return fmt.Errorf("select due product analytics events: %w", err)
		}
		leaseExpiresAt := now.Add(leaseDuration)
		for i := range due {
			token := uuid.NewString()
			result := tx.Model(&model.ProductAnalyticsOutbox{}).
				Where("id = ?", due[i].ID).
				Where(
					"(status = ? AND next_attempt_at <= ?) OR (status = ? AND lease_expires_at <= ?)",
					model.ProductAnalyticsOutboxStatusPending, now,
					model.ProductAnalyticsOutboxStatusProcessing, now,
				).
				Updates(map[string]any{
					"status": model.ProductAnalyticsOutboxStatusProcessing, "claim_token": token,
					"claimed_at": now, "lease_expires_at": leaseExpiresAt,
					"attempts": gorm.Expr("attempts + 1"),
				})
			if result.Error != nil {
				return fmt.Errorf("claim product analytics event %q: %w", due[i].ID, result.Error)
			}
			if result.RowsAffected == 0 {
				continue
			}
			due[i].Status = model.ProductAnalyticsOutboxStatusProcessing
			due[i].ClaimToken = &token
			due[i].Attempts++
			claimed = append(claimed, due[i])
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("claim due product analytics events: %w", err)
	}
	return claimed, nil
}

// MarkDelivered completes an event when the caller owns its claim.
func (r *ProductAnalyticsOutboxRepository) MarkDelivered(
	ctx context.Context,
	id, claimToken string,
) (bool, error) {
	return r.updateClaim(ctx, id, claimToken, map[string]any{
		"status": model.ProductAnalyticsOutboxStatusDelivered, "claim_token": nil,
		"claimed_at": nil, "lease_expires_at": nil, "last_error": nil,
	})
}

// ScheduleRetry releases a claim and schedules another delivery attempt.
func (r *ProductAnalyticsOutboxRepository) ScheduleRetry(
	ctx context.Context,
	id, claimToken string,
	nextAttemptAt time.Time,
	lastError string,
) (bool, error) {
	return r.updateClaim(ctx, id, claimToken, map[string]any{
		"status": model.ProductAnalyticsOutboxStatusPending, "next_attempt_at": nextAttemptAt,
		"claim_token": nil, "claimed_at": nil, "lease_expires_at": nil,
		"last_error": truncateProductAnalyticsError(lastError),
	})
}

// MarkFailed terminally fails an event when the caller owns its claim.
func (r *ProductAnalyticsOutboxRepository) MarkFailed(
	ctx context.Context,
	id, claimToken, lastError string,
) (bool, error) {
	return r.updateClaim(ctx, id, claimToken, map[string]any{
		"status": model.ProductAnalyticsOutboxStatusFailed, "claim_token": nil,
		"claimed_at": nil, "lease_expires_at": nil,
		"last_error": truncateProductAnalyticsError(lastError),
	})
}

func (r *ProductAnalyticsOutboxRepository) updateClaim(
	ctx context.Context,
	id, claimToken string,
	updates map[string]any,
) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.ProductAnalyticsOutbox{}).
		Where("id = ? AND status = ? AND claim_token = ?", id,
			model.ProductAnalyticsOutboxStatusProcessing, claimToken).
		Updates(updates)
	if result.Error != nil {
		return false, fmt.Errorf("update product analytics event %q: %w", id, result.Error)
	}
	return result.RowsAffected == 1, nil
}

func optionalAnalyticsID(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func truncateProductAnalyticsError(value string) string {
	if len(value) <= productAnalyticsLastErrorMaxBytes {
		return value
	}
	value = value[:productAnalyticsLastErrorMaxBytes]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}
