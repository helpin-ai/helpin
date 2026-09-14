-- Consolidate generated EE defaults without changing frozen run selections or
-- moving encrypted secrets: connection IDs form part of their encryption AAD.
ALTER TABLE ai_connections ADD COLUMN IF NOT EXISTS superseded_by uuid REFERENCES ai_connections(id);

CREATE TEMP TABLE duplicate_managed_ai_connections ON COMMIT DROP AS
SELECT duplicate.id,duplicate.workspace_id,duplicate.provider,canonical.id AS canonical_id
FROM ai_connections duplicate
CROSS JOIN LATERAL (
 SELECT c.id FROM ai_connections c
 WHERE c.workspace_id=duplicate.workspace_id AND c.provider=duplicate.provider
  AND c.scope='workspace' AND c.user_id IS NULL AND c.funding='managed' AND c.superseded_by IS NULL
  AND c.id<>duplicate.id
 ORDER BY c.created_at,c.id LIMIT 1
) canonical
WHERE duplicate.provider IN ('openai','anthropic','openrouter')
 AND duplicate.id=md5('helpin-standard-ai-connection|'||duplicate.workspace_id::text||'|'||duplicate.provider)::uuid
 AND duplicate.scope='workspace' AND duplicate.user_id IS NULL AND duplicate.superseded_by IS NULL
 AND duplicate.name IN ('Standard '||duplicate.provider,duplicate.provider||' (managed)')
 AND ((duplicate.funding='managed' AND duplicate.status IN ('connected','unconfigured'))
   OR (duplicate.funding='customer' AND duplicate.status='unconfigured' AND coalesce(octet_length(duplicate.encrypted_secret),0)=0));

-- Retain model choices, controls and fallback behavior; replace only matching
-- references to the redundant generated connection.
CREATE TEMP TABLE managed_ai_profile_references ON COMMIT DROP AS
SELECT p.id,primary_connection.canonical_id AS primary_id,fallback_connection.canonical_id AS fallback_id
FROM ai_profiles p
LEFT JOIN duplicate_managed_ai_connections primary_connection
 ON primary_connection.workspace_id=p.workspace_id AND p."primary"->>'connection_id'=primary_connection.id::text
 AND p."primary"->'model'->>'provider'=primary_connection.provider
LEFT JOIN duplicate_managed_ai_connections fallback_connection
 ON fallback_connection.workspace_id=p.workspace_id AND p.fallback->>'connection_id'=fallback_connection.id::text
 AND p.fallback->'model'->>'provider'=fallback_connection.provider
WHERE p.deleted_at IS NULL AND (primary_connection.id IS NOT NULL OR fallback_connection.id IS NOT NULL);

UPDATE ai_profiles p SET
 "primary"=CASE WHEN r.primary_id IS NOT NULL THEN jsonb_set(p."primary",'{connection_id}',to_jsonb(r.primary_id::text)) ELSE p."primary" END,
 fallback=CASE WHEN r.fallback_id IS NOT NULL THEN jsonb_set(p.fallback,'{connection_id}',to_jsonb(r.fallback_id::text)) ELSE p.fallback END,
 revision=p.revision+1,updated_at=now()
FROM managed_ai_profile_references r WHERE p.id=r.id;

-- Keep the old credential rows for accepted runs and continuations. They are
-- hidden from connection pickers and cannot be selected in newly saved profiles.
UPDATE ai_connections c SET superseded_by=d.canonical_id,updated_at=now()
FROM duplicate_managed_ai_connections d WHERE c.id=d.id
 AND NOT EXISTS (SELECT 1 FROM ai_profiles p WHERE p.workspace_id=d.workspace_id AND p.deleted_at IS NULL
  AND (p."primary"->>'connection_id'=d.id::text OR p.fallback->>'connection_id'=d.id::text));

-- Fresh installations have only one generated default per provider. Give these
-- the same names as the existing EE defaults without renaming user choices.
UPDATE ai_connections SET name=provider||' (managed)',updated_at=now()
WHERE scope='workspace' AND funding='managed' AND superseded_by IS NULL
 AND id=md5('helpin-standard-ai-connection|'||workspace_id::text||'|'||provider)::uuid
 AND name='Standard '||provider;
