-- Migration: git_integrations_deleted_at
-- Add soft-delete support for org-scoped integration lifecycle.

ALTER TABLE git_integrations
  ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
