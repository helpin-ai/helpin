CREATE TABLE IF NOT EXISTS docs_helpcenter_article_publications (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  document_id UUID NOT NULL,
  workspace_id UUID NOT NULL,
  space_id UUID NOT NULL,
  collection_id UUID NULL,
  locale TEXT NOT NULL,
  title TEXT NOT NULL,
  slug TEXT NOT NULL,
  excerpt TEXT NULL,
  content JSONB NULL,
  content_text TEXT NOT NULL DEFAULT '',
  seo_title TEXT NULL,
  seo_description TEXT NULL,
  published_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_docs_hc_article_pub_doc_locale
  ON docs_helpcenter_article_publications (document_id, locale);

CREATE UNIQUE INDEX IF NOT EXISTS idx_docs_hc_article_pub_space_locale_slug
  ON docs_helpcenter_article_publications (space_id, locale, slug);

CREATE INDEX IF NOT EXISTS idx_docs_hc_article_pub_collection_locale
  ON docs_helpcenter_article_publications (collection_id, locale);

CREATE INDEX IF NOT EXISTS idx_docs_hc_article_pub_published_at
  ON docs_helpcenter_article_publications (published_at);
