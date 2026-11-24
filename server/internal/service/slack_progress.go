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
func (s *SlackService) sessionStatus(ctx context.Context, token string, m slack.Message, status string) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := s.api.Call(ctx, token, "agents.sessions.setStatus", map[string]string{"channel_id": m.Destination(), "thread_ts": m.Root(), "status": status}, nil); err != nil {
		s.channels.logger.WarnContext(ctx, "Slack agent status unavailable", "status", status, "error", err)
		return false
	}
	return true
}

type slackProgressKey struct{}
type slackProgress struct {
	Native  bool
	TS      string
	settled bool
}

// Use one indicator. In DMs without native status, edit the fallback message into the answer.
func (s *SlackService) startProgress(ctx context.Context, token string, m slack.Message, label string) (context.Context, *slackProgress) {
	progress := &slackProgress{Native: s.sessionStatus(ctx, token, m, "processing")}
	if !progress.Native && slackDirect(m) && label != "" {
		ts, err := s.postProgress(ctx, token, m, label)
		if err != nil {
			s.channels.logger.WarnContext(ctx, "Slack progress message unavailable", "error", err)
		} else {
			progress.TS = ts
		}
	}
	return context.WithValue(ctx, slackProgressKey{}, progress), progress
}

func (s *SlackService) postProgress(ctx context.Context, token string, m slack.Message, label string) (string, error) {
	var response struct {
		TS string `json:"ts"`
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	err := s.api.Call(ctx, token, "chat.postMessage", map[string]any{"channel": m.Channel, "thread_ts": m.Root(), "text": label, "mrkdwn": false, "client_msg_id": inboundStableID("slack-progress:" + m.Channel + ":" + m.TS)}, &response)
	return response.TS, err
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
	if p.NativeProgress || !slackDirect(p.Message) {
		return nil
	}
	tools, err := s.repo.AgentTurnTools(ctx, a.WorkspaceID, p.ChatID, p.TurnID)
	if err != nil {
		return err
	}
	phase := slackReadPhase(tools)
	if phase == "" && p.LastPhase == "" && s.channels.now().Sub(job.CreatedAt) >= 3*time.Second {
		phase = "Working…"
	}
	if phase == "" || phase == p.LastPhase {
		return nil
	}
	token, err := s.channels.decryptToken(a)
	if err != nil {
		return err
	}
	if p.ProgressTS == "" {
		p.ProgressTS, err = s.postProgress(ctx, token, p.Message, phase)
	} else {
		err = s.private(context.WithValue(ctx, slackProgressKey{}, &slackProgress{TS: p.ProgressTS}), token, p.Message, phase)
	}
	// Avoid repeating an activity update on every poll, including after a rate limit.
	p.LastPhase = phase
	payload, marshalErr := json.Marshal(p)
	if marshalErr != nil {
		return marshalErr
	}
	job.Payload = string(payload)
	return err
}

// After a failed retry, surface an actionable outcome to the requester. Ordinary
// inbound customer messages continue to use the support delivery retry policy.
func (s *SlackService) reportRequestFailure(ctx context.Context, job *model.SupportInboundJob) bool {
	if job.ChannelAccountID == nil || job.Attempts < 2 || s.channels.now().Sub(job.CreatedAt) < time.Minute {
		return false
	}
	var message slack.Message
	var progressTS string
	switch job.Kind {
	case SlackAgentReplyJob:
		var p slackAgentResult
		if json.Unmarshal([]byte(job.Payload), &p) != nil {
			return false
		}
		message, progressTS = p.Message, p.ProgressTS
	case SlackEventJob:
		if json.Unmarshal([]byte(job.Payload), &message) != nil {
			return false
		}
	default:
		return false
	}
	a, err := s.channels.accounts.GetForWebhook(ctx, "slack", *job.ChannelAccountID)
	if err != nil || a == nil {
		return false
	}
	if job.Kind == SlackEventJob && !slackDirect(message) && message.Type != "app_mention" && !s.mentionsBot(message.Text, a.Handle) {
		return false
	}
	token, err := s.channels.decryptToken(a)
	if err != nil {
		return false
	}
	progressCtx := context.WithValue(ctx, slackProgressKey{}, &slackProgress{TS: progressTS})
	err = s.private(progressCtx, token, message, "I couldn’t confirm the result of this request. Check the record in Helpin before retrying a change. If it keeps happening, share this thread with Helpin support.")
	s.sessionStatus(ctx, token, message, "active")
	if err != nil {
		s.channels.logger.WarnContext(ctx, "Slack failure notice unavailable", "job_id", job.ID, "error", err)
	}
	return err == nil
}
