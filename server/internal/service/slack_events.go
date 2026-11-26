package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/channels"
	"github.com/helpin-ai/helpin/server/internal/channels/slack"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// SlackEventJob is a durably queued, authenticated Slack callback.
const SlackEventJob = "slack_event"

// Keep staff-channel events on their original side of the support intake boundary
// if an administrator changes the route while the event is queued.
type queuedSlackEvent struct {
	slack.Message
	Inbox        bool   `json:"helpin_inbox,omitempty"`
	NoteThreadID string `json:"helpin_note_thread,omitempty"`
}

// AcceptEvent verifies and stores an event before acknowledging to Slack.
func (s *SlackService) AcceptEvent(ctx context.Context, stamp, signature string, body []byte) (string, error) {
	if !slack.Verify(s.cfg.SigningSecret, stamp, signature, body, s.channels.now()) {
		return "", ErrChannelWebhookUnauthorized
	}
	env, err := slack.Parse(body)
	if err != nil {
		return "", ErrChannelWebhookInvalid
	}
	if env.Type == "url_verification" {
		return env.Challenge, nil
	}
	if env.Type != "event_callback" {
		return "", nil
	}
	a, err := s.channels.accounts.FindLive(ctx, "slack", env.TeamID)
	if err != nil || a == nil {
		return "", err
	}
	if env.Event.Type == "app_uninstalled" {
		return "", s.channels.accounts.Disconnect(ctx, a.WorkspaceID, a.ID, s.channels.now())
	}
	if env.Event.Type == "channel_id_changed" {
		if env.Event.OldChannelID == "" || env.Event.NewChannelID == "" || env.Event.OldChannelID == env.Event.NewChannelID {
			return "", ErrChannelWebhookInvalid
		}
		return "", s.repo.ChangeChannelID(ctx, a.ID, env.Event.OldChannelID, env.Event.NewChannelID)
	}
	welcome := env.Event.Type == "app_home_opened" && env.Event.Tab == "messages" && strings.HasPrefix(env.Event.Channel, "D") && env.Event.User != ""
	if !welcome && !env.Event.HumanMessage() {
		return "", nil
	}
	inbox := false
	if !welcome && !slackDirect(env.Event) {
		route, err := s.repo.EffectiveRoute(ctx, a.ID, env.Event.Channel)
		if err != nil || route == nil {
			return "", err
		}
		if route.Purpose == "on_request" && env.Event.Type != "app_mention" && !s.mentionsBot(env.Event.Text, a.Handle) {
			if env.Event.ThreadTS == "" {
				return "", nil
			}
			existing, err := s.repo.SupportThreadOrigin(ctx, a, route.ChannelID+":"+env.Event.Root())
			if err != nil || existing == nil {
				return "", err
			}
		}
		inbox = route.Purpose == "inbox"
		env.Event.Channel = route.ChannelID
	}
	eventKey := env.Event.TS
	if welcome {
		eventKey = "welcome:" + env.Event.User
	}
	subject := model.SupportChannelSubjectKey(a.ID, env.Event.User)
	data, err := json.Marshal(queuedSlackEvent{Message: env.Event, Inbox: inbox})
	if err != nil {
		return "", err
	}
	return "", s.channels.jobs.Create(ctx, &model.SupportInboundJob{ID: inboundStableID("slack:" + a.ID + ":" + env.Event.Channel + ":" + eventKey), WorkspaceID: &a.WorkspaceID, ChannelAccountID: &a.ID, Kind: SlackEventJob, SubjectKey: &subject, Payload: string(data), Status: "pending", AvailableAt: s.channels.now()})
}

