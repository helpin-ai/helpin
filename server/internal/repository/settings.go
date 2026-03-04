package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/d4interactive/teampulse/server/internal/model"
)

// SettingsRepository handles database operations for workspace configuration.
type SettingsRepository struct {
	db *gorm.DB
}

// NewSettingsRepository creates a new SettingsRepository.
func NewSettingsRepository(db *gorm.DB) *SettingsRepository {
	return &SettingsRepository{db: db}
}

// GetAll returns the full workspace configuration.
func (r *SettingsRepository) GetAll(ctx context.Context, workspaceID string) (*model.FullWorkspaceConfig, error) {
	cfg := &model.FullWorkspaceConfig{}

	settings, err := r.getSettings(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if settings == nil {
		settings, err = r.Initialize(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
	}
	cfg.Settings = settings

	teams, err := r.listTeams(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	cfg.Teams = teams

	people, err := r.listPeople(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	cfg.People = people

	memberships, err := r.listMemberships(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	cfg.Memberships = memberships

	managers, err := r.listManagers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	cfg.Managers = managers

	jobRoles, err := r.listJobRoleCriteria(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	cfg.JobRoles = jobRoles

	tiers, err := r.listBonusTiers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	cfg.BonusTiers = tiers

	return cfg, nil
}

func (r *SettingsRepository) getSettings(ctx context.Context, workspaceID string) (*model.WorkspaceSettings, error) {
	s := &model.WorkspaceSettings{}
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).First(s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get settings: %w", err)
	}
	return s, nil
}

func (r *SettingsRepository) listTeams(ctx context.Context, workspaceID string) ([]model.WorkspaceTeam, error) {
	var teams []model.WorkspaceTeam
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Order("name").Find(&teams).Error
	if err != nil {
		return nil, fmt.Errorf("list teams: %w", err)
	}
	return teams, nil
}

func (r *SettingsRepository) listPeople(ctx context.Context, workspaceID string) ([]model.WorkspacePerson, error) {
	var people []model.WorkspacePerson
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Order("name").Find(&people).Error
	if err != nil {
		return nil, fmt.Errorf("list people: %w", err)
	}
	return people, nil
}

func (r *SettingsRepository) listMemberships(ctx context.Context, workspaceID string) ([]model.TeamMembership, error) {
	var memberships []model.TeamMembership
	err := r.db.WithContext(ctx).
		Table("team_memberships").
		Joins("JOIN workspace_teams ON team_memberships.team_id = workspace_teams.id").
		Where("workspace_teams.workspace_id = ?", workspaceID).
		Order("team_memberships.team_id, team_memberships.person_id").
		Select("team_memberships.*").
		Find(&memberships).Error
	if err != nil {
		return nil, fmt.Errorf("list memberships: %w", err)
	}
	return memberships, nil
}

func (r *SettingsRepository) listManagers(ctx context.Context, workspaceID string) ([]model.WorkspaceManager, error) {
	var managers []model.WorkspaceManager
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Order("person_id").Find(&managers).Error
	if err != nil {
		return nil, fmt.Errorf("list managers: %w", err)
	}
	return managers, nil
}

func (r *SettingsRepository) listJobRoleCriteria(ctx context.Context, workspaceID string) ([]model.JobRoleCriteria, error) {
	var criteria []model.JobRoleCriteria
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Order("job_role, criteria_id").Find(&criteria).Error
	if err != nil {
		return nil, fmt.Errorf("list job role criteria: %w", err)
	}
	return criteria, nil
}

func (r *SettingsRepository) listBonusTiers(ctx context.Context, workspaceID string) ([]model.BonusTier, error) {
	var tiers []model.BonusTier
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Order("tier").Find(&tiers).Error
	if err != nil {
		return nil, fmt.Errorf("list bonus tiers: %w", err)
	}
	return tiers, nil
}

// Initialize creates default workspace settings.
func (r *SettingsRepository) Initialize(ctx context.Context, workspaceID string) (*model.WorkspaceSettings, error) {
	s := &model.WorkspaceSettings{
		WorkspaceID: workspaceID,
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "workspace_id"}},
			DoNothing: true,
		}).
		Create(s).Error
	if err != nil {
		return nil, fmt.Errorf("initialize settings: %w", err)
	}
	// Always fetch to get the current record (whether just created or already existed).
	return r.getSettings(ctx, workspaceID)
}

// CreateTeam inserts a new team.
func (r *SettingsRepository) CreateTeam(ctx context.Context, req model.CreateTeamRequest) (*model.WorkspaceTeam, error) {
	t := &model.WorkspaceTeam{
		WorkspaceID: req.WorkspaceID,
		Name:        req.Name,
		Description: req.Description,
		ManagerID:   req.ManagerID,
	}
	if err := r.db.WithContext(ctx).Create(t).Error; err != nil {
		return nil, fmt.Errorf("create team: %w", err)
	}
	return t, nil
}

// UpdateTeam modifies a team.
func (r *SettingsRepository) UpdateTeam(ctx context.Context, id string, req model.UpdateTeamRequest) (*model.WorkspaceTeam, error) {
	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.ManagerID != nil {
		updates["manager_id"] = *req.ManagerID
	}

	if err := r.db.WithContext(ctx).Model(&model.WorkspaceTeam{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update team: %w", err)
	}

	t := &model.WorkspaceTeam{}
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(t).Error; err != nil {
		return nil, fmt.Errorf("update team: %w", err)
	}
	return t, nil
}

// DeleteTeam removes a team by ID.
func (r *SettingsRepository) DeleteTeam(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.WorkspaceTeam{}).Error; err != nil {
		return fmt.Errorf("delete team: %w", err)
	}
	return nil
}

// CreatePerson inserts a new person and optionally assigns them to teams.
func (r *SettingsRepository) CreatePerson(ctx context.Context, req model.CreatePersonRequest) (*model.WorkspacePerson, error) {
	var person *model.WorkspacePerson

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		p := &model.WorkspacePerson{
			WorkspaceID:         req.WorkspaceID,
			UserID:              req.UserID,
			Name:                req.Name,
			Email:               req.Email,
			Role:                req.Role,
			JobRole:             req.JobRole,
			ManagerID:           req.ManagerID,
			HireDate:            req.HireDate,
			BaseSalary:          req.BaseSalary,
			ActiveForBonus:      req.ActiveForBonus,
			ActiveForEvaluation: req.ActiveForEvaluation,
			IsAccountOwner:      req.IsAccountOwner,
		}
		if err := tx.Create(p).Error; err != nil {
			return fmt.Errorf("create person: %w", err)
		}

		for _, teamID := range req.TeamIDs {
			m := &model.TeamMembership{
				TeamID:   teamID,
				PersonID: p.ID,
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(m).Error; err != nil {
				return fmt.Errorf("assign person to team: %w", err)
			}
		}

		person = p
		return nil
	})
	if err != nil {
		return nil, err
	}
	return person, nil
}

// UpdatePerson modifies a person and optionally replaces their team assignments.
func (r *SettingsRepository) UpdatePerson(ctx context.Context, id string, req model.UpdatePersonRequest) (*model.WorkspacePerson, error) {
	var person *model.WorkspacePerson

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]interface{}{}
		if req.Name != nil {
			updates["name"] = *req.Name
		}
		if req.Email != nil {
			updates["email"] = *req.Email
		}
		if req.Role != nil {
			updates["role"] = *req.Role
		}
		if req.JobRole != nil {
			updates["job_role"] = *req.JobRole
		}
		if req.ManagerID != nil {
			updates["manager_id"] = *req.ManagerID
		}
		if req.HireDate != nil {
			updates["hire_date"] = *req.HireDate
		}
		if req.Status != nil {
			updates["status"] = *req.Status
		}
		if req.BaseSalary != nil {
			updates["base_salary"] = *req.BaseSalary
		}
		if req.ActiveForBonus != nil {
			updates["active_for_bonus"] = *req.ActiveForBonus
		}
		if req.ActiveForEvaluation != nil {
			updates["active_for_evaluation"] = *req.ActiveForEvaluation
		}
		if req.IsAccountOwner != nil {
			updates["is_account_owner"] = *req.IsAccountOwner
		}

		if len(updates) > 0 {
			if err := tx.Model(&model.WorkspacePerson{}).Where("id = ?", id).Updates(updates).Error; err != nil {
				return fmt.Errorf("update person: %w", err)
			}
		}

		// Replace team memberships if provided.
		if req.TeamIDs != nil {
			if err := tx.Where("person_id = ?", id).Delete(&model.TeamMembership{}).Error; err != nil {
				return fmt.Errorf("clear team memberships: %w", err)
			}
			for _, teamID := range req.TeamIDs {
				m := &model.TeamMembership{
					TeamID:   teamID,
					PersonID: id,
				}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(m).Error; err != nil {
					return fmt.Errorf("assign person to team: %w", err)
				}
			}
		}

		p := &model.WorkspacePerson{}
		if err := tx.Where("id = ?", id).First(p).Error; err != nil {
			return fmt.Errorf("update person: %w", err)
		}
		person = p
		return nil
	})
	if err != nil {
		return nil, err
	}
	return person, nil
}

