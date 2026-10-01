-- Shared Community: private conversational flow drafts; no hosted dependencies.
ALTER TABLE dock_chats ADD COLUMN IF NOT EXISTS flow_builder JSONB;
