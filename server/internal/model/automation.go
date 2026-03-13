package model

import "time"

const (
	AutomationKindBuiltIn    = "built_in_automation"
	AutomationKindContextual = "contextual_agent"
	AutomationKindCustom     = "custom_automation"
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
	Groups      []AutomationInventoryGroup `json:"groups"`
	Items       []AutomationInventoryItem  `json:"items"`
	GeneratedAt time.Time                  `json:"generated_at"`
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
