ALTER TABLE ai_profiles ADD COLUMN IF NOT EXISTS agent_grant_id UUID;
ALTER TABLE ai_profiles ADD COLUMN IF NOT EXISTS grant_account_id TEXT NOT NULL DEFAULT '';

-- A removed member must explicitly share again after rejoining.
CREATE OR REPLACE FUNCTION revoke_departing_subscription_grants() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    UPDATE ai_profiles SET agent_grant_id = NULL, grant_account_id = '', revision = revision + 1
    WHERE workspace_id = OLD.workspace_id AND user_id = OLD.user_id AND agent_grant_id IS NOT NULL;
    RETURN NULL;
END;
$$;
DROP TRIGGER IF EXISTS revoke_subscription_grants_on_member_update ON workspace_members;
CREATE TRIGGER revoke_subscription_grants_on_member_update AFTER UPDATE OF status ON workspace_members
FOR EACH ROW WHEN (OLD.status = 'active' AND NEW.status IS DISTINCT FROM OLD.status)
EXECUTE FUNCTION revoke_departing_subscription_grants();
DROP TRIGGER IF EXISTS revoke_subscription_grants_on_member_delete ON workspace_members;
CREATE TRIGGER revoke_subscription_grants_on_member_delete AFTER DELETE ON workspace_members
FOR EACH ROW EXECUTE FUNCTION revoke_departing_subscription_grants();

CREATE OR REPLACE FUNCTION revoke_disconnected_subscription_grants() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    UPDATE ai_profiles SET agent_grant_id = NULL, grant_account_id = '', revision = revision + 1
    WHERE workspace_id = OLD.workspace_id AND "primary"->>'connection_id' = OLD.id::text AND agent_grant_id IS NOT NULL;
    RETURN NULL;
END;
$$;
DROP TRIGGER IF EXISTS revoke_subscription_grants_on_disconnect ON ai_connections;
CREATE TRIGGER revoke_subscription_grants_on_disconnect AFTER UPDATE OF status ON ai_connections
FOR EACH ROW WHEN (NEW.status = 'disconnected' AND OLD.status IS DISTINCT FROM NEW.status)
EXECUTE FUNCTION revoke_disconnected_subscription_grants();

-- Runtime revocation is an external call. Keep the work if an API replica
-- stops after committing a revocation or the runtime is temporarily down.
CREATE TABLE IF NOT EXISTS ai_model_grant_revocations (
    grant_id UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    connection_id UUID NOT NULL,
    retry_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE OR REPLACE FUNCTION queue_subscription_model_revocation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO ai_model_grant_revocations (grant_id, workspace_id, profile_id, connection_id)
    VALUES (OLD.agent_grant_id, OLD.workspace_id, OLD.id, (OLD."primary"->>'connection_id')::uuid)
    ON CONFLICT DO NOTHING;
    RETURN NULL;
END;
$$;
DROP TRIGGER IF EXISTS queue_model_revocation_on_update ON ai_profiles;
CREATE TRIGGER queue_model_revocation_on_update AFTER UPDATE OF agent_grant_id ON ai_profiles
FOR EACH ROW WHEN (OLD.agent_grant_id IS NOT NULL AND OLD.agent_grant_id IS DISTINCT FROM NEW.agent_grant_id)
EXECUTE FUNCTION queue_subscription_model_revocation();
DROP TRIGGER IF EXISTS queue_model_revocation_on_delete ON ai_profiles;
CREATE TRIGGER queue_model_revocation_on_delete AFTER DELETE ON ai_profiles
FOR EACH ROW WHEN (OLD.agent_grant_id IS NOT NULL)
EXECUTE FUNCTION queue_subscription_model_revocation();
