-- Current defaults and historical version lookups use the profile reference.
CREATE INDEX IF NOT EXISTS idx_agents_ai_profile_id ON agents (ai_profile_id) WHERE ai_profile_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_agent_versions_ai_profile_id ON agent_versions (ai_profile_id) WHERE ai_profile_id IS NOT NULL;
