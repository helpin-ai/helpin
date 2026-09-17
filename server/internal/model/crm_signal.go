package model

import "time"

// CRM CRM signal types.
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
	CRMSignalDetectorManual       = "manual"
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

	CRMCommercialMotionProspecting = "prospecting"
	CRMCommercialMotionConversion  = "conversion"
	CRMCommercialMotionOnboarding  = "onboarding"
	CRMCommercialMotionAdoption    = "adoption"
	CRMCommercialMotionExpansion   = "expansion"
	CRMCommercialMotionRenewal     = "renewal"
	CRMCommercialMotionRetention   = "retention"
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
	CRMSignalRuleCadenceDaily       = "daily"
	CRMSignalRuleCadenceMicroBatch  = "micro_batch"
	CRMSignalRuleCadenceEventDriven = "event_driven"
)

// CRM signal rule keys are stable identities independent of signal taxonomy.
const (
	CRMSignalRuleConversationExtraction   = "conversation_signal_extraction"
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

	CRMSignalRuleUsageDecline         = "account_usage_decline"
	CRMSignalRuleActivationStalled    = "activation_stalled"
	CRMSignalRuleWorkflowFailureSpike = "workflow_failure_spike"
	CRMSignalRuleCapacitySaturation   = "capacity_saturation"
	CRMSignalRulePaymentFailed        = "payment_failed"
	CRMSignalRuleDowngradeRequested   = "downgrade_requested"
)

