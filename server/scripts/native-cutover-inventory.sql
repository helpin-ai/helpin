-- Run on the Helpin database BEFORE the native-only migration.
BEGIN READ ONLY;
SET LOCAL statement_timeout = '30s';
SELECT r.runtime_kind, coalesce(a.preset_key,'custom_or_deleted') AS preset_key,
       r.status, count(*) AS runs_30d
FROM agent_runs r LEFT JOIN agents a ON a.id=r.agent_id
WHERE r.created_at >= now() - interval '30 days'
GROUP BY 1,2,3 ORDER BY 1,2,3;

SELECT r.id,r.workspace_id,r.agent_id,r.runtime_kind,r.status,r.pause_reason,
       r.approval_state,r.workflow_id,r.workflow_run_id,r.external_runtime_id,
       r.task_queue,r.created_at
FROM agent_runs r WHERE r.runtime_kind IN ('codex','opencode')
AND r.status NOT IN ('completed','failed','cancelled') ORDER BY r.created_at,r.id;

SELECT id,workspace_id,preset_key,runtime_kind,trigger_mode
FROM agents WHERE runtime_kind IN ('codex','opencode') ORDER BY workspace_id,id;

-- Rows indicate stored connections, not verified unexpired subscriptions.
SELECT count(*) AS stored_codex_auth_rows,
       count(DISTINCT workspace_id) AS connected_workspaces
FROM codex_workspace_auths;
SELECT c.id,c.workspace_id,c.provider,c.auth_mode,c.created_at,c.updated_at,
       (SELECT max(r.created_at) FROM agent_runs r
        WHERE r.workspace_id=c.workspace_id AND r.runtime_kind='codex') AS latest_codex_run_at,
       (SELECT count(*) FROM agent_runs r WHERE r.workspace_id=c.workspace_id
        AND r.runtime_kind='codex' AND r.created_at>=now()-interval '30 days') AS codex_runs_30d
FROM codex_workspace_auths c ORDER BY c.workspace_id,c.id;
-- Inspect trigger wiring separately; omit action payloads, which may hold inputs.
SELECT ar.id,ar.workspace_id,ar.enabled,ar.trigger_type,ar.action_type,
       ar.trigger_config->>'schedule' AS cron_schedule,
       ar.action_config->>'agent_id' AS agent_id,
       ar.action_config->>'preset_key' AS preset_key,
       ar.action_config->>'agent_version_id' AS agent_version_id
FROM automation_rules ar ORDER BY ar.workspace_id,ar.id;
SELECT id,workspace_id,agent_id,version_key,runtime_kind,deleted_at
FROM agent_versions WHERE runtime_kind IN ('codex','opencode') ORDER BY workspace_id,agent_id,id;
SELECT id,workspace_id,family_key,version_key,runtime_kind,deleted_at
FROM workspace_agent_preset_versions WHERE runtime_kind IN ('codex','opencode') ORDER BY workspace_id,id;
-- A declaration is not proof of compatibility. Review imported skill packages
-- referenced by migrated custom agents; publish a native version if necessary.
SELECT id,workspace_id,key,version_key,source_kind,supported_runtimes
FROM workspace_skills WHERE supported_runtimes ?| ARRAY['codex','opencode']
ORDER BY workspace_id,id;
COMMIT;
