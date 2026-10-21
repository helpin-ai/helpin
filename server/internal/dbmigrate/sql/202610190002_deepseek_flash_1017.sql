-- Upgrade only unchanged shipped Small profiles. Custom selections and accepted runs remain intact.
WITH upgraded_profiles AS (
 UPDATE ai_profiles p
 SET "primary"=jsonb_set(p."primary",'{model,model}','"deepseek/deepseek-v4.1-flash-1017:nitro"'),
     revision=p.revision+1, updated_at=now()
 WHERE p.id=md5('helpin-standard-ai-profile|'||p.workspace_id::text||'|small')::uuid
   AND p.scope='workspace' AND p.user_id IS NULL AND p.deleted_at IS NULL AND p.fallback IS NULL
   AND p.revision IN (1,2)
   AND p."primary"=jsonb_build_object(
     'connection_id',md5('helpin-standard-ai-connection|'||p.workspace_id::text||'|openrouter')::uuid,
     'model',jsonb_build_object('provider','openrouter','model','deepseek/deepseek-v4.1-flash:nitro',
       'controls','{"openrouter":{"provider":{"quantizations":["fp8","fp16","bf16","fp32"]}}}'::jsonb))
 RETURNING p.id,p.workspace_id
), updated_agents AS (
 UPDATE agents a SET provider='openrouter',model='deepseek/deepseek-v4.1-flash-1017:nitro'
 FROM upgraded_profiles p WHERE a.ai_profile_id=p.id AND a.workspace_id=p.workspace_id
 RETURNING a.id
)
UPDATE agent_versions v SET provider='openrouter',model='deepseek/deepseek-v4.1-flash-1017:nitro'
FROM upgraded_profiles p WHERE v.ai_profile_id=p.id AND v.workspace_id=p.workspace_id;
