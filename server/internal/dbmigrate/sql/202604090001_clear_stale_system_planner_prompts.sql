-- Migration: clear_stale_system_planner_prompts
-- Clear persisted built-in planner prompts so startup reconciliation rewrites
-- Atlas and Scribe with the latest preset prompt text.

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
          AND preset_key IN ('epic_planner', 'task_planner')
          AND system_prompt IS NOT NULL;
    END IF;
END $$;
