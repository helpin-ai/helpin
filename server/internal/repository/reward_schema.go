package repository

import (
	"fmt"

	"gorm.io/gorm"
)

// MigrateLegacyRewardSchema renames legacy reward tables into the reward_* namespace.
func MigrateLegacyRewardSchema(db *gorm.DB) error {
	const stmt = `
DO $$
BEGIN
    IF to_regclass('public.quarters') IS NOT NULL AND to_regclass('public.reward_quarters') IS NULL THEN
        ALTER TABLE quarters RENAME TO reward_quarters;
    END IF;
    IF to_regclass('public.sprints') IS NOT NULL AND to_regclass('public.reward_sprints') IS NULL THEN
        ALTER TABLE sprints RENAME TO reward_sprints;
    END IF;
    IF to_regclass('public.company_goals') IS NOT NULL AND to_regclass('public.reward_company_goals') IS NULL THEN
        ALTER TABLE company_goals RENAME TO reward_company_goals;
    END IF;
    IF to_regclass('public.goal_team_contributions') IS NOT NULL AND to_regclass('public.reward_goal_team_contributions') IS NULL THEN
        ALTER TABLE goal_team_contributions RENAME TO reward_goal_team_contributions;
    END IF;
    IF to_regclass('public.sprint_goals') IS NOT NULL AND to_regclass('public.reward_sprint_goals') IS NULL THEN
        ALTER TABLE sprint_goals RENAME TO reward_sprint_goals;
    END IF;
    IF to_regclass('public.goal_drafts') IS NOT NULL AND to_regclass('public.reward_goal_drafts') IS NULL THEN
        ALTER TABLE goal_drafts RENAME TO reward_goal_drafts;
    END IF;
    IF to_regclass('public.individual_checks') IS NOT NULL AND to_regclass('public.reward_individual_checks') IS NULL THEN
        ALTER TABLE individual_checks RENAME TO reward_individual_checks;
    END IF;
    IF to_regclass('public.quarterly_bonus_calculations') IS NOT NULL AND to_regclass('public.reward_bonus_calculations') IS NULL THEN
        ALTER TABLE quarterly_bonus_calculations RENAME TO reward_bonus_calculations;
    END IF;
    IF to_regclass('public.quarterly_finance_settings') IS NOT NULL AND to_regclass('public.reward_finance_settings') IS NULL THEN
        ALTER TABLE quarterly_finance_settings RENAME TO reward_finance_settings;
    END IF;
    IF to_regclass('public.bonus_audit_log') IS NOT NULL AND to_regclass('public.reward_audit_log') IS NULL THEN
        ALTER TABLE bonus_audit_log RENAME TO reward_audit_log;
    END IF;

    IF to_regclass('public.idx_quarters_workspace') IS NOT NULL THEN
        ALTER INDEX idx_quarters_workspace RENAME TO idx_reward_quarters_workspace;
    END IF;
    IF to_regclass('public.idx_quarters_status') IS NOT NULL THEN
        ALTER INDEX idx_quarters_status RENAME TO idx_reward_quarters_status;
    END IF;
    IF to_regclass('public.idx_quarters_created_by') IS NOT NULL THEN
        ALTER INDEX idx_quarters_created_by RENAME TO idx_reward_quarters_created_by;
    END IF;
    IF to_regclass('public.idx_sprints_quarter') IS NOT NULL THEN
        ALTER INDEX idx_sprints_quarter RENAME TO idx_reward_sprints_quarter;
    END IF;
    IF to_regclass('public.idx_sprints_workspace') IS NOT NULL THEN
        ALTER INDEX idx_sprints_workspace RENAME TO idx_reward_sprints_workspace;
    END IF;
    IF to_regclass('public.idx_sprints_status') IS NOT NULL THEN
        ALTER INDEX idx_sprints_status RENAME TO idx_reward_sprints_status;
    END IF;
    IF to_regclass('public.idx_sprints_locked_by') IS NOT NULL THEN
        ALTER INDEX idx_sprints_locked_by RENAME TO idx_reward_sprints_locked_by;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_quarters_updated_at') THEN
        ALTER TRIGGER trg_quarters_updated_at ON reward_quarters RENAME TO trg_reward_quarters_updated_at;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_sprints_updated_at') THEN
        ALTER TRIGGER trg_sprints_updated_at ON reward_sprints RENAME TO trg_reward_sprints_updated_at;
    END IF;

    IF to_regclass('public.idx_company_goals_workspace') IS NOT NULL THEN
        ALTER INDEX idx_company_goals_workspace RENAME TO idx_reward_company_goals_workspace;
    END IF;
    IF to_regclass('public.idx_company_goals_quarter') IS NOT NULL THEN
        ALTER INDEX idx_company_goals_quarter RENAME TO idx_reward_company_goals_quarter;
    END IF;
    IF to_regclass('public.idx_company_goals_status') IS NOT NULL THEN
        ALTER INDEX idx_company_goals_status RENAME TO idx_reward_company_goals_status;
    END IF;
    IF to_regclass('public.idx_company_goals_created_by') IS NOT NULL THEN
        ALTER INDEX idx_company_goals_created_by RENAME TO idx_reward_company_goals_created_by;
    END IF;
    IF to_regclass('public.idx_goal_team_contributions_goal') IS NOT NULL THEN
        ALTER INDEX idx_goal_team_contributions_goal RENAME TO idx_reward_goal_team_contributions_goal;
    END IF;
    IF to_regclass('public.idx_goal_team_contributions_team') IS NOT NULL THEN
        ALTER INDEX idx_goal_team_contributions_team RENAME TO idx_reward_goal_team_contributions_team;
    END IF;
    IF to_regclass('public.idx_sprint_goals_sprint') IS NOT NULL THEN
        ALTER INDEX idx_sprint_goals_sprint RENAME TO idx_reward_sprint_goals_sprint;
    END IF;
    IF to_regclass('public.idx_sprint_goals_team') IS NOT NULL THEN
        ALTER INDEX idx_sprint_goals_team RENAME TO idx_reward_sprint_goals_team;
    END IF;
    IF to_regclass('public.idx_sprint_goals_kr') IS NOT NULL THEN
        ALTER INDEX idx_sprint_goals_kr RENAME TO idx_reward_sprint_goals_kr;
    END IF;
    IF to_regclass('public.idx_goal_drafts_workspace') IS NOT NULL THEN
        ALTER INDEX idx_goal_drafts_workspace RENAME TO idx_reward_goal_drafts_workspace;
    END IF;
    IF to_regclass('public.idx_goal_drafts_quarter') IS NOT NULL THEN
        ALTER INDEX idx_goal_drafts_quarter RENAME TO idx_reward_goal_drafts_quarter;
    END IF;
    IF to_regclass('public.idx_goal_drafts_created_by') IS NOT NULL THEN
        ALTER INDEX idx_goal_drafts_created_by RENAME TO idx_reward_goal_drafts_created_by;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_company_goals_updated_at') THEN
        ALTER TRIGGER trg_company_goals_updated_at ON reward_company_goals RENAME TO trg_reward_company_goals_updated_at;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_goal_team_contributions_updated_at') THEN
        ALTER TRIGGER trg_goal_team_contributions_updated_at ON reward_goal_team_contributions RENAME TO trg_reward_goal_team_contributions_updated_at;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_sprint_goals_updated_at') THEN
        ALTER TRIGGER trg_sprint_goals_updated_at ON reward_sprint_goals RENAME TO trg_reward_sprint_goals_updated_at;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_goal_drafts_updated_at') THEN
        ALTER TRIGGER trg_goal_drafts_updated_at ON reward_goal_drafts RENAME TO trg_reward_goal_drafts_updated_at;
    END IF;

    IF to_regclass('public.idx_individual_checks_sprint') IS NOT NULL THEN
        ALTER INDEX idx_individual_checks_sprint RENAME TO idx_reward_individual_checks_sprint;
    END IF;
    IF to_regclass('public.idx_individual_checks_workspace') IS NOT NULL THEN
        ALTER INDEX idx_individual_checks_workspace RENAME TO idx_reward_individual_checks_workspace;
    END IF;
    IF to_regclass('public.idx_individual_checks_employee') IS NOT NULL THEN
        ALTER INDEX idx_individual_checks_employee RENAME TO idx_reward_individual_checks_employee;
    END IF;
    IF to_regclass('public.idx_individual_checks_scored_by') IS NOT NULL THEN
        ALTER INDEX idx_individual_checks_scored_by RENAME TO idx_reward_individual_checks_scored_by;
    END IF;
    IF to_regclass('public.idx_individual_checks_criteria') IS NOT NULL THEN
        ALTER INDEX idx_individual_checks_criteria RENAME TO idx_reward_individual_checks_criteria;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_individual_checks_updated_at') THEN
        ALTER TRIGGER trg_individual_checks_updated_at ON reward_individual_checks RENAME TO trg_reward_individual_checks_updated_at;
    END IF;

    IF to_regclass('public.idx_quarterly_bonus_calc_workspace') IS NOT NULL THEN
        ALTER INDEX idx_quarterly_bonus_calc_workspace RENAME TO idx_reward_bonus_calc_workspace;
    END IF;
    IF to_regclass('public.idx_quarterly_bonus_calc_quarter') IS NOT NULL THEN
        ALTER INDEX idx_quarterly_bonus_calc_quarter RENAME TO idx_reward_bonus_calc_quarter;
    END IF;
    IF to_regclass('public.idx_quarterly_bonus_calc_employee') IS NOT NULL THEN
        ALTER INDEX idx_quarterly_bonus_calc_employee RENAME TO idx_reward_bonus_calc_employee;
    END IF;
    IF to_regclass('public.idx_quarterly_bonus_calc_tier') IS NOT NULL THEN
        ALTER INDEX idx_quarterly_bonus_calc_tier RENAME TO idx_reward_bonus_calc_tier;
    END IF;
    IF to_regclass('public.idx_quarterly_finance_workspace') IS NOT NULL THEN
        ALTER INDEX idx_quarterly_finance_workspace RENAME TO idx_reward_finance_workspace;
    END IF;
    IF to_regclass('public.idx_quarterly_finance_quarter') IS NOT NULL THEN
        ALTER INDEX idx_quarterly_finance_quarter RENAME TO idx_reward_finance_quarter;
    END IF;
    IF to_regclass('public.idx_bonus_audit_workspace') IS NOT NULL THEN
        ALTER INDEX idx_bonus_audit_workspace RENAME TO idx_reward_audit_workspace;
    END IF;
    IF to_regclass('public.idx_bonus_audit_quarter') IS NOT NULL THEN
        ALTER INDEX idx_bonus_audit_quarter RENAME TO idx_reward_audit_quarter;
    END IF;
    IF to_regclass('public.idx_bonus_audit_employee') IS NOT NULL THEN
        ALTER INDEX idx_bonus_audit_employee RENAME TO idx_reward_audit_employee;
    END IF;
    IF to_regclass('public.idx_bonus_audit_performed_by') IS NOT NULL THEN
        ALTER INDEX idx_bonus_audit_performed_by RENAME TO idx_reward_audit_performed_by;
    END IF;
    IF to_regclass('public.idx_bonus_audit_action') IS NOT NULL THEN
        ALTER INDEX idx_bonus_audit_action RENAME TO idx_reward_audit_action;
    END IF;
    IF to_regclass('public.idx_bonus_audit_performed_at') IS NOT NULL THEN
        ALTER INDEX idx_bonus_audit_performed_at RENAME TO idx_reward_audit_performed_at;
    END IF;
    IF to_regclass('public.idx_quarterly_bonus_calc_locked') IS NOT NULL THEN
        ALTER INDEX idx_quarterly_bonus_calc_locked RENAME TO idx_reward_bonus_calc_locked;
    END IF;
    IF to_regclass('public.idx_quarterly_finance_locked') IS NOT NULL THEN
        ALTER INDEX idx_quarterly_finance_locked RENAME TO idx_reward_finance_locked;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_quarterly_bonus_calculations_updated_at') THEN
        ALTER TRIGGER trg_quarterly_bonus_calculations_updated_at ON reward_bonus_calculations RENAME TO trg_reward_bonus_calculations_updated_at;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_quarterly_finance_settings_updated_at') THEN
        ALTER TRIGGER trg_quarterly_finance_settings_updated_at ON reward_finance_settings RENAME TO trg_reward_finance_settings_updated_at;
    END IF;
END $$;`

	if err := db.Exec(stmt).Error; err != nil {
		return fmt.Errorf("migrate legacy reward schema: %w", err)
	}
	return nil
}
