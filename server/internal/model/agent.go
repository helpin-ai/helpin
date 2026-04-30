package model

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	AgentPresetEpicPlanner  = "epic_planner"
	AgentPresetTaskPlanner  = "task_planner"
	AgentPresetCRMOperator  = "crm_operator"
	AgentPresetSupportAgent = "support_agent"
	AgentPresetCodeBuilder  = "code_builder"
	AgentPresetReviewAgent  = "review_agent"
	// AgentPresetCommandAgent is stored under the legacy "researcher" key so
	// existing seeded system-agent rows reconcile in place while the product
	// surface moves to "Command Agent".
	AgentPresetCommandAgent = "researcher"
	AgentPresetResearcher   = AgentPresetCommandAgent

	AgentModelProviderAnthropic           = "anthropic"
	AgentModelProviderOpenAI              = "openai"
	AgentModelProviderOpenRouter          = "openrouter"
	AgentModelProviderOpenRouterResponses = "openrouter-responses"

	AgentRunTriggerSourceManual         = "manual"
	AgentRunTriggerSourceAutomationRule = "automation_rule"
	AgentRunTriggerSourceSystem         = "system"
	AgentRunTriggerSourceCommandBar     = "command_bar"

	AgentRunTriggerTypeManual     = "manual"
	AgentRunTriggerTypeCommandBar = "command_bar"
)

// Agent represents an LLM agent in a workspace.
type Agent struct {
	ID                         string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID                string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	IsSystem                   bool            `json:"is_system" gorm:"not null;default:false"`
	Name                       string          `json:"name" gorm:"not null"`
	PresetKey                  string          `json:"preset_key"`
	PresetVersionKey           string          `json:"preset_version_key"`
	SourcePresetKey            string          `json:"source_preset_key"`
	SourcePresetVersionKey     string          `json:"source_preset_version_key"`
	SourceTemplateID           *string         `json:"source_template_id" gorm:"type:uuid;index"`
	SourceTemplateKey          string          `json:"source_template_key"`
	Role                       string          `json:"role"`
	Status                     string          `json:"status" gorm:"not null;default:'idle'"`
	RuntimeKind                string          `json:"runtime_kind" gorm:"not null;default:'opencode'"`
	Skills                     AgentSkillRefs  `json:"skills" gorm:"type:jsonb;not null;default:'[]'"`
	TriggerMode                string          `json:"trigger_mode" gorm:"not null;default:'manual'"`
	Provider                   *string         `json:"provider"`
	Model                      *string         `json:"model"`
	ExecutionConfig            JSONBlob        `json:"execution_config" gorm:"type:jsonb;not null;default:'{}'"`
	SystemPrompt               *string         `json:"system_prompt"`
	InstructionTemplateVersion string          `json:"instruction_template_version" gorm:"not null;default:''"`
	PlanningNotes              *string         `json:"planning_notes"`
	MonthlyTokenBudget         *int            `json:"monthly_token_budget"`
	TokensUsedThisMonth        int             `json:"tokens_used_this_month" gorm:"not null;default:0"`
	ActiveTaskID               *string         `json:"active_task_id" gorm:"column:active_task_id;type:uuid"`
	TeamID                     *string         `json:"team_id" gorm:"type:uuid;index"`
	AllowedTools               json.RawMessage `json:"allowed_tools" gorm:"type:jsonb;not null;default:'[]'"`
	AllowedCommands            json.RawMessage `json:"allowed_commands" gorm:"type:jsonb;not null;default:'[]'"`
	AllowedTargets             json.RawMessage `json:"allowed_targets" gorm:"type:jsonb;not null;default:'[]'"`
	ApprovalMode               string          `json:"approval_mode" gorm:"not null;default:'preset_default'"`
	MaxConcurrentRuns          int             `json:"max_concurrent_runs" gorm:"not null;default:1"`
	DefaultInvocationMode      string          `json:"default_invocation_mode" gorm:"not null;default:'autonomous'"`
	SupportedModes             []string        `json:"supported_modes" gorm:"-"`
	ResolvedSkillInstructions  string          `json:"-" gorm:"-"`
	CreatedAt                  time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                  time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Agent) TableName() string { return "agents" }

func (a *Agent) EffectivePresetKey() string {
	if a == nil {
		return ""
	}
	return strings.TrimSpace(a.PresetKey)
}

