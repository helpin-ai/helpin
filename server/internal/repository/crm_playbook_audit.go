package repository

import (
	"context"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func (r *CRMPlaybookExecutionRepository) AgentUsage(ctx context.Context, ws, agentID string) ([]model.CRMPlaybookAgentUsage, error) {
	rows := []model.CRMPlaybookAgentUsage{}
	err := r.db.WithContext(ctx).Table("crm_playbook_connections c").Select("DISTINCT p.id AS playbook_id, p.draft->>'name' AS name").Joins("JOIN crm_playbooks p ON p.workspace_id = c.workspace_id AND p.id = c.playbook_id").Where("c.workspace_id = ? AND c.snapshot->'agent'->>'id' = ?", ws, agentID).Order("name, playbook_id").Scan(&rows).Error
	return rows, err
}

// AutomationActivity does not copy lifecycle or expose prompts, raw output,
// runtime identifiers, packages, or internal errors to CRM-only teammates.
func (r *CRMPlaybookExecutionRepository) AutomationActivity(ctx context.Context, ws, id string, page int) (*model.CRMPlaybookAutomationActivityPage, error) {
	query := `SELECT CAST(c.id AS text) AS id, 'connection' AS kind, 'published' AS status, NULL AS situation_id, '' AS situation_title, c.published_at AS occurred_at FROM crm_playbook_connections c WHERE c.workspace_id = ? AND c.playbook_id = ?
UNION ALL SELECT 'setting:' || receipt.command_key, CASE WHEN receipt.settings IS NULL THEN 'signal_automation' ELSE 'settings' END,
CASE WHEN CAST(COALESCE(receipt.settings->>'enabled', receipt.binding->>'enabled') AS text) IN ('true','1') THEN 'enabled' ELSE 'paused' END,
CASE WHEN receipt.settings IS NULL THEN CAST(s.id AS text) END, COALESCE(s.title, ''), receipt.created_at
FROM crm_playbook_automation_receipts receipt LEFT JOIN crm_situations s ON s.workspace_id = receipt.workspace_id AND s.id = receipt.subject_id
WHERE receipt.workspace_id = ? AND (receipt.subject_id = ? OR s.playbook_id = ?)
UNION ALL SELECT CAST(run.id AS text), 'check', run.status, CAST(s.id AS text), s.title, run.updated_at
FROM automation_run_bindings binding JOIN agent_runs run ON run.workspace_id = binding.workspace_id AND run.id = binding.run_id
JOIN crm_situations s ON s.workspace_id = binding.workspace_id AND s.id = binding.situation_id
WHERE binding.workspace_id = ? AND s.playbook_id = ?`
	args := []interface{}{ws, id, ws, id, id, ws, id}
	result := &model.CRMPlaybookAutomationActivityPage{Data: []model.CRMPlaybookAutomationActivity{}, Page: page}
	if err := r.db.WithContext(ctx).Raw("SELECT COUNT(*) FROM ("+query+") activity", args...).Scan(&result.Total).Error; err != nil {
		return nil, err
	}
	args = append(args, 25, (page-1)*25)
	err := r.db.WithContext(ctx).Raw("SELECT * FROM ("+query+") activity ORDER BY occurred_at DESC, id DESC LIMIT ? OFFSET ?", args...).Scan(&result.Data).Error
	return result, err
}
