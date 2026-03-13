package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

const (
	CRMEntitySummaryStatusPendingRefresh = "pending_refresh"
	CRMEntitySummaryStatusReady          = "ready"
	CRMEntitySummaryStatusStale          = "stale"
	CRMEntitySummaryStatusError          = "error"
)

const (
	CRMSummaryHighlightMomentum    = "momentum"
	CRMSummaryHighlightRisk        = "risk"
	CRMSummaryHighlightNextStep    = "next_step"
	CRMSummaryHighlightStakeholder = "stakeholder"
	CRMSummaryHighlightSignal      = "signal"
)

// CRMSummaryHighlight is a compact structured bullet rendered alongside the
// longer markdown summary.
type CRMSummaryHighlight struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// CRMSummaryHighlights persists a JSON array of summary highlights.
type CRMSummaryHighlights []CRMSummaryHighlight

func (h CRMSummaryHighlights) Value() (driver.Value, error) {
	if h == nil {
		return "[]", nil
	}
	b, err := json.Marshal(h)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (h *CRMSummaryHighlights) Scan(value interface{}) error {
	if value == nil {
		*h = CRMSummaryHighlights{}
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("failed to unmarshal CRMSummaryHighlights value: %v", value)
	}
	return json.Unmarshal(bytes, h)
}

// CRMEntitySummary is the durable system-owned summary artifact for one CRM entity.
type CRMEntitySummary struct {
	ID                string               `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID       string               `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_crm_entity_summaries_ws_entity,priority:1"`
	EntityType        string               `json:"entity_type" gorm:"not null;uniqueIndex:idx_crm_entity_summaries_ws_entity,priority:2"`
	EntityID          string               `json:"entity_id" gorm:"type:uuid;not null;uniqueIndex:idx_crm_entity_summaries_ws_entity,priority:3"`
	SummaryMarkdown   string               `json:"summary_markdown" gorm:"not null;default:''"`
	Highlights        CRMSummaryHighlights `json:"highlights" gorm:"type:jsonb;default:'[]'"`
	Status            string               `json:"status" gorm:"not null;default:'pending_refresh';index"`
	ComputedAt        *time.Time           `json:"computed_at"`
	SourceWindowStart *time.Time           `json:"source_window_start"`
	SourceWindowEnd   *time.Time           `json:"source_window_end"`
	LastTriggeredAt   *time.Time           `json:"last_triggered_at" gorm:"index"`
	LastError         *string              `json:"last_error"`
	Metadata          JSONB                `json:"metadata" gorm:"type:jsonb;default:'{}'"`
	CreatedAt         time.Time            `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time            `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMEntitySummary) TableName() string { return "crm_entity_summaries" }

// CRMEntitySummaryRefreshInput identifies the entity to recompute.
type CRMEntitySummaryRefreshInput struct {
	WorkspaceID string `json:"workspace_id"`
	EntityType  string `json:"entity_type"`
	EntityID    string `json:"entity_id"`
}

// CRMEntitySummaryRefreshResult reports the outcome of one summary generation run.
type CRMEntitySummaryRefreshResult struct {
	EntityType        string `json:"entity_type"`
	EntityID          string `json:"entity_id"`
	Status            string `json:"status"`
	NeedsContinue     bool   `json:"needs_continue"`
	Highlights        int    `json:"highlights"`
	SourceEmailCount  int    `json:"source_email_count"`
	SourceSignalCount int    `json:"source_signal_count"`
}

// CRMSummaryReconciliationResult reports the daily reconciliation enqueue counts.
type CRMSummaryReconciliationResult struct {
	DealsQueued    int `json:"deals_queued"`
	ContactsQueued int `json:"contacts_queued"`
}
