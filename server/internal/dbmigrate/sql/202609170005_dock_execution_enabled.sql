ALTER TABLE dock_chats ADD COLUMN IF NOT EXISTS execution_enabled boolean NOT NULL DEFAULT false;
