package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/helpin-ai/helpin/server/internal/email/inboundhtml"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// Slack displays the author's reply. The original HTML and quoted history remain accessible in Helpin.
func feedHTMLReply(raw string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(raw))
	if err != nil {
		return inboundhtml.Convert(raw, "")
	}
	doc.Find("[hidden], [aria-hidden=true], [data-helpin-quote=true], head, script, style").Remove()
	doc.Find("[style]").Each(func(_ int, el *goquery.Selection) {
		style := strings.ToLower(strings.Join(strings.Fields(el.AttrOr("style", "")), ""))
		if strings.Contains(style, "display:none") || strings.Contains(style, "visibility:hidden") || strings.Contains(style, "mso-hide:all") {
			el.Remove()
		}
	})
	html, err := doc.Html()
	if err != nil {
		return inboundhtml.Convert(raw, "")
	}
	return inboundhtml.Convert(html, "")
}

func (s *SlackService) feedContent(ctx context.Context, f *feedContext, m *model.SupportMessage) (body string, rich, email bool, err error) {
	body = tiptap.RichTextToMarkdown(m.Content)
	rich = body != strings.TrimSpace(m.Content)
	// The Helpin reply composer stores Markdown directly. Slack's reply modal takes plain text.
	if m.SenderType == "user" && m.ExternalMessageKey == nil && (m.MessageType == "reply" || m.MessageType == "note") {
		rich = rich || metadataStringFromJSON(json.RawMessage(m.Metadata), "via") != "slack"
	}
	email = m.MessageType == "reply" && !m.IsInternal && firstNonEmptyString(derefString(m.ViaChannel), f.conv.Channel) == "email"
	if !email {
		return body, rich, false, nil
	}
	var log *model.SupportEmailLog
	if s.channels.inbox.emailLogRepo != nil {
		log, err = s.channels.inbox.emailLogRepo.GetByMessageID(ctx, m.WorkspaceID, m.ID)
		if err != nil {
			return "", false, true, err
		}
	}
	if log != nil && log.Direction == "inbound" {
		if m.ViaChannel == nil {
			m.ViaChannel = strPtr("email")
		}
		messages := []model.SupportMessage{*m}
		hydrateEmailBodiesFromLogs(messages, []model.SupportEmailLog{*log})
		*m = messages[0]
		if strings.TrimSpace(derefString(m.SenderDisplayName)) == "" && log.FromDisplayName != "" {
			m.SenderDisplayName = &log.FromDisplayName
		}
		// Forwarded messages have their own authored attribution and projected body.
		var meta map[string]any
		if json.Unmarshal([]byte(m.Metadata), &meta) == nil && forwardedAttributionFromMetadata(meta) != nil {
			return m.Content, true, true, nil
		}
		if m.HTMLBody != "" {
			if m.EmailHasQuotedContent != nil && *m.EmailHasQuotedContent && !strings.Contains(m.HTMLBody, inboundhtml.QuotedAttr) {
				return m.EmailVisibleText, true, true, nil
			}
			return feedHTMLReply(m.HTMLBody), true, true, nil
		}
		if m.EmailVisibleText != "" || m.EmailHasQuotedContent != nil && *m.EmailHasQuotedContent {
			return m.EmailVisibleText, false, true, nil
		}
	}
	if raw := strings.TrimSpace(m.Content); strings.HasPrefix(raw, "<") && strings.Contains(raw, ">") && body != raw {
		return feedHTMLReply(raw), true, true, nil
	}
	return body, rich, true, nil
}
