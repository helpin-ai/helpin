package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Command-bar plans on Agent Runtime run groups (COMMAND_BAR_RUN_GROUPS_ENABLED).
//
// A pipeline or DAG plan gets a host-owned run group and its child runs
// join it. The runtime persists each child's terminal transition together
// with a per-group completion sequence, so a completion is never lost the
// way an event can be. The consumer reads completions after the plan's
// cursor and, for each, advances the plan using the completion's status
// and then records the cursor, so a completion is never skipped. The run
// finalizer and the stalled-plan sweep keep working as before and share
// the plan's advancement lock, so a sweep racing a completion cannot
// advance a step twice. Plans without a group keep the existing behavior.

const (
	commandBarPlanLockWait        = 15 * time.Second
	commandBarRunGroupPageSize    = 100
	commandBarRunGroupConsumerMax = 200
)

// errCommandBarRunNotRecorded means a completion names a runtime run the
// host has no record of yet; the completion is retried, not acknowledged.
var errCommandBarRunNotRecorded = errors.New("command bar plan run not recorded yet")

// CommandBarRunGroupCompletion is one child's terminal transition.
type CommandBarRunGroupCompletion struct {
	Seq          int64
	RuntimeRunID string
	Status       string
}

// commandBarRunGroupClient is the part of the Agent Runtime client the
// plan run groups use.
type commandBarRunGroupClient interface {
	RunGroupsAvailable(ctx context.Context) bool
	CreateRunGroup(ctx context.Context, creationKey string) (string, error)
	ListRunGroupCompletions(ctx context.Context, groupID string, after int64, limit int) ([]CommandBarRunGroupCompletion, error)
}

type commandBarRunGroupKey struct{}

func withCommandBarRunGroup(ctx context.Context, groupID string) context.Context {
	if strings.TrimSpace(groupID) == "" {
		return ctx
	}
	return context.WithValue(ctx, commandBarRunGroupKey{}, groupID)
}

func commandBarRunGroupFromContext(ctx context.Context) string {
	groupID, _ := ctx.Value(commandBarRunGroupKey{}).(string)
	return groupID
}

// SetCommandBarRunGroups enables run groups for new pipeline and DAG plans.
func (s *AgentService) SetCommandBarRunGroups(enabled bool, repo *repository.CommandBarPlanRunGroupRepository) *AgentService {
	s.commandBarRunGroupsEnabled = enabled
	s.commandBarRunGroupRepo = repo
	return s
}

func (s *AgentService) commandBarRunGroups() (commandBarRunGroupClient, bool) {
	if s == nil || s.commandBarRunGroupRepo == nil {
		return nil, false
	}
	client, ok := s.agentRuntimeClient.(commandBarRunGroupClient)
	return client, ok && client != nil
}

// ensureCommandBarPlanRunGroup gives a new plan a run group when enabled and
// the runtime supports it. Failure leaves the plan on the existing path.
func (s *AgentService) ensureCommandBarPlanRunGroup(ctx context.Context, workspaceID, planID string) {
	client, ok := s.commandBarRunGroups()
	if !ok || !s.commandBarRunGroupsEnabled || !client.RunGroupsAvailable(ctx) {
		return
	}
	groupID, err := client.CreateRunGroup(ctx, "command_bar_plan:"+planID)
	if err != nil {
		slog.WarnContext(ctx, "command bar plan run group not created; using event and sweep advancement",
			"workspace_id", workspaceID, "plan_id", planID, "error", err)
		return
	}
	if err := s.commandBarRunGroupRepo.Create(ctx, &model.CommandBarPlanRunGroup{PlanID: planID, WorkspaceID: workspaceID, GroupID: groupID}); err != nil {
		slog.WarnContext(ctx, "command bar plan run group not recorded", "workspace_id", workspaceID, "plan_id", planID, "error", err)
	}
}

// commandBarPlanStepContext marks a plan step's run to join the plan's group.
func (s *AgentService) commandBarPlanStepContext(ctx context.Context, workspaceID, planID string) context.Context {
	if s == nil || s.commandBarRunGroupRepo == nil || strings.TrimSpace(planID) == "" {
		return ctx
	}
	link, err := s.commandBarRunGroupRepo.GetByPlan(ctx, workspaceID, planID)
	if err != nil {
		slog.WarnContext(ctx, "command bar plan run group lookup failed", "workspace_id", workspaceID, "plan_id", planID, "error", err)
		return ctx
	}
	if link == nil {
		return ctx
	}
	return withCommandBarRunGroup(ctx, link.GroupID)
}

// withCommandBarPlanLock runs fn under the plan's advancement lock. It
// reports false when the lock stayed taken: another path is advancing the
// plan and will see the same state.
func (s *AgentService) withCommandBarPlanLock(ctx context.Context, planID string, fn func() error) (bool, error) {
	if s == nil || s.commandBarRunGroupRepo == nil {
		return true, fn()
	}
	return s.commandBarRunGroupRepo.WithPlanLock(ctx, planID, commandBarPlanLockWait, fn)
}

