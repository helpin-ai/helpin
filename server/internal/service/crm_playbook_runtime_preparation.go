package service

import (
	"encoding/json"
	"slices"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// This is an internal preparation result, deliberately not a launch request or
// endpoint. Runtime skill delivery, live authorization/billing and action guards
// must be connected before any of its contents can be used to execute work.
type crmPlaybookRuntimePreparation struct {
	agent            AgentRuntimeAgent
	helpinAgentID    string
	input            model.AgentRunInputPayload
	packages         []crmPlaybookRuntimePackage
	executionEnabled bool
}

type crmPlaybookRuntimePackage struct {
	key      string
	version  string
	archive  []byte
	checksum string
}

func prepareCRMPlaybookConnectionRuntime(connection model.CRMPlaybookConnection, policy model.CRMPlaybookVersion,
	item model.CRMSituationItem, req model.CRMPlaybookContextRequest, appID string,
) (*crmPlaybookRuntimePreparation, error) {
	snapshot := connection.Snapshot
	fingerprint, err := connectionContentFingerprint(snapshot)
	if err != nil {
		return nil, err
	}
	if !validSituationID(connection.ID) || connection.ExecutionEnabled || connection.Version < 1 || (snapshot.SchemaVersion != 1 && snapshot.SchemaVersion != 2) ||
		fingerprint != connection.Fingerprint || snapshot.WorkspaceID != req.WorkspaceID || connection.WorkspaceID != req.WorkspaceID ||
		connection.PlaybookID != req.PlaybookID || snapshot.PlaybookID != req.PlaybookID ||
		connection.PlaybookVersionID != req.PlaybookVersionID || snapshot.PolicyVersionID != req.PlaybookVersionID ||
		snapshot.PolicyFingerprint != policy.Fingerprint {
		return nil, agentcontract.ErrCRMPlaybookContextMismatch
	}
	crm, err := agentcontract.PrepareCRMPlaybookContext(req, policy, item, snapshot.Specialization)
	if err != nil {
		return nil, err
	}
	var runtime AgentRuntimeAgent
	if err := json.Unmarshal(snapshot.RuntimeAgent, &runtime); err != nil {
		return nil, err
	}
	var action model.ActionConfigRunAgent
	if err := json.Unmarshal(snapshot.Flow.ActionConfig, &action); err != nil {
		return nil, err
	}
	if runtime.ID != snapshot.Agent.ID || runtime.SystemPrompt != snapshot.Specialization.PresetPrompt ||
		action.AgentID != runtime.ID || (action.TargetType != req.Target.TargetType && !(snapshot.SchemaVersion == 2 && action.TargetType == "crm_record")) ||
		(action.TargetID != "" && action.TargetID != req.Target.TargetID) ||
		!slices.Contains(runtime.AllowedTargets, req.Target.TargetType) {
		return nil, agentcontract.ErrCRMPlaybookContextMismatch
	}
	// Read the frozen runtime projection; never recompile it against today's presets.
	runtime.AppID = appID
	// Runtime configuration has its own immutable identity. The durable Helpin
	// AgentRun must retain the existing Beacon ID for access, billing and Activity.
	runtime.ID = crmPlaybookRuntimeProfileID(connection)
	crm.ConnectionID, crm.ConnectionVersion = connection.ID, connection.Version
	crm.ConnectionFingerprint = connection.Fingerprint
	result := &crmPlaybookRuntimePreparation{agent: runtime, helpinAgentID: snapshot.Agent.ID, input: model.AgentRunInputPayload{CRMPlaybook: crm, Target: &crm.Target}}
	if action.AdditionalContext != nil {
		result.input.AdditionalContext = *action.AdditionalContext
	}
	for _, skill := range snapshot.Specialization.Skills {
		archive, checksum, err := agentcontract.BuildCRMPlaybookSkillArchive(skill)
		if err != nil {
			return nil, err
		}
		result.packages = append(result.packages, crmPlaybookRuntimePackage{key: skill.Key, version: skill.Version, archive: archive, checksum: checksum})
	}
	return result, nil
}
