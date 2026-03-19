-- Create a generic "Agent Story Run" builtin flow template and convert
-- existing run_agent automation rules to start_flow rules that reference it.
--
-- The template has two nodes:
--   implement (agent_task) → done (terminal)
--
-- Each converted rule preserves the original agent_id in the new action_config.

DO $$
DECLARE
  v_template_id UUID;
  v_rule        RECORD;
  v_agent_id    TEXT;
BEGIN

-- =========================================================================
-- 1. Create the generic "Agent Story Run" template
-- =========================================================================
IF NOT EXISTS (SELECT 1 FROM flow_templates WHERE template_slug = 'pm.agent_story_run' AND workspace_id IS NULL) THEN
  INSERT INTO flow_templates (id, workspace_id, name, description, template_slug, version, target_type, initial_node_slug, is_builtin, status)
  VALUES (gen_random_uuid(), NULL, 'Agent Story Run', 'Run an agent on a story (single autonomous step).', 'pm.agent_story_run', 1, 'story', 'implement', TRUE, 'active')
  RETURNING id INTO v_template_id;

  INSERT INTO flow_template_nodes (template_id, node_slug, label, node_type, position, next_node_slug, agent_input_key) VALUES
    (v_template_id, 'implement', 'Implement', 'agent_task', 0, 'done', 'agent_id');

  INSERT INTO flow_template_nodes (template_id, node_slug, label, node_type, position) VALUES
    (v_template_id, 'done', 'Complete', 'terminal', 1);
END IF;

-- =========================================================================
-- 2. Convert run_agent rules to start_flow rules
-- =========================================================================
FOR v_rule IN
  SELECT id, action_config
  FROM automation_rules
  WHERE action_type = 'run_agent'
    AND enabled = TRUE
LOOP
  -- Extract agent_id from the existing action_config
  v_agent_id := v_rule.action_config->>'agent_id';

  IF v_agent_id IS NOT NULL AND v_agent_id != '' THEN
    UPDATE automation_rules
    SET action_type   = 'start_flow',
        action_config = jsonb_build_object(
          'template_id', 'pm.agent_story_run',
          'agent_id',    v_agent_id
        ),
        updated_at    = now()
    WHERE id = v_rule.id;
  END IF;
END LOOP;

END $$;
