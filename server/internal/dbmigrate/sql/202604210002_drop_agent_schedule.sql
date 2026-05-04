-- Migration: drop_agent_schedule
-- Backfill any remaining legacy agent-owned schedules into automation_rules,
-- then remove the obsolete agents.schedule column.

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = current_schema()
      AND table_name = 'agents'
      AND column_name = 'schedule'
  ) THEN
    EXECUTE $migration$
      INSERT INTO automation_rules (
        id,
        workspace_id,
        name,
        description,
        enabled,
        trigger_type,
        trigger_config,
        action_type,
        action_config,
        position,
        stop_on_match,
        created_at,
        updated_at
      )
      SELECT
        gen_random_uuid(),
        a.workspace_id,
        CASE
          WHEN NULLIF(BTRIM(a.name), '') IS NULL THEN 'Legacy agent schedule'
          ELSE 'Run ' || BTRIM(a.name) || ' on schedule'
        END,
        'Migrated from a legacy agent-owned recurring schedule. Manage this trigger from Flows.',
        TRUE,
        'cron',
        jsonb_build_object('schedule', BTRIM(a.schedule)),
        'start_agent_run',
        jsonb_build_object(
          'agent_id', a.id,
          'target_type', 'workspace',
          'target_id', a.workspace_id,
          'legacy_schedule_agent_id', a.id
        ),
        0,
        FALSE,
        NOW(),
        NOW()
      FROM agents a
      WHERE a.schedule IS NOT NULL
        AND BTRIM(a.schedule) <> ''
        AND NOT EXISTS (
          SELECT 1
          FROM automation_rules r
          WHERE r.workspace_id = a.workspace_id
            AND r.action_type = 'start_agent_run'
            AND COALESCE(r.action_config->>'legacy_schedule_agent_id', '') = a.id::text
        );
    $migration$;
  END IF;
END $$;

ALTER TABLE IF EXISTS agents
  DROP COLUMN IF EXISTS schedule;
