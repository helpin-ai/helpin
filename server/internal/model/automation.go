package model

import "time"

const (
	AutomationKindBuiltIn = "built_in_automation"
	AutomationKindRule    = "automation_rule"
)

const (
	AutomationHealthHealthy  = "healthy"
	AutomationHealthWarning  = "warning"
	AutomationHealthError    = "error"
	AutomationHealthInactive = "inactive"
	AutomationHealthUnknown  = "unknown"
)

const (
	AutomationScopeWorkspace = "workspace"
	AutomationScopeTeam      = "team"
	AutomationScopeAgent     = "agent"
)

// AutomationCatalogEntry is the canonical code-defined automation identity.
type AutomationCatalogEntry struct {
	ID                  string   `json:"id"`
	Kind                string   `json:"kind"`
	Module              string   `json:"module"`
	Group               string   `json:"group"`
	Title               string   `json:"title"`
	Description         string   `json:"description"`
	TargetTypes         []string `json:"target_types"`
	TriggerModes        []string `json:"trigger_modes"`
	ConfigScope         string   `json:"config_scope"`
	ExecutionStyle      string   `json:"execution_style"`
	UserGoverned        bool     `json:"user_governed"`
	UserCreatable       bool     `json:"user_creatable"`
	Queue               string   `json:"queue"`
	CurrentWriteSurface string   `json:"current_write_surface"`
	CurrentWritePath    *string  `json:"current_write_path,omitempty"`
	CurrentRunSurface   string   `json:"current_run_surface"`
	CurrentRunPath      *string  `json:"current_run_path,omitempty"`
	OutputSurface       string   `json:"output_surface"`
	DiagnosticsSurface  string   `json:"diagnostics_surface"`
}

// AutomationHealthSummary is the shared health read model consumed by settings UI.
type AutomationHealthSummary struct {
	Status           string     `json:"status"`
	LastSeenAt       *time.Time `json:"last_seen_at,omitempty"`
	LastSuccessAt    *time.Time `json:"last_success_at,omitempty"`
	LastErrorAt      *time.Time `json:"last_error_at,omitempty"`
	LastErrorMessage *string    `json:"last_error_message,omitempty"`
	Freshness        string     `json:"freshness"`
	Metrics          JSONB      `json:"metrics"`
}

// AutomationInventoryItem is one workspace-scoped automation instance assembled
// from the code catalog and module-specific adapters.
type AutomationInventoryItem struct {
	InventoryID         string                  `json:"inventory_id"`
	CatalogID           string                  `json:"catalog_id"`
	Kind                string                  `json:"kind"`
	Module              string                  `json:"module"`
	Group               string                  `json:"group"`
	Title               string                  `json:"title"`
	Description         string                  `json:"description"`
	ScopeType           string                  `json:"scope_type"`
	ScopeID             string                  `json:"scope_id"`
	ScopeLabel          string                  `json:"scope_label"`
	TargetTypes         []string                `json:"target_types"`
	TriggerModes        []string                `json:"trigger_modes"`
	ConfigScope         string                  `json:"config_scope"`
	ExecutionStyle      string                  `json:"execution_style"`
	UserGoverned        bool                    `json:"user_governed"`
	Enabled             bool                    `json:"enabled"`
	CurrentWriteSurface string                  `json:"current_write_surface"`
	CurrentWritePath    *string                 `json:"current_write_path,omitempty"`
	CurrentRunSurface   string                  `json:"current_run_surface"`
	CurrentRunPath      *string                 `json:"current_run_path,omitempty"`
	OutputSurface       string                  `json:"output_surface"`
	DiagnosticsSurface  string                  `json:"diagnostics_surface"`
	Health              AutomationHealthSummary `json:"health"`
}

// AutomationInventoryGroup describes one top-level read-only inventory section.
type AutomationInventoryGroup struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// AutomationInventoryResponse is the combined settings payload.
type AutomationInventoryResponse struct {
	Groups         []AutomationInventoryGroup      `json:"groups"`
	Items          []AutomationInventoryItem       `json:"items"`
	TriggerCatalog []AutomationTriggerCatalogEntry `json:"trigger_catalog"`
	GeneratedAt    time.Time                       `json:"generated_at"`
}

// TriggerExecutionSearchPreset is a normalized filter contract for linking to
// workspace trigger execution history without frontend-side trigger mapping.
type TriggerExecutionSearchPreset struct {
	AgentID     *string `json:"agent_id,omitempty"`
	BindingID   *string `json:"binding_id,omitempty"`
	TriggerType *string `json:"trigger_type,omitempty"`
	Source      *string `json:"source,omitempty"`
	ReferenceID *string `json:"reference_id,omitempty"`
	Status      *string `json:"status,omitempty"`
}

// WorkflowRuleSearchPreset is a normalized contract for linking into workflow
// rule authoring or filtered workflow rule views.
type WorkflowRuleSearchPreset struct {
	ShowTrigger         *string `json:"show_trigger,omitempty"`
	ShowTriggerTitle    *string `json:"show_trigger_title,omitempty"`
	Template            *string `json:"template,omitempty"`
	TemplateTitle       *string `json:"template_title,omitempty"`
	TemplateDescription *string `json:"template_description,omitempty"`
	CreateEventRule     bool    `json:"create_event_rule,omitempty"`
	TriggerType         *string `json:"trigger_type,omitempty"`
	AgentID             *string `json:"agent_id,omitempty"`
	RepoFullName        *string `json:"repo_full_name,omitempty"`
	Branch              *string `json:"branch,omitempty"`
	BaseBranch          *string `json:"base_branch,omitempty"`
	TagName             *string `json:"tag_name,omitempty"`
	Conclusion          *string `json:"conclusion,omitempty"`
	TargetMode          *string `json:"target_mode,omitempty"`
	TargetID            *string `json:"target_id,omitempty"`
}

