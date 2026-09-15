-- A compatible connection freezes the approved destination with its secret.
-- Existing personal/workspace connections retain their IDs and encryption AAD.
ALTER TABLE ai_connections ADD COLUMN IF NOT EXISTS endpoint jsonb;
ALTER TABLE ai_connections ADD CONSTRAINT ai_connections_endpoint_provider CHECK (
 (provider = 'openai_compatible' AND endpoint IS NOT NULL) OR
 (provider <> 'openai_compatible' AND endpoint IS NULL)
);
