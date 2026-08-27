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
	ID               string           `json:"id"`
	EntityType       string           `json:"entity_type"`
	EntityID         string           `json:"entity_id"`
	AccountName      string           `json:"account_name"`
	AccountDomain    string           `json:"account_domain,omitempty"`
	OwnerMemberID    *string          `json:"owner_member_id,omitempty"`
	Priority         float64          `json:"priority"`
	SignedImpact     float64          `json:"signed_impact"`
	Severity         string           `json:"severity"`
	Polarity         string           `json:"polarity"`
	Domains          []string         `json:"domains"`
	LatestDetectedAt time.Time        `json:"latest_detected_at"`
	ChangedSince     int              `json:"changed_since"`
	ChangeSummary    string           `json:"change_summary"`
	ScoreVersion     int              `json:"score_version"`
	ScoreFactors     JSONB            `json:"score_factors"`
	Signals          []CRMBuyerSignal `json:"signals"`
}

// CRMSignalWorkspaceFeed is the ranked, explainable workspace read model.
type CRMSignalWorkspaceFeed struct {
	Data         []CRMSignalAccountStory `json:"data"`
	Total        int                     `json:"total"`
	Page         int                     `json:"page"`
	ScoreVersion int                     `json:"score_version"`
	Heuristic    bool                    `json:"heuristic"`
}
