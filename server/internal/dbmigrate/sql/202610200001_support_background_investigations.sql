-- Optional research is durable work, separate from the conversation's AI owner.
-- This table is ledger-owned so its foreign keys and checks exist even when
-- API AutoMigrate is disabled.
CREATE TABLE IF NOT EXISTS support_investigations (
 id uuid PRIMARY KEY,
 workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 conversation_id uuid NOT NULL REFERENCES support_conversations(id) ON DELETE CASCADE,
 run_id uuid NOT NULL UNIQUE,
 question text NOT NULL DEFAULT '',
 brief text NOT NULL DEFAULT '',
 status text NOT NULL CHECK (status IN ('pending', 'completed')),
 due_at timestamptz NOT NULL,
 lease_until timestamptz,
 started_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_support_investigation_due ON support_investigations(due_at) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_support_investigation_conversation ON support_investigations(workspace_id, conversation_id);
