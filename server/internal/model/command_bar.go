package model

import (
	"encoding/json"
	"time"
)

const (
	CommandBarParseStatusPlan            = "plan"
	CommandBarParseStatusNoMatchingAgent = "no_matching_agent"

	CommandBarPlanKindKnownAgent     = "known_agent"
	CommandBarPlanKindOneShotCommand = "one_shot_command"
	CommandBarPlanKindFanOut         = "fan_out"
	CommandBarPlanKindTaskPipeline   = "task_pipeline_fan_out"
	CommandBarPlanKindDAG            = "dag"

	CommandBarStepTypeAgent                = ""
	CommandBarStepTypeEnsureEpicBranch     = "ensure_epic_branch"
	CommandBarStepTypeMergeTaskToEpic      = "merge_task_to_epic"
	CommandBarStepTypeResolveMergeConflict = "resolve_merge_conflict"
	CommandBarStepTypeOpenEpicPullRequest  = "open_epic_pr"

	CommandBarPlanStatusRunning   = "running"
	CommandBarPlanStatusCompleted = "completed"
	CommandBarPlanStatusFailed    = "failed"
	CommandBarPlanStatusCancelled = "cancelled"

	CommandBarThreadStatusOpen     = "open"
	CommandBarThreadStatusArchived = "archived"

	CommandBarMessageRoleUser      = "user"
	CommandBarMessageRoleAssistant = "assistant"

	CommandBarProposalInlineAnswer      = "inline_answer"
	CommandBarProposalRunPlan           = "run_plan"
	CommandBarProposalCreateAgent       = "create_agent"
	CommandBarProposalCreateAgentAndRun = "create_agent_and_run"
	CommandBarProposalClarification     = "clarification"
	CommandBarProposalNoMatch           = "no_match"
)

type CommandBarPageContext struct {
	EntityType   string                 `json:"entity_type"`
	EntityID     string                 `json:"entity_id"`
	DisplayTitle string                 `json:"display_title"`
	RelatedIDs   map[string][]string    `json:"related_ids,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}
type CommandBarPlanStep struct {
	AgentID              string                `json:"agent_id"`
	AgentKey             string                `json:"agent_key,omitempty"`
	AgentName            string                `json:"agent_name"`
	PlanKind             string                `json:"plan_kind,omitempty"`
	StepType             string                `json:"step_type,omitempty"`
	Target               CommandBarPageContext `json:"target"`
	Instructions         string                `json:"instructions"`
	AllowedTools         []string              `json:"allowed_tools,omitempty"`
	DependsOnStepIndexes []int                 `json:"depends_on_step_indexes,omitempty"`
}

type CommandBarPlan struct {
	ID             string                `json:"id,omitempty"`
	PlanKind       string                `json:"plan_kind,omitempty"`
	Steps          []CommandBarPlanStep  `json:"steps"`
	RunCount       int                   `json:"run_count"`
	EstimatedRuns  int                   `json:"estimated_runs,omitempty"`
	MaxAllowedRuns int                   `json:"max_allowed_runs,omitempty"`
	Guardrails     []CommandBarGuardrail `json:"guardrails,omitempty"`
}

type CommandBarGuardrail struct {
	Type     string `json:"type"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}


type CommandBarDispatchRequest struct {
	Text        string                `json:"text"`
	PageContext CommandBarPageContext `json:"page_context"`
	Steps       []CommandBarPlanStep  `json:"steps"`
}

type CommandBarDispatchResponse struct {
	PlanID   string               `json:"plan_id,omitempty"`
	Steps    []CommandBarPlanStep `json:"steps,omitempty"`
	RunCount int                  `json:"run_count,omitempty"`
	Runs     []AgentRun           `json:"runs"`
}

type ConfirmCommandBarChatProposalRequest struct {
	Name           *string  `json:"name,omitempty"`
	Description    *string  `json:"description,omitempty"`
	AllowedTools   []string `json:"allowed_tools,omitempty"`
	AllowedTargets []string `json:"allowed_targets,omitempty"`
}


