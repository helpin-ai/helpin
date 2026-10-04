package repository

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// FlowBuilderReferenceExists checks workspace ownership using a fixed table
// allowlist. Names supplied by the model are never interpolated into SQL.
func (r *DockChatRepository) FlowBuilderReferenceExists(ctx context.Context, workspaceID, kind, id string) (bool, error) {
	tables := map[string]string{"task": "pm_tasks", "epic": "pm_epics", "repository": "git_repositories", "space": "docs_spaces", "collection": "docs_collections", "crm_deal": "crm_deals", "crm_contact": "crm_contacts", "crm_company": "crm_companies", "team": "workspace_teams", "workflow": "pm_workflows"}
	table, ok := tables[kind]
	if !ok {
		return false, fmt.Errorf("unsupported reference type %q", kind)
	}
	var count int64
	err := r.db.WithContext(ctx).Table(table).Where("workspace_id = ? AND id = ?", workspaceID, id).Count(&count).Error
	return count == 1, err
}

func (r *DockChatRepository) LatestFlowBuilderUserMessage(ctx context.Context, workspaceID, chatID string) (string, error) {
	var messages []model.AgentRunMessage
	err := r.db.WithContext(ctx).Select("id").Where("workspace_id = ? AND dock_chat_id = ? AND role = ? AND actor_user_id IS NOT NULL AND message_type <> ?", workspaceID, chatID, "user", "approval").Order("dock_chat_sequence DESC").Order("created_at DESC").Order("id DESC").Limit(1).Find(&messages).Error
	if err != nil || len(messages) == 0 {
		return "", err
	}
	return messages[0].ID, nil
}

// FlowBuilderReferenceLabel resolves display text from the same workspace as the flow.
func (r *DockChatRepository) FlowBuilderReferenceLabel(ctx context.Context, workspaceID, kind, id string) (string, error) {
	tables := map[string]string{"task": "pm_tasks", "epic": "pm_epics", "repository": "git_repositories", "crm_deal": "crm_deals", "crm_contact": "crm_contacts", "crm_company": "crm_companies", "space": "docs_spaces", "collection": "docs_collections"}
	table, ok := tables[kind]
	if !ok {
		return "", nil
	}
	column := "name"
	if kind == "repository" {
		column = "full_name"
	}
	if kind == "crm_contact" {
		column = "TRIM(first_name || ' ' || COALESCE(last_name, ''))"
	}
	var row struct{ Label string }
	err := r.db.WithContext(ctx).Table(table).Select(column+" AS label").Where("workspace_id = ? AND id = ?", workspaceID, id).Scan(&row).Error
	return row.Label, err
}

// ValidateFlowDocsReference checks both workspace ownership and dependent selection constraints.
func (r *DockChatRepository) ValidateFlowDocsReference(ctx context.Context, workspaceID, spaceID, collectionID, spaceType string) error {
	if spaceID != "" {
		var count int64
		q := r.db.WithContext(ctx).Table("docs_spaces").Where("workspace_id = ? AND id = ? AND deleted_at IS NULL", workspaceID, spaceID)
		if spaceType != "" && spaceType != "any" {
			q = q.Where("type = ?", spaceType)
		}
		if err := q.Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("choose an available %s docs space", spaceType)
		}
	}
	if collectionID != "" {
		var count int64
		q := r.db.WithContext(ctx).Table("docs_collections").Where("workspace_id = ? AND id = ? AND deleted_at IS NULL", workspaceID, collectionID)
		if spaceID != "" {
			q = q.Where("space_id = ?", spaceID)
		}
		if err := q.Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("choose a collection in the selected space")
		}
	}
	return nil
}
