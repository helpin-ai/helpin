-- Add an integration-level fallback git commit identity for agent-created commits.
ALTER TABLE git_integrations
  ADD COLUMN IF NOT EXISTS default_commit_author_name TEXT,
  ADD COLUMN IF NOT EXISTS default_commit_author_email TEXT;
