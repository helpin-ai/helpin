package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrCRMPlaybookConnectionUnsupported rejects settings that cannot be frozen safely.
var ErrCRMPlaybookConnectionUnsupported = errors.New("Playbook connection settings are not supported")

// ReviewConnection previews actual saved Flow/Beacon settings under both modules' permissions.
func (s *CRMPlaybookService) ReviewConnection(ctx context.Context, ws, id string, selection model.CRMPlaybookConnectionSelection) (*model.CRMPlaybookConnectionReview, error) {
	if _, err := s.authorizeConnection(ctx, ws); err != nil {
		return nil, err
	}
	if !validConnectionSelection(id, selection) {
		return nil, ErrCRMPlaybookInput
	}
	source, err := s.store.ConnectionSource(ctx, ws, id, selection)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, ErrCRMPlaybookNotFound
	}
	snapshot, fingerprint, err := compileCRMPlaybookConnection(*source)
	if err != nil {
		return nil, err
	}
	result := &model.CRMPlaybookConnectionReview{PlaybookVersionID: selection.PlaybookVersionID, ExpectedRevision: selection.ExpectedRevision, FlowID: selection.FlowID, AgentID: selection.AgentID, ConnectionVersion: source.ConnectionVersion,
		ReviewFingerprint: fingerprint, FlowName: source.Flow.Name, AgentName: source.Agent.Name, RuntimeKind: snapshot.Agent.RuntimeKind,
		Explanation: "Publishing saves the reviewed setup. It starts no work and does not change existing signals; activation is a separate confirmation."}
	if result.RuntimeKind == "" {
		result.RuntimeKind = "native_sdk"
	}
	for _, skill := range snapshot.Specialization.Skills {
		result.Skills = append(result.Skills, model.CRMPlaybookSkillSummary{Key: skill.Key, Title: skill.Title, Role: skill.Role, Version: skill.Version})
	}
	return result, nil
}

// PublishConnection saves the reviewed setup without enabling or editing the selected automation.
func (s *CRMPlaybookService) PublishConnection(ctx context.Context, ws, id string, req model.PublishCRMPlaybookConnectionRequest) (*model.CRMPlaybookConnectionResult, error) {
	actor, err := s.authorizeConnection(ctx, ws)
	if err != nil {
		return nil, err
	}
	if !validConnectionSelection(id, req.Selection()) || !validPlaybookKey(req.CommandKey) || req.ExpectedConnectionVersion < 0 || !validConnectionFingerprint(req.ReviewFingerprint) {
		return nil, ErrCRMPlaybookInput
	}
	fingerprint, err := playbookFingerprint(actor.WorkspaceMemberID, req)
	if err != nil {
		return nil, err
	}
	result, err := s.store.PublishConnection(ctx, ws, id, actor.WorkspaceMemberID, req, fingerprint, compileCRMPlaybookConnection)
	if err == nil && result == nil {
		return nil, ErrCRMPlaybookNotFound
	}
	return result, err
}

// Connection reads an exact publication receipt; private execution settings stay server-side.
func (s *CRMPlaybookService) Connection(ctx context.Context, ws, id, connectionID string) (*model.CRMPlaybookConnection, error) {
	if _, err := s.authorizeConnection(ctx, ws); err != nil {
		return nil, err
	}
	if !validSituationID(id) || !validSituationID(connectionID) {
		return nil, ErrCRMPlaybookInput
	}
	result, err := s.store.Connection(ctx, ws, id, connectionID)
	if err == nil && result == nil {
		return nil, ErrCRMPlaybookNotFound
	}
	return result, err
}

type crmPlaybookModuleAuthorizer interface {
	CanAccessModule(context.Context, *authorization.Actor, model.ModuleID) (bool, error)
}

// Connections reads a paginated publication history without loading private packages.
func (s *CRMPlaybookService) Connections(ctx context.Context, ws, id string, before int64, limit int) (*model.CRMPlaybookConnections, error) {
	if _, err := s.authorizeConnection(ctx, ws); err != nil {
		return nil, err
	}
	if _, err := s.Get(ctx, ws, id); err != nil {
		return nil, err
	}
	if before < 0 {
		return nil, ErrCRMPlaybookInput
	}
	_, limit, err := playbookPage(1, limit)
	if err != nil {
		return nil, err
	}
	return s.store.Connections(ctx, ws, id, before, limit)
}

func (s *CRMPlaybookService) authorizeConnection(ctx context.Context, ws string) (*authorization.Actor, error) {
	actor, err := s.authorize(ctx, ws, authorization.PermCRMAdmin)
	if err != nil {
		return nil, err
	}
	if !s.authz.Can(actor, authorization.PermPMAdminAutomations) {
		return nil, ErrCRMPlaybookForbidden
	}
	modules, ok := s.authz.(crmPlaybookModuleAuthorizer)
	if !ok {
		return nil, ErrCRMPlaybookForbidden
	}
	for _, module := range []model.ModuleID{model.ModuleCRM, model.ModuleAutomation} {
		allowed, err := modules.CanAccessModule(ctx, actor, module)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, ErrCRMPlaybookForbidden
		}
	}
	return actor, nil
}

func validConnectionSelection(id string, s model.CRMPlaybookConnectionSelection) bool {
	return validSituationID(id) && validSituationID(s.PlaybookVersionID) && validSituationID(s.FlowID) && validSituationID(s.AgentID) && s.ExpectedRevision > 0
}

func validConnectionFingerprint(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}

