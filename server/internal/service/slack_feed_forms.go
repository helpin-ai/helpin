package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/channels/slack"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func feedOption(label, value string) map[string]any {
	return map[string]any{"text": slackText(truncate(label, 75)), "value": value}
}
func feedOptions(c *model.SupportConversation) []map[string]any {
	var options []map[string]any
	for _, d := range c.ProviderDestinations {
		options = append(options, feedOption(d.Provider+" · "+d.Name, d.Provider+":"+d.ID))
	}
	if c.AnonymousID != nil || c.MergeHasWidget || model.SupportChannelHasChatSession(c.Source) {
		options = append(options, feedOption("Website chat", "widget"))
	}
	if supportConversationPortalReplySurface(c) {
		options = append(options, feedOption("Customer portal", "portal"))
	}
	if email := derefString(c.CustomerEmail); email != "" && !c.EmailUnsubscribed {
		options = append(options, feedOption("Email · "+email, "email"))
	}
	return options
}
func (s *SlackService) defaultFeedDestination(ctx context.Context, c *model.SupportConversation, clicked string, options []map[string]any) (string, error) {
	if len(options) == 1 {
		only := options[0]["value"].(string)
		if only == "email" && c.PrimaryRecipientState == model.SupportPrimaryRecipientStateUnconfirmed {
			return "", nil
		}
		return only, nil
	}
	channel, origin, err := s.repo.FeedReplyOrigin(ctx, c.WorkspaceID, c.ID, clicked)
	if err != nil {
		return "", err
	}
	if channel == "" && c.MergeRevision == 0 {
		channel, origin = c.Channel, c.ID
	}
	if model.SupportChannelHasChatSession(channel) {
		channel = "widget"
	}
	if channel == "email" && c.PrimaryRecipientState == model.SupportPrimaryRecipientStateUnconfirmed {
		return "", nil
	}
	for _, opt := range options {
		value := opt["value"].(string)
		if value == channel || value == channel+":"+origin {
			return value, nil
		}
	}
	return "", nil
}

func feedDestination(c *model.SupportConversation, selected string, req *model.CreateMessageRequest) error {
	for _, opt := range feedOptions(c) {
		if opt["value"] != selected {
			continue
		}
		req.DeliveryMode = model.SupportDeliveryChatOnly
		req.ReplyTransport, req.ProviderOriginID, _ = strings.Cut(selected, ":")
		if selected == "email" {
			req.DeliveryMode = model.SupportDeliveryEmailOnly
			req.ReplyTransport = ""
		}
		return nil
	}
	return errors.New("Choose an available delivery destination.")
}

