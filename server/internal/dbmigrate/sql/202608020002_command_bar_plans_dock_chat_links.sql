-- Dock chat linkage on command-bar plans: plans launched from a dock chat
-- carry the chat and parent chat-run IDs so settled results can be delivered
-- back into the chat by real FKs (replacing fuzzy prompt/timestamp matching).
-- Columns are also declared on the GORM model; everything here is idempotent.

ALTER TABLE command_bar_plans ADD COLUMN IF NOT EXISTS parent_chat_run_id uuid;
ALTER TABLE command_bar_plans ADD COLUMN IF NOT EXISTS dock_chat_id uuid;
ALTER TABLE command_bar_plans ADD COLUMN IF NOT EXISTS parent_notified_at timestamptz;

CREATE INDEX IF NOT EXISTS idx_command_bar_plans_parent_chat_run_id ON command_bar_plans (parent_chat_run_id);
CREATE INDEX IF NOT EXISTS idx_command_bar_plans_dock_chat_id ON command_bar_plans (dock_chat_id);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_command_bar_plans_parent_chat_run'
    ) THEN
        ALTER TABLE command_bar_plans
            ADD CONSTRAINT fk_command_bar_plans_parent_chat_run
            FOREIGN KEY (parent_chat_run_id) REFERENCES agent_runs (id)
            ON DELETE SET NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_command_bar_plans_dock_chat'
    ) THEN
        ALTER TABLE command_bar_plans
            ADD CONSTRAINT fk_command_bar_plans_dock_chat
            FOREIGN KEY (dock_chat_id) REFERENCES dock_chats (id)
            ON DELETE SET NULL;
    END IF;
END $$;
