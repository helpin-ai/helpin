package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// GitWebhookEventRepository handles DB operations for raw git webhooks.
type GitWebhookEventRepository struct {
	db *gorm.DB
}

// NewGitWebhookEventRepository creates a new GitWebhookEventRepository.
func NewGitWebhookEventRepository(db *gorm.DB) *GitWebhookEventRepository {
	return &GitWebhookEventRepository{db: db}
}

// Create inserts a new webhook event row.
func (r *GitWebhookEventRepository) Create(ctx context.Context, event *model.GitWebhookEvent) error {
	if r == nil {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(event).Error; err != nil {
		return fmt.Errorf("create git webhook event: %w", err)
	}
	return nil
}

// MarkHandled stores the final processing outcome for a webhook delivery.
func (r *GitWebhookEventRepository) MarkHandled(ctx context.Context, id, status string, statusCode int, errorMessage *string, integrationID *string, workspaceID *string) error {
	if r == nil || id == "" {
		return nil
	}
	updates := map[string]any{
		"status":      status,
		"status_code": statusCode,
		"updated_at":  gorm.Expr("CURRENT_TIMESTAMP"),
	}
	if errorMessage != nil {
		updates["error_message"] = *errorMessage
	}
	if integrationID != nil {
		updates["integration_id"] = *integrationID
	}
	if workspaceID != nil {
		updates["workspace_id"] = *workspaceID
	}
	if err := r.db.WithContext(ctx).
		Model(&model.GitWebhookEvent{}).
		Where("id = ?", id).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("mark git webhook event handled: %w", err)
	}
	return nil
}
