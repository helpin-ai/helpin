-- Refresh the shipped Medium, Large, and Flagship models (and the provider
-- fallbacks the startup re-mapper produces) to the 2026-11 lineup. Only
-- untouched standard profiles move: same tier, shipped model, default controls,
-- no fallback. Custom selections and accepted run inputs keep their routes.
CREATE TEMP TABLE standard_ai_model_refresh(tier text, provider text, old_model text, new_model text) ON COMMIT DROP;
INSERT INTO standard_ai_model_refresh VALUES
 ('small','openai','gpt-5.6-luna','gpt-6-luna'),
 ('small','anthropic','claude-haiku-4-5','claude-haiku-5-5'),
 ('medium','openrouter','google/gemini-3.8-flash','openai/gpt-6.1-terra'),
 ('medium','openai','gpt-5-mini','gpt-6.1-terra'),
 ('medium','anthropic','claude-haiku-4-5','claude-haiku-5-5'),
 ('large','openai','gpt-5.6-terra','gpt-6.2-sol'),
 ('large','openrouter','openai/gpt-5.6-terra','openai/gpt-6.2-sol'),
 ('large','anthropic','claude-haiku-4-5','claude-sonnet-5-5'),
 ('flagship','anthropic','claude-sonnet-5','claude-opus-5-5'),
 ('flagship','openrouter','anthropic/claude-sonnet-5','openai/gpt-6.2-astra'),
 ('flagship','openai','gpt-5.5','gpt-6.2-astra');

CREATE TEMP TABLE standard_ai_model_refresh_profiles ON COMMIT DROP AS
SELECT p.id,p.workspace_id,u.provider,u.new_model
FROM ai_profiles p JOIN standard_ai_model_refresh u
 ON p.id=md5('helpin-standard-ai-profile|'||p.workspace_id::text||'|'||u.tier)::uuid
WHERE p.scope='workspace' AND p.user_id IS NULL AND p.deleted_at IS NULL AND p.fallback IS NULL
 AND p."primary"->'model'->>'provider'=u.provider
 AND p."primary"->'model'->>'model'=u.old_model
 AND coalesce(p."primary"->'model'->'controls','{}'::jsonb)='{}'::jsonb;

UPDATE ai_profiles p SET "primary"=jsonb_set(p."primary",'{model,model}',to_jsonb(u.new_model)),
 revision=p.revision+1,updated_at=now()
FROM standard_ai_model_refresh_profiles u WHERE p.id=u.id;

UPDATE agents a SET provider=u.provider,model=u.new_model
FROM standard_ai_model_refresh_profiles u WHERE a.workspace_id=u.workspace_id AND a.ai_profile_id=u.id;
UPDATE agent_versions v SET provider=u.provider,model=u.new_model
FROM standard_ai_model_refresh_profiles u WHERE v.workspace_id=u.workspace_id AND v.ai_profile_id=u.id;

-- OpenAI retires gpt-5-mini on 2026-12-11. Move every remaining route,
-- including customized ones, to its named replacement so runs keep working.
CREATE TEMP TABLE retired_gpt_5_mini(provider text, old_model text, new_model text) ON COMMIT DROP;
INSERT INTO retired_gpt_5_mini VALUES
 ('openai','gpt-5-mini','gpt-6.1-terra'),
 ('openai_chatgpt','gpt-5-mini','gpt-6.1-terra'),
 ('openrouter','openai/gpt-5-mini','openai/gpt-6.1-terra'),
 ('openrouter-responses','openai/gpt-5-mini','openai/gpt-6.1-terra');

UPDATE ai_profiles p SET "primary"=jsonb_set(p."primary",'{model,model}',to_jsonb(r.new_model)),
 revision=p.revision+1,updated_at=now()
FROM retired_gpt_5_mini r
WHERE p."primary"->'model'->>'provider'=r.provider AND p."primary"->'model'->>'model'=r.old_model;

UPDATE ai_profiles p SET fallback=jsonb_set(p.fallback,'{model,model}',to_jsonb(r.new_model)),
 revision=p.revision+1,updated_at=now()
FROM retired_gpt_5_mini r
WHERE p.fallback IS NOT NULL AND jsonb_typeof(p.fallback)='object'
 AND p.fallback->'model'->>'provider'=r.provider AND p.fallback->'model'->>'model'=r.old_model;

UPDATE agents a SET model=r.new_model FROM retired_gpt_5_mini r WHERE a.provider=r.provider AND a.model=r.old_model;
UPDATE agent_versions v SET model=r.new_model FROM retired_gpt_5_mini r WHERE v.provider=r.provider AND v.model=r.old_model;
UPDATE workspace_agent_preset_versions v SET model=r.new_model FROM retired_gpt_5_mini r WHERE v.provider=r.provider AND v.model=r.old_model;
