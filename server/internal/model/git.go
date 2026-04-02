package model

import (
	"encoding/json"
	"time"
)

// GitIntegration represents a workspace-scoped git provider configuration.
type GitIntegration struct {
	ID             string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Provider       string     `json:"provider" gorm:"not null"` // github, gitlab
	DisplayName    string     `json:"display_name" gorm:"not null"`
	CredentialMode string     `json:"credential_mode" gorm:"not null;default:'github_app'"`
	AccountLogin   *string    `json:"account_login"`
	BaseURL        *string    `json:"base_url"`        // for self-hosted instances
	InstallationID *string    `json:"installation_id"` // GitHub App installation ID
	AppID          *string    `json:"app_id"`
	WebhookSecret  *string    `json:"-" gorm:"column:webhook_secret"`
	AccessToken    string     `json:"-" gorm:"not null"` // deprecated PAT field retained for migration compatibility
	Active         bool       `json:"active" gorm:"not null;default:true"`
	LastSyncedAt   *time.Time `json:"last_synced_at"`
	LastSyncError  *string    `json:"last_sync_error"`
	CreatedAt      time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (GitIntegration) TableName() string { return "git_integrations" }

// GitRepository represents a workspace-accessible repository synced from a git provider install.
type GitRepository struct {
	ID            string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID   string          `json:"workspace_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_git_repo_external,priority:1"`
	IntegrationID string          `json:"integration_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_git_repo_external,priority:2"`
	Provider      string          `json:"provider" gorm:"not null"`
	ExternalID    string          `json:"external_id" gorm:"not null;uniqueIndex:idx_git_repo_external,priority:3"`
	FullName      string          `json:"full_name" gorm:"not null;index"`
	DefaultBranch string          `json:"default_branch" gorm:"not null;default:'main'"`
	Permissions   json.RawMessage `json:"permissions" gorm:"type:jsonb;not null;default:'{}'"`
	Private       bool            `json:"private" gorm:"not null;default:true"`
	Archived      bool            `json:"archived" gorm:"not null;default:false"`
	Selected      bool            `json:"selected" gorm:"not null;default:true"`
	CreatedAt     time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (GitRepository) TableName() string { return "git_repositories" }

// PMTeamRepoDefault stores the default delivery repository for a team.
type PMTeamRepoDefault struct {
	ID             string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TeamID         string    `json:"team_id" gorm:"type:uuid;not null;uniqueIndex"`
	RepositoryID   string    `json:"repository_id" gorm:"type:uuid;not null"`
	BaseBranch     string    `json:"base_branch" gorm:"not null;default:'main'"`
	BranchTemplate string    `json:"branch_template" gorm:"not null;default:'tp-{display_id}-{slug}'"`
	AutoSyncStates bool      `json:"auto_sync_states" gorm:"not null;default:true"`
	ReviewStateID  *string   `json:"review_state_id" gorm:"type:uuid"`
	DoneStateID    *string   `json:"done_state_id" gorm:"type:uuid"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMTeamRepoDefault) TableName() string { return "pm_team_repo_defaults" }

// TaskDeliveryTarget stores the current delivery lane for a task.
type TaskDeliveryTarget struct {
	ID             string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	TaskID         string     `json:"task_id" gorm:"column:task_id;type:uuid;not null;uniqueIndex"`
	RepositoryID   *string    `json:"repository_id" gorm:"type:uuid;index"`
	RepoFullName   *string    `json:"repo_full_name"`
	IntegrationID  *string    `json:"integration_id" gorm:"type:uuid;index"`
	BaseBranch     *string    `json:"base_branch"`
	WorkingBranch  *string    `json:"working_branch"`
	DeliveryState  string     `json:"delivery_state" gorm:"not null;default:'unconfigured'"`
	ActivePRNumber *int       `json:"active_pr_number"`
	ActivePRTitle  *string    `json:"active_pr_title"`
	ActivePRURL    *string    `json:"active_pr_url"`
	ActivePRStatus *string    `json:"active_pr_status"`
	LastCommitSHA  *string    `json:"last_commit_sha"`
	LastRunID      *string    `json:"last_run_id" gorm:"type:uuid"`
	LastSyncedAt   *time.Time `json:"last_synced_at"`
	CreatedAt      time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (TaskDeliveryTarget) TableName() string { return "task_delivery_targets" }

// TaskGitLink links a task to a repo/branch/PR.
type TaskGitLink struct {
	ID            string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID   string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	TaskID        string    `json:"task_id" gorm:"column:task_id;type:uuid;not null;index"`
	IntegrationID string    `json:"integration_id" gorm:"type:uuid;not null"`
	RepositoryID  *string   `json:"repository_id" gorm:"type:uuid;index"`
	RunID         *string   `json:"run_id" gorm:"type:uuid;index"`
	Provider      string    `json:"provider" gorm:"not null"`
	Repo          string    `json:"repo" gorm:"not null"`
	Branch        *string   `json:"branch"`
	PRNumber      *int      `json:"pr_number"`
	PRTitle       *string   `json:"pr_title"`
	PRURL         *string   `json:"pr_url"`
	PRStatus      *string   `json:"pr_status"` // open, merged, closed
	CommitSHA     *string   `json:"commit_sha"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (TaskGitLink) TableName() string { return "task_git_links" }

// CreateGitIntegrationRequest is the payload for creating a git integration.
type CreateGitIntegrationRequest struct {
	WorkspaceID    string  `json:"workspace_id"`
	Provider       string  `json:"provider"`
	DisplayName    string  `json:"display_name"`
	CredentialMode *string `json:"credential_mode"`
	AccountLogin   *string `json:"account_login"`
	BaseURL        *string `json:"base_url"`
	InstallationID *string `json:"installation_id"`
	AppID          *string `json:"app_id"`
	WebhookSecret  *string `json:"webhook_secret"`
	AccessToken    string  `json:"access_token"`
}

// UpdateTaskDeliveryTargetRequest updates the selected delivery target for a task.
type UpdateTaskDeliveryTargetRequest struct {
	RepositoryID  *string `json:"repository_id"`
	BaseBranch    *string `json:"base_branch"`
	WorkingBranch *string `json:"working_branch"`
}

// SyncGitRepositoriesRequest controls manual repository synchronization.
type SyncGitRepositoriesRequest struct {
	SelectedRepositoryIDs []string `json:"selected_repository_ids"`
}

// UpdateGitRepositoryRequest updates repository catalog flags.
type UpdateGitRepositoryRequest struct {
	Selected *bool `json:"selected"`
}

// GitHubInstallURLResponse returns the install URL for the configured GitHub App.
type GitHubInstallURLResponse struct {
	InstallURL string `json:"install_url"`
	Action     string `json:"action"`
}

// CreateBranchRequest is the payload for creating a branch from a task.
type CreateBranchRequest struct {
	IntegrationID string `json:"integration_id"`
	Repo          string `json:"repo"`
	BranchName    string `json:"branch_name"`
}

// WebhookPayload is a generic wrapper for incoming git webhooks.
type WebhookPayload struct {
	Provider string `json:"provider"`
	Event    string `json:"event"`
	// Raw body is parsed in the handler.
}
