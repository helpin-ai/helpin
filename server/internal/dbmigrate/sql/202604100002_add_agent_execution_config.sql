-- Migration: add_agent_execution_config
-- Add per-agent execution settings for runtime-specific knobs like reasoning effort
-- and OpenAI service tier selection.

ALTER TABLE IF EXISTS agents
    ADD COLUMN IF NOT EXISTS execution_config JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE IF EXISTS workspace_agent_preset_versions
    ADD COLUMN IF NOT EXISTS execution_config JSONB NOT NULL DEFAULT '{}'::jsonb;
