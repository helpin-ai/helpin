-- Reference-only SQL.
-- The app boot path uses GORM AutoMigrate plus explicit Go schema helpers.
-- This file documents the net schema delta from main -> the simplified
-- preset-based agent runtime. It replaces the abandoned branch-local
-- reference files 051-071.

-- Agents: remove class/kind-era fields and keep the preset-based runtime shape.
ALTER TABLE agents
  ADD COLUMN IF NOT EXISTS is_system boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS preset_key text,
  ADD COLUMN IF NOT EXISTS default_invocation_mode text NOT NULL DEFAULT 'autonomous';

UPDATE agents
SET approval_mode = 'preset_default'
WHERE approval_mode = 'class_default';

ALTER TABLE agents
  ALTER COLUMN approval_mode SET DEFAULT 'preset_default';

ALTER TABLE agents
  DROP COLUMN IF EXISTS agent_kind,
  DROP COLUMN IF EXISTS user_id,
  DROP COLUMN IF EXISTS tools,
  DROP COLUMN IF EXISTS target_selector,
  DROP COLUMN IF EXISTS trigger_events,
  DROP COLUMN IF EXISTS agent_class,
  DROP COLUMN IF EXISTS capability_profile;

-- Agent runs are now direct runs only; flow/node links are gone.
ALTER TABLE agent_runs
  ADD COLUMN IF NOT EXISTS invocation_mode text NOT NULL DEFAULT 'autonomous';

ALTER TABLE agent_runs
  DROP COLUMN IF EXISTS flow_run_id,
  DROP COLUMN IF EXISTS flow_node_run_id;

-- Persisted run transcript/messages for interactive and autonomous runs.
CREATE TABLE IF NOT EXISTS agent_run_messages (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL,
  run_id uuid NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
  role text NOT NULL,
  content text NOT NULL,
  message_type text NOT NULL DEFAULT 'message',
  content_blocks jsonb,
  tool_invocations jsonb,
  token_usage jsonb,
  sequence_no integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agent_run_messages_run_sequence
  ON agent_run_messages (workspace_id, run_id, sequence_no);

CREATE INDEX IF NOT EXISTS idx_agent_run_messages_run_created
  ON agent_run_messages (workspace_id, run_id, created_at);

-- Epics no longer pin planner affinity or old flow/session pointers.
ALTER TABLE pm_epics
  DROP COLUMN IF EXISTS orchestrator_agent_id,
  DROP COLUMN IF EXISTS active_planning_session_id,
  DROP COLUMN IF EXISTS active_flow_run_id;

-- Retired flow / planning-session runtime tables.
DROP TABLE IF EXISTS flow_template_nodes CASCADE;
DROP TABLE IF EXISTS flow_templates CASCADE;
DROP TABLE IF EXISTS flow_node_runs CASCADE;
DROP TABLE IF EXISTS flow_runs CASCADE;
DROP TABLE IF EXISTS flow_triggers CASCADE;
DROP TABLE IF EXISTS planning_session_messages CASCADE;
DROP TABLE IF EXISTS planning_sessions CASCADE;