// Slack permits 100 options per group. Keep all eligible teammates and destinations searchable.
func feedSelectBlock(id, label string, options []map[string]any, initial string, optional bool) map[string]any {
	block := selectBlock(id, label, options, initial, optional)
	if len(options) <= 100 {
		return block
	}
	field := block["element"].(map[string]any)
	delete(field, "options")
	var groups []map[string]any
	for i := 0; i < len(options); i += 100 {
		end := min(i+100, len(options))
		groups = append(groups, map[string]any{"label": slackText(fmt.Sprintf("%s %d–%d", label, i+1, end)), "options": options[i:end]})
	}
	field["option_groups"] = groups
	return block
}
func (s *SlackService) feedForm(ctx context.Context, f *feedContext, p *feedActionJob, warning string) (map[string]any, error) {
	c := f.conv
	title, submit := f.replyLabel(), "Send reply"
	customer := firstNonEmptyString(derefString(c.CustomerName), derefString(c.CustomerEmail), "Customer")
	blocks := []any{map[string]any{"type": "context", "elements": []any{slackText(truncate(fmt.Sprintf("#%d · %s · %s", c.DisplayID, customer, c.Subject), 1000))}}}
	if warning != "" {
		blocks = append(blocks, map[string]any{"type": "section", "text": slackText(truncate(warning, 2000))})
	}
	if p.Kind == "feed_assign" || feedMenuAction(p.Kind) {
		formTitle, button, fields, err := s.feedTriageForm(ctx, f, p)
		if err != nil {
			return nil, err
		}
		title, submit = formTitle, button
		blocks = append(blocks, fields...)
		if submit == "" {
			return map[string]any{"type": "modal", "title": slackText(title), "close": slackText("Close"), "blocks": blocks}, nil
		}
	} else if p.Kind == "feed_manage" {
		title, submit = "Manage conversation", "Save"
		options := []map[string]any{feedOption("Unassigned", "unassigned")}
		members, err := s.channels.inbox.ListConversationAssignableUsers(ctx, c.WorkspaceID, c.ID)
		if err != nil {
			return nil, err
		}
		for _, m := range members {
			if m.UserID != nil && m.AssignmentDisabledReason == nil {
				options = append(options, feedOption(firstNonEmptyString(m.DisplayName, m.Email), *m.UserID))
			}
		}
		blocks = append(blocks, feedSelectBlock("owner", "Assignee", options, firstNonEmptyString(derefString(c.AssignedUserID), "unassigned"), false), selectBlock("status", "Status", []map[string]any{feedOption("Open", "open"), feedOption("Waiting on customer", "waiting_on_customer"), feedOption("Resolved", "resolved"), feedOption("Spam", "spam")}, c.Status, false))
	} else {
		options := feedOptions(c)
		if len(options) == 0 {
			return feedNotice(title, "This conversation has no available reply destination. Open it in Helpin to update the recipient."), nil
		}
		if len(c.MergeParticipants) > 1 {
			blocks = append(blocks, map[string]any{"type": "section", "text": slackText("Merged conversation participants: " + truncate(strings.Join(c.MergeParticipants, ", "), 2200))})
		}
		if len(c.MergeEmailRecipients) > 1 {
			blocks = append(blocks, map[string]any{"type": "section", "text": slackText("Email also reaches the merged recipients: " + truncate(strings.Join(c.MergeEmailRecipients, ", "), 2200))})
		}
		selected := p.Destination
		if selected == "" && !p.Submit {
			var err error
			selected, err = s.defaultFeedDestination(ctx, c, p.Message, options)
			if err != nil {
				return nil, err
			}
		}
		blocks = append(blocks, feedSelectBlock("destination", "Send via", options, selected, false))
		if c.CustomerEmail != nil && !c.EmailUnsubscribed {
			to, cc := derefString(c.CustomerEmail), strings.Join(c.EmailCC, ", ")
			if p.Submit {
				to = p.To
				cc = p.CC
			}
			toField := inputBlock("to", "Email recipient", to, false)
			toField["optional"] = true
			ccField := inputBlock("cc", "CC for email", cc, false)
			ccField["optional"] = true
			blocks = append(blocks, toField, ccField)
		}
		body := inputBlock("body", "Reply", p.Body, true)
		if selected != "" && !p.Submit {
			body["element"].(map[string]any)["focus_on_load"] = true
		}
		if p.Body != "" {
			body["element"].(map[string]any)["initial_value"] = p.Body
		}
		blocks = append(blocks, body)
		files := feedFiles(*p)
		p.Draft.Files = nil
		for _, file := range files {
			p.Draft.Files = append(p.Draft.Files, slack.File{ID: file.ID, Name: truncate(file.Name, 80)})
		}
		if len(files) > 0 {
			opts := make([]map[string]any, 0, len(files))
			for _, file := range files {
				opts = append(opts, feedOption(firstNonEmptyString(file.Name, "Attachment"), file.ID))
			}
			blocks = append(blocks, map[string]any{"type": "input", "block_id": "retained", "optional": true, "label": slackText("Keep attachments"), "element": map[string]any{"type": "multi_static_select", "action_id": "value", "options": opts, "initial_options": opts}})
		}
		blocks = append(blocks, map[string]any{"type": "input", "block_id": "files", "optional": true, "label": slackText("Attachments"), "element": map[string]any{"type": "file_input", "action_id": "value", "max_files": 5}})
	}
	meta, err := json.Marshal(p.Draft)
	if err != nil {
		return nil, err
	}
	return map[string]any{"type": "modal", "callback_id": p.Kind, "private_metadata": string(meta), "title": slackText(title), "submit": slackText(submit), "close": slackText("Cancel"), "blocks": blocks}, nil
}
