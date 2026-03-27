-- Normalize malformed docs redirect paths and slug parts created by older slug-change code.

WITH normalized AS (
  SELECT
    id,
    CASE
      WHEN array_length(array_remove(regexp_split_to_array(trim(coalesce(source_path, '')), '/+'), ''), 1) IS NULL THEN '/'
      ELSE '/' || array_to_string(array_remove(regexp_split_to_array(trim(coalesce(source_path, '')), '/+'), ''), '/')
    END AS source_path_norm,
    trim(both '/' from trim(coalesce(target_collection_slug, ''))) AS target_collection_slug_norm,
    nullif(trim(both '/' from trim(coalesce(target_article_slug, ''))), '') AS target_article_slug_norm
  FROM docs_redirects
)
UPDATE docs_redirects d
SET
  source_path = n.source_path_norm,
  target_collection_slug = n.target_collection_slug_norm,
  target_article_slug = n.target_article_slug_norm
FROM normalized n
WHERE d.id = n.id
  AND (
    d.source_path IS DISTINCT FROM n.source_path_norm OR
    d.target_collection_slug IS DISTINCT FROM n.target_collection_slug_norm OR
    d.target_article_slug IS DISTINCT FROM n.target_article_slug_norm
  );
