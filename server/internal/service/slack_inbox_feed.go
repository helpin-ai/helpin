package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/channels/slack"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

const SlackInboxFeedJob = "slack_inbox_feed"

func slackFeedAllowsChannel(ch SlackChannel, visibility string) bool {
	return !ch.IsArchived && !ch.IsShared && !ch.PendingShared && (ch.IsPrivate || visibility == "workspace")
}
func (s *SlackService) validateFeedSelection(ctx context.Context, a *model.SupportChannelAccount, token string, ch SlackChannel, ids []string) error {
	if len(ids) == 0 {
		return fmt.Errorf("%w: select at least one inbox", ErrChannelInvalid)
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			return fmt.Errorf("%w: duplicate inbox", ErrChannelInvalid)
		}
		seen[id] = true
		if id != "shared" {
			if _, err := s.channels.mailboxSelection(ctx, a.WorkspaceID, &id); err != nil {
				return err
			}
		}
		ok, err := s.feedAudience(ctx, a, token, ch, id)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w: use an internal channel. For a team inbox, invite Helpin to a private channel where each member has linked their Helpin account and can read that inbox", ErrChannelInvalid)
		}
	}
	return nil
}
func (s *SlackService) feedAudience(ctx context.Context, a *model.SupportChannelAccount, token string, ch SlackChannel, key string) (bool, error) {
	visibility := "members_only"
	if key == "shared" {
		visibility = "workspace"
	}
	if !slackFeedAllowsChannel(ch, visibility) {
		return false, nil
	}
	if !ch.IsPrivate {
		return true, nil
	}
	return s.channelAudience(ctx, a, token, ch.ID, func(actor *authorization.Actor) (bool, error) {
		ok, err := s.authz.CanAccessModule(ctx, actor, model.ModuleSupport)
		if err != nil || !ok || !s.authz.Can(actor, authorization.PermSupportRead) {
			return false, err
		}
		return key == "shared" || s.channels.inbox.userCanAccessMailbox(ctx, a.WorkspaceID, &key, actor.UserID), nil
	})
}
func slackFeedThreadUpdate(m *model.SupportMessage) bool {
	if m == nil || m.DeletedAt.Valid {
		return false
	}
	if m.MessageType == "note" {
		if !m.IsInternal {
			return false
		}
		if m.SenderType == "user" {
			return true
		}
		meta := slackFeedAINote(m)
		return meta.Handoff != nil || meta.finishedCodeCheck()
	}
	if m.MessageType == "reply" {
		return !m.IsInternal || m.SenderType == "user"
	}
	if derefString(m.SystemEventType) == model.SystemEventReopened && m.SenderUserID == nil {
		// The customer's reply already explains why the parent reopened.
		return false
	}
	switch derefString(m.SystemEventType) {
	case model.SystemEventAIEscalated, model.SystemEventCustomerRequestedHuman:
		var meta struct {
			HandoffNoteID string `json:"handoff_note_id"`
		}
		return json.Unmarshal([]byte(m.Metadata), &meta) != nil || meta.HandoffNoteID == ""
	case "resolved", "reopened":
		return true
	}
	return false
}

type feedContext struct {
	thread          *repository.FeedThread
	conv            *model.SupportConversation
	account         *model.SupportChannelAccount
	contact         *model.CRMContact
	token, channel  string
	conversationURL string
}

