package model

import "time"

// CRM Contact lifecycle stages.
const (
	CRMLifecycleSubscriber         = "subscriber"
	CRMLifecycleLead               = "lead"
	CRMLifecycleMarketingQualified = "marketing_qualified"
	CRMLifecycleSalesQualified     = "sales_qualified"
	CRMLifecycleOpportunity        = "opportunity"
	CRMLifecycleCustomer           = "customer"
	CRMLifecycleEvangelist         = "evangelist"
)

// CRM Contact lead statuses.
const (
	CRMLeadStatusNew         = "new"
	CRMLeadStatusOpen        = "open"
	CRMLeadStatusInProgress  = "in_progress"
	CRMLeadStatusUnqualified = "unqualified"
)

// crmLifecycleOrder maps lifecycle stages to their ordinal position.
// Higher values represent more advanced stages. Used to prevent downgrades.
var crmLifecycleOrder = map[string]int{
	CRMLifecycleSubscriber:         0,
	CRMLifecycleLead:               1,
	CRMLifecycleMarketingQualified: 2,
	CRMLifecycleSalesQualified:     3,
	CRMLifecycleOpportunity:        4,
	CRMLifecycleCustomer:           5,
	CRMLifecycleEvangelist:         6,
}

// CRMLifecycleIsHigherOrEqual returns true if current is at or above target in the lifecycle ordering.
func CRMLifecycleIsHigherOrEqual(current, target string) bool {
	return crmLifecycleOrder[current] >= crmLifecycleOrder[target]
}

// CRMContact represents a CRM contact.
type CRMContact struct {
	ID               string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	DisplayID        string    `json:"display_id" gorm:"not null"`
	FirstName        string    `json:"first_name" gorm:"not null"`
	LastName         *string   `json:"last_name"`
	Email            *string   `json:"email" gorm:"index"`
	Phone            *string   `json:"phone"`
	JobTitle         *string   `json:"job_title"`
	LifecycleStage   string    `json:"lifecycle_stage" gorm:"not null;default:'subscriber'"`
	LeadStatus       string    `json:"lead_status" gorm:"not null;default:'new'"`
	OwnerMemberID    *string   `json:"owner_member_id" gorm:"type:uuid;index"`
	AvatarURL        *string   `json:"avatar_url"`
	Source           *string   `json:"source"`
	CustomProperties JSONB     `json:"custom_properties" gorm:"type:jsonb;default:'{}'"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMContact) TableName() string { return "crm_contacts" }

// CreateCRMContactRequest is the payload for creating a contact.
type CreateCRMContactRequest struct {
	WorkspaceID      string                 `json:"workspace_id"`
	FirstName        string                 `json:"first_name"`
	LastName         *string                `json:"last_name"`
	Email            *string                `json:"email"`
	Phone            *string                `json:"phone"`
	JobTitle         *string                `json:"job_title"`
	LifecycleStage   *string                `json:"lifecycle_stage"`
	LeadStatus       *string                `json:"lead_status"`
	OwnerMemberID    *string                `json:"owner_member_id"`
	AvatarURL        *string                `json:"avatar_url"`
	Source           *string                `json:"source"`
	CustomProperties map[string]interface{} `json:"custom_properties"`
}

// SeedCRMContactsRequest is the payload for bulk-seeding test contacts.
type SeedCRMContactsRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Count       int    `json:"count"`
}

// SeedCRMContactsResponse reports how many contacts were created by a seed run.
type SeedCRMContactsResponse struct {
	Created int `json:"created"`
}

// UpdateCRMContactRequest is the payload for updating a contact.
type UpdateCRMContactRequest struct {
	FirstName        *string                `json:"first_name"`
	LastName         *string                `json:"last_name"`
	Email            *string                `json:"email"`
	Phone            *string                `json:"phone"`
	JobTitle         *string                `json:"job_title"`
	LifecycleStage   *string                `json:"lifecycle_stage"`
	LeadStatus       *string                `json:"lead_status"`
	OwnerMemberID    *string                `json:"owner_member_id"`
	AvatarURL        *string                `json:"avatar_url"`
	Source           *string                `json:"source"`
	CustomProperties map[string]interface{} `json:"custom_properties"`
}

// CRMContactListFilters applies filters when listing contacts.
type CRMContactListFilters struct {
	LifecycleStage *string
	LeadStatus     *string
	OwnerMemberID  *string
	Search         *string
	Query          *QueryFilterGroup
}
