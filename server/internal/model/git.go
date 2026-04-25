package model

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var branchTokenSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

// GitIntegration represents a workspace-scoped git provider configuration.
type GitIntegration struct {
	ID             string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	OrganizationID *string    `json:"organization_id" gorm:"type:uuid;index"`
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
	DeletedAt      *time.Time `json:"deleted_at"`
	LastSyncedAt   *time.Time `json:"last_synced_at"`
	LastSyncError  *string    `json:"last_sync_error"`
	CreatedAt      time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (GitIntegration) TableName() string { return "git_integrations" }

// GitRepository represents a workspace-accessible repository synced from a git provider install.
type GitRepository struct {
	ID            string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID   string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	IntegrationID string          `json:"integration_id" gorm:"type:uuid;not null;index"`
	Provider      string          `json:"provider" gorm:"not null"`
	ExternalID    string          `json:"external_id" gorm:"not null;index"`
	FullName      string          `json:"full_name" gorm:"not null;index"`
	DefaultBranch string          `json:"default_branch" gorm:"not null;default:'main'"`
	Permissions   json.RawMessage `json:"permissions" gorm:"type:jsonb;not null;default:'{}'"`
	Private       bool            `json:"private" gorm:"not null;default:true"`
	Archived      bool            `json:"archived" gorm:"not null;default:false"`
	Selected      bool            `json:"selected" gorm:"not null;default:true"`
	Active        bool            `json:"active" gorm:"not null;default:true"`
	DeletedAt     *time.Time      `json:"deleted_at"`
	CreatedAt     time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (GitRepository) TableName() string { return "git_repositories" }

type GitBranch struct {
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
}

func ResolveGitHubAPIBaseURL(baseURL *string) string {
	raw := strings.TrimSpace(derefStringPtr(baseURL))
	if raw == "" {
		return "https://api.github.com"
	}

	parsed, err := url.Parse(raw)
	if err != nil || strings.TrimSpace(parsed.Host) == "" {
		return strings.TrimRight(raw, "/")
	}

	scheme := strings.TrimSpace(parsed.Scheme)
	if scheme == "" {
		scheme = "https"
	}

	host := strings.ToLower(strings.TrimSpace(parsed.Host))
	switch host {
	case "github.com", "api.github.com":
		return strings.TrimRight(fmt.Sprintf("%s://api.github.com", scheme), "/")
	default:
		path := strings.TrimRight(strings.TrimSpace(parsed.Path), "/")
		if path == "" {
			path = "/api/v3"
		} else if !strings.HasSuffix(path, "/api/v3") {
			path += "/api/v3"
		}
		parsed.Scheme = scheme
		parsed.Path = path
		parsed.RawPath = ""
		parsed.RawQuery = ""
		parsed.Fragment = ""
		return strings.TrimRight(parsed.String(), "/")
	}
}

func ResolveGitHubWebBaseURL(baseURL *string) string {
	raw := strings.TrimSpace(derefStringPtr(baseURL))
	if raw == "" {
		return "https://github.com"
	}

	parsed, err := url.Parse(raw)
	if err != nil || strings.TrimSpace(parsed.Host) == "" {
		return strings.TrimRight(strings.TrimSuffix(raw, "/api/v3"), "/")
	}

	scheme := strings.TrimSpace(parsed.Scheme)
	if scheme == "" {
		scheme = "https"
	}

	host := strings.ToLower(strings.TrimSpace(parsed.Host))
	switch host {
	case "api.github.com":
		return strings.TrimRight(fmt.Sprintf("%s://github.com", scheme), "/")
	default:
		path := strings.TrimRight(strings.TrimSpace(parsed.Path), "/")
		if strings.HasSuffix(path, "/api/v3") {
			path = strings.TrimSuffix(path, "/api/v3")
		}
		parsed.Scheme = scheme
		parsed.Path = path
		parsed.RawPath = ""
		parsed.RawQuery = ""
		parsed.Fragment = ""
		return strings.TrimRight(parsed.String(), "/")
	}
}

func derefStringPtr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func BuildTaskWorkingBranch(task *PMTask, teamDefault *PMTeamRepoDefault, workspaceKey string) string {
	template := "{task_key}-{slug}"
	if teamDefault != nil && strings.TrimSpace(teamDefault.BranchTemplate) != "" {
		template = teamDefault.BranchTemplate
	}

	taskKey := FormatTaskKey(workspaceKey, task.DisplayID)
	replacements := map[string]string{
		"{task_key}":      taskKey,
		"{workspace_key}": workspaceKey,
		"{task_type}":     task.TaskType,
		"{display_id}":    fmt.Sprintf("%d", task.DisplayID),
		"{slug}":          slugifyBranchToken(task.Name),
	}
	for placeholder, value := range replacements {
		template = strings.ReplaceAll(template, placeholder, value)
	}
	template = strings.ToLower(strings.TrimSpace(template))
	template = strings.Trim(template, "/-")
	if template == "" {
		return fmt.Sprintf("%s-%s", strings.ToLower(taskKey), slugifyBranchToken(task.Name))
	}
	return template
}

func slugifyBranchToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = branchTokenSanitizer.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "task"
	}
	return value
}

