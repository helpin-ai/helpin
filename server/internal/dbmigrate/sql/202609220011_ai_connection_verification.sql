-- Track which connections were configured from deployment environment keys so
-- startup can rotate them, and record the outcome of explicit connection tests.
-- Existing rows keep a NULL source: their origin cannot be proven, so startup
-- never overwrites them with an environment key.
ALTER TABLE ai_connections ADD COLUMN IF NOT EXISTS credential_source text;
ALTER TABLE ai_connections ADD COLUMN IF NOT EXISTS last_verified_at timestamptz;
ALTER TABLE ai_connections ADD COLUMN IF NOT EXISTS last_verification_error text;
