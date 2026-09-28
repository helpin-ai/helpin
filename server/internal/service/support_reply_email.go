package service

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	emailtpl "github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type supportReplyEmailEntry struct {
	Author    string
	Role      string
	Content   string
	CreatedAt time.Time
}

// History is anchored to the triggering message so later replies never leak
// into a delayed email. Only visible email projections are safe to excerpt.
func (s *NotificationService) loadSupportReplyEmailHistory(ctx context.Context, event model.NotificationEventInput) []supportReplyEmailEntry {
	if s.supportMessageRepo == nil {
		return nil
	}
	messageID, _ := event.Metadata["support_message_id"].(string)
	if messageID == "" {
		return nil
	}
	current, err := s.supportMessageRepo.GetByID(ctx, messageID)
	if err != nil || current == nil || current.WorkspaceID != event.WorkspaceID || current.ConversationID != event.EntityID {
		return nil
	}
	messages, _, err := s.supportMessageRepo.ListConversationPageBefore(ctx, event.WorkspaceID, event.EntityID, true, 30, &current.CreatedAt, current.ID)
	if err != nil {
		return nil
	}
	if s.supportEmailLogRepo != nil {
		ids := make([]string, 0, len(messages))
		for _, message := range messages {
			if message.ViaChannel != nil && *message.ViaChannel == "email" {
				ids = append(ids, message.ID)
			}
		}
		if len(ids) > 0 {
			if logs, err := s.supportEmailLogRepo.ListByMessageIDs(ctx, event.WorkspaceID, ids); err == nil {
				visible := make(map[string]string, len(ids))
				for _, log := range logs {
					for _, id := range log.MessageIDs {
						if _, exists := visible[id]; !exists {
							visible[id] = supportReplyHistoryEmailText(log)
						}
					}
				}
				for i := range messages {
					if messages[i].ViaChannel != nil && *messages[i].ViaChannel == "email" {
						messages[i].EmailVisibleText = visible[messages[i].ID]
					}
				}
			}
		}
	}
	return supportReplyHistoryEntries(messages)
}

func supportReplyHistoryEmailText(log model.SupportEmailLog) string {
	var payload model.PostmarkInboundPayload
	if json.Unmarshal([]byte(log.RawBody), &payload) == nil {
		return supportEmailNotificationPreview(payload, inboundPayloadProjection(payload))
	}
	if log.EmailHasQuotedContent && log.EmailProjectionConfidence != "none" {
		return strings.TrimSpace(log.EmailVisibleText)
	}
	return ""
}

func supportReplyHistoryEntries(messages []model.SupportMessage) []supportReplyEmailEntry {
	entries := make([]supportReplyEmailEntry, 0, 3)
	for _, message := range messages {
		if message.MessageType != "reply" {
			continue
		}
		role := "Team reply"
		if message.IsInternal {
			role = "Private note"
		} else if message.SenderType == "customer" {
			role = "Customer"
		}
		content := strings.TrimSpace(message.Content)
		if message.ViaChannel != nil && *message.ViaChannel == "email" {
			content = strings.TrimSpace(message.EmailVisibleText)
		}
		if content == "" {
			continue
		}
		content = truncate(content, 500)
		author := strings.TrimSpace(derefString(message.SenderDisplayName))
		if author == "" {
			if role == "Customer" {
				author = "Customer"
			} else {
				author = "Team"
			}
		}
		entries = append(entries, supportReplyEmailEntry{Author: author, Role: role, Content: content, CreatedAt: message.CreatedAt})
		if len(entries) > 3 {
			entries = entries[1:]
		}
	}
	return entries
}

