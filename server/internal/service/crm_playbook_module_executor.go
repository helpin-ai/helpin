package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMPlaybookModuleExecutor adapts approved actions to the existing CRM/PM services.
// Beacon prepares; these normal services perform the exact human-authorized operation.
type CRMPlaybookModuleExecutor struct {
	execution  *CRMPlaybookExecutionService
	store      *repository.CRMPlaybookExecutionRepository
	email      *CRMEmailService
	tasks      *PMTaskService
	deals      *CRMDealService
	workspaces *repository.WorkspaceRepository
	agents     *repository.AgentRepository
	dealRepo   *repository.CRMDealRepository
}

// NewCRMPlaybookModuleExecutor composes existing module operations without starting them.
func NewCRMPlaybookModuleExecutor(execution *CRMPlaybookExecutionService, store *repository.CRMPlaybookExecutionRepository, email *CRMEmailService, tasks *PMTaskService, deals *CRMDealService, workspaces *repository.WorkspaceRepository, agents *repository.AgentRepository, dealRepo *repository.CRMDealRepository) *CRMPlaybookModuleExecutor {
	return &CRMPlaybookModuleExecutor{execution: execution, store: store, email: email, tasks: tasks, deals: deals, workspaces: workspaces, agents: agents, dealRepo: dealRepo}
}

// ContextTeams supplies names/IDs only when PM is useful and accessible. The
// actual task still needs exact review and live assignee/team checks at approval.
func (s *CRMPlaybookModuleExecutor) ContextTeams(ctx context.Context, actor *authorization.Actor) ([]model.CRMPlaybookTaskTeam, error) {
	result := []model.CRMPlaybookTaskTeam{}
	allowed, err := s.execution.authz.CanAccessModule(ctx, actor, model.ModulePM)
	if err != nil || !allowed || !s.execution.authz.Can(actor, authorization.PermPMEdit) {
		return result, err
	}
	for _, id := range actor.TeamIDs() {
		team, err := s.workspaces.GetTeamByID(ctx, actor.WorkspaceID, id)
		if err != nil {
			return nil, err
		}
		if team != nil {
			result = append(result, model.CRMPlaybookTaskTeam{ID: team.ID, Name: team.Name})
		}
	}
	return result, nil
}

// Validate checks current module access and exact destinations before approval is consumed.
func (s *CRMPlaybookModuleExecutor) Validate(ctx context.Context, source model.CRMPlaybookExecutionSource, facts model.CRMPlaybookActionFacts, action model.CRMPlaybookAction, actor *authorization.Actor) error {
	if actor == nil || actor.WorkspaceID != source.Binding.WorkspaceID {
		return ErrCRMPlaybookForbidden
	}
	liveActor, err := s.execution.liveActor(ctx, actor.WorkspaceID, actor.WorkspaceMemberID)
	if err != nil {
		return err
	}
	if liveActor.UserID != actor.UserID || !s.execution.authz.Can(liveActor, authorization.PermCRMEdit) {
		return ErrCRMPlaybookForbidden
	}
	prepared, err := prepareCRMPlaybookExecution(source, "helpin")
	if err != nil {
		return err
	}
	agent, err := s.agents.GetByID(ctx, actor.WorkspaceID, source.Connection.Snapshot.Agent.ID)
	if err != nil {
		return err
	}
	if err := validateLivePlaybookAgent(agent, source.Connection.Snapshot.Agent, prepared.input.CRMPlaybook.Target); err != nil {
		return err
	}
	if err := validateCRMPlaybookAction(action, source, facts); err != nil {
		return err
	}
	if playbookActionApprover(source, action) != actor.WorkspaceMemberID {
		return ErrCRMPlaybookForbidden
	}
	switch action.Kind {
	case "email":
		if s.email == nil {
			return fmt.Errorf("email is not configured")
		}
		return s.email.ValidateActionSender(ctx, actor.WorkspaceID, action.Email.AccountID, actor.UserID)
	case "task":
		if s.tasks == nil || !s.execution.authz.Can(liveActor, authorization.PermPMEdit) {
			return ErrCRMPlaybookForbidden
		}
		allowed, err := s.execution.authz.CanAccessModule(ctx, liveActor, model.ModulePM)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrCRMPlaybookForbidden
		}
		team, err := s.workspaces.GetTeamByID(ctx, actor.WorkspaceID, action.Task.TeamID)
		if err != nil {
			return err
		}
		if team == nil || !liveActor.IsMemberOfTeam(team.ID) {
			return ErrCRMPlaybookForbidden
		}
		member, err := s.workspaces.GetMembershipByID(ctx, actor.WorkspaceID, action.Task.OwnerMemberID)
		if err != nil {
			return err
		}
		if member == nil || member.UserID == nil || member.Status != model.WorkspaceMemberStatusActive {
			return ErrCRMPlaybookForbidden
		}
		owner, err := s.execution.authz.ResolveActor(ctx, actor.WorkspaceID, *member.UserID)
		if err != nil || owner == nil || !owner.IsMemberOfTeam(team.ID) {
			return ErrCRMPlaybookForbidden
		}
		allowed, err = s.execution.authz.CanAccessModule(ctx, owner, model.ModulePM)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrCRMPlaybookForbidden
		}
	case "deal_stage":
		if s.deals == nil || s.dealRepo == nil {
			return fmt.Errorf("deal operations are not configured")
		}
		stage, err := s.dealRepo.GetStage(ctx, action.DealStage.StageID)
		if err != nil {
			return err
		}
		if stage == nil || facts.Deal == nil || stage.PipelineID != facts.Deal.PipelineID {
			return ErrCRMPlaybookInput
		}
	case "handoff":
		if action.Handoff.ReceivingMemberID != actor.WorkspaceMemberID {
			return ErrCRMPlaybookForbidden
		}
	}
	return nil
}

