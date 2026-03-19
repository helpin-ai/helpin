-- Deprecate agent fields that are no longer used at runtime.
-- These columns are kept for backward compatibility but their values are ignored.
-- Phase 4 will drop them entirely.

-- tools: superseded by allowed_tools (per-agent override of class defaults)
-- target_selector: never read at runtime, superseded by allowed_targets
-- trigger_events: never read at runtime, superseded by automation_rules

COMMENT ON COLUMN agents.tools IS 'DEPRECATED: use allowed_tools instead';
COMMENT ON COLUMN agents.target_selector IS 'DEPRECATED: use allowed_targets instead';
COMMENT ON COLUMN agents.trigger_events IS 'DEPRECATED: use automation_rules instead';
