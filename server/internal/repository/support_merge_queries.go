package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// MergeCandidates searches only visible, active original records. It uses the
// existing mailbox authorization and conversation search predicates.
func (r *SupportConversationRepository) MergeCandidates(ctx context.Context, workspaceID, memberID, role string, source model.SupportConversation, search string, allCustomers bool, page int) ([]model.SupportConversation, bool, error) {
	if page < 1 {
		page = 1
	}
	if page > 1000 {
		return nil, false, fmt.Errorf("invalid page")
	}
	q := r.db.WithContext(ctx).Model(&model.SupportConversation{}).Where("support_conversations.workspace_id = ? AND support_conversations.id <> ? AND support_conversations.merged_into_id IS NULL AND support_conversations.anonymized_at IS NULL AND support_conversations.status <> 'spam'", workspaceID, source.ID)
	q = r.applyMailboxAccess(q, "support_conversations", memberID, role)
	if !allCustomers {
		parts := []string{}
		args := []any{}
		if source.CRMContactID != nil {
			// Distinct known contacts never match through a shared browser or email.
			q = q.Where("crm_contact_id IS NULL OR crm_contact_id = ?", *source.CRMContactID)
			parts = append(parts, "crm_contact_id = ?")
			args = append(args, *source.CRMContactID)
		}
		if source.AnonymousID != nil && strings.TrimSpace(*source.AnonymousID) != "" {
			parts = append(parts, "anonymous_id = ?")
			args = append(args, *source.AnonymousID)
		}
		if source.CustomerEmail != nil && strings.TrimSpace(*source.CustomerEmail) != "" && source.PrimaryRecipientState != model.SupportPrimaryRecipientStateUnconfirmed {
			part := "LOWER(TRIM(customer_email)) = ? AND COALESCE(primary_recipient_state, '') <> 'unconfirmed'"
			args = append(args, strings.ToLower(strings.TrimSpace(*source.CustomerEmail)))
			parts = append(parts, "("+part+")")
		}
		if len(parts) == 0 {
			return []model.SupportConversation{}, false, nil
		}
		q = q.Where("("+strings.Join(parts, " OR ")+")", args...)
	}
	if search != "" {
		q = r.applyConversationSearch(q, search)
	}
	rows := []model.SupportConversation{}
	err := q.Order("updated_at DESC,id").Limit(26).Offset((page - 1) * 25).Find(&rows).Error
	more := len(rows) > 25
	if more {
		rows = rows[:25]
	}
	return rows, more, err
}

// VisitorHistory scopes both the page and its count before reading any titles.
func (r *SupportConversationRepository) VisitorHistory(ctx context.Context, workspaceID, memberID, role string, conversation model.SupportConversation) ([]model.SupportConversation, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.SupportConversation{}).Where("support_conversations.workspace_id = ?", workspaceID)
	if conversation.CRMContactID != nil {
		q = q.Where("crm_contact_id = ?", *conversation.CRMContactID)
	} else if conversation.AnonymousID != nil {
		q = q.Where("anonymous_id = ?", *conversation.AnonymousID)
	} else {
		return []model.SupportConversation{}, 1, nil
	}
	q = r.applyMailboxAccess(q, "support_conversations", memberID, role)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.SupportConversation
	err := q.Where("support_conversations.id <> ?", conversation.ID).Order("created_at DESC,id").Limit(25).Find(&rows).Error
	return rows, total, err
}
