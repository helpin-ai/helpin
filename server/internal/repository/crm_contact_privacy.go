package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DeleteAnonymizingSupport removes a workspace-owned contact and anonymizes its
// linked support timeline atomically. Missing contacts are idempotent successes.
// The caller must authorize this destructive operation in the same workspace.
func (r *CRMContactRepository) DeleteAnonymizingSupport(ctx context.Context, workspaceID, contactID string) ([]string, error) {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(contactID) == "" {
		return nil, errors.New("workspace and contact are required")
	}
	var conversationIDs []string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := LockContactAnonymization(tx, workspaceID); err != nil {
			return err
		}
		var contact model.CRMContact
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", workspaceID, contactID).First(&contact).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		var anonymousIDs []string
		if err := tx.Model(&model.CRMIdentityLink{}).Where("workspace_id = ? AND contact_id = ?", workspaceID, contactID).Pluck("anonymous_id", &anonymousIDs).Error; err != nil {
			return err
		}

		anonymousIDs, err = exclusiveContactAnonymousIDs(tx, workspaceID, contactID, anonymousIDs)
		if err != nil {
			return err
		}
		// Explicit contact ownership wins. Legacy conversations without a CRM link
		// may be linked by the contact's workspace-local identity or exact email.
		linked := tx.Where("crm_contact_id = ?", contactID)
		if len(anonymousIDs) > 0 {
			linked = linked.Or("crm_contact_id IS NULL AND anonymous_id IN ?", anonymousIDs)
		}
		if contact.Email != nil && strings.TrimSpace(*contact.Email) != "" {
			normalizedEmail := strings.ToLower(strings.TrimSpace(*contact.Email))
			var others int64
			if err := tx.Model(&model.CRMContact{}).Where("workspace_id = ? AND id <> ? AND LOWER(TRIM(email)) = ?", workspaceID, contactID, normalizedEmail).Count(&others).Error; err != nil {
				return err
			}
			// Emails are not unique. Unattributed history is not proof of ownership
			// when another contact shares that address.
			if others == 0 {
				linked = linked.Or("crm_contact_id IS NULL AND LOWER(TRIM(customer_email)) = ?", normalizedEmail)
			}
		}
		var conversations []model.SupportConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ?", workspaceID).Where(linked).Order("id").Find(&conversations).Error; err != nil {
			return err
		}
		for _, conv := range conversations {
			conversationIDs = append(conversationIDs, conv.ID)
			if conv.AnonymousID != nil && *conv.AnonymousID != "" {
				anonymousIDs = append(anonymousIDs, *conv.AnonymousID)
			}
		}
		anonymousIDs, err = exclusiveContactAnonymousIDs(tx, workspaceID, contactID, anonymousIDs)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		sessions := tx.Model(&model.SupportWidgetSession{}).Where("workspace_id = ?", workspaceID).Where("conversation_id IN ? OR (conversation_id IS NULL AND anonymous_id IN ?)", conversationIDs, anonymousIDs)
		var sessionIDs []string
		if err := sessions.Pluck("id", &sessionIDs).Error; err != nil {
			return err
		}
		if len(sessionIDs) > 0 {
			if err := tx.Model(&model.SupportWidgetSession{}).Where("workspace_id = ? AND id IN ?", workspaceID, sessionIDs).Updates(map[string]any{
				"revoked_at": now, "expires_at": now, "session_token": gorm.Expr("'revoked:' || CAST(id AS text)"), "anonymous_id": gorm.Expr("'deleted:' || CAST(id AS text)"),
				"customer_name": nil, "customer_email": nil, "customer_phone": nil, "user_agent": nil, "last_page_url": nil, "timezone": nil, "locale": nil,
				"ip_address": nil, "country_code": nil, "country_name": nil, "region_name": nil, "city_name": nil, "crm_company_id": nil,
				"identity_method": "anonymous", "identity_trust": "untrusted", "identity_verified_at": nil, "identity_verifier_version": nil, "updated_at": now,
			}).Error; err != nil {
				return err
			}
		}
		for _, item := range []struct{ table, column string }{
			{"support_messages", "metadata"}, {"support_email_webhook_events", "raw_payload"}, {"support_conversation_triage_events", "payload"},
		} {
			if err := redactContactMetadata(tx.Unscoped().Where("workspace_id = ? AND conversation_id IN ?", workspaceID, conversationIDs), item.table, item.column); err != nil {
				return err
			}
		}
		if err := redactContactMetadata(tx.Where("workspace_id = ? AND (conversation_id IN ? OR widget_session_id IN ? OR (conversation_id IS NULL AND anonymous_id IN ?))", workspaceID, conversationIDs, sessionIDs, anonymousIDs), "support_events", "metadata"); err != nil {
			return err
		}
		if err := redactContactMetadata(tx.Where("workspace_id = ? AND contact_id = ?", workspaceID, contactID), "crm_activities", "metadata"); err != nil {
			return err
		}
		if len(conversationIDs) > 0 {
			// Include soft-deleted rows: privacy cleanup must not leave hidden originals.
			if err := tx.Unscoped().Model(&model.SupportMessage{}).Where("workspace_id = ? AND conversation_id IN ? AND sender_type = ?", workspaceID, conversationIDs, "customer").Updates(map[string]any{
				"sender_display_name": nil, "sender_avatar_url": nil, "cancellable_until": nil,
			}).Error; err != nil {
				return err
			}
			// Preserve the opaque provider receipt: retries must not recreate this customer.
			if err := tx.Model(&model.SupportEmailLog{}).Where("workspace_id = ? AND conversation_id IN ?", workspaceID, conversationIDs).Updates(map[string]any{
				"from_email": "", "from_display_name": "", "to_email": "", "reply_to": "", "recipient_address": "", "cc_emails": nil, "bcc_emails": nil,
				"rfc_message_id": "", "in_reply_to": "", "references_header": "", "error_message": "",
			}).Error; err != nil {
				return err
			}
			if err := tx.Where("workspace_id = ? AND conversation_id IN ?", workspaceID, conversationIDs).Delete(&model.SupportAIFollowUp{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.SupportEvent{}).Where("workspace_id = ? AND (conversation_id IN ? OR widget_session_id IN ? OR (conversation_id IS NULL AND anonymous_id IN ?))", workspaceID, conversationIDs, sessionIDs, anonymousIDs).Updates(map[string]any{"anonymous_id": nil}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.CRMIdentityLink{}).Where("workspace_id = ? AND contact_id = ?", workspaceID, contactID).Updates(map[string]any{
			"anonymous_id": gorm.Expr("'deleted:' || CAST(id AS text)"), "external_user_id": nil, "contact_id": nil, "company_id": nil, "identity_method": "anonymous", "identity_trust": "untrusted", "company_match_method": nil, "verified_at": nil, "verifier_version": nil,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.CRMActivity{}).Where("workspace_id = ? AND contact_id = ?", workspaceID, contactID).Updates(map[string]any{"contact_id": nil}).Error; err != nil {
			return err
		}
		// Set the marker last: PostgreSQL guards reject future writes to this timeline.
		if len(conversationIDs) > 0 {
			if err := tx.Table("support_conversations").Where("workspace_id = ? AND id IN ?", workspaceID, conversationIDs).Updates(map[string]any{
				"anonymized_at": now, "crm_contact_id": nil, "crm_company_id": nil, "customer_name": "Deleted customer", "customer_email": nil, "customer_phone": nil,
				"anonymous_id": nil, "email_cc": nil, "email_thread_participants": nil, "suggested_primary_recipient_email": nil, "suggested_primary_recipient_name": nil,
				"last_public_sender_display_name": gorm.Expr("CASE WHEN last_public_sender_type = 'customer' THEN NULL ELSE last_public_sender_display_name END"),
				"visitor_country_code":            nil, "country_code": nil, "country_name": nil, "last_message_sender_display_name": nil, "visitor_country_name": nil, "view_search_document": gorm.Expr("LOWER(CONCAT_WS(' ', subject, 'Deleted customer'))"), "human_takeover": true, "email_unsubscribed": true,
				"ai_active_run_id": nil, "customer_awaiting_response": false, "needs_human_reply": false, "updated_at": now,
			}).Error; err != nil {
				return err
			}
		}
		// The inbox projection trigger writes OLD/NEW row snapshots during the
		// update above. Redact these after the marker, in the same transaction.
		for _, column := range []string{"old_values", "new_values"} {
			if err := redactContactMetadata(tx.Where("workspace_id = ? AND conversation_id IN ?", workspaceID, conversationIDs), "support_inbox_conversation_changes", column); err != nil {
				return err
			}
		}

		return tx.Where("workspace_id = ? AND id = ?", workspaceID, contactID).Delete(&model.CRMContact{}).Error
	})
	if err != nil {
		return nil, fmt.Errorf("anonymize contact support records: %w", err)
	}
	return conversationIDs, nil
}

// A browser can have belonged to multiple people. Only infer ownership from it
// when its recorded identity links do not also identify another contact.
func exclusiveContactAnonymousIDs(tx *gorm.DB, workspaceID, contactID string, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var shared []string
	if err := tx.Model(&model.CRMIdentityLink{}).Where("workspace_id = ? AND contact_id <> ? AND anonymous_id IN ?", workspaceID, contactID, ids).Pluck("anonymous_id", &shared).Error; err != nil {
		return nil, err
	}
	excluded := make(map[string]bool, len(shared))
	for _, id := range shared {
		excluded[id] = true
	}
	filtered := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" && !excluded[id] {
			filtered = append(filtered, id)
		}
	}
	return filtered, nil
}

// LockContactAnonymization coordinates widget conversation admission with contact
// deletion across processes. Call inside a transaction, before any row locks.
// A workspace lock also covers anonymous sessions with no contact/email yet.
func LockContactAnonymization(tx *gorm.DB, workspaceID string) error {
	if tx.Dialector.Name() != "postgres" {
		return nil // SQLite is used only by isolated unit tests.
	}
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "support-contact-anonymization:"+workspaceID).Error
}
