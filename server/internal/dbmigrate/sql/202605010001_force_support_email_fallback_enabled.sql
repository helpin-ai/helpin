-- Ensure offline support email fallback is enabled for every installation.
UPDATE support_installations
SET settings = jsonb_set(
    COALESCE(settings, '{}'::jsonb),
    '{email_fallback_enabled}',
    'true'::jsonb,
    true
)
WHERE COALESCE(settings->>'email_fallback_enabled', 'false') <> 'true';
