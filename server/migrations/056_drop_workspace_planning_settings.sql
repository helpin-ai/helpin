ALTER TABLE workspace_settings
  DROP COLUMN IF EXISTS planning_methodology,
  DROP COLUMN IF EXISTS planning_web_search_enabled,
  DROP COLUMN IF EXISTS planning_web_search_provider;
