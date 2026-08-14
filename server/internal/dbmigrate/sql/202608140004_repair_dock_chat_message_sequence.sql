-- Repair deployments where next_message_sequence existed before the timeline
-- migration and therefore retained a nullable definition or NULL values.

ALTER TABLE dock_chats
    ADD COLUMN IF NOT EXISTS next_message_sequence bigint;

ALTER TABLE dock_chats
    ALTER COLUMN next_message_sequence SET DEFAULT 0;

UPDATE dock_chats AS chat
SET next_message_sequence = GREATEST(
    COALESCE(chat.next_message_sequence, 0),
    COALESCE((
        SELECT MAX(message.dock_chat_sequence)
        FROM agent_run_messages AS message
        WHERE message.dock_chat_id = chat.id
    ), 0)
)
WHERE chat.next_message_sequence IS NULL
   OR chat.next_message_sequence < COALESCE((
        SELECT MAX(message.dock_chat_sequence)
        FROM agent_run_messages AS message
        WHERE message.dock_chat_id = chat.id
    ), 0);

ALTER TABLE dock_chats
    ALTER COLUMN next_message_sequence SET NOT NULL;
