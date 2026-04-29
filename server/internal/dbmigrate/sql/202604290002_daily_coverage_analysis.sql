CREATE TABLE IF NOT EXISTS support_coverage_analysis_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL,
  window_start timestamptz NOT NULL,
  window_end timestamptz NOT NULL,
  cursor_started_at timestamptz NOT NULL,
  cursor_ended_at timestamptz NOT NULL,
  analyzer_version text NOT NULL DEFAULT 'v1',
  status text NOT NULL DEFAULT 'running',
  conversation_cnt int NOT NULL DEFAULT 0,
  gap_count int NOT NULL DEFAULT 0,
  error_message text,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  started_at timestamptz NOT NULL,
  completed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_coverage_analysis_runs_window
  ON support_coverage_analysis_runs(workspace_id, window_start, window_end, analyzer_version);

CREATE INDEX IF NOT EXISTS idx_support_coverage_analysis_runs_workspace_status
  ON support_coverage_analysis_runs(workspace_id, status, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_support_coverage_analysis_runs_cursor
  ON support_coverage_analysis_runs(workspace_id, analyzer_version, status, cursor_ended_at DESC);

CREATE TABLE IF NOT EXISTS support_coverage_conversation_analyses (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL,
  run_id uuid NOT NULL,
  conversation_id uuid NOT NULL,
  status text NOT NULL,
  has_gap boolean NOT NULL DEFAULT false,
  gap_id uuid,
  gap_kind text NOT NULL DEFAULT '',
  gap_category text NOT NULL DEFAULT '',
  primary_recommendation_type text NOT NULL DEFAULT '',
  transcript_hash text NOT NULL DEFAULT '',
  analyzer_version text NOT NULL DEFAULT 'v1',
  customer_need text NOT NULL DEFAULT '',
  ai_failure text NOT NULL DEFAULT '',
  human_resolution text NOT NULL DEFAULT '',
  decision_reason text NOT NULL DEFAULT '',
  confidence double precision NOT NULL DEFAULT 0,
  error_message text,
  raw_output jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_coverage_conversation_analyses_once
  ON support_coverage_conversation_analyses(workspace_id, conversation_id, run_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_coverage_conversation_analyses_transcript
  ON support_coverage_conversation_analyses(workspace_id, conversation_id, transcript_hash, analyzer_version);

CREATE INDEX IF NOT EXISTS idx_support_coverage_conversation_analyses_gap
  ON support_coverage_conversation_analyses(gap_id)
  WHERE gap_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_support_coverage_conversation_analyses_conversation
  ON support_coverage_conversation_analyses(workspace_id, conversation_id, created_at DESC);

CREATE TABLE IF NOT EXISTS support_ai_retrieval_traces (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL,
  conversation_id uuid NOT NULL,
  message_id uuid NOT NULL,
  search_queries jsonb NOT NULL DEFAULT '[]'::jsonb,
  results jsonb NOT NULL DEFAULT '[]'::jsonb,
  cited_source_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
  ai_confidence double precision NOT NULL DEFAULT 0,
  can_answer text,
  can_resolve text,
  failure_mode text NOT NULL DEFAULT '',
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_ai_retrieval_traces_message
  ON support_ai_retrieval_traces(workspace_id, message_id);

CREATE INDEX IF NOT EXISTS idx_support_ai_retrieval_traces_conversation
  ON support_ai_retrieval_traces(workspace_id, conversation_id, created_at DESC);

CREATE TABLE IF NOT EXISTS support_coverage_recommendations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL,
  gap_id uuid NOT NULL,
  analysis_id uuid,
  recommendation_type text NOT NULL,
  target_type text NOT NULL DEFAULT '',
  target_id text,
  target_title text NOT NULL DEFAULT '',
  target_url text NOT NULL DEFAULT '',
  priority text NOT NULL DEFAULT 'secondary',
  status text NOT NULL DEFAULT 'open',
  rationale text NOT NULL DEFAULT '',
  suggested_change text NOT NULL DEFAULT '',
  implementation_notes text NOT NULL DEFAULT '',
  suggestion_id uuid,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_support_coverage_recommendations_gap
  ON support_coverage_recommendations(gap_id, status, priority);

CREATE INDEX IF NOT EXISTS idx_support_coverage_recommendations_workspace
  ON support_coverage_recommendations(workspace_id, status, created_at DESC);
