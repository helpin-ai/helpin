package model

import (
	"encoding/json"
	"time"
)

// GitHubAppCredential stores the instance-wide GitHub App created through the
// manifest flow. There is at most one row (singleton is unique and always
// true). Secrets are AES-256-GCM encrypted with GIT_OAUTH_ENCRYPTION_KEY.
type GitHubAppCredential struct {
	ID                     string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Singleton              bool      `json:"-" gorm:"not null;default:true;uniqueIndex"`
	AppID                  string    `json:"app_id" gorm:"not null"`
	Slug                   string    `json:"slug" gorm:"not null"`
	Name                   string    `json:"name" gorm:"not null;default:''"`
	ClientID               *string   `json:"client_id"`
	ClientSecretEncrypted  *string   `json:"-" gorm:"type:text"`
	PrivateKeyEncrypted    string    `json:"-" gorm:"type:text;not null"`
	WebhookSecretEncrypted *string   `json:"-" gorm:"type:text"`
	HTMLURL                *string   `json:"html_url"`
	OwnerLogin             *string   `json:"owner_login"`
	OwnerType              *string   `json:"owner_type"`
	CreatedBy              *string   `json:"created_by" gorm:"type:uuid"`
	CreatedAt              time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt              time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the GitHub App credential table.
func (GitHubAppCredential) TableName() string { return "github_app_credentials" }

// GitHub App credential sources reported by the status endpoint.
const (
	GitHubAppSourceEnv      = "env"
	GitHubAppSourceDatabase = "database"
	GitHubAppSourceNone     = "none"
)

// GitHubAppStatusResponse describes the instance GitHub App without secrets.
type GitHubAppStatusResponse struct {
	Configured        bool   `json:"configured"`
	Source            string `json:"source"`
	Slug              string `json:"slug"`
	InstallURL        string `json:"install_url"`
	WebhookConfigured bool   `json:"webhook_configured"`
	ManifestAvailable bool   `json:"manifest_available"`
}

// GitHubAppManifestRequest optionally creates the App under a GitHub
// organization instead of the signed-in GitHub user.
type GitHubAppManifestRequest struct {
	Organization string `json:"organization"`
}

// GitHubAppManifestResponse carries the manifest the browser must POST to
// PostURL in a form field named "manifest".
type GitHubAppManifestResponse struct {
	Manifest json.RawMessage `json:"manifest"`
	PostURL  string          `json:"post_url"`
	State    string          `json:"state"`
}
