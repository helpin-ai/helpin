package model

import (
	"encoding/json"
	"time"
)

const (
	FlowTemplateEpicPlanningV1 = "pm.epic_planning_v1"

	FlowStatusRunning          = "running"
	FlowStatusAwaitingInput    = "awaiting_input"
	FlowStatusAwaitingApproval = "awaiting_approval"
	FlowStatusCompleted        = "completed"
	FlowStatusFailed           = "failed"
	FlowStatusCancelled        = "cancelled"

	FlowNodeTypeInteractiveAgent = "interactive_agent"
	FlowNodeTypeAgentTask        = "agent_task"
	FlowNodeTypeApprovalGate     = "approval_gate"
	FlowNodeTypeSystemAction     = "system_action"
	FlowNodeTypeTerminal         = "terminal"

	FlowNodeStatusQueued           = "queued"
	FlowNodeStatusRunning          = "running"
	FlowNodeStatusAwaitingInput    = "awaiting_input"
	FlowNodeStatusAwaitingApproval = "awaiting_approval"
	FlowNodeStatusCompleted        = "completed"
	FlowNodeStatusFailed           = "failed"
	FlowNodeStatusCancelled        = "cancelled"
	FlowNodeStatusSkipped          = "skipped"

	FlowTriggerManual             = "manual"
	FlowTriggerInternalDomainHook = "internal_domain_hook"
	FlowTriggerCron               = "cron"
	FlowTriggerStoryStateEntered  = TriggerStoryStateEntered
	FlowTriggerAgentRunApproved   = TriggerAgentRunApproved
	FlowTriggerAssignmentEvent    = "assignment_event"

	FlowActionFinalize = "finalize"
	FlowActionApprove  = "approve"
	FlowActionReject   = "reject"

	InvocationModeInteractive = "interactive"
	InvocationModeAutonomous  = "autonomous"

	FlowNodeEnsureSpecDoc = "ensure_spec_doc"
	FlowNodeSpecPlanning  = "spec_planning"
	FlowNodeSpecApproval  = "spec_approval"
	FlowNodeStoryPlanning = "story_planning"
	FlowNodePlanApproval  = "plan_approval"
	FlowNodeCreateStories = "create_stories"
	FlowNodeDone          = "done"
)

// FlowRun is a durable orchestration instance for a built-in or future user-authored flow.
type FlowRun struct {
	ID                 string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID        string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	TemplateID         string          `json:"template_id" gorm:"not null;index"`
	TemplateVersion    int             `json:"template_version" gorm:"not null;default:1"`
	TargetType         string          `json:"target_type" gorm:"not null;index"`
	TargetID           string          `json:"target_id" gorm:"type:uuid;not null;index"`
	Status             string          `json:"status" gorm:"not null;index"`
	CurrentNodeID      *string         `json:"current_node_id,omitempty"`
	TriggerType        string          `json:"trigger_type" gorm:"not null;default:'manual'"`
	TriggerPayload     json.RawMessage `json:"trigger_payload" gorm:"type:jsonb;not null;default:'{}'"`
	Input              json.RawMessage `json:"input" gorm:"type:jsonb;not null;default:'{}'"`
	OutputSummary      json.RawMessage `json:"output_summary" gorm:"type:jsonb;not null;default:'{}'"`
	SpecSnapshot       json.RawMessage `json:"spec_snapshot" gorm:"type:jsonb;not null;default:'{}'"`
	DedupeKey          *string         `json:"dedupe_key,omitempty" gorm:"index"`
	StartedBy          *string         `json:"started_by,omitempty" gorm:"type:uuid"`
	CompletedAt        *time.Time      `json:"completed_at,omitempty"`
	CancellationReason *string         `json:"cancellation_reason,omitempty"`
	RetryCount         int             `json:"retry_count" gorm:"not null;default:0"`
	CreatedAt          time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (FlowRun) TableName() string { return "flow_runs" }

type FlowNodeRun struct {
	ID             string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	FlowRunID      string          `json:"flow_run_id" gorm:"type:uuid;not null;index"`
	NodeID         string          `json:"node_id" gorm:"not null;index"`
	NodeType       string          `json:"node_type" gorm:"not null"`
	Status         string          `json:"status" gorm:"not null;index"`
	AttemptCount   int             `json:"attempt_count" gorm:"not null;default:1"`
	AgentID        *string         `json:"agent_id,omitempty" gorm:"type:uuid"`
	ChildRunID     *string         `json:"child_run_id,omitempty" gorm:"type:uuid;index"`
	ChildSessionID *string         `json:"child_session_id,omitempty" gorm:"type:uuid;index"`
	Input          json.RawMessage `json:"input" gorm:"type:jsonb;not null;default:'{}'"`
	Output         json.RawMessage `json:"output" gorm:"type:jsonb;not null;default:'{}'"`
	ErrorMessage   *string         `json:"error_message,omitempty"`
	StartedAt      *time.Time      `json:"started_at,omitempty"`
	CompletedAt    *time.Time      `json:"completed_at,omitempty"`
	CreatedAt      time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (FlowNodeRun) TableName() string { return "flow_node_runs" }

type FlowTrigger struct {
	ID            string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID   string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	TemplateID    string          `json:"template_id" gorm:"not null;index"`
	Enabled       bool            `json:"enabled" gorm:"not null;default:true"`
	TriggerType   string          `json:"trigger_type" gorm:"not null;index"`
	TriggerConfig json.RawMessage `json:"trigger_config" gorm:"type:jsonb;not null;default:'{}'"`
	ScopeFilters  json.RawMessage `json:"scope_filters" gorm:"type:jsonb;not null;default:'{}'"`
	DedupeKey     *string         `json:"dedupe_key,omitempty"`
	CreatedAt     time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (FlowTrigger) TableName() string { return "flow_triggers" }

type FlowNodeSpec struct {
	ID           string   `json:"id"`
	Type         string   `json:"type"`
	RequiredMode string   `json:"required_mode,omitempty"`
	Actions      []string `json:"actions,omitempty"`
}

type FlowSpec struct {
	TemplateID        string         `json:"template_id"`
	TemplateVersion   int            `json:"template_version"`
	TargetType        string         `json:"target_type"`
	SupportedTriggers []string       `json:"supported_triggers"`
	Nodes             []FlowNodeSpec `json:"nodes"`
}

type FlowRunView struct {
	Run      *FlowRun      `json:"run"`
	Spec     FlowSpec      `json:"spec"`
	NodeRuns []FlowNodeRun `json:"node_runs"`
}

type StartFlowRunRequest struct {
	TemplateID     string          `json:"template_id"`
	TargetType     string          `json:"target_type"`
	TargetID       string          `json:"target_id"`
	Input          json.RawMessage `json:"input,omitempty"`
	TriggerType    string          `json:"trigger_type,omitempty"`
	TriggerPayload json.RawMessage `json:"trigger_payload,omitempty"`
}

type StartEpicPlanningFlowInput struct {
	SpecPlannerAgentID  string `json:"spec_planner_agent_id"`
	StoryPlannerAgentID string `json:"story_planner_agent_id,omitempty"`
	AdditionalContext   string `json:"additional_context,omitempty"`
}

type FlowNodeActionRequest struct {
	ActionType string          `json:"action_type"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

type FlowInteractiveMessageRequest struct {
	Content string `json:"content"`
}

type FlowRetryNodeRequest struct{}

type FlowRunListResponse struct {
	Data  []FlowRunView `json:"data"`
	Total int64         `json:"total"`
}
