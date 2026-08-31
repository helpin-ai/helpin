package model

import "time"

const (
	CRMTimelineFilterAll     = "all"
	CRMTimelineFilterNote    = "note"
	CRMTimelineFilterEmail   = "email"
	CRMTimelineFilterCall    = "call"
	CRMTimelineFilterMeeting = "meeting"
	CRMTimelineFilterTask    = "task"
	CRMTimelineFilterDeal    = "deal"
	CRMTimelineFilterSupport = "support"

	// Backward-compatible names for existing company timeline callers.
	CRMCompanyTimelineFilterAll     = CRMTimelineFilterAll
	CRMCompanyTimelineFilterNote    = CRMTimelineFilterNote
	CRMCompanyTimelineFilterEmail   = CRMTimelineFilterEmail
	CRMCompanyTimelineFilterCall    = CRMTimelineFilterCall
	CRMCompanyTimelineFilterMeeting = CRMTimelineFilterMeeting
	CRMCompanyTimelineFilterTask    = CRMTimelineFilterTask
	CRMCompanyTimelineFilterDeal    = CRMTimelineFilterDeal
	CRMCompanyTimelineFilterSupport = CRMTimelineFilterSupport
)

// CRMTimelineReference identifies a related record displayed by a CRM timeline item.
type CRMTimelineReference struct {
	Type      string  `json:"type"`
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	DisplayID *string `json:"display_id,omitempty"`
}

// CRMTimelineItem is the normalized read model for a CRM record timeline event.
type CRMTimelineItem struct {
	ID          string                `json:"id"`
	Kind        string                `json:"kind"`
	EventType   string                `json:"event_type"`
	SourceType  string                `json:"source_type"`
	SourceID    string                `json:"source_id"`
	Title       string                `json:"title"`
	Description *string               `json:"description,omitempty"`
	OccurredAt  time.Time             `json:"occurred_at"`
	Actor       *CRMTimelineReference `json:"actor,omitempty"`
	Contact     *CRMTimelineReference `json:"contact,omitempty"`
	Entity      *CRMTimelineReference `json:"entity,omitempty"`
	CanEdit     bool                  `json:"can_edit"`
	CanDelete   bool                  `json:"can_delete"`
}

// CRMTimelinePage is one cursor-paginated CRM record timeline response.
type CRMTimelinePage struct {
	Data       []CRMTimelineItem `json:"data"`
	NextCursor *string           `json:"next_cursor,omitempty"`
}

// CRMTimelineQuery controls CRM record timeline filtering and pagination.
type CRMTimelineQuery struct {
	Filter   string
	CursorAt *time.Time
	CursorID string
	Limit    int
}

type CRMCompanyTimelineReference = CRMTimelineReference
type CRMCompanyTimelineItem = CRMTimelineItem
type CRMCompanyTimelinePage = CRMTimelinePage
type CRMCompanyTimelineQuery = CRMTimelineQuery
