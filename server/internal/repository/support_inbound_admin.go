package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// GetEmail retrieves an email receipt, excluding attachment jobs and erased timelines.
func (r *SupportInboundJobRepository) GetEmail(ctx context.Context, id string) (*model.SupportInboundJob, error) {
	var job model.SupportInboundJob
	err := r.adminBase(ctx).Select("j.*").Where("j.id = ?", id).Take(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &job, err
}

// AdminEmailLog retrieves original content only from the receipt's own workspace and log.
func (r *SupportInboundJobRepository) AdminEmailLog(ctx context.Context, job *model.SupportInboundJob) (*model.SupportEmailLog, error) {
	var log model.SupportEmailLog
	err := r.db.WithContext(ctx).Where("id = ? AND workspace_id = ?", job.EmailLogID, job.WorkspaceID).Take(&log).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &log, err
}

// ListAdmin returns a bounded queue page without loading raw attachments.
func (r *SupportInboundJobRepository) ListAdmin(ctx context.Context, status, search string, page int) (*model.AdminInboundEmailList, error) {
	result := &model.AdminInboundEmailList{Entries: []model.AdminInboundEmail{}, Page: page, PerPage: 25}
	if err := r.adminBase(ctx).Where(adminInboundAttention).Count(&result.AttentionCount).Error; err != nil {
		return nil, err
	}
	query := r.adminBase(ctx)
	switch status {
	case "attention":
		query = query.Where(adminInboundAttention)
	case "recovered":
		query = query.Where("j.status = 'completed' AND j.recovery_requested_at IS NOT NULL AND j.email_log_id IS NOT NULL")
	case "processing":
		query = query.Where("j.recovery_requested_at IS NOT NULL AND j.status IN ('pending', 'processing')")
	default:
		query = query.Where("(" + adminInboundAttention + ") OR j.recovery_requested_at IS NOT NULL")
	}
	if search != "" {
		query = query.Where("LOWER("+r.payloadField("Subject", "el.subject")+") LIKE ? ESCAPE '\\' OR LOWER("+r.payloadField("FromFull.Email", r.payloadField("From", "el.from_email"))+") LIKE ? ESCAPE '\\' OR LOWER("+r.payloadField("FromFull.Name", "el.from_display_name")+") LIKE ? ESCAPE '\\'", inboundSearchPattern(search), inboundSearchPattern(search), inboundSearchPattern(search))
	}
	if err := query.Count(&result.Total).Error; err != nil {
		return nil, err
	}
	err := query.Select(r.adminSelection()).Order("j.created_at DESC, j.id DESC").Offset((page - 1) * 25).Limit(25).Scan(&result.Entries).Error
	return result, err
}

// AdminSummary returns the same safe projection used by the queue.
func (r *SupportInboundJobRepository) AdminSummary(ctx context.Context, id string) (*model.AdminInboundEmail, error) {
	var result model.AdminInboundEmail
	err := r.adminBase(ctx).Select(r.adminSelection()).Where("j.id = ?", id).Take(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &result, err
}

// RequestRecovery conditionally requeues a failed receipt without replacing its payload.
// A worker claim or another admin's request makes the update a harmless conflict.
func (r *SupportInboundJobRepository) RequestRecovery(ctx context.Context, job *model.SupportInboundJob, actor string, target *model.SupportConversation, now time.Time) (bool, error) {
	values := map[string]any{"status": "pending", "attempts": 0, "available_at": now, "last_error": "", "lease_token": "", "recovery_requested_by": actor, "recovery_requested_at": now}
	if target != nil {
		values["recovery_conversation_id"] = target.ID
		values["workspace_id"] = target.WorkspaceID
		values["conversation_id"] = target.ID
	}
	result := r.db.WithContext(ctx).Model(&model.SupportInboundJob{}).
		Where("id = ? AND kind = 'email' AND status = ? AND updated_at = ? AND attempts = ? AND attempts > 0 AND payload <> ''", job.ID, job.Status, job.UpdatedAt, job.Attempts).
		Updates(values)
	return result.RowsAffected == 1, result.Error
}

// AdminCandidates finds potential customer threads; the service validates exact participants.
func (r *SupportInboundJobRepository) AdminCandidates(ctx context.Context, workspaceID *string, sender, query, conversationID string) ([]model.SupportConversation, error) {
	q := r.db.WithContext(ctx).Model(&model.SupportConversation{}).
		Where("anonymized_at IS NULL AND merged_into_id IS NULL AND status <> 'spam'")
	if workspaceID != nil {
		q = q.Where("workspace_id = ?", *workspaceID)
	}
	if conversationID != "" {
		q = q.Where("id = ?", conversationID)
	} else {
		q = q.Where("LOWER(customer_email) = ? OR LOWER(CAST(email_cc AS TEXT)) LIKE ? ESCAPE '\\' OR LOWER(CAST(email_thread_participants AS TEXT)) LIKE ? ESCAPE '\\'", sender, inboundSearchPattern(sender), inboundSearchPattern(sender))
		if query != "" {
			q = q.Where("LOWER(subject) LIKE ? ESCAPE '\\' OR LOWER(customer_email) LIKE ? ESCAPE '\\' OR LOWER(customer_name) LIKE ? ESCAPE '\\'", inboundSearchPattern(query), inboundSearchPattern(query), inboundSearchPattern(query))
		}
	}
	var result []model.SupportConversation
	err := q.Order("updated_at DESC, id DESC").Limit(25).Find(&result).Error
	return result, err
}

// AdminConversation returns concise destination details for a valid conversation.
func (r *SupportInboundJobRepository) AdminConversation(ctx context.Context, id string) (*model.AdminInboundConversation, error) {
	var result model.AdminInboundConversation
	err := r.db.WithContext(ctx).Table("support_conversations AS c").
		Joins("LEFT JOIN workspaces w ON w.id = c.workspace_id").
		Joins("LEFT JOIN support_mailboxes mb ON mb.id = c.mailbox_id AND mb.workspace_id = c.workspace_id").
		Where("c.id = ? AND c.anonymized_at IS NULL", id).
		Select("c.id, c.display_id, c.subject, c.workspace_id, COALESCE(w.name, '') AS workspace_name, COALESCE(w.slug, '') AS workspace_slug, COALESCE(mb.name, 'Shared Inbox') AS inbox_name, COALESCE(c.customer_name, '') AS customer_name, COALESCE(c.customer_email, '') AS customer_email").Take(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var reply model.SupportMessage
	err = r.db.WithContext(ctx).Select("content").Where("conversation_id = ? AND workspace_id = ? AND is_internal = false AND sender_type IN ('user', 'agent', 'ai') AND message_type = 'reply'", id, result.WorkspaceID).Order("created_at DESC, id DESC").Limit(1).Find(&reply).Error
	result.LastTeamReply = reply.Content
	if len(result.LastTeamReply) > 2000 {
		result.LastTeamReply = string([]rune(result.LastTeamReply)[:min(2000, len([]rune(result.LastTeamReply)))])
	}
	return &result, err
}

const adminInboundAttention = "j.payload <> '' AND (j.status = 'failed' OR (j.status = 'pending' AND j.attempts > 0))"

func (r *SupportInboundJobRepository) adminBase(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Table("support_inbound_jobs AS j").
		Joins("LEFT JOIN workspaces w ON w.id = j.workspace_id").
		Joins("LEFT JOIN support_conversations c ON c.id = j.conversation_id AND c.workspace_id = j.workspace_id").
		Joins("LEFT JOIN support_email_logs el ON el.id = j.email_log_id AND el.workspace_id = j.workspace_id").
		Joins("LEFT JOIN users u ON u.id = j.recovery_requested_by").
		Where("j.kind = 'email' AND (j.conversation_id IS NULL OR (c.id IS NOT NULL AND c.anonymized_at IS NULL))")
}

func (r *SupportInboundJobRepository) adminSelection() string {
	return "j.id, j.status, j.workspace_id, COALESCE(w.name, '') AS workspace_name, j.conversation_id, j.created_at AS received_at, j.updated_at, j.available_at, j.attempts, j.recovery_requested_at, COALESCE(u.full_name, '') AS recovery_actor_name, (j.workspace_id IS NULL AND j.conversation_id IS NULL) AS requires_destination, " +
		r.payloadField("Subject", "el.subject") + " AS subject, " +
		r.payloadField("FromFull.Email", r.payloadField("From", "el.from_email")) + " AS sender_email, " +
		r.payloadField("FromFull.Name", "el.from_display_name") + " AS sender_name, " +
		r.payloadField("To", r.payloadField("OriginalRecipient", "el.to_email")) + " AS recipient"
}

// All paths and fallback expressions below are internal constants, never request input.
func (r *SupportInboundJobRepository) payloadField(field, fallback string) string {
	value := "NULLIF(j.payload, '')::jsonb #>> '{" + strings.ReplaceAll(field, ".", ",") + "}'"
	if r.db.Dialector.Name() == "sqlite" {
		value = "json_extract(NULLIF(j.payload, ''), '$." + field + "')"
	}
	return "COALESCE(NULLIF(" + value + ", ''), " + fallback + ", '')"
}

func inboundSearchPattern(value string) string {
	return "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(strings.ToLower(value)) + "%"
}
