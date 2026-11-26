package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/channels/slack"
	"github.com/helpin-ai/helpin/server/internal/model"
)

var inboxCreatePattern = regexp.MustCompile(`(?is)^(?:(?:can|could|would) you\s+)?(?:please\s+)?(?:create|file|open|draft|make)\s+((?:(?:a|an|new|separate|linked)\s+)*)(support\s+(?:ticket|request)|ticket|bug|task)\b\s*[:–-]?\s*(?:(?:for|about|to track)\s+)?(.*)$`)

// Only an imperative addressed to this installation interrupts normal inbox note routing.
func inboxCreateCommand(text, bot string) (string, string) {
	cmd, mentioned := strings.CutPrefix(strings.TrimSpace(text), "<@"+bot+">")
	if bot == "" || !mentioned {
		return "", ""
	}
	match := inboxCreatePattern.FindStringSubmatch(strings.TrimLeft(cmd, " ,:\t\n"))
	if match == nil {
		return "", ""
	}
	kind := strings.ToLower(match[2])
	if strings.HasPrefix(kind, "support") || (kind == "ticket" && slices.Contains(strings.Fields(strings.ToLower(match[1])), "separate")) {
		kind = "support"
	}
	return kind, strings.TrimSpace(match[3])
}

// Account and user are enforced by the short-lived draft store. Only the selected
// message supplies content for a staff ticket, never the rest of the Slack thread.
type inboxTicketDraft struct {
	Channel, Root, TS, SourceURL, Details, Conversation string
}

type inboxCreateInteraction struct {
	Action, Draft, View, Mailbox, Subject, Description string
}

func (s *SlackService) inboxCreateNotice(ctx context.Context, token string, m slack.Message, view, message string) error {
	if view == "" {
		return s.private(ctx, token, m, message)
	}
	return s.api.Call(ctx, token, "views.update", map[string]any{"view_id": view, "view": feedNotice("Helpin", message)}, nil)
}

// The requester also gets a direct link in the review modal. Share a thread
// receipt only when the channel can still read both the inbox and the PM team.
func (s *SlackService) deliverInboxTaskReceipt(ctx context.Context, a *model.SupportChannelAccount, threadID, taskID, receiptID string) error {
	t, err := s.repo.FeedThread(ctx, a.ID, threadID)
	if err != nil {
		return err
	}
	f, err := s.loadFeed(ctx, a, t)
	if err != nil || f == nil || f.conv.Status == "spam" || derefString(f.conv.LinkedTaskID) != taskID {
		return err
	}
	task, err := s.tasks.taskRepo.GetByID(ctx, taskID)
	if err != nil || task == nil || task.Task.WorkspaceID != a.WorkspaceID {
		return err
	}
	ok, err := s.audienceCanRead(ctx, a, f.token, f.channel, task)
	if err != nil || !ok {
		return err
	}
	s.tasks.populateTaskKey(ctx, &task.Task)
	label := "Linked " + task.Task.TaskKey + " · " + truncate(task.Task.Name, 160)
	button := map[string]any{"type": "button", "action_id": "open_task", "text": slackText("Open task in Helpin"), "url": s.appURL + "/w/" + t.WorkspaceSlug + "/pm/tasks/" + taskID}
	return s.api.Call(ctx, f.token, "chat.postMessage", map[string]any{"channel": f.channel, "thread_ts": t.RootTS, "text": label, "mrkdwn": false, "unfurl_links": false, "unfurl_media": false, "client_msg_id": receiptID,
		"blocks": []any{map[string]any{"type": "section", "text": slackText(label)}, slackButtons(button)}}, nil)
}

