-- Enforce workspace-scoped uniqueness for message shortcut triggers.
CREATE UNIQUE INDEX IF NOT EXISTS idx_support_canned_responses_ws_short_code
  ON support_canned_responses(workspace_id, short_code);
