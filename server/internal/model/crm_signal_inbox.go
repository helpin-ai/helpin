package model

import "time"

// CRM signal priority thresholds are shared by scoring and inbox presentation.
const (
	CRMSignalHighPriority   = 15
	CRMSignalMediumPriority = 8
)

// CRMSignalInboxItem is a read-only union member, never a synthetic situation.
// Kind and ID identify a canonical situation, a standalone recommendation, or
// a read-only evidence group. Evidence group IDs are not situation IDs.
type CRMSignalInboxItem struct {
	ID                 string    `json:"id"`
	Kind               string    `json:"kind"`
	Title              string    `json:"title"`
	NextStep           string    `json:"next_step"`
	Category           string    `json:"category"`
	CustomerName       string    `json:"customer_name"`
	OwnerName          string    `json:"owner_name"`
	OwnerMemberID      *string   `json:"owner_member_id"`
	OwnerAvailable     bool      `json:"owner_available"`
	Priority           *float64  `json:"priority"`
	PriorityBand       string    `json:"priority_band"`
	Lifecycle          string    `json:"lifecycle"`
	Attention          string    `json:"attention"`
	PendingActionCount int64     `json:"pending_action_count"`
	EvidenceReview     string    `json:"evidence_review"`
	CreatedAt          time.Time `json:"created_at"`
}

// CRMSignalInboxList counts work items, not unique customers or approvals.
type CRMSignalInboxList struct {
	Data               []CRMSignalInboxItem `json:"data"`
	Total              int64                `json:"total"`
	Page               int                  `json:"page"`
	PageSize           int                  `json:"page_size"`
	CategoryCounts     map[string]int64     `json:"category_counts"`
	UncategorizedCount int64                `json:"uncategorized_count"`
}

// CRMSignalInboxFilters extends the canonical navigation with queue-specific sorting.
type CRMSignalInboxFilters struct {
	Navigation CRMSituationListFilters
	Sort       string
}

// CRMInboxRecommendation preserves the original proposal and its source identity.
// LinkedSituations allows an old recommendation deep link to open its current work.
type CRMInboxRecommendation struct {
	Action           CRMSuggestion `json:"action"`
	LinkedSituations []string      `json:"linked_situations"`
}
