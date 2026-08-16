package model

import (
	"database/sql/driver"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

const (
	CRMCalendarEventStatusConfirmed = "confirmed"
	CRMCalendarEventStatusCancelled = "cancelled"
)

// CRMCalendarAttendee is a normalized calendar participant.
type CRMCalendarAttendee struct {
	Email          string `json:"email"`
	Name           string `json:"name,omitempty"`
	ResponseStatus string `json:"response_status,omitempty"`
	Organizer      bool   `json:"organizer,omitempty"`
	Self           bool   `json:"self,omitempty"`
}

// CRMCalendarAttendees persists participants as a JSON array. Its custom
// decoder also accepts the earlier object-shaped API payload for compatibility.
type CRMCalendarAttendees []CRMCalendarAttendee

func (a CRMCalendarAttendees) Value() (driver.Value, error) {
	if a == nil {
		return "[]", nil
	}
	payload, err := json.Marshal([]CRMCalendarAttendee(a))
	if err != nil {
		return nil, err
	}
	return string(payload), nil
}

func (a *CRMCalendarAttendees) Scan(value interface{}) error {
	return scanJSONArray(value, a)
}

func (a *CRMCalendarAttendees) UnmarshalJSON(data []byte) error {
	var list []CRMCalendarAttendee
	if err := json.Unmarshal(data, &list); err == nil {
		*a = list
		return nil
	}
	var legacy map[string]CRMCalendarAttendee
	if err := json.Unmarshal(data, &legacy); err != nil {
		return err
	}
	keys := make([]string, 0, len(legacy))
	for key := range legacy {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	list = make([]CRMCalendarAttendee, 0, len(keys))
	for _, key := range keys {
		list = append(list, legacy[key])
	}
	*a = list
	return nil
}

// CRMStringList persists string identifiers as a JSON array and accepts the
// earlier object-shaped payload during decoding.
type CRMStringList []string

func (s CRMStringList) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	payload, err := json.Marshal([]string(s))
	if err != nil {
		return nil, err
	}
	return string(payload), nil
}

func (s *CRMStringList) Scan(value interface{}) error {
	return scanJSONArray(value, s)
}

func (s *CRMStringList) UnmarshalJSON(data []byte) error {
	var list []string
	if err := json.Unmarshal(data, &list); err == nil {
		*s = normalizeCRMStringList(list)
		return nil
	}
	var legacy map[string]interface{}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return err
	}
	list = make([]string, 0, len(legacy))
	for key, value := range legacy {
		if typed, ok := value.(string); ok {
			list = append(list, typed)
			continue
		}
		list = append(list, key)
	}
	*s = normalizeCRMStringList(list)
	return nil
}

func normalizeCRMStringList(values []string) CRMStringList {
	result := make(CRMStringList, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// CRMCalendarEvent represents a synced or manually-created calendar event.
type CRMCalendarEvent struct {
	ID              string               `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string               `json:"workspace_id" gorm:"type:uuid;not null;index"`
	EmailAccountID  string               `json:"email_account_id" gorm:"type:uuid;not null;index"`
	ExternalEventID *string              `json:"external_event_id" gorm:"index"`
	Title           string               `json:"title" gorm:"not null"`
	Description     *string              `json:"description"`
	StartTime       time.Time            `json:"start_time" gorm:"not null;index"`
	EndTime         time.Time            `json:"end_time" gorm:"not null"`
	Location        *string              `json:"location"`
	MeetingURL      *string              `json:"meeting_url"`
	OrganizerEmail  *string              `json:"organizer_email"`
	Status          string               `json:"status" gorm:"not null;default:'confirmed';index"`
	Visibility      string               `json:"visibility" gorm:"not null;default:'default'"`
	AllDay          bool                 `json:"all_day" gorm:"not null;default:false"`
	Attendees       CRMCalendarAttendees `json:"attendees" gorm:"type:jsonb;default:'[]'"`
	ContactIDs      CRMStringList        `json:"contact_ids" gorm:"type:jsonb;default:'[]'"`
	DealID          *string              `json:"deal_id" gorm:"type:uuid;index"`
	CreatedAt       time.Time            `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time            `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMCalendarEvent) TableName() string { return "crm_calendar_events" }

// CreateCRMCalendarEventRequest is the payload for creating a calendar event.
type CreateCRMCalendarEventRequest struct {
	WorkspaceID    string               `json:"workspace_id"`
	EmailAccountID string               `json:"email_account_id"`
	Title          string               `json:"title"`
	Description    *string              `json:"description"`
	StartTime      time.Time            `json:"start_time"`
	EndTime        time.Time            `json:"end_time"`
	Location       *string              `json:"location"`
	MeetingURL     *string              `json:"meeting_url"`
	Attendees      CRMCalendarAttendees `json:"attendees"`
	ContactIDs     CRMStringList        `json:"contact_ids"`
	DealID         *string              `json:"deal_id"`
}

// UpdateCRMCalendarEventRequest is the payload for updating a calendar event.
type UpdateCRMCalendarEventRequest struct {
	Title       *string              `json:"title"`
	Description *string              `json:"description"`
	StartTime   *time.Time           `json:"start_time"`
	EndTime     *time.Time           `json:"end_time"`
	Location    *string              `json:"location"`
	MeetingURL  *string              `json:"meeting_url"`
	Attendees   CRMCalendarAttendees `json:"attendees"`
	ContactIDs  CRMStringList        `json:"contact_ids"`
	DealID      *string              `json:"deal_id"`
}

// CRMCalendarEventListFilters applies filters when listing calendar events.
type CRMCalendarEventListFilters struct {
	EmailAccountID *string
	ContactID      *string
	DealID         *string
	StartAfter     *time.Time
	StartBefore    *time.Time
	Status         *string
	Ascending      bool
}

// CRMCalendarMeetingCandidate combines an upcoming event with its capture record.
type CRMCalendarMeetingCandidate struct {
	Event               CRMCalendarEvent `json:"event"`
	Meeting             *CRMMeeting      `json:"meeting,omitempty"`
	Eligible            bool             `json:"eligible"`
	IneligibilityReason *string          `json:"ineligibility_reason,omitempty"`
}

// UpdateCRMCalendarMeetingCaptureRequest enables or disables scheduled capture.
type UpdateCRMCalendarMeetingCaptureRequest struct {
	Enabled     bool    `json:"enabled"`
	DealID      *string `json:"deal_id"`
	RecordAudio *bool   `json:"record_audio"`
}
