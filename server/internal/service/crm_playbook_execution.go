package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrCRMPlaybookRuntimeUnavailable leaves setup reviewable without pretending execution is ready.
var ErrCRMPlaybookRuntimeUnavailable = errors.New("Automation is not available right now")

type crmPlaybookExecutionStore interface {
	Receipt(context.Context, string, string, string, string) (*model.CRMPlaybookAutomationReceipt, error)
	Settings(context.Context, string, string) (*model.CRMPlaybookAutomationSettings, error)
	Binding(context.Context, string, string) (*model.CRMPlaybookAutomationBinding, error)
	Configure(context.Context, string, string, string, model.CRMPlaybookAutomationCommand, string, time.Time) (*model.CRMPlaybookAutomationReceipt, error)
	Adopt(context.Context, string, string, string, model.CRMPlaybookAutomationAdoption, string, time.Time) (*model.CRMPlaybookAutomationReceipt, error)
	Source(context.Context, string, string) (*model.CRMPlaybookExecutionSource, error)
	ReserveRun(context.Context, model.AutomationScheduledEvent, time.Time, func(model.CRMPlaybookExecutionSource, string) (model.AutomationRunBinding, error)) (*model.AutomationRunBinding, error)
	ValidateRun(context.Context, string, string) (*model.AutomationRunBinding, *model.CRMPlaybookExecutionSource, error)
	FinishDispatch(context.Context, model.AutomationScheduledEvent, string, time.Time) error
	ActiveRunIDs(context.Context, string, string, string) ([]string, error)
	ObserveTerminalRun(context.Context, model.AgentRun, time.Time) error
	HasRuntimeProfile(context.Context, string) (bool, error)
}

type crmPlaybookRunLauncher interface {
	PlaybookRuntimeReady() bool
	StartPlaybookRun(context.Context, model.AutomationRunBinding) (*model.AgentRun, error)
	CancelRun(context.Context, string, string, string) (*model.AgentRun, error)
}

type crmPlaybookLiveAuthorizer interface {
	ResolveActor(context.Context, string, string) (*authorization.Actor, error)
	Can(*authorization.Actor, authorization.Permission) bool
	CanAccessModule(context.Context, *authorization.Actor, model.ModuleID) (bool, error)
}

type crmPlaybookMemberReader interface {
	GetMembershipByID(context.Context, string, string) (*model.WorkspaceMember, error)
}

// CRMPlaybookExecutionService connects live business gates to shared Automation and Agent runs.
// It never executes a model or provider action itself.
type CRMPlaybookExecutionService struct {
	store        crmPlaybookExecutionStore
	playbooks    *CRMPlaybookService
	launcher     crmPlaybookRunLauncher
	authz        crmPlaybookLiveAuthorizer
	members      crmPlaybookMemberReader
	now          func() time.Time
	entitlements interface {
		RequireFeature(context.Context, string, EntitlementFeature) error
	}
}

// SetEntitlements uses live Automation billing gates independently of CRM teammate access.
func (s *CRMPlaybookExecutionService) SetEntitlements(entitlements *EntitlementService) *CRMPlaybookExecutionService {
	s.entitlements = entitlements
	return s
}

// NewCRMPlaybookExecutionService makes execution, live authority and membership dependencies explicit.
func NewCRMPlaybookExecutionService(store crmPlaybookExecutionStore, playbooks *CRMPlaybookService,
	launcher crmPlaybookRunLauncher, authz crmPlaybookLiveAuthorizer, members crmPlaybookMemberReader,
) *CRMPlaybookExecutionService {
	return &CRMPlaybookExecutionService{store: store, playbooks: playbooks, launcher: launcher, authz: authz, members: members, now: time.Now}
}

// Settings exposes the live gate under CRM read access, not private Agent instructions.
func (s *CRMPlaybookExecutionService) Settings(ctx context.Context, ws, id string) (*model.CRMPlaybookAutomationSettings, error) {
	if _, err := s.playbooks.Get(ctx, ws, id); err != nil {
		return nil, err
	}
	return s.store.Settings(ctx, ws, id)
}

