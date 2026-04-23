-- Migration: git_integrations_active_installation_unique
-- Enforce one active integration per provider/installation pair.

DO $$
DECLARE
  duplicate_groups integer;
BEGIN
  SELECT COUNT(*)
    INTO duplicate_groups
    FROM (
      SELECT provider, installation_id
        FROM git_integrations
       WHERE active = true
         AND installation_id IS NOT NULL
       GROUP BY provider, installation_id
      HAVING COUNT(*) > 1
    ) q;

  IF duplicate_groups > 0 THEN
    RAISE EXCEPTION
      'git_integrations has % active duplicate provider/installation groups; reconcile before creating the unique index',
      duplicate_groups;
  END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS git_integrations_installation_unique
  ON git_integrations (provider, installation_id)
  WHERE active = true AND installation_id IS NOT NULL;