// DeletePerson removes a person by ID.
func (r *SettingsRepository) DeletePerson(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.WorkspacePerson{}).Error; err != nil {
		return fmt.Errorf("delete person: %w", err)
	}
	return nil
}

// UpdateBonusTiers replaces all bonus tiers for a workspace.
func (r *SettingsRepository) UpdateBonusTiers(ctx context.Context, workspaceID string, tiers []model.BonusTierItem) ([]model.BonusTier, error) {
	var result []model.BonusTier

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete existing tiers that are editable.
		if err := tx.Where("workspace_id = ? AND editable = true", workspaceID).Delete(&model.BonusTier{}).Error; err != nil {
			return fmt.Errorf("delete bonus tiers: %w", err)
		}

		for _, tier := range tiers {
			t := model.BonusTier{
				WorkspaceID:      workspaceID,
				Tier:             tier.Tier,
				MinScore:         tier.MinScore,
				MaxScore:         tier.MaxScore,
				SalaryMultiplier: tier.SalaryMultiplier,
				Description:      tier.Description,
			}
			err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "workspace_id"}, {Name: "tier"}},
				DoUpdates: clause.AssignmentColumns([]string{"min_score", "max_score", "salary_multiplier", "description"}),
			}).Create(&t).Error
			if err != nil {
				return fmt.Errorf("upsert bonus tier: %w", err)
			}
			// Re-fetch to get correct values after upsert.
			var fetched model.BonusTier
			if err := tx.Where("workspace_id = ? AND tier = ?", workspaceID, tier.Tier).First(&fetched).Error; err != nil {
				return fmt.Errorf("fetch bonus tier: %w", err)
			}
			result = append(result, fetched)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// UpdateJobRoleCriteria replaces all criteria for a given job role in a workspace.
