ALTER TABLE agent_trigger_executions
  ADD COLUMN IF NOT EXISTS actor_id uuid;

CREATE INDEX IF NOT EXISTS idx_agent_trigger_executions_actor_id
  ON agent_trigger_executions(actor_id);
