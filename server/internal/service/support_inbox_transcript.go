package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"log/slog"
	"net/url"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	nethtml "golang.org/x/net/html"

	"github.com/helpin-ai/helpin/server/internal/email/inboundhtml"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

var supportTranscriptPolicy = bluemonday.UGCPolicy()

// Stored replies may contain either Markdown or HTML. Allow HTML during parsing
// so code fences and inline markup keep their meaning, then always sanitize the
// result before it can reach the email template or its text alternative.
var supportTranscriptMarkdownRenderer = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(gmhtml.WithHardWraps(), gmhtml.WithXHTML(), gmhtml.WithUnsafe()),
)

var supportTranscriptTemplate = template.Must(template.New("transcript").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"></head>
<body style="margin:0;padding:0;background:#f5f5f4;color:#292524;font-family:Arial,Helvetica,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0"><tr><td align="center" style="padding:24px 12px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:680px;background:#ffffff;border:1px solid #e7e5e4;border-radius:12px;">
<tr><td style="padding:28px 28px 24px;border-bottom:1px solid #e7e5e4;">
<p style="margin:0 0 12px;font-size:12px;line-height:18px;color:#78716c;">{{.Workspace}} · {{.Label}}</p>
<h1 style="margin:0 0 12px;font-size:23px;line-height:31px;font-weight:600;overflow-wrap:anywhere;">{{.Subject}}</h1>
<p style="margin:0;font-size:14px;line-height:22px;color:#57534e;">{{.Intro}}</p>
</td></tr>
<tr><td style="padding:24px 20px 8px;">
{{range .Messages}}<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 20px;"><tr><td align="{{.Align}}">
<table role="presentation" width="94%" cellpadding="0" cellspacing="0"><tr><td align="{{.Align}}" style="padding:0 2px 7px;">
<table role="presentation" cellpadding="0" cellspacing="0"><tr>
{{if eq .Align "left"}}<td style="padding-right:8px;">{{template "avatar" .}}</td>{{end}}
<td align="{{.Align}}" style="font-size:12px;line-height:19px;color:#78716c;">
<strong style="color:#292524;font-size:13px;">{{.Sender}}</strong>{{if .AI}} <span style="color:#78716c;">· AI assistant</span>{{end}}
</td>{{if eq .Align "right"}}<td style="padding-left:8px;">{{template "avatar" .}}</td>{{end}}
</tr></table>
</td></tr><tr><td dir="auto" style="padding:14px 16px;background:{{.Background}};border:1px solid #e7e5e4;border-radius:10px;font-size:14px;line-height:22px;overflow-wrap:anywhere;word-break:break-word;">{{.Body}}{{if .Timestamp}}<div dir="ltr" style="padding-top:8px;text-align:left;font-size:11px;line-height:16px;color:#78716c;">{{.Timestamp}}</div>{{end}}</td></tr></table>
</td></tr></table>{{else}}<p style="margin:0 8px 20px;color:#78716c;font-size:14px;">No public messages in this conversation.</p>{{end}}
</td></tr></table>
</td></tr></table></body></html>
{{define "avatar"}}<table role="presentation" cellpadding="0" cellspacing="0" width="28"><tr><td align="center" width="28" height="28" style="width:28px;height:28px;background:{{.Background}};border:1px solid #d6d3d1;border-radius:50%;font-size:10px;line-height:28px;font-weight:600;color:#57534e;">{{if .AvatarURL}}<img src="{{.AvatarURL}}" alt="{{.AvatarText}}" width="28" height="28" style="display:block;width:28px;height:28px;border-radius:50%;object-fit:cover;border:0;">{{else}}{{.AvatarText}}{{end}}</td></tr></table>{{end}}`))

type supportTranscriptMessage struct {
	Sender, Timestamp, Align, Background string
	AvatarURL, AvatarText                string
	AI                                   bool
	Body                                 template.HTML
}

func supportTranscriptAvatarText(sender string, isAI bool) string {
	if isAI {
		return "AI"
	}
	parts := strings.Fields(sender)
	if len(parts) == 0 {
		return "?"
	}
	initials := string([]rune(parts[0])[0])
	if len(parts) > 1 {
		initials += string([]rune(parts[len(parts)-1])[0])
	}
	return strings.ToUpper(initials)
}

func supportTranscriptSubject(workspaceName string, conversation *model.SupportConversation) string {
	subject := strings.Join(strings.Fields(conversation.Subject), " ")
	if subject == "" {
		subject = "Conversation"
		if conversation.DisplayID > 0 {
			subject += fmt.Sprintf(" #%d", conversation.DisplayID)
		}
	}
	return "Transcript: " + truncateWebhookText(subject, 160) + " — " + supportTranscriptWorkspaceName(workspaceName)
}

func supportTranscriptWorkspaceName(name string) string {
	if name = strings.Join(strings.Fields(name), " "); name != "" {
		return name
	}
	return "Support"
}

func supportTranscriptSenderName(workspaceName string, conversation *model.SupportConversation, msg model.SupportMessage) string {
	if name := strings.TrimSpace(derefString(msg.SenderDisplayName)); name != "" {
		return name
	}
	if msg.SenderType == "customer" {
		if name := strings.TrimSpace(derefString(conversation.CustomerName)); name != "" {
			return name
		}
		return "Customer"
	}
	if msg.SenderType == "ai" {
		return "AI assistant"
	}
	return supportTranscriptWorkspaceName(workspaceName)
}

// A transcript contains the discussion, never operational events or private context.
// Progress acknowledgments can be stored as public replies, so their durable
// metadata must be checked before the widget clears old progress indicators.
func supportTranscriptIncludesMessage(msg model.SupportMessage) bool {
	if msg.IsInternal || msg.DeletedAt.Valid || msg.SystemEventType != nil || (msg.MessageType != "" && msg.MessageType != "reply") {
		return false
	}
	switch msg.SenderType {
	case "customer", "user", "agent", "ai":
	default:
		return false
	}
	var metadata AIMessageMetadata
	if json.Unmarshal([]byte(msg.Metadata), &metadata) == nil && metadata.AIProgressState == supportAIProgressChecking {
		return false
	}
	return true
}

func renderSupportTranscriptBodies(workspaceName, sentByName string, conversation *model.SupportConversation, messages []model.SupportMessage) (string, string) {
	workspaceName = supportTranscriptWorkspaceName(workspaceName)
	subject := strings.TrimSpace(conversation.Subject)
	if subject == "" {
		subject = "Conversation transcript"
	}
	label := "Conversation transcript"
	if conversation.DisplayID > 0 {
		label = fmt.Sprintf("Conversation #%d", conversation.DisplayID)
	}
	intro := "Here is a copy of the conversation with " + workspaceName + "."
	if sentByName = strings.Join(strings.Fields(sentByName), " "); sentByName != "" {
		intro = sentByName + " from " + workspaceName + " sent you a copy of this conversation."
	}
	var text strings.Builder
	fmt.Fprintf(&text, "%s\n%s · %s\n\n%s\n\n", subject, workspaceName, label, intro)
	var rendered []supportTranscriptMessage
	for _, msg := range messages {
		if !supportTranscriptIncludesMessage(msg) {
			continue
		}
		content := supportTranscriptMarkdown(msg)
		if content == "" && len(msg.Attachments) == 0 {
			continue
		}
		body := renderSupportTranscriptMessageHTML(content)
		if strings.TrimSpace(body) == "" && len(msg.Attachments) == 0 {
			continue
		}
		sender := supportTranscriptSenderName(workspaceName, conversation, msg)
		timestamp := ""
		if !msg.CreatedAt.IsZero() {
			timestamp = formatSupportTranscriptTimestamp(msg.CreatedAt)
		}
		text.WriteString(sender)
		if msg.SenderType == "ai" && sender != "AI assistant" {
			text.WriteString(" · AI assistant")
		}
		if timestamp != "" {
			text.WriteString(" — " + timestamp)
		}
		text.WriteString("\n" + supportTranscriptPlainText(body) + "\n")
		for _, attachment := range msg.Attachments {
			name := strings.TrimSpace(attachment.FileName)
			if name == "" {
				name = "Attachment"
			}
			link := supportTranscriptAttachmentURL(attachment.URL)
			text.WriteString("[Attachment: " + name + "]")
			body += `<p>Attachment: `
			if link != "" {
				text.WriteString(" " + link)
				body += `<a href="` + html.EscapeString(link) + `">` + html.EscapeString(name) + `</a>`
			} else {
				body += html.EscapeString(name)
			}
			text.WriteString("\n")
			body += `</p>`
		}
		text.WriteString("\n")
		align, background := "left", "#f5f5f4"
		if msg.SenderType != "customer" {
			align, background = "right", "#eff6ff"
		}
		rendered = append(rendered, supportTranscriptMessage{Sender: sender, Timestamp: timestamp, Align: align, Background: background,
			AvatarURL: supportTranscriptAttachmentURL(derefString(msg.SenderAvatarURL)), AvatarText: supportTranscriptAvatarText(sender, msg.SenderType == "ai" || msg.SenderType == "agent"),
			AI: msg.SenderType == "ai" && sender != "AI assistant", Body: template.HTML(styleSupportTranscriptContent(body))})
	}
	if len(rendered) == 0 {
		text.WriteString("No public messages in this conversation.\n")
	}
	var htmlBody bytes.Buffer
	err := supportTranscriptTemplate.Execute(&htmlBody, struct {
		Workspace, Subject, Label, Intro string
		Messages                         []supportTranscriptMessage
	}{Workspace: workspaceName, Subject: subject, Label: label, Intro: intro, Messages: rendered})
	if err != nil {
		slog.Error("render support transcript", "error", err)
		return "<p>" + strings.ReplaceAll(html.EscapeString(text.String()), "\n", "<br>") + "</p>", text.String()
	}
	return htmlBody.String(), text.String()
}

func supportTranscriptMarkdown(msg model.SupportMessage) string {
	content := strings.TrimSpace(msg.Content)
	switch {
	case strings.TrimSpace(msg.EmailVisibleText) != "":
		content = msg.EmailVisibleText
	case strings.TrimSpace(msg.HTMLBody) != "":
		content = inboundhtml.Convert(msg.HTMLBody, content)
	case strings.TrimSpace(msg.StrippedText) != "":
		content = msg.StrippedText
	}
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "{") {
		content = tiptap.RichTextToMarkdown(content)
	}
	// Marketing email preheaders often contain long lines of invisible padding.
	// Remove only spacer-only lines; preserve joiners inside emoji and real text.
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.TrimFunc(line, func(r rune) bool { return unicode.IsSpace(r) || unicode.In(r, unicode.Cf) }) == "" {
			lines[i] = ""
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func renderSupportTranscriptMessageHTML(content string) string {
	var body bytes.Buffer
	if err := supportTranscriptMarkdownRenderer.Convert([]byte(content), &body); err != nil {
		return "<p>" + strings.ReplaceAll(html.EscapeString(content), "\n", "<br>") + "</p>"
	}
	return supportTranscriptPolicy.Sanitize(body.String())
}

func supportTranscriptAttachmentURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return ""
	}
	return parsed.String()
}

func supportTranscriptPlainText(body string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return tiptap.StripHTML(body)
	}
	doc.Find("a").Each(func(_ int, link *goquery.Selection) {
		if href, ok := link.Attr("href"); ok && href != link.Text() {
			link.AppendNodes(&nethtml.Node{Type: nethtml.TextNode, Data: " (" + href + ")"})
		}
	})
	doc.Find("img").Each(func(_ int, img *goquery.Selection) {
		label := img.AttrOr("alt", "Image")
		img.ReplaceWithHtml(html.EscapeString("[Image: " + label + "]"))
	})
	doc.Find("table").Each(func(_ int, table *goquery.Selection) {
		var rows []string
		table.Find("tr").Each(func(_ int, row *goquery.Selection) {
			var cells []string
			row.ChildrenFiltered("th,td").Each(func(_ int, cell *goquery.Selection) {
				cells = append(cells, strings.Join(strings.Fields(cell.Text()), " "))
			})
			rows = append(rows, strings.Join(cells, " | "))
		})
		table.SetText(strings.Join(rows, "\n"))
	})
	doc.Find("br").ReplaceWithHtml("\n")
	doc.Find("p,div,ul,ol,li,pre,h1,h2,h3,h4,h5,h6,blockquote,table,tr").Each(func(_ int, block *goquery.Selection) {
		block.AppendNodes(&nethtml.Node{Type: nethtml.TextNode, Data: "\n"})
	})
	text := strings.TrimSpace(doc.Find("body").Text())
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return text
}

// Inline styles survive email clients that strip style blocks. Normalize rich
// content instead of embedding inbound email documents and their global CSS.
func styleSupportTranscriptContent(body string) string {
	safe := supportTranscriptPolicy.Sanitize(body)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(safe))
	if err != nil {
		return safe
	}
	for selector, style := range map[string]string{
		"p":                 "margin:0 0 12px;",
		"h1,h2,h3,h4,h5,h6": "margin:16px 0 10px;font-size:17px;line-height:24px;",
		"ul,ol":             "margin:8px 0 12px;padding-left:24px;",
		"li":                "margin:4px 0;",
		"a":                 "color:#2563eb;text-decoration:underline;overflow-wrap:anywhere;",
		"table":             "width:100%;border-collapse:collapse;margin:12px 0;font-size:13px;",
		"td":                "border:1px solid #d6d3d1;padding:8px;text-align:left;vertical-align:top;",
		"th":                "border:1px solid #d6d3d1;padding:8px;text-align:left;background:#e7e5e4;font-weight:600;",
		"pre":               "margin:12px 0;padding:12px;background:#e7e5e4;border-radius:6px;white-space:pre-wrap;overflow-wrap:anywhere;font-size:12px;",
		"code":              "font-family:Consolas,monospace;font-size:12px;",
		"blockquote":        "margin:12px 0;padding-left:12px;border-left:3px solid #d6d3d1;color:#57534e;",
		"img":               "max-width:100%;height:auto;border:0;",
	} {
		doc.Find(selector).SetAttr("style", style)
	}
	doc.Find("p:last-child").SetAttr("style", "margin:0;")
	styled, err := doc.Find("body").Html()
	if err != nil {
		return safe
	}
	return styled
}
