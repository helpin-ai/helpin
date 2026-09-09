package model

import "time"

// CRM suggestion types.
const (
	CRMSuggestionFollowUp    = "follow_up"
	CRMSuggestionDealCreate  = "deal_create"
	CRMSuggestionDealAdvance = "deal_advance"
	CRMSuggestionEnrichment  = "enrichment"
	CRMSuggestionRiskAlert   = "risk_alert"
)

const (
	CRMSuggestionExecutionPending   = "pending"
	CRMSuggestionExecutionSucceeded = "succeeded"
	CRMSuggestionExecutionFailed    = "failed"
	// CRMSuggestionExecutionInProgress means a claimed executor has not confirmed its result.
	CRMSuggestionExecutionInProgress = "in_progress"
	// CRMSuggestionExecutionManualRequired records a decision without claiming an automated action.
	CRMSuggestionExecutionManualRequired = "manual_required"
)

// CRM suggestion statuses.
const (
	CRMSuggestionStatusPending   = "pending"
	CRMSuggestionStatusAccepted  = "accepted"
	CRMSuggestionStatusDismissed = "dismissed"
	// Superseded and expired preserve an unconsumed decision without fabricating rejection.
	CRMSuggestionStatusSuperseded = "superseded"
	CRMSuggestionStatusExpired    = "expired"
)

// CRMSuggestion represents an AI-generated suggestion for CRM actions.
type CRMSuggestion struct {
	ID                string      `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID       string      `json:"workspace_id" gorm:"type:uuid;not null;index"`
	UserID            *string     `json:"user_id" gorm:"type:uuid;index"`
	SuggestionType    string      `json:"suggestion_type" gorm:"not null"` // follow_up, deal_create, etc.
	ObjectType        *string     `json:"object_type"`                     // contact, company, deal
	ObjectID          *string     `json:"object_id" gorm:"type:uuid"`
	Title             string      `json:"title" gorm:"not null"`
	Description       *string     `json:"description"`
	Context           JSONB       `json:"context" gorm:"type:jsonb;default:'{}'"`
	SignalIDs         StringArray `json:"signal_ids" gorm:"type:text[];not null;default:'{}'"`
	Signals           []CRMSignal `json:"signals,omitempty" gorm:"-"`
	Status            string      `json:"status" gorm:"not null;default:'pending'"` // pending, accepted, dismissed
	DismissalReason   *string     `json:"dismissal_reason,omitempty" gorm:"index"`
	Confidence        float64     `json:"confidence" gorm:"not null;default:0"`
	ExecutionStatus   string      `json:"execution_status" gorm:"not null;default:'pending';index"`
	ExecutedAt        *time.Time  `json:"executed_at"`
	ExecutionError    *string     `json:"execution_error"`
	CreatedAt         time.Time   `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time   `json:"updated_at" gorm:"autoUpdateTime"`
	Revision          string      `json:"revision" gorm:"-"`
	AssigneeMemberID  *string     `json:"assignee_member_id" gorm:"-"`
	AssigneeAvailable bool        `json:"assignee_available" gorm:"-"`
}

func (CRMSuggestion) TableName() string { return "crm_suggestions" }

// CreateCRMSuggestionRequest is the payload for creating a suggestion.
type CreateCRMSuggestionRequest struct {
	WorkspaceID    string                 `json:"workspace_id"`
	UserID         *string                `json:"user_id"`
	SuggestionType string                 `json:"suggestion_type"`
	ObjectType     *string                `json:"object_type"`
	ObjectID       *string                `json:"object_id"`
	Title          string                 `json:"title"`
	Description    *string                `json:"description"`
	Context        map[string]interface{} `json:"context"`
	SignalIDs      []string               `json:"signal_ids,omitempty"`
	Confidence     *float64               `json:"confidence"`
}

// UpdateCRMSuggestionRequest is the payload for updating a suggestion.
type UpdateCRMSuggestionRequest struct {
	Status *string `json:"status"` // accepted, dismissed
}

// CRMSuggestionListFilters applies filters when listing suggestions.
type CRMSuggestionListFilters struct {
	// IncludeInternalMeetingFollowUps is reserved for canonical processing/deduplication.
	IncludeInternalMeetingFollowUps bool `json:"-"`
	UserID                          *string
	SuggestionType                  *string
	ObjectType                      *string
	ObjectID                        *string
	Status                          *string
}
