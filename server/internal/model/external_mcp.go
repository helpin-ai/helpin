package model

import (
	"encoding/json"
	"time"
)

const (
	ExternalMCPProviderCustomerIO = "customer_io"
	ExternalMCPProviderCustom     = "custom"

	ExternalMCPAuthOAuth       = "oauth"
	ExternalMCPAuthBearerToken = "bearer_token"
	ExternalMCPAuthHeaders     = "headers"
	ExternalMCPAuthNone        = "none"

	ExternalMCPStatusPendingOAuth          = "pending_oauth"
	ExternalMCPStatusConnected             = "connected"
	ExternalMCPStatusReauthorizationNeeded = "reauthorization_required"
	ExternalMCPStatusInsufficientScope     = "insufficient_scope"
	ExternalMCPStatusRemoteDisabled        = "remote_disabled"
	ExternalMCPStatusError                 = "error"
	ExternalMCPStatusDisconnected          = "disconnected"

	ExternalMCPToolAccessRead  = "read"
	ExternalMCPToolAccessWrite = "write"
)

// ExternalMCPServer is a workspace-owned outbound MCP installation. It never
// contains credentials; those are kept in ExternalMCPCredential.
type ExternalMCPServer struct {
	ID                     string            `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID            string            `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Name                   string            `json:"name" gorm:"not null"`
	ServerName             string            `json:"server_name" gorm:"not null"`
	Provider               string            `json:"provider" gorm:"not null"`
	EndpointURL            string            `json:"endpoint_url" gorm:"not null"`
	Transport              string            `json:"transport" gorm:"not null;default:'streamable_http'"`
	AuthType               string            `json:"auth_type" gorm:"not null"`
	Status                 string            `json:"status" gorm:"not null;index"`
	Enabled                bool              `json:"enabled" gorm:"not null;default:true"`
	OAuthScopes            json.RawMessage   `json:"oauth_scopes" gorm:"type:jsonb;not null;default:'[]'"`
	RemoteIdentity         json.RawMessage   `json:"remote_identity,omitempty" gorm:"type:jsonb;not null;default:'{}'"`
	AuthorizedByUserID     *string           `json:"authorized_by_user_id,omitempty" gorm:"type:uuid"`
	AccessTokenExpiresAt   *time.Time        `json:"access_token_expires_at,omitempty"`
	LastHealthCheckedAt    *time.Time        `json:"last_health_checked_at,omitempty"`
	LastToolSyncAt         *time.Time        `json:"last_tool_sync_at,omitempty"`
	LastErrorCode          *string           `json:"last_error_code,omitempty"`
	LastErrorMessage       *string           `json:"last_error_message,omitempty"`
	AuthIncidentKey        *string           `json:"-"`
	AuthIncidentNotifiedAt *time.Time        `json:"-"`
	CreatedBy              string            `json:"created_by" gorm:"type:uuid;not null"`
	CreatedAt              time.Time         `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt              time.Time         `json:"updated_at" gorm:"autoUpdateTime"`
	Tools                  []ExternalMCPTool `json:"tools,omitempty" gorm:"-"`
}

func (ExternalMCPServer) TableName() string { return "external_mcp_servers" }

// ExternalMCPCredential stores only encrypted OAuth or static credentials.
// All encrypted fields are deliberately excluded from JSON serialization.
type ExternalMCPCredential struct {
	ServerID                string     `json:"-" gorm:"type:uuid;primaryKey"`
	WorkspaceID             string     `json:"-" gorm:"type:uuid;not null;index"`
	EncryptedAccessToken    string     `json:"-" gorm:"type:text"`
	EncryptedRefreshToken   string     `json:"-" gorm:"type:text"`
	EncryptedHeaders        string     `json:"-" gorm:"type:text"`
	EncryptedClientSecret   string     `json:"-" gorm:"type:text"`
	ClientID                string     `json:"-"`
	TokenType               string     `json:"-"`
	AuthorizationEndpoint   string     `json:"-" gorm:"type:text"`
	TokenEndpoint           string     `json:"-" gorm:"type:text"`
	RegistrationEndpoint    string     `json:"-" gorm:"type:text"`
	ResourceURL             string     `json:"-" gorm:"type:text"`
	TokenEndpointAuthMethod string     `json:"-"`
	AccessTokenExpiresAt    *time.Time `json:"-"`
	CreatedAt               time.Time  `json:"-" gorm:"autoCreateTime"`
	UpdatedAt               time.Time  `json:"-" gorm:"autoUpdateTime"`
}

func (ExternalMCPCredential) TableName() string { return "external_mcp_credentials" }

// ExternalMCPTool is a discovered remote tool and its workspace policy.
type ExternalMCPTool struct {
	ID           string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID  string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ServerID     string          `json:"server_id" gorm:"type:uuid;not null;index"`
	RemoteName   string          `json:"remote_name" gorm:"not null"`
	RuntimeAlias string          `json:"runtime_alias" gorm:"not null"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"input_schema" gorm:"type:jsonb;not null;default:'{}'"`
	Access       string          `json:"access" gorm:"not null;default:'write'"`
	Enabled      bool            `json:"enabled" gorm:"not null;default:true"`
	SchemaHash   string          `json:"schema_hash" gorm:"not null"`
	LastSeenAt   time.Time       `json:"last_seen_at" gorm:"not null"`
	CreatedAt    time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (ExternalMCPTool) TableName() string { return "external_mcp_tools" }

// ExternalMCPOAuthState binds a browser authorization response to the
// initiating workspace, user, server and PKCE verifier.
type ExternalMCPOAuthState struct {
	ID                    string          `json:"-" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	StateHash             string          `json:"-" gorm:"uniqueIndex;not null"`
	WorkspaceID           string          `json:"-" gorm:"type:uuid;not null;index"`
	ServerID              string          `json:"-" gorm:"type:uuid;not null;index"`
	UserID                string          `json:"-" gorm:"type:uuid;not null;index"`
	EncryptedPKCEVerifier string          `json:"-" gorm:"type:text;not null"`
	RequestedScopes       json.RawMessage `json:"-" gorm:"type:jsonb;not null;default:'[]'"`
	ReturnPath            string          `json:"-"`
	ExpiresAt             time.Time       `json:"-" gorm:"not null;index"`
	ConsumedAt            *time.Time      `json:"-"`
	CreatedAt             time.Time       `json:"-" gorm:"autoCreateTime"`
}

func (ExternalMCPOAuthState) TableName() string { return "external_mcp_oauth_states" }

// AgentRunExternalMCPBinding records the exact external MCP policy attached to
// a Helpin run, without retaining the runtime credential.
type AgentRunExternalMCPBinding struct {
	ID               string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	AgentRunID       string          `json:"agent_run_id" gorm:"type:uuid;not null;index"`
	RuntimeRunID     string          `json:"runtime_run_id" gorm:"not null;index"`
	ServerID         string          `json:"server_id" gorm:"type:uuid;not null;index"`
	ServerName       string          `json:"server_name" gorm:"not null"`
	ToolAliases      json.RawMessage `json:"tool_aliases" gorm:"type:jsonb;not null;default:'[]'"`
	CredentialExpiry *time.Time      `json:"credential_expiry,omitempty"`
	CreatedAt        time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

func (AgentRunExternalMCPBinding) TableName() string {
	return "agent_run_external_mcp_bindings"
}
