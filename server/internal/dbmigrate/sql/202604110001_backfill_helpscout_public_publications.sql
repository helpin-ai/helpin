-- Migration: backfill_helpcenter_publications_for_published_helpscout_imports
--
-- Older Help Scout imports could mark documents internally published without
-- creating the live public publication snapshot. Public help center reads key
-- off docs_helpcenter_article_publications plus public_published_at, so these
-- imported articles looked "published" in admin but stayed invisible publicly.
--
-- Backfill the missing public state for imported Help Scout articles that are:
--   - already internally published
--   - in external-capable spaces
--   - still missing public_published_at
--
-- The backfill also restores the default-locale article mirror because the
-- original import path saved content before the help-center article row existed.

DO $$
DECLARE
    rec RECORD;
    candidate_slug TEXT;
    suffix_num INTEGER;
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name = 'docs_documents'
    ) OR NOT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name = 'docs_helpcenter_articles'
    ) OR NOT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name = 'docs_helpcenter_article_publications'
    ) OR NOT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name = 'docs_helpcenter_article_translations'
    ) OR NOT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name = 'docs_contents'
    ) OR NOT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name = 'docs_spaces'
    ) THEN
        RETURN;
    END IF;

    DROP TABLE IF EXISTS tmp_helpscout_publication_backfill;

    CREATE TEMP TABLE tmp_helpscout_publication_backfill (
        document_id UUID PRIMARY KEY,
        workspace_id UUID NOT NULL,
        space_id UUID NOT NULL,
        collection_id UUID NULL,
        locale TEXT NOT NULL,
        source_object_id TEXT NULL,
        title TEXT NOT NULL,
        excerpt TEXT NULL,
        content JSONB NULL,
        content_text TEXT NOT NULL,
        seo_title TEXT NULL,
        seo_description TEXT NULL,
        view_count INTEGER NOT NULL,
        helpful_count INTEGER NOT NULL,
        not_helpful_count INTEGER NOT NULL,
        published_at TIMESTAMPTZ NOT NULL,
        source_updated_at TIMESTAMPTZ NULL,
        base_slug TEXT NOT NULL,
        final_slug TEXT NULL
    ) ON COMMIT DROP;

    INSERT INTO tmp_helpscout_publication_backfill (
        document_id,
        workspace_id,
        space_id,
        collection_id,
        locale,
        source_object_id,
        title,
        excerpt,
        content,
        content_text,
        seo_title,
        seo_description,
        view_count,
        helpful_count,
        not_helpful_count,
        published_at,
        source_updated_at,
        base_slug
    )
    SELECT
        d.id,
        d.workspace_id,
        d.space_id,
        d.collection_id,
        COALESCE(NULLIF(BTRIM(cfg.default_locale), ''), 'en') AS locale,
        dc.import_source_object_id,
        d.title,
        d.excerpt,
        dc.content,
        COALESCE(dc.content_text, ''),
        CASE
            WHEN NULLIF(BTRIM(ha.seo_title), '') IS NOT NULL THEN BTRIM(ha.seo_title)
            WHEN NULLIF(BTRIM(d.title), '') IS NOT NULL THEN BTRIM(d.title)
            ELSE NULL
        END AS seo_title,
        CASE
            WHEN NULLIF(BTRIM(ha.seo_description), '') IS NOT NULL THEN BTRIM(ha.seo_description)
            WHEN NULLIF(BTRIM(d.excerpt), '') IS NOT NULL THEN BTRIM(d.excerpt)
            ELSE NULL
        END AS seo_description,
        COALESCE(ha.view_count, 0),
        COALESCE(ha.helpful_count, 0),
        COALESCE(ha.not_helpful_count, 0),
        COALESCE(d.published_at, d.updated_at, NOW()) AS published_at,
        d.updated_at,
        COALESCE(
            NULLIF(
                REGEXP_REPLACE(
                    REGEXP_REPLACE(
                        LOWER(
                            COALESCE(
                                NULLIF(BTRIM(COALESCE(p.slug, ha.slug)), ''),
                                NULLIF(BTRIM(d.title), ''),
                                'article'
                            )
                        ),
                        '[^a-z0-9]+',
                        '-',
                        'g'
                    ),
                    '(^-+|-+$)',
                    '',
                    'g'
                ),
                ''
            ),
            'article'
        ) AS base_slug
    FROM docs_documents d
    JOIN docs_spaces s
      ON s.id = d.space_id
     AND s.deleted_at IS NULL
    JOIN docs_helpcenter_articles ha
      ON ha.document_id = d.id
    LEFT JOIN docs_helpcenter_configs cfg
      ON cfg.workspace_id = d.workspace_id
    LEFT JOIN docs_contents dc
      ON dc.document_id = d.id
    LEFT JOIN docs_helpcenter_article_publications p
      ON p.document_id = d.id
     AND p.locale = COALESCE(NULLIF(BTRIM(cfg.default_locale), ''), 'en')
    WHERE d.deleted_at IS NULL
      AND d.status = 'published'
      AND s.type = 'external_capable'
      AND ha.public_published_at IS NULL
      AND LOWER(COALESCE(dc.import_source_system, '')) = 'helpscout';

    FOR rec IN
        SELECT
            document_id,
            space_id,
            locale,
            base_slug,
            published_at
        FROM tmp_helpscout_publication_backfill
        ORDER BY published_at ASC, document_id ASC
    LOOP
        candidate_slug := rec.base_slug;
        suffix_num := 2;

        WHILE EXISTS (
            SELECT 1
            FROM docs_helpcenter_article_publications p
            WHERE p.space_id = rec.space_id
              AND p.locale = rec.locale
              AND p.slug = candidate_slug
              AND NOT EXISTS (
                  SELECT 1
                  FROM tmp_helpscout_publication_backfill t
                  WHERE t.document_id = p.document_id
              )
        ) OR EXISTS (
            SELECT 1
            FROM docs_helpcenter_article_translations hat
            WHERE hat.space_id = rec.space_id
              AND hat.locale = rec.locale
              AND hat.slug = candidate_slug
              AND NOT EXISTS (
                  SELECT 1
                  FROM tmp_helpscout_publication_backfill t
                  WHERE t.document_id = hat.document_id
              )
        ) OR EXISTS (
            SELECT 1
            FROM tmp_helpscout_publication_backfill t
            WHERE t.space_id = rec.space_id
              AND t.locale = rec.locale
              AND t.final_slug = candidate_slug
              AND t.document_id <> rec.document_id
        )
        LOOP
            candidate_slug := rec.base_slug || '-' || suffix_num::TEXT;
            suffix_num := suffix_num + 1;
        END LOOP;

        UPDATE tmp_helpscout_publication_backfill
        SET final_slug = candidate_slug
        WHERE document_id = rec.document_id;
    END LOOP;

    INSERT INTO docs_helpcenter_article_publications (
        document_id,
        workspace_id,
        space_id,
        collection_id,
        locale,
        title,
        slug,
        excerpt,
        content,
        content_text,
        seo_title,
        seo_description,
        published_at
    )
    SELECT
        document_id,
        workspace_id,
        space_id,
        collection_id,
        locale,
        title,
        final_slug,
        excerpt,
        content,
        content_text,
        seo_title,
        seo_description,
        published_at
    FROM tmp_helpscout_publication_backfill
    ON CONFLICT (document_id, locale) DO UPDATE
    SET workspace_id = EXCLUDED.workspace_id,
        space_id = EXCLUDED.space_id,
        collection_id = EXCLUDED.collection_id,
        title = EXCLUDED.title,
        slug = EXCLUDED.slug,
        excerpt = EXCLUDED.excerpt,
        content = EXCLUDED.content,
        content_text = EXCLUDED.content_text,
        seo_title = EXCLUDED.seo_title,
        seo_description = EXCLUDED.seo_description,
        published_at = EXCLUDED.published_at,
        updated_at = NOW();

    UPDATE docs_helpcenter_articles ha
    SET slug = t.final_slug,
        public_published_at = t.published_at
    FROM tmp_helpscout_publication_backfill t
    WHERE ha.document_id = t.document_id
      AND (
          ha.slug IS DISTINCT FROM t.final_slug
          OR ha.public_published_at IS DISTINCT FROM t.published_at
      );

    INSERT INTO docs_helpcenter_article_translations (
        document_id,
        workspace_id,
        space_id,
        collection_id,
        locale,
        title,
        slug,
        excerpt,
        content,
        content_text,
        seo_title,
        seo_description,
        status,
        source_updated_at,
        source_synced,
        published_at,
        view_count,
        helpful_count,
        not_helpful_count
    )
    SELECT
        document_id,
        workspace_id,
        space_id,
        collection_id,
        locale,
        title,
        final_slug,
        excerpt,
        content,
        content_text,
        seo_title,
        seo_description,
        'published',
        source_updated_at,
        TRUE,
        published_at,
        view_count,
        helpful_count,
        not_helpful_count
    FROM tmp_helpscout_publication_backfill
    ON CONFLICT (document_id, locale) DO UPDATE
    SET workspace_id = EXCLUDED.workspace_id,
        space_id = EXCLUDED.space_id,
        collection_id = EXCLUDED.collection_id,
        title = EXCLUDED.title,
        slug = EXCLUDED.slug,
        excerpt = EXCLUDED.excerpt,
        content = EXCLUDED.content,
        content_text = EXCLUDED.content_text,
        seo_title = EXCLUDED.seo_title,
        seo_description = EXCLUDED.seo_description,
        status = EXCLUDED.status,
        source_updated_at = EXCLUDED.source_updated_at,
        source_synced = EXCLUDED.source_synced,
        published_at = EXCLUDED.published_at,
        view_count = EXCLUDED.view_count,
        helpful_count = EXCLUDED.helpful_count,
        not_helpful_count = EXCLUDED.not_helpful_count,
        updated_at = NOW();

    IF EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name = 'docs_redirects'
    ) THEN
        UPDATE docs_redirects r
        SET target_article_slug = t.final_slug
        FROM tmp_helpscout_publication_backfill t
        WHERE r.workspace_id = t.workspace_id
          AND LOWER(COALESCE(r.source_system, '')) = 'helpscout'
          AND LOWER(COALESCE(r.source_object_type, '')) = 'article'
          AND r.source_object_id = t.source_object_id
          AND r.target_article_slug IS DISTINCT FROM t.final_slug;
    END IF;
END $$;