func (a *Agent) EffectivePresetVersionKey() string {
	if a == nil {
		return ""
	}
	return strings.TrimSpace(a.PresetVersionKey)
}

type WorkspaceAgentPresetVersion struct {
	ID                         string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID                string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	FamilyKey                  string          `json:"family_key" gorm:"not null;index"`
	VersionKey                 string          `json:"version_key" gorm:"not null;index"`
	Label                      string          `json:"label" gorm:"not null"`
	Description                *string         `json:"description"`
	SourceVersionKey           *string         `json:"source_version_key"`
	RuntimeKind                string          `json:"runtime_kind" gorm:"not null"`
	Provider                   *string         `json:"provider"`
	Model                      *string         `json:"model"`
	ExecutionConfig            JSONBlob        `json:"execution_config" gorm:"type:jsonb;not null;default:'{}'"`
	SystemPrompt               *string         `json:"system_prompt"`
	InstructionPreamble        *string         `json:"instruction_preamble"`
	InstructionSkills          json.RawMessage `json:"instruction_skills" gorm:"type:jsonb;not null;default:'[]'"`
	InstructionTemplateVersion string          `json:"instruction_template_version" gorm:"not null;default:''"`
	AllowedTools               json.RawMessage `json:"allowed_tools" gorm:"type:jsonb;not null;default:'[]'"`
	SupportedModes             json.RawMessage `json:"supported_modes" gorm:"type:jsonb;not null;default:'[]'"`
	ApprovalMode               string          `json:"approval_mode" gorm:"not null;default:'preset_default'"`
	DefaultInvocationMode      string          `json:"default_invocation_mode" gorm:"not null;default:'autonomous'"`
	CreatedBy                  *string         `json:"created_by" gorm:"type:uuid"`
	UpdatedBy                  *string         `json:"updated_by" gorm:"type:uuid"`
	LastEditedAt               *time.Time      `json:"last_edited_at"`
	DeletedAt                  *time.Time      `json:"deleted_at" gorm:"index"`
	CreatedAt                  time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                  time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (WorkspaceAgentPresetVersion) TableName() string { return "workspace_agent_preset_versions" }

// AgentRun represents a single execution run of an agent.
type AgentRun struct {
	ID                string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID       string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	AgentID           string          `json:"agent_id" gorm:"type:uuid;not null;index"`
	TaskID            *string         `json:"task_id" gorm:"column:task_id;type:uuid"`
	ConversationID    *string         `json:"conversation_id" gorm:"type:uuid"`
	TargetType        string          `json:"target_type" gorm:"not null;default:'task';index"`
	TargetID          string          `json:"target_id" gorm:"type:uuid;not null;index"`
	RuntimeKind       string          `json:"runtime_kind" gorm:"not null;default:'opencode'"`
	InvocationMode    string          `json:"invocation_mode" gorm:"not null;default:'autonomous'"`
	ParentRunID       *string         `json:"parent_run_id" gorm:"type:uuid;index"`
	HandoffState      *string         `json:"handoff_state"`
	ApprovalState     string          `json:"approval_state" gorm:"not null;default:'not_required'"`
	PauseReason       string          `json:"pause_reason" gorm:"not null;default:'none'"`
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
	CachedInputTokens int             `json:"cached_input_tokens" gorm:"not null;default:0"`
	InputTokens       int             `json:"input_tokens" gorm:"not null;default:0"`
	OutputTokens      int             `json:"output_tokens" gorm:"not null;default:0"`
	TokensUsed        int             `json:"tokens_used" gorm:"not null;default:0"`
	ErrorMessage      *string         `json:"error_message"`
	StartedAt         *time.Time      `json:"started_at"`
	CompletedAt       *time.Time      `json:"completed_at"`
	CreatedAt         time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
	TargetInfo        *AgentRunTarget `json:"target_info,omitempty" gorm:"-"`
}

// AgentRunTarget is a computed sidecar with resolved display info for the
// run's target entity (e.g. the PM task title and task_key). It is not
// persisted and is populated by the service layer on read paths so UI can
// surface a human label instead of a raw UUID.
type AgentRunTarget struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Title      string `json:"title,omitempty"`
	TaskKey    string `json:"task_key,omitempty"`
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

// AgentTriggerExecution captures one durable trigger firing attempt for an agent binding.
type AgentTriggerExecution struct {
	ID            string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID   string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	AgentID       string     `json:"agent_id" gorm:"type:uuid;not null;index"`
	BindingID     string     `json:"binding_id" gorm:"not null;index"`
	BindingKind   string     `json:"binding_kind" gorm:"not null;index"`
	TriggerType   *string    `json:"trigger_type"`
	ReferenceID   *string    `json:"reference_id" gorm:"index"`
	ReferenceType *string    `json:"reference_type"`
	TargetType    *string    `json:"target_type" gorm:"index"`
	TargetID      *string    `json:"target_id" gorm:"index"`
	RunID         *string    `json:"run_id" gorm:"type:uuid;index"`
	Status        string     `json:"status" gorm:"not null;index"`
	ErrorMessage  *string    `json:"error_message"`
	FiredAt       time.Time  `json:"fired_at" gorm:"not null;index"`
	StartedAt     *time.Time `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	CreatedAt     time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (AgentTriggerExecution) TableName() string { return "agent_trigger_executions" }

// CreateAgentRequest is the payload for creating an agent.
type CreateAgentRequest struct {
	WorkspaceID           string          `json:"workspace_id"`
	Name                  string          `json:"name"`
	PresetKey             *string         `json:"preset_key"`
	PresetVersionKey      *string         `json:"preset_version_key"`
	Role                  string          `json:"role"`
	RuntimeKind           *string         `json:"runtime_kind"`
	Skills                AgentSkillRefs  `json:"skills"`
	TriggerMode           *string         `json:"trigger_mode"`
	Provider              *string         `json:"provider"`
	Model                 *string         `json:"model"`
	ExecutionConfig       json.RawMessage `json:"execution_config"`
	SystemPrompt          *string         `json:"system_prompt"`
	PlanningNotes         *string         `json:"planning_notes"`
	MonthlyTokenBudget    *int            `json:"monthly_token_budget"`
	TeamID                *string         `json:"team_id"`
	AllowedTools          json.RawMessage `json:"allowed_tools"`
	AllowedCommands       json.RawMessage `json:"allowed_commands"`
	AllowedTargets        json.RawMessage `json:"allowed_targets"`
	ApprovalMode          *string         `json:"approval_mode"`
	MaxConcurrentRuns     *int            `json:"max_concurrent_runs"`
	DefaultInvocationMode *string         `json:"default_invocation_mode"`
}

// UpdateAgentRequest is the payload for updating an agent.
type UpdateAgentRequest struct {
	Name                  *string         `json:"name"`
	PresetKey             *string         `json:"preset_key"`
	PresetVersionKey      *string         `json:"preset_version_key"`
	Role                  *string         `json:"role"`
	Status                *string         `json:"status"`
	RuntimeKind           *string         `json:"runtime_kind"`
	Skills                AgentSkillRefs  `json:"skills"`
	TriggerMode           *string         `json:"trigger_mode"`
	Provider              *string         `json:"provider"`
	Model                 *string         `json:"model"`
	ExecutionConfig       json.RawMessage `json:"execution_config"`
	SystemPrompt          *string         `json:"system_prompt"`
	PlanningNotes         *string         `json:"planning_notes"`
	MonthlyTokenBudget    *int            `json:"monthly_token_budget"`
	ActiveTaskID          *string         `json:"active_task_id"`
	TeamID                *string         `json:"team_id"`
	AllowedTools          json.RawMessage `json:"allowed_tools"`
	AllowedCommands       json.RawMessage `json:"allowed_commands"`
	AllowedTargets        json.RawMessage `json:"allowed_targets"`
	ApprovalMode          *string         `json:"approval_mode"`
	MaxConcurrentRuns     *int            `json:"max_concurrent_runs"`
	DefaultInvocationMode *string         `json:"default_invocation_mode"`
}

type CreateWorkspaceAgentPresetVersionRequest struct {
	WorkspaceID           string          `json:"workspace_id"`
	FamilyKey             string          `json:"family_key"`
	Label                 string          `json:"label"`
	Description           *string         `json:"description"`
	SourceVersionKey      *string         `json:"source_version_key"`
	RuntimeKind           *string         `json:"runtime_kind"`
	Provider              *string         `json:"provider"`
	Model                 *string         `json:"model"`
	ExecutionConfig       json.RawMessage `json:"execution_config"`
	SystemPrompt          *string         `json:"system_prompt"`
	InstructionPreamble   *string         `json:"instruction_preamble"`
	InstructionSkills     json.RawMessage `json:"instruction_skills"`
	AllowedTools          json.RawMessage `json:"allowed_tools"`
	SupportedModes        json.RawMessage `json:"supported_modes"`
	ApprovalMode          *string         `json:"approval_mode"`
	DefaultInvocationMode *string         `json:"default_invocation_mode"`
}

type UpdateWorkspaceAgentPresetVersionRequest struct {
	// Label changes are regular edits. The backend does not maintain rename-specific history.
	Label                 *string         `json:"label"`
	Description           *string         `json:"description"`
	RuntimeKind           *string         `json:"runtime_kind"`
	Provider              *string         `json:"provider"`
	Model                 *string         `json:"model"`
	ExecutionConfig       json.RawMessage `json:"execution_config"`
	SystemPrompt          *string         `json:"system_prompt"`
	InstructionPreamble   *string         `json:"instruction_preamble"`
	InstructionSkills     json.RawMessage `json:"instruction_skills"`
	AllowedTools          json.RawMessage `json:"allowed_tools"`
	SupportedModes        json.RawMessage `json:"supported_modes"`
	DefaultInvocationMode *string         `json:"default_invocation_mode"`
}

// AgentTriggerUsageSummary is the aggregated read model for "what triggers this agent".
type AgentTriggerUsageSummary struct {
	AgentID   string              `json:"agent_id"`
	AgentName string              `json:"agent_name"`
	Items     []AgentTriggerUsage `json:"items"`
}

// AgentTriggerUsage describes one inbound trigger or binding for an agent.
type AgentTriggerUsage struct {
	ID               string                         `json:"id"`
	Kind             string                         `json:"kind"`
	Title            string                         `json:"title"`
	Description      string                         `json:"description"`
	TriggerType      *string                        `json:"trigger_type,omitempty"`
	Enabled          bool                           `json:"enabled"`
	ReferenceID      *string                        `json:"reference_id,omitempty"`
	ReferenceType    *string                        `json:"reference_type,omitempty"`
	ManagePath       *string                        `json:"manage_path,omitempty"`
	ExecutionSearch  *TriggerExecutionSearchPreset  `json:"execution_search,omitempty"`
	LastTriggeredAt  *time.Time                     `json:"last_triggered_at,omitempty"`
	LastSuccessAt    *time.Time                     `json:"last_success_at,omitempty"`
	LastErrorAt      *time.Time                     `json:"last_error_at,omitempty"`
	LastError        *string                        `json:"last_error,omitempty"`
	RecentExecutions []AgentTriggerExecutionSummary `json:"recent_executions,omitempty"`
}

// AgentTriggerExecutionSummary is a compact execution record for one trigger binding.
type AgentTriggerExecutionSummary struct {
	ExecutionID   string     `json:"execution_id"`
	RunID         *string    `json:"run_id,omitempty"`
	Status        string     `json:"status"`
	TargetType    string     `json:"target_type"`
	TargetID      string     `json:"target_id"`
	FiredAt       time.Time  `json:"fired_at"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	ErrorMessage  *string    `json:"error_message,omitempty"`
	TriggerType   *string    `json:"trigger_type,omitempty"`
	ReferenceID   *string    `json:"reference_id,omitempty"`
	ReferenceType *string    `json:"reference_type,omitempty"`
}

// ApproveAgentRunRequest approves a pending run outcome.
type ApproveAgentRunRequest struct {
	Content     string `json:"content,omitempty"`
	SendMessage bool   `json:"send_message"`
}

const (
	AgentRunResumeIntentReply          = "reply"
	AgentRunResumeIntentApprove        = "approve"
	AgentRunResumeIntentRequestChanges = "request_changes"
	AgentRunResumeIntentAuthCompleted  = "auth_completed"
)

const (
	AgentRunStatusQueued    = "queued"
	AgentRunStatusRunning   = "running"
	AgentRunStatusPaused    = "paused"
	AgentRunStatusCompleted = "completed"
	AgentRunStatusFailed    = "failed"
	AgentRunStatusCancelled = "cancelled"
)

const (
	AgentRunPauseReasonNone           = "none"
	AgentRunPauseReasonHumanInput     = "human_input"
	AgentRunPauseReasonHumanApproval  = "human_approval"
	AgentRunPauseReasonAuthentication = "authentication"
)

const (
	AgentTriggerExecutionStatusQueued    = "queued"
	AgentTriggerExecutionStatusRunning   = "running"
	AgentTriggerExecutionStatusPaused    = "paused"
	AgentTriggerExecutionStatusCompleted = "completed"
	AgentTriggerExecutionStatusFailed    = "failed"
	AgentTriggerExecutionStatusCancelled = "cancelled"
	AgentTriggerExecutionStatusSkipped   = "skipped"
)

func NormalizeAgentRunPauseState(run *AgentRun) {
	if run == nil {
		return
	}

	status, pauseReason := NormalizeAgentRunStatus(run.Status, run.PauseReason, run.ApprovalState, run.ExecutionStage)
	run.Status = status
	run.PauseReason = pauseReason
}

func NormalizeAgentRunStatus(status string, pauseReason string, approvalState string, executionStage *string) (string, string) {
	reason := normalizeAgentRunPauseReason(status, pauseReason, approvalState, executionStage)
	switch strings.TrimSpace(status) {
	case AgentRunStatusPaused:
		return AgentRunStatusPaused, reason
	default:
		return strings.TrimSpace(status), AgentRunPauseReasonNone
	}
}

func IsAgentRunPausedStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case AgentRunStatusPaused:
		return true
	default:
		return false
	}
}

func IsAgentRunActiveStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case AgentRunStatusQueued, AgentRunStatusRunning, AgentRunStatusPaused:
		return true
	default:
		return false
	}
}

func normalizeAgentRunPauseReason(status string, pauseReason string, approvalState string, executionStage *string) string {
	switch strings.TrimSpace(pauseReason) {
	case AgentRunPauseReasonHumanInput, AgentRunPauseReasonHumanApproval, AgentRunPauseReasonAuthentication:
		return strings.TrimSpace(pauseReason)
	}
	if strings.TrimSpace(approvalState) == "pending" {
		return AgentRunPauseReasonHumanApproval
	}
	switch strings.TrimSpace(derefString(executionStage)) {
	case "awaiting_approval":
		return AgentRunPauseReasonHumanApproval
	case "awaiting_input":
		return AgentRunPauseReasonHumanInput
	case "awaiting_auth":
		return AgentRunPauseReasonAuthentication
	}
	if strings.TrimSpace(status) == AgentRunStatusPaused {
		return AgentRunPauseReasonHumanInput
	}
	return AgentRunPauseReasonNone
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// HandoffAgentRunRequest records an explicit handoff from a run.
type HandoffAgentRunRequest struct {
	ToAgentID    *string         `json:"to_agent_id"`
	ToUserID     *string         `json:"to_user_id"`
	HandoffState *string         `json:"handoff_state"`
	Reason       string          `json:"reason"`
	Context      json.RawMessage `json:"context"`
}

type StartAgentRunRequest struct {
	AgentID           string                 `json:"agent_id,omitempty"`
	AdditionalContext *string                `json:"additional_context,omitempty"`
	AllowedTools      []string               `json:"allowed_tools,omitempty"`
	BaseBranch        *string                `json:"base_branch,omitempty"`
	WorkingBranch     *string                `json:"working_branch,omitempty"`
	Output            *AgentRunOutputContext `json:"output,omitempty"`
}

type StartTargetAgentRunRequest struct {
	TargetType        string  `json:"target_type"`
	TargetID          string  `json:"target_id"`
	AgentID           string  `json:"agent_id"`
	AdditionalContext *string `json:"additional_context,omitempty"`
	BaseBranch        *string `json:"base_branch,omitempty"`
	WorkingBranch     *string `json:"working_branch,omitempty"`
}

type AgentRunTriggerContext struct {
	Source      string          `json:"source,omitempty"`
	TriggerType string          `json:"trigger_type,omitempty"`
	RuleID      *string         `json:"rule_id,omitempty"`
	FiredAt     *time.Time      `json:"fired_at,omitempty"`
	Context     json.RawMessage `json:"context,omitempty"`
}

type AgentRunTargetContext struct {
	TargetType string `json:"target_type,omitempty"`
	TargetID   string `json:"target_id,omitempty"`
}

type AgentRunGitHubReleaseEventContext struct {
	TagName         string     `json:"tag_name,omitempty"`
	TargetCommitish string     `json:"target_commitish,omitempty"`
	ReleaseName     string     `json:"release_name,omitempty"`
	ReleaseURL      string     `json:"release_url,omitempty"`
	PublishedAt     *time.Time `json:"published_at,omitempty"`
	IsPrerelease    bool       `json:"is_prerelease,omitempty"`
}

type AgentRunGitHubPullRequestEventContext struct {
	Number     int    `json:"number,omitempty"`
	BaseBranch string `json:"base_branch,omitempty"`
	HeadBranch string `json:"head_branch,omitempty"`
	URL        string `json:"url,omitempty"`
	State      string `json:"state,omitempty"`
	Title      string `json:"title,omitempty"`
}

type AgentRunGitHubCheckSuiteEventContext struct {
	Branch     string `json:"branch,omitempty"`
	Conclusion string `json:"conclusion,omitempty"`
	URL        string `json:"url,omitempty"`
}

type AgentRunGitHubEventContext struct {
	EventType    string                                 `json:"event_type,omitempty"`
	RepoFullName string                                 `json:"repo_full_name,omitempty"`
	RepositoryID string                                 `json:"repository_id,omitempty"`
	Release      *AgentRunGitHubReleaseEventContext     `json:"release,omitempty"`
	PullRequest  *AgentRunGitHubPullRequestEventContext `json:"pull_request,omitempty"`
	CheckSuite   *AgentRunGitHubCheckSuiteEventContext  `json:"check_suite,omitempty"`
}

type AgentRunEventContext struct {
	StateID *string                     `json:"state_id,omitempty"`
	TeamID  *string                     `json:"team_id,omitempty"`
	RunID   *string                     `json:"run_id,omitempty"`
	Reason  *string                     `json:"reason,omitempty"`
	GitHub  *AgentRunGitHubEventContext `json:"github,omitempty"`
}

type AgentRunOutputContext struct {
	Type           string  `json:"type,omitempty"`
	SpaceID        string  `json:"space_id,omitempty"`
	CollectionID   *string `json:"collection_id,omitempty"`
	IdempotencyKey string  `json:"idempotency_key,omitempty"`
}

// AgentRunInputPayload is the shared input contract for all agent runs.
// It preserves legacy top-level IDs and planning fields while adding
// explicit trigger/target/event metadata for generic launches.
type AgentRunInputPayload struct {
	Trigger             *AgentRunTriggerContext `json:"trigger,omitempty"`
	Target              *AgentRunTargetContext  `json:"target,omitempty"`
	Event               *AgentRunEventContext   `json:"event,omitempty"`
	Output              *AgentRunOutputContext  `json:"output,omitempty"`
	StoryID             string                  `json:"story_id,omitempty"`
	EpicID              string                  `json:"epic_id,omitempty"`
	ConversationID      string                  `json:"conversation_id,omitempty"`
	AdditionalContext   string                  `json:"additional_context,omitempty"`
	AllowedTools        []string                `json:"allowed_tools,omitempty"`
	Stage               string                  `json:"stage,omitempty"`
	PlanDocumentID      string                  `json:"plan_document_id,omitempty"`
	SpecDocumentID      string                  `json:"spec_document_id,omitempty"`
	SpecVersionID       string                  `json:"spec_version_id,omitempty"`
	PlanningMethodology string                  `json:"planning_methodology,omitempty"`
	FlowOutputKind      string                  `json:"flow_output_kind,omitempty"`
}

func (p *AgentRunInputPayload) SetTarget(targetType, targetID string) {
	if p == nil {
		return
	}
	targetType = strings.TrimSpace(targetType)
	targetID = strings.TrimSpace(targetID)
	if targetType == "" || targetID == "" {
		return
	}

	p.Target = &AgentRunTargetContext{
		TargetType: targetType,
		TargetID:   targetID,
	}

	switch targetType {
	case "task", "story":
		p.StoryID = targetID
	case "epic":
		p.EpicID = targetID
	case "support_conversation":
		p.ConversationID = targetID
	}
}

type SendAgentRunMessageRequest struct {
	Content string `json:"content"`
}

type ContinueAgentRunRequest struct {
	Content *string `json:"content,omitempty"`
}

type SendAgentRunRequestChangesRequest struct {
	Content string `json:"content"`
}

type ResumeAgentRunRequest struct {
	Intent          string          `json:"intent"`
	Content         string          `json:"content,omitempty"`
	SendMessage     bool            `json:"send_message,omitempty"`
	ResponsePayload json.RawMessage `json:"response_payload,omitempty"`
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
	ID                         *string  `json:"id,omitempty"`
	Key                        string   `json:"key"`
	FamilyKey                  string   `json:"family_key"`
	VersionKey                 string   `json:"version_key"`
	VersionLabel               string   `json:"version_label"`
	IsDefaultVersion           bool     `json:"is_default_version"`
	Scope                      string   `json:"scope"`
	WorkspaceID                *string  `json:"workspace_id,omitempty"`
	SourceVersionKey           *string  `json:"source_version_key,omitempty"`
	Provider                   *string  `json:"provider,omitempty"`
	Model                      *string  `json:"model,omitempty"`
	ExecutionConfig            JSONBlob `json:"execution_config,omitempty"`
	Label                      string   `json:"label"`
	Description                string   `json:"description"`
	DefaultRole                string   `json:"default_role"`
	RuntimeKind                string   `json:"runtime_kind"`
	DefaultTriggerMode         string   `json:"default_trigger_mode"`
	AllowedTriggerModes        []string `json:"allowed_trigger_modes"`
	AllowedTools               []string `json:"allowed_tools"`
	AllowedCommands            []string `json:"allowed_commands"`
	AllowedTargetTypes         []string `json:"allowed_target_types"`
	ApprovalMode               string   `json:"approval_mode"`
	DefaultInvocationMode      string   `json:"default_invocation_mode"`
	SupportedModes             []string `json:"supported_modes"`
	InstructionPreamble        string   `json:"instruction_preamble,omitempty"`
	InstructionSkills          []string `json:"instruction_skills,omitempty"`
	InstructionTemplateVersion string   `json:"instruction_template_version,omitempty"`
	SystemPrompt               *string  `json:"system_prompt,omitempty"`
}

type AgentModelProviderOption struct {
	Value                     string   `json:"value"`
	Label                     string   `json:"label"`
	ModelPlaceholder          string   `json:"model_placeholder"`
	SupportsReasoningEffort   bool     `json:"supports_reasoning_effort"`
	SupportedReasoningEfforts []string `json:"supported_reasoning_efforts,omitempty"`
	SupportsServiceTier       bool     `json:"supports_service_tier"`
	SupportedServiceTiers     []string `json:"supported_service_tiers,omitempty"`
}

type AgentExecutionConfig struct {
	ReasoningEffort *string `json:"reasoning_effort,omitempty"`
	ServiceTier     *string `json:"service_tier,omitempty"`
}

type AgentSkillRef struct {
	SkillID    *string  `json:"skill_id,omitempty"`
	Key        string   `json:"key,omitempty"`
	VersionKey *string  `json:"version_key,omitempty"`
	Config     JSONBlob `json:"config,omitempty"`
}

type AgentSkillRefs []AgentSkillRef

func (r AgentSkillRef) Normalize() AgentSkillRef {
	if r.SkillID != nil {
		value := strings.TrimSpace(*r.SkillID)
		if value == "" {
			r.SkillID = nil
		} else {
			r.SkillID = &value
		}
	}
	r.Key = strings.TrimSpace(r.Key)
	if r.VersionKey != nil {
		value := strings.TrimSpace(*r.VersionKey)
		if value == "" {
			r.VersionKey = nil
		} else {
			r.VersionKey = &value
		}
	}
	if len(r.Config) == 0 || bytes.Equal(bytes.TrimSpace(r.Config), []byte("null")) {
		r.Config = nil
	}
	return r
}

func (r AgentSkillRefs) Normalize() AgentSkillRefs {
	if len(r) == 0 {
		return AgentSkillRefs{}
	}
	out := make(AgentSkillRefs, 0, len(r))
	for _, ref := range r {
		out = append(out, ref.Normalize())
	}
	return out
}

func (r AgentSkillRefs) MarshalJSON() ([]byte, error) {
	return json.Marshal([]AgentSkillRef(r.Normalize()))
}

func (r *AgentSkillRefs) UnmarshalJSON(data []byte) error {
	if r == nil {
		return nil
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*r = AgentSkillRefs{}
		return nil
	}
	var refs []AgentSkillRef
	if err := json.Unmarshal(trimmed, &refs); err == nil {
		*r = AgentSkillRefs(refs).Normalize()
		return nil
	}
	var legacy []string
	if err := json.Unmarshal(trimmed, &legacy); err != nil {
		return err
	}
	out := make(AgentSkillRefs, 0, len(legacy))
	for _, key := range legacy {
		out = append(out, AgentSkillRef{Key: key}.Normalize())
	}
	*r = out
	return nil
}

func (r *AgentSkillRefs) Scan(value interface{}) error {
	if r == nil {
		return nil
	}
	if value == nil {
		*r = AgentSkillRefs{}
		return nil
	}
	var data []byte
	switch typed := value.(type) {
	case []byte:
		data = typed
	case string:
		data = []byte(typed)
	default:
		return fmt.Errorf("unsupported AgentSkillRefs scan type %T", value)
	}
	return r.UnmarshalJSON(data)
}

func (r AgentSkillRefs) Value() (driver.Value, error) {
	if len(r) == 0 {
		return "[]", nil
	}
	payload, err := json.Marshal(r.Normalize())
	if err != nil {
		return nil, err
	}
	return string(payload), nil
}

type JSONBlob []byte

func (j JSONBlob) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return []byte(j), nil
}

