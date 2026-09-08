package repository

import (
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

func situationPlaybookExecutionJoin(db *gorm.DB) string {
	if !db.Migrator().HasTable(&model.CRMPlaybookAutomationBinding{}) {
		return `LEFT JOIN (SELECT '' AS blocker, false AS enabled, NULL AS escalated_at, NULL AS escalation_member_id) playbook_execution ON false`
	}
	return `LEFT JOIN (
  SELECT b.workspace_id, b.situation_id, b.blocker, b.enabled, b.escalated_at,
    CASE WHEN b.escalated_at IS NOT NULL THEN CAST(escalation.id AS text) END AS escalation_member_id
  FROM crm_playbook_automation_bindings b
  JOIN crm_playbook_connections connection ON connection.workspace_id = b.workspace_id AND connection.id = b.connection_id
  JOIN crm_playbook_versions policy ON policy.workspace_id = connection.workspace_id AND policy.id = connection.playbook_version_id
  LEFT JOIN workspace_members escalation ON escalation.workspace_id = b.workspace_id AND CAST(escalation.id AS text) = policy.definition->'responsibilities'->>'escalation_member_id' AND escalation.status = 'active'
 ) playbook_execution ON playbook_execution.workspace_id = s.workspace_id AND playbook_execution.situation_id = s.id`
}
