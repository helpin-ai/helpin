-- Repair stale Nextra imported redirect targets after docs were re-imported.
--
-- Nextra redirects store source_object_id values like "art:path/to/page.mdx".
-- The same source object id is also stored on docs_contents for the imported
-- document. If a help center is re-imported, the live article can receive a
-- new docs_helpcenter_articles.public_id while older redirect rows still point
-- at the previous /articles/{slug}-{public_id} target_path.
--
-- This migration remaps imported Nextra redirects to the current matching live
-- article by source_object_id, then recomputes target_path from the current
-- slug + public_id. It is intentionally idempotent.
WITH current_nextra_articles AS (
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
     AND LOWER(COALESCE(dc.import_source_system, '')) = 'nextra'
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
      AND LOWER(COALESCE(r.source_system, '')) = 'nextra'
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
FROM current_nextra_articles a
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
