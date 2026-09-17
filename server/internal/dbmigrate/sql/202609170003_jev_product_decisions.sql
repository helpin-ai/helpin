CREATE TABLE IF NOT EXISTS jev_decision_attempts (
 id uuid PRIMARY KEY,
 workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 feature text NOT NULL CHECK (feature IN ('meeting_routing','coverage_classification','coverage_topic_matching','automation_condition','answer_evidence')),
 source_id text NOT NULL,
 input_hash text NOT NULL,
 mode text NOT NULL CHECK (mode IN ('shadow','primary')),
 status text NOT NULL CHECK (status IN ('pending','ready','failed')),
 outcome jsonb NOT NULL DEFAULT '{}',
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_jev_decision_cache ON jev_decision_attempts(workspace_id,feature,source_id,input_hash,created_at DESC);
CREATE INDEX IF NOT EXISTS idx_jev_decision_daily ON jev_decision_attempts(workspace_id,feature,created_at);
