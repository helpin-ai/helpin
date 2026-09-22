-- Use the portable SQL timestamp expression declared by the API models.
-- CURRENT_TIMESTAMP and now() both represent the transaction start time.
ALTER TABLE support_translations
    ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP,
    ALTER COLUMN updated_at SET DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE support_translation_preferences
    ALTER COLUMN updated_at SET DEFAULT CURRENT_TIMESTAMP,
    -- Preferences are explicit booleans supplied by the application. A GORM
    -- default of true would replace an explicitly supplied false on creation.
    ALTER COLUMN auto_translate_incoming DROP DEFAULT,
    ALTER COLUMN auto_translate_outgoing DROP DEFAULT;
ALTER TABLE support_translation_conversations
    ALTER COLUMN updated_at SET DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE support_inbound_jobs
    ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP,
    ALTER COLUMN updated_at SET DEFAULT CURRENT_TIMESTAMP;