func (s *SlackService) loadFeed(ctx context.Context, a *model.SupportChannelAccount, t *repository.FeedThread) (*feedContext, error) {
	if t == nil || !t.Active || a == nil || a.ID != t.AccountID {
		return nil, nil
	}
	route, err := s.repo.Route(ctx, a.ID, t.ChannelID)
	if err != nil || route == nil || route.Purpose != "inbox" {
		return nil, err
	}
	conv, err := s.channels.inbox.loadConversationUnscoped(ctx, a.WorkspaceID, t.ConversationID)
	if err != nil || conv == nil {
		return nil, err
	}
	key := firstNonEmptyString(derefString(conv.MailboxID), "shared")
	token, err := s.channels.decryptToken(a)
	if err != nil {
		return nil, err
	}
	conversationURL := s.appURL + "/w/" + t.WorkspaceSlug + "/support/" + conv.ID
	if conv.AnonymizedAt != nil || conv.MergedIntoID != nil || !slices.Contains(route.MailboxIDs, key) {
		if t.RootTS != "" {
			notice, block := s.feedStopNotice(ctx, conv, key, route.MailboxIDs)
			payload := map[string]any{
				"channel": route.CurrentChannelID, "ts": t.RootTS,
				"text": notice + "\n" + conversationURL, "mrkdwn": false, "unfurl_links": false, "unfurl_media": false,
				"blocks": []any{
					block,
					slackButtons(feedOpenButton(t.ID, conversationURL)),
				},
			}
			err = s.api.Call(ctx, token, "chat.update", payload, nil)
			var apiErr *slack.APIError
			if err != nil && !(errors.As(err, &apiErr) && slices.Contains([]string{"channel_not_found", "message_not_found", "not_in_channel", "is_archived"}, apiErr.Code)) {
				return nil, err
			}
		}
		return nil, s.repo.DeactivateFeed(ctx, t.ID)
	}
	var info struct{ Channel SlackChannel }
	if err = s.api.Get(ctx, token, "conversations.info", url.Values{"channel": {route.CurrentChannelID}}, &info); err != nil {
		return nil, err
	}
	if info.Channel.ID != route.CurrentChannelID || !info.Channel.IsMember {
		return nil, nil
	}
	ok, err := s.feedAudience(ctx, a, token, info.Channel, key)
	if !ok || err != nil {
		return nil, err
	}
	f := &feedContext{thread: t, conv: conv, account: a, token: token, channel: route.CurrentChannelID, conversationURL: conversationURL}
	if conv.CRMContactID != nil && s.channels.inbox.contactRepo != nil {
		contacts, err := s.channels.inbox.contactRepo.ListByIDs(ctx, conv.WorkspaceID, []string{*conv.CRMContactID})
		if err != nil {
			return nil, err
		}
		if len(contacts) == 1 {
			f.contact = &contacts[0]
		}
	}
	return f, nil
}

type slackNoticePart struct {
	text string
	bold bool
}

func renderFeedStopNotice(parts ...slackNoticePart) (string, map[string]any) {
	var plain strings.Builder
	var elements []any
	for _, part := range append(parts, slackNoticePart{text: " Updates have stopped in this thread."}) {
		plain.WriteString(part.text)
		element := map[string]any{"type": "text", "text": part.text}
		if part.bold {
			element["style"] = map[string]bool{"bold": true}
		}
		elements = append(elements, element)
	}
	section := map[string]any{"type": "rich_text_section", "elements": elements}
	return plain.String(), map[string]any{"type": "rich_text", "elements": []any{section}}
}

func (s *SlackService) feedStopNotice(ctx context.Context, conv *model.SupportConversation, key string, followed []string) (string, map[string]any) {
	switch {
	case conv.AnonymizedAt != nil:
		return renderFeedStopNotice(
			slackNoticePart{text: "Personal details were removed", bold: true},
			slackNoticePart{text: " from this conversation in Helpin."},
		)
	case conv.MergedIntoID != nil:
		return renderFeedStopNotice(
			slackNoticePart{text: "This conversation was "},
			slackNoticePart{text: "merged", bold: true},
			slackNoticePart{text: " into another conversation in Helpin."},
		)
	}
	name := strings.TrimSpace(derefString(conv.MailboxName))
	if key == "shared" {
		name = "Shared Inbox"
	}
	destination := "an inbox this channel doesn’t follow"
	afterDestination := "."
	if name != "" {
		destination = "“" + truncate(name, 2800) + "”"
		afterDestination = ", which this channel doesn’t follow."
	}
	event, err := s.repo.FeedMoveEvent(ctx, conv.WorkspaceID, conv.ID, key, followed)
	if err != nil {
		// Remove the stale card even when routing history is unavailable.
		slog.WarnContext(ctx, "could not attribute Slack inbox move", "workspace_id", conv.WorkspaceID, "conversation_id", conv.ID, "error", err)
	}
	var actor string
	switch {
	case event != nil && event.ActorUserID != nil:
		actor = "A teammate"
	case event != nil && event.EventType == supportTriageEventAutoMoved:
		switch derefString(event.Source) {
		case model.SupportConversationTriageSourceAI:
			actor = "Helpin AI"
		case model.SupportConversationTriageSourceRule:
			actor = "A routing rule"
		}
	}
	parts := []slackNoticePart{{text: "This conversation is in "}}
	if actor != "" {
		parts = []slackNoticePart{{text: actor, bold: true}, {text: " moved this conversation to "}}
	}
	parts = append(parts, slackNoticePart{text: destination, bold: true}, slackNoticePart{text: afterDestination})
	return renderFeedStopNotice(parts...)
}

