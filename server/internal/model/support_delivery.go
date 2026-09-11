package model

import "encoding/json"

// Explicit support reply delivery modes. Empty preserves legacy fallback behavior.
const (
	SupportDeliveryChatOnly     = "chat_only"
	SupportDeliveryChatAndEmail = "chat_and_email"
	SupportDeliveryEmailOnly    = "email_only"
)

// DeliveryMode reads the additive, persisted delivery intent for a message.
func (m SupportMessage) DeliveryMode() string {
	var metadata struct {
		Mode string `json:"delivery_mode"`
	}
	if json.Unmarshal([]byte(m.Metadata), &metadata) != nil {
		return ""
	}
	return metadata.Mode
}

// ExplicitEmailDelivery identifies email requested independently of chat presence.
func (m SupportMessage) ExplicitEmailDelivery() bool {
	mode := m.DeliveryMode()
	return mode == SupportDeliveryEmailOnly || mode == SupportDeliveryChatAndEmail
}

// WidgetVisible excludes internal notes and explicitly email-only replies.
func (m SupportMessage) WidgetVisible() bool {
	return !m.IsInternal && m.DeliveryMode() != SupportDeliveryEmailOnly
}
