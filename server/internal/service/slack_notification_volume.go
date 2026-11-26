package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/channels/slack"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	maxPersonalDMsPerHour = 5
	maxItemsPerDM         = 12
)

type personalDM struct {
	Posted  time.Time
	Channel string
	TS      string
	Items   []personalDMItem
}

type personalDMItem struct {
	JobID string
	Added time.Time
	slackNotification
}

func (item personalDMItem) sameAlert(p slackNotification) bool {
	return item.EntityType == p.EntityType && item.EntityID == p.EntityID && item.EventType == p.EventType &&
		item.Metadata["interaction_id"] == p.Metadata["interaction_id"]
}

type personalDMDeferred struct{ until time.Time }

func (e *personalDMDeferred) Error() string { return "personal Slack DM window is full" }

// The linked member row is locked until Slack accepts the post or edit. Store routing
// references only. Read titles from the recipient's inbox when rebuilding a message.
func (s *SlackService) sendPersonalDM(ctx context.Context, raw []byte, jobID string, ws *model.Workspace, account, token string, p slackNotification) ([]byte, error) {
	var cards []personalDM
	if err := json.Unmarshal(raw, &cards); err != nil {
		return nil, err
	}
	now := s.channels.now().UTC()
	cards = slices.DeleteFunc(cards, func(c personalDM) bool { return !c.Posted.After(now.Add(-time.Hour)) })
	for _, card := range cards {
		for _, item := range card.Items {
			if item.JobID == jobID || (item.Added.After(now.Add(-15*time.Minute)) && item.sameAlert(p)) {
				return nil, nil
			}
		}
	}
	newPost := len(cards) < maxPersonalDMsPerHour
	var card *personalDM
	if newPost {
		cards = append(cards, personalDM{Posted: now})
		card = &cards[len(cards)-1]
	} else {
		for i := len(cards) - 1; i >= 0; i-- {
			if len(cards[i].Items) < maxItemsPerDM {
				card = &cards[i]
				break
			}
		}
		if card == nil {
			return nil, &personalDMDeferred{cards[0].Posted.Add(time.Hour)}
		}
	}

	var alerts []slackNotification
	var retained []personalDMItem
	for _, item := range card.Items {
		if item.sameAlert(p) {
			continue
		}
		old := item.slackNotification
		ok, err := s.personalDMAllowed(ctx, ws.ID, account, old)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		n, err := s.notifications.notifRepo.GetExisting(ctx, old.UserID, old.EntityType, old.EntityID, ws.ID)
		if err != nil {
			return nil, err
		}
		if n != nil && n.EventType == old.EventType && strings.TrimSpace(n.Title) != "" {
			old.Title = truncate(n.Title, 500)
		} else {
			// Slack-only alerts can have no inbox entry.
			old.Title = old.dmLabel()
		}
		alerts = append(alerts, old)
		retained = append(retained, item)
	}
	message := personalDMMessage(s.notifications.appBaseURL, ws.Slug, append(alerts, p))
	if newPost {
		message["channel"], message["client_msg_id"] = p.SlackUser, jobID
		var receipt struct{ Channel, TS string }
		if err := s.api.Call(ctx, token, "chat.postMessage", message, &receipt); err != nil {
			return nil, err
		}
		if receipt.Channel == "" || receipt.TS == "" {
			return nil, fmt.Errorf("Slack personal DM receipt missing channel or timestamp")
		}
		card.Channel, card.TS = receipt.Channel, receipt.TS
	} else {
		message["channel"], message["ts"] = card.Channel, card.TS
		if err := s.api.Call(ctx, token, "chat.update", message, nil); err != nil {
			var apiErr *slack.APIError
			if errors.As(err, &apiErr) && apiErr.Code == "message_not_found" {
				return nil, &personalDMDeferred{cards[0].Posted.Add(time.Hour)}
			}
			return nil, err
		}
	}
	p.Title = ""
	card.Items = append(retained, personalDMItem{JobID: jobID, Added: now, slackNotification: p})
	return json.Marshal(cards)
}

func (p slackNotification) dmLabel() string {
	label := strings.NewReplacer("_", " ", ".", " · ").Replace(p.EventType)
	return label + " (" + truncate(p.EntityID, 12) + ")"
}

func personalDMMessage(base, slug string, alerts []slackNotification) map[string]any {
	var lines []string
	var blocks []map[string]any
	for i, p := range alerts {
		if p.Title == "" {
			p.Title = p.dmLabel()
		}
		lines = append(lines, p.Title)
		blocks = append(blocks, map[string]any{"type": "section", "text": map[string]string{"type": "plain_text", "text": p.Title}})
		var links []string
		if link := buildEntityURL(base, slug, p.EntityType, p.EntityID, p.Metadata); link != "" {
			lines = append(lines, link)
			links = append(links, "<"+link+"|Open in Helpin>")
		}
		if i == len(alerts)-1 && base != "" && slug != "" {
			settings := strings.TrimRight(base, "/") + "/w/" + url.PathEscape(slug) + "/settings/notifications"
			if len(alerts) == 1 {
				settings += "?slack=" + url.QueryEscape(slackPreferenceKey(p.event("")))
			}
			lines = append(lines, "Manage Slack notifications: "+settings)
			links = append(links, "<"+settings+"|Manage Slack notifications>")
		}
		if len(links) > 0 {
			blocks = append(blocks, map[string]any{"type": "context", "elements": []map[string]string{{"type": "mrkdwn", "text": strings.Join(links, " · ")}}})
		}
	}
	return map[string]any{"text": strings.Join(lines, "\n"), "mrkdwn": false, "blocks": blocks, "unfurl_links": false, "unfurl_media": false}
}
