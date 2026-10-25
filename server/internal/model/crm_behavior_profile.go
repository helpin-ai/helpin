package model

import "time"

// CRM behavior profile match methods, recording how product events were tied
// to a CRM record. Email matches are weaker than identity links: anyone can
// type an address into an unverified identify call.
const (
	CRMBehaviorMatchIdentityLink = "identity_link"
	CRMBehaviorMatchRecordID     = "record_id"
	CRMBehaviorMatchExternalID   = "external_id"
	CRMBehaviorMatchEmail        = "email"
)

// CRMBehaviorProfile is product-usage data for one CRM contact or company,
// derived from helpin.events by CRMBehaviorProfileSynchronizer. Rows are
// replaced wholesale on every sync, so they never outlive the evidence.
type CRMBehaviorProfile struct {
	EntityType  string    `json:"entity_type" gorm:"primaryKey"`
	EntityID    string    `json:"entity_id" gorm:"type:uuid;primaryKey"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	FirstSeenAt time.Time `json:"first_seen_at" gorm:"type:timestamptz;not null"`
	LastSeenAt  time.Time `json:"last_seen_at" gorm:"type:timestamptz;not null"`
	Sessions30d int       `json:"sessions_30d" gorm:"column:sessions_30d;not null;default:0"`
	// MatchMethods is a comma-separated, sorted list of CRMBehaviorMatch* values.
	MatchMethods string    `json:"match_methods" gorm:"not null;default:''"`
	ComputedAt   time.Time `json:"computed_at" gorm:"type:timestamptz;not null"`
}

func (CRMBehaviorProfile) TableName() string { return "crm_behavior_profiles" }

// CRMBehaviorSummary is the behavior data shown on a CRM record.
type CRMBehaviorSummary struct {
	FirstSeenAt  time.Time `json:"first_seen_at"`
	LastSeenAt   time.Time `json:"last_seen_at"`
	Sessions30d  int       `json:"sessions_30d"`
	MatchMethods []string  `json:"match_methods"`
	ComputedAt   time.Time `json:"computed_at"`
}

// CRMBehaviorCoverage reports how many records of one type have behavior data.
type CRMBehaviorCoverage struct {
	RecordsWithBehavior int64      `json:"records_with_behavior"`
	TotalRecords        int64      `json:"total_records"`
	LastComputedAt      *time.Time `json:"last_computed_at,omitempty"`
}