// Overview shows the connected automation without exposing builder-only instructions.
func (s *CRMPlaybookExecutionService) Overview(ctx context.Context, ws, id string) (*model.CRMPlaybookAutomationOverview, error) {
	settings, err := s.Settings(ctx, ws, id)
	if err != nil {
		return nil, err
	}
	result := &model.CRMPlaybookAutomationOverview{Settings: settings, RuntimeAvailable: s.launcher != nil && s.launcher.PlaybookRuntimeReady()}
	connectionID := ""
	// The latest reviewed connection is available for explicit adoption. Live
	// settings and existing Signal bindings keep their independently pinned IDs.
	connections, err := s.playbooks.store.Connections(ctx, ws, id, 0, 1)
	if err != nil {
		return nil, err
	}
	if connections != nil && len(connections.Data) > 0 {
		connectionID = connections.Data[0].ID
	} else if settings != nil {
		connectionID = settings.ConnectionID
	}
	if connectionID != "" {
		connection, err := s.playbooks.store.Connection(ctx, ws, id, connectionID)
		if err != nil {
			return nil, err
		}
		if connection != nil {
			result.Connection = connection
			result.FlowID = connection.Snapshot.Flow.ID
			result.FlowName = connection.Snapshot.Flow.Name
			result.AgentID = connection.Snapshot.Agent.ID
			result.AgentName = connection.Snapshot.Agent.Name
		}
	}
	return result, nil
}

// Binding reads execution status for the same canonical Signal the user can inspect.
func (s *CRMPlaybookExecutionService) Binding(ctx context.Context, ws, id string) (*model.CRMPlaybookAutomationBinding, error) {
	if _, err := s.playbooks.situations.GetByID(ctx, ws, id); err != nil {
		return nil, err
	}
	return s.store.Binding(ctx, ws, id)
}

// Configure requires explicit approval of a published connection and both configuration permissions.
func (s *CRMPlaybookExecutionService) Configure(ctx context.Context, ws, id string, req model.CRMPlaybookAutomationCommand) (*model.CRMPlaybookAutomationReceipt, error) {
	actor, err := s.playbooks.authorizeConnection(ctx, ws)
	if err != nil {
		return nil, err
	}
	if !req.Confirmed || !validSituationID(id) || !validSituationID(req.ConnectionID) || !validPlaybookKey(req.CommandKey) || req.ExpectedRevision < 0 ||
		(req.EntryMode != "manual" && req.EntryMode != "automatic") || req.MaxRunsPerDay < 1 || req.MaxRunsPerDay > 24 || req.MaxNoProgressRuns < 1 || req.MaxNoProgressRuns > 10 {
		return nil, ErrCRMPlaybookInput
	}
	fingerprint, err := playbookFingerprint(actor.WorkspaceMemberID, req)
	if err != nil {
		return nil, err
	}
	if previous, err := s.store.Receipt(ctx, ws, id, req.CommandKey, fingerprint); err != nil || previous != nil {
		if err == nil {
			err = s.cancelRuns(ctx, ws, id, "", actor.UserID)
		}
		return previous, err
	}
	connection, err := s.playbooks.store.Connection(ctx, ws, id, req.ConnectionID)
	if err != nil {
		return nil, err
	}
	if connection == nil {
		return nil, ErrCRMPlaybookNotFound
	}
	if req.Enabled {
		if err := s.requireEntitlements(ctx, ws); err != nil {
			return nil, err
		}
		if err := s.connectionReady(*connection); err != nil {
			return nil, err
		}
		book, err := s.playbooks.Get(ctx, ws, id)
		if err != nil {
			return nil, err
		}
		if req.EntryMode == "automatic" && !book.Playbook.AcceptingCustomers {
			return nil, ErrCRMPlaybookInput
		}
	}
	result, err := s.store.Configure(ctx, ws, id, actor.WorkspaceMemberID, req, fingerprint, s.now().UTC())
	if err != nil {
		return nil, err
	}
	if !req.Enabled {
		if err := s.cancelRuns(ctx, ws, id, "", actor.UserID); err != nil {
			return result, err
		}
	}
	return result, nil
}

