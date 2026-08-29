package model

import "time"

// CRMSignalScoringConfig stores an immutable composition model version.
type CRMSignalScoringConfig struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID *string   `json:"workspace_id,omitempty" gorm:"type:uuid;index"`
	Version     int       `json:"version" gorm:"not null"`
	Enabled     bool      `json:"enabled" gorm:"not null;default:true"`
	Heuristic   bool      `json:"heuristic" gorm:"not null;default:true"`
	Parameters  JSONB     `json:"parameters" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (CRMSignalScoringConfig) TableName() string { return "crm_signal_scoring_configs" }

// CRMSignalAccountStory groups corroborating signals attached to one account entity.
type CRMSignalAccountStory struct {
	ID                             string           `json:"id"`
	EntityType                     string           `json:"entity_type"`
	EntityID                       string           `json:"entity_id"`
	AccountName                    string           `json:"account_name"`
	AccountDomain                  string           `json:"account_domain,omitempty"`
	OwnerMemberID                  *string          `json:"owner_member_id,omitempty"`
	CommercialMotion               string           `json:"commercial_motion"`
	OtherActiveMotions             []string         `json:"other_active_motions,omitempty"`
	Priority                       float64          `json:"priority"`
	SignedImpact                   float64          `json:"signed_impact"`
	PositiveStrength               float64          `json:"positive_strength"`
	NegativeStrength               float64          `json:"negative_strength"`
	NeedsJudgment                  bool             `json:"needs_judgment"`
	RecommendedActionKey           *string          `json:"recommended_action_key,omitempty"`
	RecommendedActionLabel         *string          `json:"recommended_action_label,omitempty"`
	DirectionChangedBySupersession bool             `json:"direction_changed_by_supersession"`
	Severity                       string           `json:"severity"`
	Polarity                       string           `json:"polarity"`
	Domains                        []string         `json:"domains"`
	LatestDetectedAt               time.Time        `json:"latest_detected_at"`
	ChangedSince                   int              `json:"changed_since"`
	EvidenceSourceCount            int              `json:"evidence_source_count"`
	ChangedEvidenceSourceCount     int              `json:"changed_evidence_source_count"`
	ChangeSummary                  string           `json:"change_summary"`
	ScoreVersion                   int              `json:"score_version"`
	ScoreFactors                   JSONB            `json:"score_factors"`
	Signals                        []CRMSignal `json:"signals"`
}

// CRMSignalWorkspaceFeed is the ranked, explainable workspace read model.
type CRMSignalWorkspaceFeed struct {
	Data                []CRMSignalAccountStory `json:"data"`
	Lanes               []CRMSignalLane         `json:"lanes"`
	Total               int                     `json:"total"`
	Page                int                     `json:"page"`
	ScoreVersion        int                     `json:"score_version"`
	Heuristic           bool                    `json:"heuristic"`
	MinimumLanePriority float64                 `json:"minimum_lane_priority"`
	RolloutMode         string                  `json:"rollout_mode"`
}

// CRMSignalLane is one independently ranked and paginated commercial queue.
type CRMSignalLane struct {
	CommercialMotion string                  `json:"commercial_motion"`
	Data             []CRMSignalAccountStory `json:"data"`
	Total            int                     `json:"total"`
	Page             int                     `json:"page"`
	PerPage          int                     `json:"per_page"`
}

// CRMSignalShadowGate reports whether the Phase 1 shadow rollout may cut over.
type CRMSignalShadowGate struct {
	Eligible                   bool    `json:"eligible"`
	ObservationCount           int64   `json:"observation_count"`
	UnmappedObservationRate    float64 `json:"unmapped_observation_rate"`
	DuplicateFingerprintRate   float64 `json:"duplicate_fingerprint_rate"`
	ImmutableMeaningViolations int64   `json:"immutable_meaning_violations"`
}
