-- Convert the single-column UNIQUE constraints on crm_signal_routing_settings
-- and crm_signal_rollout_settings into unique indexes named the way GORM expects.
--
-- 202608280003 created these columns as `workspace_id uuid NOT NULL UNIQUE`,
-- which Postgres implements as a UNIQUE *constraint*. The Go models tag the
-- field `uniqueIndex`, not `unique`.
--
-- gorm.io/driver/postgres reads column uniqueness from
-- information_schema.table_constraints and marks a column unique only when the
-- constraint covers exactly one column. So AutoMigrate saw
-- columnType.Unique() == true while field.Unique == false, concluded the
-- uniqueness had to be removed, and issued
--   ALTER TABLE ... DROP CONSTRAINT "uni_<table>_workspace_id"
-- using its own naming convention. That constraint never existed, so startup
-- failed with SQLSTATE 42704.
--
-- A bare unique index is not reported in table_constraints, so after this
-- migration columnType.Unique() is false and matches the struct tag, while the
-- index pass finds idx_<table>_workspace_id already present and does nothing.
--
-- Composite UNIQUE(...) constraints elsewhere in 202608280004 are unaffected:
-- the driver ignores multi-column constraints for this purpose.

DO $$
DECLARE
    target record;
    constraint_name text;
    index_name text;
BEGIN
    FOR target IN
        SELECT unnest(ARRAY[
            'crm_signal_routing_settings',
            'crm_signal_rollout_settings'
        ]) AS table_name
    LOOP
        IF to_regclass('public.' || quote_ident(target.table_name)) IS NULL THEN
            CONTINUE;
        END IF;

        index_name := 'idx_' || target.table_name || '_workspace_id';

        -- Drop every single-column UNIQUE constraint on workspace_id, whatever
        -- it happens to be called. The table may have been created by the
        -- migration (<table>_workspace_id_key) or by an earlier AutoMigrate
        -- (uni_<table>_workspace_id).
        FOR constraint_name IN
            SELECT con.conname
            FROM pg_constraint con
            JOIN pg_class rel ON rel.oid = con.conrelid
            JOIN pg_namespace nsp ON nsp.oid = rel.relnamespace
            WHERE nsp.nspname = 'public'
              AND rel.relname = target.table_name
              AND con.contype = 'u'
              AND array_length(con.conkey, 1) = 1
              AND con.conkey[1] = (
                  SELECT att.attnum FROM pg_attribute att
                  WHERE att.attrelid = rel.oid AND att.attname = 'workspace_id'
                    AND NOT att.attisdropped
              )
        LOOP
            EXECUTE format('ALTER TABLE public.%I DROP CONSTRAINT %I',
                           target.table_name, constraint_name);
        END LOOP;

        EXECUTE format(
            'CREATE UNIQUE INDEX IF NOT EXISTS %I ON public.%I (workspace_id)',
            index_name, target.table_name);
    END LOOP;
END $$;

-- Assert the end state: uniqueness preserved as an index, no single-column
-- UNIQUE constraint left to confuse AutoMigrate.
DO $$
DECLARE
    target record;
BEGIN
    FOR target IN
        SELECT unnest(ARRAY[
            'crm_signal_routing_settings',
            'crm_signal_rollout_settings'
        ]) AS table_name
    LOOP
        IF to_regclass('public.' || quote_ident(target.table_name)) IS NULL THEN
            CONTINUE;
        END IF;

        IF to_regclass('public.' || quote_ident('idx_' || target.table_name || '_workspace_id')) IS NULL THEN
            RAISE EXCEPTION
                'crm signal settings: idx_%_workspace_id missing after conversion', target.table_name;
        END IF;

        IF EXISTS (
            SELECT 1
            FROM pg_constraint con
            JOIN pg_class rel ON rel.oid = con.conrelid
            JOIN pg_namespace nsp ON nsp.oid = rel.relnamespace
            WHERE nsp.nspname = 'public'
              AND rel.relname = target.table_name
              AND con.contype = 'u'
              AND array_length(con.conkey, 1) = 1
        ) THEN
            RAISE EXCEPTION
                'crm signal settings: a single-column UNIQUE constraint remains on %', target.table_name;
        END IF;
    END LOOP;
END $$;
