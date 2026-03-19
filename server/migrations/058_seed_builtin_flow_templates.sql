-- Seed builtin flow templates into DB so they are visible via the template API
-- and can eventually be customized per workspace. The hardcoded Go definitions
-- remain as a fallback in the resolver.

DO $$
DECLARE
  v_epic_id      UUID;
  v_story_id     UUID;
  v_deal_id      UUID;
BEGIN

-- =========================================================================
-- 1. Epic Planning V2
-- =========================================================================
IF NOT EXISTS (SELECT 1 FROM flow_templates WHERE template_slug = 'pm.epic_planning_v2' AND workspace_id IS NULL) THEN
  INSERT INTO flow_templates (id, workspace_id, name, description, template_slug, version, target_type, initial_node_slug, is_builtin, status)
  VALUES (gen_random_uuid(), NULL, 'Epic Planning', 'Draft a product spec and break it into stories with approval gates.', 'pm.epic_planning_v2', 2, 'epic', 'ensure_spec_doc', TRUE, 'active')
  RETURNING id INTO v_epic_id;

  INSERT INTO flow_template_nodes (template_id, node_slug, label, node_type, position, next_node_slug, command_name) VALUES
    (v_epic_id, 'ensure_spec_doc', 'Ensure Spec Document', 'system_action', 0, 'spec_draft', 'docs.ensure_spec_doc');

  INSERT INTO flow_template_nodes (template_id, node_slug, label, node_type, position, next_node_slug, agent_input_key, allowed_tools, output_tag, actions, feedback_from_node) VALUES
    (v_epic_id, 'spec_draft', 'Draft Specification', 'interactive_agent', 1, 'spec_approval', 'spec_planner_agent_id',
     '["read_file","read_file_range","list_directory","search_files","ripgrep","grep","list_symbols","web_search","list_documents","read_document","search_documents"]'::jsonb,
     'spec_draft', '["finalize"]'::jsonb, 'spec_approval');

  INSERT INTO flow_template_nodes (template_id, node_slug, label, node_type, position, next_node_slug, loopback_node_slug, actions, approve_command_name) VALUES
    (v_epic_id, 'spec_approval', 'Approve Specification', 'approval_gate', 2, 'story_plan', 'spec_draft',
     '["approve","request_changes","reject"]'::jsonb, 'pm.approve_epic_spec');

  INSERT INTO flow_template_nodes (template_id, node_slug, label, node_type, position, next_node_slug, agent_input_key, allowed_tools, output_tag, actions, feedback_from_node) VALUES
    (v_epic_id, 'story_plan', 'Plan Stories', 'interactive_agent', 3, 'plan_approval', 'story_planner_agent_id',
     '["read_file","read_file_range","list_directory","search_files","ripgrep","grep","list_symbols","web_search","list_documents","read_document","search_documents"]'::jsonb,
     'story_plan', '["finalize"]'::jsonb, 'plan_approval');

  INSERT INTO flow_template_nodes (template_id, node_slug, label, node_type, position, next_node_slug, loopback_node_slug, actions) VALUES
    (v_epic_id, 'plan_approval', 'Approve Story Plan', 'approval_gate', 4, 'create_stories', 'story_plan',
     '["approve","request_changes","reject"]'::jsonb);

  INSERT INTO flow_template_nodes (template_id, node_slug, label, node_type, position, next_node_slug, command_name, retryable) VALUES
    (v_epic_id, 'create_stories', 'Create Stories', 'system_action', 5, 'done', 'pm.create_story_batch', TRUE);

  INSERT INTO flow_template_nodes (template_id, node_slug, label, node_type, position) VALUES
    (v_epic_id, 'done', 'Complete', 'terminal', 6);
END IF;

-- =========================================================================
-- 2. Story Completion V1
-- =========================================================================
IF NOT EXISTS (SELECT 1 FROM flow_templates WHERE template_slug = 'pm.story_completion_v1' AND workspace_id IS NULL) THEN
  INSERT INTO flow_templates (id, workspace_id, name, description, template_slug, version, target_type, initial_node_slug, is_builtin, status)
  VALUES (gen_random_uuid(), NULL, 'Story Completion', 'Assess story completion and create follow-up stories.', 'pm.story_completion_v1', 1, 'story', 'completion_assessment', TRUE, 'active')
  RETURNING id INTO v_story_id;

  INSERT INTO flow_template_nodes (template_id, node_slug, label, node_type, position, next_node_slug, agent_input_key, allowed_tools, feedback_from_node) VALUES
    (v_story_id, 'completion_assessment', 'Assess Completion', 'agent_task', 0, 'completion_review', 'agent_id',
     '["list_documents","read_document","search_documents","list_story_checklist"]'::jsonb, 'completion_review');

  INSERT INTO flow_template_nodes (template_id, node_slug, label, node_type, position, next_node_slug, loopback_node_slug, actions) VALUES
    (v_story_id, 'completion_review', 'Review Follow-Ups', 'approval_gate', 1, 'create_followups', 'completion_assessment',
     '["approve","request_changes","reject"]'::jsonb);

  INSERT INTO flow_template_nodes (template_id, node_slug, label, node_type, position, next_node_slug, command_name) VALUES
    (v_story_id, 'create_followups', 'Create Follow-Ups', 'system_action', 2, 'done', 'pm.create_followup_stories');

  INSERT INTO flow_template_nodes (template_id, node_slug, label, node_type, position) VALUES
    (v_story_id, 'done', 'Complete', 'terminal', 3);
END IF;

-- =========================================================================
-- 3. CRM Deal Review V1
-- =========================================================================
IF NOT EXISTS (SELECT 1 FROM flow_templates WHERE template_slug = 'crm.deal_review_v1' AND workspace_id IS NULL) THEN
  INSERT INTO flow_templates (id, workspace_id, name, description, template_slug, version, target_type, initial_node_slug, is_builtin, status)
  VALUES (gen_random_uuid(), NULL, 'Deal Review', 'Review CRM deal and apply recommended actions.', 'crm.deal_review_v1', 1, 'crm_deal', 'deal_review', TRUE, 'active')
  RETURNING id INTO v_deal_id;

  INSERT INTO flow_template_nodes (template_id, node_slug, label, node_type, position, next_node_slug, agent_input_key, allowed_tools, feedback_from_node) VALUES
    (v_deal_id, 'deal_review', 'Review Deal', 'agent_task', 0, 'deal_review_approval', 'agent_id',
     '["list_deals","list_buyer_signals","list_contacts"]'::jsonb, 'deal_review_approval');

  INSERT INTO flow_template_nodes (template_id, node_slug, label, node_type, position, next_node_slug, loopback_node_slug, actions) VALUES
    (v_deal_id, 'deal_review_approval', 'Approve Deal Actions', 'approval_gate', 1, 'apply_deal_actions', 'deal_review',
     '["approve","request_changes","reject"]'::jsonb);

  INSERT INTO flow_template_nodes (template_id, node_slug, label, node_type, position, next_node_slug, command_name) VALUES
    (v_deal_id, 'apply_deal_actions', 'Apply Deal Actions', 'system_action', 2, 'done', 'crm.apply_deal_actions');

  INSERT INTO flow_template_nodes (template_id, node_slug, label, node_type, position) VALUES
    (v_deal_id, 'done', 'Complete', 'terminal', 3);
END IF;

END $$;
