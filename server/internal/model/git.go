package model

import (
	"time"
)

// GitIntegration represents a workspace-scoped git provider configuration.
type GitIntegration struct {
	ID             string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Provider       string    `json:"provider" gorm:"not null"` // github, gitlab
	DisplayName    string    `json:"display_name" gorm:"not null"`
	BaseURL        *string   `json:"base_url"`                                // for self-hosted instances
	InstallationID *string   `json:"installation_id"`                         // GitHub App installation ID
	AccessToken    string    `json:"-" gorm:"not null"`                       // encrypted or raw token (not exposed via JSON)
	Active         bool      `json:"active" gorm:"not null;default:true"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (GitIntegration) TableName() string { return "git_integrations" }

// StoryGitLink links a story to a repo/branch/PR.
type StoryGitLink struct {
	ID            string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID   string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	StoryID       string    `json:"story_id" gorm:"type:uuid;not null;index"`
	IntegrationID string    `json:"integration_id" gorm:"type:uuid;not null"`
	Provider      string    `json:"provider" gorm:"not null"`
	Repo          string    `json:"repo" gorm:"not null"`
	Branch        *string   `json:"branch"`
	PRNumber      *int      `json:"pr_number"`
	PRURL         *string   `json:"pr_url"`
	PRStatus      *string   `json:"pr_status"` // open, merged, closed
	CommitSHA     *string   `json:"commit_sha"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (StoryGitLink) TableName() string { return "story_git_links" }

// CreateGitIntegrationRequest is the payload for creating a git integration.
type CreateGitIntegrationRequest struct {
	WorkspaceID    string  `json:"workspace_id"`
	Provider       string  `json:"provider"`
	DisplayName    string  `json:"display_name"`
	BaseURL        *string `json:"base_url"`
	InstallationID *string `json:"installation_id"`
	AccessToken    string  `json:"access_token"`
}

// CreateBranchRequest is the payload for creating a branch from a story.
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
