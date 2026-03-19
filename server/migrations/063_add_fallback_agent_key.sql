-- Add fallback_agent_key to flow_template_nodes.
-- When a node's agent_input_key is missing from flow input, the engine
-- copies the value from this other input key instead.
ALTER TABLE flow_template_nodes ADD COLUMN IF NOT EXISTS fallback_agent_key TEXT;

-- Set story_plan node to fall back to spec_planner_agent_id when
-- story_planner_agent_id is not provided.
UPDATE flow_template_nodes
  SET fallback_agent_key = 'spec_planner_agent_id'
  WHERE node_slug = 'story_plan'
    AND agent_input_key = 'story_planner_agent_id'
    AND template_id IN (
      SELECT id FROM flow_templates
        WHERE template_slug = 'pm.epic_planning_v2'
          AND is_builtin = TRUE
    );
