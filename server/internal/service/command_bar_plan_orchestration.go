package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
)

// CommandBarPlanInput identifies a command-bar parent plan for the local
// step scheduler. Command-bar orchestration runs in-process: dispatch starts
// the first ready steps, the agent-runtime projection finalizer advances the
// plan when a delegated step run reaches a terminal state, and the periodic
// sweep re-kicks plans that stall between the two.
type CommandBarPlanInput struct {
	WorkspaceID string
	ActorID     string
	PlanID      string
	Prompt      string
	PageContext model.CommandBarPageContext
	Steps       []model.CommandBarPlanStep
}

// CommandBarPlanProgress summarizes scheduler state after a scheduling pass.
type CommandBarPlanProgress struct {
	Terminal bool
	Status   string
	Started  []string
}

// SweepStalledCommandBarPlans re-kicks the local step scheduler for running
// pipeline/DAG plans that have not advanced since the cutoff. It replaces the
// retired CommandBarPlanWorkflow watchdog timer: the projection finalizer is
// the primary advancement path, and this sweep is the self-healing backstop
// when a finalizer errors mid-advance or a terminal event is missed.
func (s *AgentService) SweepStalledCommandBarPlans(ctx context.Context, staleAfter time.Duration, limit int) error {
	if s == nil || s.commandBarPlanRepo == nil || s.runRepo == nil {
		return nil
	}
	plans, err := s.commandBarPlanRepo.ListRunningUpdatedBefore(ctx, time.Now().Add(-staleAfter), limit)
	if err != nil {
		return err
	}
	for _, plan := range plans {
		var steps []model.CommandBarPlanStep
		if err := json.Unmarshal(plan.Steps, &steps); err != nil {
			slog.WarnContext(ctx, "command bar plan sweep: decode steps failed",
				"workspace_id", plan.WorkspaceID, "plan_id", plan.ID, "error", err)
			continue
		}
		planKind := commandBarPlanKindForSteps(steps)
		if planKind != model.CommandBarPlanKindTaskPipeline && planKind != model.CommandBarPlanKindDAG {
			continue
		}
		var pageContext model.CommandBarPageContext
		if err := json.Unmarshal(plan.PageContext, &pageContext); err != nil {
			slog.WarnContext(ctx, "command bar plan sweep: decode page context failed",
				"workspace_id", plan.WorkspaceID, "plan_id", plan.ID, "error", err)
			continue
		}
		if _, err := s.startReadyCommandBarPlanSteps(ctx, CommandBarPlanInput{
			WorkspaceID: plan.WorkspaceID,
			ActorID:     derefString(plan.ActorID),
			PlanID:      plan.ID,
			Prompt:      plan.Prompt,
			PageContext: pageContext,
			Steps:       steps,
		}); err != nil {
			slog.WarnContext(ctx, "command bar plan sweep: scheduling pass failed",
				"workspace_id", plan.WorkspaceID, "plan_id", plan.ID, "error", err)
		}
	}
	return nil
}

// StartCommandBarPlanSweep runs SweepStalledCommandBarPlans on a ticker until
// the context is cancelled. Wired next to the agent-runtime reconciliation
// sweep in cmd/api.
func (s *AgentService) StartCommandBarPlanSweep(ctx context.Context, interval, staleAfter time.Duration, limit int) error {
	if s == nil || s.commandBarPlanRepo == nil || s.runRepo == nil {
		return nil
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.ErrorContext(ctx, "command bar plan sweep panic", "panic", recovered)
		}
	}()
	if interval <= 0 {
		interval = time.Minute
	}
	if staleAfter <= 0 {
		staleAfter = 2 * time.Minute
	}
	if limit <= 0 {
		limit = 50
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		if err := s.SweepStalledCommandBarPlans(ctx, staleAfter, limit); err != nil {
			slog.ErrorContext(ctx, "command bar plan sweep failed", "error", err)
		}
	}
}

// StartReadyCommandBarPlanSteps satisfies temporalapp.CommandBarPlanAdvancer so
// in-flight CommandBarPlanWorkflow histories stay replayable during the drain
// window. New code paths call startReadyCommandBarPlanSteps directly; this
// wrapper is deleted with the temporalapp agent-run machinery.
func (s *AgentService) StartReadyCommandBarPlanSteps(ctx context.Context, input temporalapp.CommandBarPlanWorkflowInput) (*temporalapp.CommandBarPlanProgress, error) {
	progress, err := s.startReadyCommandBarPlanSteps(ctx, CommandBarPlanInput{
		WorkspaceID: input.WorkspaceID,
		ActorID:     input.ActorID,
		PlanID:      input.PlanID,
		Prompt:      input.Prompt,
		PageContext: input.PageContext,
		Steps:       input.Steps,
	})
	if progress == nil || err != nil {
		return nil, err
	}
	return &temporalapp.CommandBarPlanProgress{
		Terminal: progress.Terminal,
		Status:   progress.Status,
		Started:  progress.Started,
	}, nil
}
