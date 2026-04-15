-- Docs Coverage Loop v1: shared support events, gap engine, evidence,
-- suggestions, snapshots, and digest delivery tracking.
-- All tables are idempotent (IF NOT EXISTS).

-- Shared operational support events (append-only).
-- Used by Coverage, future analytics, and intelligence features.
CREATE TABLE IF NOT EXISTS support_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    conversation_id UUID,
    message_id UUID,
    widget_session_id UUID,
    anonymous_id TEXT,
    document_id UUID,
    article_id UUID,
    article_public_id TEXT,
    actor_type TEXT NOT NULL DEFAULT '',
    channel TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',
    issue_key TEXT NOT NULL DEFAULT '',
    issue_summary TEXT NOT NULL DEFAULT '',
    failure_mode TEXT NOT NULL DEFAULT '',
    source_signal TEXT NOT NULL DEFAULT '',
    can_answer TEXT,
    can_resolve TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_support_events_workspace_time
  ON support_events (workspace_id, occurred_at DESC);

CREATE INDEX IF NOT EXISTS idx_support_events_workspace_type_time
  ON support_events (workspace_id, event_type, occurred_at DESC);

CREATE INDEX IF NOT EXISTS idx_support_events_conversation_time
  ON support_events (conversation_id, occurred_at DESC)
  WHERE conversation_id IS NOT NULL;

-- Stable topic clusters.
CREATE TABLE IF NOT EXISTS support_coverage_topics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    issue_key TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    gap_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_coverage_topics_workspace_issue_key
  ON support_coverage_topics (workspace_id, issue_key);

-- Coverage gaps (durable blockers).
CREATE TABLE IF NOT EXISTS support_coverage_gaps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    topic_id UUID,
    dedupe_key TEXT NOT NULL,
    gap_category TEXT NOT NULL DEFAULT 'unknown',
    v1_gap_type TEXT NOT NULL DEFAULT 'needs_review',
    title TEXT NOT NULL DEFAULT '',
    issue_key TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'open',
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0,
    evidence_count INT NOT NULL DEFAULT 0,
    failure_mode TEXT NOT NULL DEFAULT '',
    source_signal TEXT NOT NULL DEFAULT '',
    can_answer TEXT,
    can_resolve TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_coverage_gaps_workspace_dedupe
  ON support_coverage_gaps (workspace_id, dedupe_key)
  WHERE status != 'merged';

CREATE INDEX IF NOT EXISTS idx_support_coverage_gaps_workspace_status_seen
  ON support_coverage_gaps (workspace_id, status, last_seen_at DESC);

CREATE INDEX IF NOT EXISTS idx_support_coverage_gaps_topic
  ON support_coverage_gaps (topic_id)
  WHERE topic_id IS NOT NULL;

-- Gap evidence (links gaps to conversations/searches/articles).
CREATE TABLE IF NOT EXISTS support_gap_evidence (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    gap_id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    evidence_type TEXT NOT NULL,
    conversation_id UUID,
    message_id UUID,
    widget_session_id UUID,
    document_id UUID,
    article_public_id TEXT,
    source_signal TEXT NOT NULL DEFAULT '',
    excerpt TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_support_gap_evidence_gap
  ON support_gap_evidence (gap_id, created_at DESC);

-- Gap suggestions (proposed fixes).
CREATE TABLE IF NOT EXISTS support_gap_suggestions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    gap_id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    suggestion_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    title TEXT NOT NULL DEFAULT '',
    content JSONB,
    evidence_summary TEXT NOT NULL DEFAULT '',
    target_space_id UUID,
    target_collection_id UUID,
    target_document_id UUID,
    result_document_id UUID,
    result_article_id UUID,
    applied_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_support_gap_suggestions_gap
  ON support_gap_suggestions (gap_id);

-- Gap-article join table (N:N).
CREATE TABLE IF NOT EXISTS support_coverage_gap_articles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    gap_id UUID NOT NULL,
    document_id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_coverage_gap_articles_unique
  ON support_coverage_gap_articles (gap_id, document_id);

-- Pre-computed coverage snapshots for summary pages.
CREATE TABLE IF NOT EXISTS support_coverage_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    snapshot_at TIMESTAMPTZ NOT NULL,
    metrics JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_support_coverage_snapshots_workspace
  ON support_coverage_snapshots (workspace_id, snapshot_at DESC);

-- Digest delivery tracking (prevents duplicate weekly emails).
CREATE TABLE IF NOT EXISTS support_coverage_digest_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    week_start TIMESTAMPTZ NOT NULL,
    recipient_user_id UUID NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_coverage_digest_deliveries_workspace_week_recipient
  ON support_coverage_digest_deliveries (workspace_id, week_start, recipient_user_id);
