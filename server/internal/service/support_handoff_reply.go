package service

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// supportCustomerNoticeTranslator puts a message into the customer's language.
// It returns the text unchanged whenever it cannot translate it.
type supportCustomerNoticeTranslator interface {
	TranslateCustomerNotice(ctx context.Context, workspaceID, conversationID, text, teamLanguage string) string
}

// SetCustomerNoticeTranslator lets handoff messages reach customers in their language.
func (s *SupportAIService) SetCustomerNoticeTranslator(translator supportCustomerNoticeTranslator) {
	s.noticeTranslator = translator
}

// handoffReplyContent writes the message the customer gets when the AI hands
// the conversation to the team. Chat uses the message for the handoff state.
// Email goes by business hours only, because an email reply comes later
// whoever is online now, and reads as an email.
func (s *SupportAIService) handoffReplyContent(
	ctx context.Context,
	conv *model.SupportConversation,
	settings model.SupportInboxSettings,
	availability supportAvailabilitySnapshot,
	handoffState, channel, transitionReply string,
	briefs []SupportHandoffBrief,
	now time.Time,
) string {
	template := selectEscalationTemplate(settings, handoffState)
	if channel == "email" {
		template = selectEmailHandoffTemplate(settings, availability.IsWithinOfficeHours)
	}
	// Short phrase for the {reply_time} token. Do NOT use
	// availability.WidgetAvailability.ReplyTimeText — that is full-sentence copy
	// (and the offline branch sets it to the outside-hours message).
	replyTime := shortReplyTimePhrase(
		availability.WidgetAvailability.ReplyTimePreset,
		availability.WidgetAvailability.ReplyTimeMinutes,
	)
	var nextOpenText string
	if strings.Contains(template, "{next_open}") {
		nextOpenText = s.nextOpenForCustomer(ctx, conv, settings, now)
	}
	body := renderEscalationMessage(template, replyTime, nextOpenText)
	if reply := strings.TrimSpace(transitionReply); reply != "" {
		body = stripConversationPII(reply, conv.CustomerEmail, conv.CustomerPhone)
	}
	// A fact-checked account update goes first; the template keeps the
	// reply-time and after-hours wording.
	var update string
	if len(briefs) > 0 {
		if checked := strings.TrimSpace(briefs[0].CustomerUpdate); checked != "" {
			update = stripConversationPII(checked, conv.CustomerEmail, conv.CustomerPhone)
		}
	}
	if channel == "email" {
		return composeHandoffEmail(derefString(conv.CustomerName), update, body)
	}
	if update != "" {
		return update + "\n\n" + body
	}
	return body
}

// composeHandoffEmail lays a handoff message out as an email: a greeting by
// first name when it is known, the message, and a sign-off. The email footer
// adds the sender's name after it.
func composeHandoffEmail(customerName, update, body string) string {
	greeting := "Hi,"
	if first := supportSystemFirstName(customerName); first != "" && !strings.Contains(first, "@") {
		greeting = "Hi " + first + ","
	}
	parts := []string{greeting}
	if update != "" {
		parts = append(parts, update)
	}
	parts = append(parts, strings.TrimSpace(body), "Best regards,")
	return strings.Join(parts, "\n\n")
}

// translateHandoffReply puts the handoff message into the customer's language.
// The team writes these messages in its own language; the message goes out as
// written when the customer's language is unknown or the same, or when
// translation fails, so a handoff is never held back by it.
func (s *SupportAIService) translateHandoffReply(ctx context.Context, conv *model.SupportConversation, settings model.SupportInboxSettings, content string) string {
	if s.noticeTranslator == nil || conv == nil {
		return content
	}
	translateCtx, cancel := context.WithTimeout(ctx, supportHandoffTranslateTimeout)
	defer cancel()
	translated := s.noticeTranslator.TranslateCustomerNotice(translateCtx, conv.WorkspaceID, conv.ID, content, settings.DefaultAgentLanguage)
	if strings.TrimSpace(translated) == "" {
		slog.WarnContext(ctx, "handoff message translation came back empty; sending the original", "workspace_id", conv.WorkspaceID, "conversation_id", conv.ID)
		return content
	}
	return translated
}

// supportHandoffTranslateTimeout bounds how long a handoff waits for translation.
const supportHandoffTranslateTimeout = 20 * time.Second