func (s *SlackService) inboxTicketAction(ctx context.Context, v SlackAction, action, id string) (any, error) {
	if !isUUIDString(id) {
		return nil, ErrChannelInvalid
	}
	a, err := s.channels.accounts.FindLive(ctx, "slack", v.Team.ID)
	if err != nil || a == nil {
		return nil, err
	}
	p := inboxCreateInteraction{Action: action, Draft: id}
	if action == "inbox_ticket_save" {
		values := v.View.State.Values
		p.Subject = strings.TrimSpace(values["subject"]["value"].Value)
		p.Description = strings.TrimSpace(values["description"]["value"].Value)
		p.Mailbox = values["inbox"]["value"].Selected.Value
		if p.Subject == "" {
			return map[string]any{"response_action": "errors", "errors": map[string]string{"subject": "Enter a subject."}}, nil
		}
	}
	return s.queueInboxInteraction(ctx, a, v, slack.Message{Channel: v.Channel.ID}, p)
}

func (s *SlackService) queueInboxInteraction(ctx context.Context, a *model.SupportChannelAccount, v SlackAction, m slack.Message, p inboxCreateInteraction) (any, error) {
	key := v.Trigger
	if p.Action == "inbox_ticket_save" {
		p.View = v.View.ID
		key = p.Draft + ":" + v.User.ID + ":" + p.View
	} else {
		token, err := s.channels.decryptToken(a)
		if err != nil {
			return nil, err
		}
		var opened struct{ View struct{ ID string } }
		if err = s.api.Call(ctx, token, "views.open", map[string]any{"trigger_id": v.Trigger, "view": feedNotice("Helpin", "Opening draft…")}, &opened); err != nil {
			return nil, err
		}
		p.View = opened.View.ID
	}
	if p.View == "" {
		return nil, ErrChannelInvalid
	}
	data, err := json.Marshal(slackShortcutJob{Requester: v.User.ID, Message: m, Inbox: &p})
	if err != nil {
		return nil, err
	}
	job := &model.SupportInboundJob{ID: inboundStableID("slack-inbox-review:" + a.ID + ":" + key), WorkspaceID: &a.WorkspaceID, ChannelAccountID: &a.ID, Kind: "slack_shortcut", Payload: string(data), Status: "pending", AvailableAt: s.channels.now()}
	if err = s.channels.jobs.Create(ctx, job); err != nil {
		return nil, err
	}
	if p.Action == "inbox_ticket_save" {
		return map[string]any{"response_action": "update", "view": feedNotice("Helpin", "Creating ticket…")}, nil
	}
	return map[string]any{}, nil
}

