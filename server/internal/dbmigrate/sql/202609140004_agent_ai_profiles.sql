-- Historical versions retain their original routing fields. New profile references
-- are nullable so migration does not invent a different execution selection.
ALTER TABLE agents ADD COLUMN IF NOT EXISTS ai_profile_id UUID REFERENCES ai_profiles(id);
ALTER TABLE agent_versions ADD COLUMN IF NOT EXISTS ai_profile_id UUID REFERENCES ai_profiles(id);
