package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	agentruntime "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const crmPlaybookPackagePrefix = "crm-playbooks/"

type crmPlaybookHostScope struct {
	run     *model.AgentRun
	binding *model.AutomationRunBinding
	source  *model.CRMPlaybookExecutionSource
	actor   *authorization.Actor
}

// SetCRMPlaybookExecution enables only the explicitly guarded Playbook callback surfaces.
func (s *AgentRuntimeHostService) SetCRMPlaybookExecution(execution *CRMPlaybookExecutionService) {
	s.playbookExecution = execution
}

// resolveCRMPlaybookCallback distinguishes ordinary runs from bound work using
// durable host state. Metadata may narrow the identity, never grant authority.
func (s *AgentRuntimeHostService) resolveCRMPlaybookCallback(ctx context.Context, remoteID, agentID string, target agentruntime.TargetRef, metadata ...map[string]interface{}) (*crmPlaybookHostScope, error) {
	var run *model.AgentRun
	var err error
	hint := runtimeMetadataString("helpin_run_id", metadata...)
	marked := false
	for _, values := range metadata {
		for _, key := range []string{"crm_playbook", "crm_playbook_connection_id", "crm_playbook_situation_id"} {
			if _, exists := values[key]; exists {
				marked = true
			}
		}
		if value, ok := values["helpin_run_id"]; ok && value != hint {
			return nil, ErrAgentRuntimeHostForbidden
		}
	}
	if s.runRepo != nil && remoteID != "" {
		run, err = s.runRepo.GetByExternalRuntimeID(ctx, agentRuntimeName, remoteID)
		if err != nil {
			return nil, err
		}
	}
	if s.runRepo != nil && hint != "" {
		if run != nil && run.ID != hint {
			return nil, ErrAgentRuntimeHostForbidden
		}
		if run == nil {
			run, err = s.runRepo.GetByIDAny(ctx, hint)
			if err != nil {
				return nil, err
			}
		}
	}
	if run == nil {
		if s.playbookExecution != nil && agentID != "" {
			bound, err := s.playbookExecution.store.HasRuntimeProfile(ctx, agentID)
			if err != nil {
				return nil, err
			}
			if bound {
				return nil, ErrAgentRuntimeHostForbidden
			}
		}
		if marked {
			return nil, ErrAgentRuntimeHostForbidden
		}
		return nil, nil
	}
	crm, err := crmPlaybookInput(run.Input)
	if err != nil {
		return nil, ErrAgentRuntimeHostForbidden
	}
	if crm == nil {
		if marked {
			return nil, ErrAgentRuntimeHostForbidden
		}
		return nil, nil
	}
	if s.playbookExecution == nil || s.agentRepo == nil || remoteID == "" || !model.IsAgentRunActiveStatus(run.Status) || run.Status == model.AgentRunStatusPaused {
		return nil, ErrAgentRuntimeHostForbidden
	}
	if mapped, exists := agentRuntimeRunID(run); exists && mapped != remoteID {
		return nil, ErrAgentRuntimeHostForbidden
	}
	binding, source, err := s.playbookExecution.AuthorizeRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return nil, fmt.Errorf("%w: Playbook work is no longer authorized", ErrAgentRuntimeHostForbidden)
	}
	if binding.Input.CRMPlaybook == nil || source.Connection.Snapshot.SchemaVersion != 2 || binding.RuntimeProfileID != agentID ||
		run.AgentID != binding.AgentID || run.TargetType != target.Type || run.TargetID != target.ID {
		return nil, ErrAgentRuntimeHostForbidden
	}
	stored, err := json.Marshal(binding.Input)
	if err != nil {
		return nil, err
	}
	if matches, err := crmPlaybookRunScopesMatch(run.Input, stored); err != nil || !matches {
		return nil, ErrAgentRuntimeHostForbidden
	}
	for _, values := range metadata {
		for key, expected := range map[string]string{"workspace_id": run.WorkspaceID, "crm_playbook_connection_id": binding.ConnectionID, "crm_playbook_situation_id": binding.SituationID, "helpin_agent_id": binding.AgentID} {
			if actual, exists := values[key]; exists && actual != expected {
				return nil, ErrAgentRuntimeHostForbidden
			}
		}
	}
	live, err := s.agentRepo.GetByID(ctx, run.WorkspaceID, binding.AgentID)
	if err != nil {
		return nil, err
	}
	if err := validateLivePlaybookAgent(live, source.Connection.Snapshot.Agent, crm.Target); err != nil {
		return nil, ErrAgentRuntimeHostForbidden
	}
	actor, err := s.playbookExecution.liveActor(ctx, run.WorkspaceID, source.Binding.AuthorizedByMemberID)
	if err != nil {
		return nil, ErrAgentRuntimeHostForbidden
	}
	return &crmPlaybookHostScope{run: run, binding: binding, source: source, actor: actor}, nil
}

func (s *AgentRuntimeHostService) resolveCRMPlaybookSkill(ctx context.Context, req AgentRuntimeSkillLookupRequest) (*AgentRuntimeWorkspaceSkill, bool, error) {
	scope, err := s.resolveCRMPlaybookCallback(ctx, req.RunID, runtimeAgentBaseID(req.AgentID), req.Target, req.Metadata, req.Target.Metadata)
	if err != nil || scope == nil {
		return nil, err != nil, err
	}
	key := strings.TrimSpace(req.Key)
	if req.SkillID != "" {
		key = strings.TrimPrefix(req.SkillID, "helpin_playbook:"+scope.source.Connection.ID+":")
	}
	for _, skill := range scope.source.Connection.Snapshot.Specialization.Skills {
		if skill.Key == key {
			result, err := crmPlaybookWorkspaceSkill(*scope.binding, skill)
			return result, true, err
		}
	}
	return nil, true, ErrAgentRuntimeHostNotFound
}

