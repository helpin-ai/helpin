package repository

import (
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// eraseChannelPersonTx removes what still carries a provider customer's id
// once their conversations are erased: queued replies, inbound job payloads,
// the chat id inside stored message keys, and the identities themselves.
// Completed jobs keep their ids, so a provider retry of an old update stays a
// no-op. Call before conversations are marked anonymized or deleted.
func eraseChannelPersonTx(tx *gorm.DB, workspaceID string, conversationIDs, identityIDs []string) error {
	if len(conversationIDs) == 0 && len(identityIDs) == 0 {
		return nil
	}
	if err := tx.Where("workspace_id = ? AND (conversation_id IN ? OR channel_identity_id IN ?)", workspaceID, nonEmpty(conversationIDs), nonEmpty(identityIDs)).
		Delete(&model.SupportChannelOutbox{}).Error; err != nil {
		return err
	}
	// Keys stay unique so a replayed message is still recognised.
	if err := tx.Unscoped().Model(&model.SupportMessage{}).
		Where("workspace_id = ? AND conversation_id IN ? AND external_message_key IS NOT NULL AND external_message_key NOT LIKE 'deleted:%'", workspaceID, nonEmpty(conversationIDs)).
		Update("external_message_key", gorm.Expr("'deleted:' || CAST(id AS text)")).Error; err != nil {
		return err
	}
	var identities []model.SupportChannelIdentity
	if err := tx.Where("workspace_id = ? AND id IN ?", workspaceID, nonEmpty(identityIDs)).Find(&identities).Error; err != nil {
		return err
	}
	subjects := make([]string, 0, len(identities))
	for _, identity := range identities {
		subjects = append(subjects, model.SupportChannelSubjectKey(identity.ChannelAccountID, identity.ExternalUserID))
	}
	// Attachment jobs carry no subject; they are matched by conversation.
	if err := tx.Model(&model.SupportInboundJob{}).
		Where("workspace_id = ? AND kind IN ? AND (subject_key IN ? OR conversation_id IN ?)", workspaceID,
			[]string{"channel_event", "channel_attachment"}, nonEmpty(subjects), nonEmpty(conversationIDs)).
		Updates(map[string]any{"status": "completed", "payload": "", "lease_token": "", "last_error": ""}).Error; err != nil {
		return err
	}
	return tx.Where("workspace_id = ? AND id IN ?", workspaceID, nonEmpty(identityIDs)).Delete(&model.SupportChannelIdentity{}).Error
}

// nonEmpty keeps IN clauses valid for empty lists; no row has the nil UUID.
func nonEmpty(ids []string) []string {
	if len(ids) == 0 {
		return []string{"00000000-0000-0000-0000-000000000000"}
	}
	return ids
}
