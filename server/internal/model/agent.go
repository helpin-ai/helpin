package model

import (
	"encoding/json"
	"time"
)

const (
	AgentPresetEpicPlanner  = "epic_planner"
	AgentPresetStoryPlanner = "story_planner"
	AgentPresetCRMOperator  = "crm_operator"
	AgentPresetSupportAgent = "support_agent"
	AgentPresetCodeBuilder  = "code_builder"
	AgentPresetReviewAgent  = "review_agent"

	AgentModelProviderAnthropic           = "anthropic"
	AgentModelProviderOpenAI              = "openai"
	AgentModelProviderOpenRouter          = "openrouter"
	AgentModelProviderOpenRouterResponses = "openrouter-responses"
)

// Agent represents an LLM agent in a workspace.
type Agent struct {
	ID                    string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID           string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	IsSystem              bool            `json:"is_system" gorm:"not null;default:false"`
	Name                  string          `json:"name" gorm:"not null"`
	PresetKey             string          `json:"preset_key"`
	Role                  string          `json:"role"`
	Status                string          `json:"status" gorm:"not null;default:'idle'"`
	RuntimeKind           string          `json:"runtime_kind" gorm:"not null;default:'opencode'"`
	Skills                json.RawMessage `json:"skills" gorm:"type:jsonb;not null;default:'[]'"`
	TriggerMode           string          `json:"trigger_mode" gorm:"not null;default:'manual'"`
	Provider              *string         `json:"provider"`
	Model                 *string         `json:"model"`
	SystemPrompt          *string         `json:"system_prompt"`
	PlanningNotes         *string         `json:"planning_notes"`
	MonthlyTokenBudget    *int            `json:"monthly_token_budget"`
	TokensUsedThisMonth   int             `json:"tokens_used_this_month" gorm:"not null;default:0"`
	ActiveStoryID         *string         `json:"active_story_id" gorm:"type:uuid"`
	TeamID                *string         `json:"team_id" gorm:"type:uuid;index"`
	AllowedTools          json.RawMessage `json:"allowed_tools" gorm:"type:jsonb;not null;default:'[]'"`
	AllowedCommands       json.RawMessage `json:"allowed_commands" gorm:"type:jsonb;not null;default:'[]'"`
	AllowedTargets        json.RawMessage `json:"allowed_targets" gorm:"type:jsonb;not null;default:'[]'"`
	Schedule              *string         `json:"schedule"`
	ApprovalMode          string          `json:"approval_mode" gorm:"not null;default:'preset_default'"`
	MaxConcurrentRuns     int             `json:"max_concurrent_runs" gorm:"not null;default:1"`
	DefaultInvocationMode string          `json:"default_invocation_mode" gorm:"not null;default:'autonomous'"`
	SupportedModes        []string        `json:"supported_modes" gorm:"-"`
	CreatedAt             time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt             time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Agent) TableName() string { return "agents" }