// Adopt starts or pauses exactly one existing Signal after explicit review.
func (s *CRMPlaybookExecutionService) Adopt(ctx context.Context, ws, id string, req model.CRMPlaybookAutomationAdoption) (*model.CRMPlaybookAutomationReceipt, error) {
	actor, err := s.playbooks.authorize(ctx, ws, authorization.PermCRMEdit)
	if err != nil {
		return nil, err
	}
	if !req.Confirmed || !validSituationID(id) || !validSituationID(req.ConnectionID) || !validPlaybookKey(req.CommandKey) || req.ExpectedRevision < 1 || req.ExpectedGeneration < 0 {
		return nil, ErrCRMPlaybookInput
	}
	if ok, err := s.authz.CanAccessModule(ctx, actor, model.ModuleCRM); err != nil || !ok {
		if err != nil {
			return nil, err
		}
		return nil, ErrCRMPlaybookForbidden
	}
	fingerprint, err := playbookFingerprint(actor.WorkspaceMemberID, req)
	if err != nil {
		return nil, err
	}
	if previous, err := s.store.Receipt(ctx, ws, id, req.CommandKey, fingerprint); err != nil || previous != nil {
		if err == nil {
			err = s.cancelRuns(ctx, ws, "", id, actor.UserID)
		}
		return previous, err
	}
	item, err := s.playbooks.situations.GetByID(ctx, ws, id)
	if err != nil {
		return nil, err
	}
	if item.Situation.PlaybookID == nil {
		return nil, ErrCRMPlaybookInput
	}
	connection, err := s.playbooks.store.Connection(ctx, ws, *item.Situation.PlaybookID, req.ConnectionID)
	if err != nil {
		return nil, err
	}
	if connection == nil {
		return nil, ErrCRMPlaybookNotFound
	}
	if req.Enabled {
		if err := s.connectionReady(*connection); err != nil {
			return nil, err
		}
		if err := s.requireEntitlements(ctx, ws); err != nil {
			return nil, err
		}
	}
	result, err := s.store.Adopt(ctx, ws, id, actor.WorkspaceMemberID, req, fingerprint, s.now().UTC())
	if err != nil {
		return nil, err
	}
	// Updating a connection also revokes the previous generation before cancellation.
	if err := s.cancelRuns(ctx, ws, "", id, actor.UserID); err != nil {
		return result, err
	}
	return result, nil
}

// HandleScheduledEvent turns a fenced wake-up into one existing-runtime run.
func (s *CRMPlaybookExecutionService) HandleScheduledEvent(ctx context.Context, event model.AutomationScheduledEvent, now time.Time) error {
	if event.Kind == model.CRMPlaybookEntryDue {
		return s.handleAutomaticEntry(ctx, event, now)
	}
	binding, err := s.store.ReserveRun(ctx, event, now, func(source model.CRMPlaybookExecutionSource, runID string) (model.AutomationRunBinding, error) {
		prepared, err := prepareCRMPlaybookExecution(source, "helpin")
		if err != nil {
			return model.AutomationRunBinding{}, err
		}
		return model.AutomationRunBinding{AgentID: prepared.helpinAgentID, RuntimeProfileID: prepared.agent.ID, Input: prepared.input}, nil
	})
	if err != nil {
		return err
	}
	if binding == nil {
		return nil
	}
	_, _, err = s.AuthorizeRun(ctx, event.WorkspaceID, binding.RunID)
	if err != nil {
		return s.store.FinishDispatch(ctx, event, playbookLaunchErrorCode(err), now)
	}
	if s.launcher == nil || !s.launcher.PlaybookRuntimeReady() {
		return s.store.FinishDispatch(ctx, event, "runtime_unavailable", now)
	}
	if _, err := s.launcher.StartPlaybookRun(ctx, *binding); err != nil {
		return s.store.FinishDispatch(ctx, event, playbookLaunchErrorCode(err), now)
	}
	return s.store.FinishDispatch(ctx, event, "started", now)
}