type CommandBarPlanRecord struct {
	ID          string  `json:"id" gorm:"type:uuid;primaryKey"`
	WorkspaceID string  `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ActorID     *string `json:"actor_id" gorm:"type:uuid;index"`
	// ParentChatRunID links a plan launched from a dock chat back to the
	// chat's backing run; when the plan settles, its result is delivered
	// into that chat (ParentNotifiedAt records delivery).
	ParentChatRunID *string `json:"parent_chat_run_id,omitempty" gorm:"type:uuid;index"`
	DockChatID      *string `json:"dock_chat_id,omitempty" gorm:"type:uuid;index"`
	// SupportConversationID links a plan launched from a support chat run to
	// its conversation (mutually exclusive with DockChatID).
	SupportConversationID *string    `json:"support_conversation_id,omitempty" gorm:"type:uuid;index"`
	ParentNotifiedAt      *time.Time `json:"parent_notified_at,omitempty"`
	Status           string          `json:"status" gorm:"not null;default:'running';index"`
	Prompt           string          `json:"prompt" gorm:"not null"`
	PageContext      json.RawMessage `json:"page_context" gorm:"type:jsonb;not null;default:'{}'"`
	Steps            json.RawMessage `json:"steps" gorm:"type:jsonb;not null;default:'[]'"`
	RunIDsByStep     json.RawMessage `json:"run_ids_by_step" gorm:"type:jsonb;not null;default:'{}'"`
	CurrentStepIndex int             `json:"current_step_index" gorm:"not null;default:0"`
	RunCount         int             `json:"run_count" gorm:"not null;default:0"`
	ErrorMessage     *string         `json:"error_message"`
	CancelledAt      *time.Time      `json:"cancelled_at"`
	CompletedAt      *time.Time      `json:"completed_at"`
	CreatedAt        time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CommandBarPlanRecord) TableName() string { return "command_bar_plans" }

// CommandBarPlanDismissal records that a user has dismissed a plan from their
// command runs rail. Dismissals are per-user so engineers in the same workspace
// can independently keep or clear runs from their own view.
type CommandBarPlanDismissal struct {
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;primaryKey"`
	PlanID      string    `json:"plan_id" gorm:"type:uuid;primaryKey"`
	UserID      string    `json:"user_id" gorm:"type:uuid;primaryKey"`
	DismissedAt time.Time `json:"dismissed_at" gorm:"autoCreateTime"`
}

// TableName names the underlying table for CommandBarPlanDismissal.
func (CommandBarPlanDismissal) TableName() string { return "command_bar_plan_dismissals" }

// DismissCommandBarPlansRequest dismisses one or more plans from the actor's rail.
type DismissCommandBarPlansRequest struct {
	PlanIDs []string `json:"plan_ids"`
}

type CommandBarPlanSummary struct {
	ID               string                `json:"id"`
	Status           string                `json:"status"`
	PlanKind         string                `json:"plan_kind,omitempty"`
	Prompt           string                `json:"prompt"`
	PageContext      CommandBarPageContext `json:"page_context"`
	Steps            []CommandBarPlanStep  `json:"steps"`
	RunIDsByStep     map[int]string        `json:"run_ids_by_step"`
	CurrentStepIndex int                   `json:"current_step_index"`
	RunCount         int                   `json:"run_count"`
	ErrorMessage     *string               `json:"error_message,omitempty"`
	CancelledAt      *time.Time            `json:"cancelled_at,omitempty"`
	CompletedAt      *time.Time            `json:"completed_at,omitempty"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
	Runs             []AgentRun            `json:"runs,omitempty"`
}

type CommandBarPlanListResponse struct {
	Plans []CommandBarPlanSummary `json:"plans"`
}

type CommandBarPlanDetailResponse struct {
	Plan CommandBarPlanSummary `json:"plan"`
}

type CommandBarCancelPlanResponse struct {
	Plan CommandBarPlanSummary `json:"plan"`
	Runs []AgentRun            `json:"runs,omitempty"`
}

type CommandBarResumePlanResponse struct {
	Plan CommandBarPlanSummary `json:"plan"`
	Run  *AgentRun             `json:"run,omitempty"`
	Runs []AgentRun            `json:"runs,omitempty"`
}

type CommandBarRetryPlanRequest struct {
	StepIndex int `json:"step_index"`
}

type CommandBarRetryPlanResponse struct {
	Plan CommandBarPlanSummary `json:"plan"`
	Run  *AgentRun             `json:"run,omitempty"`
	Runs []AgentRun            `json:"runs,omitempty"`
}

type PromoteCommandBarRunRequest struct {
	Name           string   `json:"name"`
	Description    *string  `json:"description,omitempty"`
	AllowedTools   []string `json:"allowed_tools,omitempty"`
	AllowedTargets []string `json:"allowed_targets,omitempty"`
}

type PromoteCommandBarRunResponse struct {
	Agent Agent `json:"agent"`
}

type CommandBarAgent struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`
	PresetKey      string   `json:"preset_key,omitempty"`
	Role           string   `json:"role,omitempty"`
	Status         string   `json:"status,omitempty"`
	RuntimeKind    string   `json:"runtime_kind,omitempty"`
	IsSystem       bool     `json:"is_system,omitempty"`
	SupportedModes []string `json:"supported_modes,omitempty"`
	AllowedTargets []string `json:"allowed_targets"`
	AllowedTools   []string `json:"allowed_tools"`
}

type CommandBarToolCatalogEntry struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Description    string      `json:"description"`
	Category       string      `json:"category"`
	InputSchema    interface{} `json:"input_schema,omitempty"`
	Allowed        bool        `json:"allowed"`
	Selected       bool        `json:"selected"`
	DisabledReason string      `json:"disabled_reason,omitempty"`
}

type CommandBarToolCatalogResponse struct {
	AgentID        string                       `json:"agent_id"`
	AllowedTools   []string                     `json:"allowed_tools"`
	SelectedTools  []string                     `json:"selected_tools"`
	Tools          []CommandBarToolCatalogEntry `json:"tools"`
	Categories     []string                     `json:"categories"`
	Validation     []string                     `json:"validation,omitempty"`
	AllowedTargets []string                     `json:"allowed_targets,omitempty"`
}
