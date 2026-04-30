ALTER TABLE docs_helpcenter_configs
  ADD COLUMN IF NOT EXISTS public_url_mode TEXT NOT NULL DEFAULT 'hosted_subdomain';

ALTER TABLE docs_helpcenter_configs
  ADD COLUMN IF NOT EXISTS reverse_proxy_host TEXT;

ALTER TABLE docs_helpcenter_configs
  ADD COLUMN IF NOT EXISTS reverse_proxy_base_path TEXT;

UPDATE docs_helpcenter_configs
SET public_url_mode = 'custom_domain'
WHERE NULLIF(BTRIM(custom_domain), '') IS NOT NULL
  AND public_url_mode = 'hosted_subdomain';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'chk_docs_helpcenter_configs_public_url_mode'
  ) THEN
    ALTER TABLE docs_helpcenter_configs
      ADD CONSTRAINT chk_docs_helpcenter_configs_public_url_mode
      CHECK (public_url_mode IN ('hosted_subdomain', 'custom_domain', 'reverse_proxy'));
  END IF;
END $$;
