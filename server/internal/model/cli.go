package model

import (
	"encoding/json"
	"time"
)

// CLIConnection records browser consent for one user, workspace, and CLI resource.
type CLIConnection struct {
	MFASatisfied bool       `json:"-" gorm:"not null;default:false"`
	ID           string     `json:"id" gorm:"primaryKey"`
	UserID       string     `json:"user_id" gorm:"not null;index"`
	WorkspaceID  string     `json:"workspace_id" gorm:"not null;index"`
	ClientID     string     `json:"client_id" gorm:"not null"`
	Resource     string     `json:"resource" gorm:"not null"`
	Scope        string     `json:"scope" gorm:"not null"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// TableName returns the consent table.
func (CLIConnection) TableName() string { return "cli_connections" }

// CLIToken stores only a digest of a code, access token, or refresh token.
type CLIToken struct {
	Hash          string     `json:"-" gorm:"primaryKey"`
	ConnectionID  string     `json:"-" gorm:"not null;index"`
	Kind          string     `json:"-" gorm:"not null"`
	RedirectURI   string     `json:"-"`
	CodeChallenge string     `json:"-"`
	ExpiresAt     time.Time  `json:"-" gorm:"not null;index"`
	UsedAt        *time.Time `json:"-"`
	CreatedAt     time.Time  `json:"-"`
}

// TableName returns the hashed credential table.
func (CLIToken) TableName() string { return "cli_tokens" }

// CLIExecution is a revocable, fenced local execution lease and admission record.
type CLIExecution struct {
	ID             string     `json:"id" gorm:"primaryKey"`
	ConnectionID   string     `json:"connection_id" gorm:"not null;uniqueIndex:idx_cli_admission,priority:1"`
	RequestID      string     `json:"request_id" gorm:"not null;uniqueIndex:idx_cli_admission,priority:2"`
	RequestHash    string     `json:"-" gorm:"not null"`
	WorkspaceID    string     `json:"workspace_id" gorm:"not null;index"`
	UserID         string     `json:"user_id" gorm:"not null"`
	RunID          string     `json:"run_id" gorm:"not null;uniqueIndex"`
	Epoch          int64      `json:"epoch" gorm:"not null"`
	LocalRunID     string     `json:"local_run_id,omitempty"`
	PolicyHash     string     `json:"policy_hash"`
	Snapshot       JSONBlob   `json:"-" gorm:"type:jsonb"`
	LeaseExpiresAt time.Time  `json:"lease_expires_at"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// TableName returns the local execution table.
func (CLIExecution) TableName() string { return "cli_executions" }

// CLIAuthorizationQuery is the public client's PKCE authorization request.
type CLIAuthorizationQuery struct {
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	ResponseType        string `json:"response_type"`
	Scope               string `json:"scope"`
	State               string `json:"state"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
	Resource            string `json:"resource"`
}

// CLIConsentDecision is an authenticated browser user's workspace choice.
type CLIConsentDecision struct {
	Query       CLIAuthorizationQuery `json:"query"`
	WorkspaceID string                `json:"workspace_id"`
}

// CLIAdmissionRequest contains no trusted identity or credential fields.
type CLIAdmissionRequest struct {
	RequestID         string `json:"request_id"`
	AgentID           string `json:"agent_id"`
	Target            string `json:"target"`
	Instructions      string `json:"instructions"`
	ExecutionLocation string `json:"execution_location"`
	Review            bool   `json:"review"`
}

// CLIAgentSnapshot is the immutable executable subset admitted by Helpin.
type CLIAgentSnapshot struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	RuntimeKind     string          `json:"runtime_kind"`
	SystemPrompt    string          `json:"system_prompt"`
	ApprovalMode    string          `json:"approval_mode"`
	ExecutionConfig json.RawMessage `json:"execution_config"`
}

// CLIAdmission exposes policy and lease identity, never provider credentials.
type CLIAdmission struct {
	RunID        string           `json:"run_id"`
	Agent        CLIAgentSnapshot `json:"agent"`
	Context      string           `json:"context"`
	AllowedTools []string         `json:"allowed_tools"`
	Execution    *CLIExecution    `json:"execution,omitempty"`
}

// CLILocalRun marks trusted admission provenance in the normal run input.
type CLILocalRun struct {
	ConnectionID string       `json:"connection_id"`
	ExecutionID  string       `json:"execution_id"`
	Admission    CLIAdmission `json:"admission"`
}

// IsLocalAgentRun recognizes server-owned local execution provenance.
func IsLocalAgentRun(run *AgentRun) bool {
	if run == nil {
		return false
	}
	var input struct {
		ExecutionLocation string       `json:"execution_location"`
		Local             *CLILocalRun `json:"local_execution"`
	}
	return json.Unmarshal(run.Input, &input) == nil && input.ExecutionLocation == "local" && input.Local != nil && input.Local.ExecutionID != ""
}
