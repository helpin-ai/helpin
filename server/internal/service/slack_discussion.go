package service

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/channels/slack"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// SlackPMDeliveryJob delivers only explicitly shared PM discussion replies and status.
const SlackPMDeliveryJob = "slack_pm_delivery"

func (s *SlackService) ingestComment(ctx context.Context, a *model.SupportChannelAccount, token string, m slack.Message) error {
	if m.ThreadTS == "" || m.Text == "" {
		return nil
	}
	d, err := s.repo.Discussion(ctx, a.ID, m.Channel, m.Root())
	if err != nil || d.CommentID == "" {
		return err
	}
	actor, err := s.linkedActor(ctx, a, m.User)
	if err != nil {
		return err
	}
	if actor == nil {
		return s.promptLink(ctx, a, token, m)
	}
	ok, err := s.authz.CanAccessModule(ctx, actor, model.ModulePM)
	if err != nil {
		return err
	}
	if !ok || !s.authz.Can(actor, authorization.PermPMEdit) {
		return nil
	}
	ctx = authorization.WithActor(ctx, actor)
	task, err := s.tasks.GetByID(ctx, d.TaskID)
	if err != nil || task == nil || task.Task.WorkspaceID != a.WorkspaceID {
		return nil
	}
	ctx = model.WithSlackSource(ctx, s.sourceURL(ctx, token, m.Destination(), m.TS))
	key := a.ID + ":" + m.Channel + ":" + m.TS
	_, err = s.comments.Create(ctx, model.CreateCommentRequest{EntityType: "task", EntityID: d.TaskID, ParentID: &d.CommentID, Body: m.Text, SlackSourceKey: &key}, actor.UserID, a.WorkspaceID)
	if err != nil && isUniqueConstraintViolation(err) {
		return nil
	}
	return err
}

func (s *SlackService) audienceCanRead(ctx context.Context, a *model.SupportChannelAccount, token, channel string, task *model.TaskDetail) (bool, error) {
	return s.channelAudience(ctx, a, token, channel, func(actor *authorization.Actor) (bool, error) {
		ok, err := s.authz.CanAccessModule(ctx, actor, model.ModulePM)
		return ok && s.authz.Can(actor, authorization.PermPMRead) && canAccessTeam(authorization.WithActor(ctx, actor), task.Task.TeamID), err
	})
}

// Public channel history is readable by people outside the current membership list.
func (s *SlackService) channelAudience(ctx context.Context, a *model.SupportChannelAccount, token, channel string, canRead func(*authorization.Actor) (bool, error)) (bool, error) {
	var info struct {
		Channel struct {
			IsShared bool `json:"is_ext_shared"`
			Private  bool `json:"is_private"`
			Pending  bool `json:"is_pending_ext_shared"`
			Member   bool `json:"is_member"`
		}
	}
	if err := s.api.Get(ctx, token, "conversations.info", url.Values{"channel": {channel}}, &info); err != nil {
		return false, err
	}
	if info.Channel.IsShared || info.Channel.Pending || !info.Channel.Member || !info.Channel.Private {
		return false, nil
	}
	cursor := ""
	for {
		var page struct {
			Members  []string
			Metadata struct {
				Next string `json:"next_cursor"`
			} `json:"response_metadata"`
		}
		if err := s.api.Get(ctx, token, "conversations.members", url.Values{"channel": {channel}, "limit": {"200"}, "cursor": {cursor}}, &page); err != nil {
			return false, err
		}
		for _, id := range page.Members {
			if id == a.Handle {
				continue
			}
			actor, err := s.linkedActor(ctx, a, id)
			if err != nil {
				return false, err
			}
			if actor == nil {
				return false, nil
			}
			allowed, err := canRead(actor)
			if err != nil || !allowed {
				return false, err
			}
		}
		if page.Metadata.Next == "" {
			return len(page.Members) > 0, nil
		}
		cursor = page.Metadata.Next
	}
}

