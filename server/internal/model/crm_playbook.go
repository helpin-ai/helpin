package model

import "time"

// CRMPlaybookDefinition is typed CRM policy, not executable prompts or Flow configuration.
type CRMPlaybookDefinition struct {
	Name             string                      `json:"name"`
	Description      string                      `json:"description"`
	Journey          string                      `json:"journey"`
	Objective        string                      `json:"objective"`
	Eligibility      CRMPlaybookEligibility      `json:"eligibility"`
	Responsibilities CRMPlaybookResponsibilities `json:"responsibilities"`
	Milestones       []CRMPlaybookMilestone      `json:"milestones"`
	Policy           CRMPlaybookPolicy           `json:"policy"`
}

// CRMPlaybookEligibility narrows existing Signals using the shared query builder.
type CRMPlaybookEligibility struct {
	CommercialMotions []string          `json:"commercial_motions"`
	Filter            *QueryFilterGroup `json:"filter,omitempty"`
}

// CRMPlaybookResponsibilities describes business roles without changing account ownership.
type CRMPlaybookResponsibilities struct {
	OwnerRole          string  `json:"owner_role"`
	ApproverRole       string  `json:"approver_role"`
	EscalationMemberID *string `json:"escalation_member_id"`
}

// CRMPlaybookMilestone defines an observable customer result, never an executed task.
type CRMPlaybookMilestone struct {
	Key             string `json:"key"`
	Name            string `json:"name"`
	SuccessCriteria string `json:"success_criteria"`
}

// CRMPlaybookPolicy stores approval and follow-up requirements. Saving it grants no execution authority.
type CRMPlaybookPolicy struct {
	OutboundMessages   string   `json:"outbound_messages"`
	CRMChanges         string   `json:"crm_changes"`
	PMTasks            string   `json:"pm_tasks"`
	CheckAfterHours    int      `json:"check_after_hours"`
	EscalateAfterHours int      `json:"escalate_after_hours"`
	StopConditions     []string `json:"stop_conditions"`
}

// CRMPlaybook is the editable definition and admission gate; active Signals keep their pinned version.
type CRMPlaybook struct {
	ID                  string                `json:"id" gorm:"type:uuid;primaryKey"`
	WorkspaceID         string                `json:"workspace_id" gorm:"type:uuid;not null"`
	CreationKey         string                `json:"-"`
	CreationFingerprint string                `json:"-"`
	Revision            int64                 `json:"revision"`
	Draft               CRMPlaybookDefinition `json:"draft" gorm:"serializer:json;type:jsonb"`
	PublishedVersionID  *string               `json:"published_version_id" gorm:"type:uuid"`
	AcceptingCustomers  bool                  `json:"accepting_customers"`
	CreatedByMemberID   string                `json:"created_by_member_id" gorm:"type:uuid"`
	UpdatedByMemberID   string                `json:"updated_by_member_id" gorm:"type:uuid"`
	CreatedAt           time.Time             `json:"created_at"`
	UpdatedAt           time.Time             `json:"updated_at"`
}

// TableName identifies CRM-owned configuration.
func (CRMPlaybook) TableName() string { return "crm_playbooks" }

// CRMPlaybookVersion is an immutable published policy snapshot.
type CRMPlaybookVersion struct {
	ID                  string                `json:"id" gorm:"type:uuid;primaryKey"`
	WorkspaceID         string                `json:"-" gorm:"type:uuid"`
	PlaybookID          string                `json:"playbook_id" gorm:"type:uuid"`
	Version             int64                 `json:"version"`
	Definition          CRMPlaybookDefinition `json:"definition" gorm:"serializer:json;type:jsonb"`
	Fingerprint         string                `json:"-"`
	PublishedByMemberID string                `json:"published_by_member_id" gorm:"type:uuid"`
	PublishedAt         time.Time             `json:"published_at"`
}

// TableName identifies immutable CRM policy versions.
func (CRMPlaybookVersion) TableName() string { return "crm_playbook_versions" }

// CRMPlaybookChange is the append-only audit record and idempotent command receipt.
type CRMPlaybookChange struct {
	ID                 string      `json:"id" gorm:"type:uuid;primaryKey"`
	WorkspaceID        string      `json:"-" gorm:"type:uuid"`
	PlaybookID         string      `json:"playbook_id" gorm:"type:uuid"`
	Revision           int64       `json:"revision"`
	CommandKey         string      `json:"-"`
	CommandFingerprint string      `json:"-"`
	Operation          string      `json:"operation"`
	ActorMemberID      string      `json:"actor_member_id" gorm:"type:uuid"`
	Reason             string      `json:"reason"`
	After              CRMPlaybook `json:"after" gorm:"serializer:json;type:jsonb"`
	CreatedAt          time.Time   `json:"created_at"`
}

