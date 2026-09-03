CREATE TABLE IF NOT EXISTS ai_action_executions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id text NOT NULL DEFAULT '',
  action_key text NOT NULL,
  policy_version text NOT NULL,
  feature_key text NOT NULL,
  category text NOT NULL,
  origin text NOT NULL,
  modality text NOT NULL,
  provider text NOT NULL,
  model text NOT NULL,
  idempotency_key text NOT NULL,
  attempt int NOT NULL CHECK (attempt > 0),
  status text NOT NULL CHECK (status IN ('running', 'succeeded', 'failed')),
  failure_class text NOT NULL DEFAULT '',
  failure_message text NOT NULL DEFAULT '',
  input_tokens int NOT NULL DEFAULT 0,
  output_tokens int NOT NULL DEFAULT 0,
  reasoning_tokens int NOT NULL DEFAULT 0,
  cached_input_tokens int NOT NULL DEFAULT 0,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  started_at timestamptz NOT NULL,
  completed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (idempotency_key, attempt)
);

CREATE INDEX IF NOT EXISTS idx_ai_action_executions_workspace_started
  ON ai_action_executions(workspace_id, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_ai_action_executions_action_status
  ON ai_action_executions(action_key, status, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_ai_action_executions_origin_started
  ON ai_action_executions(origin, started_at DESC);
