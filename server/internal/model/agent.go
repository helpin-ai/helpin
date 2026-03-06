package model

import (
	"encoding/json"
	"time"
)

// Agent represents a human or LLM agent in a workspace.
type Agent struct {
	ID                  string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID         string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Name                string          `json:"name" gorm:"not null"`
	AgentKind           string          `json:"agent_kind" gorm:"not null"`
	Role                string          `json:"role"`
	Status              string          `json:"status" gorm:"not null;default:'idle'"`
	BackingUserID       *string         `json:"backing_user_id" gorm:"column:user_id;type:uuid"`
	RuntimeKind         string          `json:"runtime_kind" gorm:"not null;default:'native_claude'"`
	CapabilityProfile   string          `json:"capability_profile" gorm:"not null;default:'engineer'"`
	Skills              json.RawMessage `json:"skills" gorm:"type:jsonb;not null;default:'[]'"`
	TriggerMode         string          `json:"trigger_mode" gorm:"not null;default:'manual'"`
	Model               *string         `json:"model"`
	SystemPrompt        *string         `json:"system_prompt"`
	Tools               json.RawMessage `json:"tools" gorm:"type:jsonb;not null;default:'[]'"`
	MonthlyTokenBudget  *int            `json:"monthly_token_budget"`
	TokensUsedThisMonth int             `json:"tokens_used_this_month" gorm:"not null;default:0"`
	ActiveStoryID       *string         `json:"active_story_id" gorm:"type:uuid"`
	CreatedAt           time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Agent) TableName() string { return "agents" }

// AgentRun represents a single execution run of an agent.
type AgentRun struct {
	ID                string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID       string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	AgentID           string          `json:"agent_id" gorm:"type:uuid;not null;index"`
	StoryID           *string         `json:"story_id" gorm:"type:uuid"`
	TicketID          *string         `json:"ticket_id" gorm:"type:uuid"`
	TargetType        string          `json:"target_type" gorm:"not null;default:'story';index"`
	TargetID          string          `json:"target_id" gorm:"type:uuid;not null;index"`
	RuntimeKind       string          `json:"runtime_kind" gorm:"not null;default:'native_claude'"`
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
	WorkspaceID        string          `json:"workspace_id"`
	Name               string          `json:"name"`
	AgentKind          string          `json:"agent_kind"`
	Role               string          `json:"role"`
	BackingUserID      *string         `json:"backing_user_id"`
	RuntimeKind        *string         `json:"runtime_kind"`
	CapabilityProfile  *string         `json:"capability_profile"`
	Skills             json.RawMessage `json:"skills"`
	TriggerMode        *string         `json:"trigger_mode"`
	Model              *string         `json:"model"`
	SystemPrompt       *string         `json:"system_prompt"`
	Tools              json.RawMessage `json:"tools"`
	MonthlyTokenBudget *int            `json:"monthly_token_budget"`
}

// UpdateAgentRequest is the payload for updating an agent.
type UpdateAgentRequest struct {
	Name               *string         `json:"name"`
	Role               *string         `json:"role"`
	Status             *string         `json:"status"`
	BackingUserID      *string         `json:"backing_user_id"`
	RuntimeKind        *string         `json:"runtime_kind"`
	CapabilityProfile  *string         `json:"capability_profile"`
	Skills             json.RawMessage `json:"skills"`
	TriggerMode        *string         `json:"trigger_mode"`
	Model              *string         `json:"model"`
	SystemPrompt       *string         `json:"system_prompt"`
	Tools              json.RawMessage `json:"tools"`
	MonthlyTokenBudget *int            `json:"monthly_token_budget"`
	ActiveStoryID      *string         `json:"active_story_id"`
}

// AssignAgentRequest assigns an agent to a story.
type AssignAgentRequest struct {
	AgentID string `json:"agent_id"`
}

// ApproveAgentRunRequest approves a pending run outcome.
type ApproveAgentRunRequest struct {
	SendMessage bool `json:"send_message"`
}

// HandoffAgentRunRequest records an explicit handoff from a run.
type HandoffAgentRunRequest struct {
	ToAgentID    *string         `json:"to_agent_id"`
	ToUserID     *string         `json:"to_user_id"`
	HandoffState *string         `json:"handoff_state"`
	Reason       string          `json:"reason"`
	Context      json.RawMessage `json:"context"`
}

// RuntimeProfile describes the policy attached to a capability profile.
type RuntimeProfile struct {
	Name             string   `json:"name"`
	RuntimeKind      string   `json:"runtime_kind"`
	Description      string   `json:"description"`
	AllowedTools     []string `json:"allowed_tools"`
	AllowedCommands  []string `json:"allowed_commands"`
	ApprovalRequired bool     `json:"approval_required"`
	RequiresRepo     bool     `json:"requires_repo"`
}
