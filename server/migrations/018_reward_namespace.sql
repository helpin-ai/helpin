-- 018_reward_namespace.sql
-- Namespace reward-domain tables under reward_*

ALTER TABLE IF EXISTS quarters RENAME TO reward_quarters;
ALTER TABLE IF EXISTS sprints RENAME TO reward_sprints;
ALTER TABLE IF EXISTS company_goals RENAME TO reward_company_goals;
ALTER TABLE IF EXISTS goal_team_contributions RENAME TO reward_goal_team_contributions;
ALTER TABLE IF EXISTS sprint_goals RENAME TO reward_sprint_goals;
ALTER TABLE IF EXISTS goal_drafts RENAME TO reward_goal_drafts;
ALTER TABLE IF EXISTS individual_checks RENAME TO reward_individual_checks;
ALTER TABLE IF EXISTS quarterly_bonus_calculations RENAME TO reward_bonus_calculations;
ALTER TABLE IF EXISTS quarterly_finance_settings RENAME TO reward_finance_settings;
ALTER TABLE IF EXISTS bonus_audit_log RENAME TO reward_audit_log;

ALTER INDEX IF EXISTS idx_quarters_workspace RENAME TO idx_reward_quarters_workspace;
ALTER INDEX IF EXISTS idx_quarters_status RENAME TO idx_reward_quarters_status;
ALTER INDEX IF EXISTS idx_quarters_created_by RENAME TO idx_reward_quarters_created_by;
ALTER INDEX IF EXISTS idx_sprints_quarter RENAME TO idx_reward_sprints_quarter;
ALTER INDEX IF EXISTS idx_sprints_workspace RENAME TO idx_reward_sprints_workspace;
ALTER INDEX IF EXISTS idx_sprints_status RENAME TO idx_reward_sprints_status;
ALTER INDEX IF EXISTS idx_sprints_locked_by RENAME TO idx_reward_sprints_locked_by;

ALTER INDEX IF EXISTS idx_company_goals_workspace RENAME TO idx_reward_company_goals_workspace;
ALTER INDEX IF EXISTS idx_company_goals_quarter RENAME TO idx_reward_company_goals_quarter;
ALTER INDEX IF EXISTS idx_company_goals_status RENAME TO idx_reward_company_goals_status;
ALTER INDEX IF EXISTS idx_company_goals_created_by RENAME TO idx_reward_company_goals_created_by;
ALTER INDEX IF EXISTS idx_goal_team_contributions_goal RENAME TO idx_reward_goal_team_contributions_goal;
ALTER INDEX IF EXISTS idx_goal_team_contributions_team RENAME TO idx_reward_goal_team_contributions_team;
ALTER INDEX IF EXISTS idx_sprint_goals_sprint RENAME TO idx_reward_sprint_goals_sprint;
ALTER INDEX IF EXISTS idx_sprint_goals_team RENAME TO idx_reward_sprint_goals_team;
ALTER INDEX IF EXISTS idx_sprint_goals_kr RENAME TO idx_reward_sprint_goals_kr;
ALTER INDEX IF EXISTS idx_goal_drafts_workspace RENAME TO idx_reward_goal_drafts_workspace;
ALTER INDEX IF EXISTS idx_goal_drafts_quarter RENAME TO idx_reward_goal_drafts_quarter;
ALTER INDEX IF EXISTS idx_goal_drafts_created_by RENAME TO idx_reward_goal_drafts_created_by;

ALTER INDEX IF EXISTS idx_individual_checks_sprint RENAME TO idx_reward_individual_checks_sprint;
ALTER INDEX IF EXISTS idx_individual_checks_workspace RENAME TO idx_reward_individual_checks_workspace;
ALTER INDEX IF EXISTS idx_individual_checks_employee RENAME TO idx_reward_individual_checks_employee;
ALTER INDEX IF EXISTS idx_individual_checks_scored_by RENAME TO idx_reward_individual_checks_scored_by;
ALTER INDEX IF EXISTS idx_individual_checks_criteria RENAME TO idx_reward_individual_checks_criteria;

ALTER INDEX IF EXISTS idx_quarterly_bonus_calc_workspace RENAME TO idx_reward_bonus_calc_workspace;
ALTER INDEX IF EXISTS idx_quarterly_bonus_calc_quarter RENAME TO idx_reward_bonus_calc_quarter;
ALTER INDEX IF EXISTS idx_quarterly_bonus_calc_employee RENAME TO idx_reward_bonus_calc_employee;
ALTER INDEX IF EXISTS idx_quarterly_bonus_calc_tier RENAME TO idx_reward_bonus_calc_tier;
ALTER INDEX IF EXISTS idx_quarterly_finance_workspace RENAME TO idx_reward_finance_workspace;
ALTER INDEX IF EXISTS idx_quarterly_finance_quarter RENAME TO idx_reward_finance_quarter;
ALTER INDEX IF EXISTS idx_bonus_audit_workspace RENAME TO idx_reward_audit_workspace;
ALTER INDEX IF EXISTS idx_bonus_audit_quarter RENAME TO idx_reward_audit_quarter;
ALTER INDEX IF EXISTS idx_bonus_audit_employee RENAME TO idx_reward_audit_employee;
ALTER INDEX IF EXISTS idx_bonus_audit_performed_by RENAME TO idx_reward_audit_performed_by;
ALTER INDEX IF EXISTS idx_bonus_audit_action RENAME TO idx_reward_audit_action;
ALTER INDEX IF EXISTS idx_bonus_audit_performed_at RENAME TO idx_reward_audit_performed_at;
ALTER INDEX IF EXISTS idx_quarterly_bonus_calc_locked RENAME TO idx_reward_bonus_calc_locked;
ALTER INDEX IF EXISTS idx_quarterly_finance_locked RENAME TO idx_reward_finance_locked;

-- Trigger renames are handled at runtime in server/internal/repository/reward_schema.go
-- because PostgreSQL requires ALTER TRIGGER ... ON <table>.
