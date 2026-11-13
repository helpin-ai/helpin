ALTER TABLE ai_message_processing
    ADD COLUMN IF NOT EXISTS action_validation_repair_used boolean NOT NULL DEFAULT false;
