-- Migration: native_only_agents
-- Release gate: run both inventories, dispose of old Temporal workflows, and
-- obtain API-key/coding-worker readiness before applying this migration.
DO $$
BEGIN
    IF to_regclass('agent_runs') IS NOT NULL AND EXISTS (
        SELECT 1 FROM agent_runs WHERE runtime_kind IN ('codex','opencode')
        AND status NOT IN ('completed','failed','cancelled')
    ) THEN
        RAISE EXCEPTION 'Native cutover blocked: drain or explicitly fail every retired-engine run and its workflow first';
    END IF;
END $$;

-- Preserve historical versions and run pins. Clone the selected custom version
-- instead of changing the meaning of a version used by an existing run.
INSERT INTO agent_versions
SELECT (jsonb_populate_record(NULL::agent_versions, to_jsonb(v) || jsonb_build_object(
    'id', gen_random_uuid(), 'version_key', 'native-' || v.id::text,
    'label', v.label || ' (Native)', 'runtime_kind', 'native_sdk',
    'created_at', now(), 'updated_at', now()
))).*
FROM agents a JOIN agent_versions v ON v.agent_id=a.id
AND (v.id=a.active_version_id OR (a.active_version_id IS NULL AND v.version_key='default'))
WHERE NOT a.is_system AND v.deleted_at IS NULL AND v.runtime_kind IN ('codex','opencode')
AND NOT EXISTS (SELECT 1 FROM agent_versions n WHERE n.agent_id=a.id AND n.version_key='native-' || v.id::text);

UPDATE agents a SET active_version_id=n.id, runtime_kind='native_sdk', updated_at=now()
FROM agent_versions v JOIN agent_versions n ON n.agent_id=v.agent_id AND n.version_key='native-' || v.id::text
WHERE a.id=v.agent_id AND NOT a.is_system
AND (a.active_version_id=v.id OR (a.active_version_id IS NULL AND v.version_key='default'))
AND v.runtime_kind IN ('codex','opencode');

-- Wave one: non-coding presets. Run their representative workflow checks first.
UPDATE agents SET runtime_kind='native_sdk', updated_at=now()
WHERE runtime_kind IN ('codex','opencode') AND preset_key IN ('task_planner','crm_operator','marketer');
-- Wave two: retained coding presets and custom defaults. API admission requires
-- a live dedicated coding queue poller for effective shell/write permissions.
UPDATE agents SET runtime_kind='native_sdk', updated_at=now()
WHERE runtime_kind IN ('codex','opencode');
UPDATE workspace_agent_preset_versions SET runtime_kind='native_sdk', updated_at=now()
WHERE runtime_kind IN ('codex','opencode') AND deleted_at IS NULL;
ALTER TABLE agents ALTER COLUMN runtime_kind SET DEFAULT 'native_sdk';
ALTER TABLE agent_runs ALTER COLUMN runtime_kind SET DEFAULT 'native_sdk';

-- Deliberate disconnect after inventory/credential migration. Generic encrypted
-- credential storage for integrations and MCP remains unchanged.
DROP TABLE IF EXISTS codex_workspace_auths;