func (s *SlackService) beginInboxCreate(ctx context.Context, a *model.SupportChannelAccount, token string, route *model.SlackRoute, m slack.Message, kind, details, view string) error {
	actor, err := s.requestingActor(ctx, a, token, m.User)
	if err != nil {
		return err
	}
	if actor == nil {
		if view != "" {
			if err := s.inboxCreateNotice(ctx, token, m, view, "Link your Helpin account using the button in Slack."); err != nil {
				return err
			}
		}
		return s.promptLink(ctx, a, token, m)
	}
	ctx = authorization.WithActor(ctx, actor)
	ok, err := s.authz.CanAccessModule(ctx, actor, model.ModuleSupport)
	if err != nil {
		return err
	}
	if !ok || !s.authz.Can(actor, authorization.PermSupportEdit) {
		return s.inboxCreateNotice(ctx, token, m, view, "You need permission to edit support tickets in Helpin.")
	}
	t, err := s.repo.FeedByRoot(ctx, a.ID, route.ChannelID, m.Root())
	if err != nil {
		return err
	}
	d := inboxTicketDraft{Channel: route.ChannelID, Root: m.Root(), TS: m.TS, Details: truncate(details, 2900)}
	var conv *model.SupportConversation
	if t != nil {
		f, _, err := s.editableFeed(ctx, a, t, m.User)
		if err != nil {
			return s.inboxCreateNotice(ctx, token, m, view, "This conversation is no longer available in this channel.")
		}
		conv = f.conv
		d.Conversation = conv.ID
		if kind == "task" || kind == "bug" {
			return s.prepareTaskDraft(ctx, a, token, route, m, actor, conv, view)
		}
		if d.Details == "" || (kind == "shortcut" && m.TS == t.RootTS) {
			d.Details = conv.Subject
		}
	} else if kind == "task" || kind == "bug" {
		return s.inboxCreateNotice(ctx, token, m, view, "To create a linked task, ask in a shared conversation thread.")
	} else if kind == "shortcut" && m.BotID != "" {
		return s.inboxCreateNotice(ctx, token, m, view, "Choose a team message or a shared Helpin conversation.")
	}
	d.SourceURL, err = s.api.MessagePermalink(ctx, token, m.Destination(), m.TS)
	if err != nil {
		return err
	}
	if d.SourceURL == "" {
		return s.inboxCreateNotice(ctx, token, m, view, "The selected Slack message is unavailable. Choose another message to start a ticket.")
	}
	id := uuid.NewString()
	data, err := json.Marshal(d)
	if err != nil {
		return err
	}
	if err = s.repo.SaveDraft(ctx, id, a.ID, actor.UserID, nilIfEmpty(d.Conversation), data); err != nil {
		return err
	}
	if view != "" {
		return s.reviewInboxTicket(ctx, a, token, route, actor, m, d, id, inboxCreateInteraction{View: view})
	}
	prompt := "Create a support ticket from this message?"
	buttons := []any{feedButton("inbox_ticket_review", "Review support ticket", id)}
	if kind == "ticket" && conv != nil {
		prompt = fmt.Sprintf("This thread is already linked to #%d · %s.", conv.DisplayID, truncate(conv.Subject, 140))
		buttons = []any{feedButton("inbox_ticket_review", "Open a separate support ticket", id)}
		pm, err := s.authz.CanAccessModule(ctx, actor, model.ModulePM)
		if err != nil {
			return err
		}
		if pm && s.authz.Can(actor, authorization.PermPMEdit) {
			buttons = append([]any{feedButton("inbox_linked_task", "Create linked task", id)}, buttons...)
		}
	}
	return s.private(ctx, token, m, prompt, map[string]any{"type": "section", "text": slackText(prompt)}, slackButtons(buttons...))
}

func (s *SlackService) processInboxInteraction(ctx context.Context, a *model.SupportChannelAccount, job slackShortcutJob) error {
	p, m := job.Inbox, job.Message
	token, err := s.channels.decryptToken(a)
	if err != nil {
		return err
	}
	m.User = job.Requester
	var actor *authorization.Actor
	var d inboxTicketDraft
	channel := m.Channel
	if p.Action != "shortcut" {
		actor, err = s.linkedActor(ctx, a, job.Requester)
		if err != nil {
			return err
		}
		if actor == nil {
			return s.inboxCreateNotice(ctx, token, m, p.View, "Link your Helpin account to continue.")
		}
		ctx = authorization.WithActor(ctx, actor)
		raw, err := s.repo.Draft(ctx, p.Draft, a.ID, actor.UserID)
		if err != nil {
			return err
		}
		if len(raw) == 0 || json.Unmarshal(raw, &d) != nil || d.TS == "" || d.Channel == "" {
			return s.inboxCreateNotice(ctx, token, m, p.View, "This draft expired. Ask Helpin again to start a new draft.")
		}
		channel, m.TS, m.ThreadTS = d.Channel, d.TS, d.Root
	}
	route, err := s.repo.EffectiveRoute(ctx, a.ID, channel)
	if err != nil {
		return err
	}
	if route == nil || route.Purpose != "inbox" || (m.Channel != "" && m.Channel != route.ChannelID && m.Channel != route.CurrentChannelID) {
		return s.inboxCreateNotice(ctx, token, m, p.View, "This channel is no longer connected to these inboxes.")
	}
	m.Channel, m.CurrentChannelID = route.ChannelID, route.CurrentChannelID
	if p.Action == "shortcut" {
		return s.beginInboxCreate(ctx, a, token, route, m, "shortcut", strings.TrimSpace(m.Text), p.View)
	}
	ok, err := s.authz.CanAccessModule(ctx, actor, model.ModuleSupport)
	if err != nil {
		return err
	}
	if !ok || !s.authz.Can(actor, authorization.PermSupportEdit) {
		return s.inboxCreateNotice(ctx, token, m, p.View, "You need permission to edit support tickets in Helpin.")
	}
	if d.Conversation != "" {
		t, err := s.repo.FeedByRoot(ctx, a.ID, route.ChannelID, d.Root)
		if err != nil {
			return err
		}
		if t == nil || t.ConversationID != d.Conversation {
			return s.inboxCreateNotice(ctx, token, m, p.View, "The source conversation is no longer available here.")
		}
		f, _, err := s.editableFeed(ctx, a, t, job.Requester)
		if err != nil {
			return s.inboxCreateNotice(ctx, token, m, p.View, "The source conversation is no longer available here.")
		}
		if p.Action == "inbox_linked_task" {
			return s.prepareTaskDraft(ctx, a, token, route, m, actor, f.conv, p.View)
		}
	}
	if p.Action != "inbox_ticket_review" && p.Action != "inbox_ticket_save" {
		return ErrChannelInvalid
	}
	return s.reviewInboxTicket(ctx, a, token, route, actor, m, d, p.Draft, *p)
}

