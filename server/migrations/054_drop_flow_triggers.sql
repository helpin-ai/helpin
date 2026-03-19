-- Migration: Drop the unused flow_triggers table.
-- Verify before running: SELECT count(*) FROM flow_triggers; should be 0 or only unused rows.
DROP TABLE IF EXISTS flow_triggers;
