-- Preserve personal IDs and their v1 encryption AAD. Workspace connections have
-- no owning user, so deleting their creator cannot delete automation credentials.
ALTER TABLE ai_connections ADD COLUMN IF NOT EXISTS scope TEXT NOT NULL DEFAULT 'personal';
ALTER TABLE ai_connections ALTER COLUMN user_id DROP NOT NULL;
DO $$ BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ai_connections_scope_owner') THEN
  ALTER TABLE ai_connections ADD CONSTRAINT ai_connections_scope_owner CHECK (
   (scope = 'personal' AND user_id IS NOT NULL) OR
   (scope = 'workspace' AND user_id IS NULL AND provider <> 'openai_chatgpt')
  );
 END IF;
END $$;
CREATE INDEX IF NOT EXISTS idx_ai_connections_workspace_scope ON ai_connections(workspace_id,scope);