func (s *SlackService) ticketInboxOptions(ctx context.Context, a *model.SupportChannelAccount, token string, route *model.SlackRoute, selected string) ([]map[string]any, error) {
	var info struct{ Channel SlackChannel }
	if err := s.api.Get(ctx, token, "conversations.info", url.Values{"channel": {route.CurrentChannelID}}, &info); err != nil {
		return nil, err
	}
	if info.Channel.ID != route.CurrentChannelID || !info.Channel.IsMember {
		return nil, nil
	}
	var opts []map[string]any
	for _, key := range route.MailboxIDs {
		if selected != "" && key != selected {
			continue
		}
		label := "Shared inbox"
		if key != "shared" {
			box, err := s.channels.inbox.mailboxRepo.GetByID(ctx, a.WorkspaceID, key)
			if err != nil {
				return nil, err
			}
			if box == nil || !box.Active || !s.channels.inbox.isMailboxAccessible(ctx, a.WorkspaceID, &key) {
				continue
			}
			label = box.Name
		}
		ok, err := s.feedAudience(ctx, a, token, info.Channel, key)
		if err != nil {
			return nil, err
		}
		if ok {
			opts = append(opts, feedOption(label, key))
		}
	}
	return opts, nil
}

func inboxTicketView(id string, d inboxTicketDraft, opts []map[string]any) map[string]any {
	initial := ""
	if len(opts) == 1 {
		initial = opts[0]["value"].(string)
	}
	selector := selectBlock("inbox", "Inbox", opts, initial, false)
	if len(opts) > 100 {
		field := selector["element"].(map[string]any)
		delete(field, "options")
		var groups []any
		for chunk := range slices.Chunk(opts, 100) {
			groups = append(groups, map[string]any{"label": slackText(fmt.Sprintf("Inboxes %d–%d", len(groups)*100+1, len(groups)*100+len(chunk))), "options": chunk})
		}
		field["option_groups"] = groups
	}
	subject, _, _ := strings.Cut(d.Details, "\n")
	return map[string]any{"type": "modal", "callback_id": "inbox_ticket_save", "private_metadata": id, "title": slackText("New support ticket"), "submit": slackText("Create ticket"), "close": slackText("Cancel"), "blocks": []any{
		inputBlock("subject", "Subject", truncate(subject, 120), false), inputBlock("description", "Internal description", d.Details, true), selector,
	}}
}

