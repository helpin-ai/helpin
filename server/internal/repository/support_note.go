package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

var (
	// ErrSupportNoteConflict prevents silent overwrites from another tab.
	ErrSupportNoteConflict = errors.New("this note changed; reopen it to edit the latest version")
	// ErrSupportNoteInvalid rejects empty notes and unavailable uploads.
	ErrSupportNoteInvalid = errors.New("add text or valid attachments to the note")
)

// UpdateNote atomically replaces an author's note and its attachment links.
func (r *SupportMessageRepository) UpdateNote(ctx context.Context, workspaceID, conversationID, actorID, messageID string, req model.UpdateSupportNoteRequest, now time.Time) (*model.SupportMessage, error) {
	var msg model.SupportMessage
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := LockSupportRouting(tx, workspaceID); err != nil {
			return err
		}
		var conv model.SupportConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND workspace_id = ?", conversationID, workspaceID).First(&conv).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSupportMergeNotFound
			}
			return err
		}
		if conv.MergedIntoID != nil {
			return ErrSupportMerged
		}
		if conv.AnonymizedAt != nil {
			return ErrSupportNoteConflict
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND workspace_id = ? AND conversation_id = ? AND sender_user_id = ? AND sender_type = 'user' AND is_internal = true AND message_type = 'reply'", messageID, workspaceID, conversationID, actorID).First(&msg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSupportNoteConflict
			}
			return err
		}
		if !msg.UpdatedAt.Equal(req.ExpectedUpdatedAt) {
			return ErrSupportNoteConflict
		}
		var attachments []model.SupportAttachment
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ?", workspaceID)
		if req.AttachmentIDs != nil {
			query = query.Where("message_id = ? OR id IN ?", messageID, *req.AttachmentIDs)
		} else {
			query = query.Where("message_id = ?", messageID)
		}
		if err := query.Find(&attachments).Error; err != nil {
			return err
		}
		oldIDs := []string{}
		byID := make(map[string]model.SupportAttachment, len(attachments))
		for _, att := range attachments {
			byID[att.ID] = att
			if att.MessageID != nil && *att.MessageID == messageID {
				oldIDs = append(oldIDs, att.ID)
			}
		}
		nextIDs := oldIDs
		if req.AttachmentIDs != nil {
			nextIDs = uniqueNonEmptyStrings(*req.AttachmentIDs)
			if len(nextIDs) != len(*req.AttachmentIDs) {
				return ErrSupportNoteInvalid
			}
			for _, id := range nextIDs {
				att, ok := byID[id]
				if !ok {
					return ErrSupportNoteInvalid
				}
				if att.MessageID != nil && *att.MessageID == messageID {
					continue
				}
				if att.MessageID != nil || !att.IsUploaded || att.ConversationID == nil || *att.ConversationID != conversationID || att.UploadedByType != "user" || att.UploadedByID == nil || *att.UploadedByID != actorID {
					return ErrSupportNoteInvalid
				}
			}
		}
		if strings.TrimSpace(req.Content) == "" && len(nextIDs) == 0 {
			return ErrSupportNoteInvalid
		}
		slices.Sort(oldIDs)
		slices.Sort(nextIDs)
		if msg.Content == req.Content && slices.Equal(oldIDs, nextIDs) {
			return nil
		}
		if req.AttachmentIDs != nil {
			removed := tx.Model(&model.SupportAttachment{}).Where("workspace_id = ? AND message_id = ?", workspaceID, messageID)
			if len(nextIDs) > 0 {
				removed = removed.Where("id NOT IN ?", nextIDs)
			}
			if err := removed.Update("message_id", nil).Error; err != nil {
				return err
			}
			if len(nextIDs) > 0 {
				result := tx.Model(&model.SupportAttachment{}).Where("workspace_id = ? AND id IN ? AND (message_id IS NULL OR message_id = ?)", workspaceID, nextIDs, messageID).Update("message_id", messageID)
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected != int64(len(nextIDs)) {
					return ErrSupportNoteConflict
				}
			}
		}
		metadata := map[string]json.RawMessage{}
		if strings.TrimSpace(msg.Metadata) != "" {
			if err := json.Unmarshal([]byte(msg.Metadata), &metadata); err != nil {
				return fmt.Errorf("decode note metadata: %w", err)
			}
		}
		if metadata == nil {
			metadata = map[string]json.RawMessage{}
		}
		stamp, err := json.Marshal(now)
		if err != nil {
			return err
		}
		metadata["edited_at"] = stamp
		encoded, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		msg.Content = req.Content
		msg.Metadata = string(encoded)
		msg.UpdatedAt = now
		return tx.Model(&model.SupportMessage{}).Where("id = ? AND workspace_id = ?", messageID, workspaceID).Updates(map[string]any{"content": msg.Content, "metadata": msg.Metadata, "updated_at": now}).Error
	})
	if err != nil {
		return nil, err
	}
	return &msg, nil
}
