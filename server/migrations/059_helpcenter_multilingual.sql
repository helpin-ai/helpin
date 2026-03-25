ALTER TABLE docs_helpcenter_configs
	ADD COLUMN IF NOT EXISTS default_locale TEXT NOT NULL DEFAULT 'en',
	ADD COLUMN IF NOT EXISTS enabled_locales TEXT[] NOT NULL DEFAULT ARRAY['en']::TEXT[],
	ADD COLUMN IF NOT EXISTS show_language_switcher BOOLEAN NOT NULL DEFAULT FALSE,
	ADD COLUMN IF NOT EXISTS fallback_to_default_locale BOOLEAN NOT NULL DEFAULT TRUE;

CREATE TABLE IF NOT EXISTS docs_helpcenter_space_translations (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	space_id UUID NOT NULL REFERENCES docs_spaces(id) ON DELETE CASCADE,
	workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	locale TEXT NOT NULL,
	name TEXT NOT NULL,
	slug TEXT NOT NULL,
	description TEXT NULL,
	status TEXT NOT NULL DEFAULT 'draft',
	source_updated_at TIMESTAMPTZ NULL,
	source_synced BOOLEAN NOT NULL DEFAULT FALSE,
	published_at TIMESTAMPTZ NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT uq_docs_hc_space_locale UNIQUE (space_id, locale)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_docs_hc_space_ws_locale_slug
	ON docs_helpcenter_space_translations (workspace_id, locale, slug);

CREATE TABLE IF NOT EXISTS docs_helpcenter_collection_translations (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	collection_id UUID NOT NULL REFERENCES docs_collections(id) ON DELETE CASCADE,
	workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	space_id UUID NOT NULL REFERENCES docs_spaces(id) ON DELETE CASCADE,
	locale TEXT NOT NULL,
	name TEXT NOT NULL,
	description TEXT NULL,
	slug TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'draft',
	source_updated_at TIMESTAMPTZ NULL,
	source_synced BOOLEAN NOT NULL DEFAULT FALSE,
	published_at TIMESTAMPTZ NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT uq_docs_hc_collection_locale UNIQUE (collection_id, locale)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_docs_hc_collection_space_locale_slug
	ON docs_helpcenter_collection_translations (space_id, locale, slug);

CREATE TABLE IF NOT EXISTS docs_helpcenter_article_translations (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	document_id UUID NOT NULL REFERENCES docs_documents(id) ON DELETE CASCADE,
	workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	space_id UUID NOT NULL REFERENCES docs_spaces(id) ON DELETE CASCADE,
	collection_id UUID NULL REFERENCES docs_collections(id) ON DELETE SET NULL,
	locale TEXT NOT NULL,
	title TEXT NOT NULL,
	slug TEXT NOT NULL,
	excerpt TEXT NULL,
	content JSONB NULL,
	content_text TEXT NOT NULL DEFAULT '',
	seo_title TEXT NULL,
	seo_description TEXT NULL,
	status TEXT NOT NULL DEFAULT 'draft',
	source_updated_at TIMESTAMPTZ NULL,
	source_synced BOOLEAN NOT NULL DEFAULT FALSE,
	published_at TIMESTAMPTZ NULL,
	view_count INTEGER NOT NULL DEFAULT 0,
	helpful_count INTEGER NOT NULL DEFAULT 0,
	not_helpful_count INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT uq_docs_hc_article_locale UNIQUE (document_id, locale)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_docs_hc_article_space_locale_slug
	ON docs_helpcenter_article_translations (space_id, locale, slug);

INSERT INTO docs_helpcenter_space_translations (
	space_id,
	workspace_id,
	locale,
	name,
	slug,
	status,
	source_updated_at,
	source_synced,
	published_at
)
SELECT
	s.id,
	s.workspace_id,
	COALESCE(cfg.default_locale, 'en'),
	s.name,
	s.slug,
	'published',
	s.updated_at,
	TRUE,
	s.updated_at
FROM docs_spaces s
JOIN docs_helpcenter_configs cfg ON cfg.workspace_id = s.workspace_id
WHERE s.type = 'external_capable'
	AND s.deleted_at IS NULL
ON CONFLICT (space_id, locale) DO NOTHING;

INSERT INTO docs_helpcenter_collection_translations (
	collection_id,
	workspace_id,
	space_id,
	locale,
	name,
	description,
	slug,
	status,
	source_updated_at,
	source_synced,
	published_at
)
SELECT
	c.id,
	c.workspace_id,
	c.space_id,
	COALESCE(cfg.default_locale, 'en'),
	c.name,
	c.description,
	c.slug,
	'published',
	c.updated_at,
	TRUE,
	c.updated_at
FROM docs_collections c
JOIN docs_spaces s ON s.id = c.space_id
JOIN docs_helpcenter_configs cfg ON cfg.workspace_id = c.workspace_id
WHERE s.type = 'external_capable'
	AND c.deleted_at IS NULL
	AND s.deleted_at IS NULL
ON CONFLICT (collection_id, locale) DO NOTHING;

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
	d.id,
	d.workspace_id,
	d.space_id,
	d.collection_id,
	COALESCE(cfg.default_locale, 'en'),
	d.title,
	ha.slug,
	d.excerpt,
	COALESCE(dc.content, '{}'::jsonb),
	COALESCE(dc.content_text, ''),
	ha.seo_title,
	ha.seo_description,
	CASE
		WHEN ha.public_published_at IS NOT NULL THEN 'published'
		ELSE 'draft'
	END,
	d.updated_at,
	TRUE,
	ha.public_published_at,
	ha.view_count,
	ha.helpful_count,
	ha.not_helpful_count
FROM docs_documents d
JOIN docs_spaces s ON s.id = d.space_id
JOIN docs_helpcenter_articles ha ON ha.document_id = d.id
JOIN docs_helpcenter_configs cfg ON cfg.workspace_id = d.workspace_id
LEFT JOIN docs_contents dc ON dc.document_id = d.id
WHERE s.type = 'external_capable'
	AND d.deleted_at IS NULL
	AND s.deleted_at IS NULL
	AND ha.slug <> ''
ON CONFLICT (document_id, locale) DO NOTHING;
