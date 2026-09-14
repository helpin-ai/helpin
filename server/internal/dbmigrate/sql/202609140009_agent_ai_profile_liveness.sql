-- Serialize new agent assignments with profile deletion. Historical versions
-- retain their original references, and archived agents do not block deletion.
CREATE OR REPLACE FUNCTION enforce_active_agent_ai_profile() RETURNS trigger AS $$
BEGIN
 IF NEW.ai_profile_id IS NULL OR NEW.deleted_at IS NOT NULL THEN
  RETURN NEW;
 END IF;
 PERFORM 1 FROM ai_profiles
 WHERE id = NEW.ai_profile_id AND workspace_id = NEW.workspace_id
   AND scope = 'workspace' AND user_id IS NULL AND deleted_at IS NULL
 FOR SHARE;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'agent requires an active shared AI profile in its workspace'
   USING ERRCODE = '23514';
 END IF;
 RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS agents_active_ai_profile ON agents;
CREATE TRIGGER agents_active_ai_profile
 BEFORE INSERT OR UPDATE OF ai_profile_id, workspace_id, deleted_at ON agents
 FOR EACH ROW EXECUTE FUNCTION enforce_active_agent_ai_profile();
