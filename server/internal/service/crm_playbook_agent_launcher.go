package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// ErrCRMPlaybookLaunchUncertain requires correlation/recovery rather than another launch.
var ErrCRMPlaybookLaunchUncertain = errors.New("Beacon start needs checking")

// CRMPlaybookAgentLauncher adapts approved customer work to the existing Agent Runtime.
// It creates normal agent_runs and uses the same preflight, client and cancellation.
type CRMPlaybookAgentLauncher struct {
	agents    *AgentService
	store     *repository.CRMPlaybookExecutionRepository
	execution *CRMPlaybookExecutionService
	meter     *AIUsageMeter
}

// NewCRMPlaybookAgentLauncher preserves the shared Helpin Agent's saved configuration.
func NewCRMPlaybookAgentLauncher(agents *AgentService, store *repository.CRMPlaybookExecutionRepository, meter *AIUsageMeter) *CRMPlaybookAgentLauncher {
	return &CRMPlaybookAgentLauncher{agents: agents, store: store, meter: meter}
}

// SetExecutionService supplies the live boundary after explicit dependency construction.
func (s *CRMPlaybookAgentLauncher) SetExecutionService(execution *CRMPlaybookExecutionService) {
	s.execution = execution
}

// PlaybookRuntimeReady reports infrastructure availability, not customer activation.
func (s *CRMPlaybookAgentLauncher) PlaybookRuntimeReady() bool {
	if s == nil || s.agents == nil || s.agents.agentRuntimeClient == nil || s.meter == nil {
		return false
	}
	_, ok := s.agents.agentRuntimeClient.(agentRuntimeLaunchClient)
	return ok && s.agents.agentRuntimeLaunchEnabled
}

