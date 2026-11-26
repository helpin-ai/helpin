package service

import (
	"encoding/json"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/helpin-ai/helpin/server/internal/linkpreview"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func feedOneLine(text string) string { return strings.Join(strings.Fields(text), " ") }

func feedExcerpt(subject, text string) string {
	text = feedOneLine(text)
	title := strings.TrimRight(feedOneLine(subject), "….")
	if title != "" && (strings.EqualFold(title, strings.TrimRight(text, "….")) ||
		utf8.RuneCountInString(title) >= 45 && strings.HasPrefix(strings.ToLower(text), strings.ToLower(title))) {
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

type feedSpeaker struct{ name, role, avatar, icon, colorSeed string }

func (a feedSpeaker) label() string {
	if a.name == "" || strings.EqualFold(a.name, a.role) {
		return a.role
	}
	return a.name + " · " + a.role
}

func (a feedSpeaker) header(apiBase string) map[string]any {
	initials := supportTranscriptAvatarText(a.name, false)
	avatar, prefix := a.avatar, ""
	if avatar == "" {
		switch {
		case a.icon != "":
			prefix = a.icon + " "
		case a.name != "":
			seed := firstNonEmptyString(a.colorSeed, strings.ToLower(a.name))
			avatar = linkpreview.InitialsAvatarURL(apiBase, initials, seed)
			if avatar == "" {
				prefix = initials + " "
			}
		}
	}

	var elements []any
	if avatar != "" {
		elements = append(elements, map[string]any{"type": "image", "image_url": avatar, "alt_text": initials})
	}
	// Slack spaces context items independently. Keep the icon fallback, name,
	// separator and role together, including names that need literal text.
	if strings.ContainsAny(a.name, "*_~`<>&@:\\") {
		elements = append(elements, slackText(prefix+a.label()))
	} else {
		label := "*" + a.role + "*"
		if a.name != "" && !strings.EqualFold(a.name, a.role) {
			label = "*" + a.name + "* · " + label
		}
		elements = append(elements, map[string]any{"type": "mrkdwn", "text": prefix + label, "verbatim": true})
	}
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
		primary := from == "" && f.conv.MergeRevision == 0 || from != "" && strings.EqualFold(from, derefString(f.conv.CustomerEmail))
		a.colorSeed = strings.ToLower(from)
		if primary && (from != "" || a.name == "" || strings.EqualFold(a.name, derefString(f.conv.CustomerName))) {
			a.colorSeed = firstNonEmptyString(derefString(f.conv.CRMContactID), strings.ToLower(derefString(f.conv.CustomerEmail)), derefString(f.conv.AnonymousID), derefString(f.conv.ChannelIdentityID), a.colorSeed)
		}
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
		if f.contact != nil && (from == "" && f.conv.MergeRevision == 0 || from != "" && strings.EqualFold(from, derefString(f.contact.Email))) {
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
			a.colorSeed = derefString(m.SenderUserID)
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

// Use two-space Markdown hard breaks for Slack. Only rewrite breaks parsed as prose,
// so code samples and escaped literal backslashes survive intact.
func feedMarkdownLineBreaks(body string) string {
	if !strings.Contains(body, "\\\n") && !strings.Contains(body, "\\\r\n") {
		return body
	}
	doc := supportTranscriptMarkdownRenderer.Parser().Parse(text.NewReader([]byte(body)))
	var out strings.Builder
	start := 0
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if t, ok := n.(*ast.Text); entering && ok && t.HardLineBreak() {
			end := t.Segment.Stop
			if end < len(body) && body[end] == '\\' {
				out.WriteString(body[start:end])
				out.WriteString("  ")
				start = end + 1
			}
		}
		return ast.WalkContinue, nil
	})
	if start == 0 {
		return body
	}
	out.WriteString(body[start:])
	return out.String()
}

func feedReplyBody(body string, formatted bool) ([]any, bool) {
	if strings.TrimSpace(body) == "" {
		return nil, false
	}
	if formatted {
		body = feedMarkdownLineBreaks(body)
		return []any{slackMarkdown(body)}, utf8.RuneCountInString(body) > 12000
	}
	runes := []rune(body)
	var blocks []any
	for len(runes) > 0 && len(blocks) < 12 {
		end := min(2800, len(runes))
		blocks = append(blocks, map[string]any{"type": "section", "text": slackText(string(runes[:end]))})
		runes = runes[end:]
	}
	return blocks, len(runes) > 0
}

func feedRichText(kind string, elements ...any) map[string]any {
	return map[string]any{"type": "rich_text", "elements": []any{map[string]any{"type": kind, "elements": elements}}}
}