func compileCRMPlaybookConnection(source model.CRMPlaybookConnectionSource) (model.CRMPlaybookConnectionSnapshot, string, error) {
	var snapshot model.CRMPlaybookConnectionSnapshot
	// Own copies before normalization; never mutate the loaded/saved Agent or Flow.
	encoded, err := json.Marshal(struct {
		Agent model.Agent
		Flow  model.AutomationRule
	}{source.Agent, source.Flow})
	if err != nil {
		return snapshot, "", err
	}
	var copied struct {
		Agent model.Agent
		Flow  model.AutomationRule
	}
	if err := json.Unmarshal(encoded, &copied); err != nil {
		return snapshot, "", err
	}
	source.Agent, source.Flow = copied.Agent, copied.Flow
	return compileCRMPlaybookConnectionCopy(source)
}

func connectionContentFingerprint(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	// PostgreSQL jsonb changes object-key order, including RawMessage fields.
	// Canonicalize nested JSON without rounding integer-valued configuration.
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var canonical any
	if err := decoder.Decode(&canonical); err != nil {
		return "", err
	}
	encoded, err = json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func connectionPolicyFingerprint(definition model.CRMPlaybookDefinition) (string, error) {
	// Preserve the existing immutable business-policy hash contract.
	encoded, err := json.Marshal(definition)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func compileCRMPlaybookConnectionCopy(source model.CRMPlaybookConnectionSource) (model.CRMPlaybookConnectionSnapshot, string, error) {
	var snapshot model.CRMPlaybookConnectionSnapshot
	policyHash, err := connectionPolicyFingerprint(source.Policy.Definition)
	if err != nil {
		return snapshot, "", err
	}
	if source.Policy.Fingerprint != policyHash || source.Policy.WorkspaceID != source.Playbook.WorkspaceID ||
		source.Agent.WorkspaceID != source.Playbook.WorkspaceID || source.Flow.WorkspaceID != source.Playbook.WorkspaceID ||
		source.Policy.PlaybookID != source.Playbook.ID {
		return snapshot, "", ErrCRMPlaybookConnectionUnsupported
	}
	if !source.Agent.IsSystem || source.Agent.EffectivePresetKey() != model.AgentPresetCRMOperator || source.Flow.Enabled ||
		(source.Flow.ActionType != model.ActionStartAgentRun && source.Flow.ActionType != model.ActionRunAgent) {
		return snapshot, "", ErrCRMPlaybookConnectionUnsupported
	}
	var action model.ActionConfigRunAgent
	if json.Unmarshal(source.Flow.ActionConfig, &action) != nil || action.AgentID != source.Agent.ID || action.Output != nil || action.BaseBranch != "" || action.WorkingBranch != "" {
		return snapshot, "", ErrCRMPlaybookConnectionUnsupported
	}
	switch action.TargetType {
	case "crm_deal", "crm_contact", "crm_company":
	case "crm_record":
		if source.Flow.TriggerType != model.CRMPlaybookWorkDue || action.TargetID != "" {
			return snapshot, "", ErrCRMPlaybookConnectionUnsupported
		}
	default:
		return snapshot, "", ErrCRMPlaybookConnectionUnsupported
	}
	if action.TargetID != "" && !validSituationID(action.TargetID) {
		return snapshot, "", ErrCRMPlaybookConnectionUnsupported
	}
	agent, flow := source.Agent, source.Flow
	agent.TokensUsedThisMonth, agent.TokensUsedTotal = 0, 0
	agent.ActiveTaskID, agent.Status = nil, ""
	agent.CreatedAt, agent.UpdatedAt = time.Time{}, time.Time{}
	flow.CreatedAt, flow.UpdatedAt = time.Time{}, time.Time{}
	materializeAgentSystemPrompt(&agent)
	runtime := runtimeAgentFromHelpinAgent(&agent, "")
	// Optional skill references are mutable dependencies until their exact package
	// transport is connected. Do not falsely call an unresolved reference frozen.
	if len(runtime.Skills) != 0 || runtime.RuntimeKind != "native_sdk" {
		return snapshot, "", ErrCRMPlaybookConnectionUnsupported
	}
	specialization, err := agentcontract.CaptureCRMPlaybookSpecializationForPrompt(source.Policy.Definition.Journey, runtime.SystemPrompt)
	if err != nil {
		return snapshot, "", ErrCRMPlaybookConnectionUnsupported
	}
	schemaVersion := 1
	if flow.TriggerType == model.CRMPlaybookWorkDue {
		if source.Policy.Definition.Journey == "sales_handoff" && !slices.ContainsFunc(source.Policy.Definition.Milestones, func(m model.CRMPlaybookMilestone) bool { return m.Key == "handoff_accepted" }) {
			return snapshot, "", ErrCRMPlaybookConnectionUnsupported
		}
		schemaVersion = 2
		runtime.ApprovalMode = "never" // Only preparation/proposal tools; CRM owns exact action approval.
		runtime.AllowedTools = crmPlaybookRuntimeTools()
		for _, skill := range specialization.Skills {
			runtime.Skills = append(runtime.Skills, AgentRuntimeSkillRef{Key: skill.Key, VersionKey: skill.Version})
		}
	}
	payload, err := json.Marshal(runtime)
	if err != nil {
		return snapshot, "", err
	}
	snapshot = model.CRMPlaybookConnectionSnapshot{SchemaVersion: schemaVersion, WorkspaceID: agent.WorkspaceID, PlaybookID: source.Playbook.ID,
		PlaybookRevision: source.Playbook.Revision, PolicyVersionID: source.Policy.ID, PolicyFingerprint: policyHash,
		Flow: flow, Agent: agent, RuntimeAgent: payload, Specialization: specialization}
	fingerprint, err := connectionContentFingerprint(snapshot)
	return snapshot, fingerprint, err
}