// StartPlaybookRun reloads the server receipt; supplied input never grants authority.
func (s *CRMPlaybookAgentLauncher) StartPlaybookRun(ctx context.Context, requested model.AutomationRunBinding) (*model.AgentRun, error) {
	if !s.PlaybookRuntimeReady() || s.execution == nil {
		return nil, ErrCRMPlaybookRuntimeUnavailable
	}
	binding, source, err := s.execution.AuthorizeRun(ctx, requested.WorkspaceID, requested.RunID)
	if err != nil {
		return nil, err
	}
	client := s.agents.agentRuntimeClient.(agentRuntimeLaunchClient)
	prepared, err := prepareCRMPlaybookExecution(*source, client.AppID())
	if err != nil {
		return nil, err
	}
	if source.Connection.Snapshot.SchemaVersion != 2 || binding.RuntimeProfileID != prepared.agent.ID {
		return nil, ErrCRMPlaybookConnectionUnsupported
	}
	live, err := s.agents.agentRepo.GetByID(ctx, binding.WorkspaceID, binding.AgentID)
	if err != nil {
		return nil, err
	}
	if err := validateLivePlaybookAgent(live, source.Connection.Snapshot.Agent, binding.Input.CRMPlaybook.Target); err != nil {
		return nil, err
	}
	actor, err := s.execution.liveActor(ctx, binding.WorkspaceID, source.Binding.AuthorizedByMemberID)
	if err != nil {
		return nil, err
	}
	input := binding.Input
	input.AllowedTools = slices.Clone(prepared.agent.AllowedTools)
	input.AdditionalContext = playbookRunInstructions(input)
	input.Trigger = &model.AgentRunTriggerContext{Source: model.AgentRunTriggerSourceAutomationRule, TriggerType: model.CRMPlaybookWorkDue, RuleID: &source.Connection.Snapshot.Flow.ID, ActorID: &actor.UserID, FiredAt: &binding.CreatedAt}
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	run := model.AgentRun{ID: binding.RunID, WorkspaceID: binding.WorkspaceID, AgentID: binding.AgentID,
		TargetType: input.CRMPlaybook.Target.TargetType, TargetID: input.CRMPlaybook.Target.TargetID, RuntimeKind: prepared.agent.RuntimeKind,
		InvocationMode: model.InvocationModeAutonomous, ApprovalState: "not_required", PauseReason: model.AgentRunPauseReasonNone,
		Status: model.AgentRunStatusQueued, TriggeredByUserID: &actor.UserID, Input: payload, OutputSummary: json.RawMessage(`{}`),
		ExecutionStage: strPtr("prepared"), ExternalRuntime: strPtr(agentRuntimeName)}
	// Meter against the reviewed model, retaining the real Helpin Agent identity.
	if err := PreflightAgentRunAIUsage(ctx, s.meter, &run, &source.Connection.Snapshot.Agent); err != nil {
		return nil, err
	}
	stored, _, err := s.store.CreateBoundRun(ctx, run)
	if err != nil {
		return nil, err
	}
	if _, ok := agentRuntimeRunID(stored); ok {
		return stored, nil
	}
	claimed, err := s.store.ClaimBoundRuntimeStart(ctx, stored.WorkspaceID, stored.ID)
	if err != nil {
		return nil, err
	}
	if !claimed {
		return s.recoverLaunch(ctx, stored)
	}
	registered, err := client.UpsertAgent(ctx, prepared.agent)
	if err != nil {
		return nil, s.failBeforeLaunch(ctx, stored, err)
	}
	if !sameCRMPlaybookRuntimeConfiguration(registered, prepared.agent) {
		return nil, s.failBeforeLaunch(ctx, stored, ErrCRMPlaybookConnectionUnsupported)
	}
	// Recheck pause, material facts and live permissions after registration/network delay.
	if _, _, err := s.execution.AuthorizeRun(ctx, stored.WorkspaceID, stored.ID); err != nil {
		return nil, s.failBeforeLaunch(ctx, stored, err)
	}
	request, err := buildRuntimeStartRunRequest(stored, &source.Connection.Snapshot.Agent, *registered)
	if err != nil {
		return nil, s.failBeforeLaunch(ctx, stored, err)
	}
	request.AgentID = prepared.agent.ID
	request.Metadata["crm_playbook_connection_id"] = binding.ConnectionID
	request.Metadata["crm_playbook_situation_id"] = binding.SituationID
	request.Metadata["helpin_agent_id"] = binding.AgentID
	request.TurnPolicy = AgentRuntimeTurnPolicy{Mode: agentRuntimeTurnCompleteOnFinish}
	remote, err := client.StartRun(ctx, request)
	if err != nil || !matchesPlaybookRuntimeRun(remote, stored, binding, client.AppID()) {
		// The provider may have accepted it. Keep its durable identity and recover
		// through GetRun(host_run_id); never create a replacement on this path.
		if stageErr := s.agents.runRepo.UpdateStage(ctx, stored.WorkspaceID, stored.ID, "start_uncertain", nil); stageErr != nil {
			return nil, stageErr
		}
		return stored, ErrCRMPlaybookLaunchUncertain
	}
	if err := s.store.SetBoundRuntimeMapping(ctx, stored.WorkspaceID, stored.ID, remote.ID); err != nil {
		return stored, err
	}
	stored.ExternalRuntimeID = strPtr(remote.ID)
	s.agents.recordTriggerExecution(ctx, stored.WorkspaceID, stored.AgentID, input.Trigger, stored.TargetType, stored.TargetID, stored, nil)
	s.agents.publishRunEvent(stored, actor.UserID)
	return stored, nil
}

// CancelRun delegates to existing runtime cancellation, including unknown remote-ID recovery.
func (s *CRMPlaybookAgentLauncher) CancelRun(ctx context.Context, ws, id, actor string) (*model.AgentRun, error) {
	return s.agents.CancelRun(ctx, ws, id, actor)
}

func (s *CRMPlaybookAgentLauncher) recoverLaunch(ctx context.Context, run *model.AgentRun) (*model.AgentRun, error) {
	binding, err := s.store.RunBinding(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return run, err
	}
	client, ok := s.agents.agentRuntimeClient.(agentRuntimeLaunchClient)
	if !ok || binding == nil {
		return run, ErrCRMPlaybookLaunchUncertain
	}
	remote, err := s.agents.agentRuntimeClient.GetRun(ctx, run.ID)
	if err != nil || !matchesPlaybookRuntimeRun(remote, run, binding, client.AppID()) {
		return run, ErrCRMPlaybookLaunchUncertain
	}
	if err := s.store.SetBoundRuntimeMapping(ctx, run.WorkspaceID, run.ID, remote.ID); err != nil {
		return run, err
	}
	run.ExternalRuntimeID = strPtr(remote.ID)
	return run, nil
}