// Execute performs one claimed operation. Unknown provider/local commit results are kept unresolved.
func (s *CRMPlaybookModuleExecutor) Execute(ctx context.Context, intent model.CRMPlaybookActionIntent, action model.CRMPlaybookAction, actor *authorization.Actor) crmPlaybookActionResult {
	_, source, err := s.execution.AuthorizeRun(ctx, intent.WorkspaceID, intent.RunID)
	if err != nil {
		return playbookActionFailed("The Signal changed or automation was paused before execution.")
	}
	facts, err := s.store.ActionFacts(ctx, intent.WorkspaceID, intent.SituationID)
	if err != nil {
		return playbookActionFailed("Customer context could not be rechecked.")
	}
	fingerprint, err := crmPlaybookActionContextFingerprint(*source, *facts)
	if err != nil || fingerprint != intent.ContextFingerprint {
		return playbookActionFailed("Customer details changed. Review a fresh action.")
	}
	if err := s.Validate(ctx, *source, *facts, action, actor); err != nil {
		return playbookActionFailed("Permissions or the action destination changed. Review the action.")
	}
	ctx = authorization.WithActor(ctx, actor)
	switch action.Kind {
	case "email":
		message, err := s.email.SendActionEmail(ctx, intent.WorkspaceID, actor.UserID, intent.SuggestionID, *action.Email)
		if err != nil || message == nil || message.MessageExternalID == "" {
			return playbookActionUnknown("The send result needs checking. Do not send it again.")
		}
		var id *string
		if validSituationID(message.ID) {
			id = &message.ID
		}
		return crmPlaybookActionResult{status: "succeeded", resultType: "crm_email_message", resultID: id}
	case "task":
		a := action.Task
		externalID := "crm-action:" + intent.SuggestionID
		task, err := s.tasks.Create(ctx, model.CreateTaskRequest{WorkspaceID: intent.WorkspaceID, Name: a.Name, Description: &a.Description, TeamID: &a.TeamID,
			OwnerMemberIDs: []string{a.OwnerMemberID}, Deadline: a.Deadline, ExternalID: &externalID, RunOnCreate: false}, actor.UserID)
		if err != nil || task == nil {
			return playbookActionUnknown("Task creation needs checking. Do not create a duplicate.")
		}
		if err := s.store.LinkActionTask(ctx, intent, task.Task); err != nil {
			return playbookActionUnknown("The task was created, but its customer links need checking.")
		}
		return crmPlaybookActionResult{status: "succeeded", resultType: "task", resultID: &task.Task.ID}
	case "deal_stage":
		deal, err := s.deals.UpdateStageForPlaybook(ctx, *facts.Deal, action.DealStage.StageID, actor.UserID, intent)
		if errors.Is(err, repository.ErrCRMDealChanged) {
			return playbookActionFailed("The deal changed. Review its current stage.")
		}
		if err != nil || deal == nil {
			return playbookActionUnknown("The deal update needs checking.")
		}
		return crmPlaybookActionResult{status: "succeeded", resultType: "crm_deal", resultID: &deal.ID}
	case "handoff":
		if err := s.store.AcceptHandoff(ctx, intent, *action.Handoff, time.Now().UTC()); err != nil {
			return playbookActionUnknown("Handoff acceptance needs checking.")
		}
		return crmPlaybookActionResult{status: "succeeded", resultType: "crm_situation", resultID: &intent.SituationID}
	case "milestone":
		_, err := s.execution.playbooks.AssessMilestone(ctx, intent.WorkspaceID, source.Binding.PlaybookID, intent.SituationID,
			model.CRMPlaybookMilestoneRequest{CommandKey: "action:" + intent.SuggestionID, ExpectedRevision: intent.SituationRevision, MilestoneKey: action.Milestone.Key, Status: "achieved", Summary: action.Milestone.Evidence})
		if err != nil {
			return playbookActionUnknown("The milestone assessment needs checking.")
		}
		return crmPlaybookActionResult{status: "succeeded", resultType: "crm_situation", resultID: &intent.SituationID}
	}
	return playbookActionFailed("This action is not supported.")
}

