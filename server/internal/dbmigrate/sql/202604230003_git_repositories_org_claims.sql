-- Migration: git_repositories_org_claims
-- Extend git_repositories in place for org-scoped claims and soft-delete semantics.

ALTER TABLE git_repositories
  ADD COLUMN IF NOT EXISTS active boolean NOT NULL DEFAULT true;

ALTER TABLE git_repositories
  ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

DO $$
DECLARE
  duplicate_groups integer;
BEGIN
  SELECT COUNT(*)
    INTO duplicate_groups
    FROM (
      SELECT integration_id, external_id
        FROM git_repositories
       GROUP BY integration_id, external_id
      HAVING COUNT(*) > 1
    ) q;

  IF duplicate_groups > 0 THEN
    RAISE EXCEPTION
      'git_repositories has % duplicate (integration_id, external_id) groups; reconcile before org-scoping migration',
      duplicate_groups;
  END IF;
END $$;

DROP INDEX IF EXISTS idx_git_repo_external;

CREATE UNIQUE INDEX IF NOT EXISTS git_repositories_integration_external_live_uidx
  ON git_repositories (integration_id, external_id)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS git_repositories_integration_external_idx
  ON git_repositories (integration_id, external_id);

CREATE INDEX IF NOT EXISTS git_repositories_workspace_active_idx
  ON git_repositories (workspace_id, active)
  WHERE deleted_at IS NULL;
