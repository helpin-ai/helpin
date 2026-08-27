-- The visibility backfill runs before application startup, so it cannot rely
-- on GORM AutoMigrate having added this model column first.
ALTER TABLE IF EXISTS dock_chats
    ADD COLUMN IF NOT EXISTS support_conversation_id UUID;
