package model

import (
	"encoding/json"
	"time"
)

const (
	CommandBarParseStatusPlan            = "plan"
	CommandBarParseStatusNoMatchingAgent = "no_matching_agent"

	CommandBarPlanStatusRunning   = "running"
	CommandBarPlanStatusCompleted = "completed"
	CommandBarPlanStatusFailed    = "failed"
	CommandBarPlanStatusCancelled = "cancelled"
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
	AllowedTools []string              `json:"allowed_tools,omitempty"`
}

type CommandBarPlan struct {
	ID             string                `json:"id,omitempty"`
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

type CommandBarPlanSummary struct {
	ID               string                `json:"id"`
	Status           string                `json:"status"`
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

type CommandBarRetryPlanRequest struct {
	StepIndex int `json:"step_index"`
}

type CommandBarRetryPlanResponse struct {
	Plan CommandBarPlanSummary `json:"plan"`
	Run  AgentRun              `json:"run"`
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