func (s *SlackService) reviewInboxTicket(ctx context.Context, a *model.SupportChannelAccount, token string, route *model.SlackRoute, actor *authorization.Actor, m slack.Message, d inboxTicketDraft, id string, p inboxCreateInteraction) error {
	opts, err := s.ticketInboxOptions(ctx, a, token, route, p.Mailbox)
	if err != nil {
		return err
	}
	if len(opts) == 0 {
		return s.inboxCreateNotice(ctx, token, m, p.View, "This inbox is unavailable. Check the channel’s followed inboxes and your Helpin access.")
	}
	if p.Action != "inbox_ticket_save" {
		return s.api.Call(ctx, token, "views.update", map[string]any{"view_id": p.View, "view": inboxTicketView(id, d, opts)}, nil)
	}
	if p.Mailbox == "" || p.Subject == "" {
		return ErrChannelInvalid
	}
	ticketID := inboundStableID("slack-staff-ticket:" + a.ID + ":" + route.ChannelID + ":" + d.TS)
	return s.repo.WithFeedLock(ctx, ticketID, func() error {
		return s.saveInboxTicket(ctx, a, token, route, actor, m, d, ticketID, p)
	})
}

func (s *SlackService) saveInboxTicket(ctx context.Context, a *model.SupportChannelAccount, token string, route *model.SlackRoute, actor *authorization.Actor, m slack.Message, d inboxTicketDraft, ticketID string, p inboxCreateInteraction) error {
	inbox := s.channels.inbox
	ctx = model.WithSlackSource(ctx, d.SourceURL)
	ticket, err := inbox.loadConversationAccessible(ctx, a.WorkspaceID, ticketID)
	if err != nil {
		return err
	}
	if ticket == nil {
		var mailbox *string
		if p.Mailbox != "shared" {
			mailbox = &p.Mailbox
		}
		note := &model.SupportMessage{SenderType: "user", SenderUserID: &actor.UserID, IsInternal: true, MessageType: "note", Content: p.Description}
		if strings.TrimSpace(note.Content) == "" {
			note.Content = p.Subject
		}
		ticket, err = inbox.createConversation(ctx, model.CreateConversationRequest{WorkspaceID: a.WorkspaceID, MailboxID: mailbox, Subject: p.Subject}, actor.UserID, ticketID, note)
		if err != nil {
			return err
		}
	}
	if ticket.MergedIntoID != nil || ticket.AnonymizedAt != nil || ticket.Status == "spam" || firstNonEmptyString(derefString(ticket.MailboxID), "shared") != p.Mailbox {
		return s.inboxCreateNotice(ctx, token, m, p.View, "A ticket was already created from this message. Check Helpin for its current inbox.")
	}
	thread, err := s.repo.FeedByConversation(ctx, a.ID, route.ChannelID, ticket.ID)
	if err != nil {
		return err
	}
	if thread == nil || !thread.Active {
		return s.inboxCreateNotice(ctx, token, m, p.View, "The ticket was created. Its inbox is no longer shared in this channel.")
	}
	label := fmt.Sprintf("Created #%d · %s", ticket.DisplayID, truncate(ticket.Subject, 160))
	link := map[string]any{"type": "button", "action_id": "open_ticket", "text": slackText("Open in Helpin"), "url": s.appURL + "/w/" + thread.WorkspaceSlug + "/support/" + ticket.ID}
	blocks := []any{map[string]any{"type": "section", "text": slackText(label)}, slackButtons(link)}
	m.Type = "created-ticket-" + ticket.ID
	const receiptKey = "staff_ticket_created"
	sent, err := s.repo.FeedReceipt(ctx, thread.ID, receiptKey)
	if err != nil {
		return err
	}
	if sent == "" {
		if err := s.reply(ctx, token, m, label, false, blocks...); err != nil {
			return err
		}
		if err := s.repo.SaveFeedReceipt(ctx, thread.ID, receiptKey, "sent"); err != nil {
			return err
		}
	}
	receipt := feedNotice("Ticket created", label)
	receipt["blocks"] = blocks
	return s.api.Call(ctx, token, "views.update", map[string]any{"view_id": p.View, "view": receipt}, nil)
}