func (j *JSONBlob) UnmarshalJSON(data []byte) error {
	if j == nil {
		return nil
	}
	if len(data) == 0 {
		*j = nil
		return nil
	}
	*j = append((*j)[:0], data...)
	return nil
}

func (j *JSONBlob) Scan(value interface{}) error {
	if j == nil {
		return nil
	}
	switch typed := value.(type) {
	case nil:
		*j = nil
		return nil
	case []byte:
		*j = append((*j)[:0], typed...)
		return nil
	case string:
		*j = append((*j)[:0], typed...)
		return nil
	default:
		return fmt.Errorf("unsupported JSONBlob scan type %T", value)
	}
}

func (j JSONBlob) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "{}", nil
	}
	return string(j), nil
}

func (c AgentExecutionConfig) Normalize() AgentExecutionConfig {
	if c.ReasoningEffort != nil {
		value := strings.TrimSpace(*c.ReasoningEffort)
		if value == "" {
			c.ReasoningEffort = nil
		} else {
			c.ReasoningEffort = &value
		}
	}
	if c.ServiceTier != nil {
		value := strings.TrimSpace(*c.ServiceTier)
		if value == "" {
			c.ServiceTier = nil
		} else {
			c.ServiceTier = &value
		}
	}
	return c
}

func (c AgentExecutionConfig) IsZero() bool {
	normalized := c.Normalize()
	return normalized.ReasoningEffort == nil && normalized.ServiceTier == nil
}

