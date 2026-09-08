package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrCRMPlaybookExecutionNotReady prevents ordinary launch/approval paths from
// treating published instructions as permission to execute customer work.
var ErrCRMPlaybookExecutionNotReady = errors.New("Manage this playbook run and its approvals from CRM Signals")

// ErrCRMPlaybookRunScopeConflict prevents one customer objective from borrowing another's run.
var ErrCRMPlaybookRunScopeConflict = errors.New("this run belongs to different customer work")

func crmPlaybookInput(data json.RawMessage) (*model.CRMPlaybookRunContext, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var input struct {
		CRMPlaybook *model.CRMPlaybookRunContext `json:"crm_playbook"`
	}
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("invalid Agent run context: %w", err)
	}
	return input.CRMPlaybook, nil
}

// No existing generic launch/resume route has a durable Playbook run claim or
// action authorization. Reject the reserved context before metering or writes.
// Bound dispatch has a separate, server-authorized entry boundary;
// do not replace this with a caller-provided enabled/approved boolean.
func rejectUnclaimedCRMPlaybookRun(data json.RawMessage) error {
	crm, err := crmPlaybookInput(data)
	if err != nil {
		return err
	}
	if crm != nil {
		return ErrCRMPlaybookExecutionNotReady
	}
	return nil
}

func crmPlaybookRunScopesMatch(existing, requested json.RawMessage) (bool, error) {
	left, err := crmPlaybookInput(existing)
	if err != nil {
		return false, err
	}
	right, err := crmPlaybookInput(requested)
	if err != nil {
		return false, err
	}
	if left == nil || right == nil {
		return left == nil && right == nil, nil
	}
	// A run is one evaluation, not a reusable customer chat. New revisions need
	// a fresh durable claim; equality never grants authority to start/resume.
	if left.WorkspaceID == "" || left.SituationID == "" || left.ConnectionID == "" {
		return false, nil
	}
	return left.WorkspaceID == right.WorkspaceID && left.PlaybookID == right.PlaybookID &&
		left.PlaybookVersionID == right.PlaybookVersionID && left.SituationID == right.SituationID &&
		left.SituationRevision == right.SituationRevision && left.ConnectionID == right.ConnectionID &&
		left.ConnectionVersion == right.ConnectionVersion && left.ConnectionFingerprint == right.ConnectionFingerprint &&
		left.SpecializationVersion == right.SpecializationVersion && left.Target == right.Target, nil
}

// crmPlaybookRuntimeProfileID addresses immutable runtime configuration, not a
// new Helpin Agent. Include workspace and fingerprint so neither another tenant
// nor a later publication can overwrite Beacon's reviewed runtime definition.
func crmPlaybookRuntimeProfileID(connection model.CRMPlaybookConnection) string {
	identity := strings.Join([]string{"helpin.crm.playbook.profile.v1", connection.WorkspaceID, connection.ID, connection.Fingerprint}, ":")
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(identity)).String()
}
