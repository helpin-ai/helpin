-- Migration: gitlab_oauth_to_pat
-- GitLab integrations switched from a Helpin-owned OAuth app to per-org
-- Personal/Group/Project Access Tokens. Mark any existing oauth_user GitLab
-- credentials as revoked so admins are forced to reconnect with a PAT; also
-- deactivate the integrations that point at those credentials. We do NOT
-- delete the rows so the audit trail is preserved.

UPDATE git_credentials
SET
  status = 'revoked',
  last_error = 'GitLab OAuth was removed; reconnect using a Personal/Group/Project Access Token.',
  refresh_token_encrypted = NULL,
  expires_at = NULL,
  updated_at = now()
WHERE provider = 'gitlab'
  AND auth_type = 'oauth_user'
  AND status = 'active';

UPDATE git_integrations
SET
  active = false,
  updated_at = now()
WHERE provider = 'gitlab'
  AND credential_id IN (
    SELECT id FROM git_credentials
    WHERE provider = 'gitlab' AND auth_type = 'oauth_user' AND status = 'revoked'
  );