// AgentRun represents a single execution run of an agent.
type AgentRun struct {
	ID                string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID       string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	AgentID           string          `json:"agent_id" gorm:"type:uuid;not null;index"`
	StoryID           *string         `json:"story_id" gorm:"type:uuid"`
	ConversationID    *string         `json:"conversation_id" gorm:"type:uuid"`
	TargetType        string          `json:"target_type" gorm:"not null;default:'story';index"`
	TargetID          string          `json:"target_id" gorm:"type:uuid;not null;index"`
	RuntimeKind       string          `json:"runtime_kind" gorm:"not null;default:'opencode'"`
	InvocationMode    string          `json:"invocation_mode" gorm:"not null;default:'autonomous'"`
	ParentRunID       *string         `json:"parent_run_id" gorm:"type:uuid;index"`
	HandoffState      *string         `json:"handoff_state"`
	ApprovalState     string          `json:"approval_state" gorm:"not null;default:'not_required'"`
	TriggeredByUserID *string         `json:"triggered_by_user_id" gorm:"type:uuid"`
	Status            string          `json:"status" gorm:"not null;default:'queued'"`
	WorkflowID        *string         `json:"workflow_id"`
	WorkflowRunID     *string         `json:"workflow_run_id"`
	TaskQueue         *string         `json:"task_queue"`
	RunnerPool        *string         `json:"runner_pool"`
	RepositoryID      *string         `json:"repository_id" gorm:"type:uuid;index"`
	RepoFullName      *string         `json:"repo_full_name"`
	BaseBranch        *string         `json:"base_branch"`
	WorkingBranch     *string         `json:"working_branch"`
	DeliveryTargetID  *string         `json:"delivery_target_id" gorm:"type:uuid;index"`
	ExecutionStage    *string         `json:"execution_stage"`
	LastHeartbeatAt   *time.Time      `json:"last_heartbeat_at"`
	Input             json.RawMessage `json:"input" gorm:"type:jsonb;not null;default:'{}'"`
	OutputSummary     json.RawMessage `json:"output_summary" gorm:"type:jsonb;not null;default:'{}'"`
	TokensUsed        int             `json:"tokens_used" gorm:"not null;default:0"`
	ErrorMessage      *string         `json:"error_message"`
	StartedAt         *time.Time      `json:"started_at"`
	CompletedAt       *time.Time      `json:"completed_at"`
	CreatedAt         time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (AgentRun) TableName() string { return "agent_runs" }

// AgentRunArtifact represents a first-class artifact produced by an agent run.
type AgentRunArtifact struct {
	ID            string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID   string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	RunID         string          `json:"run_id" gorm:"type:uuid;not null;index"`
	ArtifactType  string          `json:"artifact_type" gorm:"not null"`
	Format        string          `json:"format" gorm:"not null;default:'text'"`
	StorageMode   string          `json:"storage_mode" gorm:"not null;default:'inline'"`
	InlineContent *string         `json:"inline_content"`
	ObjectKey     *string         `json:"object_key"`
	Metadata      json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	SequenceNo    int             `json:"sequence_no" gorm:"not null;default:0"`
	CreatedAt     time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

func (AgentRunArtifact) TableName() string { return "agent_run_artifacts" }

// CreateAgentRequest is the payload for creating an agent.
type CreateAgentRequest struct {
	WorkspaceID           string          `json:"workspace_id"`
	Name                  string          `json:"name"`
	PresetKey             *string         `json:"preset_key"`
	Role                  string          `json:"role"`
	RuntimeKind           *string         `json:"runtime_kind"`
	Skills                json.RawMessage `json:"skills"`
	TriggerMode           *string         `json:"trigger_mode"`
	Provider              *string         `json:"provider"`
	Model                 *string         `json:"model"`
	SystemPrompt          *string         `json:"system_prompt"`
	PlanningNotes         *string         `json:"planning_notes"`
	MonthlyTokenBudget    *int            `json:"monthly_token_budget"`
	TeamID                *string         `json:"team_id"`
	AllowedTools          json.RawMessage `json:"allowed_tools"`
	AllowedCommands       json.RawMessage `json:"allowed_commands"`
	AllowedTargets        json.RawMessage `json:"allowed_targets"`
	Schedule              *string         `json:"schedule"`
	ApprovalMode          *string         `json:"approval_mode"`
	MaxConcurrentRuns     *int            `json:"max_concurrent_runs"`
	DefaultInvocationMode *string         `json:"default_invocation_mode"`
}

// UpdateAgentRequest is the payload for updating an agent.
type UpdateAgentRequest struct {
	Name                  *string         `json:"name"`
	PresetKey             *string         `json:"preset_key"`
	Role                  *string         `json:"role"`
	Status                *string         `json:"status"`
	RuntimeKind           *string         `json:"runtime_kind"`
	Skills                json.RawMessage `json:"skills"`
	TriggerMode           *string         `json:"trigger_mode"`
	Provider              *string         `json:"provider"`
	Model                 *string         `json:"model"`
	SystemPrompt          *string         `json:"system_prompt"`
	PlanningNotes         *string         `json:"planning_notes"`
	MonthlyTokenBudget    *int            `json:"monthly_token_budget"`
	ActiveStoryID         *string         `json:"active_story_id"`
	TeamID                *string         `json:"team_id"`
	AllowedTools          json.RawMessage `json:"allowed_tools"`
	AllowedCommands       json.RawMessage `json:"allowed_commands"`
	AllowedTargets        json.RawMessage `json:"allowed_targets"`
	Schedule              *string         `json:"schedule"`
	ApprovalMode          *string         `json:"approval_mode"`
	MaxConcurrentRuns     *int            `json:"max_concurrent_runs"`
	DefaultInvocationMode *string         `json:"default_invocation_mode"`
}

// AssignAgentRequest assigns an agent to a story.
type AssignAgentRequest struct {
	AgentID string `json:"agent_id"`
}

// ApproveAgentRunRequest approves a pending run outcome.
type ApproveAgentRunRequest struct {
	SendMessage bool `json:"send_message"`
}

const (
	AgentRunResumeIntentReply          = "reply"
	AgentRunResumeIntentApprove        = "approve"
	AgentRunResumeIntentRequestChanges = "request_changes"
)

const (
	AgentRunStatusQueued           = "queued"
	AgentRunStatusRunning          = "running"
	AgentRunStatusAwaitingInput    = "awaiting_input"
	AgentRunStatusAwaitingApproval = "awaiting_approval"
	AgentRunStatusCompleted        = "completed"
	AgentRunStatusFailed           = "failed"
	AgentRunStatusCancelled        = "cancelled"
)

// HandoffAgentRunRequest records an explicit handoff from a run.
type HandoffAgentRunRequest struct {
	ToAgentID    *string         `json:"to_agent_id"`
	ToUserID     *string         `json:"to_user_id"`
	HandoffState *string         `json:"handoff_state"`
	Reason       string          `json:"reason"`
	Context      json.RawMessage `json:"context"`
}

type StartAgentRunRequest struct {
	AgentID           string  `json:"agent_id,omitempty"`
	AdditionalContext *string `json:"additional_context,omitempty"`
}

type SendAgentRunMessageRequest struct {
	Content string `json:"content"`
}

type SendAgentRunRequestChangesRequest struct {
	Content string `json:"content"`
}

type ResumeAgentRunRequest struct {
	Intent      string `json:"intent"`
	Content     string `json:"content,omitempty"`
	SendMessage bool   `json:"send_message,omitempty"`
}

// RuntimeProfile describes the policy attached to a capability profile.
type RuntimeProfile struct {
	Name               string   `json:"name"`
	RuntimeKind        string   `json:"runtime_kind"`
	Description        string   `json:"description"`
	AllowedTools       []string `json:"allowed_tools"`
	AllowedCommands    []string `json:"allowed_commands"`
	AllowedTargetTypes []string `json:"allowed_target_types"`
	ApprovalRequired   bool     `json:"approval_required"`
	RequiresRepo       bool     `json:"requires_repo"`
}

// AgentPresetDefinition describes a preset/template for a generic agent.
type AgentPresetDefinition struct {
	Key                   string   `json:"key"`
	Label                 string   `json:"label"`
	Description           string   `json:"description"`
	DefaultRole           string   `json:"default_role"`
	RuntimeKind           string   `json:"runtime_kind"`
	DefaultTriggerMode    string   `json:"default_trigger_mode"`
	AllowedTriggerModes   []string `json:"allowed_trigger_modes"`
	AllowedTools          []string `json:"allowed_tools"`
	AllowedCommands       []string `json:"allowed_commands"`
	AllowedTargetTypes    []string `json:"allowed_target_types"`
	ApprovalMode          string   `json:"approval_mode"`
	DefaultInvocationMode string   `json:"default_invocation_mode"`
	SupportedModes        []string `json:"supported_modes"`
	SystemPrompt          *string  `json:"system_prompt,omitempty"`
}

type AgentModelProviderOption struct {
	Value            string `json:"value"`
	Label            string `json:"label"`
	ModelPlaceholder string `json:"model_placeholder"`
}

// ToolCatalogEntry describes a tool with its category and preset usage.
type ToolCatalogEntry struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Category    string      `json:"category"`
	InputSchema interface{} `json:"input_schema"`
	Presets     []string    `json:"presets"`
}

// ToolCatalogResponse is the response for GET /pm/tool-catalog.
type ToolCatalogResponse struct {
	Tools      []ToolCatalogEntry `json:"tools"`
	Categories []string           `json:"categories"`
}
