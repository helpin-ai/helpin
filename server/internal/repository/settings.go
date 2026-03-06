package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
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

	userMemberships, err := r.listUserMemberships(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	cfg.UserMemberships = userMemberships

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

	preassignments, err := r.listInvitationTeamPreassignments(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	cfg.InvitationTeamPreassignments = preassignments

	estimateSettings, err := r.listTeamEstimateSettings(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	cfg.TeamEstimateSettings = estimateSettings

	fieldVisibility, err := r.listTeamFieldVisibility(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	cfg.TeamFieldVisibility = fieldVisibility

	teamRepoDefaults, err := r.listTeamRepoDefaults(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	cfg.TeamRepoDefaults = teamRepoDefaults

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

func (r *SettingsRepository) listUserMemberships(ctx context.Context, workspaceID string) ([]model.TeamUserMembership, error) {
	var memberships []model.TeamUserMembership
	err := r.db.WithContext(ctx).
		Table("team_user_memberships").
		Joins("JOIN workspace_teams ON team_user_memberships.team_id = workspace_teams.id").
		Where("workspace_teams.workspace_id = ?", workspaceID).
		Order("team_user_memberships.team_id, team_user_memberships.user_id").
		Select("team_user_memberships.*").
		Find(&memberships).Error
	if err != nil {
		return nil, fmt.Errorf("list user memberships: %w", err)
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
		Handle:      req.Handle,
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
	if req.Handle != nil {
		updates["handle"] = *req.Handle
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
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("team_id = ?", id).Delete(&model.TeamMembership{}).Error; err != nil {
			return fmt.Errorf("delete person team memberships: %w", err)
		}
		if err := tx.Where("team_id = ?", id).Delete(&model.TeamUserMembership{}).Error; err != nil {
			return fmt.Errorf("delete user team memberships: %w", err)
		}
		if err := tx.Where("id = ?", id).Delete(&model.WorkspaceTeam{}).Error; err != nil {
			return fmt.Errorf("delete team: %w", err)
		}
		return nil
	})
}

// GetTeamByID loads a team by ID.
func (r *SettingsRepository) GetTeamByID(ctx context.Context, id string) (*model.WorkspaceTeam, error) {
	team := &model.WorkspaceTeam{}
	err := r.db.WithContext(ctx).Where("id = ?", id).First(team).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get team: %w", err)
	}
	return team, nil
}

// GetTeamByHandle loads a team by workspace/handle.
func (r *SettingsRepository) GetTeamByHandle(ctx context.Context, workspaceID, handle string) (*model.WorkspaceTeam, error) {
	team := &model.WorkspaceTeam{}
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND LOWER(handle) = LOWER(?)", workspaceID, handle).
		First(team).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get team by handle: %w", err)
	}
	return team, nil
}

func (r *SettingsRepository) listTeamRepoDefaults(ctx context.Context, workspaceID string) ([]model.PMTeamRepoDefault, error) {
	var defaults []model.PMTeamRepoDefault
	err := r.db.WithContext(ctx).
		Table("pm_team_repo_defaults").
		Joins("JOIN workspace_teams ON pm_team_repo_defaults.team_id = workspace_teams.id").
		Where("workspace_teams.workspace_id = ?", workspaceID).
		Order("workspace_teams.name").
		Select("pm_team_repo_defaults.*").
		Find(&defaults).Error
	if err != nil {
		return nil, fmt.Errorf("list team repo defaults: %w", err)
	}
	return defaults, nil
}

// GetTeamRepoDefault returns the repo default for a team.
func (r *SettingsRepository) GetTeamRepoDefault(ctx context.Context, teamID string) (*model.PMTeamRepoDefault, error) {
	var cfg model.PMTeamRepoDefault
	if err := r.db.WithContext(ctx).Where("team_id = ?", teamID).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get team repo default: %w", err)
	}
	return &cfg, nil
}

// UpsertTeamRepoDefault creates or updates the repo default for a team.
func (r *SettingsRepository) UpsertTeamRepoDefault(ctx context.Context, teamID string, req model.UpdateTeamRepoDefaultRequest) (*model.PMTeamRepoDefault, error) {
	cfg, err := r.GetTeamRepoDefault(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		cfg = &model.PMTeamRepoDefault{
			TeamID:         teamID,
			RepositoryID:   req.RepositoryID,
			BaseBranch:     "main",
			BranchTemplate: "tp-{display_id}-{slug}",
			AutoSyncStates: true,
		}
	}

	cfg.RepositoryID = req.RepositoryID
	if req.BaseBranch != nil && *req.BaseBranch != "" {
		cfg.BaseBranch = *req.BaseBranch
	}
	if req.BranchTemplate != nil && *req.BranchTemplate != "" {
		cfg.BranchTemplate = *req.BranchTemplate
	}
	if req.AutoSyncStates != nil {
		cfg.AutoSyncStates = *req.AutoSyncStates
	}
	cfg.ReviewStateID = req.ReviewStateID
	cfg.DoneStateID = req.DoneStateID

	if err := r.db.WithContext(ctx).Save(cfg).Error; err != nil {
		return nil, fmt.Errorf("upsert team repo default: %w", err)
	}
	return cfg, nil
}

// AddTeamUserMembership adds or updates a workspace member's team membership.
func (r *SettingsRepository) AddTeamUserMembership(ctx context.Context, teamID, userID, role string) (*model.TeamUserMembership, error) {
	var membership *model.TeamUserMembership
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		team, err := r.getTeamByIDTx(tx, teamID)
		if err != nil {
			return err
		}
		if team == nil {
			return fmt.Errorf("team not found")
		}

		var count int64
		if err := tx.Model(&model.WorkspaceMember{}).
			Where("workspace_id = ? AND user_id = ?", team.WorkspaceID, userID).
			Count(&count).Error; err != nil {
			return fmt.Errorf("check workspace membership: %w", err)
		}
		if count == 0 {
			return fmt.Errorf("user is not a member of this workspace")
		}

		entry := &model.TeamUserMembership{
			TeamID: teamID,
			UserID: userID,
			Role:   role,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "team_id"}, {Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"role", "updated_at"}),
		}).Create(entry).Error; err != nil {
			return fmt.Errorf("add team user membership: %w", err)
		}

		if err := r.syncPersonMembershipForUserTx(tx, team.WorkspaceID, teamID, userID, true); err != nil {
			return err
		}

		fetched := &model.TeamUserMembership{}
		if err := tx.Where("team_id = ? AND user_id = ?", teamID, userID).First(fetched).Error; err != nil {
			return fmt.Errorf("fetch team user membership: %w", err)
		}
		membership = fetched
		return nil
	})
	if err != nil {
		return nil, err
	}
	return membership, nil
}

