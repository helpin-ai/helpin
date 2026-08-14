-- Give Dock chats one durable message timeline across all backing agent runs.
-- Existing run-local sequence_no remains unchanged for runtime compatibility.

ALTER TABLE dock_chats
    ADD COLUMN IF NOT EXISTS next_message_sequence bigint NOT NULL DEFAULT 0;

ALTER TABLE agent_run_messages
    ADD COLUMN IF NOT EXISTS dock_chat_id uuid,
    ADD COLUMN IF NOT EXISTS dock_chat_sequence bigint,
    ADD COLUMN IF NOT EXISTS client_message_id uuid,
    ADD COLUMN IF NOT EXISTS delivery_status text NOT NULL DEFAULT 'sent';

UPDATE agent_run_messages AS message
SET dock_chat_id = run.dock_chat_id
FROM agent_runs AS run
WHERE message.run_id = run.id
  AND message.dock_chat_id IS NULL
  AND run.dock_chat_id IS NOT NULL;

WITH ordered AS (
    SELECT
        message.id,
        ROW_NUMBER() OVER (
            PARTITION BY message.dock_chat_id
            ORDER BY message.created_at ASC, run.created_at ASC, message.sequence_no ASC, message.id ASC
        ) AS sequence_no
    FROM agent_run_messages AS message
    JOIN agent_runs AS run ON run.id = message.run_id
    WHERE message.dock_chat_id IS NOT NULL
      AND message.dock_chat_sequence IS NULL
)
UPDATE agent_run_messages AS message
SET dock_chat_sequence = ordered.sequence_no
FROM ordered
WHERE message.id = ordered.id;

UPDATE dock_chats AS chat
SET next_message_sequence = latest.sequence_no
FROM (
    SELECT dock_chat_id, COALESCE(MAX(dock_chat_sequence), 0) AS sequence_no
    FROM agent_run_messages
    WHERE dock_chat_id IS NOT NULL
    GROUP BY dock_chat_id
) AS latest
WHERE chat.id = latest.dock_chat_id
  AND chat.next_message_sequence < latest.sequence_no;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_agent_run_messages_dock_chat'
          AND conrelid = 'agent_run_messages'::regclass
    ) THEN
        ALTER TABLE agent_run_messages
            ADD CONSTRAINT fk_agent_run_messages_dock_chat
            FOREIGN KEY (dock_chat_id) REFERENCES dock_chats (id)
            ON DELETE SET NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'agent_run_messages_delivery_status_valid'
          AND conrelid = 'agent_run_messages'::regclass
    ) THEN
        ALTER TABLE agent_run_messages
            ADD CONSTRAINT agent_run_messages_delivery_status_valid
            CHECK (delivery_status IN ('pending', 'sent', 'failed'));
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_run_messages_dock_chat_sequence
    ON agent_run_messages (dock_chat_id, dock_chat_sequence)
    WHERE dock_chat_id IS NOT NULL AND dock_chat_sequence IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_run_messages_client_message_id
    ON agent_run_messages (workspace_id, client_message_id)
    WHERE client_message_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_agent_run_messages_dock_chat_timeline
    ON agent_run_messages (workspace_id, dock_chat_id, dock_chat_sequence DESC)
    WHERE dock_chat_id IS NOT NULL;
