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
	TicketID    *string         `json:"ticket_id" gorm:"type:uuid;index"`
	EpicID      *string         `json:"epic_id" gorm:"type:uuid;index"`
	RunID       *string         `json:"run_id" gorm:"type:uuid"`
	HandoffType string          `json:"handoff_type" gorm:"not null"` // agent_to_agent, agent_to_human, human_to_agent
	Reason      string          `json:"reason" gorm:"not null"`
	Context     json.RawMessage `json:"context" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt   time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

func (AgentHandoff) TableName() string { return "agent_handoffs" }

// OrchestrateRequest is the request to decompose an epic into stories.
type OrchestrateRequest struct {
	AdditionalContext string `json:"additional_context"`
}

// ProposedStory is a story proposed by the orchestrator before confirmation.
type ProposedStory struct {
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	StoryType     string  `json:"story_type"`
	Estimate      *int    `json:"estimate"`
	AssignAgentID *string `json:"assign_agent_id"`
}

// OrchestrationProposal is the result of an orchestration request.
type OrchestrationProposal struct {
	EpicID          string          `json:"epic_id"`
	Summary         string          `json:"summary"`
	ProposedStories []ProposedStory `json:"proposed_stories"`
	TokensUsed      int             `json:"tokens_used"`
}

// ConfirmOrchestrationRequest confirms and creates the proposed stories.
type ConfirmOrchestrationRequest struct {
	ProposedStories []ProposedStory `json:"proposed_stories"`
}
