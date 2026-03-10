package model

import "time"

// CRM buyer signal types.
const (
	CRMSignalBuyingIntent     = "buying_intent"
	CRMSignalObjection        = "objection"
	CRMSignalCompetitorMention = "competitor_mention"
	CRMSignalBudgetSignal     = "budget_signal"
	CRMSignalTimelineSignal   = "timeline_signal"
	CRMSignalChampionSignal   = "champion_signal"
	CRMSignalRiskSignal       = "risk_signal"
)

// CRM signal source types.
const (
	CRMSignalSourceEmail   = "email"
	CRMSignalSourceMeeting = "meeting"
	CRMSignalSourceNote    = "note"
	CRMSignalSourceManual  = "manual"
	CRMSignalSourceSupport = "support"
)

// CRMBuyerSignal represents a detected buyer signal in CRM interactions.
type CRMBuyerSignal struct {
	ID         string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string   `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ContactID  *string   `json:"contact_id" gorm:"type:uuid;index"`
	DealID     *string   `json:"deal_id" gorm:"type:uuid;index"`
	SignalType string    `json:"signal_type" gorm:"not null"` // buying_intent, objection, etc.
	SourceType string    `json:"source_type" gorm:"not null;default:'manual'"` // email, meeting, note, manual
	SourceID   *string   `json:"source_id" gorm:"type:uuid"`
	Summary    string    `json:"summary" gorm:"not null"`
	Confidence float64   `json:"confidence" gorm:"not null;default:0"`
	DetectedAt time.Time `json:"detected_at" gorm:"not null"`
	CreatedAt  time.Time `json:"created_at" gorm:"autoCreateTime"`
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
	WorkspaceID string   `json:"workspace_id"`
	ContactID   *string  `json:"contact_id"`
	DealID      *string  `json:"deal_id"`
	SignalType  string   `json:"signal_type"`
	SourceType  string   `json:"source_type"`
	SourceID    *string  `json:"source_id"`
	Summary     string   `json:"summary"`
	Confidence  *float64 `json:"confidence"`
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
	ContactID  *string
	DealID     *string
	SignalType *string
	SourceType *string
}
