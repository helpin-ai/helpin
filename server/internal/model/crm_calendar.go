package model

import "time"

// CRMCalendarEvent represents a calendar event linked to CRM objects.
type CRMCalendarEvent struct {
	ID              string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	EmailAccountID  string    `json:"email_account_id" gorm:"type:uuid;not null;index"`
	ExternalEventID *string   `json:"external_event_id"`
	Title           string    `json:"title" gorm:"not null"`
	Description     *string   `json:"description"`
	StartTime       time.Time `json:"start_time" gorm:"not null"`
	EndTime         time.Time `json:"end_time" gorm:"not null"`
	Location        *string   `json:"location"`
	Attendees       JSONB     `json:"attendees" gorm:"type:jsonb;default:'[]'"`
	ContactIDs      JSONB     `json:"contact_ids" gorm:"type:jsonb;default:'[]'"`
	DealID          *string   `json:"deal_id" gorm:"type:uuid;index"`
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMCalendarEvent) TableName() string { return "crm_calendar_events" }

// CreateCRMCalendarEventRequest is the payload for creating a calendar event.
type CreateCRMCalendarEventRequest struct {
	WorkspaceID    string                 `json:"workspace_id"`
	EmailAccountID string                 `json:"email_account_id"`
	Title          string                 `json:"title"`
	Description    *string                `json:"description"`
	StartTime      time.Time              `json:"start_time"`
	EndTime        time.Time              `json:"end_time"`
	Location       *string                `json:"location"`
	Attendees      map[string]interface{} `json:"attendees"`
	ContactIDs     map[string]interface{} `json:"contact_ids"`
	DealID         *string                `json:"deal_id"`
}

// UpdateCRMCalendarEventRequest is the payload for updating a calendar event.
type UpdateCRMCalendarEventRequest struct {
	Title       *string                `json:"title"`
	Description *string                `json:"description"`
	StartTime   *time.Time             `json:"start_time"`
	EndTime     *time.Time             `json:"end_time"`
	Location    *string                `json:"location"`
	Attendees   map[string]interface{} `json:"attendees"`
	ContactIDs  map[string]interface{} `json:"contact_ids"`
	DealID      *string                `json:"deal_id"`
}

// CRMCalendarEventListFilters applies filters when listing calendar events.
type CRMCalendarEventListFilters struct {
	EmailAccountID *string
	ContactID      *string
	DealID         *string
	StartAfter     *time.Time
	StartBefore    *time.Time
}
