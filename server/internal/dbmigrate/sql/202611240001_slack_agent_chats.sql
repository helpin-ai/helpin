ALTER TABLE dock_chats ADD COLUMN IF NOT EXISTS slack_thread boolean NOT NULL DEFAULT false;
