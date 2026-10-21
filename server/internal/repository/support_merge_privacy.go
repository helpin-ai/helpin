package repository

import (
	"context"
	"github.com/helpin-ai/helpin/server/internal/model"
	"slices"
	"time"
)

// DetachErasedConversations ends handling grants during the EXISTING authorized
// customer-deletion operation. It does not initiate deletion or erase content.
func (r *SupportMergeRepository) DetachErasedConversations(ctx context.Context, workspaceID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	edges, err := r.Active(ctx, workspaceID)
	if err != nil {
		return err
	}
	affected := []string{}
	actor := ""
	for _, edge := range edges {
		if !slices.Contains(ids, edge.SourceID) && !slices.Contains(ids, edge.TargetID) {
			continue
		}
		root, err := r.Root(ctx, workspaceID, edge.TargetID)
		if err != nil {
			return err
		}
		group, err := r.Group(ctx, workspaceID, root)
		if err != nil {
			return err
		}
		for _, c := range group {
			affected = append(affected, c.ID)
		}
		actor = edge.ActorID
	}
	if len(affected) == 0 {
		return nil
	}
	slices.Sort(affected)
	affected = slices.Compact(affected)
	now := time.Now().UTC()
	if err := r.fence(ctx, workspaceID, affected, actor, now); err != nil {
		return err
	}
	for _, edge := range edges {
		if !slices.Contains(ids, edge.SourceID) && !slices.Contains(ids, edge.TargetID) {
			continue
		}
		if err := r.db.Model(&model.SupportConversationMerge{}).Where("workspace_id = ? AND id = ?", workspaceID, edge.ID).Update("ended_at", now).Error; err != nil {
			return err
		}
		if err := r.db.Model(&model.SupportConversation{}).Where("workspace_id = ? AND id = ?", workspaceID, edge.SourceID).Update("merged_into_id", nil).Error; err != nil {
			return err
		}
	}
	return nil
}
