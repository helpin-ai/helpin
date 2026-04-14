-- Remove collection and article slug uniqueness now that PublicIDs
-- are the durable identity for public help center URLs.
-- Space slugs remain unique (spaces do not have PublicIDs yet).

-- Collections: drop workspace-wide slug uniqueness, add non-unique lookup index.
DROP INDEX IF EXISTS idx_docs_collection_ws_slug;
DROP INDEX IF EXISTS idx_docs_collections_ws_slug_alive;

CREATE INDEX IF NOT EXISTS idx_docs_collections_ws_slug_lookup
  ON docs_collections (workspace_id, slug)
  WHERE deleted_at IS NULL;

-- Collection translations: drop unique slug constraint, add lookup index.
DROP INDEX IF EXISTS idx_docs_hc_collection_space_locale_slug;

CREATE INDEX IF NOT EXISTS idx_docs_hc_collection_space_locale_slug_lookup
  ON docs_helpcenter_collection_translations (space_id, locale, slug);

-- Article translations: drop unique slug constraint, add lookup index.
DROP INDEX IF EXISTS idx_docs_hc_article_space_locale_slug;

CREATE INDEX IF NOT EXISTS idx_docs_hc_article_space_locale_slug_lookup
  ON docs_helpcenter_article_translations (space_id, locale, slug);

-- Live article publications: drop unique slug constraint, add lookup index.
DROP INDEX IF EXISTS idx_docs_hc_article_pub_space_locale_slug;

CREATE INDEX IF NOT EXISTS idx_docs_hc_article_pub_space_locale_slug_lookup
  ON docs_helpcenter_article_publications (space_id, locale, slug);
