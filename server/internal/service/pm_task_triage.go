package service

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// SetTriageService enables bounded task-created and task-edited decisions.
func (s *PMTaskService) SetTriageService(triage *PMTriageService) { s.triageService = triage }

func (s *PMTaskService) triageTask(ctx context.Context, workspaceID, taskID string) bool {
	if s.triageService == nil || !s.triageService.enabled(workspaceID) || authorization.GetActor(ctx) == nil {
		return false
	}
	added, err := s.triageService.automaticTask(ctx, workspaceID, taskID)
	if err != nil {
		s.logger.ErrorContext(ctx, "triage task", "error", err, "workspace_id", workspaceID, "task_id", taskID)
		return false
	}
	if len(added) == 0 {
		return false
	}
	actor := authorization.GetActor(ctx)
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "task", EntityID: taskID, WorkspaceID: workspaceID, ActorID: actor.UserID})
	return true
}

func (s *PMTriageService) automaticTask(ctx context.Context, workspaceID, taskID string) ([]string, error) {
	view, err := s.Analyze(ctx, workspaceID, "task", taskID)
	if err != nil {
		return nil, err
	}
	if view.Status != "ready" || view.Assessment == nil || len(view.Assessment.Labels) == 0 {
		return nil, nil
	}
	source, err := s.loadSource(ctx, workspaceID, "task", taskID)
	if err != nil {
		return nil, err
	}
	if source.hash != view.SourceHash {
		return nil, nil
	}
	task, err := s.tasks.GetRawByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil || pmTriageHash(task.Name+"\n"+pmTriageText(task.Description)+"\n"+task.UpdatedAt.String()) != view.SourceHash {
		return nil, nil
	}
	actor := authorization.GetActor(ctx)
	ids := make([]string, 0, len(view.Assessment.Labels))
	for _, label := range view.Assessment.Labels {
		ids = append(ids, label.ID)
	}
	return s.tasks.ApplyTriageLabels(ctx, repository.PMTriageScope{WorkspaceID: workspaceID, TeamIDs: actor.TeamIDs(), AllTeams: isPrivileged(actor)}, taskID, actor.UserID, view.ID, task.UpdatedAt, ids)
}