// AuthorizeRun reloads both initiating and configuration authority for each bound operation.
func (s *CRMPlaybookExecutionService) AuthorizeRun(ctx context.Context, ws, runID string) (*model.AutomationRunBinding, *model.CRMPlaybookExecutionSource, error) {
	binding, source, err := s.store.ValidateRun(ctx, ws, runID)
	if err != nil {
		return nil, nil, err
	}
	if err := s.requireEntitlements(ctx, ws); err != nil {
		return nil, nil, err
	}
	for _, grant := range []struct {
		member        string
		configuration bool
	}{{source.Settings.AuthorizedByMemberID, true}, {source.Binding.AuthorizedByMemberID, false}} {
		actor, err := s.liveActor(ctx, ws, grant.member)
		if err != nil {
			return nil, nil, err
		}
		if !s.authz.Can(actor, authorization.PermCRMEdit) {
			return nil, nil, ErrCRMPlaybookForbidden
		}
		if allowed, err := s.authz.CanAccessModule(ctx, actor, model.ModuleCRM); err != nil || !allowed {
			if err != nil {
				return nil, nil, err
			}
			return nil, nil, ErrCRMPlaybookForbidden
		}
		if grant.configuration && (!s.authz.Can(actor, authorization.PermCRMAdmin) || !s.authz.Can(actor, authorization.PermPMAdminAutomations)) {
			return nil, nil, ErrCRMPlaybookForbidden
		}
		if grant.configuration {
			if err := s.requireModules(ctx, actor); err != nil {
				return nil, nil, err
			}
		}
	}
	return binding, source, nil
}

func (s *CRMPlaybookExecutionService) requireEntitlements(ctx context.Context, ws string) error {
	if s.entitlements == nil {
		return nil
	}
	for _, feature := range []EntitlementFeature{EntitlementFeatureAutomationFlows, EntitlementFeatureAgentScheduling} {
		if err := s.entitlements.RequireFeature(ctx, ws, feature); err != nil {
			return err
		}
	}
	return nil
}

// ObserveTerminalRun keeps follow-through durable without trusting an Agent's claimed outcome.
func (s *CRMPlaybookExecutionService) ObserveTerminalRun(ctx context.Context, run model.AgentRun) error {
	return s.store.ObserveTerminalRun(ctx, run, s.now().UTC())
}

// AfterSituationCommand uses the existing cancel path after atomic fencing. Explicit
// Signal resume rechecks the previous connection; enabling a Playbook never resumes old work.
func (s *CRMPlaybookExecutionService) AfterSituationCommand(ctx context.Context, ws, id, userID string, result model.CRMSituationCommandResult) error {
	if err := s.cancelRuns(ctx, ws, "", id, userID); err != nil {
		return err
	}
	if result.Change.Operation != "resume" {
		return nil
	}
	binding, err := s.store.Binding(ctx, ws, id)
	if err != nil || binding == nil || binding.Enabled || binding.Blocker != "signal_paused" {
		return err
	}
	item, err := s.playbooks.situations.GetByID(ctx, ws, id)
	if err != nil {
		return err
	}
	if item.Situation.Revision != result.Change.Revision {
		return nil
	}
	_, err = s.Adopt(ctx, ws, id, model.CRMPlaybookAutomationAdoption{CommandKey: "resume:" + result.Change.ID, ExpectedRevision: item.Situation.Revision, ExpectedGeneration: binding.Generation, ConnectionID: binding.ConnectionID, Enabled: true, Confirmed: true})
	return err
}

func (s *CRMPlaybookExecutionService) connectionReady(connection model.CRMPlaybookConnection) error {
	if s.launcher == nil || !s.launcher.PlaybookRuntimeReady() {
		return ErrCRMPlaybookRuntimeUnavailable
	}
	if connection.Snapshot.SchemaVersion != 2 || connection.Snapshot.Flow.TriggerType != model.CRMPlaybookWorkDue {
		return ErrCRMPlaybookConnectionUnsupported
	}
	fingerprint, err := connectionContentFingerprint(connection.Snapshot)
	if err != nil {
		return err
	}
	if fingerprint != connection.Fingerprint {
		return ErrCRMPlaybookConnectionUnsupported
	}
	return nil
}

