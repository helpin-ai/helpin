-- Dock chats: table, agent_runs.dock_chat_id column, and FK constraints.
-- GORM AutoMigrate also knows these structs (model.DockChat,
-- model.AgentRun.DockChatID); everything here is idempotent so it is safe
-- regardless of whether AutoMigrate or this migration runs first.

CREATE TABLE IF NOT EXISTS dock_chats (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    user_id uuid NOT NULL,
    title text,
    active_run_id uuid,
    last_message_at timestamptz,
    archived_at timestamptz,
    created_at timestamptz DEFAULT now(),
    updated_at timestamptz DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_dock_chats_ws_user ON dock_chats (workspace_id, user_id);
CREATE INDEX IF NOT EXISTS idx_dock_chats_active_run_id ON dock_chats (active_run_id);

ALTER TABLE agent_runs ADD COLUMN IF NOT EXISTS dock_chat_id uuid;
CREATE INDEX IF NOT EXISTS idx_agent_runs_dock_chat_id ON agent_runs (dock_chat_id);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_dock_chats_active_run'
    ) THEN
        ALTER TABLE dock_chats
            ADD CONSTRAINT fk_dock_chats_active_run
            FOREIGN KEY (active_run_id) REFERENCES agent_runs (id)
            ON DELETE SET NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_agent_runs_dock_chat'
    ) THEN
        ALTER TABLE agent_runs
            ADD CONSTRAINT fk_agent_runs_dock_chat
            FOREIGN KEY (dock_chat_id) REFERENCES dock_chats (id)
            ON DELETE SET NULL;
    END IF;
END $$;
