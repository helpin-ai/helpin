-- Archiving is an explicit user action. Keep archived support chats linked to
-- their source conversation for history, while allowing a fresh active chat
-- to be created when the user starts another Ask thread from that conversation.
DROP INDEX IF EXISTS idx_dock_chats_support_conversation;

CREATE UNIQUE INDEX idx_dock_chats_support_conversation
    ON dock_chats (workspace_id, user_id, support_conversation_id)
    WHERE archived_at IS NULL;
