-- Migration: upgrade_agent_models_to_gpt55
-- Move existing persisted OpenAI agent model selections from GPT-5.4 to GPT-5.5.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'model'
    ) THEN
        UPDATE agents
        SET model = 'gpt-5.5'
        WHERE model = 'gpt-5.4';

        UPDATE agents
        SET model = 'openai/gpt-5.5'
        WHERE model = 'openai/gpt-5.4';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'workspace_agent_preset_versions' AND column_name = 'model'
    ) THEN
        UPDATE workspace_agent_preset_versions
        SET model = 'gpt-5.5'
        WHERE model = 'gpt-5.4';

        UPDATE workspace_agent_preset_versions
        SET model = 'openai/gpt-5.5'
        WHERE model = 'openai/gpt-5.4';
    END IF;
END $$;
