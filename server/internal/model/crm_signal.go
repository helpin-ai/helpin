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
	CRMSignalSourceEmail    = "email"
	CRMSignalSourceMeeting  = "meeting"
	CRMSignalSourceNote     = "note"
	CRMSignalSourceCall     = "call"
	CRMSignalSourceManual   = "manual"
	CRMSignalSourceSupport  = "support"
	CRMSignalSourceCRM      = "crm"
	CRMSignalSourcePM       = "pm"
	CRMSignalSourceWeb      = "web_behavior"
	CRMSignalSourceProduct  = "product_usage"
	CRMSignalSourceExternal = "external"
)

// CRM signal evaluator cadences.
const (
	CRMSignalRuleCadenceDaily      = "daily"
	CRMSignalRuleCadenceMicroBatch = "micro_batch"
)

// CRM signal rule keys are stable identities independent of signal taxonomy.
const (
	CRMSignalRuleSupportVolumeSpike       = "support_volume_spike"
	CRMSignalRuleUrgentIssueOpenDeal      = "urgent_issue_open_deal"
	CRMSignalRuleSupportAIEscalation      = "support_ai_escalation"
	CRMSignalRuleSupportCSATDeterioration = "support_csat_deterioration"
	CRMSignalRuleRequestedFeatureShipped  = "requested_feature_shipped"
	CRMSignalRuleDealStageStalled         = "deal_stage_stalled"
	CRMSignalRuleDealGoneDark             = "deal_gone_dark"
	CRMSignalRuleChampionQuiet            = "champion_quiet"
	CRMSignalRuleTimelineFollowupLapsed   = "timeline_followup_lapsed"
	CRMSignalRuleDealSingleThreaded       = "deal_single_threaded"
	CRMSignalRuleRenewalApproaching       = "renewal_approaching"
	CRMSignalRuleBuyingCommitteeExpanded  = "buying_committee_expanded"
	CRMSignalRuleBuyingCommitteeShrank    = "buying_committee_shrank"

	CRMSignalRuleRepeatedPricingActivity = "repeated_pricing_activity"
	CRMSignalRuleProcurementPageActivity = "procurement_page_activity"
	CRMSignalRuleKnownContactReturned    = "known_contact_returned"
	CRMSignalRuleHighIntentProductEvent  = "high_intent_product_event"
	CRMSignalRuleSessionDepthSpike       = "session_depth_spike"
	CRMSignalRuleNewAccountStakeholder   = "new_account_stakeholder"
	CRMSignalRuleAnonymousAccountTraffic = "anonymous_account_traffic"
	CRMSignalRuleCampaignReturn          = "campaign_attributed_return"
	CRMSignalRulePreIdentification       = "pre_identification_history"
	CRMSignalRuleConfiguredForm          = "configured_form_submission"
	CRMSignalRuleIdentifiedArticleView   = "identified_article_view"
	CRMSignalRuleVersionedInteraction    = "versioned_interaction"
	CRMSignalRuleExternalEvidence        = "external_provider_evidence"
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
	DismissalReason        *string    `json:"dismissal_reason,omitempty" gorm:"index"`
	ReviewedAt             *time.Time `json:"reviewed_at,omitempty" gorm:"index"`
	ActedAt                *time.Time `json:"acted_at,omitempty" gorm:"index"`
	CreatedAt              time.Time  `json:"created_at" gorm:"autoCreateTime"`
	ContactName            string     `json:"contact_name,omitempty" gorm:"-"`
	DealName               string     `json:"deal_name,omitempty" gorm:"-"`
	DealDisplayID          string     `json:"deal_display_id,omitempty" gorm:"-"`
	AccountName            string     `json:"account_name,omitempty" gorm:"-"`
	AccountDomain          string     `json:"account_domain,omitempty" gorm:"-"`
	OwnerMemberID          *string    `json:"owner_member_id,omitempty" gorm:"-"`
	DealAmount             *float64   `json:"deal_amount,omitempty" gorm:"-"`
	DealStageProbability   *int       `json:"deal_stage_probability,omitempty" gorm:"-"`
	BusinessPriority       float64    `json:"business_priority" gorm:"-"`
	SignedImpact           float64    `json:"signed_impact" gorm:"-"`
	Severity               string     `json:"severity" gorm:"-"`
	ScoreVersion           int        `json:"score_version" gorm:"-"`
	ScoreFactors           JSONB      `json:"score_factors,omitempty" gorm:"-"`
	ActivationEligible     bool       `json:"activation_eligible" gorm:"-"`
	ActivationBlockers     []string   `json:"activation_blockers,omitempty" gorm:"-"`
	ExistingOpenTaskID     *string    `json:"existing_open_task_id,omitempty" gorm:"-"`
}

