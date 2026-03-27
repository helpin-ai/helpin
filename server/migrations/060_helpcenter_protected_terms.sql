ALTER TABLE docs_helpcenter_configs
ADD COLUMN IF NOT EXISTS protected_terms text[] DEFAULT '{}';
