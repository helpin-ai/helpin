ALTER TABLE docs_helpcenter_configs
  ADD COLUMN IF NOT EXISTS chat_widget_enabled BOOLEAN NOT NULL DEFAULT TRUE;

UPDATE docs_helpcenter_configs
SET chat_widget_enabled = TRUE
WHERE chat_widget_enabled IS NULL;
