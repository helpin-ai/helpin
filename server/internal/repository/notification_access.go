package repository

import (
	"context"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// AgentRunNotificationAccess checks the real chat relationship, never metadata.
// Private chats remain owner-only even for administrators. Existing standalone
// runs still require the caller's ordinary module and resource access checks.
func (r *NotificationRepository) AgentRunNotificationAccess(ctx context.Context, actor *authorization.Actor, runID string) (standalone, allowed bool, err error) {
	var row struct {
		DockChatID *string
		OwnerID    *string
	}
	err = r.db.WithContext(ctx).Table("agent_runs AS run").
		Select("run.dock_chat_id, chat.user_id AS owner_id").
		Joins("LEFT JOIN dock_chats chat ON chat.id = run.dock_chat_id AND chat.workspace_id = run.workspace_id AND chat.archived_at IS NULL").
		Where("run.workspace_id = ? AND run.id = ?", actor.WorkspaceID, runID).Take(&row).Error
	if err == gorm.ErrRecordNotFound {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	return row.DockChatID == nil, row.OwnerID != nil && *row.OwnerID == actor.UserID, nil
}

// CanReadNotificationResource mirrors the resource's current workspace/team/mailbox scope.
// Stored routing hints never grant access to an existing resource that has since moved.
func (r *NotificationRepository) CanReadNotificationResource(ctx context.Context, actor *authorization.Actor, event model.NotificationEventInput) (bool, error) {
	privileged := actor.Role == model.RoleOwner || actor.Role == model.RoleAdmin
	entityType, entityID := event.EntityType, event.EntityID
	if entityType == "agent_run" {
		if taskID, ok := event.Metadata["task_id"].(string); ok && taskID != "" {
			entityType, entityID = "task", taskID
		}
	}
	table := map[string]string{"task": "pm_tasks", "epic": "pm_epics", "sprint": "pm_sprints", "objective": "pm_objectives", "doc": "docs_documents", "document": "docs_documents", "support_conversation": "support_conversations", "crm_signal": "crm_signals", "external_mcp_server": "external_mcp_servers", "agent_run": "agent_runs", "helpcenter_config": "docs_helpcenter_configs"}[entityType]
	if table == "" {
		return false, nil
	}
	q := r.db.WithContext(ctx).Table(table).Where(table+".workspace_id = ? AND "+table+".id = ?", event.WorkspaceID, entityID)
	switch entityType {
	case "task", "epic", "sprint":
		if !privileged {
			q = q.Where("team_id IN ?", actor.TeamIDs())
		}
	case "objective":
		if !privileged {
			q = q.Where("EXISTS (SELECT 1 FROM pm_objective_teams ot WHERE ot.objective_id = pm_objectives.id AND ot.team_id IN ?)", actor.TeamIDs())
		}
	case "doc", "document":
		q = q.Joins("JOIN docs_spaces ds ON ds.id = docs_documents.space_id AND ds.workspace_id = docs_documents.workspace_id").Where("ds.deleted_at IS NULL AND docs_documents.deleted_at IS NULL")
		if !privileged {
			q = q.Where("ds.visibility = ? OR (ds.visibility = ? AND EXISTS (SELECT 1 FROM docs_space_teams dst WHERE dst.space_id = ds.id AND dst.team_id IN ?))", model.SpaceVisibilityWorkspaceWide, model.SpaceVisibilityTeamOnly, actor.TeamIDs())
		}
	case "support_conversation":
		var row struct{ MailboxID *string }
		if err := q.Select("mailbox_id").Take(&row).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return false, nil
			}
			return false, err
		}
		if privileged || row.MailboxID == nil || strings.TrimSpace(*row.MailboxID) == "" {
			return true, nil
		}
		return NewSupportMailboxRepository(r.db).IsMember(ctx, *row.MailboxID, actor.WorkspaceMemberID)
	}
	var count int64
	err := q.Count(&count).Error
	if err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	// Deleted PM items have no row to inspect; their original team remains the only scope.
	if strings.HasSuffix(event.EventType, ".deleted") && (entityType == "epic" || entityType == "objective") {
		var existing int64
		if err := r.db.WithContext(ctx).Table(table).Where("workspace_id = ? AND id = ?", event.WorkspaceID, entityID).Count(&existing).Error; err != nil {
			return false, err
		}
		return existing == 0 && (privileged || (event.TeamID != "" && actor.IsMemberOfTeam(event.TeamID))), nil
	}
	return false, nil
}
