ALTER TABLE support_inbox_views
ADD COLUMN IF NOT EXISTS view_type TEXT NOT NULL DEFAULT 'custom';

ALTER TABLE support_inbox_views
ADD COLUMN IF NOT EXISTS view_key TEXT;

UPDATE support_inbox_views
SET view_type = 'custom'
WHERE view_type IS NULL OR view_type = '';

CREATE INDEX IF NOT EXISTS idx_support_inbox_views_type_key
ON support_inbox_views (workspace_id, created_by, view_type, view_key);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_inbox_views_builtin_unique
ON support_inbox_views (workspace_id, created_by, view_type, view_key)
WHERE view_type IN ('default', 'team') AND view_key IS NOT NULL;
