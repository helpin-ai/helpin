-- Keep the automatic reply language stable for the conversation. Existing
-- manual choices are preserved; legacy per-message labels are not backfilled.
ALTER TABLE support_translation_conversations
 ADD COLUMN IF NOT EXISTS detected_language text NOT NULL DEFAULT '';
