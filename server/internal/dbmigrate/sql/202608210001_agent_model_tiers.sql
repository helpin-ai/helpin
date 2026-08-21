ALTER TABLE IF EXISTS agents
    ADD COLUMN IF NOT EXISTS model_tier TEXT NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS workspace_agent_preset_versions
    ADD COLUMN IF NOT EXISTS model_tier TEXT NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS agent_versions
    ADD COLUMN IF NOT EXISTS model_tier TEXT NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS agent_runs
    ADD COLUMN IF NOT EXISTS model_tier TEXT NOT NULL DEFAULT '';

UPDATE agents
SET model_tier = CASE
    WHEN lower(COALESCE(model, '')) LIKE '%sonnet-5%' OR lower(COALESCE(model, '')) LIKE '%opus%' THEN 'flagship'
    WHEN lower(COALESCE(model, '')) LIKE '%terra%' OR lower(COALESCE(model, '')) LIKE '%sonnet-4%' OR lower(COALESCE(model, '')) LIKE '%haiku-4%' THEN 'large'
    WHEN lower(COALESCE(model, '')) LIKE '%gemini%' OR lower(COALESCE(model, '')) LIKE '%gpt-5-mini%' THEN 'medium'
    WHEN lower(COALESCE(model, '')) LIKE '%deepseek%' OR lower(COALESCE(model, '')) LIKE '%luna%' THEN 'small'
    ELSE model_tier
END
WHERE model_tier = '';

UPDATE workspace_agent_preset_versions
SET model_tier = CASE
    WHEN lower(COALESCE(model, '')) LIKE '%sonnet-5%' OR lower(COALESCE(model, '')) LIKE '%opus%' THEN 'flagship'
    WHEN lower(COALESCE(model, '')) LIKE '%terra%' OR lower(COALESCE(model, '')) LIKE '%sonnet-4%' OR lower(COALESCE(model, '')) LIKE '%haiku-4%' THEN 'large'
    WHEN lower(COALESCE(model, '')) LIKE '%gemini%' OR lower(COALESCE(model, '')) LIKE '%gpt-5-mini%' THEN 'medium'
    WHEN lower(COALESCE(model, '')) LIKE '%deepseek%' OR lower(COALESCE(model, '')) LIKE '%luna%' THEN 'small'
    ELSE model_tier
END
WHERE model_tier = '';

UPDATE agent_versions
SET model_tier = CASE
    WHEN lower(COALESCE(model, '')) LIKE '%sonnet-5%' OR lower(COALESCE(model, '')) LIKE '%opus%' THEN 'flagship'
    WHEN lower(COALESCE(model, '')) LIKE '%terra%' OR lower(COALESCE(model, '')) LIKE '%sonnet-4%' OR lower(COALESCE(model, '')) LIKE '%haiku-4%' THEN 'large'
    WHEN lower(COALESCE(model, '')) LIKE '%gemini%' OR lower(COALESCE(model, '')) LIKE '%gpt-5-mini%' THEN 'medium'
    WHEN lower(COALESCE(model, '')) LIKE '%deepseek%' OR lower(COALESCE(model, '')) LIKE '%luna%' THEN 'small'
    ELSE model_tier
END
WHERE model_tier = '';

UPDATE agent_versions AS version
SET model_tier = agent.model_tier
FROM agents AS agent
WHERE version.agent_id = agent.id
  AND version.model_tier = ''
  AND agent.model_tier <> '';

UPDATE workspace_agent_preset_versions AS version
SET model_tier = agent.model_tier
FROM agents AS agent
WHERE version.workspace_id = agent.workspace_id
  AND version.family_key = agent.preset_key
  AND version.version_key = agent.preset_version_key
  AND version.model_tier = ''
  AND agent.model_tier <> '';

UPDATE agent_runs AS run
SET model_tier = version.model_tier
FROM agent_versions AS version
WHERE run.agent_version_id = version.id
  AND run.model_tier = ''
  AND version.model_tier <> '';

UPDATE agent_runs AS run
SET model_tier = agent.model_tier
FROM agents AS agent
WHERE run.agent_id = agent.id
  AND run.model_tier = ''
  AND agent.model_tier <> '';