func (s *SlackService) deliverInboxFeed(ctx context.Context, j *model.SupportInboundJob) error {
	var p struct{ Thread string }
	if err := json.Unmarshal([]byte(j.Payload), &p); err != nil {
		return err
	}
	if j.ChannelAccountID == nil || !isUUIDString(p.Thread) {
		return errors.New("missing Slack inbox thread")
	}
	return s.repo.WithFeedLock(ctx, p.Thread, func() error {
		a, err := s.channels.accounts.GetForWebhook(ctx, "slack", *j.ChannelAccountID)
		if err != nil || a == nil {
			return err
		}
		t, err := s.repo.FeedThread(ctx, a.ID, p.Thread)
		if err != nil {
			return err
		}
		f, err := s.loadFeed(ctx, a, t)
		if err != nil || f == nil {
			return err
		}
		if err = s.refreshFeedCard(ctx, f); err != nil {
			return err
		}
		if j.MessageID == nil || f.conv.Status == "spam" {
			return nil
		}
		m, err := s.channels.inbox.messageRepo.GetByID(ctx, *j.MessageID)
		if err != nil || m == nil || m.ConversationID != f.conv.ID || !slackFeedThreadUpdate(m) {
			return err
		}
		if m.MessageType == "note" && m.SenderType != "user" {
			return s.mirrorFeedAINote(ctx, f, m)
		}
		return s.mirrorFeedMessage(ctx, f, m)
	})
}
func feedButton(id, label, value string) map[string]any {
	return map[string]any{"type": "button", "action_id": id, "text": slackText(label), "value": value}
}

func feedOpenButton(threadID, link string) map[string]any {
	open := feedButton("open_support", "Open in Helpin", threadID)
	open["url"] = link
	return open
}

