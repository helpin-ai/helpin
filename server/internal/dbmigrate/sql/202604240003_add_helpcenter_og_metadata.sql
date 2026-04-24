ALTER TABLE docs_helpcenter_configs
  ADD COLUMN IF NOT EXISTS og_title TEXT,
  ADD COLUMN IF NOT EXISTS og_description TEXT,
  ADD COLUMN IF NOT EXISTS og_image_url TEXT,
  ADD COLUMN IF NOT EXISTS og_image_alt TEXT;

ALTER TABLE docs_helpcenter_articles
  ADD COLUMN IF NOT EXISTS og_title TEXT,
  ADD COLUMN IF NOT EXISTS og_description TEXT,
  ADD COLUMN IF NOT EXISTS og_image_url TEXT,
  ADD COLUMN IF NOT EXISTS og_image_alt TEXT;

ALTER TABLE docs_helpcenter_article_translations
  ADD COLUMN IF NOT EXISTS og_title TEXT,
  ADD COLUMN IF NOT EXISTS og_description TEXT,
  ADD COLUMN IF NOT EXISTS og_image_url TEXT,
  ADD COLUMN IF NOT EXISTS og_image_alt TEXT;

ALTER TABLE docs_helpcenter_article_publications
  ADD COLUMN IF NOT EXISTS og_title TEXT,
  ADD COLUMN IF NOT EXISTS og_description TEXT,
  ADD COLUMN IF NOT EXISTS og_image_url TEXT,
  ADD COLUMN IF NOT EXISTS og_image_alt TEXT;
