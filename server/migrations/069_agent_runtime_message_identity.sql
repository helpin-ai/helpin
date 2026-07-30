ALTER TABLE agent_run_messages
  ADD COLUMN IF NOT EXISTS runtime_message_id TEXT;

CREATE INDEX IF NOT EXISTS idx_agent_run_messages_runtime_message_id
  ON agent_run_messages (workspace_id, run_id, runtime_message_id)
  WHERE runtime_message_id IS NOT NULL AND runtime_message_id <> '';
