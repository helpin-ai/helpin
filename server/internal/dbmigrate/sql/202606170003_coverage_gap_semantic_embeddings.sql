ALTER TABLE support_coverage_gaps
  ADD COLUMN IF NOT EXISTS embedding vector(1536),
  ADD COLUMN IF NOT EXISTS embedding_provider text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS embedding_model text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS embedding_version text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS embedding_dimensions integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS embedding_text_hash text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS embedding_updated_at timestamptz,
  ADD COLUMN IF NOT EXISTS nearest_content_score double precision NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS nearest_content_document_id uuid,
  ADD COLUMN IF NOT EXISTS nearest_content_title text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS nearest_content_checked_at timestamptz,
  ADD COLUMN IF NOT EXISTS impact_score double precision NOT NULL DEFAULT 0;

ALTER TABLE support_coverage_conversation_analyses
  ADD COLUMN IF NOT EXISTS canonical_title text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS embedding vector(1536),
  ADD COLUMN IF NOT EXISTS embedding_provider text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS embedding_model text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS embedding_version text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS embedding_dimensions integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS embedding_text_hash text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS embedding_updated_at timestamptz,
  ADD COLUMN IF NOT EXISTS materialization_metadata jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE support_gap_evidence
  ADD COLUMN IF NOT EXISTS source_key text NOT NULL DEFAULT '';

ALTER TABLE docs_chunks
  ADD COLUMN IF NOT EXISTS embedding_provider text NOT NULL DEFAULT 'openai',
  ADD COLUMN IF NOT EXISTS embedding_model text NOT NULL DEFAULT 'text-embedding-3-small',
  ADD COLUMN IF NOT EXISTS embedding_version text NOT NULL DEFAULT 'content-chunk-v1',
  ADD COLUMN IF NOT EXISTS embedding_dimensions integer NOT NULL DEFAULT 1536;

ALTER TABLE support_content_chunks
  ADD COLUMN IF NOT EXISTS embedding_provider text NOT NULL DEFAULT 'openai',
  ADD COLUMN IF NOT EXISTS embedding_model text NOT NULL DEFAULT 'text-embedding-3-small',
  ADD COLUMN IF NOT EXISTS embedding_version text NOT NULL DEFAULT 'content-chunk-v1',
  ADD COLUMN IF NOT EXISTS embedding_dimensions integer NOT NULL DEFAULT 1536;

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_gap_evidence_workspace_source_key
  ON support_gap_evidence(workspace_id, source_key)
  WHERE source_key <> '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_coverage_conversation_analyses_transcript
  ON support_coverage_conversation_analyses(workspace_id, conversation_id, transcript_hash, analyzer_version);

CREATE INDEX IF NOT EXISTS idx_support_coverage_gaps_embedding_ivfflat
  ON support_coverage_gaps
  USING ivfflat (embedding vector_cosine_ops)
  WITH (lists = 100)
  WHERE embedding IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_support_coverage_analyses_embedding_ivfflat
  ON support_coverage_conversation_analyses
  USING ivfflat (embedding vector_cosine_ops)
  WITH (lists = 100)
  WHERE embedding IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_support_coverage_analyses_unmaterialized
  ON support_coverage_conversation_analyses(workspace_id, run_id, created_at)
  WHERE has_gap = true AND gap_id IS NULL;