// UpdateTeamUserMembership updates role for an existing team membership.
func (r *SettingsRepository) UpdateTeamUserMembership(ctx context.Context, teamID, userID string, req model.UpdateTeamMemberRequest) (*model.TeamUserMembership, error) {
	membership := &model.TeamUserMembership{}
	updates := map[string]interface{}{}
	if req.Role != nil {
		updates["role"] = *req.Role
	}
	if len(updates) == 0 {
		var err error
		membership, err = r.GetTeamUserMembership(ctx, teamID, userID)
		return membership, err
	}
	if err := r.db.WithContext(ctx).
		Model(&model.TeamUserMembership{}).
		Where("team_id = ? AND user_id = ?", teamID, userID).
		Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update team user membership: %w", err)
	}
	return r.GetTeamUserMembership(ctx, teamID, userID)
}

// GetTeamUserMembership loads a specific team membership.
func (r *SettingsRepository) GetTeamUserMembership(ctx context.Context, teamID, userID string) (*model.TeamUserMembership, error) {
	membership := &model.TeamUserMembership{}
	err := r.db.WithContext(ctx).
		Where("team_id = ? AND user_id = ?", teamID, userID).
		First(membership).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get team user membership: %w", err)
	}
	return membership, nil
}

// RemoveTeamUserMembership removes a user's team membership.
func (r *SettingsRepository) RemoveTeamUserMembership(ctx context.Context, teamID, userID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		team, err := r.getTeamByIDTx(tx, teamID)
		if err != nil {
			return err
		}
		if team == nil {
			return fmt.Errorf("team not found")
		}

		if err := tx.Where("team_id = ? AND user_id = ?", teamID, userID).Delete(&model.TeamUserMembership{}).Error; err != nil {
			return fmt.Errorf("remove team user membership: %w", err)
		}
		if err := r.syncPersonMembershipForUserTx(tx, team.WorkspaceID, teamID, userID, false); err != nil {
			return err
		}
		return nil
	})
}

func (r *SettingsRepository) getTeamByIDTx(tx *gorm.DB, id string) (*model.WorkspaceTeam, error) {
	team := &model.WorkspaceTeam{}
	err := tx.Where("id = ?", id).First(team).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get team: %w", err)
	}
	return team, nil
}

