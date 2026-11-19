package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	agentruntime "github.com/helpin-ai/agent-runtime-go"
)

// SubtaskJob is the progress of one runtime subtask job spawned by an agent run.
type SubtaskJob struct {
	ID          string                     `json:"id"`
	RunID       string                     `json:"run_id"`
	Objective   string                     `json:"objective"`
	State       string                     `json:"state"`
	Error       string                     `json:"error,omitempty"`
	Counts      agentruntime.WorkJobCounts `json:"counts"`
	Usage       agentruntime.WorkJobUsage  `json:"usage"`
	CreatedAt   time.Time                  `json:"created_at"`
	CompletedAt *time.Time                 `json:"completed_at,omitempty"`
}

type subtaskJobReader interface {
	GetWorkJob(ctx context.Context, jobID string) (*agentruntime.WorkJob, error)
}

// GetSubtaskJob returns a subtask job when the run that spawned it belongs to
// the workspace. The parent-run check is the access boundary: job ids are not
// workspace-scoped in the runtime.
func (s *AgentService) GetSubtaskJob(ctx context.Context, workspaceID, jobID string) (*SubtaskJob, error) {
	jobID = strings.TrimSpace(jobID)
	reader, ok := s.agentRuntimeClient.(subtaskJobReader)
	if !ok || s.runRepo == nil || jobID == "" {
		return nil, fmt.Errorf("subtask jobs are not available")
	}
	job, err := reader.GetWorkJob(ctx, jobID)
	if err != nil || job == nil || strings.TrimSpace(job.ParentRunID) == "" {
		return nil, fmt.Errorf("subtask job not found")
	}
	run, err := s.runRepo.GetByExternalRuntimeID(ctx, agentRuntimeName, strings.TrimSpace(job.ParentRunID))
	if err != nil || run == nil || run.WorkspaceID != strings.TrimSpace(workspaceID) {
		return nil, fmt.Errorf("subtask job not found")
	}
	return &SubtaskJob{
		ID:          job.ID,
		RunID:       run.ID,
		Objective:   job.Spec.Objective,
		State:       job.State,
		Error:       job.Error,
		Counts:      job.Counts,
		Usage:       job.Usage,
		CreatedAt:   job.CreatedAt,
		CompletedAt: job.CompletedAt,
	}, nil
}
