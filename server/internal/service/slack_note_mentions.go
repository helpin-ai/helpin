package service

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/channels/slack"
)

// Slack leaves mention IDs in message text. Code spans and blocks are literal.
var slackNoteTokens = regexp.MustCompile("(?s)```.*?(?:```|$)|`[^`\n]*(?:`|\n|$)|<@[UW][A-Z0-9]+(?:\\|[^<>|\r\n]+)?>")

type slackPerson struct {
	ID       string
	Name     string
	TeamID   string `json:"team_id"`
	RealName string `json:"real_name"`
	Profile  struct {
		DisplayName string `json:"display_name"`
		RealName    string `json:"real_name"`
	}
}

func (s *SlackService) slackPerson(ctx context.Context, token, id string) (slackPerson, error) {
	var response struct{ User slackPerson }
	err := s.api.Get(ctx, token, "users.info", url.Values{"user": {id}}, &response)
	return response.User, err
}

func (u slackPerson) displayName() string {
	return firstNonEmptyString(u.Profile.DisplayName, u.Profile.RealName, u.RealName, u.Name)
}

var slackNoteNameEscapes = strings.NewReplacer("\\", "\\\\", "*", "\\*", "_", "\\_", "~", "\\~", "`", "\\`", "[", "\\[", "]", "\\]", "<", "\\<", ">", "\\>", "&", "&amp;")

func (s *SlackService) feedNoteText(ctx context.Context, token, body string) string {
	if !strings.Contains(body, "<@") {
		return body
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	names := make(map[string]string)
	return slackNoteTokens.ReplaceAllStringFunc(body, func(part string) string {
		if !strings.HasPrefix(part, "<@") {
			return part
		}
		id, alias, _ := strings.Cut(part[2:len(part)-1], "|")
		name, known := names[id]
		if !known && len(names) < 20 && ctx.Err() == nil {
			u, err := s.slackPerson(ctx, token, id)
			if err != nil {
				s.channels.logger.DebugContext(ctx, "Slack note mention lookup unavailable", "error", err)
				var apiErr *slack.APIError
				if errors.As(err, &apiErr) && apiErr.Code == "ratelimited" {
					cancel()
				}
			} else if u.ID == id {
				name = u.displayName()
			}
			names[id] = name
		}
		name = truncate(feedOneLine(firstNonEmptyString(name, alias, "Slack member")), 120)
		return "@" + slackNoteNameEscapes.Replace(name)
	})
}
