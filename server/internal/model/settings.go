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
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Name        string    `json:"name" gorm:"not null"`
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

// FullWorkspaceConfig aggregates all settings for a workspace.
type FullWorkspaceConfig struct {
	Settings    *WorkspaceSettings `json:"settings"`
	Teams       []WorkspaceTeam    `json:"teams"`
	People      []WorkspacePerson  `json:"people"`
	Memberships []TeamMembership   `json:"memberships"`
	Managers    []WorkspaceManager `json:"managers"`
	JobRoles    []JobRoleCriteria  `json:"job_roles"`
	BonusTiers  []BonusTier        `json:"bonus_tiers"`
}

// CreateTeamRequest is the payload for creating a team.
type CreateTeamRequest struct {
	WorkspaceID string  `json:"workspace_id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	ManagerID   *string `json:"manager_id"`
}

// UpdateTeamRequest is the payload for updating a team.
type UpdateTeamRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	ManagerID   *string `json:"manager_id"`
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
