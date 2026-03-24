package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

// ListAdminOwnerUserIDs returns user IDs of active workspace members with admin or owner roles.
func (r *SettingsRepository) ListAdminOwnerUserIDs(ctx context.Context, workspaceID string) ([]string, error) {
	var userIDs []string
	err := r.db.WithContext(ctx).
		Table("workspace_members").
		Select("user_id").
		Where("workspace_id = ? AND status = ? AND role IN (?, ?) AND user_id IS NOT NULL", workspaceID, model.WorkspaceMemberStatusActive, model.RoleAdmin, model.RoleOwner).
		Scan(&userIDs).Error
	if err != nil {
		return nil, fmt.Errorf("list admin/owner user ids: %w", err)
	}
	return userIDs, nil
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

	workspaceMemberships, err := r.listWorkspaceMemberships(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	cfg.WorkspaceMemberships = workspaceMemberships

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

// GetWorkspaceSettings returns the workspace-level settings row without loading the full config graph.
func (r *SettingsRepository) GetWorkspaceSettings(ctx context.Context, workspaceID string) (*model.WorkspaceSettings, error) {
	return r.getSettings(ctx, workspaceID)
}

// ListTeams returns teams for a workspace without loading the full settings graph.
func (r *SettingsRepository) ListTeams(ctx context.Context, workspaceID string) ([]model.WorkspaceTeam, error) {
	return r.listTeams(ctx, workspaceID)
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
	err := r.db.WithContext(ctx).
		Table("workspace_members wm").
		Select(`
			wm.id,
			wm.workspace_id,
			wm.user_id,
			wm.display_name AS name,
			wm.email,
			COALESCE(NULLIF(rp.role, ''), 'employee') AS role,
			COALESCE(rp.job_role, '') AS job_role,
			rp.manager_member_id AS manager_id,
			COALESCE(rp.hire_date, '') AS hire_date,
			wm.status,
			COALESCE(rp.base_salary, 0) AS base_salary,
			COALESCE(rp.active_for_bonus, true) AS active_for_bonus,
			COALESCE(rp.active_for_evaluation, true) AS active_for_evaluation,
			COALESCE(rp.is_account_owner, false) AS is_account_owner
		`).
		Joins("LEFT JOIN reward_profiles rp ON rp.workspace_member_id = wm.id").
		Where("wm.workspace_id = ? AND wm.status <> ?", workspaceID, model.WorkspaceMemberStatusRevoked).
		Order("LOWER(wm.display_name), LOWER(wm.email)").
		Scan(&people).Error
	if err != nil {
		return nil, fmt.Errorf("list people: %w", err)
	}
	return people, nil
}

func (r *SettingsRepository) listMemberships(ctx context.Context, workspaceID string) ([]model.TeamMembership, error) {
	var memberships []model.TeamMembership
	err := r.db.WithContext(ctx).
		Table("team_workspace_memberships twm").
		Joins("JOIN workspace_teams wt ON twm.team_id = wt.id").
		Where("wt.workspace_id = ?", workspaceID).
		Order("twm.team_id, twm.workspace_member_id").
		Select("twm.id, twm.team_id, twm.workspace_member_id AS person_id").
		Find(&memberships).Error
	if err != nil {
		return nil, fmt.Errorf("list memberships: %w", err)
	}
	return memberships, nil
}

func (r *SettingsRepository) listUserMemberships(ctx context.Context, workspaceID string) ([]model.TeamUserMembership, error) {
	var memberships []model.TeamUserMembership
	err := r.db.WithContext(ctx).
		Table("team_workspace_memberships twm").
		Joins("JOIN workspace_teams wt ON twm.team_id = wt.id").
		Joins("JOIN workspace_members wm ON wm.id = twm.workspace_member_id").
		Where("wt.workspace_id = ? AND wm.user_id IS NOT NULL", workspaceID).
		Order("twm.team_id, wm.user_id").
		Select("twm.id, twm.team_id, wm.user_id, twm.role, twm.created_at, twm.updated_at").
		Find(&memberships).Error
	if err != nil {
		return nil, fmt.Errorf("list user memberships: %w", err)
	}
	return memberships, nil
}

func (r *SettingsRepository) listWorkspaceMemberships(ctx context.Context, workspaceID string) ([]model.TeamWorkspaceMembership, error) {
	var memberships []model.TeamWorkspaceMembership
	err := r.db.WithContext(ctx).
		Table("team_workspace_memberships").
		Joins("JOIN workspace_teams ON team_workspace_memberships.team_id = workspace_teams.id").
		Where("workspace_teams.workspace_id = ?", workspaceID).
		Order("team_workspace_memberships.team_id, team_workspace_memberships.workspace_member_id").
		Select("team_workspace_memberships.*").
		Find(&memberships).Error
	if err != nil {
		return nil, fmt.Errorf("list workspace memberships: %w", err)
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
	var team *model.WorkspaceTeam
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		managerID, err := r.resolveWorkspaceMemberReferenceTx(tx, req.WorkspaceID, req.ManagerID)
		if err != nil {
			return err
		}
		t := &model.WorkspaceTeam{
			WorkspaceID:      req.WorkspaceID,
			Name:             req.Name,
			Handle:           req.Handle,
			Description:      req.Description,
			ManagerID:        managerID,
			TeamType:         req.TeamType,
			DefaultStoryType: req.DefaultStoryType,
		}
		if err := tx.Create(t).Error; err != nil {
			return fmt.Errorf("create team: %w", err)
		}
		team = t
		return nil
	})
	if err != nil {
		return nil, err
	}
	return team, nil
}

// UpdateTeam modifies a team.
func (r *SettingsRepository) UpdateTeam(ctx context.Context, id string, req model.UpdateTeamRequest) (*model.WorkspaceTeam, error) {
	var team *model.WorkspaceTeam
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := r.getTeamByIDTx(tx, id)
		if err != nil {
			return err
		}
		if current == nil {
			return fmt.Errorf("team not found")
		}

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
			managerID, err := r.resolveWorkspaceMemberReferenceTx(tx, current.WorkspaceID, req.ManagerID)
			if err != nil {
				return err
			}
			updates["manager_id"] = managerID
		}
		if req.TeamType != nil {
			updates["team_type"] = *req.TeamType
		}
		if req.DefaultStoryType != nil {
			updates["default_story_type"] = *req.DefaultStoryType
		}

		if err := tx.Model(&model.WorkspaceTeam{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return fmt.Errorf("update team: %w", err)
		}

		team = &model.WorkspaceTeam{}
		if err := tx.Where("id = ?", id).First(team).Error; err != nil {
			return fmt.Errorf("update team: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return team, nil
}

// DeleteTeam removes a team by ID, including its workflow and states.
// Most FK constraints use ON DELETE CASCADE/SET NULL, so the DB handles
// related rows automatically. We only explicitly delete team workflows
// (and their states) to avoid orphaning them (ON DELETE SET NULL would
// leave them as workspace-level workflows).
func (r *SettingsRepository) DeleteTeam(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete workflow states for team-specific workflows, then the workflows themselves.
		var workflowIDs []string
		if err := tx.Model(&model.PMWorkflow{}).Where("team_id = ?", id).Pluck("id", &workflowIDs).Error; err != nil {
			return fmt.Errorf("find team workflows: %w", err)
		}
		if len(workflowIDs) > 0 {
			if err := tx.Where("workflow_id IN ?", workflowIDs).Delete(&model.PMWorkflowState{}).Error; err != nil {
				return fmt.Errorf("delete team workflow states: %w", err)
			}
			if err := tx.Where("id IN ?", workflowIDs).Delete(&model.PMWorkflow{}).Error; err != nil {
				return fmt.Errorf("delete team workflows: %w", err)
			}
		}
		// Delete the team — FK cascades handle memberships, estimate settings, etc.
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

		workspaceMember, err := r.getWorkspaceMemberByUserTx(tx, team.WorkspaceID, userID)
		if err != nil {
			return err
		}
		if workspaceMember == nil || workspaceMember.Status != model.WorkspaceMemberStatusActive {
			return fmt.Errorf("user is not a member of this workspace")
		}

		if err := r.syncWorkspaceMembershipTx(tx, teamID, workspaceMember.ID, role, true); err != nil {
			return err
		}
		fetched, err := r.getTeamUserMembershipTx(tx, teamID, userID)
		if err != nil {
			return err
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
	role := ""
	if req.Role != nil {
		role = *req.Role
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		team, err := r.getTeamByIDTx(tx, teamID)
		if err != nil {
			return err
		}
		if team == nil {
			return fmt.Errorf("team not found")
		}
		workspaceMember, err := r.getWorkspaceMemberByUserTx(tx, team.WorkspaceID, userID)
		if err != nil {
			return err
		}
		if workspaceMember == nil {
			return fmt.Errorf("user is not a member of this workspace")
		}
		return r.syncWorkspaceMembershipTx(tx, teamID, workspaceMember.ID, role, true)
	})
	if err != nil {
		return nil, err
	}
	return r.GetTeamUserMembership(ctx, teamID, userID)
}

// CountTeamMembersByRole counts team members with a given role.
func (r *SettingsRepository) CountTeamMembersByRole(ctx context.Context, teamID, role string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("team_workspace_memberships").
		Where("team_id = ? AND role = ?", teamID, role).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("count team members by role: %w", err)
	}
	return count, nil
}

// GetTeamUserMembership loads a specific team membership.
func (r *SettingsRepository) GetTeamUserMembership(ctx context.Context, teamID, userID string) (*model.TeamUserMembership, error) {
	return r.getTeamUserMembershipTx(r.db.WithContext(ctx), teamID, userID)
}

func (r *SettingsRepository) getTeamUserMembershipTx(tx *gorm.DB, teamID, userID string) (*model.TeamUserMembership, error) {
	membership := &model.TeamUserMembership{}
	err := tx.
		Table("team_workspace_memberships twm").
		Select("twm.id, twm.team_id, wm.user_id, twm.role, twm.created_at, twm.updated_at").
		Joins("JOIN workspace_members wm ON wm.id = twm.workspace_member_id").
		Where("twm.team_id = ? AND wm.user_id = ?", teamID, userID).
		Scan(membership).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get team user membership: %w", err)
	}
	if membership.ID == "" {
		return nil, nil
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

		workspaceMember, err := r.getWorkspaceMemberByUserTx(tx, team.WorkspaceID, userID)
		if err != nil {
			return err
		}

		if workspaceMember != nil {
			if err := r.syncWorkspaceMembershipTx(tx, teamID, workspaceMember.ID, "", false); err != nil {
				return err
			}
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

func (r *SettingsRepository) syncWorkspaceMembershipTx(tx *gorm.DB, teamID, workspaceMemberID, role string, add bool) error {
	if !add {
		if err := tx.Where("team_id = ? AND workspace_member_id = ?", teamID, workspaceMemberID).Delete(&model.TeamWorkspaceMembership{}).Error; err != nil {
			return fmt.Errorf("remove workspace member team membership: %w", err)
		}
		return nil
	}

	entry := &model.TeamWorkspaceMembership{
		TeamID:            teamID,
		WorkspaceMemberID: workspaceMemberID,
		Role:              role,
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "team_id"}, {Name: "workspace_member_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"role", "updated_at"}),
	}).Create(entry).Error; err != nil {
		return fmt.Errorf("sync workspace member team membership: %w", err)
	}
	return nil
}

func (r *SettingsRepository) getWorkspaceMemberByUserTx(tx *gorm.DB, workspaceID, userID string) (*model.WorkspaceMember, error) {
	member := &model.WorkspaceMember{}
	err := tx.Where("workspace_id = ? AND user_id = ?", workspaceID, userID).First(member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get workspace member by user: %w", err)
	}
	return member, nil
}

func (r *SettingsRepository) getWorkspaceMemberByEmailTx(tx *gorm.DB, workspaceID, email string) (*model.WorkspaceMember, error) {
	member := &model.WorkspaceMember{}
	err := tx.Where("workspace_id = ? AND LOWER(email) = LOWER(?)", workspaceID, strings.ToLower(strings.TrimSpace(email))).First(member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get workspace member by email: %w", err)
	}
	return member, nil
}

func (r *SettingsRepository) getWorkspaceMemberByIDTx(tx *gorm.DB, id string) (*model.WorkspaceMember, error) {
	member := &model.WorkspaceMember{}
	err := tx.Where("id = ?", id).First(member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get workspace member by id: %w", err)
	}
	return member, nil
}

func (r *SettingsRepository) resolveWorkspaceMemberReferenceTx(tx *gorm.DB, workspaceID string, reference *string) (*string, error) {
	if reference == nil || strings.TrimSpace(*reference) == "" {
		return nil, nil
	}

	member, err := r.getWorkspaceMemberByIDTx(tx, *reference)
	if err != nil {
		return nil, err
	}
	if member != nil && member.WorkspaceID == workspaceID {
		return reference, nil
	}
	return nil, fmt.Errorf("workspace member not found")
}

func (r *SettingsRepository) upsertWorkspaceMemberIdentityTx(tx *gorm.DB, workspaceID string, userID *string, email, displayName, status string) (*model.WorkspaceMember, error) {
	var member *model.WorkspaceMember
	var err error
	if userID != nil && *userID != "" {
		member, err = r.getWorkspaceMemberByUserTx(tx, workspaceID, *userID)
		if err != nil {
			return nil, err
		}
	}
	if member == nil && strings.TrimSpace(email) != "" {
		member, err = r.getWorkspaceMemberByEmailTx(tx, workspaceID, email)
		if err != nil {
			return nil, err
		}
	}

	email = strings.ToLower(strings.TrimSpace(email))
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = email
	}
	if status == "" {
		status = model.WorkspaceMemberStatusActive
	}

	if member == nil {
		member = &model.WorkspaceMember{
			WorkspaceID: workspaceID,
			UserID:      userID,
			Email:       email,
			DisplayName: displayName,
			Role:        model.RoleMember,
			Status:      status,
		}
		if err := tx.Create(member).Error; err != nil {
			return nil, fmt.Errorf("create workspace member identity: %w", err)
		}
		return member, nil
	}

	member.UserID = userID
	member.Email = email
	member.DisplayName = displayName
	if member.Role == "" {
		member.Role = model.RoleMember
	}
	member.Status = status
	if err := tx.Save(member).Error; err != nil {
		return nil, fmt.Errorf("update workspace member identity: %w", err)
	}
	return member, nil
}

func (r *SettingsRepository) upsertRewardProfileTx(tx *gorm.DB, workspaceMemberID, role, jobRole string, managerMemberID *string, hireDate string, baseSalary float64, activeForBonus, activeForEvaluation, isAccountOwner bool) error {
	profile := &model.RewardProfile{}
	err := tx.Where("workspace_member_id = ?", workspaceMemberID).First(profile).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("load reward profile: %w", err)
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		profile = &model.RewardProfile{
			WorkspaceMemberID:   workspaceMemberID,
			Role:                role,
			JobRole:             jobRole,
			ActiveForBonus:      activeForBonus,
			ActiveForEvaluation: activeForEvaluation,
			IsAccountOwner:      isAccountOwner,
		}
	} else {
		profile.Role = role
		profile.JobRole = jobRole
		profile.ActiveForBonus = activeForBonus
		profile.ActiveForEvaluation = activeForEvaluation
		profile.IsAccountOwner = isAccountOwner
	}
	if role == "" {
		profile.Role = "employee"
	}
	profile.ManagerMemberID = managerMemberID
	if strings.TrimSpace(hireDate) != "" {
		profile.HireDate = &hireDate
	} else {
		profile.HireDate = nil
	}
	profile.BaseSalary = baseSalary
	if err := tx.Save(profile).Error; err != nil {
		return fmt.Errorf("save reward profile: %w", err)
	}
	return nil
}

func (r *SettingsRepository) getWorkspacePersonByIDTx(tx *gorm.DB, id string) (*model.WorkspacePerson, error) {
	person := &model.WorkspacePerson{}
	err := tx.
		Table("workspace_members wm").
		Select(`
			wm.id,
			wm.workspace_id,
			wm.user_id,
			wm.display_name AS name,
			wm.email,
			COALESCE(NULLIF(rp.role, ''), 'employee') AS role,
			COALESCE(rp.job_role, '') AS job_role,
			rp.manager_member_id AS manager_id,
			COALESCE(rp.hire_date, '') AS hire_date,
			wm.status,
			COALESCE(rp.base_salary, 0) AS base_salary,
			COALESCE(rp.active_for_bonus, true) AS active_for_bonus,
			COALESCE(rp.active_for_evaluation, true) AS active_for_evaluation,
			COALESCE(rp.is_account_owner, false) AS is_account_owner
		`).
		Joins("LEFT JOIN reward_profiles rp ON rp.workspace_member_id = wm.id").
		Where("wm.id = ?", id).
		Scan(person).Error
	if err != nil {
		return nil, fmt.Errorf("get workspace person by id: %w", err)
	}
	if person.ID == "" {
		return nil, nil
	}
	return person, nil
}

// CreatePerson inserts a new person and optionally assigns them to teams.
func (r *SettingsRepository) CreatePerson(ctx context.Context, req model.CreatePersonRequest) (*model.WorkspacePerson, error) {
	var person *model.WorkspacePerson

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		workspaceMember, err := r.upsertWorkspaceMemberIdentityTx(tx, req.WorkspaceID, req.UserID, req.Email, req.Name, model.WorkspaceMemberStatusActive)
		if err != nil {
			return err
		}
		managerID, err := r.resolveWorkspaceMemberReferenceTx(tx, req.WorkspaceID, req.ManagerID)
		if err != nil {
			return err
		}
		if err := r.upsertRewardProfileTx(tx, workspaceMember.ID, req.Role, req.JobRole, managerID, req.HireDate, req.BaseSalary, req.ActiveForBonus, req.ActiveForEvaluation, req.IsAccountOwner); err != nil {
			return err
		}
		if req.TeamIDs != nil {
			if err := tx.Where("workspace_member_id = ?", workspaceMember.ID).Delete(&model.TeamWorkspaceMembership{}).Error; err != nil {
				return fmt.Errorf("clear team memberships: %w", err)
			}
			for _, teamID := range req.TeamIDs {
				if err := r.syncWorkspaceMembershipTx(tx, teamID, workspaceMember.ID, "member", true); err != nil {
					return err
				}
			}
		}

		person, err = r.getWorkspacePersonByIDTx(tx, workspaceMember.ID)
		if err != nil {
			return err
		}
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
		workspaceMember, err := r.getWorkspaceMemberByIDTx(tx, id)
		if err != nil {
			return err
		}
		if workspaceMember == nil {
			return fmt.Errorf("person not found")
		}

		memberUpdates := map[string]interface{}{}
		if req.Name != nil {
			memberUpdates["display_name"] = *req.Name
		}
		if req.Email != nil {
			memberUpdates["email"] = strings.ToLower(strings.TrimSpace(*req.Email))
		}
		if req.Status != nil {
			memberUpdates["status"] = *req.Status
		}
		if len(memberUpdates) > 0 {
			if err := tx.Model(&model.WorkspaceMember{}).Where("id = ?", id).Updates(memberUpdates).Error; err != nil {
				return fmt.Errorf("update workspace member: %w", err)
			}
		}

		profile := &model.RewardProfile{}
		if err := tx.Where("workspace_member_id = ?", id).First(profile).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("load reward profile: %w", err)
			}
			profile = &model.RewardProfile{
				WorkspaceMemberID:   id,
				Role:                "employee",
				JobRole:             "",
				ActiveForBonus:      true,
				ActiveForEvaluation: true,
			}
		}
		if req.Role != nil {
			profile.Role = *req.Role
		}
		if req.JobRole != nil {
			profile.JobRole = *req.JobRole
		}
		if req.ManagerID != nil {
			managerID, err := r.resolveWorkspaceMemberReferenceTx(tx, workspaceMember.WorkspaceID, req.ManagerID)
			if err != nil {
				return err
			}
			profile.ManagerMemberID = managerID
		}
		if req.HireDate != nil {
			profile.HireDate = req.HireDate
		}
		if req.BaseSalary != nil {
			profile.BaseSalary = *req.BaseSalary
		}
		if req.ActiveForBonus != nil {
			profile.ActiveForBonus = *req.ActiveForBonus
		}
		if req.ActiveForEvaluation != nil {
			profile.ActiveForEvaluation = *req.ActiveForEvaluation
		}
		if req.IsAccountOwner != nil {
			profile.IsAccountOwner = *req.IsAccountOwner
		}
		if err := tx.Save(profile).Error; err != nil {
			return fmt.Errorf("update reward profile: %w", err)
		}

		// Replace team memberships if provided.
		if req.TeamIDs != nil {
			if err := tx.Where("workspace_member_id = ?", id).Delete(&model.TeamWorkspaceMembership{}).Error; err != nil {
				return fmt.Errorf("clear team memberships: %w", err)
			}
			for _, teamID := range req.TeamIDs {
				if err := r.syncWorkspaceMembershipTx(tx, teamID, id, "member", true); err != nil {
					return err
				}
			}
		}

		person, err = r.getWorkspacePersonByIDTx(tx, id)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return person, nil
}

