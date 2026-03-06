package model

import "time"

// WorkspaceSettings represents a row in the workspace_settings table.
type WorkspaceSettings struct {
	ID                   string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID          string    `json:"workspace_id" gorm:"type:uuid;uniqueIndex;not null"`
	QuarterStartDate     *string   `json:"quarter_start_date"`
	SprintDurationWeeks  int       `json:"sprint_duration_weeks" gorm:"not null;default:2"`
	NotificationsEnabled bool      `json:"notifications_enabled" gorm:"not null;default:true"`
	AutoCalculateBonuses bool      `json:"auto_calculate_bonuses" gorm:"not null;default:false"`
	TeamWeight           int       `json:"team_weight" gorm:"not null;default:50"`
	CreatedAt            time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt            time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (WorkspaceSettings) TableName() string { return "workspace_settings" }

// WorkspaceTeam represents a row in the workspace_teams table.
type WorkspaceTeam struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_workspace_team_handle,priority:1"`
	Name        string    `json:"name" gorm:"not null"`
	Handle      *string   `json:"handle" gorm:"uniqueIndex:idx_workspace_team_handle,priority:2"`
	Description *string   `json:"description"`
	ManagerID   *string   `json:"manager_id" gorm:"type:uuid"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (WorkspaceTeam) TableName() string { return "workspace_teams" }

// WorkspacePerson represents a row in the workspace_people table.
type WorkspacePerson struct {
	ID                  string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID         string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	UserID              *string   `json:"user_id" gorm:"type:uuid"`
	Name                string    `json:"name" gorm:"not null"`
	Email               string    `json:"email" gorm:"not null"`
	Role                string    `json:"role" gorm:"not null"`
	JobRole             string    `json:"job_role" gorm:"not null"`
	ManagerID           *string   `json:"manager_id" gorm:"type:uuid"`
	HireDate            string    `json:"hire_date" gorm:"not null"`
	Status              string    `json:"status" gorm:"not null;default:'active'"`
	BaseSalary          float64   `json:"base_salary" gorm:"not null;default:0"`
	ActiveForBonus      bool      `json:"active_for_bonus" gorm:"not null;default:true"`
	ActiveForEvaluation bool      `json:"active_for_evaluation" gorm:"not null;default:true"`
	IsAccountOwner      bool      `json:"is_account_owner" gorm:"not null;default:false"`
	CreatedAt           time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (WorkspacePerson) TableName() string { return "workspace_people" }

// TeamMembership represents a row in the team_memberships table.
type TeamMembership struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TeamID    string    `json:"team_id" gorm:"type:uuid;not null;uniqueIndex:idx_team_membership_unique"`
	PersonID  string    `json:"person_id" gorm:"type:uuid;not null;uniqueIndex:idx_team_membership_unique"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (TeamMembership) TableName() string { return "team_memberships" }

// TeamUserMembership links an authenticated workspace member to a team.
type TeamUserMembership struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TeamID    string    `json:"team_id" gorm:"type:uuid;not null;uniqueIndex:idx_team_user_membership_unique"`
	UserID    string    `json:"user_id" gorm:"type:uuid;not null;uniqueIndex:idx_team_user_membership_unique"`
	Role      string    `json:"role" gorm:"not null;default:'member'"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (TeamUserMembership) TableName() string { return "team_user_memberships" }

// WorkspaceManager represents a row in the workspace_managers table.
type WorkspaceManager struct {
	ID                  string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID         string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	PersonID            string    `json:"person_id" gorm:"type:uuid;not null"`
	CanCreateGoals      bool      `json:"can_create_goals" gorm:"not null;default:false"`
	CanScorePerformance bool      `json:"can_score_performance" gorm:"not null;default:false"`
	ReportingTo         *string   `json:"reporting_to" gorm:"type:uuid"`
	CreatedAt           time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (WorkspaceManager) TableName() string { return "workspace_managers" }

// JobRoleCriteria represents a row in the job_role_criteria table.
type JobRoleCriteria struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	JobRole     string    `json:"job_role" gorm:"not null"`
	CriteriaID  string    `json:"criteria_id" gorm:"not null"`
	Name        string    `json:"name" gorm:"not null"`
	Description *string   `json:"description"`
	Question    string    `json:"question" gorm:"not null"`
	Enabled     bool      `json:"enabled" gorm:"not null;default:true"`
	Weight      int       `json:"weight" gorm:"not null;default:0"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (JobRoleCriteria) TableName() string { return "job_role_criteria" }

// BonusTier represents a row in the bonus_tiers table.
type BonusTier struct {
	ID               string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string    `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_bonus_tier_ws_tier"`
	Tier             string    `json:"tier" gorm:"not null;uniqueIndex:idx_bonus_tier_ws_tier"`
	MinScore         int       `json:"min_score" gorm:"not null;default:0"`
	MaxScore         int       `json:"max_score" gorm:"not null;default:100"`
	SalaryMultiplier float64   `json:"salary_multiplier" gorm:"not null;default:0"`
	Description      *string   `json:"description"`
	Editable         bool      `json:"editable" gorm:"not null;default:true"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (BonusTier) TableName() string { return "bonus_tiers" }

// PMTeamEstimateSettings stores per-team estimate configuration.
type PMTeamEstimateSettings struct {
	ID                    string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TeamID                string    `json:"team_id" gorm:"type:uuid;not null;uniqueIndex"`
	Enabled               bool      `json:"enabled" gorm:"not null;default:false"`
	Scale                 string    `json:"scale" gorm:"not null;default:'linear'"`
	Extended              bool      `json:"extended" gorm:"not null;default:false"`
	AllowZero             bool      `json:"allow_zero" gorm:"not null;default:false"`
	CountUnestimatedAsOne bool      `json:"count_unestimated_as_one" gorm:"not null;default:true"`
	CreatedAt             time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt             time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMTeamEstimateSettings) TableName() string { return "pm_team_estimate_settings" }

// UpdateTeamEstimateSettingsRequest is the payload for updating team estimate settings.
type UpdateTeamEstimateSettingsRequest struct {
	Enabled               *bool   `json:"enabled"`
	Scale                 *string `json:"scale"`
	Extended              *bool   `json:"extended"`
	AllowZero             *bool   `json:"allow_zero"`
	CountUnestimatedAsOne *bool   `json:"count_unestimated_as_one"`
}

// PMTeamFieldVisibility stores per-team field visibility configuration.
type PMTeamFieldVisibility struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TeamID    string    `json:"team_id" gorm:"type:uuid;not null;uniqueIndex"`
	Priority  bool      `json:"priority" gorm:"not null;default:true"`
	StoryType bool      `json:"story_type" gorm:"not null;default:true"`
	Severity  bool      `json:"severity" gorm:"not null;default:true"`
	Labels    bool      `json:"labels" gorm:"not null;default:true"`
	Epic      bool      `json:"epic" gorm:"not null;default:true"`
	Sprint    bool      `json:"sprint" gorm:"not null;default:true"`
	Estimate  bool      `json:"estimate" gorm:"not null;default:true"`
	DueDate   bool      `json:"due_date" gorm:"not null;default:true"`
	Blocked   bool      `json:"blocked" gorm:"not null;default:true"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMTeamFieldVisibility) TableName() string { return "pm_team_field_visibility" }

// UpdateTeamFieldVisibilityRequest is the payload for updating team field visibility.
type UpdateTeamFieldVisibilityRequest struct {
	Priority  *bool `json:"priority"`
	StoryType *bool `json:"story_type"`
	Severity  *bool `json:"severity"`
	Labels    *bool `json:"labels"`
	Epic      *bool `json:"epic"`
	Sprint    *bool `json:"sprint"`
	Estimate  *bool `json:"estimate"`
	DueDate   *bool `json:"due_date"`
	Blocked   *bool `json:"blocked"`
}

// InvitationTeamPreassignment pre-assigns a pending invitation to a team.
// When the invitation is accepted, the user is automatically added to the team.
type InvitationTeamPreassignment struct {
	ID           string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	InvitationID string    `json:"invitation_id" gorm:"type:uuid;not null;uniqueIndex:idx_itp_inv_team"`
	TeamID       string    `json:"team_id" gorm:"type:uuid;not null;uniqueIndex:idx_itp_inv_team;index"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (InvitationTeamPreassignment) TableName() string { return "invitation_team_preassignments" }

// FullWorkspaceConfig aggregates all settings for a workspace.
type FullWorkspaceConfig struct {
	Settings                       *WorkspaceSettings             `json:"settings"`
	Teams                          []WorkspaceTeam                `json:"teams"`
	People                         []WorkspacePerson              `json:"people"`
	Memberships                    []TeamMembership               `json:"memberships"`
	UserMemberships                []TeamUserMembership           `json:"user_memberships"`
	Managers                       []WorkspaceManager             `json:"managers"`
	JobRoles                       []JobRoleCriteria              `json:"job_roles"`
	BonusTiers                     []BonusTier                    `json:"bonus_tiers"`
	InvitationTeamPreassignments   []InvitationTeamPreassignment  `json:"invitation_team_preassignments"`
	TeamEstimateSettings           []PMTeamEstimateSettings       `json:"team_estimate_settings"`
	TeamFieldVisibility            []PMTeamFieldVisibility        `json:"team_field_visibility"`
}

// CreateTeamRequest is the payload for creating a team.
type CreateTeamRequest struct {
	WorkspaceID string  `json:"workspace_id"`
	Name        string  `json:"name"`
	Handle      *string `json:"handle"`
	Description *string `json:"description"`
	ManagerID   *string `json:"manager_id"`
}

// UpdateTeamRequest is the payload for updating a team.
type UpdateTeamRequest struct {
	Name        *string `json:"name"`
	Handle      *string `json:"handle"`
	Description *string `json:"description"`
	ManagerID   *string `json:"manager_id"`
}

// AddTeamMemberRequest is the payload for adding a workspace member to a team.
type AddTeamMemberRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// UpdateTeamMemberRequest is the payload for updating a team membership.
type UpdateTeamMemberRequest struct {
	Role *string `json:"role"`
}

// CreatePersonRequest is the payload for creating a person.
type CreatePersonRequest struct {
	WorkspaceID         string   `json:"workspace_id"`
	Name                string   `json:"name"`
	Email               string   `json:"email"`
	Role                string   `json:"role"`
	JobRole             string   `json:"job_role"`
	ManagerID           *string  `json:"manager_id"`
	HireDate            string   `json:"hire_date"`
	BaseSalary          float64  `json:"base_salary"`
	ActiveForBonus      bool     `json:"active_for_bonus"`
	ActiveForEvaluation bool     `json:"active_for_evaluation"`
	IsAccountOwner      bool     `json:"is_account_owner"`
	TeamIDs             []string `json:"team_ids"`
	UserID              *string  `json:"user_id"`
}

// UpdatePersonRequest is the payload for updating a person.
type UpdatePersonRequest struct {
	Name                *string  `json:"name"`
	Email               *string  `json:"email"`
	Role                *string  `json:"role"`
	JobRole             *string  `json:"job_role"`
	ManagerID           *string  `json:"manager_id"`
	HireDate            *string  `json:"hire_date"`
	Status              *string  `json:"status"`
	BaseSalary          *float64 `json:"base_salary"`
	ActiveForBonus      *bool    `json:"active_for_bonus"`
	ActiveForEvaluation *bool    `json:"active_for_evaluation"`
	IsAccountOwner      *bool    `json:"is_account_owner"`
	TeamIDs             []string `json:"team_ids"`
}

// UpdateBonusTiersRequest is the payload for updating bonus tiers.
type UpdateBonusTiersRequest struct {
	WorkspaceID string          `json:"workspace_id"`
	Tiers       []BonusTierItem `json:"tiers"`
}

// BonusTierItem is a single tier in the update request.
type BonusTierItem struct {
	Tier             string  `json:"tier"`
	MinScore         int     `json:"min_score"`
	MaxScore         int     `json:"max_score"`
	SalaryMultiplier float64 `json:"salary_multiplier"`
	Description      *string `json:"description"`
}

// UpdateJobRoleCriteriaRequest is the payload for upserting job role criteria.
type UpdateJobRoleCriteriaRequest struct {
	WorkspaceID string                `json:"workspace_id"`
	JobRole     string                `json:"job_role"`
	Criteria    []JobRoleCriteriaItem `json:"criteria"`
}

// JobRoleCriteriaItem is a single criterion in the update request.
type JobRoleCriteriaItem struct {
	CriteriaID  string  `json:"criteria_id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Question    *string `json:"question"`
	Enabled     bool    `json:"enabled"`
	Weight      int     `json:"weight"`
}

// UpdateSystemSettingsRequest is the payload for updating workspace system settings.
type UpdateSystemSettingsRequest struct {
	SprintDurationWeeks  *int  `json:"sprint_duration_weeks"`
	NotificationsEnabled *bool `json:"notifications_enabled"`
	AutoCalculateBonuses *bool `json:"auto_calculate_bonuses"`
	TeamWeight           *int  `json:"team_weight"`
}

// InitializeSettingsRequest is the payload for initializing workspace settings.
type InitializeSettingsRequest struct {
	WorkspaceID string `json:"workspace_id"`
}
