-- Workspace skill overrides keep the built-in default so it can be restored.
ALTER TABLE workspace_skills ADD COLUMN IF NOT EXISTS is_customized BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE workspace_skills ADD COLUMN IF NOT EXISTS default_content JSONB;
ALTER TABLE workspace_skills ADD COLUMN IF NOT EXISTS default_package_object_key TEXT NOT NULL DEFAULT '';
