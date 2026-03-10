-- 033_helpcenter_config_extended.sql
-- Add extended configuration fields to docs_helpcenter_configs:
-- favicon, theme mode, header links, footer, homepage, space nav, search placeholder.

ALTER TABLE docs_helpcenter_configs ADD COLUMN IF NOT EXISTS favicon_url TEXT;
ALTER TABLE docs_helpcenter_configs ADD COLUMN IF NOT EXISTS theme_mode TEXT NOT NULL DEFAULT 'system';
ALTER TABLE docs_helpcenter_configs ADD COLUMN IF NOT EXISTS header_links JSONB NOT NULL DEFAULT '[]';
ALTER TABLE docs_helpcenter_configs ADD COLUMN IF NOT EXISTS footer_config JSONB NOT NULL DEFAULT '{}';
ALTER TABLE docs_helpcenter_configs ADD COLUMN IF NOT EXISTS homepage_config JSONB NOT NULL DEFAULT '{}';
ALTER TABLE docs_helpcenter_configs ADD COLUMN IF NOT EXISTS space_nav_config JSONB NOT NULL DEFAULT '{}';
ALTER TABLE docs_helpcenter_configs ADD COLUMN IF NOT EXISTS search_placeholder TEXT;
