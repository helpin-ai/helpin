-- Drop deprecated Agent columns that are no longer read at runtime.
-- agent_kind → all agents are LLM; column is no longer in the Go model
-- tools → superseded by allowed_tools
-- target_selector → superseded by allowed_targets
-- trigger_events → superseded by automation_rules

ALTER TABLE agents DROP COLUMN IF EXISTS agent_kind;
ALTER TABLE agents DROP COLUMN IF EXISTS tools;
ALTER TABLE agents DROP COLUMN IF EXISTS target_selector;
ALTER TABLE agents DROP COLUMN IF EXISTS trigger_events;
