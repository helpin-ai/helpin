-- Migration: Backfill pm_automations into automation_rules table.
-- This migrates epic and sprint automations from the old pm_automations table
-- to the unified automation_rules table.

-- Epic auto-start: fires on any "started" state type
INSERT INTO automation_rules (workspace_id, name, enabled, trigger_type, trigger_config, action_type, action_config, position)
SELECT
    pa.workspace_id,
    'Epic auto-start',
    pa.enabled,
    'story.state_entered',
    jsonb_build_object('state_type', 'started'),
    'run_command',
    jsonb_build_object('command_name', 'pm.auto_start_epic', 'input', jsonb_build_object('target_state_id', pa.config_state_id)),
    -100
FROM pm_automations pa
WHERE pa.automation_type = 'epic_auto_start'
  AND pa.config_state_id IS NOT NULL
ON CONFLICT DO NOTHING;

-- Epic auto-complete: fires on any "done" state type
INSERT INTO automation_rules (workspace_id, name, enabled, trigger_type, trigger_config, action_type, action_config, position)
SELECT
    pa.workspace_id,
    'Epic auto-complete',
    pa.enabled,
    'story.state_entered',
    jsonb_build_object('state_type', 'done'),
    'run_command',
    jsonb_build_object('command_name', 'pm.auto_complete_epic', 'input', jsonb_build_object('target_state_id', pa.config_state_id)),
    -99
FROM pm_automations pa
WHERE pa.automation_type = 'epic_auto_complete'
  AND pa.config_state_id IS NOT NULL
ON CONFLICT DO NOTHING;

-- Sprint auto-create: cron trigger, team-scoped
INSERT INTO automation_rules (workspace_id, name, enabled, team_id, trigger_type, trigger_config, action_type, action_config, position)
SELECT
    pa.workspace_id,
    'Sprint auto-create',
    pa.enabled,
    pa.team_id,
    'cron',
    jsonb_build_object('category', 'sprint_hourly'),
    'run_command',
    jsonb_build_object('command_name', 'pm.sprint_auto_create', 'input',
        jsonb_build_object('target_count', pa.config_int, 'week_length', pa.config_int2, 'start_day', COALESCE(pa.config_int3, 1))),
    -98
FROM pm_automations pa
WHERE pa.automation_type = 'sprint_auto_create'
  AND pa.team_id IS NOT NULL
ON CONFLICT DO NOTHING;

-- Sprint move-unfinished: cron trigger, team-scoped
INSERT INTO automation_rules (workspace_id, name, enabled, team_id, trigger_type, trigger_config, action_type, action_config, position)
SELECT
    pa.workspace_id,
    'Sprint move unfinished stories',
    pa.enabled,
    pa.team_id,
    'cron',
    jsonb_build_object('category', 'sprint_hourly'),
    'run_command',
    jsonb_build_object('command_name', 'pm.sprint_move_unfinished'),
    -97
FROM pm_automations pa
WHERE pa.automation_type = 'sprint_move_unfinished'
  AND pa.team_id IS NOT NULL
ON CONFLICT DO NOTHING;
