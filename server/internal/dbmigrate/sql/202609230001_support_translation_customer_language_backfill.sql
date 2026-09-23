-- Existing rows can predate the required customer_language constraint.
UPDATE support_translation_conversations
SET customer_language = ''
WHERE customer_language IS NULL;
