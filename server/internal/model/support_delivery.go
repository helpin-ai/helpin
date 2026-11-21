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

// CustomerChatVisible is shared by widget and portal history. Empty transport
// preserves the visibility of messages created before explicit routing.
func (m SupportMessage) CustomerChatVisible() bool {
	var intent struct {
		Mode      string `json:"delivery_mode"`
		Transport string `json:"reply_transport"`
	}
	if err := json.Unmarshal([]byte(m.Metadata), &intent); err != nil {
		return !m.IsInternal
	}
	return !m.IsInternal && intent.Mode != SupportDeliveryEmailOnly && (intent.Transport == "" || intent.Transport == "widget" || intent.Transport == "portal")
}

// WidgetVisible also excludes staff-only system events.
func (m SupportMessage) WidgetVisible() bool {
	if !m.CustomerChatVisible() {
		return false
	}
	// Internal routing events stay private even if a producer forgets is_internal.
	if m.SystemEventType != nil {
		switch *m.SystemEventType {
		case SystemEventTeammateJoined, SystemEventDelayedTeamReply:
		default:
			return false
		}
	}
	return true
}