func (s *SlackService) process(ctx context.Context, job *model.SupportInboundJob) error {
	if job.ChannelAccountID == nil {
		return errors.New("missing Slack installation")
	}
	a, err := s.channels.accounts.GetForWebhook(ctx, "slack", *job.ChannelAccountID)
	if a == nil || err != nil {
		return err
	}
	var queued queuedSlackEvent
	if err = json.Unmarshal([]byte(job.Payload), &queued); err != nil {
		return err
	}
	m := queued.Message
	if slackDirect(m) || m.Type == "app_home_opened" {
		token, err := s.channels.decryptToken(a)
		if err != nil {
			return err
		}
		if m.Type == "app_home_opened" {
			return s.welcome(ctx, a, token, m)
		}
		return s.directAgent(ctx, a, token, m)
	}
	route, err := s.repo.EffectiveRoute(ctx, a.ID, m.Channel)
	if route == nil || err != nil || (queued.Inbox && route.Purpose != "inbox") {
		return err
	}
	token, err := s.channels.decryptToken(a)
	if err != nil {
		return err
	}
	m.Channel, m.CurrentChannelID = route.ChannelID, route.CurrentChannelID
	if route.Purpose == "inbox" {
		if kind, details := inboxCreateCommand(m.Text, a.Handle); kind != "" && queued.NoteThreadID == "" {
			return s.beginInboxCreate(ctx, a, token, route, m, kind, details, "")
		}
		queued.Message = m
		return s.receiveFeedNote(ctx, a, token, queued, job.ConversationID)
	}
	if m.ThreadTS != "" {
		thread, err := s.repo.FeedByRoot(ctx, a.ID, route.ChannelID, m.ThreadTS)
		if err != nil || thread != nil {
			return err
		}
	}
	if route.Purpose == "pm" {
		return s.processPM(ctx, a, token, route, m)
	}
	return s.receiveSupport(ctx, a, token, route, m, job, false)
}

func (s *SlackService) receiveSupport(ctx context.Context, a *model.SupportChannelAccount, token string, route *model.SlackRoute, m slack.Message, job *model.SupportInboundJob, manual bool) error {
	person, err := s.slackPerson(ctx, token, m.User)
	if err != nil {
		return err
	}
	name := firstNonEmptyString(person.displayName(), m.User)
	teammate := person.TeamID == a.ExternalAccountID
	explicit := manual || m.Type == "app_mention" || s.mentionsBot(m.Text, a.Handle)
	if explicit && !manual {
		hasTaskReference := slackTaskLink.MatchString(m.Text) || slackTaskKey.MatchString(m.Text)
		if teammate && hasTaskReference && !slackSupportLink.MatchString(m.Text) && slackCommand(m.Text) != "link" && !slackDraftRequest(m.Text, a.Handle) && !slackCreateSupportRequest(m.Text, a.Handle) {
			return s.processPM(ctx, a, token, route, m)
		}
		if (route.Purpose == "on_request" || teammate) && !slackCreateSupportRequest(m.Text, a.Handle) {
			existing, err := s.repo.SupportThreadOrigin(ctx, a, route.ChannelID+":"+m.Root())
			if err != nil {
				return err
			}
			if existing == nil {
				if teammate {
					return s.processPM(ctx, a, token, route, m)
				}
				return s.private(ctx, token, m, "Choose Create support request on the message you’d like to send to Helpin.")
			}
		}
		handled, err := s.supportCommand(ctx, a, token, route, m, teammate)
		if handled || err != nil {
			return err
		}
	}
	e := channels.Event{Kind: channels.EventMessage, ExternalUserID: m.User, ExternalChatID: m.Channel, ExternalMessageKey: m.Channel + ":" + m.TS, ThreadKey: m.Channel + ":" + m.Root(), DefaultCompanyID: route.CRMCompanyID, ChannelName: route.ChannelName, Text: m.Text, DisplayName: name, Teammate: teammate, ExistingThreadOnly: !explicit && (route.Purpose == "mixed" || route.Purpose == "on_request" || teammate)}
	if teammate {
		userID, err := s.repo.Member(ctx, a.ID, m.User)
		if err != nil {
			return err
		}
		e.TeammateUserID = nilIfEmpty(userID)
		ctx = model.WithSlackSource(ctx, s.sourceURL(ctx, token, m.Destination(), m.TS))
	}
	for _, f := range m.Files {
		e.Files = append(e.Files, channels.InboundFile{FileID: f.ID, FileName: firstNonEmptyString(f.Name, "Attachment"), ContentType: f.Mime, Size: f.Size})
	}
	if route.ChannelName == "" && explicit {
		var info struct{ Channel SlackChannel }
		if err := s.api.Get(ctx, token, "conversations.info", url.Values{"channel": {m.Destination()}}, &info); err == nil {
			e.ChannelName = info.Channel.Name
		} else {
			s.channels.logger.WarnContext(ctx, "Slack channel name unavailable", "error", err)
		}
	}
	a.MailboxID = route.MailboxID
	receipt, err := s.channels.inbox.receiveChannelMessage(ctx, a, e, 25<<20)
	if err != nil || receipt.duplicate {
		return err
	}
	if err = s.channels.jobs.SetConversation(ctx, job, receipt.message.ConversationID, receipt.message.ID); err != nil {
		return err
	}
	s.channels.inbox.publishChannelMessage(ctx, a, e, receipt)
	return nil
}

