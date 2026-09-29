-- Saved defaults and completion triggers must follow the same team/state
-- mapping as tasks. Preserve schedules, counters and unrelated JSON fields.
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
), saved_templates AS (
    UPDATE pm_task_templates template
    SET workflow_state_id = mapping.new_state_id, updated_at = CURRENT_TIMESTAMP
    FROM state_mapping mapping
    WHERE template.workspace_id = mapping.workspace_id
      AND template.team_id = mapping.team_id
      AND template.workflow_state_id = mapping.old_state_id
    RETURNING template.id
), recurring_changes AS (
    SELECT template.id,
           CASE WHEN seed.new_state_id IS NULL THEN template.seed_payload
                ELSE template.seed_payload || jsonb_build_object(
                    'workflow_id', seed.new_workflow_id,
                    'workflow_state_id', seed.new_state_id)
           END AS seed_payload,
           CASE WHEN triggers.mapped_count = 0 THEN template.config
                ELSE jsonb_set(template.config, '{completion_state_ids}', triggers.state_ids)
           END AS config
    FROM pm_recurring_templates template
    LEFT JOIN state_mapping seed
      ON seed.workspace_id = template.workspace_id AND seed.team_id = template.team_id
     AND seed.old_workflow_id::text = template.seed_payload->>'workflow_id'
     AND seed.old_state_id::text = template.seed_payload->>'workflow_state_id'
    CROSS JOIN LATERAL (
        SELECT jsonb_agg(COALESCE(mapping.new_state_id::text, original.state_id) ORDER BY original.position) AS state_ids,
               count(mapping.new_state_id) AS mapped_count
        FROM jsonb_array_elements_text(
            CASE WHEN jsonb_typeof(template.config->'completion_state_ids') = 'array'
                 THEN template.config->'completion_state_ids' ELSE '[]'::jsonb END
        ) WITH ORDINALITY original(state_id, position)
        LEFT JOIN state_mapping mapping
          ON mapping.workspace_id = template.workspace_id AND mapping.team_id = template.team_id
         AND mapping.old_state_id::text = original.state_id
    ) triggers
)
UPDATE pm_recurring_templates template
SET seed_payload = changes.seed_payload, config = changes.config, updated_at = CURRENT_TIMESTAMP
FROM recurring_changes changes
WHERE template.id = changes.id
  AND (template.seed_payload, template.config) IS DISTINCT FROM (changes.seed_payload, changes.config);
