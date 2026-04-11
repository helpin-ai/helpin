-- Migration: add_helpcenter_public_ids
-- Adds an immutable short public id to help center articles so public URLs can
-- move away from collection-based article paths and remain stable across
-- recategorization or slug changes.

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

    ALTER TABLE docs_helpcenter_articles
        ADD COLUMN IF NOT EXISTS public_id TEXT;

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

    CREATE UNIQUE INDEX IF NOT EXISTS idx_docs_helpcenter_articles_public_id
        ON docs_helpcenter_articles (public_id);

    ALTER TABLE docs_helpcenter_articles
        ALTER COLUMN public_id SET NOT NULL;
END $$;
