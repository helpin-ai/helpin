package model

import "time"

const (
	AgentTemplateRuntimeKindNativeSDK = "native_sdk"
	AgentTemplateTypeReleaseNotes     = "release_notes_writer"
	AgentTemplateTypeCompetitiveIntel = "competitive_intelligence_digest"
)

type AgentTemplate struct {
	ID                    string         `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID           *string        `json:"workspace_id,omitempty" gorm:"type:uuid;index"`
	Key                   string         `json:"key" gorm:"not null"`
	Name                  string         `json:"name" gorm:"not null"`
	Description           *string        `json:"description,omitempty"`
	RuntimeKind           string         `json:"runtime_kind" gorm:"not null;default:'native_sdk'"`
	DefaultRole           string         `json:"default_role"`
	ExecutionConfig       JSONBlob       `json:"execution_config" gorm:"type:jsonb;not null;default:'{}'"`
	SystemPrompt          *string        `json:"system_prompt,omitempty"`
	PlanningNotes         *string        `json:"planning_notes,omitempty"`
	Skills                AgentSkillRefs `json:"skills" gorm:"type:jsonb;not null;default:'[]'"`
	AllowedTools          JSONBlob       `json:"allowed_tools" gorm:"type:jsonb;not null;default:'[]'"`
	AllowedCommands       JSONBlob       `json:"allowed_commands" gorm:"type:jsonb;not null;default:'[]'"`
	AllowedTargets        JSONBlob       `json:"allowed_targets" gorm:"type:jsonb;not null;default:'[]'"`
	RequiredContext       JSONBlob       `json:"required_context" gorm:"type:jsonb;not null;default:'[]'"`
	StarterFlows          JSONBlob       `json:"starter_flows" gorm:"type:jsonb;not null;default:'[]'"`
	ApprovalMode          string         `json:"approval_mode" gorm:"not null;default:'preset_default'"`
	DefaultInvocationMode string         `json:"default_invocation_mode" gorm:"not null;default:'autonomous'"`
	MonthlyTokenBudget    *int           `json:"monthly_token_budget,omitempty"`
	IsEnabled             bool           `json:"is_enabled" gorm:"not null;default:true"`
	CreatedBy             *string        `json:"created_by,omitempty" gorm:"type:uuid"`
	UpdatedBy             *string        `json:"updated_by,omitempty" gorm:"type:uuid"`
	DeletedAt             *time.Time     `json:"deleted_at,omitempty" gorm:"index"`
	CreatedAt             time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt             time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
}

func (AgentTemplate) TableName() string { return "agent_templates" }

type CreateAgentFromTemplateOverrides struct {
	Role                  *string         `json:"role,omitempty"`
	RuntimeKind           *string         `json:"runtime_kind,omitempty"`
	Skills                *AgentSkillRefs `json:"skills,omitempty"`
	Provider              *string         `json:"provider,omitempty"`
	Model                 *string         `json:"model,omitempty"`
	MonthlyTokenBudget    *int            `json:"monthly_token_budget,omitempty"`
	ExecutionConfig       JSONBlob        `json:"execution_config,omitempty"`
	SystemPrompt          *string         `json:"system_prompt,omitempty"`
	PlanningNotes         *string         `json:"planning_notes,omitempty"`
	AllowedTools          JSONBlob        `json:"allowed_tools,omitempty"`
	AllowedCommands       JSONBlob        `json:"allowed_commands,omitempty"`
	AllowedTargets        JSONBlob        `json:"allowed_targets,omitempty"`
	ApprovalMode          *string         `json:"approval_mode,omitempty"`
	MaxConcurrentRuns     *int            `json:"max_concurrent_runs,omitempty"`
	DefaultInvocationMode *string         `json:"default_invocation_mode,omitempty"`
}

type CreateAgentFromTemplateFlow struct {
	FlowKey           string   `json:"flow_key,omitempty"`
	FlowInput         JSONBlob `json:"flow_input,omitempty"`
	RepositoryID      string   `json:"repository_id,omitempty"`
	RepoFullName      string   `json:"repo_full_name,omitempty"`
	ReleaseKinds      []string `json:"release_kinds,omitempty"`
	IncludePrerelease bool     `json:"include_prerelease,omitempty"`
	TagPattern        string   `json:"tag_pattern,omitempty"`
	SpaceID           string   `json:"space_id,omitempty"`
	CollectionID      *string  `json:"collection_id,omitempty"`
}

type CreateAgentFromTemplateRequest struct {
	Name       *string                           `json:"name,omitempty"`
	TeamID     *string                           `json:"team_id,omitempty"`
	Overrides  *CreateAgentFromTemplateOverrides `json:"overrides,omitempty"`
	CreateFlow bool                              `json:"create_flow,omitempty"`
	Flow       *CreateAgentFromTemplateFlow      `json:"flow,omitempty"`
}

type CreateAgentFromTemplateResponse struct {
	Agent *Agent          `json:"agent"`
	Flow  *AutomationRule `json:"flow,omitempty"`
}
