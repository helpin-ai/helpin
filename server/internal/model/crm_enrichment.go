package model

import "time"

// CRM enrichment sources.
const (
	CRMEnrichmentSourceApollo = "apollo"
	CRMEnrichmentSourceAI     = "ai"
	CRMEnrichmentSourceManual = "manual"
)

// CRMEnrichmentResult stores enrichment data for CRM objects.
type CRMEnrichmentResult struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ObjectType  string    `json:"object_type" gorm:"not null"` // contact, company, deal
	ObjectID    string    `json:"object_id" gorm:"type:uuid;not null;index"`
	Source      string    `json:"source" gorm:"not null;default:'manual'"` // apollo, ai, manual
	Data        JSONB     `json:"data" gorm:"type:jsonb;default:'{}'"`
	Confidence  float64   `json:"confidence" gorm:"not null;default:0"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (CRMEnrichmentResult) TableName() string { return "crm_enrichment_results" }

// CreateCRMEnrichmentRequest is the payload for creating an enrichment result.
type CreateCRMEnrichmentRequest struct {
	WorkspaceID string                 `json:"workspace_id"`
	ObjectType  string                 `json:"object_type"`
	ObjectID    string                 `json:"object_id"`
	Source      string                 `json:"source"`
	Data        map[string]interface{} `json:"data"`
	Confidence  *float64               `json:"confidence"`
}

// CRMEnrichmentListFilters applies filters when listing enrichments.
type CRMEnrichmentListFilters struct {
	ObjectType *string
	ObjectID   *string
	Source     *string
}
