-- Speeds up the epic-visible delivery view, which lists command-bar plans by
-- the entity encoded in page_context (entity_type='epic', entity_id=<epicId>).
CREATE INDEX IF NOT EXISTS idx_command_bar_plans_ws_entity
  ON command_bar_plans (workspace_id, (page_context->>'entity_id'));