func (s *SlackService) refreshFeedCard(ctx context.Context, f *feedContext) error {
	c, t := f.conv, f.thread
	status := strings.ReplaceAll(c.Status, "_", " ")
	if c.Status == "open" {
		switch derefString(c.FlowState) {
		case "waiting_for_human", "queued_for_human", "after_hours_queue":
			status = "🙋 Needs human reply"
		}
	}
	if c.SnoozedUntil != nil && c.SnoozedUntil.After(s.channels.now()) {
		status = "Snoozed until " + c.SnoozedUntil.UTC().Format(time.RFC822)
	}
	owner := "Unassigned"
	if c.AssignedUserID != nil {
		u, err := s.channels.inbox.userRepo.GetByID(ctx, *c.AssignedUserID)
		if err != nil {
			return err
		}
		if u != nil {
			owner = u.FullName
		}
	}
	title := fmt.Sprintf("Conversation #%d", c.DisplayID)
	if subject := feedOneLine(c.Subject); subject != "" {
		title = fmt.Sprintf("#%d · %s", c.DisplayID, truncate(subject, 80))
	}
	customer := firstNonEmptyString(derefString(c.CustomerName), derefString(c.CustomerEmail), "Visitor")
	mailbox := firstNonEmptyString(derefString(c.MailboxName), "Shared Inbox")
	details := strings.Join([]string{truncate(feedOneLine(customer), 80), truncate(feedOneLine(mailbox), 80), status}, " · ")
	switch c.Priority {
	case "urgent":
		details += " · 🔴 Urgent"
	case "high":
		details += " · 🟠 High"
	}
	preview, attachment, err := s.repo.FeedPreview(ctx, c.WorkspaceID, c.ID)
	if err != nil {
		return err
	}
	blocks := []any{feedRichText("rich_text_section", map[string]any{"type": "link", "url": f.conversationURL, "text": title, "style": map[string]bool{"bold": true}}), map[string]any{"type": "context", "elements": []any{slackText(details)}}}
	fallback := title + "\n" + details
	if preview != nil {
		visible, rich, email, err := s.feedContent(ctx, f, preview)
		if err != nil {
			return err
		}
		if rich || !email {
			visible = supportTranscriptPlainText(renderSupportTranscriptMessageHTML(visible))
		}
		if strings.TrimSpace(visible) == "" && attachment != "" {
			visible = "📎 Attachment: " + attachment
		}
		if excerpt := feedExcerpt(c.Subject, visible); excerpt != "" {
			blocks = append(blocks, feedRichText("rich_text_quote", map[string]string{"type": "text", "text": excerpt}))
			fallback += "\n" + excerpt
		}
	}
	if c.Status != "spam" {
		blocks = append(blocks, map[string]any{"type": "context", "elements": []any{slackText("Thread replies are internal notes.")}})
	}
	fallback += "\n" + owner + " · " + f.conversationURL
	buttons := []any{}
	if c.Status != "spam" {
		buttons = append(buttons, feedButton("feed_reply", "Reply to customer", t.ID), feedButton("feed_manage", "Manage", t.ID))
		if c.AssignedUserID == nil {
			buttons = append(buttons, feedButton("feed_claim", "Assign to me", t.ID))
		}
	}
	blocks = append(blocks, slackButtons(append(buttons, feedOpenButton(t.ID, f.conversationURL))...))
	payload := map[string]any{"channel": f.channel, "text": fallback, "mrkdwn": false, "blocks": blocks, "unfurl_links": false, "unfurl_media": false}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	hash := inboundStableID(string(raw))
	previous, err := s.repo.FeedReceipt(ctx, t.ID, "card")
	if err != nil {
		return err
	}
	if t.RootTS != "" && t.Permalink == "" {
		t.Permalink = s.sourceURL(ctx, f.token, f.channel, t.RootTS)
		if t.Permalink != "" {
			if err = s.repo.UpdateFeedRoot(ctx, t.ID, t.RootTS, t.Permalink); err != nil {
				return err
			}
		}
	}
	if t.RootTS != "" && previous == hash {
		return nil
	}
	if t.RootTS == "" {
		var result struct{ TS string }
		payload["client_msg_id"] = t.ID
		if err = s.api.Call(ctx, f.token, "chat.postMessage", payload, &result); err != nil {
			return err
		}
		if result.TS == "" {
			return errors.New("Slack did not confirm thread creation")
		}
		t.RootTS = result.TS
		t.Permalink = s.sourceURL(ctx, f.token, f.channel, result.TS)
		if err = s.repo.UpdateFeedRoot(ctx, t.ID, t.RootTS, t.Permalink); err != nil {
			return err
		}
	} else {
		payload["ts"] = t.RootTS
		if err = s.api.Call(ctx, f.token, "chat.update", payload, nil); err != nil {
			return err
		}
	}
	return s.repo.SaveFeedReceipt(ctx, t.ID, "card", hash)
}
func (s *SlackService) mirrorFeedMessage(ctx context.Context, f *feedContext, m *model.SupportMessage) error {
	var meta struct {
		SourceThread string `json:"slack_feed_thread"`
		EmailStatus  string `json:"email_delivery_status"`
	}
	if m.Metadata != "" {
		if err := json.Unmarshal([]byte(m.Metadata), &meta); err != nil {
			return err
		}
	}
	if m.IsInternal && m.MessageType != "system" && meta.SourceThread == f.thread.ID {
		return nil
	}
	body, rich, email, err := s.feedContent(ctx, f, m)
	if err != nil {
		return err
	}
	author := f.feedAuthor(m)
	if m.MessageType == "system" {
		body = tiptap.RichTextToMarkdown(m.Content)
		if strings.TrimSpace(body) == "" {
			switch derefString(m.SystemEventType) {
			case model.SystemEventAIEscalated:
				body = "AI handed this conversation to a teammate."
			case model.SystemEventCustomerRequestedHuman:
				body = "The customer asked for a teammate."
			}
		}
	}
	var delivery *model.SupportChannelDelivery
	if m.SenderType != "customer" && !m.IsInternal {
		rows, err := s.channels.outbox.ListByMessageIDs(ctx, f.account.WorkspaceID, []string{m.ID})
		if err != nil {
			return err
		}
		if len(rows) > 0 {
			delivery = channelDeliveryView(rows[0])
			body += "\n" + delivery.Provider + " · " + delivery.Status
			if delivery.Status == "unconfirmed" {
				body += " · Check the original destination before resending."
			}
		}
	}
	if meta.EmailStatus != "" {
		body += "\nEmail · " + meta.EmailStatus
	}
	blocks := []any{author.header()}
	formatted := rich || !m.IsInternal && author.role == "AI reply"
	display, clipped := feedReplyBody(body, formatted)
	blocks = append(blocks, display...)
	if formatted {
		body = supportTranscriptPlainText(renderSupportTranscriptMessageHTML(body))
	}
	var buttons []any
	if delivery != nil && delivery.Status == "failed" {
		buttons = append(buttons, feedButton("feed_retry", "Retry delivery", f.thread.ID+":"+m.ID))
	}
	if clipped || email {
		buttons = append(buttons, feedOpenButton(f.thread.ID, f.conversationURL+"#message-"+m.ID))
	}
	if len(buttons) > 0 {
		blocks = append(blocks, slackButtons(buttons...))
	}
	if err := s.writeFeedReply(ctx, f, m.ID, author.label()+"\n"+body, blocks); err != nil {
		return err
	}
	files, err := s.channels.replyFiles(ctx, m, 25<<20)
	if err != nil {
		return err
	}
	for _, file := range files {
		key := m.ID + ":file:" + file.ID
		got, err := s.repo.FeedReceipt(ctx, f.thread.ID, key)
		if err != nil {
			return err
		}
		if got != "" {
			continue
		}
		id, err := s.repo.FeedReceipt(ctx, f.thread.ID, key+":staged")
		if err != nil {
			return err
		}
		shared := false
		if id != "" {
			shared, err = s.api.FileSharedInChannel(ctx, f.token, f.channel, id)
			if err != nil {
				return err
			}
		} else {
			id, err = s.api.PrepareUpload(ctx, f.token, file.Name, file.Data)
			if err != nil {
				return err
			}
			if err = s.repo.SaveFeedReceipt(ctx, f.thread.ID, key+":staged", id); err != nil {
				return err
			}
		}
		if !shared {
			if err = s.api.ShareUpload(ctx, f.token, f.channel, f.thread.RootTS, file.Name, id); err != nil {
				return err
			}
		}
		if err = s.repo.SaveFeedReceipt(ctx, f.thread.ID, key, id); err != nil {
			return err
		}
	}
	return nil
}