func (CRMBuyerSignal) TableName() string { return "crm_buyer_signals" }

// CRMSignalRuleConfig stores one immutable detector version and its thresholds.
// A newer version is inserted rather than mutating detector semantics in place.
type CRMSignalRuleConfig struct {
	ID                 string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID        *string   `json:"workspace_id,omitempty" gorm:"type:uuid;index"`
	RuleKey            string    `json:"rule_key" gorm:"not null;index"`
	Version            int       `json:"version" gorm:"not null"`
	Cadence            string    `json:"cadence" gorm:"not null;index"`
	Enabled            bool      `json:"enabled" gorm:"not null;default:true"`
	ShadowMode         bool      `json:"shadow_mode" gorm:"not null;default:true"`
	ActivationEligible bool      `json:"activation_eligible" gorm:"not null;default:false"`
	Thresholds         JSONB     `json:"thresholds" gorm:"type:jsonb;default:'{}'"`
	BusinessWeight     float64   `json:"business_weight" gorm:"not null;default:10"`
	HalfLifeDays       float64   `json:"half_life_days" gorm:"not null;default:30"`
	CreatedAt          time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMSignalRuleConfig) TableName() string { return "crm_signal_rule_configs" }

// CRMSignalEvaluationRun records evaluator coverage and shadow output.
type CRMSignalEvaluationRun struct {
	ID              string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Cadence         string     `json:"cadence" gorm:"not null;index"`
	RuleKey         string     `json:"rule_key" gorm:"not null;index"`
	RuleVersion     int        `json:"rule_version" gorm:"not null"`
	WindowStartedAt time.Time  `json:"window_started_at" gorm:"type:timestamptz;not null"`
	WindowEndedAt   time.Time  `json:"window_ended_at" gorm:"type:timestamptz;not null"`
	Status          string     `json:"status" gorm:"not null"`
	CandidateCount  int        `json:"candidate_count" gorm:"not null;default:0"`
	InsertedCount   int        `json:"inserted_count" gorm:"not null;default:0"`
	ErrorMessage    *string    `json:"error_message,omitempty"`
	StartedAt       time.Time  `json:"started_at" gorm:"type:timestamptz;not null"`
	CompletedAt     *time.Time `json:"completed_at,omitempty" gorm:"type:timestamptz"`
	CreatedAt       time.Time  `json:"created_at" gorm:"autoCreateTime"`
}

func (CRMSignalEvaluationRun) TableName() string { return "crm_signal_evaluation_runs" }

// CRMSignalEvaluatorWatermark coordinates one global evaluator cadence.
type CRMSignalEvaluatorWatermark struct {
	Cadence       string     `json:"cadence" gorm:"primaryKey"`
	Watermark     time.Time  `json:"watermark" gorm:"type:timestamptz;not null"`
	LeaseOwner    *string    `json:"lease_owner,omitempty"`
	LeaseUntil    *time.Time `json:"lease_until,omitempty" gorm:"type:timestamptz"`
	LastStartedAt *time.Time `json:"last_started_at,omitempty" gorm:"type:timestamptz"`
	UpdatedAt     time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMSignalEvaluatorWatermark) TableName() string { return "crm_signal_evaluator_watermarks" }

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
	ContactID             *string
	DealID                *string
	CompanyID             *string
	SignalType            *string
	SourceType            *string
	IncludeDismissed      bool
	IncludeLowConfidence  bool
	OwnerMemberID         *string
	SignalDomain          *string
	Polarity              *string
	EvidenceIdentityTrust *string
	Status                *string
	Severity              *string
	MaxAgeDays            *int
}
