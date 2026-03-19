package model

import (
	"encoding/json"
	"time"
)

const (
	FlowTemplateEpicPlanningV2    = "pm.epic_planning_v2"
	FlowTemplateStoryCompletionV1 = "pm.story_completion_v1"
	FlowTemplateAgentStoryRun     = "pm.agent_story_run"
	FlowTemplateCRMDealReviewV1   = "crm.deal_review_v1"

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
	FlowActionRequestChanges = "request_changes"
	FlowActionReject   = "reject"

	InvocationModeInteractive = "interactive"
	InvocationModeAutonomous  = "autonomous"

	FlowNodeEnsureSpecDoc = "ensure_spec_doc"
	FlowNodeSpecApproval  = "spec_approval"
	FlowNodePlanApproval  = "plan_approval"
	FlowNodeCreateStories = "create_stories"
	FlowNodeDone          = "done"

	FlowNodeSpecDraft           = "spec_draft"
	FlowNodeStoryPlan           = "story_plan"
	FlowNodeCompletionAssessment = "completion_assessment"
	FlowNodeCompletionReview     = "completion_review"
	FlowNodeCreateFollowups      = "create_followups"
	FlowNodeDealReview           = "deal_review"
	FlowNodeDealReviewApproval   = "deal_review_approval"
	FlowNodeApplyDealActions     = "apply_deal_actions"
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
	ID             string   `json:"id"`
	Label          string   `json:"label,omitempty"`
	Type           string   `json:"type"`
	RequiredMode   string   `json:"required_mode,omitempty"`
	Actions        []string `json:"actions,omitempty"`
	AllowedTools   []string `json:"allowed_tools,omitempty"`
	LoopbackNodeID *string  `json:"loopback_node_id,omitempty"`
	CommandName    *string  `json:"command_name,omitempty"`
}

type FlowSpec struct {
	TemplateID        string         `json:"template_id"`
	Name              string         `json:"name,omitempty"`
	Description       string         `json:"description,omitempty"`
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

type StartStoryCompletionFlowInput struct {
	AgentID           string `json:"agent_id"`
	AdditionalContext string `json:"additional_context,omitempty"`
}

type StartCRMDealReviewFlowInput struct {
	AgentID           string `json:"agent_id"`
	AdditionalContext string `json:"additional_context,omitempty"`
}

type FlowNodeActionRequest struct {
	ActionType string          `json:"action_type"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

type FlowRequestChangesPayload struct {
	Comment            string          `json:"comment"`
	StructuredFeedback json.RawMessage `json:"structured_feedback,omitempty"`
}

type FlowApprovalDecision struct {
	Decision           string          `json:"decision"`
	ActorID            string          `json:"actor_id,omitempty"`
	Comment            string          `json:"comment,omitempty"`
	StructuredFeedback json.RawMessage `json:"structured_feedback,omitempty"`
	OverridePayload    json.RawMessage `json:"override_payload,omitempty"`
	DecidedAt          *time.Time      `json:"decided_at,omitempty"`
}

type StoryCompletionFollowupProposal struct {
	Title       string  `json:"title"`
	Description string  `json:"description,omitempty"`
	StoryType   string  `json:"story_type,omitempty"`
	Priority    *string `json:"priority,omitempty"`
}

type StoryCompletionAssessment struct {
	Summary   string                          `json:"summary"`
	Followups []StoryCompletionFollowupProposal `json:"followups,omitempty"`
}

type CRMDealReviewActionPlan struct {
	Summary            string  `json:"summary"`
	RecommendedStageID *string `json:"recommended_stage_id,omitempty"`
	Note               *string `json:"note,omitempty"`
}

type FlowInteractiveMessageRequest struct {
	Content string `json:"content"`
}

type FlowRetryNodeRequest struct{}

type FlowRunListResponse struct {
	Data  []FlowRunView `json:"data"`
	Total int64         `json:"total"`
}
