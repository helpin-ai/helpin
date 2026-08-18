-- Repair stale HelpScout redirect destinations after articles are re-imported.
--
-- Imported redirects are unique by workspace and source path. Before imported
-- redirects became refreshable, a re-import kept the original row and its old
-- article public ID. Match each redirect to its newest published imported
-- article by stable HelpScout source object ID and rebuild the canonical path.
-- This migration is intentionally idempotent.
WITH current_helpscout_articles AS (
    SELECT
        r.id AS redirect_id,
        COALESCE(c.slug, '') AS target_collection_slug,
        ha.slug AS target_article_slug,
        '/articles/' || ha.slug || '-' || ha.public_id AS target_path,
        ROW_NUMBER() OVER (
            PARTITION BY r.id
            ORDER BY
                CASE WHEN ha.public_published_at IS NOT NULL THEN 0 ELSE 1 END,
                COALESCE(ha.public_published_at, ha.updated_at, d.updated_at, dc.updated_at) DESC,
                d.id ASC
        ) AS rn
    FROM docs_redirects r
    JOIN docs_contents dc
      ON dc.import_source_object_id = r.source_object_id
     AND LOWER(COALESCE(dc.import_source_system, '')) = 'helpscout'
    JOIN docs_documents d
      ON d.id = dc.document_id
     AND d.workspace_id = r.workspace_id
     AND d.deleted_at IS NULL
    JOIN docs_helpcenter_articles ha
      ON ha.document_id = d.id
    LEFT JOIN docs_collections c
      ON c.id = d.collection_id
     AND c.deleted_at IS NULL
    WHERE r.type = 'imported'
      AND LOWER(COALESCE(r.source_system, '')) = 'helpscout'
      AND COALESCE(r.source_object_id, '') != ''
      AND COALESCE(ha.slug, '') != ''
      AND COALESCE(ha.public_id, '') != ''
)
UPDATE docs_redirects r
SET
    target_path = a.target_path,
    target_article_slug = a.target_article_slug,
    target_collection_slug = CASE
        WHEN a.target_collection_slug != '' THEN a.target_collection_slug
        ELSE r.target_collection_slug
    END
FROM current_helpscout_articles a
WHERE a.redirect_id = r.id
  AND a.rn = 1
  AND (
      r.target_path IS DISTINCT FROM a.target_path
      OR r.target_article_slug IS DISTINCT FROM a.target_article_slug
      OR (
          a.target_collection_slug != ''
          AND r.target_collection_slug IS DISTINCT FROM a.target_collection_slug
      )
  );

-- Category imports do not store their source ID on the collection row. Resolve
-- them by slug inside the workspace's latest completed HelpScout import space.
-- Only update unambiguous matches; historical categories without a current
-- replacement remain untouched.
WITH latest_completed_helpscout_import AS (
    SELECT DISTINCT ON (workspace_id)
        workspace_id,
        space_id
    FROM docs_import_jobs
    WHERE LOWER(source) = 'helpscout'
      AND status = 'done'
      AND space_id IS NOT NULL
    ORDER BY
        workspace_id,
        completed_at DESC NULLS LAST,
        created_at DESC
), current_helpscout_categories AS (
    SELECT
        r.id AS redirect_id,
        c.slug AS target_collection_slug,
        '/c/' || c.slug || '-' || c.public_id AS target_path,
        COUNT(*) OVER (PARTITION BY r.id) AS matches
    FROM docs_redirects r
    JOIN latest_completed_helpscout_import j
      ON j.workspace_id = r.workspace_id
    JOIN docs_collections c
      ON c.space_id = j.space_id
     AND c.deleted_at IS NULL
     AND c.slug = r.target_collection_slug
    WHERE r.type = 'imported'
      AND LOWER(COALESCE(r.source_system, '')) = 'helpscout'
      AND r.source_object_type = 'category'
)
UPDATE docs_redirects r
SET
    target_path = c.target_path,
    target_collection_slug = c.target_collection_slug
FROM current_helpscout_categories c
WHERE c.redirect_id = r.id
  AND c.matches = 1
  AND (
      r.target_path IS DISTINCT FROM c.target_path
      OR r.target_collection_slug IS DISTINCT FROM c.target_collection_slug
  );
