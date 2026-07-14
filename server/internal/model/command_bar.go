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

type CommandBarParseRequest struct {
	Text        string                `json:"text"`
	PageContext CommandBarPageContext `json:"page_context"`
}

type CommandBarChatTurnRequest struct {
	ThreadID    *string               `json:"thread_id,omitempty"`
	Text        string                `json:"text"`
	PageContext CommandBarPageContext `json:"page_context"`
	// ClientTurnID correlates websocket progress events with the in-flight
	// turn on the client that sent it. Optional; no progress is published
	// without it.
	ClientTurnID string `json:"client_turn_id,omitempty"`
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
	PlanID   string               `json:"plan_id,omitempty"`
	Steps    []CommandBarPlanStep `json:"steps,omitempty"`
	RunCount int                  `json:"run_count,omitempty"`
	Runs     []AgentRun           `json:"runs"`
}

type CommandBarThread struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ActorID     *string   `json:"actor_id" gorm:"type:uuid;index"`
	Title       string    `json:"title" gorm:"not null"`
	Status      string    `json:"status" gorm:"not null;default:'open';index"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CommandBarThread) TableName() string { return "command_bar_threads" }

type CommandBarMessage struct {
	ID           string          `json:"id" gorm:"type:uuid;primaryKey"`
	ThreadID     string          `json:"thread_id" gorm:"type:uuid;not null;index"`
	WorkspaceID  string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ActorID      *string         `json:"actor_id,omitempty" gorm:"type:uuid;index"`
	Role         string          `json:"role" gorm:"not null"`
	Content      string          `json:"content" gorm:"not null"`
	PageContext  json.RawMessage `json:"page_context,omitempty" gorm:"type:jsonb"`
	ProposalJSON json.RawMessage `json:"proposal_json,omitempty" gorm:"type:jsonb"`
	CreatedAt    time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

func (CommandBarMessage) TableName() string { return "command_bar_messages" }

type CommandBarProposal struct {
	Type            string                   `json:"type"`
	Answer          string                   `json:"answer,omitempty"`
	Context         json.RawMessage          `json:"context,omitempty"`
	Plan            *CommandBarPlan          `json:"plan,omitempty"`
	Draft           *CustomAgentDraft        `json:"draft,omitempty"`
	RunTarget       *CommandBarPageContext   `json:"run_target,omitempty"`
	RunInstructions string                   `json:"run_instructions,omitempty"`
	Reasons         []CustomAgentDraftReason `json:"reasons,omitempty"`
	Warnings        []string                 `json:"warnings,omitempty"`
	Reason          string                   `json:"reason,omitempty"`
	Suggestions     []string                 `json:"suggestions,omitempty"`
	Guardrails      []CommandBarGuardrail    `json:"guardrails,omitempty"`
	CreatedAgentID  string                   `json:"created_agent_id,omitempty"`
	CreatedRunID    string                   `json:"created_run_id,omitempty"`
}

type CommandBarThreadSummary struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	ActorID     *string   `json:"actor_id,omitempty"`
	Title       string    `json:"title"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CommandBarMessageSummary struct {
	ID          string                `json:"id"`
	ThreadID    string                `json:"thread_id"`
	Role        string                `json:"role"`
	Content     string                `json:"content"`
	PageContext CommandBarPageContext `json:"page_context,omitempty"`
	Proposal    *CommandBarProposal   `json:"proposal,omitempty"`
	CreatedAt   time.Time             `json:"created_at"`
}

type CommandBarChatTurnResponse struct {
	Thread           CommandBarThreadSummary  `json:"thread"`
	UserMessage      CommandBarMessageSummary `json:"user_message"`
	AssistantMessage CommandBarMessageSummary `json:"assistant_message"`
	Proposal         *CommandBarProposal      `json:"proposal,omitempty"`
}

type CommandBarThreadDetail struct {
	Thread   CommandBarThreadSummary    `json:"thread"`
	Messages []CommandBarMessageSummary `json:"messages"`
}

type ListCommandBarChatThreadsResponse struct {
	Threads []CommandBarThreadDetail `json:"threads"`
}

type ConfirmCommandBarChatProposalRequest struct {
	Name           *string  `json:"name,omitempty"`
	Description    *string  `json:"description,omitempty"`
	AllowedTools   []string `json:"allowed_tools,omitempty"`
	AllowedTargets []string `json:"allowed_targets,omitempty"`
}

type ConfirmCommandBarChatCreateAgentResponse struct {
	Agent Agent     `json:"agent"`
	Run   *AgentRun `json:"run,omitempty"`
}

type CommandBarPlanRecord struct {
	ID               string          `json:"id" gorm:"type:uuid;primaryKey"`
	WorkspaceID      string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ActorID          *string         `json:"actor_id" gorm:"type:uuid;index"`
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

type CommandBarUnmetIntent struct {
	ID              string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ActorID         *string         `json:"actor_id" gorm:"type:uuid;index"`
	Prompt          string          `json:"prompt" gorm:"not null"`
	PageContext     json.RawMessage `json:"page_context" gorm:"type:jsonb;not null;default:'{}'"`
	CandidateAgents json.RawMessage `json:"candidate_agents" gorm:"type:jsonb;not null;default:'[]'"`
	Reason          string          `json:"reason" gorm:"not null"`
	Status          string          `json:"status" gorm:"not null;default:'open';index"`
	ReviewNotes     *string         `json:"review_notes"`
	ReviewedAt      *time.Time      `json:"reviewed_at"`
	CreatedAt       time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

func (CommandBarUnmetIntent) TableName() string { return "command_bar_unmet_intents" }

type CommandBarUnmetIntentListResponse struct {
	Intents []CommandBarUnmetIntentSummary `json:"intents"`
}

type CommandBarUnmetIntentSummary struct {
	ID              string                `json:"id"`
	WorkspaceID     string                `json:"workspace_id"`
	ActorID         *string               `json:"actor_id,omitempty"`
	Prompt          string                `json:"prompt,omitempty"`
	PromptPreview   string                `json:"prompt_preview"`
	PromptRedacted  bool                  `json:"prompt_redacted"`
	PageContext     CommandBarPageContext `json:"page_context"`
	CandidateAgents []CommandBarAgent     `json:"candidate_agents"`
	Reason          string                `json:"reason"`
	Status          string                `json:"status"`
	ReviewNotes     *string               `json:"review_notes,omitempty"`
	ReviewedAt      *time.Time            `json:"reviewed_at,omitempty"`
	CreatedAt       time.Time             `json:"created_at"`
}

type ReviewCommandBarUnmetIntentRequest struct {
	Status string  `json:"status"`
	Notes  *string `json:"notes,omitempty"`
}
