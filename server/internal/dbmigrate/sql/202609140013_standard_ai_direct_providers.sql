-- Standard Large and Flagship use their direct providers. Secrets are filled
-- by edition-aware startup provisioning, never copied or decrypted by SQL.
CREATE TEMP TABLE standard_ai_direct_providers(tier text PRIMARY KEY, provider text, model text) ON COMMIT DROP;
INSERT INTO standard_ai_direct_providers VALUES
 ('large','openai','gpt-5.6-terra'),
 ('flagship','anthropic','claude-sonnet-5');

INSERT INTO ai_connections(id,workspace_id,user_id,scope,funding,name,provider,status)
SELECT md5('helpin-standard-ai-connection|'||w.id::text||'|'||u.provider)::uuid,
 w.id,NULL,'workspace','customer','Standard '||u.provider,u.provider,'unconfigured'
FROM workspaces w CROSS JOIN standard_ai_direct_providers u
ON CONFLICT(id) DO NOTHING;

-- Match the complete shipped route. Preserve customized connections, models,
-- controls, fallbacks, personal profiles, and deleted profiles.
CREATE TEMP TABLE standard_ai_direct_profile_updates ON COMMIT DROP AS
SELECT p.id,p.workspace_id,u.tier,u.provider,u.model
FROM ai_profiles p CROSS JOIN standard_ai_direct_providers u
WHERE p.id=md5('helpin-standard-ai-profile|'||p.workspace_id::text||'|'||u.tier)::uuid
 AND p.scope='workspace' AND p.user_id IS NULL AND p.deleted_at IS NULL AND p.fallback IS NULL
 AND p."primary"=jsonb_build_object('connection_id',md5('helpin-standard-ai-connection|'||p.workspace_id::text||'|openrouter')::uuid,
 'model',jsonb_build_object('provider','openrouter','model',u.provider||'/'||u.model,'controls','{}'::jsonb));

UPDATE ai_profiles p SET "primary"=jsonb_build_object(
 'connection_id',md5('helpin-standard-ai-connection|'||u.workspace_id::text||'|'||u.provider)::uuid,
 'model',jsonb_build_object('provider',u.provider,'model',u.model,'controls','{}'::jsonb)),
 revision=p.revision+1,updated_at=now()
FROM standard_ai_direct_profile_updates u WHERE p.id=u.id;

UPDATE agents a SET provider=u.provider,model=u.model,
 execution_config=coalesce(a.execution_config,'{}'::jsonb)-'reasoning_effort'-'service_tier'-'openrouter'
FROM standard_ai_direct_profile_updates u WHERE a.workspace_id=u.workspace_id AND a.ai_profile_id=u.id;
UPDATE agent_versions v SET provider=u.provider,model=u.model,
 execution_config=coalesce(v.execution_config,'{}'::jsonb)-'reasoning_effort'-'service_tier'-'openrouter'
FROM standard_ai_direct_profile_updates u WHERE v.workspace_id=u.workspace_id AND v.ai_profile_id=u.id;

UPDATE workspace_agent_preset_versions v SET provider=u.provider,model=u.model
FROM standard_ai_direct_providers u WHERE v.model_tier=u.tier AND v.provider='openrouter'
 AND v.model=u.provider||'/'||u.model
 AND NOT (coalesce(v.execution_config,'{}'::jsonb) ?| ARRAY['reasoning_effort','service_tier','openrouter']);

-- Accepted run inputs retain their original route and credential selection.
