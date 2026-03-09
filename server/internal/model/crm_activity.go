package model

import "time"

// CRM activity types.
const (
	CRMActivityNote    = "note"
	CRMActivityCall    = "call"
	CRMActivityMeeting = "meeting"
	CRMActivityEmail   = "email"
	CRMActivityTask    = "task"
)

// CRMActivity represents an activity logged against CRM objects.
type CRMActivity struct {
	ID            string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID   string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ActivityType  string    `json:"activity_type" gorm:"not null;default:'note'"`
	ContactID     *string   `json:"contact_id" gorm:"type:uuid;index"`
	CompanyID     *string   `json:"company_id" gorm:"type:uuid;index"`
	DealID        *string   `json:"deal_id" gorm:"type:uuid;index"`
	OwnerMemberID *string   `json:"owner_member_id" gorm:"type:uuid;index"`
	Subject       *string   `json:"subject"`
	Body          *string   `json:"body"`
	OccurredAt    time.Time `json:"occurred_at" gorm:"not null"`
	Metadata      JSONB     `json:"metadata" gorm:"type:jsonb;default:'{}'"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMActivity) TableName() string { return "crm_activities" }

// CreateCRMActivityRequest is the payload for creating an activity.
type CreateCRMActivityRequest struct {
	WorkspaceID   string                 `json:"workspace_id"`
	ActivityType  string                 `json:"activity_type"`
	ContactID     *string                `json:"contact_id"`
	CompanyID     *string                `json:"company_id"`
	DealID        *string                `json:"deal_id"`
	OwnerMemberID *string                `json:"owner_member_id"`
	Subject       *string                `json:"subject"`
	Body          *string                `json:"body"`
	OccurredAt    *time.Time             `json:"occurred_at"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// UpdateCRMActivityRequest is the payload for updating an activity.
type UpdateCRMActivityRequest struct {
	ActivityType  *string                `json:"activity_type"`
	ContactID     *string                `json:"contact_id"`
	CompanyID     *string                `json:"company_id"`
	DealID        *string                `json:"deal_id"`
	Subject       *string                `json:"subject"`
	Body          *string                `json:"body"`
	OccurredAt    *time.Time             `json:"occurred_at"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// CRMActivityListFilters applies filters when listing activities.
type CRMActivityListFilters struct {
	ActivityType *string
	ContactID    *string
	CompanyID    *string
	DealID       *string
}
