-- Migration: git_repository_fk_lifecycle
-- Align dependent repository foreign keys with grace-window cleanup semantics.

DO $$
DECLARE
  constraint_name text;
BEGIN
  SELECT con.conname
    INTO constraint_name
    FROM pg_constraint con
    JOIN pg_class rel
      ON rel.oid = con.conrelid
    JOIN pg_attribute att
      ON att.attrelid = rel.oid
     AND att.attnum = ANY (con.conkey)
   WHERE rel.relname = 'task_delivery_targets'
     AND att.attname = 'repository_id'
     AND con.contype = 'f'
   LIMIT 1;

  IF constraint_name IS NOT NULL THEN
    EXECUTE format('ALTER TABLE task_delivery_targets DROP CONSTRAINT %I', constraint_name);
  END IF;

  ALTER TABLE task_delivery_targets
    ADD CONSTRAINT task_delivery_targets_repository_id_fkey
    FOREIGN KEY (repository_id)
    REFERENCES git_repositories(id)
    ON DELETE SET NULL;
END $$;

DO $$
DECLARE
  constraint_name text;
BEGIN
  SELECT con.conname
    INTO constraint_name
    FROM pg_constraint con
    JOIN pg_class rel
      ON rel.oid = con.conrelid
    JOIN pg_attribute att
      ON att.attrelid = rel.oid
     AND att.attnum = ANY (con.conkey)
   WHERE rel.relname = 'task_git_links'
     AND att.attname = 'repository_id'
     AND con.contype = 'f'
   LIMIT 1;

  IF constraint_name IS NOT NULL THEN
    EXECUTE format('ALTER TABLE task_git_links DROP CONSTRAINT %I', constraint_name);
  END IF;

  ALTER TABLE task_git_links
    ADD CONSTRAINT task_git_links_repository_id_fkey
    FOREIGN KEY (repository_id)
    REFERENCES git_repositories(id)
    ON DELETE SET NULL;
END $$;

DO $$
DECLARE
  constraint_name text;
BEGIN
  SELECT con.conname
    INTO constraint_name
    FROM pg_constraint con
    JOIN pg_class rel
      ON rel.oid = con.conrelid
    JOIN pg_attribute att
      ON att.attrelid = rel.oid
     AND att.attnum = ANY (con.conkey)
   WHERE rel.relname = 'pm_team_repo_defaults'
     AND att.attname = 'repository_id'
     AND con.contype = 'f'
   LIMIT 1;

  IF constraint_name IS NOT NULL THEN
    EXECUTE format('ALTER TABLE pm_team_repo_defaults DROP CONSTRAINT %I', constraint_name);
  END IF;

  ALTER TABLE pm_team_repo_defaults
    ADD CONSTRAINT pm_team_repo_defaults_repository_id_fkey
    FOREIGN KEY (repository_id)
    REFERENCES git_repositories(id)
    ON DELETE RESTRICT;
END $$;
