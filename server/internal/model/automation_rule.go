package model

import (
	"encoding/json"
	"time"
)

// Trigger type constants.
const (
	TriggerTaskStateEntered  = "task.state_entered"
	TriggerStoryStateEntered = TriggerTaskStateEntered // legacy alias
	TriggerAgentRunApproved  = "agent_run.approved"
	TriggerGitHubPush        = "github.push"
	TriggerGitHubPROpened    = "github.pull_request_opened"
	TriggerGitHubPRMerged    = "github.pull_request_merged"
	TriggerGitHubPRReviewReq = "github.pull_request_review_requested"
	TriggerGitHubReleasePub  = "github.release_published"
	TriggerGitHubCheckSuite  = "github.check_suite_completed"
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

// TriggerConfigStateEntered holds config for task.state_entered triggers.
type TriggerConfigStateEntered struct {
	StateID   string `json:"state_id,omitempty"`
	StateType string `json:"state_type,omitempty"` // "started", "done" — match any state of this type
}

// TriggerConfigCron holds config for cron triggers.
type TriggerConfigCron struct {
	Category string `json:"category,omitempty"` // legacy category, e.g. "workspace_hourly"
	Preset   string `json:"preset,omitempty"`   // e.g. "hourly", "daily", "weekly"
	Schedule string `json:"schedule,omitempty"` // cron expression
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

// TriggerConfigGitHubPRMerged holds config for github.pull_request_merged triggers.
type TriggerConfigGitHubPRMerged struct {
	BaseBranch   string `json:"base_branch,omitempty"`
	RepoFullName string `json:"repo_full_name,omitempty"`
}

// TriggerConfigGitHubPush holds config for github.push triggers.
type TriggerConfigGitHubPush struct {
	Branch       string `json:"branch,omitempty"`
	RepoFullName string `json:"repo_full_name,omitempty"`
}

// TriggerConfigGitHubPullRequest holds config for PR lifecycle triggers.
type TriggerConfigGitHubPullRequest struct {
	BaseBranch   string `json:"base_branch,omitempty"`
	RepoFullName string `json:"repo_full_name,omitempty"`
}

// TriggerConfigGitHubReleasePublished holds config for github.release_published triggers.
type TriggerConfigGitHubReleasePublished struct {
	RepoFullName      string   `json:"repo_full_name,omitempty"`
	TagName           string   `json:"tag_name,omitempty"`
	TagPattern        string   `json:"tag_pattern,omitempty"`
	ReleaseKinds      []string `json:"release_kinds,omitempty"`
	IncludePrerelease bool     `json:"include_prerelease,omitempty"`
}

// TriggerConfigGitHubCheckSuiteCompleted holds config for github.check_suite_completed triggers.
type TriggerConfigGitHubCheckSuiteCompleted struct {
	Branch       string `json:"branch,omitempty"`
	Conclusion   string `json:"conclusion,omitempty"`
	RepoFullName string `json:"repo_full_name,omitempty"`
}

// Action config shapes (deserialized from JSONB).

type ActionConfigRunAgentOutput struct {
	Type           string  `json:"type,omitempty"`
	SpaceID        string  `json:"space_id,omitempty"`
	CollectionID   *string `json:"collection_id,omitempty"`
	IdempotencyKey string  `json:"idempotency_key,omitempty"`
}

// ActionConfigRunAgent holds config for start_agent_run actions.
type ActionConfigRunAgent struct {
	TargetType            string                      `json:"target_type,omitempty"`
	TargetID              string                      `json:"target_id,omitempty"`
	AgentID               string                      `json:"agent_id"`
	LegacyScheduleAgentID string                      `json:"legacy_schedule_agent_id,omitempty"`
	AdditionalContext     *string                     `json:"additional_context,omitempty"`
	BaseBranch            string                      `json:"base_branch,omitempty"`
	WorkingBranch         string                      `json:"working_branch,omitempty"`
	Output                *ActionConfigRunAgentOutput `json:"output,omitempty"`
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
	// Task-specific context.
	TaskID  string
	StoryID string // legacy alias
	StateID string
	AgentID string
	RunID   string
	// Generic fields for non-task triggers
	TargetType        string // "task", "epic", "sprint", ""
	TargetID          string // entity UUID
	TeamID            string // for scope matching without a task
	RepoFullName      string
	RepositoryID      string
	Branch            string
	BaseBranch        string
	PullRequestNumber int
	TagName           string
	TargetCommitish   string
	ReleaseName       string
	ReleaseURL        string
	PublishedAt       *time.Time
	IsPrerelease      bool
	ReleaseKind       string
	Conclusion        string
}

// RuleExecutionContext tracks chain depth and prevents loops.
type RuleExecutionContext struct {
	OriginEventID string
	Depth         int
	MaxDepth      int
	FiredRuleIDs  []string
}
