BEGIN;

UPDATE agents
SET runtime_kind = 'native_sdk'
WHERE runtime_kind IN ('native_claude', 'claude_code');

UPDATE agent_runs
SET runtime_kind = 'native_sdk'
WHERE runtime_kind IN ('native_claude', 'claude_code');

COMMIT;