// StartCommandBarRunGroupConsumer applies plan completions until ctx ends.
// Run it on one replica (the Agent Runtime projection leader).
func (s *AgentService) StartCommandBarRunGroupConsumer(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := s.ConsumeCommandBarRunGroupCompletions(ctx); err != nil && ctx.Err() == nil {
			slog.WarnContext(ctx, "command bar run group consumer pass failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// ConsumeCommandBarRunGroupCompletions applies new completions of every
// running plan that has a run group.
func (s *AgentService) ConsumeCommandBarRunGroupCompletions(ctx context.Context) error {
	client, ok := s.commandBarRunGroups()
	if !ok || s.commandBarPlanRepo == nil || s.runRepo == nil {
		return nil
	}
	links, err := s.commandBarRunGroupRepo.ListRunning(ctx, commandBarRunGroupConsumerMax)
	if err != nil {
		return err
	}
	for _, link := range links {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := s.consumeCommandBarPlanCompletions(ctx, client, link); err != nil {
			slog.WarnContext(ctx, "command bar plan completions not applied",
				"workspace_id", link.WorkspaceID, "plan_id", link.PlanID, "error", err)
		}
	}
	return nil
}

func (s *AgentService) consumeCommandBarPlanCompletions(ctx context.Context, client commandBarRunGroupClient, link model.CommandBarPlanRunGroup) error {
	completions, err := client.ListRunGroupCompletions(ctx, link.GroupID, link.CompletionCursor, commandBarRunGroupPageSize)
	if err != nil {
		return err
	}
	if len(completions) == 0 {
		return s.commandBarRunGroupRepo.Touch(ctx, link.WorkspaceID, link.PlanID)
	}
	for _, completion := range completions {
		applied := false
		_, err := s.withCommandBarPlanLock(ctx, link.PlanID, func() error {
			return s.applyThenAdvance(ctx, link, completion.Seq, func() error {
				applied = true
				return s.applyCommandBarCompletion(ctx, link, completion)
			})
		})
		if err != nil {
			return err
		}
		if applied {
			slog.InfoContext(ctx, "command bar plan completion applied", "workspace_id", link.WorkspaceID,
				"plan_id", link.PlanID, "seq", completion.Seq, "status", completion.Status)
		}
	}
	return nil
}

// applyThenAdvance applies one completion and only then moves the plan's
// cursor past it, under the plan's lock. A failure or crash before the
// cursor moves leaves the completion to be applied again on the next poll;
// applying is idempotent (each step starts at most once), so a completion
// can repeat but is never skipped. A completion the cursor already passed
// is not applied again.
func (s *AgentService) applyThenAdvance(ctx context.Context, link model.CommandBarPlanRunGroup, seq int64, apply func() error) error {
	current, err := s.commandBarRunGroupRepo.GetByPlan(ctx, link.WorkspaceID, link.PlanID)
	if err != nil || current == nil || current.CompletionCursor >= seq {
		return err
	}
	if err := apply(); err != nil {
		return err
	}
	_, err = s.commandBarRunGroupRepo.AdvanceCursor(ctx, link.WorkspaceID, link.PlanID, seq)
	return err
}

// applyCommandBarCompletion advances the plan with the child's settled
// status. The child's own row is left to the projection, whose finalizers
// must still see the terminal transition.
func (s *AgentService) applyCommandBarCompletion(ctx context.Context, link model.CommandBarPlanRunGroup, completion CommandBarRunGroupCompletion) error {
	run, err := s.runRepo.GetByExternalRuntimeID(ctx, agentRuntimeName, completion.RuntimeRunID)
	if err != nil {
		return err
	}
	if run == nil {
		// The host has not recorded this runtime run yet; leave the
		// completion unacknowledged so the next poll applies it.
		return fmt.Errorf("%w: runtime run %s", errCommandBarRunNotRecorded, completion.RuntimeRunID)
	}
	// Completions from another workspace or for another plan are not this
	// plan's to apply; acknowledging them is correct.
	if run.WorkspaceID != link.WorkspaceID {
		return nil
	}
	settled := *run
	if isTerminalAgentRunStatus(completion.Status) {
		settled.Status = completion.Status
	}
	payload, ok := commandBarRunPayload(&settled)
	if !ok || payload.PlanID != link.PlanID {
		return nil
	}
	_, err = s.startReadyCommandBarPlanStepsLocked(ctx, CommandBarPlanInput{
		WorkspaceID: run.WorkspaceID,
		ActorID:     derefString(run.TriggeredByUserID),
		PlanID:      payload.PlanID,
		Prompt:      payload.Prompt,
		PageContext: payload.PageContext,
		Steps:       payload.Steps,
		SettledRun:  &settled,
	})
	return err
}