// DeletePerson removes a person by ID.
func (r *SettingsRepository) DeletePerson(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		workspaceMember, err := r.getWorkspaceMemberByIDTx(tx, id)
		if err != nil {
			return err
		}
		if workspaceMember == nil {
			return fmt.Errorf("person not found")
		}
		if workspaceMember.UserID != nil && workspaceMember.Status == model.WorkspaceMemberStatusActive {
			return fmt.Errorf("joined workspace members cannot be deleted from People; remove them from Members instead")
		}
		if err := tx.Where("workspace_member_id = ?", id).Delete(&model.TeamWorkspaceMembership{}).Error; err != nil {
			return fmt.Errorf("delete team memberships: %w", err)
		}
		if err := tx.Model(&model.WorkspaceTeam{}).Where("manager_id = ?", id).Update("manager_id", nil).Error; err != nil {
			return fmt.Errorf("clear team manager references: %w", err)
		}
		if err := tx.Model(&model.WorkspaceManager{}).Where("reporting_to = ?", id).Update("reporting_to", nil).Error; err != nil {
			return fmt.Errorf("clear manager reporting references: %w", err)
		}
		if err := tx.Where("person_id = ?", id).Delete(&model.WorkspaceManager{}).Error; err != nil {
			return fmt.Errorf("delete workspace manager rows: %w", err)
		}
		if err := tx.Where("workspace_member_id = ?", id).Delete(&model.RewardProfile{}).Error; err != nil {
			return fmt.Errorf("delete reward profile: %w", err)
		}
		if err := tx.Where("id = ?", id).Delete(&model.WorkspaceMember{}).Error; err != nil {
			return fmt.Errorf("delete workspace member: %w", err)
		}
		return nil
	})
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
		boolVal := func(p *bool, fallback bool) bool {
			if p != nil {
				return *p
			}
			return fallback
		}
		row := map[string]interface{}{
			"team_id":     teamID,
			"priority":    boolVal(req.Priority, true),
			"story_type":  boolVal(req.StoryType, true),
			"severity":    boolVal(req.Severity, true),
			"labels":      boolVal(req.Labels, true),
			"epic":        boolVal(req.Epic, true),
			"sprint":      boolVal(req.Sprint, true),
			"estimate":    boolVal(req.Estimate, true),
			"due_date":    boolVal(req.DueDate, true),
			"blocked":     boolVal(req.Blocked, true),
			"delivery":    boolVal(req.Delivery, true),
			"dev_history": boolVal(req.DevHistory, true),
		}
		if err := r.db.WithContext(ctx).Model(&model.PMTeamFieldVisibility{}).Create(row).Error; err != nil {
			return nil, fmt.Errorf("create team field visibility: %w", err)
		}
		// Fetch the created record to return
		created, err := r.GetTeamFieldVisibility(ctx, teamID)
		if err != nil {
			return nil, err
		}
		return created, nil
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
	if req.Delivery != nil {
		updates["delivery"] = *req.Delivery
	}
	if req.DevHistory != nil {
		updates["dev_history"] = *req.DevHistory
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
