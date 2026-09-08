package model

import "time"

// CRMSituationCategory is a navigation group, not a replacement commercial motion.
type CRMSituationCategory struct {
	Key     string   `json:"key"`
	Label   string   `json:"label"`
	Motions []string `json:"motions"`
}

// CRMSituationCategories returns fresh copies of the four agreed navigation groups.
func CRMSituationCategories() []CRMSituationCategory {
	return []CRMSituationCategory{
		{Key: "sales", Label: "Sales", Motions: []string{"prospecting", "conversion"}},
		{Key: "onboarding_adoption", Label: "Onboarding & adoption", Motions: []string{"onboarding", "adoption"}},
		{Key: "expansion", Label: "Expansion", Motions: []string{"expansion"}},
		{Key: "retention", Label: "Retention", Motions: []string{"renewal", "retention"}},
	}
}

// CRMSituationCategoryForMotion preserves the original motion while grouping navigation.
func CRMSituationCategoryForMotion(motion string) string {
	for _, category := range CRMSituationCategories() {
		for _, candidate := range category.Motions {
			if candidate == motion {
				return category.Key
			}
		}
	}
	return ""
}

const (
	// CRMSituationOpen means the customer objective is still being pursued.
	CRMSituationOpen = "open"
	// CRMSituationPaused suspends customer work without claiming an outcome.
	CRMSituationPaused = "paused"
	// CRMSituationClosed requires an explicit customer outcome.
	CRMSituationClosed = "closed"
	// CRMSituationNeedsContext requests human triage or missing information.
	CRMSituationNeedsContext = "needs_context"
	// CRMSituationNeedsApproval identifies a pending decision, not execution success.
	CRMSituationNeedsApproval = "needs_approval"
	// CRMSituationFollowUpDue identifies a due human commitment.
	CRMSituationFollowUpDue = "follow_up_due"
	// CRMSituationWaitingCustomer means the next event belongs to the customer.
	CRMSituationWaitingCustomer = "waiting_customer"
	// CRMSituationWaitingWork means a linked dependency is outstanding.
	CRMSituationWaitingWork = "waiting_work"
	// CRMSituationAutomationFailed requires execution recovery.
	CRMSituationAutomationFailed = "automation_failed"
	// CRMSituationReferenceSignal links immutable evidence owned by CRM intelligence.
	CRMSituationReferenceSignal = "signal"
	// CRMSituationReferenceSuggestion links the authoritative existing CRM action.
	CRMSituationReferenceSuggestion = "suggestion"
)

