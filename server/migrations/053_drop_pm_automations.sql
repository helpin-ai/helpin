-- Migration: Drop the legacy pm_automations table after data has been migrated
-- to automation_rules (migration 052).
-- Run only after verifying all pm_automations data is in automation_rules.
DROP TABLE IF EXISTS pm_automations;
