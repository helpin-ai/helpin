-- Existing and new workspaces start with detailed previews enabled. Reapplying
-- must preserve explicit opt-outs; never backfill false values to true.
ALTER TABLE workspaces
    ADD COLUMN IF NOT EXISTS link_previews_enabled boolean NOT NULL DEFAULT true;
