ALTER TABLE agent_run_messages
  ADD COLUMN IF NOT EXISTS turn_segments JSONB;