func (s *CRMPlaybookExecutionService) liveActor(ctx context.Context, ws, memberID string) (*authorization.Actor, error) {
	if s.authz == nil || s.members == nil {
		return nil, ErrCRMPlaybookForbidden
	}
	member, err := s.members.GetMembershipByID(ctx, ws, memberID)
	if err != nil {
		return nil, err
	}
	if member == nil || member.UserID == nil || member.Status != model.WorkspaceMemberStatusActive {
		return nil, ErrCRMPlaybookForbidden
	}
	actor, err := s.authz.ResolveActor(ctx, ws, *member.UserID)
	if err != nil {
		return nil, ErrCRMPlaybookForbidden
	}
	if ok, err := s.authz.CanAccessModule(ctx, actor, model.ModuleCRM); err != nil || !ok {
		if err != nil {
			return nil, err
		}
		return nil, ErrCRMPlaybookForbidden
	}
	return actor, nil
}

func (s *CRMPlaybookExecutionService) requireModules(ctx context.Context, actor *authorization.Actor) error {
	if s.authz == nil {
		return ErrCRMPlaybookForbidden
	}
	for _, module := range []model.ModuleID{model.ModuleCRM, model.ModuleAutomation} {
		ok, err := s.authz.CanAccessModule(ctx, actor, module)
		if err != nil {
			return err
		}
		if !ok {
			return ErrCRMPlaybookForbidden
		}
	}
	return nil
}

func (s *CRMPlaybookExecutionService) cancelRuns(ctx context.Context, ws, pb, id, actor string) error {
	ids, err := s.store.ActiveRunIDs(ctx, ws, pb, id)
	if err != nil {
		return err
	}
	for _, runID := range ids {
		if s.launcher == nil {
			return ErrCRMPlaybookRuntimeUnavailable
		}
		if _, err := s.launcher.CancelRun(ctx, ws, runID, actor); err != nil {
			return err
		}
	}
	return nil
}

func prepareCRMPlaybookExecution(source model.CRMPlaybookExecutionSource, appID string) (*crmPlaybookRuntimePreparation, error) {
	var action model.ActionConfigRunAgent
	if err := json.Unmarshal(source.Connection.Snapshot.Flow.ActionConfig, &action); err != nil {
		return nil, err
	}
	target := model.AgentRunTargetContext{TargetType: action.TargetType, TargetID: action.TargetID}
	if target.TargetType == "crm_record" {
		switch {
		case source.Item.Situation.DealID != nil:
			target = model.AgentRunTargetContext{TargetType: "crm_deal", TargetID: *source.Item.Situation.DealID}
		case source.Item.Situation.CompanyID != nil:
			target = model.AgentRunTargetContext{TargetType: "crm_company", TargetID: *source.Item.Situation.CompanyID}
		case source.Item.Situation.ContactID != nil:
			target = model.AgentRunTargetContext{TargetType: "crm_contact", TargetID: *source.Item.Situation.ContactID}
		default:
			return nil, ErrCRMPlaybookInput
		}
	}
	if target.TargetID == "" {
		switch target.TargetType {
		case "crm_company":
			target.TargetID = derefString(source.Item.Situation.CompanyID)
		case "crm_contact":
			target.TargetID = derefString(source.Item.Situation.ContactID)
		case "crm_deal":
			target.TargetID = derefString(source.Item.Situation.DealID)
		}
	}
	return prepareCRMPlaybookConnectionRuntime(source.Connection, source.Policy, source.Item, model.CRMPlaybookContextRequest{
		WorkspaceID: source.Binding.WorkspaceID, PlaybookID: source.Binding.PlaybookID, PlaybookVersionID: source.Policy.ID,
		SituationID: source.Binding.SituationID, ExpectedSituationRevision: source.Item.Situation.Revision,
		SpecializationVersion: source.Connection.Snapshot.Specialization.Version, Target: target}, appID)
}

func playbookLaunchErrorCode(err error) string {
	// Do not persist provider messages, prompts or billing internals in CRM status.
	if errors.Is(err, ErrCRMPlaybookLaunchUncertain) {
		return "start_uncertain"
	}
	if errors.Is(err, ErrCRMPlaybookRuntimeUnavailable) {
		return "runtime_unavailable"
	}
	if errors.Is(err, ErrCRMPlaybookForbidden) {
		return "permission_changed"
	}
	var entitlement *EntitlementError
	if errors.As(err, &entitlement) {
		return "usage_limit"
	}
	if strings.Contains(strings.ToLower(err.Error()), "budget") || strings.Contains(strings.ToLower(err.Error()), "usage") {
		return "usage_limit"
	}
	return "run_start_failed"
}
