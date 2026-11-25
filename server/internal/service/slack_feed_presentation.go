package service

import (
	"encoding/json"
	"github.com/helpin-ai/helpin/server/internal/model"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"
)

func feedOneLine(text string) string { return strings.Join(strings.Fields(text), " ") }

func feedExcerpt(subject, text string) string {
	text = feedOneLine(text)
	if strings.EqualFold(feedOneLine(subject), text) {
		return ""
	}
	if utf8.RuneCountInString(text) <= 100 {
		return text
	}
	runes := []rune(text)
	cut := 99
	if !unicode.IsSpace(runes[cut]) {
		if boundary := strings.LastIndex(string(runes[:cut]), " "); boundary >= 0 {
			return string(runes[:cut])[:boundary] + "…"
		}
	}
	return strings.TrimSpace(string(runes[:cut])) + "…"
}

type feedSpeaker struct{ name, role, avatar, icon string }

func (a feedSpeaker) label() string {
	if a.name == "" || strings.EqualFold(a.name, a.role) {
		return a.role
	}
	return a.name + " · " + a.role
}

func (a feedSpeaker) header() map[string]any {
	var elements []any
	if a.avatar != "" {
		elements = append(elements, map[string]any{"type": "image", "image_url": a.avatar, "alt_text": firstNonEmptyString(a.name, a.role)})
	} else if a.icon != "" {
		elements = append(elements, slackText(a.icon))
	} else if a.name != "" {
		elements = append(elements, slackText(supportTranscriptAvatarText(a.name, false)))
	}
	if a.name != "" && !strings.EqualFold(a.name, a.role) {
		if strings.ContainsAny(a.name, "*_~`<>&@:\\") {
			elements = append(elements, slackText(a.name))
		} else {
			elements = append(elements, map[string]any{"type": "mrkdwn", "text": "*" + a.name + "*", "verbatim": true})
		}
	}
	elements = append(elements, map[string]any{"type": "mrkdwn", "text": "*" + a.role + "*", "verbatim": true})
	return map[string]any{"type": "context", "elements": elements}
}

func (f *feedContext) feedAuthor(m *model.SupportMessage) feedSpeaker {
	a := feedSpeaker{name: truncate(feedOneLine(derefString(m.SenderDisplayName)), 120), avatar: supportTranscriptAttachmentURL(derefString(m.SenderAvatarURL))}
	if m.MessageType == "system" {
		return feedSpeaker{role: "Conversation update"}
	}
	channel := firstNonEmptyString(derefString(m.ViaChannel), f.conv.Channel)
	switch m.SenderType {
	case "customer":
		from := m.EmailFrom
		if address, err := mail.ParseAddress(from); err == nil {
			from = address.Address
			if a.name == "" {
				a.name = address.Name
			}
		}
		primary := from == "" || strings.EqualFold(from, derefString(f.conv.CustomerEmail))
		if a.name == "" {
			a.name = from
		}
		if primary && a.name == "" {
			a.name = feedOneLine(derefString(f.conv.CustomerName))
		}
		a.role = "Contact"
		if channel == "email" {
			a.role = "✉️ Email"
		} else if channel == "widget" || model.SupportChannelHasChatSession(channel) {
			a.role = "Visitor"
		}
		if f.contact != nil && (from == "" && primary || strings.EqualFold(from, derefString(f.contact.Email))) {
			if f.contact.LifecycleStage == "customer" {
				a.role = "Customer"
			}
			if a.avatar == "" {
				a.avatar = supportTranscriptAttachmentURL(derefString(f.contact.AvatarURL))
			}
		}
		if a.name == "" {
			a.icon = "👤"
		}
	case "user", "agent", "ai":
		var meta struct {
			AIAgentID string `json:"ai_agent_id"`
		}
		legacyAI := m.Metadata != "" && json.Unmarshal([]byte(m.Metadata), &meta) == nil && meta.AIAgentID != ""
		ai := m.SenderType == "ai" || m.SenderUserID == nil && (m.SenderType == "agent" || m.SenderAgentID != nil || legacyAI)
		if ai {
			a.role = "AI reply"
			a.name = firstNonEmptyString(a.name, "Helpin AI")
			a.icon = "🤖"
		} else {
			a.role = "Team reply"
			if a.name == "" {
				a.icon = "🎧"
			}
		}
	default:
		a.role = "Conversation update"
	}
	if m.IsInternal {
		a.role = "📝 Internal note"
	}
	a.name = truncate(feedOneLine(a.name), 120)
	return a
}

func feedReplyBody(body string, formatted bool) ([]any, bool) {
	if strings.TrimSpace(body) == "" {
		return nil, false
	}
	if formatted {
		return []any{slackMarkdown(body)}, utf8.RuneCountInString(body) > 12000
	}
	runes := []rune(body)
	var blocks []any
	for len(runes) > 0 && len(blocks) < 40 {
		end := min(2800, len(runes))
		blocks = append(blocks, map[string]any{"type": "section", "text": slackText(string(runes[:end]))})
		runes = runes[end:]
	}
	return blocks, len(runes) > 0
}

func feedRichText(kind string, elements ...any) map[string]any {
	return map[string]any{"type": "rich_text", "elements": []any{map[string]any{"type": kind, "elements": elements}}}
}
