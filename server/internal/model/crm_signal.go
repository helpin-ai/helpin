package model

import "time"

// CRM buyer signal types.
const (
	CRMSignalBuyingIntent      = "buying_intent"
	CRMSignalObjection         = "objection"
	CRMSignalCompetitorMention = "competitor_mention"
	CRMSignalBudgetSignal      = "budget_signal"
	CRMSignalTimelineSignal    = "timeline_signal"
	CRMSignalChampionSignal    = "champion_signal"
	CRMSignalRiskSignal        = "risk_signal"
)

// CRM signal detector kinds distinguish extraction from deterministic rules.
const (
	CRMSignalDetectorLLMExtracted = "llm_extracted"
	CRMSignalDetectorRuleDerived  = "rule_derived"
)

// CRM signal domains describe the independent evidence axis.
const (
	CRMSignalDomainConversation = "conversation"
	CRMSignalDomainWebBehavior  = "web_behavior"
	CRMSignalDomainProductUsage = "product_usage"
	CRMSignalDomainSupport      = "support"
	CRMSignalDomainDelivery     = "delivery"
	CRMSignalDomainRelationship = "relationship"
	CRMSignalDomainMarket       = "market"
)

// CRM signal polarities describe direction without changing the legacy taxonomy.
const (
	CRMSignalPolarityPositive = "positive"
	CRMSignalPolarityNegative = "negative"
	CRMSignalPolarityNeutral  = "neutral"
)

// CRM signal source types.
const (
	CRMSignalSourceEmail   = "email"
	CRMSignalSourceMeeting = "meeting"
	CRMSignalSourceNote    = "note"
	CRMSignalSourceCall    = "call"
	CRMSignalSourceManual  = "manual"
	CRMSignalSourceSupport = "support"
)

// CRMBuyerSignal represents a detected buyer signal in CRM interactions.
type CRMBuyerSignal struct {
	ID                     string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID            string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ContactID              *string    `json:"contact_id" gorm:"type:uuid;index"`
	DealID                 *string    `json:"deal_id" gorm:"type:uuid;index"`
	CompanyID              *string    `json:"company_id,omitempty" gorm:"type:uuid;index"`
	SignalType             string     `json:"signal_type" gorm:"not null"`                  // buying_intent, objection, etc.
	SourceType             string     `json:"source_type" gorm:"not null;default:'manual'"` // email, meeting, note, manual
	SourceID               *string    `json:"source_id" gorm:"type:uuid"`
	SourceThreadID         *string    `json:"source_thread_id" gorm:"type:uuid;index"`
	Summary                string     `json:"summary" gorm:"not null"`
	EvidenceExcerpt        *string    `json:"evidence_excerpt"`
	Metadata               JSONB      `json:"metadata" gorm:"type:jsonb;default:'{}'"`
	Confidence             float64    `json:"confidence" gorm:"not null;default:0"`
	DetectedAt             time.Time  `json:"detected_at" gorm:"not null"`
	DetectorKind           string     `json:"detector_kind" gorm:"not null;default:'llm_extracted';index"`
	SignalDomain           string     `json:"signal_domain" gorm:"not null;default:'conversation';index"`
	Polarity               string     `json:"polarity" gorm:"not null;default:'neutral';index"`
	RuleKey                *string    `json:"rule_key,omitempty" gorm:"index"`
	RuleVersion            *int       `json:"rule_version,omitempty"`
	WindowStartedAt        *time.Time `json:"window_started_at,omitempty" gorm:"type:timestamptz"`
	WindowEndedAt          *time.Time `json:"window_ended_at,omitempty" gorm:"type:timestamptz"`
	EvidenceIdentityMethod string     `json:"evidence_identity_method" gorm:"not null;default:'connected_mailbox';index"`
	EvidenceIdentityTrust  string     `json:"evidence_identity_trust" gorm:"not null;default:'verified';index"`
	EvidenceFingerprint    string     `json:"evidence_fingerprint,omitempty" gorm:"not null;default:'';index"`
	DismissedAt            *time.Time `json:"dismissed_at,omitempty" gorm:"index"`
	DismissedByMemberID    *string    `json:"dismissed_by_member_id,omitempty" gorm:"type:uuid"`
	CreatedAt              time.Time  `json:"created_at" gorm:"autoCreateTime"`
	ContactName            string     `json:"contact_name,omitempty" gorm:"-"`
	DealName               string     `json:"deal_name,omitempty" gorm:"-"`
	DealDisplayID          string     `json:"deal_display_id,omitempty" gorm:"-"`
}

func (CRMBuyerSignal) TableName() string { return "crm_buyer_signals" }

// CRMDealHealthScore represents a calculated health score for a deal.
type CRMDealHealthScore struct {
	ID           string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID  string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	DealID       string    `json:"deal_id" gorm:"type:uuid;not null;index"`
	Score        int       `json:"score" gorm:"not null;default:0"` // 0-100
	Factors      JSONB     `json:"factors" gorm:"type:jsonb;default:'{}'"`
	CalculatedAt time.Time `json:"calculated_at" gorm:"not null"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (CRMDealHealthScore) TableName() string { return "crm_deal_health_scores" }

// CreateCRMBuyerSignalRequest is the payload for creating a buyer signal.
type CreateCRMBuyerSignalRequest struct {
	WorkspaceID            string                 `json:"workspace_id"`
	ContactID              *string                `json:"contact_id"`
	DealID                 *string                `json:"deal_id"`
	CompanyID              *string                `json:"company_id"`
	SignalType             string                 `json:"signal_type"`
	SourceType             string                 `json:"source_type"`
	SourceID               *string                `json:"source_id"`
	SourceThreadID         *string                `json:"source_thread_id"`
	Summary                string                 `json:"summary"`
	EvidenceExcerpt        *string                `json:"evidence_excerpt"`
	Metadata               map[string]interface{} `json:"metadata"`
	Confidence             *float64               `json:"confidence"`
	DetectorKind           string                 `json:"detector_kind,omitempty"`
	SignalDomain           string                 `json:"signal_domain,omitempty"`
	Polarity               string                 `json:"polarity,omitempty"`
	RuleKey                *string                `json:"rule_key,omitempty"`
	RuleVersion            *int                   `json:"rule_version,omitempty"`
	WindowStartedAt        *time.Time             `json:"window_started_at,omitempty"`
	WindowEndedAt          *time.Time             `json:"window_ended_at,omitempty"`
	EvidenceIdentityMethod string                 `json:"evidence_identity_method,omitempty"`
	EvidenceIdentityTrust  string                 `json:"evidence_identity_trust,omitempty"`
}

// CreateCRMDealHealthScoreRequest is the payload for creating a deal health score.
type CreateCRMDealHealthScoreRequest struct {
	WorkspaceID string                 `json:"workspace_id"`
	DealID      string                 `json:"deal_id"`
	Score       int                    `json:"score"`
	Factors     map[string]interface{} `json:"factors"`
}

// CRMBuyerSignalListFilters applies filters when listing signals.
type CRMBuyerSignalListFilters struct {
	ContactID            *string
	DealID               *string
	CompanyID            *string
	SignalType           *string
	SourceType           *string
	IncludeDismissed     bool
	IncludeLowConfidence bool
}
