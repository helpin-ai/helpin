ALTER TABLE agent_run_messages
    ADD COLUMN IF NOT EXISTS actor_user_id uuid;

UPDATE agent_run_messages AS message
SET actor_user_id = run.triggered_by_user_id
FROM agent_runs AS run
WHERE message.run_id = run.id
  AND message.workspace_id = run.workspace_id
  AND message.role = 'user'
  AND message.actor_user_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_agent_run_messages_actor_user
    ON agent_run_messages (workspace_id, actor_user_id)
    WHERE actor_user_id IS NOT NULL;
