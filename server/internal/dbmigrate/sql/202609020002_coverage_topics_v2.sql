CREATE TABLE IF NOT EXISTS coverage_batches (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL,
  window_start timestamptz NOT NULL,
  window_end timestamptz NOT NULL,
  analyzer_version text NOT NULL,
  policy_version text NOT NULL,
  status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'succeeded', 'partial_failed', 'failed')),
  cursor text NOT NULL DEFAULT '',
  lease_owner text NOT NULL DEFAULT '',
  lease_expires_at timestamptz,
  candidate_count int NOT NULL DEFAULT 0 CHECK (candidate_count >= 0),
  succeeded_count int NOT NULL DEFAULT 0 CHECK (succeeded_count >= 0),
  retryable_count int NOT NULL DEFAULT 0 CHECK (retryable_count >= 0),
  dead_letter_count int NOT NULL DEFAULT 0 CHECK (dead_letter_count >= 0),
  failure_class text NOT NULL DEFAULT '',
  failure_message text NOT NULL DEFAULT '',
  correlation_id text NOT NULL DEFAULT '',
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  started_at timestamptz,
  completed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_coverage_batches_logical UNIQUE (workspace_id, window_start, window_end, analyzer_version, policy_version),
  CONSTRAINT uq_coverage_batches_workspace_id UNIQUE (workspace_id, id),
  CONSTRAINT ck_coverage_batches_window CHECK (window_end > window_start)
);

CREATE INDEX IF NOT EXISTS idx_coverage_batches_workspace_status
  ON coverage_batches (workspace_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS coverage_analysis_attempts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL,
  batch_id uuid NOT NULL,
  logical_work_key text NOT NULL,
  source_kind text NOT NULL CHECK (source_kind IN ('conversation', 'widget_search')),
  source_id text NOT NULL,
  segment_id text NOT NULL DEFAULT '',
  content_hash text NOT NULL,
  analyzer_version text NOT NULL,
  policy_version text NOT NULL,
  attempt int NOT NULL CHECK (attempt > 0),
  status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'leased', 'retryable', 'succeeded', 'dead_letter', 'paused_configuration')),
  stage text NOT NULL DEFAULT 'candidate_query',
  lease_owner text NOT NULL DEFAULT '',
  lease_expires_at timestamptz,
  retry_at timestamptz,
  retry_budget_used int NOT NULL DEFAULT 0 CHECK (retry_budget_used >= 0),
  failure_class text NOT NULL DEFAULT '',
  failure_message text NOT NULL DEFAULT '',
  ai_execution_id uuid,
  correlation_id text NOT NULL DEFAULT '',
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  started_at timestamptz,
  completed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT fk_coverage_attempt_batch_workspace FOREIGN KEY (workspace_id, batch_id)
    REFERENCES coverage_batches (workspace_id, id),
  CONSTRAINT uq_coverage_attempt_number UNIQUE (workspace_id, logical_work_key, attempt),
  CONSTRAINT uq_coverage_attempt_workspace_id UNIQUE (workspace_id, id)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_coverage_attempt_active_lease
  ON coverage_analysis_attempts (workspace_id, logical_work_key)
  WHERE status = 'leased';
CREATE INDEX IF NOT EXISTS idx_coverage_attempt_claim
  ON coverage_analysis_attempts (status, retry_at, lease_expires_at, created_at);
CREATE INDEX IF NOT EXISTS idx_coverage_attempt_batch
  ON coverage_analysis_attempts (workspace_id, batch_id, status);

