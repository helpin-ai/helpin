package model

import (
	"encoding/json"
	"time"
)

// Trigger type constants.
const (
	TriggerStoryStateEntered = "story.state_entered"
	TriggerAgentRunApproved  = "agent_run.approved"
	TriggerCron              = "cron"
)

// Action type constants.
const (
	ActionRunAgent      = "run_agent"
	ActionStartAgentRun = "start_agent_run"
	ActionMoveToState   = "move_to_state"
	ActionMergeBranch   = "merge_branch"
	ActionRunCommand    = "run_command"
	ActionStartFlow     = "start_flow"
)

// AutomationRule represents a user-configured trigger → action mapping.
type AutomationRule struct {
	ID            string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID   string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Name          string          `json:"name" gorm:"not null"`
	Description   *string         `json:"description"`
	Enabled       bool            `json:"enabled" gorm:"not null;default:true"`
	TeamID        *string         `json:"team_id" gorm:"type:uuid;index"`
	WorkflowID    *string         `json:"workflow_id" gorm:"type:uuid;index"`
	TriggerType   string          `json:"trigger_type" gorm:"not null"`
	TriggerConfig json.RawMessage `json:"trigger_config" gorm:"type:jsonb;not null;default:'{}'"`
	ActionType    string          `json:"action_type" gorm:"not null"`
	ActionConfig  json.RawMessage `json:"action_config" gorm:"type:jsonb;not null;default:'{}'"`
	Position      int             `json:"position" gorm:"not null;default:0"`
	StopOnMatch   bool            `json:"stop_on_match" gorm:"not null;default:false"`
	CreatedBy     *string         `json:"created_by" gorm:"type:uuid"`
	CreatedAt     time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (AutomationRule) TableName() string { return "automation_rules" }

// Trigger config shapes (deserialized from JSONB).

// TriggerConfigStateEntered holds config for story.state_entered triggers.
type TriggerConfigStateEntered struct {
	StateID   string `json:"state_id,omitempty"`
	StateType string `json:"state_type,omitempty"` // "started", "done" — match any state of this type
}

// TriggerConfigCron holds config for cron triggers.
type TriggerConfigCron struct {
	Category string `json:"category"` // e.g. "sprint_hourly"
}

// ActionConfigRunCommand holds config for run_command actions.
type ActionConfigRunCommand struct {
	CommandName string          `json:"command_name"`
	Input       json.RawMessage `json:"input,omitempty"`
}

// ActionConfigStartFlow holds config for start_flow actions.
type ActionConfigStartFlow struct {
	TemplateID string          `json:"template_id"`
	AgentID    string          `json:"agent_id,omitempty"`
	FlowInput  json.RawMessage `json:"flow_input,omitempty"`
}

// TriggerConfigRunApproved holds config for agent_run.approved triggers.
type TriggerConfigRunApproved struct {
	StateID string `json:"state_id"`
}

// Action config shapes (deserialized from JSONB).

// ActionConfigRunAgent holds config for start_agent_run actions.
type ActionConfigRunAgent struct {
	TargetType        string  `json:"target_type,omitempty"`
	TargetID          string  `json:"target_id,omitempty"`
	AgentID           string  `json:"agent_id"`
	AdditionalContext *string `json:"additional_context,omitempty"`
}

// ActionConfigMoveToState holds config for move_to_state actions.
type ActionConfigMoveToState struct {
	TargetStateID string `json:"target_state_id"`
}

// ActionConfigMergeBranch holds config for merge_branch actions.
type ActionConfigMergeBranch struct {
	TargetBranch string `json:"target_branch"`
}

// CreateAutomationRuleRequest is the payload for creating a rule.
type CreateAutomationRuleRequest struct {
	WorkspaceID   string          `json:"workspace_id"`
	Name          string          `json:"name"`
	Description   *string         `json:"description"`
	TeamID        *string         `json:"team_id"`
	WorkflowID    *string         `json:"workflow_id"`
	TriggerType   string          `json:"trigger_type"`
	TriggerConfig json.RawMessage `json:"trigger_config"`
	ActionType    string          `json:"action_type"`
	ActionConfig  json.RawMessage `json:"action_config"`
	Position      *int            `json:"position"`
	StopOnMatch   *bool           `json:"stop_on_match"`
}

// UpdateAutomationRuleRequest is the payload for updating a rule.
type UpdateAutomationRuleRequest struct {
	Name          *string          `json:"name"`
	Description   *string          `json:"description"`
	Enabled       *bool            `json:"enabled"`
	TriggerType   *string          `json:"trigger_type"`
	TriggerConfig *json.RawMessage `json:"trigger_config"`
	ActionType    *string          `json:"action_type"`
	ActionConfig  *json.RawMessage `json:"action_config"`
	Position      *int             `json:"position"`
	StopOnMatch   *bool            `json:"stop_on_match"`
}

// AutomationEvent is the internal event emitted to the rule engine.
type AutomationEvent struct {
	WorkspaceID string
	TriggerType string
	// Story-specific (existing, kept for backward compat)
	StoryID string
	StateID string
	AgentID string
	RunID   string
	// Generic fields for non-story triggers
	TargetType string // "story", "epic", "sprint", ""
	TargetID   string // entity UUID
	TeamID     string // for scope matching without a story
}

// RuleExecutionContext tracks chain depth and prevents loops.
type RuleExecutionContext struct {
	OriginEventID string
	Depth         int
	MaxDepth      int
	FiredRuleIDs  []string
}
