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

// ListRecent returns recent support email logs ordered newest-first.
func (r *SupportEmailLogRepository) ListRecent(ctx context.Context, limit int) ([]model.SupportEmailLog, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	var logs []model.SupportEmailLog
	if err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("list recent support email logs: %w", err)
	}
	return logs, nil
}

// CountByDirectionStatus returns grouped log counts for admin diagnostics.
func (r *SupportEmailLogRepository) CountByDirectionStatus(ctx context.Context) ([]model.EmailLogCount, error) {
	var counts []model.EmailLogCount
	if err := r.db.WithContext(ctx).
		Model(&model.SupportEmailLog{}).
		Select("direction, status, count(*) as count").
		Group("direction, status").
		Order("direction ASC, status ASC").
		Scan(&counts).Error; err != nil {
		return nil, fmt.Errorf("count support email logs by status: %w", err)
	}
	return counts, nil
}

// GetByMessageID returns the email log that referenced the given support_message ID.
func (r *SupportEmailLogRepository) GetByMessageID(ctx context.Context, workspaceID, messageID string) (*model.SupportEmailLog, error) {
	if workspaceID == "" || messageID == "" {
		return nil, nil
	}
	if r.db != nil && r.db.Dialector != nil && r.db.Dialector.Name() == "sqlite" {
		var logs []model.SupportEmailLog
		if err := r.db.WithContext(ctx).
			Where("workspace_id = ?", workspaceID).
			Order("created_at DESC").
			Find(&logs).Error; err != nil {
			return nil, fmt.Errorf("get support email log by message id: %w", err)
		}
		for i := range logs {
			for _, id := range logs[i].MessageIDs {
				if strings.TrimSpace(id) == messageID {
					return &logs[i], nil
				}
			}
		}
		return nil, nil
	}
	var log model.SupportEmailLog
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND ? = ANY(message_ids)", workspaceID, messageID).
		Order("created_at DESC").
		First(&log).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get support email log by message id: %w", err)
	}
	return &log, nil
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

// MarkDelivered updates an outbound email log with its first observed delivery time.
func (r *SupportEmailLogRepository) MarkDelivered(ctx context.Context, id string, deliveredAt time.Time) error {
	if id == "" {
		return nil
	}
	if err := r.db.WithContext(ctx).
		Model(&model.SupportEmailLog{}).
		Where("id = ? AND (delivered_at IS NULL OR delivered_at > ?)", id, deliveredAt).
		Updates(map[string]any{
			"status": gorm.Expr(
				"CASE WHEN status IN ('opened', 'bounced', 'spam_complaint') THEN status ELSE ? END",
				"delivered",
			),
			"delivered_at": deliveredAt,
		}).Error; err != nil {
		return fmt.Errorf("mark support email log delivered: %w", err)
	}
	return nil
}

// MarkBounced updates an outbound email log with its first observed bounce or complaint time.
func (r *SupportEmailLogRepository) MarkBounced(ctx context.Context, id, status string, bouncedAt time.Time, errorMessage string) error {
	if id == "" {
		return nil
	}
	status = strings.TrimSpace(status)
	if status == "" {
		status = "bounced"
	}
	errorMessage = strings.TrimSpace(errorMessage)
	if err := r.db.WithContext(ctx).
		Model(&model.SupportEmailLog{}).
		Where(
			"id = ? AND (bounced_at IS NULL OR bounced_at > ? OR status <> ? OR COALESCE(error_message, '') <> ?)",
			id,
			bouncedAt,
			status,
			errorMessage,
		).
		Updates(map[string]any{
			"status":        status,
			"bounced_at":    bouncedAt,
			"error_message": errorMessage,
		}).Error; err != nil {
		return fmt.Errorf("mark support email log bounced: %w", err)
	}
	return nil
}

// WithTx returns a new SupportEmailLogRepository using the provided transaction.
func (r *SupportEmailLogRepository) WithTx(tx *gorm.DB) *SupportEmailLogRepository {
	return &SupportEmailLogRepository{db: tx}
}