CREATE TABLE IF NOT EXISTS coverage_findings (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL,
  logical_work_key text NOT NULL,
  analysis_attempt_id uuid NOT NULL,
  source_kind text NOT NULL CHECK (source_kind IN ('conversation', 'widget_search')),
  source_id text NOT NULL,
  conversation_id uuid,
  customer_id uuid,
  customer_need text NOT NULL,
  ai_answer text NOT NULL DEFAULT '',
  ai_failure text NOT NULL DEFAULT '',
  human_answer text NOT NULL DEFAULT '',
  fix_type text NOT NULL,
  fix_target text NOT NULL,
  rationale text NOT NULL,
  suggested_change text NOT NULL,
  confidence double precision NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  is_current boolean NOT NULL DEFAULT true,
  embedding_status text NOT NULL DEFAULT 'pending' CHECK (embedding_status IN ('pending', 'ready', 'retryable', 'failed')),
  assignment_status text NOT NULL DEFAULT 'pending' CHECK (assignment_status IN ('pending', 'assigned', 'review', 'retryable', 'failed')),
  embedding vector(1536),
  embedding_provider text NOT NULL DEFAULT '',
  embedding_model text NOT NULL DEFAULT '',
  embedding_version text NOT NULL DEFAULT '',
  embedding_dimensions int NOT NULL DEFAULT 0 CHECK (embedding_dimensions >= 0),
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  superseded_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT fk_coverage_finding_attempt_workspace FOREIGN KEY (workspace_id, analysis_attempt_id)
    REFERENCES coverage_analysis_attempts (workspace_id, id),
  CONSTRAINT uq_coverage_findings_workspace_id UNIQUE (workspace_id, id),
  CONSTRAINT ck_coverage_finding_answer CHECK (ai_answer <> '' OR ai_failure <> '')
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_coverage_findings_current
  ON coverage_findings (workspace_id, logical_work_key)
  WHERE is_current;
CREATE INDEX IF NOT EXISTS idx_coverage_findings_source
  ON coverage_findings (workspace_id, source_kind, source_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_coverage_findings_downstream
  ON coverage_findings (workspace_id, embedding_status, assignment_status, created_at);

CREATE TABLE IF NOT EXISTS coverage_topics (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL,
  canonical_key text NOT NULL,
  title text NOT NULL,
  customer_need text NOT NULL,
  status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'done', 'archived')),
  finding_count int NOT NULL DEFAULT 0 CHECK (finding_count >= 0),
  conversation_count int NOT NULL DEFAULT 0 CHECK (conversation_count >= 0),
  customer_count int NOT NULL DEFAULT 0 CHECK (customer_count >= 0),
  assignment_policy text NOT NULL,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_coverage_topics_key UNIQUE (workspace_id, canonical_key),
  CONSTRAINT uq_coverage_topics_workspace_id UNIQUE (workspace_id, id)
);

CREATE INDEX IF NOT EXISTS idx_coverage_topics_rank
  ON coverage_topics (workspace_id, status, conversation_count DESC, customer_count DESC, updated_at DESC);

CREATE TABLE IF NOT EXISTS coverage_assignment_attempts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL,
  finding_id uuid NOT NULL,
  attempt int NOT NULL CHECK (attempt > 0),
  policy_version text NOT NULL,
  candidate_topic_id uuid,
  similarity double precision NOT NULL DEFAULT 0,
  compatibility double precision NOT NULL DEFAULT 0,
  outcome text NOT NULL CHECK (outcome IN ('attach', 'create', 'review', 'failed')),
  failure_class text NOT NULL DEFAULT '',
  failure_message text NOT NULL DEFAULT '',
  ai_execution_id uuid,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT fk_coverage_assignment_finding_workspace FOREIGN KEY (workspace_id, finding_id)
    REFERENCES coverage_findings (workspace_id, id),
  CONSTRAINT fk_coverage_assignment_topic_workspace FOREIGN KEY (workspace_id, candidate_topic_id)
    REFERENCES coverage_topics (workspace_id, id),
  CONSTRAINT uq_coverage_assignment_attempt UNIQUE (workspace_id, finding_id, attempt),
  CONSTRAINT uq_coverage_assignment_workspace_id UNIQUE (workspace_id, id)
);

