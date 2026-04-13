-- Migration: add_docs_collection_public_ids
-- Adds an immutable short public id to docs collections so public collection
-- URLs can remain stable when a collection slug changes.

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

    ALTER TABLE docs_collections
        ADD COLUMN IF NOT EXISTS public_id TEXT NOT NULL DEFAULT '';

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
END $$;
