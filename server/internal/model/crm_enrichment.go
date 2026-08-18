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

// CRMEnrichmentFieldInput is one researched CRM field proposed by an agent.
type CRMEnrichmentFieldInput struct {
	Field      string      `json:"field"`
	Value      interface{} `json:"value"`
	SourceURL  string      `json:"source_url"`
	Evidence   string      `json:"evidence"`
	Confidence float64     `json:"confidence"`
}

// EnrichCRMContactRequest is the guarded agent-facing contact enrichment payload.
type EnrichCRMContactRequest struct {
	ContactID       string                    `json:"contact_id"`
	Fields          []CRMEnrichmentFieldInput `json:"fields"`
	EvidenceSummary string                    `json:"evidence_summary"`
	DryRun          bool                      `json:"dry_run"`
	ActorUserID     string                    `json:"-"`
}

// EnrichCRMCompanyRequest is the guarded agent-facing company enrichment payload.
type EnrichCRMCompanyRequest struct {
	CompanyID       string                    `json:"company_id"`
	Fields          []CRMEnrichmentFieldInput `json:"fields"`
	EvidenceSummary string                    `json:"evidence_summary"`
	DryRun          bool                      `json:"dry_run"`
	ActorUserID     string                    `json:"-"`
}

// ApplyCRMEnrichmentSuggestionRequest accepts one protected-value suggestion.
type ApplyCRMEnrichmentSuggestionRequest struct {
	Field       string `json:"field"`
	ActorUserID string `json:"-"`
}

// EnsureCRMContactCompanyRequest creates or reuses a company and links it to a contact.
type EnsureCRMContactCompanyRequest struct {
	ContactID        string  `json:"contact_id"`
	CompanyName      string  `json:"company_name"`
	Domain           *string `json:"domain,omitempty"`
	SourceURL        string  `json:"source_url"`
	Evidence         string  `json:"evidence"`
	Confidence       float64 `json:"confidence"`
	AssociationLabel *string `json:"association_label,omitempty"`
	DryRun           bool    `json:"dry_run"`
}

// EnsureCRMContactCompanyResult reports company creation/reuse and contact association.
type EnsureCRMContactCompanyResult struct {
	Status           string `json:"status"`
	ContactID        string `json:"contact_id"`
	CompanyID        string `json:"company_id,omitempty"`
	CompanyName      string `json:"company_name"`
	Domain           string `json:"domain,omitempty"`
	AssociationID    string `json:"association_id,omitempty"`
	AssociationLabel string `json:"association_label,omitempty"`
	CreatedCompany   bool   `json:"created_company"`
	CreatedLink      bool   `json:"created_link"`
	DryRun           bool   `json:"dry_run"`
}

// CRMEnrichmentFieldResult describes one applied or skipped field.
type CRMEnrichmentFieldResult struct {
	Field               string      `json:"field"`
	OldValue            interface{} `json:"old_value,omitempty"`
	NewValue            interface{} `json:"new_value,omitempty"`
	SourceURL           string      `json:"source_url,omitempty"`
	Confidence          float64     `json:"confidence,omitempty"`
	Reason              string      `json:"reason,omitempty"`
	CurrentValuePresent bool        `json:"current_value_present,omitempty"`
	ProposedValue       interface{} `json:"proposed_value,omitempty"`
}

// CRMEnrichmentApplyResult is returned by guarded contact/company enrichment.
type CRMEnrichmentApplyResult struct {
	Status             string                     `json:"status"`
	ObjectType         string                     `json:"object_type"`
	ObjectID           string                     `json:"object_id"`
	Applied            []CRMEnrichmentFieldResult `json:"applied"`
	Skipped            []CRMEnrichmentFieldResult `json:"skipped"`
	EnrichmentResultID string                     `json:"enrichment_result_id,omitempty"`
	DryRun             bool                       `json:"dry_run"`
}

// CRMEnrichmentListFilters applies filters when listing enrichments.
type CRMEnrichmentListFilters struct {
	ObjectType *string
	ObjectID   *string
	Source     *string
}