// writeFeedReply uses the Helpin message ID for both the Slack retry key and the delivery receipt.
func (s *SlackService) writeFeedReply(ctx context.Context, f *feedContext, id, body string, blocks []any) error {
	prior, err := s.repo.FeedReceipt(ctx, f.thread.ID, id)
	if err != nil {
		return err
	}
	payload := map[string]any{"channel": f.channel, "text": truncate(body, 3900), "mrkdwn": false, "unfurl_links": false, "unfurl_media": false}
	if blocks != nil {
		payload["blocks"] = blocks
	}
	if prior != "" {
		payload["ts"] = prior
		if blocks == nil {
			payload["blocks"] = []any{}
		}
		return s.api.Call(ctx, f.token, "chat.update", payload, nil)
	}
	payload["thread_ts"] = f.thread.RootTS
	payload["client_msg_id"] = inboundStableID(f.thread.ID + ":" + id)
	var out struct{ TS string }
	if err := s.api.Call(ctx, f.token, "chat.postMessage", payload, &out); err != nil {
		return err
	}
	if out.TS == "" {
		return errors.New("Slack did not confirm the message")
	}
	return s.repo.SaveFeedReceipt(ctx, f.thread.ID, id, out.TS)
}

func (s *SlackService) InboxThreadLink(ctx context.Context, workspace, id string) (string, error) {
	c, err := s.channels.inbox.loadConversationAccessible(ctx, workspace, id)
	if err != nil || c == nil || c.AnonymizedAt != nil || c.MergedIntoID != nil {
		return "", err
	}
	return s.repo.FeedPermalink(ctx, workspace, id)
}