func (s *SlackService) deliverDiscussion(ctx context.Context, j *model.SupportInboundJob) error {
	var p struct {
		Channel, Root, Task, Comment, Support, Inbox string
		Completed                                    bool
	}
	if err := json.Unmarshal([]byte(j.Payload), &p); err != nil {
		return err
	}
	a, err := s.channels.accounts.GetForWebhook(ctx, "slack", *j.ChannelAccountID)
	if err != nil || a == nil {
		return err
	}
	if p.Inbox != "" {
		return s.deliverInboxTaskReceipt(ctx, a, p.Inbox, p.Task, j.ID)
	}
	if p.Support != "" {
		return s.notifySupportOwner(ctx, a, p.Support, p.Task)
	}
	route, err := s.repo.Route(ctx, a.ID, p.Channel)
	if err != nil || route == nil || route.Purpose != "pm" {
		return err
	}
	d, err := s.repo.Discussion(ctx, a.ID, p.Channel, p.Root)
	if err != nil || d.TaskID != p.Task {
		return err
	}
	task, err := s.tasks.taskRepo.GetByID(ctx, p.Task)
	if err != nil || task == nil || task.Task.WorkspaceID != a.WorkspaceID {
		return err
	}
	token, err := s.channels.decryptToken(a)
	if err != nil {
		return err
	}
	ok, err := s.audienceCanRead(ctx, a, token, route.CurrentChannelID, task)
	if err != nil {
		return err
	}
	if !ok {
		if p.Comment == "" || s.tasks.notificationService == nil {
			return nil
		}
		comment, err := s.comments.Get(ctx, p.Comment)
		if err != nil || comment == nil {
			return err
		}
		return s.tasks.notificationService.Emit(ctx, model.NotificationEventInput{
			WorkspaceID: a.WorkspaceID, EventType: "task.comment_delivery_failed", EntityType: "task", EntityID: p.Task,
			Title: "Slack reply stayed in Helpin", Body: "The channel audience could not be verified. Review the channel membership before sharing.",
			Category: model.NotifCategoryComments, ExplicitRecipients: []string{comment.AuthorID}, SkipFollowers: true,
		})
	}
	label := task.Task.Name
	if task.State != nil {
		label += " · " + task.State.Name
	}
	post := map[string]any{"channel": route.CurrentChannelID, "text": label, "mrkdwn": false, "unfurl_links": false, "unfurl_media": false}
	if p.Comment != "" {
		comment, err := s.comments.Get(ctx, p.Comment)
		if err != nil || comment == nil {
			return err
		}
		if comment.ParentID == nil || *comment.ParentID != d.CommentID || comment.EntityID != d.TaskID {
			return nil
		}
		post["text"] = tiptap.RichTextToMarkdown(comment.Body)
		post["thread_ts"] = p.Root
		post["client_msg_id"] = j.ID
		return s.api.Call(ctx, token, "chat.postMessage", post, nil)
	}
	if d.CardTS == "" {
		post["thread_ts"] = p.Root
		post["client_msg_id"] = inboundStableID(a.ID + p.Channel + p.Root + "card")
		var response struct{ TS string }
		if err = s.api.Call(ctx, token, "chat.postMessage", post, &response); err != nil {
			return err
		}
		if err = s.repo.RecordCard(ctx, a.ID, p.Channel, p.Root, response.TS); err != nil {
			return err
		}
	} else {
		post["ts"] = d.CardTS
		if err = s.api.Call(ctx, token, "chat.update", post, nil); err != nil {
			return err
		}
	}
	if p.Completed && task.Task.Completed {
		return s.api.Call(ctx, token, "chat.postMessage", map[string]any{"channel": route.CurrentChannelID, "thread_ts": p.Root, "text": "Completed: " + task.Task.Name, "mrkdwn": false, "client_msg_id": j.ID}, nil)
	}
	return nil
}

func (s *SlackService) notifySupportOwner(ctx context.Context, a *model.SupportChannelAccount, conversationID, taskID string) error {
	inbox := s.channels.inbox
	if inbox.notificationService == nil {
		return nil
	}
	conv, err := inbox.conversationRepo.GetByID(ctx, a.WorkspaceID, conversationID, "", model.RoleOwner)
	if err != nil || conv == nil {
		return err
	}
	if conv.AnonymizedAt != nil || conv.AssignedUserID == nil || derefString(conv.ChannelAccountID) != a.ID || derefString(conv.LinkedTaskID) != taskID {
		return nil
	}
	task, err := s.tasks.taskRepo.GetRawByID(ctx, taskID)
	if err != nil || task == nil || !task.Completed {
		return err
	}
	return inbox.notificationService.Emit(ctx, model.NotificationEventInput{
		WorkspaceID: a.WorkspaceID, EventType: "support_conversation.linked_task_completed", Category: model.NotifCategorySupportReplies,
		EntityType: "support_conversation", EntityID: conv.ID, Title: "Linked task completed", Body: "Review the conversation and send the customer an update.",
		ExplicitRecipients: []string{*conv.AssignedUserID}, SkipFollowers: true,
	})
}

// Check Slack at delivery time because a channel can become externally shared while the agent works.
func (s *SlackService) reply(ctx context.Context, token string, m slack.Message, text string, mrkdwn bool, blocks ...any) error {
	if slackDirect(m) || strings.HasPrefix(m.Channel, "D") {
		return s.sendPrivate(ctx, token, m, text, mrkdwn, blocks...)
	}
	var info struct {
		Channel struct {
			ID       string
			External *bool `json:"is_ext_shared"`
			Pending  bool  `json:"is_pending_ext_shared"`
			Member   bool  `json:"is_member"`
		}
	}
	err := s.api.Get(ctx, token, "conversations.info", url.Values{"channel": {m.Destination()}}, &info)
	if err != nil {
		s.channels.logger.WarnContext(ctx, "Slack channel visibility unavailable", "error", err)
	}
	channel := info.Channel
	if err != nil || channel.ID != m.Destination() || channel.External == nil || *channel.External || channel.Pending || !channel.Member {
		return s.sendPrivate(ctx, token, m, text, mrkdwn, blocks...)
	}
	post := map[string]any{"channel": m.Destination(), "thread_ts": m.Root(), "text": text, "mrkdwn": mrkdwn, "unfurl_links": false, "unfurl_media": false, "client_msg_id": inboundStableID("slack-response:" + m.Channel + ":" + m.TS + ":" + m.Type)}
	if len(blocks) > 0 {
		post["blocks"] = blocks
	}
	return s.api.Call(ctx, token, "chat.postMessage", post, nil)
}
