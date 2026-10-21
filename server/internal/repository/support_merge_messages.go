package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// routeSupportMessage runs inside the message's transaction. Customer replies
// follow their authenticated original route; staff drafts never change target.
func routeSupportMessage(ctx context.Context, tx *gorm.DB, message *model.SupportMessage) ([]model.SupportMergeMessage, error) {
	if message == nil || message.ConversationID == "" {
		return nil, nil
	}
	if err := LockSupportRouting(tx, message.WorkspaceID); err != nil {
		return nil, err
	}
	repo := NewSupportMergeRepository(tx)
	conversation, err := repo.LockConversation(ctx, message.WorkspaceID, message.ConversationID)
	if err != nil {
		return nil, err
	}
	if conversation.AnonymizedAt != nil {
		return nil, fmt.Errorf("this conversation is read-only")
	}
	if message.SenderType == "user" && message.MessageType != "system" && !message.TrustedInbound && conversation.MergeRevision > 0 && (message.ExpectedMergeRevision == nil || *message.ExpectedMergeRevision != conversation.MergeRevision) {
		return nil, ErrSupportMergeChanged
	}
	origin := message.ConversationID
	if conversation.MergedIntoID != nil {
		if message.SenderType != "customer" && !message.TrustedInbound {
			return nil, ErrSupportMerged
		}
		root, err := repo.Root(ctx, message.WorkspaceID, origin)
		if err != nil {
			return nil, err
		}
		conversation, err = repo.LockConversation(ctx, message.WorkspaceID, root)
		if err != nil {
			return nil, err
		}
		message.ConversationID = root
	}
	if message.MergeParticipantEmail != nil {
		group, err := repo.Group(ctx, message.WorkspaceID, conversation.ID)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(model.SupportMergeEmailAudience(group), *message.MergeParticipantEmail) {
			return nil, ErrSupportMergeChanged
		}
	}
	if message.OriginConversationID == nil {
		message.OriginConversationID = &origin
	}
	message.MergeRevision = conversation.MergeRevision
	if conversation.MergeRevision == 0 {
		return nil, nil
	}
	edges, err := repo.Active(ctx, message.WorkspaceID)
	if err != nil {
		return nil, err
	}
	parents := mergeParents(edges)
	if message.ID == "" {
		message.ID = uuid.NewString()
	}
	grants := []model.SupportMergeMessage{}
	ids := []string{conversation.ID}
	for _, edge := range edges {
		root, err := model.SupportMergeRoot(edge.SourceID, parents)
		if err != nil {
			return nil, err
		}
		if root != conversation.ID {
			continue
		}
		ids = append(ids, edge.SourceID)
		message.WidgetConversationIDs = append(message.WidgetConversationIDs, edge.SourceID)
		grants = append(grants, model.SupportMergeMessage{WorkspaceID: message.WorkspaceID, ConversationID: edge.SourceID, MessageID: message.ID, MergeID: edge.ID})
	}
	if len(grants) == 0 {
		return nil, nil
	}
	// Store the reviewed merge audience on each reply. Delivery never reconstructs
	// an old message's audience from the later state of the merge graph.
	records, err := repo.Conversations(ctx, message.WorkspaceID, ids)
	if err != nil {
		return nil, err
	}
	emails := model.SupportMergeEmailAudience(records)
	var meta map[string]any
	if err := json.Unmarshal([]byte(message.Metadata), &meta); err != nil && message.Metadata != "" {
		return nil, fmt.Errorf("invalid message metadata")
	}
	if meta == nil {
		meta = map[string]any{}
	}
	meta["merge_email_recipients"] = emails
	meta["merge_shared"] = true
	if message.ViaChannel != nil && *message.ViaChannel == "email" && message.SenderType == "customer" {
		visible := message.EmailVisibleText
		if message.EmailProjectionConfidence == "none" || message.EmailProjectionConfidence == "" {
			visible = "A new email was received. A teammate will review it."
		}
		meta["merge_public_text"] = visible
	}
	encoded, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	message.Metadata = string(encoded)
	return grants, nil
}

func recordSupportMergeMessage(tx *gorm.DB, grants []model.SupportMergeMessage) error {
	if len(grants) == 0 {
		return nil
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&grants).Error
}

// SharedMessages returns a bounded retained interval, excluding removed content.
// Staff services authorize both history container and original message owner.
func (r *SupportMergeRepository) SharedMessages(ctx context.Context, workspaceID, conversationID, mergeID string, publicOnly bool, before *model.SupportMessage, limit int) ([]model.SupportMessage, bool, error) {
	if limit != 0 && (limit < 1 || limit > 100) {
		limit = 50
	}
	q := r.db.WithContext(ctx).Model(&model.SupportMessage{}).
		Joins("JOIN support_merge_messages grant_row ON grant_row.message_id = support_messages.id AND grant_row.workspace_id = support_messages.workspace_id").
		Joins("JOIN support_conversations original ON original.id = COALESCE(support_messages.origin_conversation_id, support_messages.conversation_id) AND original.workspace_id = support_messages.workspace_id").
		Where("grant_row.workspace_id = ? AND grant_row.conversation_id = ? AND original.anonymized_at IS NULL", workspaceID, conversationID)
	if mergeID != "" {
		q = q.Where("grant_row.merge_id = ?", mergeID)
	}
	if publicOnly {
		q = q.Where("support_messages.is_internal = false AND support_messages.message_type <> 'system'")
	}
	if before != nil {
		q = q.Where("support_messages.created_at < ? OR (support_messages.created_at = ? AND support_messages.id < ?)", before.CreatedAt, before.CreatedAt, before.ID)
	}
	rows := []model.SupportMessage{}
	if limit > 0 {
		q = q.Limit(limit + 1)
	}
	err := q.Select("support_messages.*").Order("support_messages.created_at DESC,support_messages.id DESC").Find(&rows).Error
	more := limit > 0 && len(rows) > limit
	if more {
		rows = rows[:limit]
	}
	slices.Reverse(rows)
	return rows, more, err
}

// HydrateMergeMessageGrants preserves per-message audience on attachment/link
// enrichment events too. Ended grants allow updates only to already shared data.
func (r *SupportMessageRepository) HydrateMergeMessageGrants(ctx context.Context, messages []model.SupportMessage) error {
	ids := []string{}
	for _, m := range messages {
		if m.MergeRevision > 0 {
			ids = append(ids, m.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var grants []struct {
		model.SupportMergeMessage
		WidgetReadAt *time.Time
	}
	if err := r.db.WithContext(ctx).Model(&model.SupportMergeMessage{}).Select("support_merge_messages.*, c.contact_last_seen_at AS widget_read_at").Joins("JOIN support_conversations c ON c.id = support_merge_messages.conversation_id AND c.workspace_id = support_merge_messages.workspace_id").Where("message_id IN ? AND c.anonymized_at IS NULL", ids).Find(&grants).Error; err != nil {
		return err
	}
	for i := range messages {
		for _, g := range grants {
			if g.WorkspaceID == messages[i].WorkspaceID && g.MessageID == messages[i].ID {
				messages[i].WidgetConversationIDs = append(messages[i].WidgetConversationIDs, g.ConversationID)
				if g.WidgetReadAt != nil && messages[i].WidgetVisible() && !g.WidgetReadAt.Before(messages[i].CreatedAt) {
					messages[i].WidgetReadAt = g.WidgetReadAt
				}
			}
		}
	}
	return nil
}
