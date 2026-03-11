-- 036: Add partial unique index on custom_domain for help center config lookup.
CREATE UNIQUE INDEX IF NOT EXISTS idx_docs_hc_config_custom_domain
ON docs_helpcenter_configs (custom_domain)
WHERE custom_domain IS NOT NULL;
