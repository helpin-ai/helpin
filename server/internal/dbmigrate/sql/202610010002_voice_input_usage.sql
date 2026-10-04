ALTER TABLE ai_execution_usage ADD COLUMN IF NOT EXISTS audio_milliseconds bigint NOT NULL DEFAULT 0;
