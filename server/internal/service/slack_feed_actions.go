package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/channels/slack"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

const SlackFeedActionJob = "slack_feed_action"

type expectedSupportAssigneeKey struct{}

type feedDraft struct {
	Thread, Latest, Owner, Status, Primary, CC, Token string
	Revision                                          int64
	Priority, Mailbox, Snooze                         string
	AIControlVersion                                  int64
	HumanRequested                                    bool
	Files                                             []slack.File
}
type feedActionJob struct {
	Kind, User, View, Destination, Body, To, CC, Status, Owner, Message string
	Choice                                                              string
	CustomTime                                                          int64
	Submit                                                              bool
	Draft                                                               feedDraft
	Files                                                               []slack.File
}

func feedNotice(title, text string) map[string]any {
	return map[string]any{"type": "modal", "title": slackText(title), "close": slackText("Close"), "blocks": []any{map[string]any{"type": "section", "text": slackText(truncate(text, 2900))}}}
}

// Open a lightweight view immediately. Network access checks and file uploads run in the job worker.
func (s *SlackService) feedAction(ctx context.Context, v SlackAction, submit bool) (any, error) {
	a, err := s.channels.accounts.FindLive(ctx, "slack", v.Team.ID)
	if a == nil || err != nil {
		return nil, err
	}
	p := feedActionJob{User: v.User.ID, Submit: submit, View: v.View.ID}
	if submit {
		if err = json.Unmarshal([]byte(v.View.Metadata), &p.Draft); err != nil {
			return nil, ErrChannelInvalid
		}
		p.Kind = v.View.Callback
		vals := v.View.State.Values
		p.Body = vals["body"]["value"].Value
		p.Destination = vals["destination"]["value"].Selected.Value
		p.Files = vals["files"]["value"].Files
		// The retained-files selector lets a teammate remove uploads when revising a draft.
		if keep, exists := vals["retained"]; exists {
			kept := map[string]bool{}
			for _, opt := range keep["value"].Multiple {
				kept[opt.Value] = true
			}
			p.Draft.Files = slices.DeleteFunc(p.Draft.Files, func(file slack.File) bool { return !kept[file.ID] })
		}
		if len(feedFiles(p)) > 5 {
			return map[string]any{"response_action": "errors", "errors": map[string]string{"files": "Choose up to five attachments in total."}}, nil
		}
		p.To = vals["to"]["value"].Value
		p.CC = vals["cc"]["value"].Value
		p.Owner = vals["owner"]["value"].Selected.Value
		p.Status = vals["status"]["value"].Selected.Value
		p.Choice = vals["choice"]["value"].Selected.Value
		p.CustomTime = vals["custom"]["value"].DateTime
		if p.Kind == "feed_reply" && strings.TrimSpace(p.Body) == "" && len(p.Files)+len(p.Draft.Files) == 0 {
			return map[string]any{"response_action": "errors", "errors": map[string]string{"body": "Write a reply or attach a file."}}, nil
		}
	} else {
		p.Kind = v.Actions[0].ID
		if p.Kind == "feed_more" {
			p.Draft.Thread, p.Kind, _ = strings.Cut(v.Actions[0].Selected.Value, ":")
			if !feedMenuAction(p.Kind) {
				return map[string]any{}, nil
			}
		} else {
			p.Draft.Thread, p.Message, _ = strings.Cut(v.Actions[0].Value, ":")
		}
		p.Draft.Token = inboundStableID(v.Trigger)
	}
	if !isUUIDString(p.Draft.Thread) {
		return nil, ErrChannelInvalid
	}
	t, err := s.repo.FeedThread(ctx, a.ID, p.Draft.Thread)
	if err != nil || t == nil {
		return map[string]any{}, err
	}
	if !submit {
		current, err := s.repo.CurrentChannelID(ctx, a.ID, t.ChannelID)
		if err != nil {
			return nil, err
		}
		expectedTS := t.RootTS
		if p.Kind == "feed_retry" || (p.Kind == "feed_reply" && p.Message != "") {
			if !isUUIDString(p.Message) {
				return nil, ErrChannelInvalid
			}
			expectedTS, err = s.repo.FeedReceipt(ctx, t.ID, p.Message)
			if err != nil {
				return nil, err
			}
		}
		if expectedTS == "" || v.Channel.ID != current || v.Message.TS != expectedTS {
			return map[string]any{}, nil
		}
		if !feedFormAction(p.Kind) && p.Kind != "feed_resolve" && p.Kind != "feed_reopen" && p.Kind != "feed_claim" && p.Kind != "feed_retry" {
			return map[string]any{}, nil
		}
		token, err := s.channels.decryptToken(a)
		if err != nil {
			return nil, err
		}
		var response struct{ View struct{ ID string } }
		if err = s.api.Call(ctx, token, "views.open", map[string]any{"trigger_id": v.Trigger, "view": feedNotice("Helpin", "Opening conversation…")}, &response); err != nil {
			return nil, err
		}
		p.View = response.View.ID
	}
	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	job := &model.SupportInboundJob{ID: inboundStableID("slack-feed-action:" + a.ID + ":" + v.Trigger), Kind: SlackFeedActionJob, WorkspaceID: &a.WorkspaceID, ChannelAccountID: &a.ID, ConversationID: &t.ConversationID, Payload: string(data), Status: "pending", AvailableAt: s.channels.now()}
	if err = s.channels.jobs.Create(ctx, job); err != nil {
		return nil, err
	}
	if submit {
		return map[string]any{"response_action": "update", "view": feedNotice("Helpin", "Saving your changes…")}, nil
	}
	return map[string]any{}, nil
}
func (s *SlackService) editableFeed(ctx context.Context, a *model.SupportChannelAccount, t *repository.FeedThread, user string) (*feedContext, *authorization.Actor, error) {
	f, err := s.loadFeed(ctx, a, t)
	if err != nil {
		return nil, nil, err
	}
	if f == nil || f.conv.Status == "spam" {
		return nil, nil, errors.New("This thread is no longer connected to an available conversation.")
	}
	actor, err := s.requestingActor(ctx, a, f.token, user)
	if err != nil {
		return nil, nil, err
	}
	if actor == nil {
		return f, nil, errors.New("Link your Slack account to Helpin to update this conversation.")
	}
	access, err := s.authz.CanAccessModule(ctx, actor, model.ModuleSupport)
	if err != nil {
		return nil, nil, err
	}
	if !access || !s.authz.Can(actor, authorization.PermSupportEdit) {
		return nil, nil, errors.New("You need permission to reply in this inbox.")
	}
	err = s.channels.inbox.RequireWritableSupportConversation(authorization.WithActor(ctx, actor), a.WorkspaceID, t.ConversationID)
	return f, actor, err
}
func (s *SlackService) receiveFeedNote(ctx context.Context, a *model.SupportChannelAccount, token string, event queuedSlackEvent, originalConv *string) error {
	m := event.Message
	if m.ThreadTS == "" {
		return nil
	}
	t, err := s.repo.FeedByRoot(ctx, a.ID, m.Channel, m.ThreadTS)
	if err != nil || t == nil {
		return err
	}
	if event.NoteThreadID != "" && (event.NoteThreadID != t.ID || derefString(originalConv) != t.ConversationID) {
		return nil
	}
	f, actor, err := s.editableFeed(ctx, a, t, m.User)
	if err != nil {
		if actor == nil && f != nil && event.NoteThreadID == "" {
			data, err := json.Marshal(queuedSlackEvent{Message: m, Inbox: true, NoteThreadID: t.ID})
			if err != nil {
				return err
			}
			subject := model.SupportChannelSubjectKey(a.ID, m.User)
			note := &model.SupportInboundJob{
				Kind: SlackEventJob, ChannelAccountID: &a.ID, WorkspaceID: &a.WorkspaceID,
				ConversationID: &t.ConversationID, SubjectKey: &subject,
				Payload: string(data), Status: model.InboundAwaitingSlackLink,
			}
			return s.sendLinkPrompt(ctx, a, token, m, note)
		}
		return s.private(ctx, token, m, "This message could not be added as a Helpin note. Check your inbox access.")
	}
	ctx = model.WithSlackSource(authorization.WithActor(ctx, actor), s.sourceURL(ctx, token, m.Destination(), m.TS))
	key := a.ID + ":" + m.Channel + ":" + m.TS
	ids, err := s.stageFeedFiles(ctx, f, actor.UserID, key, m.Files)
	if err != nil {
		return s.private(ctx, token, m, "The note was not saved. "+err.Error())
	}
	req := model.CreateMessageRequest{
		Content: s.feedNoteText(ctx, token, m.Text), SlackMentionSource: &m.Text,
		SourceSlackThreadID: t.ID, ClientMessageID: inboundStableID(key),
		AttachmentIDs: ids, ExpectedMergeRevision: &f.conv.MergeRevision,
		IsInternal: true, MessageType: "reply",
	}
	_, err = s.channels.inbox.CreateConversationMessage(ctx, a.WorkspaceID, t.ConversationID, req, "user", &actor.UserID, nil, nil)
	return err
}
func (s *SlackService) stageFeedFiles(ctx context.Context, f *feedContext, user, key string, files []slack.File) ([]string, error) {
	var ids []string
	for _, selected := range files {
		service := s.channels.inbox.attachmentService
		if service == nil {
			return nil, errors.New("File storage is unavailable.")
		}
		id := inboundStableID("feed-file:" + f.thread.ID + ":" + user + ":" + key + ":" + selected.ID)
		existing, err := service.attachmentRepo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if existing == nil {
			file, data, err := s.api.DownloadFile(ctx, f.token, selected.ID, 25<<20)
			if err != nil {
				return nil, fmt.Errorf("Could not download %s: %w", selected.Name, err)
			}
			stored, err := service.StoreInboundAttachmentBytes(ctx, supportInboundEmailAttachmentRequest{AttachmentID: id, WorkspaceID: f.conv.WorkspaceID, ConversationID: f.conv.ID, FileName: file.Name, ContentType: file.Mime}, data)
			if err != nil {
				return nil, err
			}
			attachment := &model.SupportAttachment{ID: id, WorkspaceID: f.conv.WorkspaceID, ConversationID: &f.conv.ID, FileName: stored.FileName, ContentType: stored.FileType, FileSize: stored.FileSize, StorageKey: stored.FileKey, UploadedByType: "user", UploadedByID: &user, IsUploaded: true}
			if err = service.attachmentRepo.Create(ctx, attachment); err != nil && !isUniqueConstraintViolation(err) {
				return nil, err
			}
		}
		ids = append(ids, id)
	}
	return ids, nil
}
func (s *SlackService) processFeedAction(ctx context.Context, job *model.SupportInboundJob) error {
	var p feedActionJob
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return err
	}
	if job.ChannelAccountID == nil {
		return ErrChannelInvalid
	}
	a, err := s.channels.accounts.GetForWebhook(ctx, "slack", *job.ChannelAccountID)
	if a == nil || err != nil {
		return err
	}
	token, err := s.channels.decryptToken(a)
	if err != nil {
		return err
	}
	update := func(view map[string]any) error {
		err := s.api.Call(ctx, token, "views.update", map[string]any{"view_id": p.View, "view": view}, nil)
		var apiErr *slack.APIError
		if errors.As(err, &apiErr) && apiErr.Code == "not_found" {
			return nil
		}
		return err
	}
	return s.repo.WithFeedLock(ctx, p.Draft.Thread, func() error {
		t, err := s.repo.FeedThread(ctx, a.ID, p.Draft.Thread)
		if err != nil {
			return err
		}
		f, actor, err := s.editableFeed(ctx, a, t, p.User)
		if err != nil {
			if f != nil && actor == nil {
				if err := s.promptLink(ctx, a, token, slack.Message{Channel: f.channel, User: p.User, TS: t.RootTS, ThreadTS: t.RootTS}); err != nil {
					return err
				}
			}
			return update(feedNotice("Helpin", err.Error()))
		}
		ctx = model.WithSlackSource(authorization.WithActor(ctx, actor), t.Permalink)
		inbox := s.channels.inbox
		c, err := inbox.GetConversation(ctx, a.WorkspaceID, t.ConversationID)
		if err != nil {
			return err
		}
		f.conv = c
		latest, err := s.repo.FeedLatestReply(ctx, c.ID)
		if err != nil {
			return err
		}
		show := func(warning string) error {
			p.Draft.Latest = latest
			p.Draft.Owner = derefString(c.AssignedUserID)
			p.Draft.Priority = c.Priority
			p.Draft.Mailbox = derefString(c.MailboxID)
			p.Draft.Snooze = feedSnoozeStamp(c)
			p.Draft.AIControlVersion = c.AIControlVersion
			p.Draft.HumanRequested = c.CustomerRequestedHumanAt != nil
			p.Draft.Status = c.Status
			p.Draft.Primary = derefString(c.CustomerEmail)
			p.Draft.CC = feedEmailAudience(c)
			p.Draft.Revision = c.MergeRevision
			view, err := s.feedForm(ctx, f, &p, warning)
			if err != nil {
				return err
			}
			return update(view)
		}
		if p.Kind == "feed_retry" {
			msg, err := inbox.messageRepo.GetByID(ctx, p.Message)
			if err != nil {
				return err
			}
			if msg == nil || msg.ConversationID != c.ID {
				return update(feedNotice("Delivery", "This reply is no longer available."))
			}
			conv, err := s.channels.outbox.RetryRejected(ctx, a.WorkspaceID, p.Message)
			if err != nil {
				return err
			}
			if conv == "" {
				return update(feedNotice("Delivery", "This reply is no longer waiting for a retry."))
			}
			inbox.teammateSystemEvent(ctx, c, actor.UserID, model.SystemEventDeliveryRetried, func(actor string) string { return actor + " retried a failed reply." }, map[string]any{"message_id": msg.ID})
			return update(feedNotice("Delivery", "Helpin queued the reply for another delivery attempt."))
		}
		if p.Kind == "feed_claim" {
			if c.AssignedUserID != nil && *c.AssignedUserID != actor.UserID {
				return update(feedNotice("Already assigned", "Another teammate has claimed this conversation."))
			}
			if c.AssignedUserID == nil {
				if err = inbox.AssignConversationUser(context.WithValue(ctx, expectedSupportAssigneeKey{}, ""), a.WorkspaceID, c.ID, &actor.UserID, actor.UserID); err != nil {
					return update(feedNotice("Assignment", err.Error()))
				}
			}
			return update(feedNotice("Assigned", "This conversation is assigned to you."))
		}
		if p.Kind == "feed_resolve" || p.Kind == "feed_reopen" {
			next := "resolved"
			if p.Kind == "feed_reopen" {
				next = "open"
			}
			if c.Status != p.Message || (p.Kind == "feed_reopen") != (c.Status == "resolved") {
				return update(feedNotice("Status changed", "The conversation changed. Check its current status on the card."))
			}
			if _, err := inbox.UpdateConversationStatus(ctx, c.WorkspaceID, c.ID, next, actor.UserID); err != nil {
				return update(feedNotice("Status", err.Error()))
			}
			return update(feedNotice("Saved", "Conversation status updated."))
		}
		if !p.Submit {
			return show("")
		}
		if feedMenuAction(p.Kind) || p.Kind == "feed_assign" {
			if p.Draft.Revision != c.MergeRevision {
				return show("This conversation changed. Review it before saving.")
			}
			if err := s.applyFeedTriage(ctx, f, &p, actor.UserID); err != nil {
				return show(err.Error())
			}
			return update(feedNotice("Saved", "Conversation updated in Helpin."))
		}
		if p.Kind == "feed_manage" {
			if p.Draft.Owner != derefString(c.AssignedUserID) || p.Draft.Status != c.Status {
				return show("The status or assignee changed. Review the current values before saving.")
			}
			if p.Owner != p.Draft.Owner && !(p.Owner == "unassigned" && p.Draft.Owner == "") {
				var next *string
				if p.Owner != "unassigned" {
					next = &p.Owner
				}
				if err = inbox.AssignConversationUser(context.WithValue(ctx, expectedSupportAssigneeKey{}, p.Draft.Owner), a.WorkspaceID, c.ID, next, actor.UserID); err != nil {
					return show(err.Error())
				}
			}
			if p.Status != c.Status {
				if _, err = inbox.UpdateConversationStatus(ctx, a.WorkspaceID, c.ID, p.Status, actor.UserID); err != nil {
					return show(err.Error())
				}
			}
			return update(feedNotice("Saved", "Conversation updated in Helpin."))
		}
		clientID := inboundStableID(p.Draft.Thread + ":" + p.Draft.Token)
		prior, err := inbox.messageRepo.FindTeammateReplyByClientID(ctx, a.WorkspaceID, c.ID, actor.UserID, clientID)
		if err != nil {
			return err
		}
		if prior != nil {
			return update(feedNotice("Reply saved", "Your reply is in the conversation."))
		}
		if latest != p.Draft.Latest || c.MergeRevision != p.Draft.Revision || derefString(c.CustomerEmail) != p.Draft.Primary || feedEmailAudience(c) != p.Draft.CC {
			warning := "The conversation changed while you were drafting. Review your reply and destination."
			if latest != p.Draft.Latest {
				m, err := inbox.messageRepo.GetByID(ctx, latest)
				if err != nil {
					return err
				}
				if m != nil {
					warning += " Latest message: " + truncate(tiptap.RichTextToMarkdown(m.Content), 600)
				}
			}
			return show(warning)
		}
		req := model.CreateMessageRequest{Content: p.Body, MessageType: "reply", ClientMessageID: clientID, ExpectedMergeRevision: &p.Draft.Revision, ExpectedLatestPublicReply: &p.Draft.Latest}
		if err := feedDestination(c, p.Destination, &req); err != nil {
			return show(err.Error())
		}
		var cc []string
		if req.DeliveryMode == model.SupportDeliveryEmailOnly {
			if _, err = normalizeSupportEmailAddress(p.To); err != nil {
				return show("Enter a valid recipient email address.")
			}
			for _, raw := range strings.Split(p.CC, ",") {
				if strings.TrimSpace(raw) != "" {
					email, err := normalizeSupportEmailAddress(raw)
					if err != nil {
						return show("Enter valid CC addresses separated by commas.")
					}
					cc = append(cc, email)
				}
			}
			req.CCEmails = cc
		}
		req.AttachmentIDs, err = s.stageFeedFiles(ctx, f, actor.UserID, clientID, feedFiles(p))
		if err != nil {
			return show(err.Error())
		}
		if req.DeliveryMode == model.SupportDeliveryEmailOnly && (!strings.EqualFold(p.To, p.Draft.Primary) || !slices.Equal([]string(c.EmailCC), cc)) {
			_, err = inbox.UpdateConversationEmailRecipients(ctx, a.WorkspaceID, c.ID, model.UpdateConversationEmailRecipientsRequest{PrimaryRecipientEmail: &p.To, CCEmails: append([]string{}, cc...)}, actor.UserID)
			if err != nil {
				return show(err.Error())
			}
		}
		_, err = inbox.CreateConversationMessage(ctx, a.WorkspaceID, c.ID, req, "user", &actor.UserID, nil, nil)
		if err != nil {
			return show(err.Error())
		}
		return update(feedNotice("Reply saved", "Helpin has accepted your reply. Delivery updates appear in the thread."))
	})
}
func feedEmailAudience(c *model.SupportConversation) string {
	recipients := slices.Clone(c.MergeEmailRecipients)
	slices.Sort(recipients)
	return inboundStableID(strings.Join(c.EmailCC, ",") + "|" + strings.Join(recipients, ","))
}

func feedFiles(p feedActionJob) []slack.File {
	files := slices.Clone(p.Draft.Files)
	for _, f := range p.Files {
		if !slices.ContainsFunc(files, func(old slack.File) bool { return old.ID == f.ID }) {
			files = append(files, f)
		}
	}
	return files
}
