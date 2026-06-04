-- Convert legacy "image" TipTap node types to "resizableImage" so the internal
-- docs editor can render them. Earlier versions of the Nextra importer produced
-- standard TipTap `image` nodes, but the editor registers only the custom
-- `resizableImage` extension. The public help center renderer falls back to
-- rendering either, which is why the same docs showed images publicly but not
-- in the editor.
--
-- Idempotent: replacing `"type": "image"` with `"type": "resizableImage"` is
-- a no-op on rows that already use the correct type.

UPDATE docs_contents
SET content = replace(content::text, '"type": "image"', '"type": "resizableImage"')::jsonb
WHERE content::text LIKE '%"type": "image"%';

UPDATE docs_versions
SET content = replace(content::text, '"type": "image"', '"type": "resizableImage"')::jsonb
WHERE content::text LIKE '%"type": "image"%';

UPDATE docs_helpcenter_article_publications
SET content = replace(content::text, '"type": "image"', '"type": "resizableImage"')::jsonb
WHERE content::text LIKE '%"type": "image"%';

UPDATE docs_helpcenter_article_translations
SET content = replace(content::text, '"type": "image"', '"type": "resizableImage"')::jsonb
WHERE content::text LIKE '%"type": "image"%';
