package model

import "time"

const (
	// AgentRuntimeKindA2A marks a Helpin agent whose runs are delegated to a
	// remote Agent2Agent (A2A) agent instead of a model.
	AgentRuntimeKindA2A = "a2a"

	ExternalA2AStatusActive   = "active"
	ExternalA2AStatusDisabled = "disabled"
	ExternalA2AStatusError    = "error"
)

// ExternalA2AAgent is a workspace-owned connection to a remote A2A agent. The
// linked Helpin agent (AgentID) is what users assign tasks to. The bearer token
// is stored encrypted and never serialized.
type ExternalA2AAgent struct {
	ID              string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	AgentID         string     `json:"agent_id" gorm:"type:uuid;not null;uniqueIndex"`
	Name            string     `json:"name" gorm:"not null"`
	Description     string     `json:"description" gorm:"not null;default:''"`
	CardURL         string     `json:"card_url" gorm:"not null"`
	InterfaceURL    string     `json:"interface_url" gorm:"not null"`
	ProtocolBinding string     `json:"protocol_binding" gorm:"not null;default:'JSONRPC'"`
	ProtocolVersion string     `json:"protocol_version" gorm:"not null;default:''"`
	ProviderName    string     `json:"provider_name" gorm:"not null;default:''"`
	Version         string     `json:"version" gorm:"not null;default:''"`
	Skills          JSONBlob   `json:"skills" gorm:"type:jsonb;not null;default:'[]'"`
	Capabilities    JSONBlob   `json:"capabilities" gorm:"type:jsonb;not null;default:'{}'"`
	AgentCard       JSONBlob   `json:"-" gorm:"type:jsonb;not null;default:'{}'"`
	EncryptedToken  string     `json:"-" gorm:"type:text;not null;default:''"`
	TokenHint       string     `json:"token_hint" gorm:"not null;default:''"`
	Status          string     `json:"status" gorm:"not null;default:'active'"`
	LastCheckedAt   *time.Time `json:"last_checked_at"`
	LastError       string     `json:"last_error" gorm:"not null;default:''"`
	AllowedTeamIDs  JSONBlob   `json:"allowed_team_ids" gorm:"type:jsonb;not null;default:'[]'"`
	CreatedBy       string     `json:"created_by" gorm:"type:uuid;not null"`
	CreatedAt       time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (ExternalA2AAgent) TableName() string { return "external_a2a_agents" }

// A2ATaskContext remembers the remote A2A contextId used for a Helpin task so
// later runs continue the same remote conversation.
type A2ATaskContext struct {
	ID                 string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID        string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	TaskID             string    `json:"task_id" gorm:"type:uuid;not null"`
	ExternalA2AAgentID string    `json:"external_a2a_agent_id" gorm:"column:external_a2a_agent_id;type:uuid;not null"`
	ContextID          string    `json:"context_id" gorm:"not null"`
	LastRemoteTaskID   string    `json:"last_remote_task_id" gorm:"not null;default:''"`
	CreatedAt          time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (A2ATaskContext) TableName() string { return "a2a_task_contexts" }

// A2ARunUploadToken is a per-run upload credential handed to the remote agent.
// Only the SHA-256 of the bearer token is stored.
type A2ARunUploadToken struct {
	ID                 string     `json:"-" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID        string     `json:"-" gorm:"type:uuid;not null"`
	AgentRunID         string     `json:"-" gorm:"type:uuid;not null;index"`
	PMTaskID           string     `json:"-" gorm:"column:pm_task_id;type:uuid;not null"`
	ExternalA2AAgentID string     `json:"-" gorm:"column:external_a2a_agent_id;type:uuid;not null"`
	TokenSHA256        string     `json:"-" gorm:"column:token_sha256;not null;uniqueIndex"`
	ExpiresAt          time.Time  `json:"-" gorm:"not null"`
	RevokedAt          *time.Time `json:"-"`
	BytesUsed          int64      `json:"-" gorm:"not null;default:0"`
	CreatedAt          time.Time  `json:"-" gorm:"autoCreateTime"`
}

func (A2ARunUploadToken) TableName() string { return "a2a_run_upload_tokens" }

// A2AProjectedItem is an idempotency claim for a side effect projected from an
// a2a.task runtime event.
type A2AProjectedItem struct {
	AgentRunID  string    `gorm:"type:uuid;primaryKey"`
	ItemKey     string    `gorm:"primaryKey"`
	WorkspaceID string    `gorm:"type:uuid;not null"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
}

func (A2AProjectedItem) TableName() string { return "a2a_projected_items" }

// ExternalA2ASkill is the summary of one skill advertised by an agent card.
type ExternalA2ASkill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ExternalA2ACapabilities summarizes the optional A2A features an agent supports.
type ExternalA2ACapabilities struct {
	Streaming         bool `json:"streaming"`
	PushNotifications bool `json:"push_notifications"`
}

// ExternalA2ACardSummary is the validated, display-safe part of an agent card.
type ExternalA2ACardSummary struct {
	Name            string                  `json:"name"`
	Description     string                  `json:"description"`
	CardURL         string                  `json:"card_url"`
	InterfaceURL    string                  `json:"interface_url"`
	ProtocolBinding string                  `json:"protocol_binding"`
	ProtocolVersion string                  `json:"protocol_version"`
	ProviderName    string                  `json:"provider_name"`
	Version         string                  `json:"version"`
	Skills          []ExternalA2ASkill      `json:"skills"`
	Capabilities    ExternalA2ACapabilities `json:"capabilities"`
}

// PreviewExternalA2AAgentRequest fetches and validates a card without saving.
type PreviewExternalA2AAgentRequest struct {
	CardURL string `json:"card_url"`
	Token   string `json:"token"`
}

// CreateExternalA2AAgentRequest connects a remote agent and creates its Helpin agent.
type CreateExternalA2AAgentRequest struct {
	CardURL        string   `json:"card_url"`
	Token          string   `json:"token"`
	AllowedTeamIDs []string `json:"allowed_team_ids,omitempty"`
}

// UpdateExternalA2AAgentRequest edits a connection. A nil field is unchanged;
// an empty token clears the stored credential.
type UpdateExternalA2AAgentRequest struct {
	Name           *string   `json:"name,omitempty"`
	Token          *string   `json:"token,omitempty"`
	AllowedTeamIDs *[]string `json:"allowed_team_ids,omitempty"`
	Status         *string   `json:"status,omitempty"`
}
