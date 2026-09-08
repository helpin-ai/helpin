package model

import "time"

// CRMSituationCommandRequest changes customer work, never executor or evidence state.
// CommandKey is stable across retries; ExpectedRevision is the last observed work revision.
type CRMSituationCommandRequest struct {
	CommandKey       string               `json:"command_key"`
	ExpectedRevision int64                `json:"expected_revision"`
	Operation        string               `json:"operation"`
	Reason           string               `json:"reason,omitempty"`
	Changes          *CRMSituationChanges `json:"changes,omitempty"`
	Outcome          *CRMSituationOutcome `json:"outcome,omitempty"`
}

// CRMSituationChanges is a partial update. Absent fields retain their values.
// Nested owner/checkpoint objects distinguish explicit clearing from no change.
type CRMSituationChanges struct {
	Owner           *CRMSituationOwnerChange      `json:"owner,omitempty"`
	NextActionOwner *CRMSituationOwnerChange      `json:"next_action_owner,omitempty"`
	NextStep        *string                       `json:"next_step,omitempty"`
	Attention       *string                       `json:"attention,omitempty"`
	Checkpoint      *CRMSituationCheckpointChange `json:"checkpoint,omitempty"`
}

// CRMSituationOwnerChange assigns an active workspace member; nil means Unassigned.
type CRMSituationOwnerChange struct {
	MemberID *string `json:"member_id"`
}

// CRMSituationCheckpointChange sets a durable checkpoint; nil explicitly clears it.
type CRMSituationCheckpointChange struct {
	At *time.Time `json:"at"`
}

// CRMSituationOutcome records an explicit human assessment, not execution success.
type CRMSituationOutcome struct {
	Kind                   string  `json:"kind"`
	Summary                string  `json:"summary"`
	DuplicateOfSituationID *string `json:"duplicate_of_situation_id,omitempty"`
}

// CRMSituationWorkState snapshots only lifecycle-controlled facts. Source priority,
// evidence and canonical action results remain owned by their respective systems.
type CRMSituationWorkState struct {
	OwnerMemberID             *string                        `json:"owner_member_id"`
	NextActionOwnerMemberID   *string                        `json:"next_action_owner_member_id"`
	Lifecycle                 string                         `json:"lifecycle"`
	Attention                 string                         `json:"attention"`
	NextStep                  string                         `json:"next_step"`
	NextCheckpointAt          *time.Time                     `json:"next_checkpoint_at"`
	PlaybookID                *string                        `json:"playbook_id,omitempty"`
	PlaybookVersionID         *string                        `json:"playbook_version_id,omitempty"`
	PlaybookAppliedByMemberID *string                        `json:"playbook_applied_by_member_id,omitempty"`
	PlaybookAppliedAt         *time.Time                     `json:"playbook_applied_at,omitempty"`
	PlaybookMilestones        []CRMPlaybookMilestoneProgress `json:"playbook_milestones,omitempty"`
	OutcomeKind               *string                        `json:"outcome_kind"`
	OutcomeSummary            *string                        `json:"outcome_summary"`
	OutcomeBasis              *string                        `json:"outcome_basis"`
	DuplicateOfSituationID    *string                        `json:"duplicate_of_situation_id"`
	ClosedAt                  *time.Time                     `json:"closed_at"`
	ClosedByMemberID          *string                        `json:"closed_by_member_id"`
}

// CRMSituationState returns a snapshot without copying customer or execution payloads.
func CRMSituationState(s CRMSituation) CRMSituationWorkState {
	return CRMSituationWorkState{
		OwnerMemberID: s.OwnerMemberID, NextActionOwnerMemberID: s.NextActionOwnerMemberID,
		Lifecycle: s.Lifecycle, Attention: s.Attention, NextStep: s.NextStep,
		NextCheckpointAt: s.NextCheckpointAt, OutcomeKind: s.OutcomeKind,
		PlaybookID: s.PlaybookID, PlaybookVersionID: s.PlaybookVersionID,
		PlaybookAppliedByMemberID: s.PlaybookAppliedByMemberID, PlaybookAppliedAt: s.PlaybookAppliedAt,
		PlaybookMilestones: append([]CRMPlaybookMilestoneProgress(nil), s.PlaybookMilestones...),
		OutcomeSummary:     s.OutcomeSummary, OutcomeBasis: s.OutcomeBasis,
		DuplicateOfSituationID: s.DuplicateOfSituationID,
		ClosedAt:               s.ClosedAt, ClosedByMemberID: s.ClosedByMemberID,
	}
}

// CRMSituationChange is append-only history and the durable receipt for one command.
// ActorKind is member, signal or suggestion; imported work never invents a human actor.
type CRMSituationChange struct {
	ID                  string                 `json:"id" gorm:"type:uuid;primaryKey"`
	WorkspaceID         string                 `json:"-" gorm:"type:uuid;not null"`
	SituationID         string                 `json:"situation_id" gorm:"type:uuid;not null"`
	Revision            int64                  `json:"revision" gorm:"not null"`
	CommandKey          string                 `json:"-" gorm:"not null"`
	CommandFingerprint  string                 `json:"-" gorm:"not null"`
	Operation           string                 `json:"operation" gorm:"not null"`
	ActorKind           string                 `json:"actor_kind" gorm:"not null"`
	ActorMemberID       *string                `json:"actor_member_id" gorm:"type:uuid"`
	Reason              string                 `json:"reason" gorm:"not null"`
	Before              *CRMSituationWorkState `json:"before" gorm:"serializer:json;type:jsonb"`
	After               CRMSituationWorkState  `json:"after" gorm:"serializer:json;type:jsonb;not null"`
	InFlightActionCount int64                  `json:"in_flight_action_count" gorm:"not null"`
	CreatedAt           time.Time              `json:"created_at" gorm:"autoCreateTime"`
}

// TableName identifies the lifecycle history and command-receipt table, not a runtime.
func (CRMSituationChange) TableName() string { return "crm_situation_changes" }

// CRMSituationCommandResult returns the original committed change on retry.
// Its snapshot may precede newer changes; callers refetch detail for current state.
type CRMSituationCommandResult struct {
	Change   CRMSituationChange `json:"change"`
	Replayed bool               `json:"replayed"`
}

// CRMSituationHistory is newest-first keyset pagination of immutable work changes.
type CRMSituationHistory struct {
	Data               []CRMSituationChange `json:"data"`
	NextBeforeRevision *int64               `json:"next_before_revision"`
}
