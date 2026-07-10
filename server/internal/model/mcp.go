package model

import (
	"encoding/json"
	"time"
)

const (
	MCPConnectionStatusActive  = "active"
	MCPConnectionStatusRevoked = "revoked"

	MCPServicePrincipalStatusActive  = "active"
	MCPServicePrincipalStatusRevoked = "revoked"

	MCPPrincipalKindUser    = "user"
	MCPPrincipalKindService = "service"
)

// MCPWorkspacePolicy controls public MCP access for one workspace.
type MCPWorkspacePolicy struct {
	WorkspaceID            string          `json:"workspace_id" gorm:"type:uuid;primaryKey"`
	Enabled                bool            `json:"enabled" gorm:"not null;default:false"`
	EnforceReadOnly        bool            `json:"enforce_read_only" gorm:"not null;default:true"`
	ServiceAccountsEnabled bool            `json:"service_accounts_enabled" gorm:"not null;default:false"`
	AllowedToolsets        json.RawMessage `json:"allowed_toolsets" gorm:"type:jsonb;not null;default:'[\"context\",\"pm\",\"docs\",\"agents\"]'"`
	AllowedScopes          json.RawMessage `json:"allowed_scopes" gorm:"type:jsonb;not null;default:'[\"helpin.context.read\",\"helpin.pm.read\",\"helpin.docs.read\",\"helpin.agents.read\"]'"`
	UpdatedBy              *string         `json:"updated_by,omitempty" gorm:"type:uuid"`
	CreatedAt              time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt              time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the MCP workspace-policy table name.
func (MCPWorkspacePolicy) TableName() string { return "mcp_workspace_policies" }

// MCPClientRegistration is an OAuth client allowed to request an MCP connection.
type MCPClientRegistration struct {
	ID           string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ClientID     string          `json:"client_id" gorm:"uniqueIndex;not null"`
	ClientName   string          `json:"client_name" gorm:"not null"`
	ClientURI    *string         `json:"client_uri,omitempty"`
	RedirectURIs json.RawMessage `json:"redirect_uris" gorm:"type:jsonb;not null;default:'[]'"`
	CreatedAt    time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the MCP OAuth-client table name.
func (MCPClientRegistration) TableName() string { return "mcp_client_registrations" }

// MCPConnection records a user-authorized, workspace-bound MCP client connection.
type MCPConnection struct {
	ID           string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID  string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	UserID       string          `json:"user_id" gorm:"type:uuid;not null;index"`
	ClientID     string          `json:"client_id" gorm:"not null;index"`
	ClientName   string          `json:"client_name" gorm:"not null"`
	Scopes       json.RawMessage `json:"scopes" gorm:"type:jsonb;not null;default:'[]'"`
	Toolsets     json.RawMessage `json:"toolsets" gorm:"type:jsonb;not null;default:'[]'"`
	ReadOnly     bool            `json:"read_only" gorm:"not null;default:true"`
	Status       string          `json:"status" gorm:"not null;default:'active';index"`
	TokenVersion int             `json:"-" gorm:"not null;default:1"`
	LastUsedAt   *time.Time      `json:"last_used_at,omitempty"`
	RevokedAt    *time.Time      `json:"revoked_at,omitempty"`
	RevokedBy    *string         `json:"revoked_by,omitempty" gorm:"type:uuid"`
	CreatedAt    time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the MCP connection table name.
func (MCPConnection) TableName() string { return "mcp_connections" }

// MCPOAuthAuthorizationCode is a single-use, PKCE-bound OAuth authorization code.
type MCPOAuthAuthorizationCode struct {
	ID                  string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CodeHash            string     `json:"-" gorm:"uniqueIndex;not null"`
	ConnectionID        string     `json:"connection_id" gorm:"type:uuid;not null;index"`
	ClientID            string     `json:"client_id" gorm:"not null;index"`
	RedirectURI         string     `json:"redirect_uri" gorm:"not null"`
	CodeChallenge       string     `json:"-" gorm:"not null"`
	CodeChallengeMethod string     `json:"-" gorm:"not null;default:'S256'"`
	ExpiresAt           time.Time  `json:"expires_at" gorm:"not null;index"`
	ConsumedAt          *time.Time `json:"consumed_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at" gorm:"autoCreateTime"`
}

// TableName returns the MCP OAuth authorization-code table name.
func (MCPOAuthAuthorizationCode) TableName() string { return "mcp_oauth_authorization_codes" }

// MCPRefreshToken stores a hashed, rotating OAuth refresh token.
type MCPRefreshToken struct {
	ID           string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TokenHash    string     `json:"-" gorm:"uniqueIndex;not null"`
	FamilyID     string     `json:"family_id" gorm:"type:uuid;not null;index"`
	ConnectionID string     `json:"connection_id" gorm:"type:uuid;not null;index"`
	ExpiresAt    time.Time  `json:"expires_at" gorm:"not null;index"`
	ConsumedAt   *time.Time `json:"consumed_at,omitempty"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
	ReplacedByID *string    `json:"replaced_by_id,omitempty" gorm:"type:uuid"`
	CreatedAt    time.Time  `json:"created_at" gorm:"autoCreateTime"`
}

// TableName returns the MCP refresh-token table name.
func (MCPRefreshToken) TableName() string { return "mcp_refresh_tokens" }

// MCPServicePrincipal is a named headless identity restricted to one workspace.
type MCPServicePrincipal struct {
	ID          string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Name        string          `json:"name" gorm:"not null"`
	Description *string         `json:"description,omitempty"`
	ActorUserID string          `json:"actor_user_id" gorm:"type:uuid;not null;index"`
	Scopes      json.RawMessage `json:"scopes" gorm:"type:jsonb;not null;default:'[]'"`
	Toolsets    json.RawMessage `json:"toolsets" gorm:"type:jsonb;not null;default:'[]'"`
	ReadOnly    bool            `json:"read_only" gorm:"not null;default:true"`
	Status      string          `json:"status" gorm:"not null;default:'active';index"`
	ExpiresAt   *time.Time      `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time      `json:"last_used_at,omitempty"`
	RevokedAt   *time.Time      `json:"revoked_at,omitempty"`
	CreatedBy   string          `json:"created_by" gorm:"type:uuid;not null"`
	CreatedAt   time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the MCP service-principal table name.
func (MCPServicePrincipal) TableName() string { return "mcp_service_principals" }

// MCPServiceToken stores one hashed secret for a service principal.
type MCPServiceToken struct {
	ID                 string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ServicePrincipalID string     `json:"service_principal_id" gorm:"type:uuid;not null;index"`
	TokenHash          string     `json:"-" gorm:"uniqueIndex;not null"`
	TokenPrefix        string     `json:"token_prefix" gorm:"not null"`
	ExpiresAt          *time.Time `json:"expires_at,omitempty"`
	LastUsedAt         *time.Time `json:"last_used_at,omitempty"`
	RevokedAt          *time.Time `json:"revoked_at,omitempty"`
	CreatedBy          string     `json:"created_by" gorm:"type:uuid;not null"`
	CreatedAt          time.Time  `json:"created_at" gorm:"autoCreateTime"`
}

// TableName returns the MCP service-token table name.
func (MCPServiceToken) TableName() string { return "mcp_service_tokens" }

// MCPAuditEvent is a sanitized record of an MCP security or tool event.
type MCPAuditEvent struct {
	ID                 string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID        string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ConnectionID       *string   `json:"connection_id,omitempty" gorm:"type:uuid;index"`
	ServicePrincipalID *string   `json:"service_principal_id,omitempty" gorm:"type:uuid;index"`
	UserID             *string   `json:"user_id,omitempty" gorm:"type:uuid;index"`
	ClientName         string    `json:"client_name"`
	EventType          string    `json:"event_type" gorm:"not null;index"`
	ToolName           *string   `json:"tool_name,omitempty"`
	Outcome            string    `json:"outcome" gorm:"not null;index"`
	ReasonCode         *string   `json:"reason_code,omitempty"`
	RequestHash        *string   `json:"request_hash,omitempty"`
	ResultHash         *string   `json:"result_hash,omitempty"`
	DurationMS         *int64    `json:"duration_ms,omitempty"`
	CreatedAt          time.Time `json:"created_at" gorm:"autoCreateTime;index"`
}

// TableName returns the MCP audit-event table name.
func (MCPAuditEvent) TableName() string { return "mcp_audit_events" }

// MCPIdempotencyRecord stores the bounded result of a mutation replay key.
type MCPIdempotencyRecord struct {
	ID           string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	PrincipalKey string          `json:"principal_key" gorm:"not null;uniqueIndex:idx_mcp_idempotency_principal_key,priority:1"`
	Key          string          `json:"key" gorm:"not null;uniqueIndex:idx_mcp_idempotency_principal_key,priority:2"`
	ToolName     string          `json:"tool_name" gorm:"not null"`
	RequestHash  string          `json:"request_hash" gorm:"not null"`
	Result       json.RawMessage `json:"result" gorm:"type:jsonb;not null;default:'{}'"`
	IsError      bool            `json:"is_error" gorm:"not null;default:false"`
	ExpiresAt    time.Time       `json:"expires_at" gorm:"not null;index"`
	CreatedAt    time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

// TableName returns the MCP idempotency table name.
func (MCPIdempotencyRecord) TableName() string { return "mcp_idempotency_records" }

// MCPAgentRunAttribution links an MCP invocation to a normal Helpin agent run.
type MCPAgentRunAttribution struct {
	RunID              string    `json:"run_id" gorm:"type:uuid;primaryKey"`
	WorkspaceID        string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ConnectionID       *string   `json:"connection_id,omitempty" gorm:"type:uuid;index"`
	ServicePrincipalID *string   `json:"service_principal_id,omitempty" gorm:"type:uuid;index"`
	ClientName         string    `json:"client_name"`
	CreatedAt          time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName returns the MCP run-attribution table name.
func (MCPAgentRunAttribution) TableName() string { return "mcp_agent_run_attributions" }

// MCPPrincipal is the resolved external execution identity used by policy and tools.
type MCPPrincipal struct {
	Kind               string   `json:"kind"`
	WorkspaceID        string   `json:"workspace_id"`
	UserID             string   `json:"user_id,omitempty"`
	ServicePrincipalID string   `json:"service_principal_id,omitempty"`
	ConnectionID       string   `json:"connection_id,omitempty"`
	ClientID           string   `json:"client_id,omitempty"`
	ClientName         string   `json:"client_name"`
	Scopes             []string `json:"scopes"`
	Toolsets           []string `json:"toolsets"`
	ReadOnly           bool     `json:"read_only"`
	TokenVersion       int      `json:"-"`
}

// UpdateMCPWorkspacePolicyRequest changes workspace-level MCP exposure.
type UpdateMCPWorkspacePolicyRequest struct {
	Enabled                bool     `json:"enabled"`
	EnforceReadOnly        bool     `json:"enforce_read_only"`
	ServiceAccountsEnabled bool     `json:"service_accounts_enabled"`
	AllowedToolsets        []string `json:"allowed_toolsets"`
	AllowedScopes          []string `json:"allowed_scopes"`
}

// CreateMCPServicePrincipalRequest creates a restricted headless identity.
type CreateMCPServicePrincipalRequest struct {
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	Scopes      []string   `json:"scopes"`
	Toolsets    []string   `json:"toolsets"`
	ReadOnly    bool       `json:"read_only"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// MCPServiceTokenSecret is returned once when a service token is created or rotated.
type MCPServiceTokenSecret struct {
	Token  string          `json:"token"`
	Record MCPServiceToken `json:"record"`
}
