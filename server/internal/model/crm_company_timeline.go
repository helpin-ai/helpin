package model

import "time"

const (
	CRMCompanyTimelineFilterAll     = "all"
	CRMCompanyTimelineFilterNote    = "note"
	CRMCompanyTimelineFilterEmail   = "email"
	CRMCompanyTimelineFilterCall    = "call"
	CRMCompanyTimelineFilterMeeting = "meeting"
	CRMCompanyTimelineFilterTask    = "task"
)

// CRMCompanyTimelineReference identifies a related record displayed by a timeline item.
type CRMCompanyTimelineReference struct {
	Type      string  `json:"type"`
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	DisplayID *string `json:"display_id,omitempty"`
}

// CRMCompanyTimelineItem is the normalized read model for a company timeline event.
type CRMCompanyTimelineItem struct {
	ID          string                       `json:"id"`
	Kind        string                       `json:"kind"`
	EventType   string                       `json:"event_type"`
	SourceType  string                       `json:"source_type"`
	SourceID    string                       `json:"source_id"`
	Title       string                       `json:"title"`
	Description *string                      `json:"description,omitempty"`
	OccurredAt  time.Time                    `json:"occurred_at"`
	Actor       *CRMCompanyTimelineReference `json:"actor,omitempty"`
	Contact     *CRMCompanyTimelineReference `json:"contact,omitempty"`
	Entity      *CRMCompanyTimelineReference `json:"entity,omitempty"`
	CanEdit     bool                         `json:"can_edit"`
	CanDelete   bool                         `json:"can_delete"`
}

// CRMCompanyTimelinePage is one cursor-paginated company timeline response.
type CRMCompanyTimelinePage struct {
	Data       []CRMCompanyTimelineItem `json:"data"`
	NextCursor *string                  `json:"next_cursor,omitempty"`
}

// CRMCompanyTimelineQuery controls company timeline filtering and pagination.
type CRMCompanyTimelineQuery struct {
	Filter   string
	CursorAt *time.Time
	CursorID string
	Limit    int
}
