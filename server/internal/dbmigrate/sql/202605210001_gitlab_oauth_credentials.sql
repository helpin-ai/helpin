-- Migration: gitlab_oauth_credentials
-- Add provider credentials for GitLab OAuth-backed project connections.

CREATE TABLE IF NOT EXISTS git_credentials (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
  provider text NOT NULL,
  base_url text NOT NULL,
  auth_type text NOT NULL,
  external_user_id text,
  account_login text,
  display_name text NOT NULL,
  scopes text,
  access_token_encrypted text,
  refresh_token_encrypted text,
  expires_at timestamptz,
  status text NOT NULL DEFAULT 'active',
  connected_by uuid,
  last_error text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE git_integrations
  ADD COLUMN IF NOT EXISTS credential_id uuid;

ALTER TABLE git_repositories
  ADD COLUMN IF NOT EXISTS base_url text;

ALTER TABLE task_git_links
  ADD COLUMN IF NOT EXISTS base_url text;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'git_integrations_credential_id_fk'
  ) THEN
    ALTER TABLE git_integrations
      ADD CONSTRAINT git_integrations_credential_id_fk
      FOREIGN KEY (credential_id)
      REFERENCES git_credentials(id)
      ON DELETE SET NULL;
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS git_credentials_org_provider_idx
  ON git_credentials (organization_id, provider, base_url);

CREATE INDEX IF NOT EXISTS git_integrations_credential_id_idx
  ON git_integrations (credential_id);

CREATE UNIQUE INDEX IF NOT EXISTS git_credentials_active_gitlab_user_unique
  ON git_credentials (organization_id, provider, base_url, external_user_id)
  WHERE status = 'active' AND external_user_id IS NOT NULL;
