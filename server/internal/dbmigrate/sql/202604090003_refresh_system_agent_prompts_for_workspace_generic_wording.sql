-- Migration: refresh_system_agent_prompts_for_workspace_generic_wording
-- Clear persisted built-in system prompts so startup reconciliation rewrites
-- product-owned agents with workspace-generic prompt wording.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'is_system'
    ) AND EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'preset_key'
    ) AND EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'system_prompt'
    ) THEN
        UPDATE agents
        SET system_prompt = NULL
        WHERE is_system = true
          AND preset_key IN (
              'epic_planner',
              'task_planner',
              'crm_operator',
              'support_agent',
              'code_builder',
              'review_agent'
          )
          AND system_prompt IS NOT NULL;
    END IF;
END $$;
