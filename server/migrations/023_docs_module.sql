-- 023_docs_module.sql
-- Docs module: spaces, collections, documents, content, versions, links,
-- help center config, help center articles, slug aliases, review queue,
-- article feedback, comments.
-- Reference documentation — GORM AutoMigrate handles schema creation.

-- ─── docs_spaces ────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS docs_spaces (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    team_id         UUID,
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL,
    icon            TEXT,
    visibility      TEXT NOT NULL DEFAULT 'workspace_wide',
    type            TEXT NOT NULL DEFAULT 'internal',
    restrict_to_owners BOOLEAN NOT NULL DEFAULT FALSE,
    default_review_days INTEGER,
    is_system       BOOLEAN NOT NULL DEFAULT FALSE,
    position        INTEGER NOT NULL DEFAULT 0,
    created_by      UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_docs_space_ws_slug ON docs_spaces (workspace_id, slug);
CREATE INDEX IF NOT EXISTS idx_docs_space_ws_team ON docs_spaces (workspace_id, team_id);
CREATE INDEX IF NOT EXISTS idx_docs_space_ws_pos ON docs_spaces (workspace_id, position);

-- ─── docs_collections ───────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS docs_collections (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id        UUID NOT NULL,
    workspace_id    UUID NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT,
    icon            TEXT,
    position        INTEGER NOT NULL DEFAULT 0,
    created_by      UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_docs_collection_space_pos ON docs_collections (space_id, position);

-- ─── docs_documents ─────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS docs_documents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    space_id        UUID NOT NULL,
    collection_id   UUID,
    title           TEXT NOT NULL,
    doc_type        TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'draft',
    visibility      TEXT NOT NULL DEFAULT 'workspace_wide',
    owner_id        UUID,
    team_id         UUID NOT NULL,
    template_key    TEXT,
    excerpt         TEXT,
    icon            TEXT,
    tags            TEXT[],
    is_pinned       BOOLEAN NOT NULL DEFAULT FALSE,
    last_reviewed_at TIMESTAMPTZ,
    next_review_at  TIMESTAMPTZ,
    published_at    TIMESTAMPTZ,
    created_by      UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_docs_doc_ws_space_status ON docs_documents (workspace_id, space_id, status);
CREATE INDEX IF NOT EXISTS idx_docs_doc_ws_type_status ON docs_documents (workspace_id, doc_type, status);
CREATE INDEX IF NOT EXISTS idx_docs_doc_owner_review ON docs_documents (owner_id, next_review_at);
CREATE INDEX IF NOT EXISTS idx_docs_doc_ws_team_updated ON docs_documents (workspace_id, team_id, updated_at);

-- ─── docs_contents ──────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS docs_contents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id     UUID NOT NULL UNIQUE,
    content         JSONB,
    content_text    TEXT,
    word_count      INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── docs_versions ──────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS docs_versions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id     UUID NOT NULL,
    content         JSONB,
    content_text    TEXT,
    snapshot_label  TEXT,
    created_by      UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_docs_version_doc_created ON docs_versions (document_id, created_at DESC);

-- ─── docs_links ─────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS docs_links (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id     UUID NOT NULL,
    document_id      UUID NOT NULL,
    linked_object_type TEXT NOT NULL,
    linked_object_id UUID NOT NULL,
    link_context     TEXT NOT NULL DEFAULT 'attached',
    created_by       UUID NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_docs_link_doc_type ON docs_links (document_id, linked_object_type);
CREATE INDEX IF NOT EXISTS idx_docs_link_obj ON docs_links (linked_object_type, linked_object_id);
CREATE INDEX IF NOT EXISTS idx_docs_link_ws_obj ON docs_links (workspace_id, linked_object_type, linked_object_id);

-- ─── docs_helpcenter_configs ────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS docs_helpcenter_configs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL UNIQUE,
    subdomain       TEXT NOT NULL,
    custom_domain   TEXT,
    brand_name      TEXT NOT NULL,
    brand_logo_url  TEXT,
    brand_color     TEXT NOT NULL DEFAULT '#000000',
    is_published    BOOLEAN NOT NULL DEFAULT FALSE,
    seo_title       TEXT,
    seo_description TEXT,
    support_email   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── docs_helpcenter_articles ───────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS docs_helpcenter_articles (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id      UUID NOT NULL UNIQUE,
    seo_title        TEXT,
    seo_description  TEXT,
    helpful_count    INTEGER NOT NULL DEFAULT 0,
    not_helpful_count INTEGER NOT NULL DEFAULT 0,
    view_count       INTEGER NOT NULL DEFAULT 0,
    public_published_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_docs_hc_article_published ON docs_helpcenter_articles (public_published_at);

-- ─── docs_slug_aliases ──────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS docs_slug_aliases (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    document_id     UUID NOT NULL,
    old_slug        TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_docs_slug_alias_ws_slug ON docs_slug_aliases (workspace_id, old_slug);
CREATE INDEX IF NOT EXISTS idx_docs_slug_alias_doc ON docs_slug_aliases (document_id);

-- ─── docs_review_queue (schema only) ────────────────────────────────────────

CREATE TABLE IF NOT EXISTS docs_review_queue (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    document_id     UUID NOT NULL,
    assigned_to     UUID,
    status          TEXT NOT NULL DEFAULT 'pending',
    due_at          TIMESTAMPTZ,
    created_by      UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── docs_article_feedback (schema only) ────────────────────────────────────

CREATE TABLE IF NOT EXISTS docs_article_feedback (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id     UUID NOT NULL,
    is_helpful      BOOLEAN NOT NULL,
    comment         TEXT,
    session_id      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── docs_comments (schema only) ────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS docs_comments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id     UUID NOT NULL,
    parent_id       UUID,
    author_id       UUID NOT NULL,
    content         TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ
);

-- ─── workspace_teams: add docs_publisher_enabled ────────────────────────────

ALTER TABLE workspace_teams ADD COLUMN IF NOT EXISTS docs_publisher_enabled BOOLEAN NOT NULL DEFAULT FALSE;
