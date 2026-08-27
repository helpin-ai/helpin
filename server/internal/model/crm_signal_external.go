package model

import "time"

const (
	CRMExternalEvidenceFunding          = "funding"
	CRMExternalEvidenceHiring           = "hiring"
	CRMExternalEvidenceJobChange        = "job_change"
	CRMExternalEvidenceTechnology       = "technology"
	CRMExternalEvidenceLeadership       = "leadership"
	CRMExternalEvidenceThirdPartyIntent = "third_party_intent"
)

// CRMSignalExternalEvidence is the provider-neutral, immutable ingress record.
type CRMSignalExternalEvidence struct {
	ID                 string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID        string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Provider           string    `json:"provider" gorm:"not null;index"`
	ProviderEvidenceID string    `json:"provider_evidence_id" gorm:"not null"`
	EvidenceType       string    `json:"evidence_type" gorm:"not null;index"`
	RuleKey            string    `json:"rule_key" gorm:"not null;index"`
	RuleVersion        int       `json:"rule_version" gorm:"not null"`
	SignalType         string    `json:"signal_type" gorm:"not null"`
	SignalDomain       string    `json:"signal_domain" gorm:"not null;index"`
	Polarity           string    `json:"polarity" gorm:"not null;index"`
	Summary            string    `json:"summary" gorm:"not null"`
	EvidenceExcerpt    *string   `json:"evidence_excerpt,omitempty"`
	SourceURL          *string   `json:"source_url,omitempty"`
	ContactID          *string   `json:"contact_id,omitempty" gorm:"type:uuid;index"`
	DealID             *string   `json:"deal_id,omitempty" gorm:"type:uuid;index"`
	CompanyID          *string   `json:"company_id,omitempty" gorm:"type:uuid;index"`
	IdentityMethod     string    `json:"identity_method" gorm:"not null;index"`
	IdentityTrust      string    `json:"identity_trust" gorm:"not null;index"`
	Provenance         JSONB     `json:"provenance" gorm:"type:jsonb;not null;default:'{}'"`
	ObservedAt         time.Time `json:"observed_at" gorm:"not null;index"`
	SignalID           *string   `json:"signal_id,omitempty" gorm:"type:uuid;index"`
	CreatedAt          time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (CRMSignalExternalEvidence) TableName() string { return "crm_signal_external_evidence" }

type IngestCRMSignalExternalEvidenceRequest struct {
	WorkspaceID        string                 `json:"workspace_id"`
	Provider           string                 `json:"provider"`
	ProviderEvidenceID string                 `json:"provider_evidence_id"`
	EvidenceType       string                 `json:"evidence_type"`
	RuleKey            string                 `json:"rule_key"`
	RuleVersion        int                    `json:"rule_version"`
	SignalType         string                 `json:"signal_type"`
	SignalDomain       string                 `json:"signal_domain"`
	Polarity           string                 `json:"polarity"`
	Summary            string                 `json:"summary"`
	EvidenceExcerpt    *string                `json:"evidence_excerpt"`
	SourceURL          *string                `json:"source_url"`
	ContactID          *string                `json:"contact_id"`
	DealID             *string                `json:"deal_id"`
	CompanyID          *string                `json:"company_id"`
	IdentityMethod     string                 `json:"identity_method"`
	IdentityTrust      string                 `json:"identity_trust"`
	Provenance         map[string]interface{} `json:"provenance"`
	ObservedAt         time.Time              `json:"observed_at"`
}
