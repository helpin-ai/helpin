package agentcontract

import (
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrCRMPlaybookContextMismatch rejects foreign, stale or incorrectly bound process context.
var ErrCRMPlaybookContextMismatch = errors.New("Playbook context does not match the bound customer process")

// ErrCRMPlaybookContextBlocked rejects paused/closed work and unavailable responsibility.
var ErrCRMPlaybookContextBlocked = errors.New("Playbook customer work is not ready for preparation")

// PrepareCRMPlaybookContext copies minimal context from already-authorized CRM records.
// Callers must load pinned policy/skills from trusted storage and recheck current
// target access, activation, permissions, budgets and action state before any run
// or side effect. This pure preparer never launches, schedules or records progress.
func PrepareCRMPlaybookContext(
	req model.CRMPlaybookContextRequest,
	version model.CRMPlaybookVersion,
	item model.CRMSituationItem,
	specialization model.CRMPlaybookSpecialization,
) (*model.CRMPlaybookRunContext, error) {
	if err := ValidateCRMPlaybookSpecialization(specialization); err != nil {
		return nil, err
	}
	work := item.Situation
	if !crmPlaybookContextMatches(req, version, work, specialization) {
		return nil, ErrCRMPlaybookContextMismatch
	}
	policyVersion, err := crmContentVersion(version.Definition)
	if err != nil {
		return nil, err
	}
	if policyVersion != version.Fingerprint {
		return nil, ErrCRMPlaybookContextMismatch
	}
	if work.Lifecycle != model.CRMSituationOpen || work.ClosedAt != nil || work.OutcomeKind != nil ||
		work.DuplicateOfSituationID != nil || !item.OwnerAvailable ||
		!validCRMContextPointer(work.OwnerMemberID) || strings.TrimSpace(work.Objective) == "" ||
		strings.TrimSpace(version.Definition.Objective) == "" {
		return nil, ErrCRMPlaybookContextBlocked
	}
	if work.NextActionOwnerMemberID != nil &&
		(!item.NextActionOwnerAvailable || !validCRMContextPointer(work.NextActionOwnerMemberID)) {
		return nil, ErrCRMPlaybookContextBlocked
	}
	result := &model.CRMPlaybookRunContext{
		SchemaVersion: 1, WorkspaceID: req.WorkspaceID, PlaybookID: version.PlaybookID,
		PlaybookVersionID: version.ID, SituationID: work.ID, SituationRevision: work.Revision,
		SpecializationVersion: specialization.Version, Journey: specialization.Journey, Target: req.Target,
		PlaybookObjective: version.Definition.Objective, CustomerObjective: work.Objective,
		OwnerMemberID: *work.OwnerMemberID, NextStep: work.NextStep,
	}
	if work.NextActionOwnerMemberID != nil {
		result.NextActionOwnerID = *work.NextActionOwnerMemberID
	}
	if work.NextCheckpointAt != nil {
		checkpoint := *work.NextCheckpointAt
		result.NextCheckpointAt = &checkpoint
	}
	return result, nil
}

func crmPlaybookContextMatches(req model.CRMPlaybookContextRequest, version model.CRMPlaybookVersion,
	work model.CRMSituation, specialization model.CRMPlaybookSpecialization,
) bool {
	for _, id := range []string{req.WorkspaceID, req.PlaybookID, req.PlaybookVersionID, req.SituationID, req.Target.TargetID} {
		if !validCRMContextID(id) {
			return false
		}
	}
	if version.WorkspaceID != req.WorkspaceID || work.WorkspaceID != req.WorkspaceID ||
		version.PlaybookID != req.PlaybookID || version.ID != req.PlaybookVersionID || version.Version < 1 ||
		work.ID != req.SituationID || work.PlaybookID == nil || *work.PlaybookID != version.PlaybookID ||
		work.PlaybookVersionID == nil || *work.PlaybookVersionID != version.ID ||
		req.ExpectedSituationRevision < 1 || work.Revision != req.ExpectedSituationRevision ||
		req.SpecializationVersion != specialization.Version || version.Definition.Journey != specialization.Journey {
		return false
	}
	var targetID *string
	switch req.Target.TargetType {
	case "crm_deal":
		targetID = work.DealID
	case "crm_contact":
		targetID = work.ContactID
	case "crm_company":
		targetID = work.CompanyID
	default:
		return false
	}
	return targetID != nil && *targetID == req.Target.TargetID
}

func validCRMContextID(id string) bool {
	parsed, err := uuid.Parse(id)
	return err == nil && parsed != uuid.Nil && parsed.String() == id
}

func validCRMContextPointer(id *string) bool {
	return id != nil && validCRMContextID(*id)
}