func (s *SlackService) dispatchSlack(ctx context.Context, a *model.SupportChannelAccount, conv *model.SupportConversation, msg *model.SupportMessage, row *model.SupportChannelOutbox) dispatchOutcome {
	if conv.ProviderThreadKey == nil || conv.ChannelAccountID == nil || *conv.ChannelAccountID != a.ID {
		return dispatchOutcome{status: "rejected", code: "destination", detail: "Select the original Slack conversation"}
	}
	channel, root, ok := strings.Cut(*conv.ProviderThreadKey, ":")
	if !ok || channel == "" || root == "" {
		return dispatchOutcome{status: "rejected", code: "destination"}
	}
	route, err := s.repo.EffectiveRoute(ctx, a.ID, channel)
	if err != nil {
		return dispatchOutcome{status: "retry", code: "database"}
	}
	if route == nil || route.Purpose == "pm" || route.Purpose == "inbox" {
		return dispatchOutcome{status: "cancelled", code: "disconnected"}
	}
	token, err := s.channels.decryptToken(a)
	if err != nil {
		return dispatchOutcome{status: "retry", code: "credentials"}
	}
	return s.sendReply(ctx, token, route.CurrentChannelID, root, msg, row)
}

func slackDeliveryError(err error) dispatchOutcome {
	var e *slack.APIError
	if errors.As(err, &e) {
		if e.Code == "ratelimited" {
			return dispatchOutcome{status: "retry", code: e.Code, retry: e.RetryAfter}
		}
		return dispatchOutcome{status: "rejected", code: e.Code, detail: e.Error()}
	}
	return dispatchOutcome{status: "uncertain", code: "unconfirmed", detail: "Slack did not confirm delivery"}
}

func (s *SlackService) sendReply(ctx context.Context, token, channel, root string, msg *model.SupportMessage, row *model.SupportChannelOutbox) dispatchOutcome {
	files, err := s.channels.replyFiles(ctx, msg, 25<<20)
	if err != nil {
		return dispatchOutcome{status: "retry", code: "attachments"}
	}
	parts := 0
	record := func(key string) error { parts++; return s.channels.outbox.RecordPart(ctx, row, parts, key) }
	if strings.TrimSpace(msg.Content) != "" {
		if row.PartsSent == 0 {
			var response struct{ TS string }
			if err = s.api.Call(ctx, token, "chat.postMessage", map[string]any{"channel": channel, "thread_ts": root, "text": tiptap.RichTextToMarkdown(msg.Content), "client_msg_id": msg.ID, "mrkdwn": false, "unfurl_links": false, "unfurl_media": false}, &response); err != nil {
				return slackDeliveryError(err)
			}
			if err = record(response.TS); err != nil {
				return dispatchOutcome{status: "uncertain", code: "receipt"}
			}
		} else {
			parts++
		}
	}
	for _, file := range files {
		if parts < row.PartsSent {
			parts++
			continue
		}
		key, err := s.api.Upload(ctx, token, channel, root, file.Name, file.Data)
		if err != nil {
			return slackDeliveryError(err)
		}
		if err = record(key); err != nil {
			return dispatchOutcome{status: "uncertain", code: "receipt"}
		}
	}
	if parts == 0 {
		return dispatchOutcome{status: "cancelled", code: "empty", detail: "empty Slack reply"}
	}
	return dispatchOutcome{status: "accepted"}
}
