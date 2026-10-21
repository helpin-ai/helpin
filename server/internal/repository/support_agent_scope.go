package repository

import (
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"gorm.io/gorm"
)

func applySupportAgentMailboxAccess(query *gorm.DB, alias string, scope *authorization.SupportAgentScope) *gorm.DB {
	query = query.Where(alias+".workspace_id = ?", scope.WorkspaceID)
	if len(scope.TeamIDs) > 0 {
		query = query.Where(alias+".active = true AND "+alias+".linked_team_id IN ?", scope.TeamIDs)
	}
	return query
}

func applySupportAgentConversationAccess(query *gorm.DB, alias string, scope *authorization.SupportAgentScope) *gorm.DB {
	query = query.Where(alias+".workspace_id = ?", scope.WorkspaceID)
	if len(scope.TeamIDs) > 0 {
		query = query.Where(fmt.Sprintf(`(%s.mailbox_id IS NULL OR %s.mailbox_id IN (SELECT agent_mailbox.id FROM support_mailboxes agent_mailbox WHERE agent_mailbox.workspace_id = ? AND agent_mailbox.active = true AND agent_mailbox.linked_team_id IN ?))`, alias, alias), scope.WorkspaceID, scope.TeamIDs)
	}
	return query
}
