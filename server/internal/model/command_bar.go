package model

import (
	"encoding/json"
	"time"
)

const (
	CommandBarParseStatusPlan            = "plan"
	CommandBarParseStatusNoMatchingAgent = "no_matching_agent"
)

type CommandBarPageContext struct {
	EntityType   string                 `json:"entity_type"`
	EntityID     string                 `json:"entity_id"`
	DisplayTitle string                 `json:"display_title"`
	RelatedIDs   map[string][]string    `json:"related_ids,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type CommandBarParseRequest struct {
	Text        string                `json:"text"`
	PageContext CommandBarPageContext `json:"page_context"`
}

type CommandBarPlanStep struct {
	AgentID      string                `json:"agent_id"`
	AgentKey     string                `json:"agent_key,omitempty"`
	AgentName    string                `json:"agent_name"`
	Target       CommandBarPageContext `json:"target"`
	Instructions string                `json:"instructions"`
}

type CommandBarPlan struct {
	Steps    []CommandBarPlanStep `json:"steps"`
	RunCount int                  `json:"run_count"`
}

type CommandBarParseResponse struct {
	Status      string            `json:"status"`
	Plan        *CommandBarPlan   `json:"plan,omitempty"`
	Rationale   string            `json:"rationale,omitempty"`
	Reason      string            `json:"reason,omitempty"`
	Suggestions []string          `json:"suggestions,omitempty"`
	Candidates  []CommandBarAgent `json:"candidates,omitempty"`
}

type CommandBarDispatchRequest struct {
	Text        string                `json:"text"`
	PageContext CommandBarPageContext `json:"page_context"`
	Steps       []CommandBarPlanStep  `json:"steps"`
}

type CommandBarDispatchResponse struct {
	Runs []AgentRun `json:"runs"`
}

type CommandBarAgent struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	PresetKey      string   `json:"preset_key,omitempty"`
	Role           string   `json:"role,omitempty"`
	AllowedTargets []string `json:"allowed_targets"`
	AllowedTools   []string `json:"allowed_tools"`
}

type CommandBarUnmetIntent struct {
	ID              string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ActorID         *string         `json:"actor_id" gorm:"type:uuid;index"`
	Prompt          string          `json:"prompt" gorm:"not null"`
	PageContext     json.RawMessage `json:"page_context" gorm:"type:jsonb;not null;default:'{}'"`
	CandidateAgents json.RawMessage `json:"candidate_agents" gorm:"type:jsonb;not null;default:'[]'"`
	Reason          string          `json:"reason" gorm:"not null"`
	CreatedAt       time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

func (CommandBarUnmetIntent) TableName() string { return "command_bar_unmet_intents" }
