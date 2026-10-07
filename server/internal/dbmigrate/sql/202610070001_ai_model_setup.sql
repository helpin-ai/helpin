-- Preserve existing route identities and choices; discovery never rewrites them.
ALTER TABLE ai_profiles ADD COLUMN IF NOT EXISTS hidden_from_ask_agent boolean NOT NULL DEFAULT false;
ALTER TABLE ai_connections ADD COLUMN IF NOT EXISTS discovered_models jsonb;
ALTER TABLE ai_connections ADD COLUMN IF NOT EXISTS models_fetched_at timestamptz;
