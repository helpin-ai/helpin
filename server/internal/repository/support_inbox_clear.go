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

// SupportInboxClearRepository stores inbox clears and counts the facts the
// inbox-zero celebration reports.
type SupportInboxClearRepository struct {
	db *gorm.DB
}

// NewSupportInboxClearRepository creates a SupportInboxClearRepository.
func NewSupportInboxClearRepository(db *gorm.DB) *SupportInboxClearRepository {
	return &SupportInboxClearRepository{db: db}
}

// FindConversationStatus returns the status of a conversation in the
// workspace. The boolean is false when the conversation does not exist there.
func (r *SupportInboxClearRepository) FindConversationStatus(ctx context.Context, workspaceID, conversationID string) (string, bool, error) {
	var rows []struct{ Status string }
	if err := r.db.WithContext(ctx).
		Table("support_conversations").
		Select("status").
		Where("id = ? AND workspace_id = ?", conversationID, workspaceID).
		Limit(1).
		Find(&rows).Error; err != nil {
		return "", false, fmt.Errorf("find conversation %q status: %w", conversationID, err)
	}
	if len(rows) == 0 {
		return "", false, nil
	}
	return rows[0].Status, true, nil
}

// CreateIfAbsent records the clear unless this person already recorded a
// clear of the same scope ending with the same conversation. It reports
// whether a row was inserted.
func (r *SupportInboxClearRepository) CreateIfAbsent(ctx context.Context, entry *model.SupportInboxClear) (bool, error) {
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "workspace_id"}, {Name: "user_id"}, {Name: "scope_key"}, {Name: "conversation_id"}},
			DoNothing: true,
		}).
		Create(entry)
	if result.Error != nil {
		return false, fmt.Errorf("record support inbox clear: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

// FindClear returns the clear recorded for the scope and conversation, or nil.
func (r *SupportInboxClearRepository) FindClear(ctx context.Context, workspaceID, userID, scopeKey, conversationID string) (*model.SupportInboxClear, error) {
	var entry model.SupportInboxClear
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND user_id = ? AND scope_key = ? AND conversation_id = ?", workspaceID, userID, scopeKey, conversationID).
		First(&entry).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find support inbox clear: %w", err)
	}
	return &entry, nil
}

// PreviousClearAt returns when this person last cleared the scope before the
// given clear, or nil when they never did.
func (r *SupportInboxClearRepository) PreviousClearAt(ctx context.Context, workspaceID, userID, scopeKey string, before time.Time, excludeConversationID string) (*time.Time, error) {
	var clears []model.SupportInboxClear
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND user_id = ? AND scope_key = ? AND conversation_id <> ? AND cleared_at < ?",
			workspaceID, userID, scopeKey, excludeConversationID, before.UTC()).
		Order("cleared_at DESC").
		Limit(1).
		Find(&clears).Error; err != nil {
		return nil, fmt.Errorf("find previous support inbox clear: %w", err)
	}
	if len(clears) == 0 {
		return nil, nil
	}
	return &clears[0].ClearedAt, nil
}

// CountClearsOnDate counts this person's clears of the scope on a local date.
func (r *SupportInboxClearRepository) CountClearsOnDate(ctx context.Context, workspaceID, userID, scopeKey, localDate string) (int, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.SupportInboxClear{}).
		Where("workspace_id = ? AND user_id = ? AND scope_key = ? AND local_date = ?", workspaceID, userID, scopeKey, localDate).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count support inbox clears on %s: %w", localDate, err)
	}
	return int(count), nil
}

// ListClearDates returns the distinct local dates on which this person cleared
// any scope, newest first.
func (r *SupportInboxClearRepository) ListClearDates(ctx context.Context, workspaceID, userID string, limit int) ([]string, error) {
	var dates []string
	if err := r.db.WithContext(ctx).
		Model(&model.SupportInboxClear{}).
		Where("workspace_id = ? AND user_id = ?", workspaceID, userID).
		Distinct("local_date").
		Order("local_date DESC").
		Limit(limit).
		Pluck("local_date", &dates).Error; err != nil {
		return nil, fmt.Errorf("list support inbox clear dates: %w", err)
	}
	return dates, nil
}

// CountResolvedBy counts the conversations this person resolved after since,
// from the internal resolved notices, limited to one inbox when count says so.
func (r *SupportInboxClearRepository) CountResolvedBy(ctx context.Context, workspaceID, userID string, since time.Time, count model.SupportInboxClearCount) (int, error) {
	q := r.db.WithContext(ctx).
		Table("support_messages m").
		Where("m.workspace_id = ? AND m.sender_user_id = ? AND m.message_type = ? AND m.system_event_type = ? AND m.created_at > ?",
			workspaceID, userID, "system", string(model.SystemEventResolved), since.UTC())
	switch {
	case count.MailboxID != nil:
		q = q.Joins("JOIN support_conversations c ON c.id = m.conversation_id").
			Where("c.mailbox_id = ?", *count.MailboxID)
	case count.SharedOnly:
		q = q.Joins("JOIN support_conversations c ON c.id = m.conversation_id").
			Where("c.mailbox_id IS NULL")
	}
	var total int64
	if err := q.Distinct("m.conversation_id").Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count conversations resolved by user: %w", err)
	}
	return int(total), nil
}
