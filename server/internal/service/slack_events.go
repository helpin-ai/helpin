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
	if !env.Event.HumanMessage() {
		return "", nil
	}
	route, err := s.repo.Route(ctx, a.ID, env.Event.Channel)
	if err != nil || route == nil {
		return "", err
	}
	env.Event.Channel = route.ChannelID
	subject := model.SupportChannelSubjectKey(a.ID, env.Event.User)
	data, err := json.Marshal(env.Event)
	if err != nil {
		return "", err
	}
	return "", s.channels.jobs.Create(ctx, &model.SupportInboundJob{ID: inboundStableID("slack:" + a.ID + ":" + env.Event.Channel + ":" + env.Event.TS), WorkspaceID: &a.WorkspaceID, ChannelAccountID: &a.ID, Kind: SlackEventJob, SubjectKey: &subject, Payload: string(data), Status: "pending", AvailableAt: s.channels.now()})
}

func (s *SlackService) process(ctx context.Context, job *model.SupportInboundJob) error {
	if job.ChannelAccountID == nil {
		return errors.New("missing Slack installation")
	}
	a, err := s.channels.accounts.GetForWebhook(ctx, "slack", *job.ChannelAccountID)
	if a == nil || err != nil {
		return err
	}
	var m slack.Message
	if err = json.Unmarshal([]byte(job.Payload), &m); err != nil {
		return err
	}
	route, err := s.repo.Route(ctx, a.ID, m.Channel)
	if route == nil || err != nil {
		return err
	}
	token, err := s.channels.decryptToken(a)
	if err != nil {
		return err
	}
	m.Channel, m.CurrentChannelID = route.ChannelID, route.CurrentChannelID
	if route.Purpose == "pm" {
		return s.processPM(ctx, a, token, route, m)
	}
	return s.receiveSupport(ctx, a, token, route, m, job, false)
}

func (s *SlackService) receiveSupport(ctx context.Context, a *model.SupportChannelAccount, token string, route *model.SlackRoute, m slack.Message, job *model.SupportInboundJob, manual bool) error {
	var result struct {
		User struct {
			TeamID  string `json:"team_id"`
			Name    string
			Profile struct {
				DisplayName string `json:"display_name"`
				RealName    string `json:"real_name"`
			}
		}
	}
	if err := s.api.Get(ctx, token, "users.info", url.Values{"user": {m.User}}, &result); err != nil {
		return err
	}
	name := result.User.Profile.DisplayName
	if name == "" {
		name = result.User.Profile.RealName
	}
	if name == "" {
		name = result.User.Name
	}
	if name == "" {
		name = m.User
	}
	teammate := result.User.TeamID == a.ExternalAccountID
	explicit := manual || s.mentionsBot(m.Text, a.Handle)
	if explicit && !manual {
		handled, err := s.supportCommand(ctx, a, token, route, m, teammate)
		if handled || err != nil {
			return err
		}
	}
	e := channels.Event{Kind: channels.EventMessage, ExternalUserID: m.User, ExternalChatID: m.Channel, ExternalMessageKey: m.Channel + ":" + m.TS, ThreadKey: m.Channel + ":" + m.Root(), ChannelName: route.ChannelName, Text: m.Text, DisplayName: name, Teammate: teammate, ExistingThreadOnly: !explicit && (route.Purpose == "mixed" || teammate)}
	for _, f := range m.Files {
		e.Files = append(e.Files, channels.InboundFile{FileID: f.ID, FileName: firstNonEmptyString(f.Name, "Attachment"), ContentType: f.Mime, Size: f.Size})
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
	route, err := s.repo.Route(ctx, a.ID, channel)
	if err != nil {
		return dispatchOutcome{status: "retry", code: "database"}
	}
	if route == nil || route.Purpose == "pm" {
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
