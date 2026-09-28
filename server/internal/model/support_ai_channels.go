package model

import (
	"encoding/json"
	"strings"
)

// SupportAIChannelEnabled applies the saved reply-channel choice. Older settings
// default to chat so introducing the selector never silently enables email AI.
func SupportAIChannelEnabled(settings SupportInboxSettings, channel string) bool {
	selection := settings.AIReplyChannels
	if selection == "" {
		selection = "chat"
	}
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel == "" || channel == "widget" {
		channel = "chat"
	}
	if channel == "portal" {
		return settings.PortalEnabled && (settings.PortalAIMode == "ai_first" || settings.PortalAIMode == "internal_note")
	}
	if channel != "chat" && channel != "email" {
		return false
	}
	return selection == "both" || selection == channel
}

// SupportAIReplyAllowed checks the incoming medium, falling back to the
// conversation channel for scheduled work. Email continuations count as email.
func SupportAIReplyAllowed(settings SupportInboxSettings, conversation *SupportConversation, message *SupportMessage) bool {
	channel := SupportAIReplyChannel(conversation, message)
	if channel == "portal" && (conversation == nil || !conversation.PortalVisible) {
		return false
	}
	if message != nil {
		if conversation != nil && conversation.AIResumedAt != nil && !message.CreatedAt.After(*conversation.AIResumedAt) {
			return false
		}
		if message.SenderType != "customer" || message.IsInternal || IsSupportEmailNotice(message) {
			return false
		}

		if strings.EqualFold(channel, "email") && SupportEmailSuppressesAI(message) {
			return false
		}
	}
	return SupportAIChannelEnabled(settings, channel)
}

// SupportEmailSuppressesAI rejects machine mail, spam, and common
// standalone absence notices, including historical emails without saved headers.
func SupportEmailSuppressesAI(message *SupportMessage) bool {
	if message == nil {
		return false
	}
	var metadata struct {
		Automatic         bool    `json:"email_auto_reply"`
		SuppressionReason string  `json:"email_ai_suppression_reason"`
		SpamStatus        string  `json:"postmark_spam_status"`
		SpamScore         float64 `json:"postmark_spam_score"`
	}
	_ = json.Unmarshal([]byte(message.Metadata), &metadata)
	if metadata.Automatic || metadata.SuppressionReason != "" || strings.HasPrefix(strings.ToLower(strings.TrimSpace(metadata.SpamStatus)), "yes") || metadata.SpamScore >= 5 {
		return true
	}
	body := strings.ToLower(strings.Join(strings.Fields(message.Content), " "))
	// Restrict the fallback to absence notices; normal questions containing the
	// words "out of office" must still be answerable.
	if strings.Contains(body, "?") {
		return false
	}
	return strings.Contains(body, "i am currently away from the office") ||
		strings.Contains(body, "i am currently out of the office") ||
		strings.Contains(body, "i'm currently out of the office")
}

// SupportAIReplyChannel identifies the medium of the customer message.
func SupportAIReplyChannel(conversation *SupportConversation, message *SupportMessage) string {
	channel := "chat"
	if conversation != nil {
		channel = conversation.Channel
		if strings.EqualFold(conversation.Source, "email") {
			channel = "email"
		}
	}
	if message != nil && message.ViaChannel != nil && strings.TrimSpace(*message.ViaChannel) != "" && !strings.EqualFold(channel, "email") {
		channel = *message.ViaChannel
	}
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel == "" || channel == "widget" {
		return "chat"
	}
	return channel
}

// SupportAIConversationBlocked is rechecked before saving a delayed reply.
func SupportAIConversationBlocked(c *SupportConversation) bool {
	if c == nil {
		return true
	}
	nonempty := func(s *string) bool { return s != nil && strings.TrimSpace(*s) != "" }
	return c.AnonymizedAt != nil || (c.HumanTakeover != nil && *c.HumanTakeover) || c.CustomerRequestedHumanAt != nil || nonempty(c.AssignedUserID) || nonempty(c.OpenedByUserID) || (c.AIState != nil && *c.AIState == "escalated") || (c.Status == "resolved" && (c.AIState == nil || *c.AIState != "resolved")) || c.Status == "spam"
}