// TriggerExecutionListFilters defines server-side filtering for workspace
// trigger execution history.
type TriggerExecutionListFilters struct {
	ExecutionID *string    `json:"execution_id,omitempty"`
	AgentID     *string    `json:"agent_id,omitempty"`
	BindingID   *string    `json:"binding_id,omitempty"`
	TriggerType *string    `json:"trigger_type,omitempty"`
	BindingKind *string    `json:"binding_kind,omitempty"`
	Status      *string    `json:"status,omitempty"`
	ReferenceID *string    `json:"reference_id,omitempty"`
	RunID       *string    `json:"run_id,omitempty"`
	FiredAfter  *time.Time `json:"fired_after,omitempty"`
	FiredBefore *time.Time `json:"fired_before,omitempty"`
}

// AutomationTriggerExecutionListItem is the workspace-level execution row used
// by the AI & Automations diagnostics surface.
type AutomationTriggerExecutionListItem struct {
	ExecutionID           string     `json:"execution_id"`
	AgentID               string     `json:"agent_id"`
	AgentName             string     `json:"agent_name"`
	ActorID               *string    `json:"actor_id,omitempty"`
	ActorName             *string    `json:"actor_name,omitempty"`
	BindingID             string     `json:"binding_id"`
	BindingKind           string     `json:"binding_kind"`
	BindingTitle          string     `json:"binding_title"`
	TriggerType           *string    `json:"trigger_type,omitempty"`
	TriggerTitle          *string    `json:"trigger_title,omitempty"`
	ReferenceID           *string    `json:"reference_id,omitempty"`
	ReferenceType         *string    `json:"reference_type,omitempty"`
	ReferenceTitle        *string    `json:"reference_title,omitempty"`
	ManagePath            *string    `json:"manage_path,omitempty"`
	TargetType            *string    `json:"target_type,omitempty"`
	TargetID              *string    `json:"target_id,omitempty"`
	TargetTitle           *string    `json:"target_title,omitempty"`
	TargetKey             *string    `json:"target_key,omitempty"`
	RunID                 *string    `json:"run_id,omitempty"`
	ConditionOutcome      *string    `json:"condition_outcome,omitempty"`
	ConditionAssessmentID *string    `json:"condition_assessment_id,omitempty"`
	Status                string     `json:"status"`
	ErrorMessage          *string    `json:"error_message,omitempty"`
	FiredAt               time.Time  `json:"fired_at"`
	StartedAt             *time.Time `json:"started_at,omitempty"`
	CompletedAt           *time.Time `json:"completed_at,omitempty"`
}

// AutomationTriggerExecutionListResponse is the paginated settings payload for
// workspace trigger execution history.
type AutomationTriggerExecutionListResponse struct {
	Data       []AutomationTriggerExecutionListItem `json:"data"`
	Total      int                                  `json:"total"`
	Page       int                                  `json:"page"`
	PerPage    int                                  `json:"per_page"`
	TotalPages int                                  `json:"total_pages"`
}

// AutomationTriggerCatalogEntry describes one canonical trigger surface in the product.
type AutomationTriggerCatalogEntry struct {
	ID                string                        `json:"id"`
	BindingKind       string                        `json:"binding_kind"`
	Category          string                        `json:"category"`
	TriggerType       string                        `json:"trigger_type"`
	Title             string                        `json:"title"`
	Description       string                        `json:"description"`
	SourceSurface     string                        `json:"source_surface"`
	ConfigSurface     *string                       `json:"config_surface,omitempty"`
	SupportsAgentRuns bool                          `json:"supports_agent_runs"`
	BindingCount      int                           `json:"binding_count"`
	ExecutionSearch   *TriggerExecutionSearchPreset `json:"execution_search,omitempty"`
	ShowRulesSearch   *WorkflowRuleSearchPreset     `json:"show_rules_search,omitempty"`
	CreateRuleSearch  *WorkflowRuleSearchPreset     `json:"create_rule_search,omitempty"`
}

// AutomationHealthSnapshot stores runtime health observations for built-in
// automations only. Inventory and config remain assembled elsewhere.
type AutomationHealthSnapshot struct {
	ID               string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string     `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_automation_health_scope,priority:1"`
	CatalogID        string     `json:"catalog_id" gorm:"not null;uniqueIndex:idx_automation_health_scope,priority:2"`
	ScopeType        string     `json:"scope_type" gorm:"not null;uniqueIndex:idx_automation_health_scope,priority:3"`
	ScopeID          string     `json:"scope_id" gorm:"not null;uniqueIndex:idx_automation_health_scope,priority:4"`
	Status           string     `json:"status" gorm:"not null;default:'unknown'"`
	LastSeenAt       *time.Time `json:"last_seen_at"`
	LastSuccessAt    *time.Time `json:"last_success_at"`
	LastErrorAt      *time.Time `json:"last_error_at"`
	LastErrorMessage *string    `json:"last_error_message"`
	Metrics          JSONB      `json:"metrics" gorm:"type:jsonb;default:'{}'"`
	CreatedAt        time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (AutomationHealthSnapshot) TableName() string { return "automation_health_snapshots" }