// Keep the legacy person-based team memberships aligned when a user-backed membership changes.
func (r *SettingsRepository) syncPersonMembershipForUserTx(tx *gorm.DB, workspaceID, teamID, userID string, add bool) error {
	var people []model.WorkspacePerson
	if err := tx.Where("workspace_id = ? AND user_id = ?", workspaceID, userID).Find(&people).Error; err != nil {
		return fmt.Errorf("find workspace people by user: %w", err)
	}
	for _, person := range people {
		if add {
			entry := &model.TeamMembership{TeamID: teamID, PersonID: person.ID}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(entry).Error; err != nil {
				return fmt.Errorf("sync person team membership: %w", err)
			}
			continue
		}
		if err := tx.Where("team_id = ? AND person_id = ?", teamID, person.ID).Delete(&model.TeamMembership{}).Error; err != nil {
			return fmt.Errorf("remove synced person team membership: %w", err)
		}
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

// listInvitationTeamPreassignments returns all preassignments for a workspace's teams.
func (r *SettingsRepository) listInvitationTeamPreassignments(ctx context.Context, workspaceID string) ([]model.InvitationTeamPreassignment, error) {
	var results []model.InvitationTeamPreassignment
	err := r.db.WithContext(ctx).
		Table("invitation_team_preassignments").
		Joins("JOIN workspace_teams ON invitation_team_preassignments.team_id = workspace_teams.id").
		Where("workspace_teams.workspace_id = ?", workspaceID).
		Select("invitation_team_preassignments.*").
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("list invitation team preassignments: %w", err)
	}
	return results, nil
}

// AddInvitationTeamPreassignment creates a preassignment.
func (r *SettingsRepository) AddInvitationTeamPreassignment(ctx context.Context, invitationID, teamID string) (*model.InvitationTeamPreassignment, error) {
	entry := &model.InvitationTeamPreassignment{
		InvitationID: invitationID,
		TeamID:       teamID,
	}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "invitation_id"}, {Name: "team_id"}},
		DoNothing: true,
	}).Create(entry).Error; err != nil {
		return nil, fmt.Errorf("add invitation team preassignment: %w", err)
	}
	return entry, nil
}

// RemoveInvitationTeamPreassignment removes a preassignment.
func (r *SettingsRepository) RemoveInvitationTeamPreassignment(ctx context.Context, invitationID, teamID string) error {
	result := r.db.WithContext(ctx).
		Where("invitation_id = ? AND team_id = ?", invitationID, teamID).
		Delete(&model.InvitationTeamPreassignment{})
	if result.Error != nil {
		return fmt.Errorf("remove invitation team preassignment: %w", result.Error)
	}
	return nil
}

// GetTeamEstimateSettings returns estimate settings for a team.
func (r *SettingsRepository) GetTeamEstimateSettings(ctx context.Context, teamID string) (*model.PMTeamEstimateSettings, error) {
	s := &model.PMTeamEstimateSettings{}
	err := r.db.WithContext(ctx).Where("team_id = ?", teamID).First(s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get team estimate settings: %w", err)
	}
	return s, nil
}

// UpsertTeamEstimateSettings creates or updates estimate settings for a team.
func (r *SettingsRepository) UpsertTeamEstimateSettings(ctx context.Context, teamID string, req model.UpdateTeamEstimateSettingsRequest) (*model.PMTeamEstimateSettings, error) {
	existing, err := r.GetTeamEstimateSettings(ctx, teamID)
	if err != nil {
		return nil, err
	}

	if existing == nil {
		s := &model.PMTeamEstimateSettings{TeamID: teamID}
		if req.Enabled != nil {
			s.Enabled = *req.Enabled
		}
		if req.Scale != nil {
			s.Scale = *req.Scale
		}
		if req.Extended != nil {
			s.Extended = *req.Extended
		}
		if req.AllowZero != nil {
			s.AllowZero = *req.AllowZero
		}
		if req.CountUnestimatedAsOne != nil {
			s.CountUnestimatedAsOne = *req.CountUnestimatedAsOne
		}
		if err := r.db.WithContext(ctx).Create(s).Error; err != nil {
			return nil, fmt.Errorf("create team estimate settings: %w", err)
		}
		return s, nil
	}

	updates := map[string]interface{}{}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.Scale != nil {
		updates["scale"] = *req.Scale
	}
	if req.Extended != nil {
		updates["extended"] = *req.Extended
	}
	if req.AllowZero != nil {
		updates["allow_zero"] = *req.AllowZero
	}
	if req.CountUnestimatedAsOne != nil {
		updates["count_unestimated_as_one"] = *req.CountUnestimatedAsOne
	}

	if len(updates) > 0 {
		if err := r.db.WithContext(ctx).Model(&model.PMTeamEstimateSettings{}).Where("team_id = ?", teamID).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("update team estimate settings: %w", err)
		}
	}

	return r.GetTeamEstimateSettings(ctx, teamID)
}

