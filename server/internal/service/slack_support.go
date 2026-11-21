package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/channels/slack"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// supportCommand returns true when a request was handled privately or as a factual customer update.
func (s *SlackService) supportCommand(ctx context.Context, a *model.SupportChannelAccount, token string, route *model.SlackRoute, m slack.Message, teammate bool) (bool, error) {
	q := strings.ToLower(m.Text)
	link := slackCommand(m.Text) == "link"
	draft := (strings.Contains(q, "creat") && strings.Contains(q, "task")) || strings.Contains(q, "bug")
	summary := strings.Contains(q, "summar") || strings.Contains(q, "recap")
	update := strings.Contains(q, "update") || strings.Contains(q, "status")
	if !draft && !summary && !update && !link && !(teammate && m.ThreadTS != "") {
		return false, nil
	}
	lookup := s.repo.SupportThread
	if !teammate && update {
		lookup = s.repo.SupportThreadOrigin
	}
	conv, err := lookup(ctx, a, m.Channel+":"+m.Root())
	if err != nil {
		return true, err
	}
	if conv == nil {
		return false, nil
	}
	if !teammate {
		if !update {
			return true, s.private(ctx, token, m, "You can ask for an update in this support thread.")
		}
		reply, err := s.repo.PublishedReply(ctx, a, conv.ID)
		if err != nil {
			return true, err
		}
		text := "There’s no published update yet."
		if reply.ID != "" {
			text = "Latest published update: " + truncate(tiptap.RichTextToMarkdown(reply.Content), 2600)
		}
		ownerConversation := conv
		if conv.MergedIntoID != nil {
			inbox := s.channels.inbox
			root, err := inbox.mergeRepository().Root(ctx, a.WorkspaceID, conv.ID)
			if err != nil {
				return true, err
			}
			ownerConversation, err = inbox.conversationRepo.GetByID(ctx, a.WorkspaceID, root, "", model.RoleOwner)
			if err != nil {
				return true, err
			}
			if ownerConversation == nil || ownerConversation.AnonymizedAt != nil {
				return true, nil
			}
		}
		first, err := s.repo.RecordUpdateRequest(ctx, conv.ID, reply.ID)
		if err != nil {
			return true, err
		}
		if first && s.channels.inbox.notificationService != nil && ownerConversation.AssignedUserID != nil {
			err = s.channels.inbox.notificationService.Emit(ctx, model.NotificationEventInput{WorkspaceID: a.WorkspaceID, EventType: "support_conversation.customer_reply", EntityType: "support_conversation", EntityID: ownerConversation.ID, Title: "Customer requested a Slack update", Body: ownerConversation.Subject, Category: model.NotifCategorySupportReplies, ExplicitRecipients: []string{*ownerConversation.AssignedUserID}, SkipFollowers: true})
			if err != nil {
				return true, err
			}
		}
		return true, s.api.Call(ctx, token, "chat.postMessage", map[string]any{"channel": m.Destination(), "thread_ts": m.Root(), "text": text, "mrkdwn": false, "unfurl_links": false, "unfurl_media": false, "client_msg_id": inboundStableID(a.ID + ":" + m.Channel + ":" + m.TS + ":status")}, nil)
	}
	actor, err := s.linkedActor(ctx, a, m.User)
	if err != nil {
		return true, err
	}
	if actor == nil {
		return true, s.promptLink(ctx, a, token, m)
	}
	if link {
		ctx = authorization.WithActor(ctx, actor)
		pm, err := s.authz.CanAccessModule(ctx, actor, model.ModulePM)
		if err != nil {
			return true, err
		}
		support, err := s.authz.CanAccessModule(ctx, actor, model.ModuleSupport)
		if err != nil {
			return true, err
		}
		if !pm || !support || !s.authz.Can(actor, authorization.PermPMEdit) || !s.authz.Can(actor, authorization.PermSupportEdit) {
			return true, s.private(ctx, token, m, "You need access to both Support and PM.")
		}
		key := slackTaskKey.FindStringSubmatch(m.Text)
		if len(key) < 3 || !strings.EqualFold(key[1], s.tasks.GetWorkspaceKey(ctx, a.WorkspaceID)) {
			return true, s.private(ctx, token, m, "Include a task key from this workspace.")
		}
		n, err := strconv.Atoi(key[2])
		if err != nil {
			return true, err
		}
		task, err := s.tasks.GetByDisplayID(ctx, a.WorkspaceID, n)
		if err != nil || task == nil {
			return true, s.private(ctx, token, m, "This task is unavailable.")
		}
		if err = s.channels.inbox.LinkConversationTask(ctx, a.WorkspaceID, conv.ID, task.Task.ID, actor.UserID); err != nil {
			return true, err
		}
		return true, s.private(ctx, token, m, "Linked "+task.Task.TaskKey+" to this support conversation.")
	}
	return true, s.internalSupportAnswer(ctx, a, token, route, m, actor, conv.ID, draft)
}

