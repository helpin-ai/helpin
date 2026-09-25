-- Non-fatal policy skips must remain visible without marking a successful sync failed.
ALTER TABLE support_content_sources ADD COLUMN IF NOT EXISTS last_sync_warning text;
