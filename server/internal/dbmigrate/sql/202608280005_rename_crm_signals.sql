-- Rename crm_buyer_signals -> crm_signals.
--
-- Cutover migration: must ship in the same release as the model.CRMSignal
-- TableName() change. Paired rollback lives in
-- server/internal/dbmigrate/rollback/202608280005_rename_crm_signals.sql
-- (that directory is not embedded, so it is never applied as a forward step).
--
-- 202608280003 already deleted the legacy signal corpus, so this moves an empty
-- or freshly repopulated table. Foreign keys from crm_signal_feedback,
-- crm_signal_deliveries, and crm_signal_external_evidence follow the OID and are
-- auto-named after their child tables, so they need no changes.

-- ---------------------------------------------------------------------------
-- Step 1: the rename.
--
-- The API runs GORM AutoMigrate at startup. If a pod carrying the renamed model
-- boots before this job, AutoMigrate creates an empty crm_signals and a plain
-- rename would fail. Drop that shell, but only after proving it is genuinely
-- empty and unreferenced -- never silently skip the rename and leave real data
-- stranded in the legacy table.
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    legacy_kind "char";
    target_kind "char";
    target_rows bigint;
BEGIN
    SELECT c.relkind INTO legacy_kind
    FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = 'public' AND c.relname = 'crm_buyer_signals';

    SELECT c.relkind INTO target_kind
    FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = 'public' AND c.relname = 'crm_signals';

    IF legacy_kind IS NOT NULL AND legacy_kind <> 'r' THEN
        RAISE EXCEPTION
            'crm signal rename: public.crm_buyer_signals is relkind %, not an ordinary table', legacy_kind;
    END IF;
    IF target_kind IS NOT NULL AND target_kind <> 'r' THEN
        RAISE EXCEPTION
            'crm signal rename: public.crm_signals is relkind %, not an ordinary table', target_kind;
    END IF;

    IF legacy_kind IS NULL AND target_kind IS NULL THEN
        RAISE EXCEPTION
            'crm signal rename: neither crm_buyer_signals nor crm_signals exists';
    END IF;

    IF legacy_kind IS NULL THEN
        RAISE NOTICE 'crm signal rename: already applied, crm_signals present';
        RETURN;
    END IF;

    IF target_kind IS NOT NULL THEN
        -- Both exist. The only legitimate cause is an API pod running AutoMigrate
        -- against the renamed struct before this job. That table is empty by
        -- construction; anything else means real writes landed in the wrong table.
        EXECUTE 'SELECT count(*) FROM public.crm_signals' INTO target_rows;
        IF target_rows > 0 THEN
            RAISE EXCEPTION
                'crm signal rename: public.crm_signals already holds % row(s); refusing to drop it, reconcile manually',
                target_rows;
        END IF;

        IF EXISTS (
            SELECT 1 FROM pg_constraint con
            JOIN pg_class t ON t.oid = con.confrelid
            JOIN pg_namespace n ON n.oid = t.relnamespace
            WHERE n.nspname = 'public' AND t.relname = 'crm_signals' AND con.contype = 'f'
        ) THEN
            RAISE EXCEPTION
                'crm signal rename: public.crm_signals is referenced by foreign keys; refusing to drop it';
        END IF;

        RAISE NOTICE 'crm signal rename: dropping empty AutoMigrate-created crm_signals shell';
        EXECUTE 'DROP TABLE public.crm_signals';
    END IF;

    EXECUTE 'ALTER TABLE public.crm_buyer_signals RENAME TO crm_signals';
END $$;

-- ---------------------------------------------------------------------------
-- Step 2: index renames.
--
-- Postgres keeps old index names through a table rename. This is not cosmetic:
-- GORM derives index names as idx_<table>_<column>, and model.CRMSignal has 22
-- index-tagged fields. Leaving the old names means every API pod builds a full
-- duplicate set of 22 indexes on startup. Catalog-driven so it matches whatever
-- each environment actually has, including the GORM-created set.
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    idx record;
    new_name text;
BEGIN
    FOR idx IN
        SELECT c.relname AS name
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        JOIN pg_index i ON i.indexrelid = c.oid
        JOIN pg_class t ON t.oid = i.indrelid
        WHERE n.nspname = 'public'
          AND t.relname = 'crm_signals'
          AND c.relname LIKE 'idx\_crm\_buyer\_signals\_%'
        ORDER BY c.relname
    LOOP
        new_name := 'idx_crm_signals_' ||
                    substring(idx.name from length('idx_crm_buyer_signals_') + 1);

        IF octet_length(new_name) > 63 THEN
            RAISE EXCEPTION
                'crm signal rename: target index name % exceeds the 63-byte identifier limit', new_name;
        END IF;
        IF to_regclass('public.' || quote_ident(new_name)) IS NOT NULL THEN
            RAISE EXCEPTION
                'crm signal rename: cannot rename index % to %, that name is already taken',
                idx.name, new_name;
        END IF;

        EXECUTE format('ALTER INDEX public.%I RENAME TO %I', idx.name, new_name);
    END LOOP;
END $$;

-- The primary key index follows the table name by convention only. Renaming the
-- index renames its constraint with it.
DO $$
BEGIN
    IF to_regclass('public.crm_buyer_signals_pkey') IS NOT NULL
       AND to_regclass('public.crm_signals_pkey') IS NULL THEN
        ALTER INDEX public.crm_buyer_signals_pkey RENAME TO crm_signals_pkey;
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- Step 3: adopt what repository.MigrateCRMSignalSchema used to do at API
-- startup. That function is deleted in this release; migrations own these from
-- here on. The unique index name is written in its 63-byte truncated form
-- because Postgres has been storing it truncated since it was first created.
-- ---------------------------------------------------------------------------
ALTER TABLE crm_signals ALTER COLUMN metadata SET DEFAULT '{}'::jsonb;
UPDATE crm_signals SET metadata = '{}'::jsonb WHERE metadata IS NULL;
ALTER TABLE crm_signals ALTER COLUMN metadata SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_crm_signals_source_thread
    ON crm_signals (source_thread_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_signals_workspace_source_type_source_id_signal_type_uni
    ON crm_signals (workspace_id, source_type, source_id, signal_type)
    WHERE source_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- Step 4: assert the end state.
-- ---------------------------------------------------------------------------
DO $$
BEGIN
    IF to_regclass('public.crm_signals') IS NULL THEN
        RAISE EXCEPTION 'crm signal rename: crm_signals missing after rename';
    END IF;
    IF to_regclass('public.crm_buyer_signals') IS NOT NULL THEN
        RAISE EXCEPTION 'crm signal rename: crm_buyer_signals still present after rename';
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public' AND c.relkind = 'i'
          AND c.relname LIKE 'idx\_crm\_buyer\_signals\_%'
    ) THEN
        RAISE EXCEPTION 'crm signal rename: stale idx_crm_buyer_signals_* indexes remain';
    END IF;
    IF to_regclass('public.idx_crm_rule_signal_evidence') IS NULL THEN
        RAISE EXCEPTION 'crm signal rename: idx_crm_rule_signal_evidence missing after rename';
    END IF;
END $$;
