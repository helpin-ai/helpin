package service

import (
	"context"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

const agentRunTriggerTypeTaskCommentMention = "task_comment_mention"

// externalA2ACommentTarget is an external agent a human comment addresses.
type externalA2ACommentTarget struct {
	agent     model.Agent
	mentioned bool
}

// RouteTaskComment forwards a human task comment to the external agents it
// addresses. The task's assigned external agent, a mentioned one, or the
// author of the comment being replied to receives the text as the answer when
// its latest run on the task is waiting for input. A mention with no active
// run starts a new run, which continues the stored remote conversation.
func (s *ExternalA2AService) RouteTaskComment(ctx context.Context, workspaceID string, comment *model.PMComment) {
	if !s.Enabled() || comment == nil || comment.EntityType != "task" || comment.AgentID != nil {
		return
	}
	text := strings.TrimSpace(tiptap.StripHTML(comment.Body))
	if text == "" {
		return
	}
	targets, err := s.commentTargets(ctx, workspaceID, comment)
	if err != nil {
		slog.WarnContext(ctx, "resolve external agents for comment", "workspace_id", workspaceID, "comment_id", comment.ID, "error", err)
		return
	}
	for _, target := range targets {
		s.routeCommentToAgent(ctx, workspaceID, comment, text, target)
	}
}

func (s *ExternalA2AService) commentTargets(ctx context.Context, workspaceID string, comment *model.PMComment) ([]externalA2ACommentTarget, error) {
	agents, err := s.repo.ListLinkedAgents(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	external := make(map[string]model.Agent, len(agents))
	for _, agent := range agents {
		external[agent.ID] = agent
	}
	if len(external) == 0 {
		return nil, nil
	}
	addressed := map[string]bool{}
	mentions := map[string]struct{}{}
	for _, handle := range extractMentions(comment.Body) {
		mentions[handle] = struct{}{}
	}
	for id, agent := range external {
		for _, handle := range mentionHandleVariants(agent.Name) {
			if _, ok := mentions[handle]; ok {
				addressed[id] = true
			}
		}
	}
	if task, err := s.agents.taskRepo.GetRawByID(ctx, comment.EntityID); err != nil {
		return nil, err
	} else if task != nil && task.AssignedAgentID != nil {
		if _, ok := external[*task.AssignedAgentID]; ok && !addressed[*task.AssignedAgentID] {
			addressed[*task.AssignedAgentID] = false
		}
	}
	if comment.ParentID != nil && s.comments != nil {
		if parent, err := s.comments.Get(ctx, *comment.ParentID); err == nil && parent != nil && parent.AgentID != nil {
			if _, ok := external[*parent.AgentID]; ok && !addressed[*parent.AgentID] {
				addressed[*parent.AgentID] = false
			}
		}
	}
	targets := make([]externalA2ACommentTarget, 0, len(addressed))
	for id, mentioned := range addressed {
		targets = append(targets, externalA2ACommentTarget{agent: external[id], mentioned: mentioned})
	}
	return targets, nil
}

func (s *ExternalA2AService) routeCommentToAgent(ctx context.Context, workspaceID string, comment *model.PMComment, text string, target externalA2ACommentTarget) {
	record, err := s.repo.GetByAgentID(ctx, workspaceID, target.agent.ID)
	if err != nil || record == nil || record.Status != model.ExternalA2AStatusActive {
		return
	}
	runs, err := s.agents.runRepo.ListByTarget(ctx, workspaceID, "task", comment.EntityID)
	if err != nil {
		slog.WarnContext(ctx, "list task runs for comment", "workspace_id", workspaceID, "task_id", comment.EntityID, "error", err)
		return
	}
	var latest *model.AgentRun
	for i := range runs {
		if runs[i].AgentID == target.agent.ID {
			latest = &runs[i]
			break
		}
	}
	authorID, commentID, taskID, agentID := comment.AuthorID, comment.ID, comment.EntityID, target.agent.ID
	switch {
	case latest != nil && externalA2ARunAwaitsReply(latest):
		runID := latest.ID
		s.dispatch(ctx, func(ctx context.Context) {
			if _, err := s.agents.SendRunMessage(ctx, workspaceID, runID, authorID, model.SendAgentRunMessageRequest{Content: text, ClientMessageID: commentID}); err != nil {
				slog.WarnContext(ctx, "resume external agent from comment", "workspace_id", workspaceID, "run_id", runID, "comment_id", commentID, "error", err)
			}
		})
	case target.mentioned && (latest == nil || !model.IsAgentRunActiveStatus(latest.Status)):
		additional := "A teammate mentioned you in a comment on this task:\n" + text
		s.dispatch(ctx, func(ctx context.Context) {
			if _, err := s.agents.startTargetRun(ctx, workspaceID, "task", taskID, model.StartAgentRunRequest{AgentID: agentID, AdditionalContext: &additional},
				&authorID, systemRunTriggerContext(agentRunTriggerTypeTaskCommentMention), nil, nil); err != nil {
				slog.WarnContext(ctx, "start external agent from comment", "workspace_id", workspaceID, "task_id", taskID, "agent_id", agentID, "error", err)
			}
		})
	}
}

// externalA2ARunAwaitsReply reports whether a run is paused for a human answer
// (not for approval, authentication, or a manual pause).
func externalA2ARunAwaitsReply(run *model.AgentRun) bool {
	if run == nil || !model.IsAgentRunPausedStatus(run.Status) {
		return false
	}
	switch run.PauseReason {
	case model.AgentRunPauseReasonHumanInput, model.AgentRunPauseReasonUserMessage:
		return true
	default:
		return false
	}
}
