package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Sentinel launch-precondition errors. They let callers such as the public
// MCP boundary return an actionable, typed failure while preserving the
// original human-readable message shown by the app.
var (
	// ErrAgentRunTargetNotFound means the run target does not exist in the workspace.
	ErrAgentRunTargetNotFound = errors.New("agent run target not found")
	// ErrAgentRunTargetNotAllowed means the agent may not run on this target type or team.
	ErrAgentRunTargetNotAllowed = errors.New("agent is not allowed to run on this target")
	// ErrAgentRunTargetBusy means another agent already has an active run on the target.
	ErrAgentRunTargetBusy = errors.New("an agent run is already active for this target")
)

// AgentRunPreconditionError keeps a launch failure's product message while
// classifying it under one of the sentinel errors above.
type AgentRunPreconditionError struct {
	Kind    error
	Message string
}

// Error implements error.
func (e *AgentRunPreconditionError) Error() string { return e.Message }

// Unwrap exposes the sentinel classification to errors.Is.
func (e *AgentRunPreconditionError) Unwrap() error { return e.Kind }

func agentRunPreconditionError(kind error, format string, args ...any) error {
	return &AgentRunPreconditionError{Kind: kind, Message: fmt.Sprintf(format, args...)}
}

// SelectTaskRunRepository sets a task's delivery repository before a run, the
// same choice the app's repository picker makes. It first confirms the agent
// may run on the task so a rejected launch never changes the task.
func (s *AgentService) SelectTaskRunRepository(ctx context.Context, workspaceID, taskID, agentID, repositoryID string, baseBranch *string, actorID string) error {
	if s.gitService == nil {
		return fmt.Errorf("repository delivery is not configured")
	}
	task, err := s.taskRepo.GetRawByID(ctx, strings.TrimSpace(taskID))
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}
	if task == nil || task.WorkspaceID != workspaceID {
		return agentRunPreconditionError(ErrAgentRunTargetNotFound, "task not found")
	}
	if err := s.ValidateRunnableTargetAgent(ctx, workspaceID, agentID, "task", task.TeamID); err != nil {
		return err
	}
	repositoryID = strings.TrimSpace(repositoryID)
	_, err = s.gitService.UpdateTaskDeliveryTarget(ctx, workspaceID, task.ID, model.UpdateTaskDeliveryTargetRequest{
		RepositoryID: &repositoryID,
		BaseBranch:   baseBranch,
	}, actorID)
	return err
}