// CRMSignal represents a detected CRM signal in CRM interactions.
type CRMSignal struct {
	ID                             string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID                    string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ContactID                      *string    `json:"contact_id" gorm:"type:uuid;index"`
	DealID                         *string    `json:"deal_id" gorm:"type:uuid;index"`
	CompanyID                      *string    `json:"company_id,omitempty" gorm:"type:uuid;index"`
	SignalType                     string     `json:"signal_type" gorm:"not null"`                  // buying_intent, objection, etc.
	SourceType                     string     `json:"source_type" gorm:"not null;default:'manual'"` // email, meeting, note, manual
	SourceID                       *string    `json:"source_id" gorm:"type:uuid"`
	SourceThreadID                 *string    `json:"source_thread_id" gorm:"type:uuid;index"`
	Summary                        string     `json:"summary" gorm:"not null"`
	EvidenceExcerpt                *string    `json:"evidence_excerpt"`
	Metadata                       JSONB      `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	Confidence                     float64    `json:"confidence" gorm:"not null;default:0"`
	DetectedAt                     time.Time  `json:"detected_at" gorm:"not null"`
	DetectorKind                   string     `json:"detector_kind" gorm:"not null;default:'llm_extracted';index"`
	SignalDomain                   string     `json:"signal_domain" gorm:"not null;default:'conversation';index"`
	Polarity                       string     `json:"polarity" gorm:"not null;default:'neutral';index"`
	RuleKey                        *string    `json:"rule_key,omitempty" gorm:"index"`
	RuleVersion                    *int       `json:"rule_version,omitempty"`
	WindowStartedAt                *time.Time `json:"window_started_at,omitempty" gorm:"type:timestamptz"`
	WindowEndedAt                  *time.Time `json:"window_ended_at,omitempty" gorm:"type:timestamptz"`
	EvidenceIdentityMethod         string     `json:"evidence_identity_method" gorm:"not null;default:'manual_entry';index"`
	EvidenceIdentityTrust          string     `json:"evidence_identity_trust" gorm:"not null;default:'untrusted';index"`
	EvidenceFingerprint            string     `json:"evidence_fingerprint,omitempty" gorm:"not null;default:'';index"`
	ObservationID                  *string    `json:"observation_id,omitempty" gorm:"type:uuid;index"`
	CommercialMotion               string     `json:"commercial_motion" gorm:"not null;default:'conversion';index"`
	InterpretationVersion          int        `json:"interpretation_version" gorm:"not null;default:1"`
	BusinessWeightSnapshot         float64    `json:"business_weight_snapshot" gorm:"not null;default:0"`
	HalfLifeDaysSnapshot           float64    `json:"half_life_days_snapshot" gorm:"not null;default:0"`
	InterpretationSnapshot         JSONB      `json:"interpretation_snapshot" gorm:"type:jsonb;not null;default:'{}'"`
	MeaningFingerprint             string     `json:"meaning_fingerprint" gorm:"not null;default:'';index"`
	RecommendedActionKey           *string    `json:"recommended_action_key,omitempty"`
	RecommendedActionLabel         *string    `json:"recommended_action_label,omitempty"`
	ReplayCalibrationExcluded      bool       `json:"replay_calibration_excluded" gorm:"not null;default:false;index"`
	SupersededAt                   *time.Time `json:"superseded_at,omitempty" gorm:"index"`
	SupersededReason               *string    `json:"superseded_reason,omitempty" gorm:"index"`
	DirectionChangedBySupersession bool       `json:"direction_changed_by_supersession" gorm:"not null;default:false"`
	DismissedAt                    *time.Time `json:"dismissed_at,omitempty" gorm:"index"`
	DismissedByMemberID            *string    `json:"dismissed_by_member_id,omitempty" gorm:"type:uuid"`
	DismissalReason                *string    `json:"dismissal_reason,omitempty" gorm:"index"`
	ReviewedAt                     *time.Time `json:"reviewed_at,omitempty" gorm:"index"`
	ActedAt                        *time.Time `json:"acted_at,omitempty" gorm:"index"`
	CreatedAt                      time.Time  `json:"created_at" gorm:"autoCreateTime"`
	ContactName                    string     `json:"contact_name,omitempty" gorm:"-"`
	DealName                       string     `json:"deal_name,omitempty" gorm:"-"`
	DealDisplayID                  string     `json:"deal_display_id,omitempty" gorm:"-"`
	AccountName                    string     `json:"account_name,omitempty" gorm:"-"`
	AccountDomain                  string     `json:"account_domain,omitempty" gorm:"-"`
	OwnerMemberID                  *string    `json:"owner_member_id,omitempty" gorm:"-"`
	DealAmount                     *float64   `json:"deal_amount,omitempty" gorm:"-"`
	DealStageProbability           *int       `json:"deal_stage_probability,omitempty" gorm:"-"`
	BusinessPriority               float64    `json:"business_priority" gorm:"-"`
	SignedImpact                   float64    `json:"signed_impact" gorm:"-"`
	Severity                       string     `json:"severity" gorm:"-"`
	ScoreVersion                   int        `json:"score_version" gorm:"-"`
	ScoreFactors                   JSONB      `json:"score_factors,omitempty" gorm:"-"`
	ActivationEligible             bool       `json:"activation_eligible" gorm:"-"`
	ActivationBlockers             []string   `json:"activation_blockers,omitempty" gorm:"-"`
	ExistingOpenTaskID             *string    `json:"existing_open_task_id,omitempty" gorm:"-"`
}

func (CRMSignal) TableName() string { return "crm_signals" }

// CRMSignalObservation stores source evidence before commercial interpretation.
type CRMSignalObservation struct {
	ID                        string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID               string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ContactID                 *string    `json:"contact_id,omitempty" gorm:"type:uuid;index"`
	DealID                    *string    `json:"deal_id,omitempty" gorm:"type:uuid;index"`
	CompanyID                 *string    `json:"company_id,omitempty" gorm:"type:uuid;index"`
	RuleKey                   string     `json:"rule_key" gorm:"not null;index"`
	RuleVersion               int        `json:"rule_version" gorm:"not null"`
	DetectorKind              string     `json:"detector_kind" gorm:"not null;index"`
	SignalDomain              string     `json:"signal_domain" gorm:"not null;index"`
	SourceType                string     `json:"source_type" gorm:"not null"`
	SourceID                  *string    `json:"source_id,omitempty"`
	SourceThreadID            *string    `json:"source_thread_id,omitempty"`
	Summary                   string     `json:"summary" gorm:"not null"`
	EvidenceExcerpt           *string    `json:"evidence_excerpt,omitempty"`
	Metadata                  JSONB      `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	Confidence                float64    `json:"confidence" gorm:"not null"`
	ObservedAt                time.Time  `json:"observed_at" gorm:"not null;index"`
	WindowStartedAt           *time.Time `json:"window_started_at,omitempty"`
	WindowEndedAt             *time.Time `json:"window_ended_at,omitempty"`
	IdentityMethod            string     `json:"identity_method" gorm:"not null;index"`
	IdentityTrust             string     `json:"identity_trust" gorm:"not null;index"`
	EvidenceFingerprint       string     `json:"evidence_fingerprint" gorm:"not null;index"`
	MotionsAtDetection        JSONB      `json:"motions_at_detection" gorm:"type:jsonb;not null;default:'{}'"`
	ContextSnapshot           JSONB      `json:"context_snapshot" gorm:"type:jsonb;not null;default:'{}'"`
	ReplayCalibrationExcluded bool       `json:"replay_calibration_excluded" gorm:"not null;default:false"`
	CreatedAt                 time.Time  `json:"created_at" gorm:"autoCreateTime"`
}

