CREATE TABLE IF NOT EXISTS command_bar_threads (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
  title TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'open',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_command_bar_threads_workspace_actor_updated
  ON command_bar_threads(workspace_id, actor_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS command_bar_messages (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  thread_id UUID NOT NULL REFERENCES command_bar_threads(id) ON DELETE CASCADE,
  workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
  role TEXT NOT NULL,
  content TEXT NOT NULL,
  page_context JSONB,
  proposal_json JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_command_bar_messages_thread_created
  ON command_bar_messages(thread_id, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_command_bar_messages_workspace_created
  ON command_bar_messages(workspace_id, created_at DESC);
