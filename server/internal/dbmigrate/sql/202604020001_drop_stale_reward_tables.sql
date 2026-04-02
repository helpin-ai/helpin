-- Drop reward and bonus tables that no longer have Go models.
-- These were removed from the codebase but never cleaned up in the database,
-- causing errors during workspace deletion.
DROP TABLE IF EXISTS reward_goal_team_contributions;
DROP TABLE IF EXISTS reward_sprint_goals;
DROP TABLE IF EXISTS reward_bonus_calculations;
DROP TABLE IF EXISTS reward_individual_checks;
DROP TABLE IF EXISTS reward_finance_settings;
DROP TABLE IF EXISTS reward_audit_log;
DROP TABLE IF EXISTS reward_company_goals;
DROP TABLE IF EXISTS reward_goal_drafts;
DROP TABLE IF EXISTS reward_sprints;
DROP TABLE IF EXISTS reward_quarters;
DROP TABLE IF EXISTS bonus_tiers;