func (CRMSignalObservation) TableName() string { return "crm_signal_observations" }

// CRMSignalInterpretationConfig versions commercial meaning independently of detection.
type CRMSignalInterpretationConfig struct {
	ID                     string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID            *string   `json:"workspace_id,omitempty" gorm:"type:uuid;index"`
	RuleKey                string    `json:"rule_key" gorm:"not null;index"`
	RuleVersion            int       `json:"rule_version" gorm:"not null"`
	Motion                 string    `json:"motion" gorm:"not null;index"`
	ObservationSignalType  string    `json:"observation_signal_type" gorm:"not null;default:'*'"`
	Version                int       `json:"version" gorm:"not null"`
	SignalType             string    `json:"signal_type" gorm:"not null"`
	Polarity               string    `json:"polarity" gorm:"not null"`
	BusinessWeight         float64   `json:"business_weight" gorm:"not null"`
	HalfLifeDays           float64   `json:"half_life_days" gorm:"not null"`
	RecommendedActionKey   *string   `json:"recommended_action_key,omitempty"`
	RecommendedActionLabel *string   `json:"recommended_action_label,omitempty"`
	Enabled                bool      `json:"enabled" gorm:"not null;default:true"`
	CreatedAt              time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (CRMSignalInterpretationConfig) TableName() string {
	return "crm_signal_interpretation_configs"
}

// CRMSignalMotionState is one append-only resolver applicability snapshot.
type CRMSignalMotionState struct {
	ID              string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string    `json:"workspace_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_crm_signal_motion_state_history,priority:1"`
	EntityType      string    `json:"entity_type" gorm:"not null;uniqueIndex:idx_crm_signal_motion_state_history,priority:2"`
	EntityID        string    `json:"entity_id" gorm:"type:uuid;not null;uniqueIndex:idx_crm_signal_motion_state_history,priority:3"`
	ResolverVersion int       `json:"resolver_version" gorm:"not null;uniqueIndex:idx_crm_signal_motion_state_history,priority:4"`
	Motions         JSONB     `json:"motions" gorm:"type:jsonb;not null;default:'{}'"`
	InputSnapshot   JSONB     `json:"input_snapshot" gorm:"type:jsonb;not null;default:'{}'"`
	EffectiveAt     time.Time `json:"effective_at" gorm:"not null;uniqueIndex:idx_crm_signal_motion_state_history,priority:5"`
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (CRMSignalMotionState) TableName() string { return "crm_signal_motion_states" }

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

// CreateCRMSignalRequest is the payload for creating a CRM signal.
type CreateCRMSignalRequest struct {
	WorkspaceID     string                 `json:"workspace_id"`
	ContactID       *string                `json:"contact_id"`
	DealID          *string                `json:"deal_id"`
	CompanyID       *string                `json:"company_id"`
	SignalType      string                 `json:"signal_type"`
	Summary         string                 `json:"summary"`
	EvidenceExcerpt *string                `json:"evidence_excerpt"`
	Metadata        map[string]interface{} `json:"metadata"`
	Confidence      *float64               `json:"confidence"`
}

// CreateCRMDealHealthScoreRequest is the payload for creating a deal health score.
type CreateCRMDealHealthScoreRequest struct {
	WorkspaceID string                 `json:"workspace_id"`
	DealID      string                 `json:"deal_id"`
	Score       int                    `json:"score"`
	Factors     map[string]interface{} `json:"factors"`
}

// CRMSignalListFilters applies filters when listing signals.
type CRMSignalListFilters struct {
	ContactID             *string
	DealID                *string
	CompanyID             *string
	SignalType            *string
	SourceType            *string
	IncludeDismissed      bool
	IncludeLowConfidence  bool
	CommercialOnly        bool
	IncludeContext        bool
	OwnerMemberID         *string
	SignalDomain          *string
	Polarity              *string
	EvidenceIdentityTrust *string
	Status                *string
	Severity              *string
	CommercialMotion      *string
	MaxAgeDays            *int
	Query                 *QueryFilterGroup
}
