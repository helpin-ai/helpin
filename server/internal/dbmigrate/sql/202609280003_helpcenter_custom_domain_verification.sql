-- Help center custom domains are served only after the workspace proves it
-- owns them (a TXT token) and DNS points at Helpin. A daily check tracks health.
ALTER TABLE docs_helpcenter_configs ADD COLUMN IF NOT EXISTS custom_domain_status TEXT;
ALTER TABLE docs_helpcenter_configs ADD COLUMN IF NOT EXISTS custom_domain_token TEXT;
ALTER TABLE docs_helpcenter_configs ADD COLUMN IF NOT EXISTS custom_domain_verified_at TIMESTAMPTZ;
ALTER TABLE docs_helpcenter_configs ADD COLUMN IF NOT EXISTS custom_domain_checked_at TIMESTAMPTZ;
ALTER TABLE docs_helpcenter_configs ADD COLUMN IF NOT EXISTS custom_domain_last_error TEXT;
ALTER TABLE docs_helpcenter_configs ADD COLUMN IF NOT EXISTS custom_domain_failing_since TIMESTAMPTZ;
ALTER TABLE docs_helpcenter_configs ADD COLUMN IF NOT EXISTS custom_domain_alerted_status TEXT;

-- Domains already in use keep working: they count as verified, and the daily
-- health check takes over from here.
UPDATE docs_helpcenter_configs
SET custom_domain_status = 'verified', custom_domain_verified_at = now(), custom_domain_alerted_status = 'verified'
WHERE NULLIF(BTRIM(custom_domain), '') IS NOT NULL AND custom_domain_status IS NULL;

UPDATE docs_helpcenter_configs
SET custom_domain_token = encode(gen_random_bytes(16), 'hex')
WHERE custom_domain_token IS NULL;