func ParseAgentExecutionConfig(raw []byte) (AgentExecutionConfig, error) {
	if len(raw) == 0 {
		return AgentExecutionConfig{}, nil
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" || trimmed == "{}" {
		return AgentExecutionConfig{}, nil
	}
	var config AgentExecutionConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return AgentExecutionConfig{}, err
	}
	return config.Normalize(), nil
}

func MarshalAgentExecutionConfig(config AgentExecutionConfig) JSONBlob {
	config = config.Normalize()
	if config.IsZero() {
		return JSONBlob("{}")
	}
	payload, err := json.Marshal(config)
	if err != nil {
		return JSONBlob("{}")
	}
	return JSONBlob(payload)
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
type SkillCatalogEntry struct {
	ID                *string  `json:"id,omitempty"`
	Key               string   `json:"key"`
	VersionKey        string   `json:"version_key,omitempty"`
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	Instructions      string   `json:"instructions,omitempty"`
	SourceKind        string   `json:"source_kind"`
	SourceRuntime     *string  `json:"source_runtime,omitempty"`
	RequiredTools     []string `json:"required_tools,omitempty"`
	SupportedRuntimes []string `json:"supported_runtimes,omitempty"`
	Presets           []string `json:"presets,omitempty"`
}

// SkillCatalogResponse is the response for GET /automation/library/skills.
type SkillCatalogResponse struct {
	Skills []SkillCatalogEntry `json:"skills"`
}