func (r *SettingsRepository) UpdateJobRoleCriteria(ctx context.Context, workspaceID, jobRole string, criteria []model.JobRoleCriteriaItem) ([]model.JobRoleCriteria, error) {
	var result []model.JobRoleCriteria

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete existing criteria for this job role.
		if err := tx.Where("workspace_id = ? AND job_role = ?", workspaceID, jobRole).Delete(&model.JobRoleCriteria{}).Error; err != nil {
			return fmt.Errorf("delete job role criteria: %w", err)
		}

		for _, c := range criteria {
			question := ""
			if c.Question != nil {
				question = *c.Question
			}
			jrc := model.JobRoleCriteria{
				WorkspaceID: workspaceID,
				JobRole:     jobRole,
				CriteriaID:  c.CriteriaID,
				Name:        c.Name,
				Description: c.Description,
				Question:    question,
				Enabled:     c.Enabled,
				Weight:      c.Weight,
			}
			if err := tx.Create(&jrc).Error; err != nil {
				return fmt.Errorf("insert job role criteria: %w", err)
			}
			result = append(result, jrc)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// DeleteJobRole removes all criteria for a given job role.
func (r *SettingsRepository) DeleteJobRole(ctx context.Context, workspaceID, jobRole string) error {
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND job_role = ?", workspaceID, jobRole).Delete(&model.JobRoleCriteria{}).Error; err != nil {
		return fmt.Errorf("delete job role: %w", err)
	}
	return nil
}

// UpdateSystem updates workspace system settings.
func (r *SettingsRepository) UpdateSystem(ctx context.Context, workspaceID string, req model.UpdateSystemSettingsRequest) (*model.WorkspaceSettings, error) {
	updates := map[string]interface{}{}
	if req.SprintDurationWeeks != nil {
		updates["sprint_duration_weeks"] = *req.SprintDurationWeeks
	}
	if req.NotificationsEnabled != nil {
		updates["notifications_enabled"] = *req.NotificationsEnabled
	}
	if req.AutoCalculateBonuses != nil {
		updates["auto_calculate_bonuses"] = *req.AutoCalculateBonuses
	}
	if req.TeamWeight != nil {
		updates["team_weight"] = *req.TeamWeight
	}

	if err := r.db.WithContext(ctx).Model(&model.WorkspaceSettings{}).Where("workspace_id = ?", workspaceID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update system settings: %w", err)
	}

	s := &model.WorkspaceSettings{}
	if err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).First(s).Error; err != nil {
		return nil, fmt.Errorf("update system settings: %w", err)
	}
	return s, nil
}
