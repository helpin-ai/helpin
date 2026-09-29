-- Repair tasks left in a workspace workflow despite having a team workflow.
-- Match status by type, preferring the same name. Never reopen completed work
-- or move tasks to a different team. Tasks without an equivalent type stay put.
WITH team_workflows AS (
    SELECT DISTINCT ON (workspace_id, team_id) id, workspace_id, team_id
    FROM pm_workflows
    WHERE team_id IS NOT NULL
    ORDER BY workspace_id, team_id, created_at, id
), state_mapping AS (
    SELECT team.workspace_id, team.team_id, source.id AS old_workflow_id,
           old_state.id AS old_state_id, team.id AS new_workflow_id,
           destination.id AS new_state_id
    FROM pm_workflows source
    JOIN team_workflows team ON team.workspace_id = source.workspace_id
    JOIN pm_workflow_states old_state ON old_state.workflow_id = source.id
    CROSS JOIN LATERAL (
        SELECT candidate.id
        FROM pm_workflow_states candidate
        WHERE candidate.workflow_id = team.id
          AND candidate.state_type = old_state.state_type
        ORDER BY (lower(trim(candidate.name)) = lower(trim(old_state.name))) DESC,
                 candidate.position, candidate.id
        LIMIT 1
    ) destination
    WHERE source.team_id IS NULL
)
UPDATE pm_tasks task
SET workflow_id = mapping.new_workflow_id,
    workflow_state_id = mapping.new_state_id,
    updated_at = CURRENT_TIMESTAMP
FROM state_mapping mapping
WHERE task.workspace_id = mapping.workspace_id
  AND task.team_id = mapping.team_id
  AND task.workflow_id = mapping.old_workflow_id
  AND task.workflow_state_id = mapping.old_state_id;