func (s *SlackService) internalSupportAnswer(ctx context.Context, a *model.SupportChannelAccount, token string, route *model.SlackRoute, m slack.Message, actor *authorization.Actor, id string, draft bool) error {
	ok, err := s.authz.CanAccessModule(ctx, actor, model.ModuleSupport)
	if err != nil {
		return err
	}
	if !ok || !s.authz.Can(actor, authorization.PermSupportRead) {
		return s.private(ctx, token, m, "This conversation is unavailable.")
	}
	ctx = authorization.WithActor(ctx, actor)
	inbox := s.channels.inbox
	conv, err := inbox.loadConversationAccessible(ctx, a.WorkspaceID, id)
	if err != nil || conv == nil || conv.AnonymizedAt != nil {
		return s.private(ctx, token, m, "This conversation is unavailable.")
	}
	if draft {
		if !s.authz.Can(actor, authorization.PermSupportEdit) {
			return s.private(ctx, token, m, "You need Support edit permission to link a task.")
		}
		return s.offerDraft(ctx, a, token, route, m, actor, conv)
	}
	messages, err := inbox.ListConversationMessages(ctx, a.WorkspaceID, conv.ID, true)
	if err != nil {
		return err
	}
	info := fmt.Sprintf("%s · %s", conv.Subject, conv.Status)
	var lines []string
	for _, msg := range messages {
		if msg.DeletedAt.Valid || msg.MessageType == "system" {
			continue
		}
		content := strings.TrimSpace(tiptap.RichTextToMarkdown(msg.Content))
		if content != "" {
			lines = append(lines, msg.SenderType+": "+truncate(content, 900))
		}
	}
	if len(lines) > 12 {
		lines = lines[len(lines)-12:]
	}
	if len(lines) > 0 {
		info += "\n" + lines[len(lines)-1]
	}
	if ai := inbox.supportAIService; !a.AIRepliesDisabled && ai != nil && ai.llmProvider != nil {
		evidence, err := json.Marshal(map[string]any{"question": m.Text, "subject": conv.Subject, "status": conv.Status, "messages": lines})
		if err != nil {
			return err
		}
		result, err := s.ask(ctx, a, m, evidence, "Answer the teammate's question about this support conversation. For a summary, include the report, progress, and next action if recorded.")

		if err == nil && result != nil {
			info = conv.Subject + " · " + conv.Status + "\n" + result.Content
		}
	}
	ws, err := s.tasks.workspaceRepo.GetByID(ctx, a.WorkspaceID)
	if err != nil {
		return err
	}
	if route.Purpose == "pm" && slackCommand(m.Text) == "share" {
		allowed, err := s.channelAudience(ctx, a, token, m.Destination(), func(viewer *authorization.Actor) (bool, error) {
			ok, err := s.authz.CanAccessModule(ctx, viewer, model.ModuleSupport)
			if err != nil || !ok || !s.authz.Can(viewer, authorization.PermSupportRead) {
				return false, err
			}
			c, err := inbox.loadConversationAccessible(authorization.WithActor(ctx, viewer), a.WorkspaceID, id)
			return err == nil && c != nil && c.AnonymizedAt == nil, nil
		})
		if err != nil {
			return err
		}
		return s.shareAnswer(ctx, a, token, m, info, allowed)
	}
	return s.private(ctx, token, m, info, map[string]any{"type": "section", "text": slackText(truncate(info, 2800))}, slackButtons(map[string]any{"type": "button", "text": slackText("Open conversation"), "url": s.appURL + "/w/" + ws.Slug + "/support/" + conv.ID, "action_id": "open_support"}))
}

// SetAI updates the explicit Slack assistance setting on the installation.
func (s *SlackService) SetAI(ctx context.Context, ws string, enabled bool) error {
	a, err := s.repo.Installation(ctx, ws)
	if err != nil {
		return err
	}
	if a == nil {
		return ErrChannelInvalid
	}
	return s.channels.accounts.Update(ctx, ws, a.ID, map[string]any{"ai_replies_disabled": !enabled})
}

// MessageSource resolves the original inbound message, including when displayed in a merged inbox thread.
func (s *SlackService) MessageSource(ctx context.Context, workspaceID, messageID string) (string, error) {
	actor := supportActorFromContext(ctx, workspaceID)
	if actor == nil || actor.WorkspaceMemberID == "" {
		return "", nil
	}
	inbox := s.channels.inbox
	msg, err := inbox.messageRepo.GetByID(ctx, messageID)
	if err != nil || msg == nil || msg.WorkspaceID != workspaceID {
		return "", err
	}
	if msg.SenderType != "customer" || msg.IsInternal || msg.MessageType != "reply" || derefString(msg.ViaChannel) != "slack" || msg.ChannelAccountID == nil {
		return "", nil
	}
	channel, ts, ok := strings.Cut(derefString(msg.ExternalMessageKey), ":")
	if !ok || channel == "" || ts == "" {
		return "", nil
	}
	conv, err := inbox.loadConversationAccessible(ctx, workspaceID, msg.ConversationID)
	if err != nil || conv == nil || conv.AnonymizedAt != nil {
		return "", err
	}
	if origin := derefString(msg.OriginConversationID); origin != "" && origin != conv.ID {
		original, err := inbox.loadConversationUnscoped(ctx, workspaceID, origin)
		if err != nil || original == nil || original.AnonymizedAt != nil {
			return "", err
		}
	}
	account, err := s.repo.Installation(ctx, workspaceID)
	if err != nil || account == nil || account.ID != *msg.ChannelAccountID || account.Status != model.SupportChannelAccountActive {
		return "", err
	}
	token, err := s.channels.decryptToken(account)
	if err != nil {
		return "", err
	}
	channel, err = s.repo.CurrentChannelID(ctx, account.ID, channel)
	if err != nil {
		return "", err
	}
	return s.api.MessagePermalink(ctx, token, channel, ts)
}
