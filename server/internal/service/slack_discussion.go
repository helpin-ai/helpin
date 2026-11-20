package service

import (
	"context"
	"encoding/json"
	"net/url"

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
		Channel, Root, Task, Comment, Support string
		Completed                             bool
	}
	if err := json.Unmarshal([]byte(j.Payload), &p); err != nil {
		return err
	}
	a, err := s.channels.accounts.GetForWebhook(ctx, "slack", *j.ChannelAccountID)
	if err != nil || a == nil {
		return err
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
	ok, err := s.audienceCanRead(ctx, a, token, p.Channel, task)
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
	post := map[string]any{"channel": p.Channel, "text": label, "mrkdwn": false, "unfurl_links": false, "unfurl_media": false}
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
		return s.api.Call(ctx, token, "chat.postMessage", map[string]any{"channel": p.Channel, "thread_ts": p.Root, "text": "Completed: " + task.Task.Name, "mrkdwn": false, "client_msg_id": j.ID}, nil)
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

func (s *SlackService) shareAnswer(ctx context.Context, a *model.SupportChannelAccount, token string, m slack.Message, info string, allowed bool) error {
	if !allowed {
		return s.private(ctx, token, m, "Only you can see this answer. Sharing requires a private internal channel where everyone can read the record.\n"+truncate(info, 2500))
	}
	return s.api.Call(ctx, token, "chat.postMessage", map[string]any{"channel": m.Channel, "thread_ts": m.Root(), "text": truncate(info, 2800), "mrkdwn": false, "unfurl_links": false, "unfurl_media": false, "client_msg_id": inboundStableID(a.ID + m.Channel + m.TS + "share")}, nil)
}