// CRMSituation tracks one customer objective independently of evidence and executor state.
// Owner IDs are workspace-member IDs, never user IDs. Nil ownership is persisted,
// not resolved to a different person on each read.
type CRMSituation struct {
	ID                        string                         `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID               string                         `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_crm_situation_creation,priority:1"`
	CreationKey               string                         `json:"-" gorm:"not null;uniqueIndex:idx_crm_situation_creation,priority:2"`
	CreationFingerprint       string                         `json:"-" gorm:"not null"`
	OriginKind                string                         `json:"origin_kind" gorm:"not null;default:'manual'"`
	Title                     string                         `json:"title" gorm:"not null"`
	Objective                 string                         `json:"objective" gorm:"not null"`
	CommercialMotion          string                         `json:"commercial_motion" gorm:"not null"`
	CompanyID                 *string                        `json:"company_id" gorm:"type:uuid"`
	ContactID                 *string                        `json:"contact_id" gorm:"type:uuid"`
	DealID                    *string                        `json:"deal_id" gorm:"type:uuid"`
	OwnerMemberID             *string                        `json:"owner_member_id" gorm:"type:uuid"`
	NextActionOwnerMemberID   *string                        `json:"next_action_owner_member_id" gorm:"type:uuid"`
	Lifecycle                 string                         `json:"lifecycle" gorm:"not null;default:'open'"`
	Revision                  int64                          `json:"revision" gorm:"not null;default:1"`
	Attention                 string                         `json:"attention" gorm:"not null;default:'needs_context'"`
	NextStep                  string                         `json:"next_step" gorm:"not null;default:''"`
	Priority                  float64                        `json:"priority" gorm:"not null;default:0"`
	NextCheckpointAt          *time.Time                     `json:"next_checkpoint_at"`
	PlaybookID                *string                        `json:"playbook_id,omitempty" gorm:"type:uuid"`
	PlaybookVersionID         *string                        `json:"playbook_version_id,omitempty" gorm:"type:uuid"`
	PlaybookAppliedByMemberID *string                        `json:"playbook_applied_by_member_id,omitempty" gorm:"type:uuid"`
	PlaybookAppliedAt         *time.Time                     `json:"playbook_applied_at,omitempty"`
	PlaybookMilestones        []CRMPlaybookMilestoneProgress `json:"playbook_milestones,omitempty" gorm:"serializer:json;type:jsonb"`
	OutcomeKind               *string                        `json:"outcome_kind"`
	OutcomeSummary            *string                        `json:"outcome_summary"`
	OutcomeBasis              *string                        `json:"outcome_basis"`
	DuplicateOfSituationID    *string                        `json:"duplicate_of_situation_id" gorm:"type:uuid"`
	ClosedAt                  *time.Time                     `json:"closed_at"`
	ClosedByMemberID          *string                        `json:"closed_by_member_id" gorm:"type:uuid"`
	CreatedByMemberID         *string                        `json:"created_by_member_id" gorm:"type:uuid"`
	CreatedAt                 time.Time                      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                 time.Time                      `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName identifies persistent customer-work records.
func (CRMSituation) TableName() string { return "crm_situations" }

// CRMSituationReference preserves source identity without copying its payload or status.
type CRMSituationReference struct {
	WorkspaceID string    `json:"-" gorm:"type:uuid;primaryKey"`
	SituationID string    `json:"-" gorm:"type:uuid;primaryKey"`
	Kind        string    `json:"kind" gorm:"primaryKey"`
	SourceID    string    `json:"source_id" gorm:"type:uuid;primaryKey"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName identifies links to authoritative evidence and action records.
func (CRMSituationReference) TableName() string { return "crm_situation_references" }

// CRMSituationItem is the read projection, including current identity availability.
type CRMSituationItem struct {
	Situation                CRMSituation            `json:"situation" gorm:"embedded"`
	Category                 string                  `json:"category"`
	CompanyName              string                  `json:"company_name"`
	ContactName              string                  `json:"contact_name"`
	DealName                 string                  `json:"deal_name"`
	OwnerName                string                  `json:"owner_name"`
	NextActionOwnerName      string                  `json:"next_action_owner_name"`
	OwnerAvailable           bool                    `json:"owner_available"`
	NextActionOwnerAvailable bool                    `json:"next_action_owner_available"`
	References               []CRMSituationReference `json:"references,omitempty" gorm:"-"`
	Actions                  []CRMSuggestion         `json:"actions,omitempty" gorm:"-"`
	Evidence                 []CRMSituationEvidence  `json:"evidence,omitempty" gorm:"-"`
	PendingActionCount       int64                   `json:"pending_action_count"`
	FailedActionCount        int64                   `json:"failed_action_count"`
	UncertainActionCount     int64                   `json:"uncertain_action_count"`
	ManualActionCount        int64                   `json:"manual_action_count"`
	ExecutingActionCount     int64                   `json:"executing_action_count"`
	EffectiveAttention       string                  `json:"effective_attention"`
	CheckpointStatus         string                  `json:"checkpoint_status,omitempty"`
	CheckpointResult         string                  `json:"checkpoint_result,omitempty"`
	CheckpointAttempts       int                     `json:"checkpoint_attempts,omitempty"`
	CheckpointCompletedAt    *time.Time              `json:"checkpoint_completed_at,omitempty"`
}

// CRMSituationEvidence exposes linked CRM provenance without internal scoring metadata.
// The CRM read permission applies; opening the source still requires source-module access.
type CRMSituationEvidence struct {
	ID                    string     `json:"id"`
	Summary               string     `json:"summary"`
	EvidenceExcerpt       *string    `json:"evidence_excerpt,omitempty"`
	SourceType            string     `json:"source_type"`
	SourceID              *string    `json:"source_id,omitempty"`
	SourceThreadID        *string    `json:"source_thread_id,omitempty"`
	ContactID             *string    `json:"contact_id,omitempty"`
	CompanyID             *string    `json:"company_id,omitempty"`
	EvidenceIdentityTrust string     `json:"evidence_identity_trust"`
	DetectedAt            time.Time  `json:"detected_at"`
	ReviewedAt            *time.Time `json:"reviewed_at,omitempty"`
	DismissedAt           *time.Time `json:"dismissed_at,omitempty"`
	SupersededAt          *time.Time `json:"superseded_at,omitempty"`
}

// CRMSituationListFilters combines navigation controls with the shared query builder.
type CRMSituationListFilters struct {
	PlaybookID string
	Scope      string
	State      string
	Category   string
	Search     string
	Query      *QueryFilterGroup
	Page       int
	PageSize   int
}

// CRMSituationList returns complete filtered counts before pagination.
type CRMSituationList struct {
	Data               []CRMSituationItem `json:"data"`
	Total              int64              `json:"total"`
	Page               int                `json:"page"`
	PageSize           int                `json:"page_size"`
	CategoryCounts     map[string]int64   `json:"category_counts"`
	UncategorizedCount int64              `json:"uncategorized_count"`
	PendingActionTotal int64              `json:"pending_action_total"`
}

// CreateCRMSituationRequest creates manual customer work without invoking automation.
// OwnerMode is routing, member, or unassigned. CreationKey must be stable per request.
// Attention, lifecycle and approval/execution state are not client-writable inputs.
type CreateCRMSituationRequest struct {
	CreationKey             string                  `json:"creation_key"`
	Title                   string                  `json:"title"`
	Objective               string                  `json:"objective"`
	CommercialMotion        string                  `json:"commercial_motion"`
	CompanyID               *string                 `json:"company_id"`
	ContactID               *string                 `json:"contact_id"`
	DealID                  *string                 `json:"deal_id"`
	OwnerMode               string                  `json:"owner_mode"`
	OwnerMemberID           *string                 `json:"owner_member_id"`
	NextActionOwnerMemberID *string                 `json:"next_action_owner_member_id"`
	NextStep                string                  `json:"next_step"`
	Priority                float64                 `json:"priority"`
	References              []CRMSituationReference `json:"references"`
}
