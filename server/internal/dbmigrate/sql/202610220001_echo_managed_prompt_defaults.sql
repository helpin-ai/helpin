-- Replace shipped Echo snapshots with the current managed generic prompt.
-- Exact fingerprints preserve custom business instructions; runtime policies still apply.
UPDATE agents
SET system_prompt = NULL, instruction_template_version = '', updated_at = NOW()
WHERE is_system = TRUE AND preset_key = 'support_agent'
  AND md5(btrim(system_prompt, E' \n\r\t')) IN (
    'ada05706bea90acef1adf58f002db0bc',
    '9ad52a24bf9bdfb3abccf94f7815c1d1',
    '1120b81959816c1e7fca0357134a6cbb',
    'e04eac8c4e60361a71070c2f816c6329',
    '97046fd449b2bfbc8c15b29f06fc85f0',
    '0ebc5ba8c65ad1925a62c908024820f8',
    '807518f640b76116a19d965d6f5a12d4');

-- Workspace preset copies are another source of the same shipped prompt.
-- Leave their tools, models, and genuine custom instructions unchanged.
UPDATE workspace_agent_preset_versions
SET system_prompt = NULL, instruction_preamble = NULL,
    instruction_skills = '[]'::jsonb, instruction_template_version = '', updated_at = NOW()
WHERE family_key = 'support_agent'
  AND md5(btrim(system_prompt, E' \n\r\t')) IN (
    'ada05706bea90acef1adf58f002db0bc',
    '9ad52a24bf9bdfb3abccf94f7815c1d1',
    '1120b81959816c1e7fca0357134a6cbb',
    'e04eac8c4e60361a71070c2f816c6329',
    '97046fd449b2bfbc8c15b29f06fc85f0',
    '0ebc5ba8c65ad1925a62c908024820f8',
    '807518f640b76116a19d965d6f5a12d4');

-- Unpin only the exact new generic skill saved during the rollout. Customized
-- workspace guidance (including ContentStudio's) remains an override.
UPDATE workspace_skills
SET is_customized = false, updated_at = NOW()
WHERE source_kind = 'built_in' AND key = 'support_triage_response' AND is_customized = true
  AND title = 'Support Triage and Response'
  AND description = 'Live support conversation handling - grounded replies, escalation judgment, and sub-agent usage.'
  AND md5(instructions) = '91fa8a0e916d79b8f3d6e09b4e964e28';
