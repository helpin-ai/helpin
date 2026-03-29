package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SupportEmailLogRepository handles DB operations for support email logs.
type SupportEmailLogRepository struct {
	db *gorm.DB
}

// NewSupportEmailLogRepository creates a new SupportEmailLogRepository.
func NewSupportEmailLogRepository(db *gorm.DB) *SupportEmailLogRepository {
	return &SupportEmailLogRepository{db: db}
}

// Create inserts a new support email log row.
func (r *SupportEmailLogRepository) Create(ctx context.Context, log *model.SupportEmailLog) error {
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("create support email log: %w", err)
	}
	return nil
}

// ListByConversation returns email logs for a conversation ordered oldest-first.
func (r *SupportEmailLogRepository) ListByConversation(ctx context.Context, workspaceID, conversationID string) ([]model.SupportEmailLog, error) {
	var logs []model.SupportEmailLog
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND conversation_id = ?", workspaceID, conversationID).
		Order("created_at ASC").
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("list support email logs: %w", err)
	}
	return logs, nil
}

// GetByPostmarkMessageID returns an existing log for the provider message ID.
func (r *SupportEmailLogRepository) GetByPostmarkMessageID(ctx context.Context, postmarkMessageID string) (*model.SupportEmailLog, error) {
	if postmarkMessageID == "" {
		return nil, nil
	}
	var log model.SupportEmailLog
	if err := r.db.WithContext(ctx).
		Where("postmark_message_id = ?", postmarkMessageID).
		First(&log).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get support email log by postmark message id: %w", err)
	}
	return &log, nil
}

// FindConversationByRFCReferences resolves a conversation from inbound threading headers.
func (r *SupportEmailLogRepository) FindConversationByRFCReferences(ctx context.Context, workspaceID string, references []string) (*model.SupportEmailLog, error) {
	cleaned := make([]string, 0, len(references))
	for _, reference := range references {
		reference = strings.TrimSpace(reference)
		if reference == "" {
			continue
		}
		cleaned = append(cleaned, reference)
	}
	if len(cleaned) == 0 {
		return nil, nil
	}

	var log model.SupportEmailLog
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND rfc_message_id IN ?", workspaceID, cleaned).
		Order("created_at DESC").
		First(&log).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("find support email log by rfc references: %w", err)
	}
	return &log, nil
}

// MarkOpened updates an outbound email log with its first observed open time.
func (r *SupportEmailLogRepository) MarkOpened(ctx context.Context, id string, openedAt time.Time) error {
	if id == "" {
		return nil
	}
	if err := r.db.WithContext(ctx).
		Model(&model.SupportEmailLog{}).
		Where("id = ? AND (opened_at IS NULL OR opened_at > ?)", id, openedAt).
		Updates(map[string]any{
			"status":    "opened",
			"opened_at": openedAt,
		}).Error; err != nil {
		return fmt.Errorf("mark support email log opened: %w", err)
	}
	return nil
}

// WithTx returns a new SupportEmailLogRepository using the provided transaction.
func (r *SupportEmailLogRepository) WithTx(tx *gorm.DB) *SupportEmailLogRepository {
	return &SupportEmailLogRepository{db: tx}
}
