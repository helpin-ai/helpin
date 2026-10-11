package repository

import (
	"context"
	"encoding/json"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ListForSupportAI scopes context to the audience of the next reply. A
// cross-customer merge must never expose either customer's earlier private
// discussion to the answering agent, even when invoked through a tool.
func (r *SupportMessageRepository) ListForSupportAI(ctx context.Context, workspaceID, conversationID string, includeInternal bool) ([]model.SupportMessage, error) {
	var conversation model.SupportConversation
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, conversationID).First(&conversation).Error; err != nil {
		return nil, err
	}
	if conversation.MergeRevision == 0 {
		return r.ListByConversation(ctx, workspaceID, conversationID, includeInternal)
	}
	repo := NewSupportMergeRepository(r.db)
	root, err := repo.Root(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	group, err := repo.Group(ctx, workspaceID, root)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	cross := false
	for _, c := range group {
		if c.AnonymizedAt != nil {
			continue
		}
		ids = append(ids, c.ID)
		if !model.SameSupportCustomer(&conversation, &c) && c.ID != conversation.ID {
			cross = true
		}
	}
	q := r.db.WithContext(ctx).Model(&model.SupportMessage{}).Where("workspace_id = ?", workspaceID).Where("NOT EXISTS (SELECT 1 FROM support_conversations original WHERE original.id = support_messages.origin_conversation_id AND original.workspace_id = support_messages.workspace_id AND original.anonymized_at IS NOT NULL)")
	if cross {
		sources := []string{}
		for _, id := range ids {
			if id != root {
				sources = append(sources, id)
			}
		}
		q = q.Where("conversation_id = ? AND is_internal = false AND (SELECT COUNT(DISTINCT grant_row.conversation_id) FROM support_merge_messages grant_row WHERE grant_row.workspace_id = support_messages.workspace_id AND grant_row.message_id = support_messages.id AND grant_row.conversation_id IN ?) = ?", root, sources, len(sources))
	} else {
		q = q.Where("conversation_id IN ?", ids)
		if !includeInternal {
			q = q.Where("is_internal = false")
		}
	}
	rows := []model.SupportMessage{}
	err = q.Order("created_at,id").Find(&rows).Error
	for i := range rows {
		var projection struct {
			Text *string `json:"merge_public_text"`
		}
		if json.Unmarshal([]byte(rows[i].Metadata), &projection) == nil && projection.Text != nil {
			rows[i].Content = *projection.Text
		}
	}
	return rows, err
}

// AIContextConversation removes private original metadata from a shared audience.
func (r *SupportConversationRepository) AIContextConversation(ctx context.Context, c *model.SupportConversation) (*model.SupportConversation, error) {
	if c == nil || c.MergeRevision == 0 {
		return c, nil
	}
	group, err := NewSupportMergeRepository(r.db).Group(ctx, c.WorkspaceID, c.ID)
	if err != nil {
		return nil, err
	}
	for _, member := range group {
		if member.ID != c.ID && !model.SameSupportCustomer(c, &member) {
			safe := *c
			safe.Subject = "Shared customer conversation"
			safe.CRMContactID = nil
			safe.CRMCompanyID = nil
			safe.LinkedTaskID = nil
			safe.Tags = nil
			return &safe, nil
		}
	}
	return c, nil
}
