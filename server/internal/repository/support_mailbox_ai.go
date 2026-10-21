package repository

import (
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// LoadSupportMailboxAIPolicy reads the current inbox policy, including inside
// the routing lock immediately before delayed replies and follow-ups publish.
func LoadSupportMailboxAIPolicy(db *gorm.DB, conversation *model.SupportConversation) error {
	conversation.MailboxAIRepliesDisabled = false
	if conversation.MailboxID == nil {
		return nil
	}
	var mailbox model.SupportMailbox
	if err := db.Select("ai_replies_disabled").Where("workspace_id = ? AND id = ?", conversation.WorkspaceID, *conversation.MailboxID).Find(&mailbox).Error; err != nil {
		return err
	}
	conversation.MailboxAIRepliesDisabled = mailbox.AIRepliesDisabled
	return nil
}