func matchesPlaybookRuntimeRun(remote *AgentRuntimeRun, run *model.AgentRun, binding *model.AutomationRunBinding, appID string) bool {
	return remote != nil && run != nil && binding != nil && strings.TrimSpace(remote.ID) != "" && remote.HostRunID == run.ID && remote.AppID == appID && remote.AgentID == binding.RuntimeProfileID && remote.Target.Type == run.TargetType && remote.Target.ID == run.TargetID
}

// RecoverPlaybookRun correlates a lost start response. The ordinary runtime
// projection loop handles mapped lifecycle/usage events; recovery never StartRuns.
func (s *CRMPlaybookAgentLauncher) RecoverPlaybookRun(ctx context.Context, run *model.AgentRun) error {
	if s == nil || s.agents == nil || s.agents.agentRuntimeClient == nil {
		return ErrCRMPlaybookRuntimeUnavailable
	}
	if _, mapped := agentRuntimeRunID(run); mapped {
		return nil
	}
	_, err := s.recoverLaunch(ctx, run)
	return err
}

func (s *CRMPlaybookAgentLauncher) failBeforeLaunch(ctx context.Context, run *model.AgentRun, cause error) error {
	now := time.Now().UTC()
	run.Status = model.AgentRunStatusFailed
	run.CompletedAt = &now
	run.ExecutionStage = strPtr("failed_to_start")
	run.ErrorMessage = strPtr("Playbook run could not start. Review automation settings.")
	if err := s.agents.runRepo.UpdateReconciledFailure(ctx, run); err != nil {
		return err
	}
	return cause
}

func crmPlaybookRuntimeTools() []string {
	return []string{"get_crm_playbook_context", "propose_crm_playbook_action", "finish_task"}
}

func sameCRMPlaybookRuntimeConfiguration(registered *AgentRuntimeAgent, expected AgentRuntimeAgent) bool {
	if registered == nil {
		return false
	}
	actual := *registered
	actual.CreatedAt, actual.UpdatedAt = time.Time{}, time.Time{}
	expected.CreatedAt, expected.UpdatedAt = time.Time{}, time.Time{}
	left, err := connectionContentFingerprint(actual)
	if err != nil {
		return false
	}
	right, err := connectionContentFingerprint(expected)
	return err == nil && left == right
}

func validateLivePlaybookAgent(live *model.Agent, frozen model.Agent, target model.AgentRunTargetContext) error {
	if live == nil || !live.IsSystem || live.EffectivePresetKey() != model.AgentPresetCRMOperator || !slices.Contains(parseJSONStringSlice(live.AllowedTargets), target.TargetType) {
		return ErrCRMPlaybookForbidden
	}
	readTool := map[string]string{"crm_company": "get_crm_company", "crm_contact": "get_crm_contact", "crm_deal": "get_crm_deal"}[target.TargetType]
	if readTool == "" || !slices.Contains(parseJSONStringSlice(live.AllowedTools), readTool) || !slices.Contains(parseJSONStringSlice(frozen.AllowedTools), readTool) {
		return ErrCRMPlaybookForbidden
	}
	if !slices.Equal(live.TeamIDs, frozen.TeamIDs) {
		return ErrCRMPlaybookForbidden
	}
	return nil
}

func playbookRunInstructions(input model.AgentRunInputPayload) string {
	return "Work only on the bound CRM Playbook objective. Read get_crm_playbook_context, then use the relevant captured skill. " +
		"Use propose_crm_playbook_action only for a supported useful next action. If waiting, finish with a concise factual explanation without inventing an action. CRM owns approvals and progress. " +
		"Do not request a second Agent approval, send directly, create tasks directly, or invent progress. Finish when useful preparation is recorded.\n\n" + input.AdditionalContext
}
