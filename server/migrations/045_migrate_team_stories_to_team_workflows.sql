-- Migrate stories from workspace-level default workflows to team-scoped workflows.
-- Handles stories with NULL team_id by assigning them to the team when there is
-- exactly one team in the workspace (to avoid ambiguity in multi-team workspaces).

WITH team_workflows AS (
    SELECT w.id AS new_workflow_id, w.team_id, w.workspace_id, w.default_state_id
    FROM pm_workflows w
    WHERE w.team_id IS NOT NULL
),
-- Count teams per workspace to decide whether to migrate NULL-team stories
workspace_team_counts AS (
    SELECT workspace_id, COUNT(*) AS team_count
    FROM team_workflows
    GROUP BY workspace_id
),
-- Find the workspace default workflow (no team_id) for each workspace
default_workflows AS (
    SELECT w.id AS old_workflow_id, w.workspace_id
    FROM pm_workflows w
    WHERE w.team_id IS NULL
      AND w.workspace_id IN (SELECT workspace_id FROM team_workflows)
),
-- Stories on the default workflow that either:
--   1. Already belong to the team, OR
--   2. Have NULL team_id AND there's exactly 1 team in the workspace
orphaned_stories AS (
    SELECT s.id AS story_id, s.workflow_state_id AS old_state_id,
           dw.old_workflow_id,
           tw.new_workflow_id, tw.team_id AS target_team_id,
           tw.default_state_id AS fallback_state_id
    FROM pm_stories s
    JOIN default_workflows dw ON dw.old_workflow_id = s.workflow_id
    JOIN team_workflows tw ON tw.workspace_id = dw.workspace_id
    JOIN workspace_team_counts wtc ON wtc.workspace_id = dw.workspace_id
    WHERE s.archived = false
      AND (
          s.team_id = tw.team_id
          OR (s.team_id IS NULL AND wtc.team_count = 1)
      )
),
old_states AS (
    SELECT ws.id, ws.state_type, ws.position
    FROM pm_workflow_states ws
    WHERE ws.id IN (SELECT DISTINCT old_state_id FROM orphaned_stories)
),
new_states AS (
    SELECT ws.id, ws.workflow_id, ws.state_type, ws.position
    FROM pm_workflow_states ws
    WHERE ws.workflow_id IN (SELECT DISTINCT new_workflow_id FROM team_workflows)
),
state_mapping AS (
    SELECT os.story_id, os.new_workflow_id, os.target_team_id,
        COALESCE(
            (SELECT ns.id FROM new_states ns
             JOIN old_states olds ON olds.id = os.old_state_id
             WHERE ns.workflow_id = os.new_workflow_id
               AND ns.state_type = olds.state_type
               AND ns.position = olds.position
             LIMIT 1),
            (SELECT ns.id FROM new_states ns
             JOIN old_states olds ON olds.id = os.old_state_id
             WHERE ns.workflow_id = os.new_workflow_id
               AND ns.state_type = olds.state_type
             ORDER BY ns.position
             LIMIT 1),
            os.fallback_state_id,
            (SELECT ns.id FROM new_states ns
             WHERE ns.workflow_id = os.new_workflow_id
             ORDER BY ns.position LIMIT 1)
        ) AS new_state_id
    FROM orphaned_stories os
)
UPDATE pm_stories s
SET workflow_id = sm.new_workflow_id,
    workflow_state_id = sm.new_state_id,
    team_id = sm.target_team_id,
    updated_at = NOW()
FROM state_mapping sm
WHERE s.id = sm.story_id
  AND sm.new_state_id IS NOT NULL;