func (s *NotificationService) renderSupportReplyEmail(_ context.Context, event model.NotificationEventInput, workspaceName, workspaceSlug string, history []supportReplyEmailEntry) (string, string, string) {
	actor := "Customer"
	if name, ok := event.ActorSnapshot["name"].(string); ok && strings.TrimSpace(name) != "" {
		actor = strings.TrimSpace(name)
	}
	title, _ := event.EntitySnapshot["title"].(string)
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Support conversation"
	}
	number := ""
	switch value := event.EntitySnapshot["display_id"].(type) {
	case int:
		if value > 0 {
			number = strconv.Itoa(value)
		}
	case int64:
		if value > 0 {
			number = strconv.FormatInt(value, 10)
		}
	case float64:
		if value > 0 {
			number = strconv.FormatInt(int64(value), 10)
		}
	}
	conversationLabel := "conversation"
	if number != "" {
		conversationLabel += " #" + number
	}
	subject := fmt.Sprintf("%s replied to %s [%s]", actor, conversationLabel, workspaceName)
	latest, _ := event.Metadata["support_email_body"].(string)
	if strings.TrimSpace(latest) == "" {
		latest = event.Body
	}
	latest = strings.TrimSpace(latest)
	if latest == "" {
		latest = "Open the conversation to read the latest reply."
	}
	url := buildEntityURL(s.appBaseURL, workspaceSlug, event.EntityType, event.EntityID)
	var htmlHistory, textHistory strings.Builder
	if len(history) > 0 {
		htmlHistory.WriteString(`<tr><td style="padding:24px 0 10px;color:#777;font-size:11px;font-weight:700;text-transform:uppercase;letter-spacing:.08em;">Recent conversation</td></tr>`)
		textHistory.WriteString("\nRecent conversation:\n")
	}
	for _, entry := range history {
		border, background := "#97b6da", "#f7faff"
		switch entry.Role {
		case "Private note":
			border, background = "#cfa963", "#fffaf0"
		case "Team reply":
			border, background = "#95a3a9", "#f8f9f9"
		}
		timeLabel := ""
		if !entry.CreatedAt.IsZero() {
			timeLabel = entry.CreatedAt.UTC().Format("Jan 2, 3:04 PM UTC")
		}
		htmlHistory.WriteString(fmt.Sprintf(`<tr><td style="padding:0 0 9px;"><table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="border:1px solid #e5e7eb;border-left:3px solid %s;border-radius:7px;background:%s;"><tr><td style="padding:13px 15px;"><div style="font-size:12px;line-height:1.4;"><strong>%s</strong> <span style="color:#666;">· %s</span> <span style="color:#888;">%s</span></div><div style="padding-top:7px;color:#4b4b4b;font-size:13px;line-height:1.55;white-space:pre-wrap;">%s</div></td></tr></table></td></tr>`, border, background, html.EscapeString(entry.Author), html.EscapeString(entry.Role), html.EscapeString(timeLabel), html.EscapeString(entry.Content)))
		textHistory.WriteString(fmt.Sprintf("%s · %s", entry.Author, entry.Role))
		if timeLabel != "" {
			textHistory.WriteString(" · " + timeLabel)
		}
		textHistory.WriteString("\n" + entry.Content + "\n\n")
	}
	label := "Support · " + conversationLabel
	htmlBody := fmt.Sprintf(`<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">%s</head><body style="margin:0;padding:0;background:#f5f5f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif;color:#252420;"><div style="display:none;max-height:0;overflow:hidden;">%s: %s</div><table role="presentation" width="100%%" cellspacing="0" cellpadding="0"><tr><td align="center" style="padding:32px 16px;"><table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="max-width:560px;background:#fff;border:1px solid #e7e6e3;border-radius:10px;overflow:hidden;">%s<tr><td style="padding:28px 32px 32px;"><table role="presentation" width="100%%" cellspacing="0" cellpadding="0"><tr><td style="color:#777;font-size:11px;font-weight:700;text-transform:uppercase;letter-spacing:.08em;padding-bottom:8px;">%s</td></tr><tr><td style="font-size:21px;font-weight:650;line-height:1.35;padding-bottom:22px;">%s</td></tr><tr><td style="border:1px solid #d9e5f2;border-left:3px solid #6d98c9;border-radius:8px;background:#f7faff;padding:18px 20px;"><div style="font-size:12px;"><strong>%s</strong> <span style="color:#315f91;">· Customer</span></div><div style="padding-top:10px;font-size:15px;line-height:1.6;white-space:pre-wrap;">%s</div></td></tr>%s%s</table></td></tr>%s</table></td></tr></table></body></html>`, emailtpl.BrandHeaderCSS(), html.EscapeString(actor), html.EscapeString(latest), emailtpl.NotificationEmailHeaderHTML(workspaceName, url), html.EscapeString(label), html.EscapeString(title), html.EscapeString(actor), html.EscapeString(latest), htmlHistory.String(), emailtpl.CTAButtonHTML(url), emailtpl.NotificationFooterHTML("New reply in "+workspaceName))
	textBody := fmt.Sprintf("%s\n%s\n\n%s · Customer\n%s\n", label, title, actor, latest) + textHistory.String()
	if url != "" {
		textBody += "\nView in Helpin: " + url
	}
	return subject, htmlBody, textBody
}
