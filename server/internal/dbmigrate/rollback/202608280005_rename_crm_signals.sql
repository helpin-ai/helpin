-- Rollback for 202608280005_rename_crm_signals.sql.
--
-- This directory is NOT embedded by the migration runner. Apply it by hand only,
-- and only alongside rolling the server image back to a build whose
-- model.CRMSignal still reports TableName() == "crm_buyer_signals".
--
-- Order matters: rename the indexes back first, then the table, so the catalog
-- sweep can still find the indexes by their parent table name.

DO $$
DECLARE
    idx record;
    new_name text;
BEGIN
    IF to_regclass('public.crm_signals') IS NULL THEN
        RAISE NOTICE 'crm signal rename rollback: crm_signals absent, nothing to do';
        RETURN;
    END IF;

    FOR idx IN
        SELECT c.relname AS name
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        JOIN pg_index i ON i.indexrelid = c.oid
        JOIN pg_class t ON t.oid = i.indrelid
        WHERE n.nspname = 'public'
          AND t.relname = 'crm_signals'
          AND c.relname LIKE 'idx\_crm\_signals\_%'
          -- These two were always named idx_crm_signals_* and must keep their
          -- names; only the renamed set goes back.
          AND c.relname NOT IN (
              'idx_crm_signals_source_thread',
              'idx_crm_signals_workspace_source_type_source_id_signal_type_uni'
          )
        ORDER BY c.relname
    LOOP
        new_name := 'idx_crm_buyer_signals_' ||
                    substring(idx.name from length('idx_crm_signals_') + 1);

        IF octet_length(new_name) > 63 THEN
            RAISE EXCEPTION
                'crm signal rename rollback: target index name % exceeds the 63-byte limit', new_name;
        END IF;
        IF to_regclass('public.' || quote_ident(new_name)) IS NOT NULL THEN
            RAISE EXCEPTION
                'crm signal rename rollback: cannot rename index % to %, name already taken',
                idx.name, new_name;
        END IF;

        EXECUTE format('ALTER INDEX public.%I RENAME TO %I', idx.name, new_name);
    END LOOP;

    IF to_regclass('public.crm_signals_pkey') IS NOT NULL
       AND to_regclass('public.crm_buyer_signals_pkey') IS NULL THEN
        ALTER INDEX public.crm_signals_pkey RENAME TO crm_buyer_signals_pkey;
    END IF;

    EXECUTE 'ALTER TABLE public.crm_signals RENAME TO crm_buyer_signals';
END $$;

DELETE FROM schema_migrations WHERE version = '202608280005';
