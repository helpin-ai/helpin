package model

import (
	"encoding/json"
	"time"
)

// WidgetMessage contains only fields used to render a visitor's public message.
// Never embed SupportMessage here: new staff fields must remain private by default.
type WidgetMessage struct {
	ID                        string             `json:"id"`
	ClientMessageID           string             `json:"client_message_id,omitempty"`
	ConversationID            string             `json:"conversation_id"`
	Content                   string             `json:"content"`
	SenderType                string             `json:"sender_type"`
	MessageType               string             `json:"message_type,omitempty"`
	SystemEventType           *string            `json:"system_event_type,omitempty"`
	SenderName                *string            `json:"sender_name"`
	SenderAvatar              *string            `json:"sender_avatar"`
	Metadata                  string             `json:"metadata,omitempty"`
	ViaChannel                string             `json:"via_channel,omitempty"`
	Attachments               []WidgetAttachment `json:"attachments,omitempty"`
	EmailVisibleText          string             `json:"email_visible_text,omitempty"`
	EmailQuotedText           string             `json:"email_quoted_text,omitempty"`
	EmailHasQuotedContent     *bool              `json:"email_has_quoted_content,omitempty"`
	EmailProjectionConfidence string             `json:"email_projection_confidence,omitempty"`
	EmailProjectionVersion    int                `json:"email_projection_version,omitempty"`
	CreatedAt                 string             `json:"created_at"`
}

// WidgetAttachment exposes a download URL without internal storage bookkeeping.
type WidgetAttachment struct {
	ID       string `json:"id"`
	FileName string `json:"file_name"`
	FileType string `json:"file_type"`
	FileSize int64  `json:"file_size"`
	URL      string `json:"url"`
}

// WidgetConversation is the visitor's conversation summary, not the staff record.
type WidgetConversation struct {
	ID                  string    `json:"id"`
	Subject             string    `json:"subject"`
	Status              string    `json:"status"`
	FlowState           *string   `json:"flow_state,omitempty"`
	AIState             *string   `json:"ai_state,omitempty"`
	HandoffState        *string   `json:"handoff_state,omitempty"`
	LastMessage         *string   `json:"last_message,omitempty"`
	UnreadCount         int       `json:"unread_count"`
	OpenedByUserID      *string   `json:"opened_by_user_id,omitempty"`
	OpenedByDisplayName *string   `json:"opened_by_display_name,omitempty"`
	OpenedByAvatarURL   *string   `json:"opened_by_avatar_url,omitempty"`
	OpenedByStatus      *string   `json:"opened_by_status,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// PublicWidgetMessage converts a public message; nil means it must not be delivered.
func PublicWidgetMessage(m *SupportMessage) *WidgetMessage {
	if m == nil || !m.WidgetVisible() {
		return nil
	}
	result := &WidgetMessage{
		ID: m.ID, ClientMessageID: m.ClientMessageID, ConversationID: m.ConversationID,
		Content: m.Content, SenderType: m.SenderType, MessageType: m.MessageType,
		SystemEventType: m.SystemEventType, SenderName: m.SenderDisplayName, SenderAvatar: m.SenderAvatarURL,
		Metadata: PublicWidgetMetadata(m.Metadata), EmailVisibleText: m.EmailVisibleText,
		EmailQuotedText: m.EmailQuotedText, EmailHasQuotedContent: m.EmailHasQuotedContent,
		EmailProjectionConfidence: m.EmailProjectionConfidence, EmailProjectionVersion: m.EmailProjectionVersion,
		CreatedAt: m.CreatedAt.Format(time.RFC3339),
	}
	if m.ViaChannel != nil {
		result.ViaChannel = *m.ViaChannel
	}
	// Older AI senders used agent + ai_agent_id. Preserve the public role without the ID.
	var role struct {
		AIAgentID string `json:"ai_agent_id"`
	}
	if json.Unmarshal([]byte(m.Metadata), &role) == nil && role.AIAgentID != "" && m.SenderType != "customer" {
		result.SenderType = "ai"
	}
	for _, a := range m.Attachments {
		result.Attachments = append(result.Attachments, WidgetAttachment{ID: a.ID, FileName: a.FileName, FileType: a.FileType, FileSize: a.FileSize, URL: a.URL})
	}
	return result
}

// PublicWidgetMessages filters and projects history using the same contract as live replies.
func PublicWidgetMessages(messages []SupportMessage) []WidgetMessage {
	result := make([]WidgetMessage, 0, len(messages))
	for i := range messages {
		if message := PublicWidgetMessage(&messages[i]); message != nil {
			result = append(result, *message)
		}
	}
	return result
}

// PublicWidgetConversations excludes internal routing, CRM and AI bookkeeping.
func PublicWidgetConversations(conversations []SupportConversation) []WidgetConversation {
	result := make([]WidgetConversation, 0, len(conversations))
	for _, c := range conversations {
		result = append(result, WidgetConversation{
			ID: c.ID, Subject: c.Subject, Status: c.Status, FlowState: c.FlowState, AIState: c.AIState, HandoffState: c.HandoffState,
			LastMessage: c.LastMessage, UnreadCount: c.UnreadCount, OpenedByUserID: c.OpenedByUserID,
			OpenedByDisplayName: c.OpenedByDisplayName, OpenedByAvatarURL: c.OpenedByAvatarURL, OpenedByStatus: c.OpenedByStatus,
			CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
		})
	}
	return result
}

// PublicWidgetMetadata allowlists visitor features, including nested source/preview fields.
// Unknown keys, AI diagnostics and link-security assessments are private by default.
func PublicWidgetMetadata(raw string) string {
	var metadata struct {
		VisitorFeedback  *SupportAnswerFeedback `json:"visitor_feedback,omitempty"`
		DelayedTeamReply bool                   `json:"delayed_team_reply,omitempty"`
		CaptureEmail     bool                   `json:"capture_email,omitempty"`
		AIReplyKind      string                 `json:"ai_reply_kind,omitempty"`
		AISources        []struct {
			DocID    string `json:"docId,omitempty"`
			Title    string `json:"title"`
			URL      string `json:"url,omitempty"`
			Language string `json:"language,omitempty"`
		} `json:"ai_sources,omitempty"`
		LinkPreviews []SupportLinkPreview `json:"link_previews,omitempty"`
	}
	if json.Unmarshal([]byte(raw), &metadata) != nil {
		return "{}"
	}
	metadata.VisitorFeedback = (SupportMessage{Metadata: raw}).VisitorFeedback()
	switch metadata.AIReplyKind {
	case "answer", "clarify", "conversational", "confirmation", "greeting":
	default:
		metadata.AIReplyKind = ""
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}
