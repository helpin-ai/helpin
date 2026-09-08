package model

import "time"

// CRMPlaybookWorkDue is the shared Automation wake-up for one customer objective.
const CRMPlaybookWorkDue = "crm.playbook.work_due"

// CRMPlaybookEntryDue evaluates a newly qualified source; it never backfills work.
const CRMPlaybookEntryDue = "crm.playbook.entry_due"

// CRMPlaybookAutomationSettings is the live gate for an immutable publication.
// Publication, entry and permission to work an existing Signal remain distinct.
type CRMPlaybookAutomationSettings struct {
	WorkspaceID          string     `json:"-" gorm:"type:uuid;primaryKey"`
	PlaybookID           string     `json:"playbook_id" gorm:"type:uuid;primaryKey"`
	ConnectionID         string     `json:"connection_id" gorm:"type:uuid"`
	Revision             int64      `json:"revision"`
	Enabled              bool       `json:"enabled"`
	EntryMode            string     `json:"entry_mode"`
	AutomaticSince       *time.Time `json:"automatic_since,omitempty"`
	MaxRunsPerDay        int        `json:"max_runs_per_day"`
	MaxNoProgressRuns    int        `json:"max_no_progress_runs"`
	AuthorizedByMemberID string     `json:"authorized_by_member_id" gorm:"type:uuid"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// TableName identifies activation, never an editable copy of the publication.
func (CRMPlaybookAutomationSettings) TableName() string { return "crm_playbook_automation_settings" }

// CRMPlaybookAutomationBinding connects the canonical Signal to execution.
// Customer lifecycle, milestones, responsibilities and checkpoints stay on CRM.
type CRMPlaybookAutomationBinding struct {
	WorkspaceID          string     `json:"-" gorm:"type:uuid;primaryKey"`
	SituationID          string     `json:"situation_id" gorm:"type:uuid;primaryKey"`
	PlaybookID           string     `json:"playbook_id" gorm:"type:uuid"`
	ConnectionID         string     `json:"connection_id" gorm:"type:uuid"`
	Enabled              bool       `json:"enabled"`
	Generation           int64      `json:"generation"`
	AuthorizedByMemberID string     `json:"authorized_by_member_id" gorm:"type:uuid"`
	LastRunID            *string    `json:"last_run_id" gorm:"type:uuid"`
	LastCheckedAt        *time.Time `json:"last_checked_at"`
	NoProgressRuns       int        `json:"no_progress_runs"`
	ContextFingerprint   string     `json:"-"`
	ActionFingerprint    string     `json:"-"`
	LastMaintenanceAt    *time.Time `json:"-"`
	ProgressObservedAt   *time.Time `json:"-"`
	EscalatedAt          *time.Time `json:"escalated_at,omitempty"`
	Blocker              string     `json:"blocker"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// TableName identifies a connection, not a second customer-process record.
func (CRMPlaybookAutomationBinding) TableName() string { return "crm_playbook_automation_bindings" }

// AutomationRunBinding is the durable link from a fenced event to a normal Agent run.
// Its identity and captured context are immutable; the runtime owns run execution.
type AutomationRunBinding struct {
	RunID              string               `json:"run_id" gorm:"type:uuid;primaryKey"`
	WorkspaceID        string               `json:"-" gorm:"type:uuid"`
	EventID            string               `json:"event_id" gorm:"type:uuid"`
	SituationID        string               `json:"situation_id" gorm:"type:uuid"`
	ConnectionID       string               `json:"connection_id" gorm:"type:uuid"`
	Generation         int64                `json:"generation"`
	SituationRevision  int64                `json:"situation_revision"`
	AgentID            string               `json:"agent_id" gorm:"type:uuid"`
	RuntimeProfileID   string               `json:"-" gorm:"type:uuid"`
	Input              AgentRunInputPayload `json:"-" gorm:"serializer:json;type:jsonb"`
	CreatedAt          time.Time            `json:"created_at"`
	ObservedTerminalAt *time.Time           `json:"observed_terminal_at"`
	LastMaintenanceAt  *time.Time           `json:"-"`
}

// TableName identifies shared Automation dispatch receipts, not an executor.
func (AutomationRunBinding) TableName() string { return "automation_run_bindings" }

// CRMPlaybookAutomationCommand is an optimistic, explicitly confirmed gate change.
type CRMPlaybookAutomationCommand struct {
	CommandKey        string `json:"command_key"`
	ExpectedRevision  int64  `json:"expected_revision"`
	ConnectionID      string `json:"connection_id"`
	Enabled           bool   `json:"enabled"`
	EntryMode         string `json:"entry_mode"`
	MaxRunsPerDay     int    `json:"max_runs_per_day"`
	MaxNoProgressRuns int    `json:"max_no_progress_runs"`
	Confirmed         bool   `json:"confirmed"`
}

// CRMPlaybookAutomationAdoption explicitly starts, pauses or updates one existing Signal.
type CRMPlaybookAutomationAdoption struct {
	CommandKey         string `json:"command_key"`
	ExpectedRevision   int64  `json:"expected_revision"`
	ExpectedGeneration int64  `json:"expected_generation"`
	ConnectionID       string `json:"connection_id"`
	Enabled            bool   `json:"enabled"`
	Confirmed          bool   `json:"confirmed"`
}

// CRMPlaybookExecutionSource contains tenant-local records used by live admission.
type CRMPlaybookExecutionSource struct {
	StopReason string `json:"-"`
	Settings   CRMPlaybookAutomationSettings
	Binding    CRMPlaybookAutomationBinding
	Connection CRMPlaybookConnection
	Policy     CRMPlaybookVersion
	Item       CRMSituationItem
}

// CRMPlaybookAutomationReceipt makes gate changes retry-safe without replaying effects.
type CRMPlaybookAutomationReceipt struct {
	WorkspaceID string                         `json:"-" gorm:"type:uuid;primaryKey"`
	SubjectID   string                         `json:"-" gorm:"type:uuid;primaryKey"`
	CommandKey  string                         `json:"-" gorm:"primaryKey"`
	Fingerprint string                         `json:"-"`
	Settings    *CRMPlaybookAutomationSettings `json:"settings,omitempty" gorm:"serializer:json;type:jsonb"`
	Binding     *CRMPlaybookAutomationBinding  `json:"binding,omitempty" gorm:"serializer:json;type:jsonb"`
	CreatedAt   time.Time                      `json:"created_at"`
}

// TableName identifies append-only activation/adoption receipts.
func (CRMPlaybookAutomationReceipt) TableName() string { return "crm_playbook_automation_receipts" }
