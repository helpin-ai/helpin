ALTER TABLE agent_templates
  ADD COLUMN IF NOT EXISTS required_context jsonb NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS starter_flows jsonb NOT NULL DEFAULT '[]'::jsonb;
