-- One-time product reset: preset families use their standard size, custom agents
-- use Small. Accepted run inputs/checkpoints are deliberately not updated.
-- Connection secrets are supplied by application provisioning, never migrations.
CREATE TEMP TABLE standard_ai_sizes(tier text PRIMARY KEY, model text, controls jsonb) ON COMMIT DROP;
INSERT INTO standard_ai_sizes VALUES
 ('small','deepseek/deepseek-v4-flash-0731:nitro','{"openrouter":{"provider":{"quantizations":["fp8","fp16","bf16","fp32"]}}}'),
 ('medium','google/gemini-3.7-flash','{}'),
 ('large','openai/gpt-5.6-terra','{}'),
 ('flagship','anthropic/claude-sonnet-5','{}');

INSERT INTO ai_connections(id,workspace_id,user_id,scope,funding,name,provider,status)
SELECT md5('helpin-standard-ai-connection|'||id::text||'|openrouter')::uuid,
 id,NULL,'workspace','customer','Standard openrouter','openrouter','unconfigured'
FROM workspaces ON CONFLICT(id) DO NOTHING;

INSERT INTO ai_profiles(id,workspace_id,scope,name,revision,"primary")
SELECT md5('helpin-standard-ai-profile|'||w.id::text||'|'||s.tier)::uuid,
 w.id,'workspace',initcap(s.tier),1,
 jsonb_build_object('connection_id',md5('helpin-standard-ai-connection|'||w.id::text||'|openrouter')::uuid,
 'model',jsonb_build_object('provider','openrouter','model',s.model,'controls',s.controls))
FROM workspaces w CROSS JOIN standard_ai_sizes s ON CONFLICT(id) DO NOTHING;

CREATE TEMP TABLE standard_preset_sizes(family text PRIMARY KEY,tier text) ON COMMIT DROP;
INSERT INTO standard_preset_sizes VALUES
 ('epic_planner','small'),('task_planner','large'),('crm_operator','large'),
 ('support_agent','small'),('documentation_agent','small'),('marketer','large'),
 ('code_builder','large'),('review_agent','large'),('command_agent','medium'),('ask_agent','medium');

CREATE TEMP TABLE reset_agent_sizes ON COMMIT DROP AS
SELECT a.id,coalesce(p.tier,'small') AS tier FROM agents a
LEFT JOIN standard_preset_sizes p ON p.family=coalesce(nullif(a.preset_key,''),nullif(a.source_preset_key,''));

UPDATE agents a SET ai_profile_id=md5('helpin-standard-ai-profile|'||a.workspace_id::text||'|'||s.tier)::uuid,
 model_tier=s.tier,provider='openrouter',model=s.model,
 execution_config=(coalesce(a.execution_config,'{}'::jsonb)-'reasoning_effort'-'service_tier'-'openrouter')||s.controls
FROM reset_agent_sizes r JOIN standard_ai_sizes s ON s.tier=r.tier WHERE a.id=r.id;

UPDATE agent_versions v SET ai_profile_id=md5('helpin-standard-ai-profile|'||v.workspace_id::text||'|'||s.tier)::uuid,
 model_tier=s.tier,provider='openrouter',model=s.model,
 execution_config=(coalesce(v.execution_config,'{}'::jsonb)-'reasoning_effort'-'service_tier'-'openrouter')||s.controls
FROM reset_agent_sizes r JOIN standard_ai_sizes s ON s.tier=r.tier WHERE v.agent_id=r.id;

UPDATE workspace_agent_preset_versions v SET model_tier=s.tier,provider='openrouter',model=s.model,
 execution_config=(coalesce(v.execution_config,'{}'::jsonb)-'reasoning_effort'-'service_tier'-'openrouter')||s.controls
FROM standard_ai_sizes s WHERE s.tier=coalesce((SELECT p.tier FROM standard_preset_sizes p WHERE p.family=v.family_key),'small');

INSERT INTO ai_workspace_settings(workspace_id,default_profile_id)
SELECT id,md5('helpin-standard-ai-profile|'||id::text||'|small')::uuid FROM workspaces
ON CONFLICT(workspace_id) DO UPDATE SET default_profile_id=excluded.default_profile_id;

ALTER TABLE ai_workspace_settings DROP COLUMN IF EXISTS profiles_bootstrapped_at;
