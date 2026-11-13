package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ListOutboundByIDs returns the outbound email logs among ids, from any
// workspace. Each outbound email carries its log ID in its Message-ID, so a
// reply's threading headers name logs by primary key and need no other index.
func (r *SupportEmailLogRepository) ListOutboundByIDs(ctx context.Context, ids []string) ([]model.SupportEmailLog, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var logs []model.SupportEmailLog
	if err := r.db.WithContext(ctx).
		Select("id", "workspace_id", "conversation_id", "rfc_message_id", "created_at").
		Where("id IN ? AND direction = ?", ids, "outbound").
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("list outbound support email logs by id: %w", err)
	}
	return logs, nil
}

// InboundCopyQuery describes an inbound email that may already have arrived
// through another Helpin address: the same Message-ID, or, when FromEmail is
// set, the same text from the same sender since Since (one message sent
// separately to two addresses gets two Message-IDs).
type InboundCopyQuery struct {
	WorkspaceID string
	// ConversationID narrows the search; empty searches the whole workspace.
	ConversationID string
	RFCMessageID   string
	FromEmail      string
	StrippedText   string
	Since          time.Time
}

// HasInboundCopy reports whether an inbound email matching q was logged.
func (r *SupportEmailLogRepository) HasInboundCopy(ctx context.Context, q InboundCopyQuery) (bool, error) {
	rfcMessageID := strings.TrimSpace(q.RFCMessageID)
	fromEmail := strings.ToLower(strings.TrimSpace(q.FromEmail))
	byText := fromEmail != "" && strings.TrimSpace(q.StrippedText) != ""
	if q.WorkspaceID == "" || (rfcMessageID == "" && !byText) {
		return false, nil
	}
	query := r.db.WithContext(ctx).Model(&model.SupportEmailLog{}).
		Where("workspace_id = ? AND direction = ?", q.WorkspaceID, "inbound")
	if q.ConversationID != "" {
		query = query.Where("conversation_id = ?", q.ConversationID)
	}
	switch {
	case rfcMessageID != "" && byText:
		query = query.Where("(rfc_message_id = ? OR (LOWER(from_email) = ? AND stripped_text = ? AND created_at >= ?))", rfcMessageID, fromEmail, q.StrippedText, q.Since)
	case rfcMessageID != "":
		query = query.Where("rfc_message_id = ?", rfcMessageID)
	default:
		query = query.Where("LOWER(from_email) = ? AND stripped_text = ? AND created_at >= ?", fromEmail, q.StrippedText, q.Since)
	}
	var ids []string
	if err := query.Limit(1).Pluck("id", &ids).Error; err != nil {
		return false, fmt.Errorf("find inbound support email copy: %w", err)
	}
	return len(ids) > 0, nil
}

// OutboundWorkspacesFromSender returns every workspace that emailed recipient
// from sender or a plus-tagged form of it (local+tag@domain).
func (r *SupportEmailLogRepository) OutboundWorkspacesFromSender(ctx context.Context, sender, recipient string) ([]string, error) {
	query := r.outboundFromSender(ctx, sender, recipient)
	if query == nil {
		return nil, nil
	}
	var workspaceIDs []string
	if err := query.Distinct("workspace_id").Pluck("workspace_id", &workspaceIDs).Error; err != nil {
		return nil, fmt.Errorf("list workspaces that emailed from sender: %w", err)
	}
	return workspaceIDs, nil
}

// ListRecentOutboundFromSender returns one workspace's newest outbound emails
// to recipient sent from sender or a plus-tagged form of it.
func (r *SupportEmailLogRepository) ListRecentOutboundFromSender(ctx context.Context, workspaceID, sender, recipient string, limit int) ([]model.SupportEmailLog, error) {
	query := r.outboundFromSender(ctx, sender, recipient)
	if query == nil || workspaceID == "" || limit <= 0 {
		return nil, nil
	}
	var logs []model.SupportEmailLog
	if err := query.
		Select("id", "workspace_id", "conversation_id", "subject", "created_at").
		Where("workspace_id = ?", workspaceID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("list outbound support email logs from sender: %w", err)
	}
	return logs, nil
}

func (r *SupportEmailLogRepository) outboundFromSender(ctx context.Context, sender, recipient string) *gorm.DB {
	local, domain, ok := strings.Cut(strings.ToLower(strings.TrimSpace(sender)), "@")
	recipient = strings.ToLower(strings.TrimSpace(recipient))
	if !ok || local == "" || domain == "" || recipient == "" {
		return nil
	}
	return r.db.WithContext(ctx).Model(&model.SupportEmailLog{}).
		Where("direction = ? AND LOWER(to_email) = ?", "outbound", recipient).
		Where(`(LOWER(from_email) = ? OR LOWER(from_email) LIKE ? ESCAPE '\')`, local+"@"+domain, escapeLike(local)+"+%@"+escapeLike(domain))
}
