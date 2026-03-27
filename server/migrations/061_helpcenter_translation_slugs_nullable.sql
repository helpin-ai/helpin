ALTER TABLE docs_helpcenter_space_translations
  ALTER COLUMN slug DROP NOT NULL;

ALTER TABLE docs_helpcenter_collection_translations
  ALTER COLUMN slug DROP NOT NULL;

ALTER TABLE docs_helpcenter_article_translations
  ALTER COLUMN slug DROP NOT NULL;

UPDATE docs_helpcenter_space_translations
SET slug = NULL
WHERE slug = '';

UPDATE docs_helpcenter_collection_translations
SET slug = NULL
WHERE slug = '';

UPDATE docs_helpcenter_article_translations
SET slug = NULL
WHERE slug = '';
