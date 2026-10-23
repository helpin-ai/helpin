-- Records each time a person empties a support inbox view, so the inbox-zero
-- celebration can report streaks and repeat clears truthfully. This table is
-- ledger-owned: API AutoMigrate does not create it.
CREATE TABLE IF NOT EXISTS support_inbox_clears (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 scope_key text NOT NULL,
 mailbox_id uuid,
 conversation_id uuid NOT NULL REFERENCES support_conversations(id) ON DELETE CASCADE,
 local_date varchar(10) NOT NULL,
 cleared_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_support_inbox_clears_once
 ON support_inbox_clears(workspace_id, user_id, scope_key, conversation_id);
CREATE INDEX IF NOT EXISTS idx_support_inbox_clears_user_date
 ON support_inbox_clears(workspace_id, user_id, local_date);
