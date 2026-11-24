package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/channels/slack"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// Slack's agent-session indicator is shared with the thread. Keep record details
// in the requester's DM or ephemeral answer.
func (s *SlackService) sessionStatus(ctx context.Context, token string, m slack.Message, status string) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := s.api.Call(ctx, token, "agents.sessions.setStatus", map[string]string{"channel_id": m.Destination(), "thread_ts": m.Root(), "status": status}, nil); err != nil {
		s.channels.logger.WarnContext(ctx, "Slack agent status unavailable", "status", status, "error", err)
	}
}

type slackProgressKey struct{}
type slackProgress struct {
	TS      string
	settled bool
}

// In a DM one editable message carries the work label and then the answer.
// Channel members see only Slack's standard working indicator.
func (s *SlackService) startProgress(ctx context.Context, token string, m slack.Message, label string) (context.Context, *slackProgress) {
	progress := &slackProgress{}
	s.sessionStatus(ctx, token, m, "processing")
	if slackDirect(m) {
		var response struct {
			TS string `json:"ts"`
		}
		callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		err := s.api.Call(callCtx, token, "chat.postMessage", map[string]any{"channel": m.Channel, "thread_ts": m.Root(), "text": label, "mrkdwn": false, "client_msg_id": inboundStableID("slack-progress:" + m.Channel + ":" + m.TS)}, &response)
		if err == nil {
			progress.TS = response.TS
		} else {
			s.channels.logger.WarnContext(ctx, "Slack progress message unavailable", "error", err)
		}
	}
	return context.WithValue(ctx, slackProgressKey{}, progress), progress
}

func (s *SlackService) finishProgress(ctx context.Context, token string, m slack.Message, p *slackProgress) {
	if p.TS != "" && !p.settled {
		if err := s.private(ctx, token, m, "I couldn’t complete this request. Please try again."); err != nil {
			s.channels.logger.WarnContext(ctx, "Slack progress cleanup failed", "error", err)
		}
	}
	s.sessionStatus(ctx, token, m, "active")
}

// Labels come only from observed, permitted tool calls. Tool inputs and outputs
// stay inside Helpin.
func slackReadPhase(tools []string) string {
	for i := len(tools) - 1; i >= 0; i-- {
		switch strings.TrimPrefix(tools[i], "mcp__helpin__") {
		case "get_task", "list_tasks", "get_task_context", "list_task_checklist", "list_epic_tasks", "list_sprint_tasks":
			return "Reading tasks…"
		case "list_epics", "get_epic", "list_sprints", "list_workspace_teams", "list_team_workflows_with_stages":
			return "Reading project details…"
		case "list_support_conversations", "get_support_conversation", "list_conversation_messages", "list_support_tags", "list_support_inboxes", "list_support_assignees":
			return "Reading support information…"
		}
	}
	return ""
}

func (s *SlackService) updateReadProgress(ctx context.Context, a *model.SupportChannelAccount, p *slackAgentResult, job *model.SupportInboundJob) error {
	if p.ProgressTS == "" {
		return nil
	}
	tools, err := s.repo.AgentTurnTools(ctx, a.WorkspaceID, p.ChatID, p.TurnID)
	if err != nil {
		return err
	}
	phase := slackReadPhase(tools)
	if phase == "" || phase == p.LastPhase {
		return nil
	}
	token, err := s.channels.decryptToken(a)
	if err != nil {
		return err
	}
	// Store the attempted phase even when Slack rate-limits it. The next poll
	// prioritizes delivery of the final answer.
	p.LastPhase = phase
	payload, err := json.Marshal(p)
	if err != nil {
		return err
	}
	job.Payload = string(payload)
	return s.private(context.WithValue(ctx, slackProgressKey{}, &slackProgress{TS: p.ProgressTS}), token, p.Message, phase)
}