func playbookActionFailed(message string) crmPlaybookActionResult {
	return crmPlaybookActionResult{status: "failed", safeError: &message}
}
func playbookActionUnknown(message string) crmPlaybookActionResult {
	return crmPlaybookActionResult{status: "in_progress", safeError: &message}
}

// Reconcile inspects effects without repeating them. Pausing does not erase an already-sent message.
func (s *CRMPlaybookModuleExecutor) Reconcile(ctx context.Context, intent model.CRMPlaybookActionIntent, action model.CRMPlaybookAction, actor *authorization.Actor) crmPlaybookActionResult {
	switch action.Kind {
	case "email":
		if s.email == nil || action.Email == nil {
			return playbookActionUnknown("The sending account is unavailable.")
		}
		found, id, err := s.email.ReconcileActionEmail(ctx, intent.WorkspaceID, actor.UserID, intent.SuggestionID, *action.Email)
		if err == nil && found {
			return crmPlaybookActionResult{status: "succeeded", resultType: "crm_email_message", resultID: id}
		}
	case "task":
		if actor == nil || action.Task == nil || !s.execution.authz.Can(actor, authorization.PermPMRead) || !actor.IsMemberOfTeam(action.Task.TeamID) {
			return playbookActionUnknown("PM access to the task team is required to check this result.")
		}
		if allowed, err := s.execution.authz.CanAccessModule(ctx, actor, model.ModulePM); err != nil || !allowed {
			return playbookActionUnknown("PM access to the task team is required to check this result.")
		}
		task, err := s.store.ActionTask(ctx, intent.WorkspaceID, "crm-action:"+intent.SuggestionID)
		// The native PM create path trims surrounding whitespace from names.
		// Compare its canonical value while retaining exact identity/team guards.
		if err == nil && task != nil && task.Name == strings.TrimSpace(action.Task.Name) && task.TeamID != nil && *task.TeamID == action.Task.TeamID {
			if err := s.store.LinkActionTask(ctx, intent, *task); err != nil {
				return playbookActionUnknown("The task exists, but its customer links need checking.")
			}
			return crmPlaybookActionResult{status: "succeeded", resultType: "task", resultID: &task.ID}
		}
	case "handoff", "milestone":
		change, err := s.store.ActionWorkChange(ctx, intent.WorkspaceID, intent.SituationID, "action:"+intent.SuggestionID)
		if err == nil && change != nil && change.ActorMemberID != nil && *change.ActorMemberID == actor.WorkspaceMemberID {
			return crmPlaybookActionResult{status: "succeeded", resultType: "crm_situation", resultID: &intent.SituationID}
		}
	case "deal_stage":
		current, err := s.store.ActionIntent(ctx, intent.WorkspaceID, intent.SuggestionID)
		if err == nil && current != nil && current.ApprovedFingerprint == intent.ApprovedFingerprint && current.ResultType == "crm_deal" && current.ResultID != nil {
			return crmPlaybookActionResult{status: "succeeded", resultType: current.ResultType, resultID: current.ResultID}
		}
	}
	return playbookActionUnknown("The result is not confirmed. Inspect the destination before taking further action.")
}
