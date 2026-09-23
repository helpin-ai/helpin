-- AutoMigrate may have created these columns before the SQL ledger ran.
-- ADD COLUMN IF NOT EXISTS in the original Live Translate migration then left
-- their older nullability/defaults unchanged.
UPDATE support_translation_conversations
SET customer_language = ''
WHERE customer_language IS NULL;
UPDATE support_translation_conversations
SET revision = 1
WHERE revision IS NULL;
ALTER TABLE support_translation_conversations
    ALTER COLUMN customer_language SET DEFAULT '',
    ALTER COLUMN revision SET DEFAULT 1,
    ALTER COLUMN revision SET NOT NULL;

-- Supply both values explicitly so new conversations remain safe even if a
-- database default is later changed independently of the application.
CREATE OR REPLACE FUNCTION support_initialize_live_translate() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO support_translation_conversations(workspace_id,conversation_id,translation_mode,customer_language,revision)
 SELECT NEW.workspace_id,NEW.id, CASE WHEN COALESCE((settings::jsonb->>'translation_enabled')::boolean,true) THEN 'on' ELSE 'off' END, '', 1
 FROM support_widget_installations WHERE workspace_id=NEW.workspace_id ON CONFLICT DO NOTHING;
 INSERT INTO support_translation_conversations(workspace_id,conversation_id,translation_mode,customer_language,revision)
 VALUES(NEW.workspace_id,NEW.id,'on','',1) ON CONFLICT DO NOTHING;
 RETURN NEW;
END $$;
