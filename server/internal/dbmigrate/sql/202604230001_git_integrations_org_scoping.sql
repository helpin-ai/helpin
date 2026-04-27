-- Migration: git_integrations_org_scoping
-- Add organization scoping to git integrations and backfill from workspaces.

ALTER TABLE git_integrations
  ADD COLUMN IF NOT EXISTS organization_id uuid;

UPDATE git_integrations gi
   SET organization_id = w.organization_id
  FROM workspaces w
 WHERE gi.workspace_id = w.id
   AND gi.organization_id IS NULL;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
      FROM git_integrations
     WHERE organization_id IS NULL
  ) THEN
    RAISE EXCEPTION 'git_integrations contains rows with null organization_id after backfill';
  END IF;
END $$;

ALTER TABLE git_integrations
  ALTER COLUMN organization_id SET NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
      FROM pg_constraint
     WHERE conname = 'git_integrations_organization_id_fk'
  ) THEN
    ALTER TABLE git_integrations
      ADD CONSTRAINT git_integrations_organization_id_fk
      FOREIGN KEY (organization_id)
      REFERENCES organizations(id)
      ON DELETE RESTRICT;
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS git_integrations_org_provider_idx
  ON git_integrations (organization_id, provider);
