CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE IF NOT EXISTS docs_helpcenter_search_entries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    document_id uuid NOT NULL,
    locale varchar(16) NOT NULL,
    entry_key text NOT NULL,
    entry_type varchar(16) NOT NULL,
    content text NOT NULL,
    section_title text,
    anchor text,
    position integer NOT NULL DEFAULT 0,
    rank_weight double precision NOT NULL DEFAULT 1,
    search_config varchar(32) NOT NULL DEFAULT 'simple',
    search_vector tsvector NOT NULL DEFAULT ''::tsvector,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT docs_helpcenter_search_entry_type_check
        CHECK (entry_type IN ('title', 'excerpt', 'heading', 'body'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_docs_hc_search_doc_locale_key
    ON docs_helpcenter_search_entries (document_id, locale, entry_key);

CREATE INDEX IF NOT EXISTS idx_docs_hc_search_workspace_locale
    ON docs_helpcenter_search_entries (workspace_id, locale);

CREATE INDEX IF NOT EXISTS idx_docs_hc_search_document
    ON docs_helpcenter_search_entries (document_id);

CREATE INDEX IF NOT EXISTS idx_docs_hc_search_entry_type
    ON docs_helpcenter_search_entries (entry_type);

CREATE INDEX IF NOT EXISTS idx_docs_hc_search_vector
    ON docs_helpcenter_search_entries USING gin (search_vector);

CREATE INDEX IF NOT EXISTS idx_docs_hc_search_content_trgm
    ON docs_helpcenter_search_entries USING gin (content gin_trgm_ops);
