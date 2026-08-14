-- Support the public collection-page projection without scanning all article
-- translations or all collection translations in a workspace.

CREATE INDEX IF NOT EXISTS idx_docs_hc_article_translation_collection_locale_read
  ON docs_helpcenter_article_translations (collection_id, locale, document_id);

CREATE INDEX IF NOT EXISTS idx_docs_hc_collection_translation_ws_locale_slug_read
  ON docs_helpcenter_collection_translations (workspace_id, locale, slug)
  WHERE status = 'published' AND published_at IS NOT NULL;

-- Keep the live publication lookup explicit even on databases where the
-- legacy publication index was created with a different column order.
CREATE INDEX IF NOT EXISTS idx_docs_hc_article_pub_collection_locale_read
  ON docs_helpcenter_article_publications (collection_id, locale, document_id)
  WHERE collection_id IS NOT NULL;