// listTeamEstimateSettings returns all estimate settings for a workspace's teams.
func (r *SettingsRepository) listTeamEstimateSettings(ctx context.Context, workspaceID string) ([]model.PMTeamEstimateSettings, error) {
	var results []model.PMTeamEstimateSettings
	err := r.db.WithContext(ctx).
		Table("pm_team_estimate_settings").
		Joins("JOIN workspace_teams ON pm_team_estimate_settings.team_id = workspace_teams.id").
		Where("workspace_teams.workspace_id = ?", workspaceID).
		Select("pm_team_estimate_settings.*").
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("list team estimate settings: %w", err)
	}
	return results, nil
}

// GetTeamFieldVisibility returns field visibility settings for a team.
func (r *SettingsRepository) GetTeamFieldVisibility(ctx context.Context, teamID string) (*model.PMTeamFieldVisibility, error) {
	s := &model.PMTeamFieldVisibility{}
	err := r.db.WithContext(ctx).Where("team_id = ?", teamID).First(s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get team field visibility: %w", err)
	}
	return s, nil
}

// UpsertTeamFieldVisibility creates or updates field visibility settings for a team.
func (r *SettingsRepository) UpsertTeamFieldVisibility(ctx context.Context, teamID string, req model.UpdateTeamFieldVisibilityRequest) (*model.PMTeamFieldVisibility, error) {
	existing, err := r.GetTeamFieldVisibility(ctx, teamID)
	if err != nil {
		return nil, err
	}

	if existing == nil {
		s := &model.PMTeamFieldVisibility{
			TeamID:    teamID,
			Priority:  true,
			StoryType: true,
			Severity:  true,
			Labels:    true,
			Epic:      true,
			Sprint:    true,
			Estimate:  true,
			DueDate:   true,
			Blocked:   true,
		}
		if req.Priority != nil {
			s.Priority = *req.Priority
		}
		if req.StoryType != nil {
			s.StoryType = *req.StoryType
		}
		if req.Severity != nil {
			s.Severity = *req.Severity
		}
		if req.Labels != nil {
			s.Labels = *req.Labels
		}
		if req.Epic != nil {
			s.Epic = *req.Epic
		}
		if req.Sprint != nil {
			s.Sprint = *req.Sprint
		}
		if req.Estimate != nil {
			s.Estimate = *req.Estimate
		}
		if req.DueDate != nil {
			s.DueDate = *req.DueDate
		}
		if req.Blocked != nil {
			s.Blocked = *req.Blocked
		}
		if err := r.db.WithContext(ctx).Create(s).Error; err != nil {
			return nil, fmt.Errorf("create team field visibility: %w", err)
		}
		return s, nil
	}

	updates := map[string]interface{}{}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if req.StoryType != nil {
		updates["story_type"] = *req.StoryType
	}
	if req.Severity != nil {
		updates["severity"] = *req.Severity
	}
	if req.Labels != nil {
		updates["labels"] = *req.Labels
	}
	if req.Epic != nil {
		updates["epic"] = *req.Epic
	}
	if req.Sprint != nil {
		updates["sprint"] = *req.Sprint
	}
	if req.Estimate != nil {
		updates["estimate"] = *req.Estimate
	}
	if req.DueDate != nil {
		updates["due_date"] = *req.DueDate
	}
	if req.Blocked != nil {
		updates["blocked"] = *req.Blocked
	}

	if len(updates) > 0 {
		if err := r.db.WithContext(ctx).Model(&model.PMTeamFieldVisibility{}).Where("team_id = ?", teamID).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("update team field visibility: %w", err)
		}
	}

	return r.GetTeamFieldVisibility(ctx, teamID)
}

// listTeamFieldVisibility returns all field visibility settings for a workspace's teams.
func (r *SettingsRepository) listTeamFieldVisibility(ctx context.Context, workspaceID string) ([]model.PMTeamFieldVisibility, error) {
	var results []model.PMTeamFieldVisibility
	err := r.db.WithContext(ctx).
		Table("pm_team_field_visibility").
		Joins("JOIN workspace_teams ON pm_team_field_visibility.team_id = workspace_teams.id").
		Where("workspace_teams.workspace_id = ?", workspaceID).
		Select("pm_team_field_visibility.*").
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("list team field visibility: %w", err)
	}
	return results, nil
}

// GetInvitationTeamPreassignmentsByInvitation returns all team preassignments for an invitation.
func (r *SettingsRepository) GetInvitationTeamPreassignmentsByInvitation(ctx context.Context, invitationID string) ([]model.InvitationTeamPreassignment, error) {
	var results []model.InvitationTeamPreassignment
	err := r.db.WithContext(ctx).
		Where("invitation_id = ?", invitationID).
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("get invitation team preassignments: %w", err)
	}
	return results, nil
}
