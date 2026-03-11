package model

import (
	"encoding/json"
	"time"
)

// AgentHandoff tracks agent-to-agent and agent-to-human handoff events.
type AgentHandoff struct {
	ID          string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	FromAgentID *string         `json:"from_agent_id" gorm:"type:uuid"`
	ToAgentID   *string         `json:"to_agent_id" gorm:"type:uuid"`
	ToUserID    *string         `json:"to_user_id" gorm:"type:uuid"`
	StoryID     *string         `json:"story_id" gorm:"type:uuid;index"`
	ConversationID *string      `json:"conversation_id" gorm:"type:uuid;index"`
	EpicID      *string         `json:"epic_id" gorm:"type:uuid;index"`
	RunID       *string         `json:"run_id" gorm:"type:uuid"`
	HandoffType string          `json:"handoff_type" gorm:"not null"` // agent_to_agent, agent_to_human, human_to_agent
	Reason      string          `json:"reason" gorm:"not null"`
	Context     json.RawMessage `json:"context" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt   time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

func (AgentHandoff) TableName() string { return "agent_handoffs" }
