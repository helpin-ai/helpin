-- Migration: enable_support_email_fallback
-- Turn on offline email fallback for existing support widget installations.

UPDATE support_widget_installations
SET
  settings = jsonb_set(
    COALESCE(settings, '{}'::jsonb),
    '{email_fallback_enabled}',
    'true'::jsonb,
    true
  ),
  updated_at = NOW()
WHERE COALESCE(settings->>'email_fallback_enabled', 'false') <> 'true';
