ALTER TABLE crm_meeting_intelligence
    ADD COLUMN IF NOT EXISTS participants_context JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE crm_meeting_intelligence
    ADD COLUMN IF NOT EXISTS open_questions JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE crm_meeting_intelligence
    ADD COLUMN IF NOT EXISTS rapport JSONB NOT NULL DEFAULT '[]'::jsonb;
