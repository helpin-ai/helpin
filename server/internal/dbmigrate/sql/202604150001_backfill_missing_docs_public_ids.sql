-- Migration: backfill_missing_docs_public_ids
-- Some docs_collections rows still carry empty public_id values despite the
-- earlier backfill (202604130003). This re-runs the backfill so every
-- non-deleted collection gets an 8-char public_id, and tightens the schema
-- with a CHECK constraint so future inserts cannot leave it empty.
-- Also defensively re-runs the backfill for docs_helpcenter_articles.

DO $$
DECLARE
    updated_rows integer := 0;
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name = 'docs_collections'
    ) THEN
        RETURN;
    END IF;

    UPDATE docs_collections
    SET public_id = lower(substr(md5(gen_random_uuid()::text), 1, 8))
    WHERE COALESCE(btrim(public_id), '') = '';

    LOOP
        WITH ranked AS (
            SELECT
                id,
                public_id,
                row_number() OVER (
                    PARTITION BY public_id
                    ORDER BY created_at ASC, id ASC
                ) AS row_num
            FROM docs_collections
            WHERE COALESCE(btrim(public_id), '') <> ''
        ),
        duplicates AS (
            SELECT id
            FROM ranked
            WHERE row_num > 1
            LIMIT 1000
        )
        UPDATE docs_collections collection
        SET public_id = lower(substr(md5(gen_random_uuid()::text), 1, 8))
        FROM duplicates
        WHERE collection.id = duplicates.id;

        GET DIAGNOSTICS updated_rows = ROW_COUNT;
        EXIT WHEN updated_rows = 0;
    END LOOP;

    CREATE UNIQUE INDEX IF NOT EXISTS idx_docs_collections_public_id
        ON docs_collections (public_id)
        WHERE public_id != '';

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'docs_collections'::regclass
          AND conname = 'docs_collections_public_id_nonempty'
    ) THEN
        ALTER TABLE docs_collections
            ADD CONSTRAINT docs_collections_public_id_nonempty
            CHECK (public_id <> '');
    END IF;
END $$;

DO $$
DECLARE
    updated_rows integer := 0;
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name = 'docs_helpcenter_articles'
    ) THEN
        RETURN;
    END IF;

    UPDATE docs_helpcenter_articles
    SET public_id = lower(substr(md5(gen_random_uuid()::text), 1, 8))
    WHERE COALESCE(btrim(public_id), '') = '';

    LOOP
        WITH ranked AS (
            SELECT
                id,
                public_id,
                row_number() OVER (
                    PARTITION BY public_id
                    ORDER BY created_at ASC, id ASC
                ) AS row_num
            FROM docs_helpcenter_articles
            WHERE COALESCE(btrim(public_id), '') <> ''
        ),
        duplicates AS (
            SELECT id
            FROM ranked
            WHERE row_num > 1
            LIMIT 1000
        )
        UPDATE docs_helpcenter_articles article
        SET public_id = lower(substr(md5(gen_random_uuid()::text), 1, 8))
        FROM duplicates
        WHERE article.id = duplicates.id;

        GET DIAGNOSTICS updated_rows = ROW_COUNT;
        EXIT WHEN updated_rows = 0;
    END LOOP;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'docs_helpcenter_articles'::regclass
          AND conname = 'docs_helpcenter_articles_public_id_nonempty'
    ) THEN
        ALTER TABLE docs_helpcenter_articles
            ADD CONSTRAINT docs_helpcenter_articles_public_id_nonempty
            CHECK (public_id <> '');
    END IF;
END $$;
