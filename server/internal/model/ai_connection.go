package model

import (
	"time"

	sdk "github.com/helpin-ai/agent-runtime-go"
)

// AIConnection belongs to a workspace or one of its members. Secret material is never
// serialized; device sessions and OAuth refresh tokens remain app-owned.
type AIConnection struct {
	DiscoveredModels []DiscoveredAIModel `json:"-" gorm:"serializer:json;type:jsonb"`
	ModelsFetchedAt  *time.Time          `json:"-"`
	// Superseded generated defaults remain readable for frozen run selections,
	// but are no longer offered when configuring new profiles.
	SupersededBy    *string                 `json:"-" gorm:"type:uuid"`
	Endpoint        *sdk.ModelEndpoint      `json:"endpoint,omitempty" gorm:"serializer:json;type:jsonb"`
	Policy          *AIConnectionPolicyView `json:"policy,omitempty" gorm:"-"`
	ID              string                  `json:"id" gorm:"type:uuid;primaryKey"`
	WorkspaceID     string                  `json:"workspace_id" gorm:"type:uuid;not null;index"`
	UserID          *string                 `json:"user_id" gorm:"type:uuid;index"`
	Scope           string                  `json:"scope" gorm:"not null;default:personal"`
	Funding         string                  `json:"funding" gorm:"not null;default:customer"`
	Name            string                  `json:"name" gorm:"not null"`
	Provider        string                  `json:"provider" gorm:"not null"`
	Status          string                  `json:"status" gorm:"not null"`
	AccountID       string                  `json:"account_id,omitempty"`
	ExpiresAt       *time.Time              `json:"expires_at,omitempty"`
	EncryptedSecret []byte                  `json:"-"`
	// CredentialSource is "environment" when the deployment's provider key
	// configured this connection; startup may then rotate it from the environment.
	// Any other value, including nil, marks a key that only users may change.
	CredentialSource *string `json:"credential_source,omitempty"`
	// LastVerifiedAt records the last explicit connection test. A nil
	// LastVerificationError alongside it means that test succeeded.
	LastVerifiedAt        *time.Time `json:"last_verified_at,omitempty"`
	LastVerificationError *string    `json:"last_verification_error,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// AIConnectionCredentialSourceEnvironment marks a key sealed from deployment configuration.
const AIConnectionCredentialSourceEnvironment = "environment"

// TestAIConnectionRequest optionally selects the model used for a connection test.
type TestAIConnectionRequest struct {
	Model string `json:"model,omitempty"`
}

// AIConnectionTestResult reports a live provider check. Error is sanitized and
// never contains credential material or raw provider responses.
type AIConnectionTestResult struct {
	OK        bool   `json:"ok"`
	Model     string `json:"model"`
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

type CreateAIConnectionRequest struct {
	EndpointID string `json:"endpoint_id,omitempty"`
	Scope      string `json:"scope,omitempty"`
	Name       string `json:"name"`
	Provider   string `json:"provider"`
	APIKey     string `json:"api_key,omitempty"`
}

type AIConnectionLogin struct {
	Connection      AIConnection `json:"connection"`
	VerificationURL string       `json:"verification_url,omitempty"`
	UserCode        string       `json:"user_code,omitempty"`
	ExpiresAt       *time.Time   `json:"expires_at,omitempty"`
	IntervalSeconds int          `json:"interval_seconds,omitempty"`
}

func (AIConnection) TableName() string { return "ai_connections" }

// AIConnectionPolicyView describes whether edition policy permits a new
// selection. It does not probe credentials or claim that the provider is online.
type AIConnectionPolicyView struct {
	Allowed bool                       `json:"allowed"`
	Message string                     `json:"message,omitempty"`
	Pricing *AIExecutionPolicySnapshot `json:"pricing,omitempty"`
}

// DiscoveredAIModel contains public provider metadata, never credentials.
type DiscoveredAIModel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// DiscoveredAt is absent for the initial catalog baseline.
	DiscoveredAt *time.Time `json:"discovered_at,omitempty"`
}
type AIConnectionModels struct {
	Models    []DiscoveredAIModel `json:"models"`
	Source    string              `json:"source"`
	FetchedAt *time.Time          `json:"fetched_at,omitempty"`
	Stale     bool                `json:"stale"`
	Warning   string              `json:"warning,omitempty"`
}
