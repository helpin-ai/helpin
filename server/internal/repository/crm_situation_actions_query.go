package repository

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// Canonical suggestions remain authoritative; no copied approval/execution rows.
const situationActionJoin = `LEFT JOIN (
    SELECT ref.workspace_id, ref.situation_id,
        SUM(CASE WHEN a.status = 'pending' THEN 1 ELSE 0 END) AS pending,
        SUM(CASE WHEN a.status = 'accepted' AND a.execution_status = 'failed' THEN 1 ELSE 0 END) AS failed,
        SUM(CASE WHEN a.status = 'accepted' AND (a.execution_status IS NULL OR a.execution_status IN ('','pending') OR
            (a.execution_status = 'in_progress' AND (a.updated_at IS NULL OR a.updated_at < CURRENT_TIMESTAMP - INTERVAL '5 minutes'))) THEN 1 ELSE 0 END) AS uncertain,
        SUM(CASE WHEN a.status = 'accepted' AND (a.execution_status = 'manual_required' OR
            (a.suggestion_type NOT IN ('deal_create','deal_advance','playbook_action') AND a.execution_status = 'succeeded')) THEN 1 ELSE 0 END) AS manual,
        SUM(CASE WHEN a.status = 'accepted' AND (a.execution_status IS NULL OR a.execution_status IN ('','pending','in_progress')) THEN 1 ELSE 0 END) AS executing
    FROM crm_situation_references ref JOIN crm_suggestions a
        ON ref.kind = 'suggestion' AND a.id = ref.source_id AND a.workspace_id = ref.workspace_id
    WHERE ref.workspace_id = ?
    GROUP BY ref.workspace_id, ref.situation_id
) action_counts ON action_counts.workspace_id = s.workspace_id AND action_counts.situation_id = s.id`

// Membership can change without a Signal revision. Live ownership, not a prior
// checkpoint receipt, determines whether a commitment needs to be reassigned.
const situationMissingCheckpointOwnerSQL = `(s.lifecycle = 'open' AND s.next_checkpoint_at IS NOT NULL AND
    ((s.next_action_owner_member_id IS NOT NULL AND COALESCE(next_owner.status, '') <> 'active') OR
     (s.next_action_owner_member_id IS NULL AND COALESCE(owner.status, '') <> 'active')))`

const situationAttentionSQL = `CASE
    WHEN playbook_execution.enabled = true AND (playbook_execution.escalated_at IS NOT NULL OR COALESCE(playbook_execution.blocker,'') NOT IN ('','awaiting_approval','awaiting_work','awaiting_result','daily_run_limit')) THEN 'automation_failed'
    WHEN checkpoint_event.status = 'failed' OR COALESCE(action_counts.failed,0) + COALESCE(action_counts.uncertain,0) > 0 THEN 'automation_failed'
    WHEN ` + situationMissingCheckpointOwnerSQL + ` THEN 'needs_context'
    WHEN COALESCE(action_counts.pending,0) > 0 THEN 'needs_approval'
    WHEN COALESCE(action_counts.manual,0) > 0 THEN 'needs_context'
    WHEN COALESCE(action_counts.executing,0) > 0 THEN 'waiting_work'
    WHEN s.next_checkpoint_at IS NOT NULL AND s.next_checkpoint_at <= CURRENT_TIMESTAMP THEN 'follow_up_due'
    WHEN playbook_execution.enabled = true AND playbook_execution.blocker = 'awaiting_work' THEN 'waiting_work'
    ELSE s.attention END`

const situationOpenActionSQL = `(a.status = 'pending' OR (a.status = 'accepted' AND
    (a.execution_status IS NULL OR a.execution_status IN ('','pending','in_progress','failed','manual_required') OR
    (a.suggestion_type NOT IN ('deal_create','deal_advance','playbook_action') AND a.execution_status = 'succeeded'))))`

const situationActionExistsSQL = `EXISTS (SELECT 1 FROM crm_situation_references ref
    JOIN crm_suggestions a ON ref.kind = 'suggestion' AND a.id = ref.source_id AND a.workspace_id = ref.workspace_id
    LEFT JOIN workspace_members am ON am.workspace_id = a.workspace_id AND am.user_id = a.user_id AND am.status = 'active'
    WHERE ref.workspace_id = s.workspace_id AND ref.situation_id = s.id AND ` + situationOpenActionSQL + ` AND %s)`

func situationCountsJoin(db *gorm.DB) string {
	if db.Dialector.Name() == "sqlite" {
		return strings.ReplaceAll(situationActionJoin, "CURRENT_TIMESTAMP - INTERVAL '5 minutes'", "datetime('now', '-5 minutes')")
	}
	return situationActionJoin
}

func situationPersonalAttentionSQL() string {
	return fmt.Sprintf(situationActionExistsSQL, `am.id = ? OR
		(a.user_id IS NULL AND COALESCE(CASE WHEN next_owner.status = 'active' THEN next_owner.id END, owner.id) = ?) OR
		(a.user_id IS NOT NULL AND am.id IS NULL AND s.owner_member_id = ?)`) +
		` OR ((COALESCE(checkpoint_event.status, '') = 'failed' OR NOT ` + fmt.Sprintf(situationActionExistsSQL, "TRUE") +
		`) AND COALESCE(CASE WHEN next_owner.status = 'active' THEN next_owner.id END, owner.id) = ?)`
}
