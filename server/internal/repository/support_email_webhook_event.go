package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SupportEmailWebhookEventRepository handles DB operations for raw support email webhooks.
type SupportEmailWebhookEventRepository struct {
	db *gorm.DB
}

// NewSupportEmailWebhookEventRepository creates a new SupportEmailWebhookEventRepository.
func NewSupportEmailWebhookEventRepository(db *gorm.DB) *SupportEmailWebhookEventRepository {
	return &SupportEmailWebhookEventRepository{db: db}
}

// Create inserts a new webhook event row.
func (r *SupportEmailWebhookEventRepository) Create(ctx context.Context, event *model.SupportEmailWebhookEvent) error {
	if err := r.db.WithContext(ctx).Create(event).Error; err != nil {
		return fmt.Errorf("create support email webhook event: %w", err)
	}
	return nil
}

// ListByConversation returns webhook events for a conversation ordered oldest-first.
func (r *SupportEmailWebhookEventRepository) ListByConversation(ctx context.Context, workspaceID, conversationID string) ([]model.SupportEmailWebhookEvent, error) {
	var events []model.SupportEmailWebhookEvent
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND conversation_id = ?", workspaceID, conversationID).
		Order("created_at ASC").
		Find(&events).Error; err != nil {
		return nil, fmt.Errorf("list support email webhook events: %w", err)
	}
	return events, nil
}

// ListPaginated returns a paginated list of webhook events ordered newest-first.
func (r *SupportEmailWebhookEventRepository) ListPaginated(ctx context.Context, page, perPage int, eventType, provider string) (*model.WebhookEventListResponse, error) {
	q := r.db.WithContext(ctx).Model(&model.SupportEmailWebhookEvent{})

	if eventType != "" {
		q = q.Where("event_type = ?", eventType)
	}
	if provider != "" {
		q = q.Where("provider = ?", provider)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count webhook events: %w", err)
	}

	var events []model.SupportEmailWebhookEvent
	if err := q.Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&events).Error; err != nil {
		return nil, fmt.Errorf("list webhook events: %w", err)
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	return &model.WebhookEventListResponse{
		Data:       events,
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

// GetByID returns a single webhook event by ID.
func (r *SupportEmailWebhookEventRepository) GetByID(ctx context.Context, id string) (*model.SupportEmailWebhookEvent, error) {
	var event model.SupportEmailWebhookEvent
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&event).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get webhook event %s: %w", id, err)
	}
	return &event, nil
}

// WithTx returns a new SupportEmailWebhookEventRepository using the provided transaction.
func (r *SupportEmailWebhookEventRepository) WithTx(tx *gorm.DB) *SupportEmailWebhookEventRepository {
	return &SupportEmailWebhookEventRepository{db: tx}
}