CREATE TABLE IF NOT EXISTS coverage_topic_memberships (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL,
  finding_id uuid NOT NULL,
  topic_id uuid NOT NULL,
  decision_source text NOT NULL CHECK (decision_source IN ('automatic', 'manual', 'promotion', 'rebuild')),
  confidence double precision NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  policy_version text NOT NULL,
  assignment_attempt_id uuid,
  actor_id uuid,
  valid_from timestamptz NOT NULL,
  valid_to timestamptz,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT fk_coverage_membership_finding_workspace FOREIGN KEY (workspace_id, finding_id)
    REFERENCES coverage_findings (workspace_id, id),
  CONSTRAINT fk_coverage_membership_topic_workspace FOREIGN KEY (workspace_id, topic_id)
    REFERENCES coverage_topics (workspace_id, id),
  CONSTRAINT fk_coverage_membership_assignment_workspace FOREIGN KEY (workspace_id, assignment_attempt_id)
    REFERENCES coverage_assignment_attempts (workspace_id, id),
  CONSTRAINT ck_coverage_membership_window CHECK (valid_to IS NULL OR valid_to > valid_from)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_coverage_membership_current
  ON coverage_topic_memberships (workspace_id, finding_id)
  WHERE valid_to IS NULL;
CREATE INDEX IF NOT EXISTS idx_coverage_membership_topic_current
  ON coverage_topic_memberships (workspace_id, topic_id, valid_from DESC)
  WHERE valid_to IS NULL;

CREATE TABLE IF NOT EXISTS coverage_unreviewed_signals (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL,
  source_kind text NOT NULL CHECK (source_kind IN ('conversation', 'widget_search')),
  source_id text NOT NULL,
  session_id text NOT NULL DEFAULT '',
  normalized_query text NOT NULL,
  signal_key text NOT NULL,
  meaningful_tokens int NOT NULL DEFAULT 0 CHECK (meaningful_tokens >= 0),
  status text NOT NULL DEFAULT 'unreviewed' CHECK (status IN ('unreviewed', 'promoted', 'attached', 'dismissed')),
  confidence double precision NOT NULL DEFAULT 0 CHECK (confidence >= 0 AND confidence <= 1),
  finding_id uuid,
  topic_id uuid,
  reviewed_by uuid,
  reviewed_at timestamptz,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  observed_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_coverage_signal_key UNIQUE (workspace_id, signal_key),
  CONSTRAINT fk_coverage_signal_finding_workspace FOREIGN KEY (workspace_id, finding_id)
    REFERENCES coverage_findings (workspace_id, id),
  CONSTRAINT fk_coverage_signal_topic_workspace FOREIGN KEY (workspace_id, topic_id)
    REFERENCES coverage_topics (workspace_id, id)
);

CREATE INDEX IF NOT EXISTS idx_coverage_signals_review
  ON coverage_unreviewed_signals (workspace_id, status, observed_at DESC);

CREATE TABLE IF NOT EXISTS coverage_rebuild_audits (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL,
  status text NOT NULL CHECK (status IN ('applied', 'rolled_back')),
  analyzer_version text NOT NULL,
  policy_version text NOT NULL,
  legacy_gap_count int NOT NULL DEFAULT 0,
  evidence_count int NOT NULL DEFAULT 0,
  conversation_count int NOT NULL DEFAULT 0,
  search_count int NOT NULL DEFAULT 0,
  queued_work_count int NOT NULL DEFAULT 0,
  qualified_search_count int NOT NULL DEFAULT 0,
  legacy_status_snapshot jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  rolled_back_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_coverage_rebuild_audits_workspace_created
  ON coverage_rebuild_audits (workspace_id, created_at DESC);

ALTER TABLE support_coverage_gaps
  DROP CONSTRAINT IF EXISTS support_coverage_gaps_status_check;
ALTER TABLE support_coverage_gaps
  ADD CONSTRAINT support_coverage_gaps_status_check
  CHECK (status IN ('open', 'done', 'rejected', 'drafted', 'fixed', 'ignored', 'merged', 'human_only', 'archived_v1'));