// TableName identifies Playbook configuration history, not process execution.
func (CRMPlaybookChange) TableName() string { return "crm_playbook_changes" }

// CRMPlaybookItem combines configuration with counts of canonical customer work.
type CRMPlaybookItem struct {
	ExecutionEnabled bool                `json:"execution_enabled" gorm:"-"`
	Playbook         CRMPlaybook         `json:"playbook" gorm:"embedded"`
	OpenCount        int64               `json:"open_count"`
	PausedCount      int64               `json:"paused_count"`
	ClosedCount      int64               `json:"closed_count"`
	PublishedVersion *CRMPlaybookVersion `json:"published_version,omitempty" gorm:"-"`
}

// CRMPlaybookList returns complete counts before pagination.
type CRMPlaybookList struct {
	Data     []CRMPlaybookItem `json:"data"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

// CRMPlaybookListFilters provides bounded navigation without client-side totals.
type CRMPlaybookListFilters struct {
	Search   string
	State    string
	Page     int
	PageSize int
}

// CreateCRMPlaybookRequest creates a draft with enrollment disabled.
type CreateCRMPlaybookRequest struct {
	CreationKey string                `json:"creation_key"`
	Definition  CRMPlaybookDefinition `json:"definition"`
}

// CRMPlaybookCommandRequest binds a configuration edit to an observed revision.
type CRMPlaybookCommandRequest struct {
	CommandKey         string                 `json:"command_key"`
	ExpectedRevision   int64                  `json:"expected_revision"`
	Operation          string                 `json:"operation"`
	Definition         *CRMPlaybookDefinition `json:"definition,omitempty"`
	AcceptingCustomers *bool                  `json:"accepting_customers,omitempty"`
	Reason             string                 `json:"reason"`
}

// CRMPlaybookCommandResult returns the original receipt on retry, not a fabricated current snapshot.
type CRMPlaybookCommandResult struct {
	Change   CRMPlaybookChange `json:"change"`
	Replayed bool              `json:"replayed"`
}

// CRMPlaybookHistory provides keyset-paginated configuration history.
type CRMPlaybookHistory struct {
	Data               []CRMPlaybookChange `json:"data"`
	NextBeforeRevision *int64              `json:"next_before_revision"`
}

// CRMPlaybookVersions provides keyset-paginated published definitions.
type CRMPlaybookVersions struct {
	Data              []CRMPlaybookVersion `json:"data"`
	NextBeforeVersion *int64               `json:"next_before_version"`
}

// CRMPlaybookPreview is a read-only eligibility preview, never an execution simulation.
type CRMPlaybookPreview struct {
	VersionID        *string               `json:"version_id"`
	PlaybookRevision int64                 `json:"playbook_revision"`
	Definition       CRMPlaybookDefinition `json:"definition"`
	Signals          CRMSituationList      `json:"signals"`
	Scope            string                `json:"scope"`
	ExecutionEnabled bool                  `json:"execution_enabled"`
}

// CRMPlaybookMilestoneProgress records an explicit assessment separately from customer closure.
type CRMPlaybookMilestoneProgress struct {
	Key                string     `json:"key"`
	Status             string     `json:"status"`
	Summary            string     `json:"summary"`
	AssessedByMemberID *string    `json:"assessed_by_member_id"`
	AssessedAt         *time.Time `json:"assessed_at"`
	Basis              *string    `json:"basis"`
}

// ApplyCRMPlaybookRequest explicitly attaches one published version to existing work.
type ApplyCRMPlaybookRequest struct {
	CommandKey                string `json:"command_key"`
	SituationID               string `json:"situation_id"`
	VersionID                 string `json:"version_id"`
	ExpectedPlaybookRevision  int64  `json:"expected_playbook_revision"`
	ExpectedSituationRevision int64  `json:"expected_situation_revision"`
	Confirmed                 bool   `json:"confirmed"`
}

// CRMPlaybookMilestoneRequest records a human assessment against an observed Signal revision.
type CRMPlaybookMilestoneRequest struct {
	CommandKey       string `json:"command_key"`
	ExpectedRevision int64  `json:"expected_revision"`
	MilestoneKey     string `json:"milestone_key"`
	Status           string `json:"status"`
	Summary          string `json:"summary"`
}