func crmPlaybookWorkspaceSkill(binding model.AutomationRunBinding, skill model.CRMPlaybookSkillSnapshot) (*AgentRuntimeWorkspaceSkill, error) {
	archive, checksum, err := agentcontract.BuildCRMPlaybookSkillArchive(skill)
	if err != nil {
		return nil, err
	}
	var instructions string
	for _, file := range skill.Files {
		if file.Path == "SKILL.md" {
			instructions = string(file.Data)
		}
	}
	return &AgentRuntimeWorkspaceSkill{ID: "helpin_playbook:" + binding.ConnectionID + ":" + skill.Key,
		Key: skill.Key, VersionKey: skill.Version, Title: skill.Title, SourceKind: model.WorkspaceSkillSourceBuiltIn,
		Instructions: instructions, RequiredTools: crmPlaybookRuntimeTools(), SupportedRuntimes: []string{"native_sdk", "codex"},
		PackageObjectKey: crmPlaybookPackagePrefix + binding.WorkspaceID + "/" + binding.RunID + "/" + skill.Key + "/" + skill.Version + ".zip",
		PackageFileName:  skill.Key + ".zip", PackageChecksum: checksum, PackageSize: int64(len(archive))}, nil
}

func (s *AgentRuntimeHostService) crmPlaybookPackage(ctx context.Context, key string) ([]byte, error) {
	parts := strings.Split(strings.TrimPrefix(key, crmPlaybookPackagePrefix), "/")
	if len(parts) != 4 || !validSituationID(parts[0]) || !validSituationID(parts[1]) || s.playbookExecution == nil || s.runRepo == nil {
		return nil, ErrAgentRuntimeHostForbidden
	}
	run, err := s.runRepo.GetByID(ctx, parts[0], parts[1])
	if err != nil {
		return nil, err
	}
	if run == nil || !model.IsAgentRunActiveStatus(run.Status) || run.Status == model.AgentRunStatusPaused {
		return nil, ErrAgentRuntimeHostForbidden
	}
	binding, source, err := s.playbookExecution.AuthorizeRun(ctx, parts[0], parts[1])
	if err != nil {
		return nil, ErrAgentRuntimeHostForbidden
	}
	if s.agentRepo == nil || binding.Input.CRMPlaybook == nil {
		return nil, ErrAgentRuntimeHostForbidden
	}
	live, err := s.agentRepo.GetByID(ctx, binding.WorkspaceID, binding.AgentID)
	if err != nil {
		return nil, err
	}
	if err := validateLivePlaybookAgent(live, source.Connection.Snapshot.Agent, binding.Input.CRMPlaybook.Target); err != nil {
		return nil, ErrAgentRuntimeHostForbidden
	}
	for _, skill := range source.Connection.Snapshot.Specialization.Skills {
		if skill.Key == parts[2] && skill.Version+".zip" == parts[3] {
			resolved, err := crmPlaybookWorkspaceSkill(*binding, skill)
			if err != nil || resolved.PackageObjectKey != key {
				return nil, ErrAgentRuntimeHostForbidden
			}
			archive, _, err := agentcontract.BuildCRMPlaybookSkillArchive(skill)
			return archive, err
		}
	}
	return nil, ErrAgentRuntimeHostNotFound
}

func (s *AgentRuntimeHostService) executeCRMPlaybookCommand(ctx context.Context, req agentruntime.CommandExecutionRequest, scope *crmPlaybookHostScope) (*agentruntime.CommandExecutionResponse, error) {
	if req.Meta.WorkspaceID != "" && req.Meta.WorkspaceID != scope.run.WorkspaceID {
		return nil, ErrAgentRuntimeHostForbidden
	}
	if req.Meta.ExternalActorID != "" && req.Meta.ExternalActorID != scope.actor.UserID {
		return nil, ErrAgentRuntimeHostForbidden
	}
	if req.CommandName != "crm.get_playbook_context" && req.CommandName != "crm.propose_playbook_action" {
		return nil, ErrAgentRuntimeHostForbidden
	}
	meta := model.InternalCommandContext{WorkspaceID: scope.run.WorkspaceID, ActorID: scope.actor.UserID, ActorRole: scope.actor.Role,
		ActorTeamIDs: scope.actor.TeamIDs(), AgentID: scope.run.AgentID, RunID: scope.run.ID, TargetType: scope.run.TargetType, TargetID: scope.run.TargetID}
	if err := s.enrichCommandAgentScope(ctx, &meta); err != nil {
		return nil, err
	}
	output, err := s.commandService.Execute(authorization.WithActor(ctx, scope.actor), meta, req.CommandName, req.Input)
	if err != nil {
		return &agentruntime.CommandExecutionResponse{Error: "Playbook action could not be prepared. Refresh context or ask the Signal owner to review."}, nil
	}
	return &agentruntime.CommandExecutionResponse{Output: output}, nil
}
