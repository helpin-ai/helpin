package model

import (
	"time"
)

const (
	PlanningMethodologyStructuredV1 = "structured_v1"
	PlanningMethodologyBasicV1      = "basic_v1"
)

func NormalizePlanningMethodology(value string) string {
	switch value {
	case "", PlanningMethodologyStructuredV1:
		return PlanningMethodologyStructuredV1
	case PlanningMethodologyBasicV1:
		return PlanningMethodologyBasicV1
	default:
		return PlanningMethodologyStructuredV1
	}
}

// WorkspaceSettings represents a row in the workspace_settings table.
type WorkspaceSettings struct {
	ID                        string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID               string    `json:"workspace_id" gorm:"type:uuid;uniqueIndex;not null"`
	QuarterStartDate          *string   `json:"quarter_start_date"`
	SprintDurationWeeks       int       `json:"sprint_duration_weeks" gorm:"not null;default:2"`
	NotificationsEnabled      bool      `json:"notifications_enabled" gorm:"not null;default:true"`
	AutoCalculateBonuses      bool      `json:"auto_calculate_bonuses" gorm:"not null;default:false"`
	TeamWeight                int       `json:"team_weight" gorm:"not null;default:50"`
	CreatedAt                 time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                 time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (WorkspaceSettings) TableName() string { return "workspace_settings" }

// Team type values used on WorkspaceTeam.TeamType. The column is a free-form
// string for forward-compatibility, but the recognized set lives here so the
// service layer can validate input and seed type-specific defaults (workflow
// states, default task type).
const (
	TeamTypeEngineering = "engineering"
	TeamTypeProduct     = "product"
	TeamTypeDesign      = "design"
	TeamTypeSupport     = "support"
	TeamTypeMarketing   = "marketing"
	TeamTypeSales       = "sales"
	TeamTypeHR          = "hr"
	TeamTypeOperations  = "operations"
	TeamTypeCustom      = "custom"
)

// WorkspaceTeam represents a row in the workspace_teams table.
type WorkspaceTeam struct {
	ID                   string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID          string    `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_workspace_team_handle,priority:1"`
	Name                 string    `json:"name" gorm:"not null"`
	Handle               *string   `json:"handle" gorm:"uniqueIndex:idx_workspace_team_handle,priority:2"`
	Description          *string   `json:"description"`
	ManagerID            *string   `json:"manager_id" gorm:"type:uuid"`
	TeamType             string    `json:"team_type" gorm:"not null;default:'engineering'"`
	DefaultStoryType     string    `json:"default_task_type" gorm:"column:default_task_type;not null;default:'feature'"`
	DocsPublisherEnabled bool      `json:"docs_publisher_enabled" gorm:"not null;default:false"`
	SprintsEnabled       bool      `json:"sprints_enabled" gorm:"not null;default:true"`
	CreatedAt            time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt            time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (WorkspaceTeam) TableName() string { return "workspace_teams" }

// WorkspacePerson is the Settings "People" response DTO.
// It is hydrated from workspace_members + reward_profiles.
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

// TeamMembership is a compatibility DTO for person-based team membership data.
// It is backed by team_workspace_memberships.
type TeamMembership struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TeamID    string    `json:"team_id" gorm:"type:uuid;not null;uniqueIndex:idx_team_membership_unique"`
	PersonID  string    `json:"person_id" gorm:"type:uuid;not null;uniqueIndex:idx_team_membership_unique"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TeamUserMembership is a compatibility DTO for user-based team membership
// responses. It is synthesized from team_workspace_memberships +
// workspace_members when a linked user account exists.
type TeamUserMembership struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TeamID    string    `json:"team_id" gorm:"type:uuid;not null;uniqueIndex:idx_team_user_membership_unique"`
	UserID    string    `json:"user_id" gorm:"type:uuid;not null;uniqueIndex:idx_team_user_membership_unique"`
	Role      string    `json:"role" gorm:"not null;default:'member'"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TeamWorkspaceMembership links a workspace member identity to a team.
type TeamWorkspaceMembership struct {
	ID                string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TeamID            string    `json:"team_id" gorm:"type:uuid;not null;uniqueIndex:idx_team_workspace_member_unique"`
	WorkspaceMemberID string    `json:"workspace_member_id" gorm:"type:uuid;not null;uniqueIndex:idx_team_workspace_member_unique"`
	Role              string    `json:"role" gorm:"not null;default:'member'"`
	CreatedAt         time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (TeamWorkspaceMembership) TableName() string { return "team_workspace_memberships" }

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
	ID         string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TeamID     string    `json:"team_id" gorm:"type:uuid;not null;uniqueIndex"`
	Priority   bool      `json:"priority" gorm:"not null;default:true"`
	StoryType  bool      `json:"task_type" gorm:"column:task_type;not null;default:true"`
	Severity   bool      `json:"severity" gorm:"not null;default:true"`
	Labels     bool      `json:"labels" gorm:"not null;default:true"`
	Epic       bool      `json:"epic" gorm:"not null;default:true"`
	Sprint     bool      `json:"sprint" gorm:"not null;default:true"`
	Estimate   bool      `json:"estimate" gorm:"not null;default:true"`
	DueDate    bool      `json:"due_date" gorm:"not null;default:true"`
	Blocked    bool      `json:"blocked" gorm:"not null;default:true"`
	Delivery   bool      `json:"delivery" gorm:"not null;default:true"`
	DevHistory bool      `json:"dev_history" gorm:"not null;default:true"`
	CreatedAt  time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMTeamFieldVisibility) TableName() string { return "pm_team_field_visibility" }

// UpdateTeamFieldVisibilityRequest is the payload for updating team field visibility.
type UpdateTeamFieldVisibilityRequest struct {
	Priority   *bool `json:"priority"`
	StoryType  *bool `json:"task_type"`
	Severity   *bool `json:"severity"`
	Labels     *bool `json:"labels"`
	Epic       *bool `json:"epic"`
	Sprint     *bool `json:"sprint"`
	Estimate   *bool `json:"estimate"`
	DueDate    *bool `json:"due_date"`
	Blocked    *bool `json:"blocked"`
	Delivery   *bool `json:"delivery"`
	DevHistory *bool `json:"dev_history"`
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

// RewardProfile holds reward- and HR-specific member metadata separately from PM identity.
type RewardProfile struct {
	ID                  string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceMemberID   string    `json:"workspace_member_id" gorm:"type:uuid;not null;uniqueIndex"`
	ManagerMemberID     *string   `json:"manager_member_id" gorm:"type:uuid"`
	Role                string    `json:"role" gorm:"not null;default:'employee'"`
	JobRole             string    `json:"job_role" gorm:"not null;default:''"`
	HireDate            *string   `json:"hire_date"`
	BaseSalary          float64   `json:"base_salary" gorm:"not null;default:0"`
	ActiveForBonus      bool      `json:"active_for_bonus" gorm:"not null;default:true"`
	ActiveForEvaluation bool      `json:"active_for_evaluation" gorm:"not null;default:true"`
	IsAccountOwner      bool      `json:"is_account_owner" gorm:"not null;default:false"`
	CreatedAt           time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (RewardProfile) TableName() string { return "reward_profiles" }

// FullWorkspaceConfig aggregates all settings for a workspace.
type FullWorkspaceConfig struct {
	Settings                     *WorkspaceSettings            `json:"settings"`
	Teams                        []WorkspaceTeam               `json:"teams"`
	People                       []WorkspacePerson             `json:"people"`
	Memberships                  []TeamMembership              `json:"memberships"`
	UserMemberships              []TeamUserMembership          `json:"user_memberships"`
	WorkspaceMemberships         []TeamWorkspaceMembership     `json:"workspace_memberships"`
	Managers                     []WorkspaceManager            `json:"managers"`
	JobRoles                     []JobRoleCriteria             `json:"job_roles"`
	InvitationTeamPreassignments []InvitationTeamPreassignment `json:"invitation_team_preassignments"`
	TeamEstimateSettings         []PMTeamEstimateSettings      `json:"team_estimate_settings"`
	TeamFieldVisibility          []PMTeamFieldVisibility       `json:"team_field_visibility"`
	TeamRepoDefaults             []PMTeamRepoDefault           `json:"team_repo_defaults"`
}

// CreateTeamRequest is the payload for creating a team.
type CreateTeamRequest struct {
	WorkspaceID      string  `json:"workspace_id"`
	Name             string  `json:"name"`
	Handle           *string `json:"handle"`
	Description      *string `json:"description"`
	ManagerID        *string `json:"manager_id"`
	TeamType         string  `json:"team_type"`
	DefaultStoryType string  `json:"default_task_type"`
}

// EnsureDefaultTeamRequest is the payload for POST /api/settings/teams/ensure-default.
// Returns the workspace's canonical team of the given type, creating one lazily
// when none exists (see SettingsService.EnsureDefaultTeam).
type EnsureDefaultTeamRequest struct {
	WorkspaceID string `json:"workspace_id"`
	TeamType    string `json:"team_type"`
}

// UpdateTeamRequest is the payload for updating a team.
type UpdateTeamRequest struct {
	Name             *string `json:"name"`
	Handle           *string `json:"handle"`
	Description      *string `json:"description"`
	ManagerID        *string `json:"manager_id"`
	TeamType         *string `json:"team_type"`
	DefaultStoryType *string `json:"default_task_type"`
	SprintsEnabled   *bool   `json:"sprints_enabled"`
}

// UpdateTeamRepoDefaultRequest configures the delivery repository default for a team.
type UpdateTeamRepoDefaultRequest struct {
	RepositoryID   string  `json:"repository_id"`
	BaseBranch     *string `json:"base_branch"`
	BranchTemplate *string `json:"branch_template"`
	AutoSyncStates *bool   `json:"auto_sync_states"`
	ReviewStateID  *string `json:"review_state_id"`
	DoneStateID    *string `json:"done_state_id"`
	ClosedStateID  *string `json:"closed_state_id"`
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
