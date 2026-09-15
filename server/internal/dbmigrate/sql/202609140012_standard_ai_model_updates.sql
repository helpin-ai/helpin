-- Update shipped defaults only. Custom profiles and accepted run inputs stay frozen.
CREATE TEMP TABLE standard_ai_model_updates(tier text PRIMARY KEY, old_model text, new_model text, controls jsonb) ON COMMIT DROP;
INSERT INTO standard_ai_model_updates VALUES
 ('small','deepseek/deepseek-v4-flash-0731:nitro','deepseek/deepseek-v4.1-flash:nitro','{"openrouter":{"provider":{"quantizations":["fp8","fp16","bf16","fp32"]}}}'),
 ('medium','google/gemini-3.7-flash','google/gemini-3.8-flash','{}');

UPDATE ai_profiles p SET "primary"=jsonb_set(p."primary",'{model,model}',to_jsonb(u.new_model)),
 revision=p.revision+1,updated_at=now()
FROM standard_ai_model_updates u
WHERE p.id=md5('helpin-standard-ai-profile|'||p.workspace_id::text||'|'||u.tier)::uuid
 AND p.scope='workspace' AND p.user_id IS NULL AND p.deleted_at IS NULL AND p.fallback IS NULL
 AND p."primary"=jsonb_build_object('connection_id',md5('helpin-standard-ai-connection|'||p.workspace_id::text||'|openrouter')::uuid,
 'model',jsonb_build_object('provider','openrouter','model',u.old_model,'controls',u.controls));

CREATE TEMP TABLE current_standard_ai_profiles ON COMMIT DROP AS
SELECT p.id,p.workspace_id,u.tier,u.new_model,u.controls FROM ai_profiles p CROSS JOIN standard_ai_model_updates u
WHERE p.id=md5('helpin-standard-ai-profile|'||p.workspace_id::text||'|'||u.tier)::uuid
 AND p.scope='workspace' AND p.user_id IS NULL AND p.deleted_at IS NULL AND p.fallback IS NULL
 AND p."primary"=jsonb_build_object('connection_id',md5('helpin-standard-ai-connection|'||p.workspace_id::text||'|openrouter')::uuid,
 'model',jsonb_build_object('provider','openrouter','model',u.new_model,'controls',u.controls));

-- Ask Agent and copies inheriting the old default move from Medium to Small.
UPDATE agents a SET ai_profile_id=s.id
FROM current_standard_ai_profiles s JOIN current_standard_ai_profiles m ON m.workspace_id=s.workspace_id AND m.tier='medium'
WHERE s.tier='small' AND a.workspace_id=s.workspace_id AND a.ai_profile_id=m.id
 AND coalesce(nullif(a.preset_key,''),nullif(a.source_preset_key,''))='ask_agent';

UPDATE agent_versions v SET ai_profile_id=s.id
FROM agents a,current_standard_ai_profiles s JOIN current_standard_ai_profiles m ON m.workspace_id=s.workspace_id AND m.tier='medium'
WHERE v.agent_id=a.id AND v.workspace_id=s.workspace_id AND v.ai_profile_id=m.id AND s.tier='small'
 AND coalesce(nullif(a.preset_key,''),nullif(a.source_preset_key,''))='ask_agent';

-- Keep editable agent/version metadata consistent with their selected profile.
UPDATE agents a SET model_tier=p.tier,provider='openrouter',model=p.new_model,
 execution_config=(coalesce(a.execution_config,'{}'::jsonb)-'reasoning_effort'-'service_tier'-'openrouter')||p.controls
FROM current_standard_ai_profiles p WHERE a.ai_profile_id=p.id AND a.workspace_id=p.workspace_id;
UPDATE agent_versions v SET model_tier=p.tier,provider='openrouter',model=p.new_model,
 execution_config=(coalesce(v.execution_config,'{}'::jsonb)-'reasoning_effort'-'service_tier'-'openrouter')||p.controls
FROM current_standard_ai_profiles p WHERE v.ai_profile_id=p.id AND v.workspace_id=p.workspace_id;

UPDATE workspace_agent_preset_versions v SET model_tier='small',model=u.new_model,
 execution_config=coalesce(v.execution_config,'{}'::jsonb)||u.controls
FROM standard_ai_model_updates u WHERE u.tier='small' AND v.family_key='ask_agent'
 AND v.model_tier='medium' AND v.provider='openrouter' AND v.model IN ('google/gemini-3.7-flash','google/gemini-3.8-flash')
 AND NOT (coalesce(v.execution_config,'{}'::jsonb) ?| ARRAY['reasoning_effort','service_tier','openrouter']);

UPDATE workspace_agent_preset_versions v SET model=u.new_model
FROM standard_ai_model_updates u WHERE v.model_tier=u.tier AND v.provider='openrouter' AND v.model=u.old_model;
