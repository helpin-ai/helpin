package model

import "time"

// CRMPlaybookAction is an exact supported proposal, never an instruction to execute free text.
// The canonical CRMSuggestion owns its decision and execution status.
type CRMPlaybookAction struct {
	Version   int                         `json:"version"`
	Kind      string                      `json:"kind"`
	Title     string                      `json:"title"`
	Reason    string                      `json:"reason"`
	Email     *CRMPlaybookEmailAction     `json:"email,omitempty"`
	Task      *CRMPlaybookTaskAction      `json:"task,omitempty"`
	Handoff   *CRMPlaybookHandoffAction   `json:"handoff,omitempty"`
	Milestone *CRMPlaybookMilestoneAction `json:"milestone,omitempty"`
	DealStage *CRMPlaybookDealStageAction `json:"deal_stage,omitempty"`
}

// CRMPlaybookEmailAction narrows outbound to one existing CRM contact and authorized mailbox.
type CRMPlaybookEmailAction struct {
	AccountID string `json:"account_id"`
	ContactID string `json:"contact_id"`
	To        string `json:"to"`
	Subject   string `json:"subject"`
	BodyHTML  string `json:"body_html"`
}

// CRMPlaybookTaskAction creates a genuine deliverable in an explicitly chosen team.
type CRMPlaybookTaskAction struct {
	TeamID        string     `json:"team_id"`
	OwnerMemberID string     `json:"owner_member_id"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	Deadline      *time.Time `json:"deadline,omitempty"`
}

// CRMPlaybookHandoffAction is accepted by the receiving owner, not inferred from a generated summary.
type CRMPlaybookHandoffAction struct {
	ReceivingMemberID string `json:"receiving_member_id"`
	Summary           string `json:"summary"`
}

// CRMPlaybookMilestoneAction requests an attributed human assessment of observable evidence.
type CRMPlaybookMilestoneAction struct {
	Key      string `json:"key"`
	Evidence string `json:"evidence"`
}

// CRMPlaybookDealStageAction cannot mutate arbitrary CRM properties or another customer's deal.
type CRMPlaybookDealStageAction struct {
	DealID  string `json:"deal_id"`
	StageID string `json:"stage_id"`
}

// CRMPlaybookActionIntent binds an existing canonical suggestion to its business authority.
// It deliberately has no copied decision or execution status.
type CRMPlaybookActionIntent struct {
	SuggestionID        string     `json:"suggestion_id" gorm:"type:uuid;primaryKey"`
	WorkspaceID         string     `json:"-" gorm:"type:uuid"`
	SituationID         string     `json:"situation_id" gorm:"type:uuid"`
	RunID               string     `json:"run_id" gorm:"type:uuid"`
	ConnectionID        string     `json:"connection_id" gorm:"type:uuid"`
	Generation          int64      `json:"generation"`
	SituationRevision   int64      `json:"situation_revision"`
	IntentKey           string     `json:"-"`
	RecipientKey        string     `json:"-"`
	ContextFingerprint  string     `json:"context_fingerprint"`
	ApproverMemberID    string     `json:"approver_member_id" gorm:"type:uuid"`
	ExpiresAt           time.Time  `json:"expires_at"`
	ApprovedByMemberID  *string    `json:"approved_by_member_id,omitempty" gorm:"type:uuid"`
	ApprovedAt          *time.Time `json:"approved_at,omitempty"`
	ApprovedFingerprint string     `json:"approved_fingerprint,omitempty"`
	ResultType          string     `json:"result_type,omitempty"`
	ResultID            *string    `json:"result_id,omitempty" gorm:"type:uuid"`
	CreatedAt           time.Time  `json:"created_at"`
}

// TableName identifies proposal authority, not a separate action queue.
func (CRMPlaybookActionIntent) TableName() string { return "crm_playbook_action_intents" }

// CRMPlaybookActionInspection is a human-attributed resolution, never a resend.
type CRMPlaybookActionInspection struct {
	Revision  string `json:"revision"`
	Outcome   string `json:"outcome"`
	Evidence  string `json:"evidence"`
	Confirmed bool   `json:"confirmed"`
}

// CRMPlaybookActionFacts excludes mailbox credentials and unrelated customer records.
type CRMPlaybookActionFacts struct {
	Contact               *CRMContact             `json:"contact,omitempty"`
	Company               *CRMCompany             `json:"company,omitempty"`
	Deal                  *CRMDeal                `json:"deal,omitempty"`
	LastContactMessageAt  *time.Time              `json:"last_contact_message_at,omitempty"`
	LastOutboundMessageAt *time.Time              `json:"last_outbound_message_at,omitempty"`
	LinkedTasks           []CRMPlaybookLinkedTask `json:"linked_tasks,omitempty"`
}

// CRMPlaybookLinkedTask reports delivery state only for tasks created by this Signal.
// It does not expose unrelated PM content or infer a customer milestone.
type CRMPlaybookLinkedTask struct {
	ID          string     `json:"id"`
	TeamID      *string    `json:"team_id"`
	StateID     string     `json:"state_id" gorm:"column:workflow_state_id"`
	CompletedAt *time.Time `json:"completed_at"`
	Deadline    *time.Time `json:"deadline"`
}

// CRMPlaybookTaskTeam exposes only the current caller's real PM destinations.
type CRMPlaybookTaskTeam struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
