package service

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/helpin-ai/helpin/server/internal/email/inboundhtml"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

const feedEmailSignatureSelector = ".gmail_signature, .gmail_signature_prefix, .moz-signature, #Signature, #AppleMailSignature, [data-smartmail=gmail_signature]"

// Keep quote markers in place until the inbox parser has expanded Outlook reply boundaries.
func feedHTMLReply(raw string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(raw))
	if err != nil {
		return inboundhtml.Convert(raw, "")
	}
	doc.Find("["+inboundhtml.QuotedAttr+"]").SetAttr(inboundhtml.QuotedAttr, "true")
	doc.Find("[hidden], [aria-hidden=true], head, script, style, " + feedEmailSignatureSelector).Remove()
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
	return feedEmailText(inboundhtml.Convert(html, ""), nil)
}

// A provider's visible text can be more precise than its HTML quote markers.
// Use HTML signatures only to identify matching footer text in that projection.
func feedEmailSignatures(raw string) []string {
	if raw == "" {
		return nil
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(raw))
	if err != nil {
		return nil
	}
	var signatures []string
	doc.Find(feedEmailSignatureSelector).Each(func(_ int, el *goquery.Selection) {
		if html, err := el.Html(); err == nil {
			if signature := supportTranscriptPlainText(html); signature != "" {
				signatures = append(signatures, signature)
			}
		}
	})
	slices.Reverse(signatures)
	return signatures
}

func feedEmailText(raw string, signatures []string) string {
	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(raw, "\r\n", "\n")), "\n")
	// A reply attribution followed exclusively by quoted lines terminates the authored text.
	end := len(lines)
	for end > 0 && (strings.HasPrefix(strings.TrimSpace(lines[end-1]), ">") || strings.TrimSpace(lines[end-1]) == "") {
		end--
	}
	if end > 0 && end < len(lines) {
		header := strings.ToLower(strings.TrimSpace(lines[end-1]))
		if strings.HasPrefix(header, "on ") && strings.HasSuffix(header, " wrote:") {
			lines = lines[:end-1]
		}
	}
	fence := ""
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			if fence == "" {
				fence = line[:3]
			} else if strings.Trim(line, fence[:1]) == "" && strings.HasPrefix(line, fence) {
				fence = ""
			}
			continue
		}
		if fence == "" && (line == "--" || line == supportEmailReplyDelimiter) {
			lines = lines[:i]
			break
		}
	}
	text := strings.TrimSpace(strings.Join(lines, "\n"))
	for _, signature := range signatures {
		if text == signature {
			return ""
		}
		text = strings.TrimSpace(strings.TrimSuffix(text, "\n"+signature))
	}
	return text
}

func (s *SlackService) feedContent(ctx context.Context, f *feedContext, m *model.SupportMessage) (body string, rich, email bool, err error) {
	body = tiptap.RichTextToMarkdown(m.Content)
	rich = body != strings.TrimSpace(m.Content)
	// The Helpin reply composer stores Markdown directly. Slack's reply modal takes plain text.
	if m.SenderType == "user" && m.ExternalMessageKey == nil && (m.MessageType == "reply" || m.MessageType == "note") {
		rich = rich || metadataStringFromJSON(json.RawMessage(m.Metadata), "via") != "slack"
	}
	channel := derefString(m.ViaChannel)
	if channel == "" && !m.IsInternal {
		channel = f.conv.Channel
	}
	email = m.MessageType == "reply" && channel == "email"
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
	// A teammate replying from their email client is logged as outbound/external.
	receivedEmail := log != nil && (log.Direction == "inbound" || log.Status == "external")
	if receivedEmail {
		if m.ViaChannel == nil {
			m.ViaChannel = strPtr("email")
		}
		hydrateReceivedEmailBody(m, log)
		if strings.TrimSpace(derefString(m.SenderDisplayName)) == "" && log.FromDisplayName != "" {
			m.SenderDisplayName = &log.FromDisplayName
		}
	}
	var meta map[string]any
	if json.Unmarshal([]byte(m.Metadata), &meta) == nil && forwardedAttributionFromMetadata(meta) != nil {
		note, original := forwardedEmailProjectionParts(m.Content)
		signatures := feedEmailSignatures(m.HTMLBody)
		return strings.Join(nonEmptyForwardedEmailParts(feedEmailText(note, signatures), feedEmailText(original, signatures)), "\n\n"), true, true, nil
	}
	hasQuote := m.EmailHasQuotedContent != nil && *m.EmailHasQuotedContent
	if m.HTMLBody != "" && (!hasQuote || strings.Contains(m.HTMLBody, inboundhtml.QuotedAttr)) {
		return feedHTMLReply(m.HTMLBody), true, true, nil
	}
	if m.EmailVisibleText != "" || hasQuote {
		return feedEmailText(m.EmailVisibleText, feedEmailSignatures(m.HTMLBody)), m.HTMLBody != "", true, nil
	}
	if raw := strings.TrimSpace(m.Content); strings.HasPrefix(raw, "<") && strings.Contains(raw, ">") && body != raw {
		return feedHTMLReply(raw), true, true, nil
	}
	if m.SenderType == "customer" || receivedEmail {
		body = feedEmailText(body, nil)
	}
	return body, rich, true, nil
}