// PMTeamRepoDefault stores the default delivery repository for a team.
type PMTeamRepoDefault struct {
	ID             string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TeamID         string    `json:"team_id" gorm:"type:uuid;not null;uniqueIndex"`
	RepositoryID   string    `json:"repository_id" gorm:"type:uuid;not null"`
	BaseBranch     string    `json:"base_branch" gorm:"not null;default:'main'"`
	BranchTemplate string    `json:"branch_template" gorm:"not null;default:'{task_key}-{slug}'"`
	AutoSyncStates bool      `json:"auto_sync_states" gorm:"not null;default:true"`
	ReviewStateID  *string   `json:"review_state_id" gorm:"type:uuid"`
	DoneStateID    *string   `json:"done_state_id" gorm:"type:uuid"`
	ClosedStateID  *string   `json:"closed_state_id" gorm:"type:uuid"`
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
	InstallURL    string  `json:"install_url"`
	Action        string  `json:"action"`
	IntegrationID *string `json:"integration_id,omitempty"`
}

type GitAvailableRepoClaim struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	RepoID        string `json:"repo_id"`
}

type GitAvailableRepo struct {
	ExternalID    string                 `json:"external_id"`
	FullName      string                 `json:"full_name"`
	DefaultBranch string                 `json:"default_branch"`
	Private       bool                   `json:"private"`
	Archived      bool                   `json:"archived"`
	Permissions   map[string]bool        `json:"permissions,omitempty"`
	ClaimedBy     *GitAvailableRepoClaim `json:"claimed_by,omitempty"`
}

type WireGitRepositoriesRequest struct {
	WorkspaceID string   `json:"workspace_id"`
	RepoIDs     []string `json:"repo_ids"`
}

type WireGitRepositoriesConflict struct {
	ExternalID            string `json:"external_id"`
	ClaimedByWorkspaceID  string `json:"claimed_by_workspace_id"`
	ClaimedByWorkspaceName string `json:"claimed_by_workspace_name,omitempty"`
}

type WireGitRepositoriesResponse struct {
	Repositories []GitRepository `json:"repositories"`
}

type WireGitRepositoriesConflictResponse struct {
	Conflicts []WireGitRepositoriesConflict `json:"conflicts"`
}

type GitIntegrationWorkspaceUsage struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	RepoCount     int64  `json:"repo_count"`
}

type GitIntegrationDetail struct {
	Integration       GitIntegration                 `json:"integration"`
	AffectedWorkspaces []GitIntegrationWorkspaceUsage `json:"affected_workspaces"`
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
