package service

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const assignmentRunTimeout = 2 * time.Minute

// assignmentRunTracker tracks background starts of auto_on_assignment runs so
// shutdown and tests can wait for them.
type assignmentRunTracker struct {
	wg sync.WaitGroup
}

// WaitForAssignmentRuns blocks until dispatched assignment runs have started
// or failed.
func (s *PMTaskService) WaitForAssignmentRuns() {
	s.assignmentRuns.wg.Wait()
}

// dispatchAssignmentRun starts a run for an agent with trigger_mode
// auto_on_assignment after a task is assigned to it. Starting a run calls
// Agent Runtime, so it runs after the task write commits and outside the
// request; failures are logged and never undo the assignment.
func (s *PMTaskService) dispatchAssignmentRun(ctx context.Context, workspaceID, taskID string, agentID *string, actorID string) {
	if s == nil || s.agentService == nil || agentID == nil || strings.TrimSpace(*agentID) == "" {
		return
	}
	assigned := strings.TrimSpace(*agentID)
	s.assignmentRuns.wg.Add(1)
	go func() {
		defer s.assignmentRuns.wg.Done()
		runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), assignmentRunTimeout)
		defer cancel()
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("assignment run panic", "panic", recovered, "task_id", taskID)
			}
		}()
		run, err := s.agentService.StartAssignmentRun(runCtx, workspaceID, taskID, assigned, actorID)
		if err != nil {
			s.logger.WarnContext(runCtx, "start agent run on assignment", "workspace_id", workspaceID, "task_id", taskID, "agent_id", assigned, "error", err)
			return
		}
		if run != nil {
			s.logger.InfoContext(runCtx, "started agent run on assignment", "workspace_id", workspaceID, "task_id", taskID, "agent_id", assigned, "run_id", run.ID)
		}
	}()
}
