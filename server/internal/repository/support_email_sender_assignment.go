package repository

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// SetMailboxSender changes one inbox without replacing other inbox assignments
// or moving the workspace default into an inbox-only scope.
func (r *SupportEmailSenderRepository) SetMailboxSender(ctx context.Context, workspaceID, mailboxID, senderID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := LockSupportRouting(tx, workspaceID); err != nil {
			return err
		}
		repo := NewSupportEmailSenderRepository(tx)
		sender, err := repo.GetByID(ctx, workspaceID, senderID)
		if err != nil {
			return err
		}
		if sender == nil || !sender.DKIMVerified || !sender.ReturnPathDomainVerified {
			return fmt.Errorf("sender must be verified")
		}
		// Materialize legacy single-inbox mappings before changing the chosen inbox.
		var all []model.SupportEmailSender
		if err := tx.Where("workspace_id = ? AND default_scope = ?", workspaceID, "mailbox").
			Where("id = ? OR mailbox_id = ? OR id IN (?)", senderID, mailboxID,
				tx.Model(&model.SupportEmailSenderMailbox{}).Select("sender_id").Where("workspace_id = ? AND mailbox_id = ?", workspaceID, mailboxID)).Find(&all).Error; err != nil {
			return err
		}
		for _, old := range all {
			if old.MailboxID != nil {
				var count int64
				if err := tx.Model(&model.SupportEmailSenderMailbox{}).Where("workspace_id = ? AND sender_id = ?", workspaceID, old.ID).Count(&count).Error; err != nil {
					return err
				}
				if count == 0 {
					if err := tx.Create(&model.SupportEmailSenderMailbox{WorkspaceID: workspaceID, SenderID: old.ID, MailboxID: *old.MailboxID}).Error; err != nil {
						return err
					}
				}
			}
		}
		if err := tx.Where("workspace_id = ? AND mailbox_id = ?", workspaceID, mailboxID).Delete(&model.SupportEmailSenderMailbox{}).Error; err != nil {
			return err
		}
		if sender.DefaultScope != "workspace" {
			if err := tx.Create(&model.SupportEmailSenderMailbox{WorkspaceID: workspaceID, SenderID: senderID, MailboxID: mailboxID}).Error; err != nil {
				return err
			}
			if sender.DefaultScope != "mailbox" {
				all = append(all, *sender)
			}
		}
		for _, old := range all {
			var mappings []model.SupportEmailSenderMailbox
			if err := tx.Where("workspace_id = ? AND sender_id = ?", workspaceID, old.ID).Order("mailbox_id").Find(&mappings).Error; err != nil {
				return err
			}
			fields := map[string]any{"default_scope": "none", "active": false, "mailbox_id": nil}
			if len(mappings) > 0 {
				fields = map[string]any{"default_scope": "mailbox", "active": true, "mailbox_id": mappings[0].MailboxID}
			}
			if err := tx.Model(&model.SupportEmailSender{}).Where("workspace_id = ? AND id = ?", workspaceID, old.ID).Updates(fields).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
