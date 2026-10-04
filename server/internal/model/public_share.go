package model

import "time"

const (
	PublicShareResourceDockChat = "dock_chat"
	PublicShareResourceAgentRun = "agent_run"
)

// PublicShare is a revocable bearer link for a live, read-only resource.
type PublicShare struct {
	ID           string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID  string     `json:"-" gorm:"type:uuid;not null;index"`
	ResourceType string     `json:"resource_type" gorm:"type:text;not null;uniqueIndex:idx_public_shares_active_resource,priority:1,where:revoked_at IS NULL"`
	ResourceID   string     `json:"resource_id" gorm:"type:uuid;not null;uniqueIndex:idx_public_shares_active_resource,priority:2,where:revoked_at IS NULL"`
	Token        string     `json:"token" gorm:"type:text;not null;uniqueIndex"`
	CreatedBy    string     `json:"-" gorm:"type:uuid;not null"`
	RevokedBy    *string    `json:"-" gorm:"type:uuid"`
	RevokedAt    *time.Time `json:"-" gorm:"index"`
	CreatedAt    time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the public shares table name.
func (PublicShare) TableName() string { return "public_shares" }

// PublicShareLink is returned to authenticated clients managing a share.
type PublicShareLink struct {
	Token string `json:"token"`
	URL   string `json:"url"`
}

// PublicSharedMessage deliberately excludes storage, actor, and runtime metadata.
type PublicSharedMessage struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

// PublicSharedDockChat is the live, public projection of an Ask chat.
type PublicSharedDockChat struct {
	Title     string                `json:"title"`
	OpenPath  string                `json:"open_path,omitempty"`
	Messages  []PublicSharedMessage `json:"messages"`
	UpdatedAt time.Time             `json:"updated_at"`
}

// PublicSharedAgentRun is the live, public projection of an agent run.
type PublicSharedAgentRun struct {
	Title        string                `json:"title"`
	OpenPath     string                `json:"open_path,omitempty"`
	Session      *CodingSession        `json:"session,omitempty"`
	Events       []CodingSessionEvent  `json:"events"`
	Interactions []AgentRunInteraction `json:"interactions"`
	Artifacts    []AgentRunArtifact    `json:"artifacts"`
}

// PublicSharedResource is the anonymous response for one active share token.
type PublicSharedResource struct {
	ResourceType string                `json:"resource_type"`
	DockChat     *PublicSharedDockChat `json:"dock_chat,omitempty"`
	AgentRun     *PublicSharedAgentRun `json:"agent_run,omitempty"`
}
