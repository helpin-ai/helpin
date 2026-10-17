package model

import "time"

// AdminInboundEmail is a receipt summary without message bodies or attachment bytes.
type AdminInboundEmail struct {
	ID                  string     `json:"id"`
	Status              string     `json:"status"`
	WorkspaceID         *string    `json:"workspace_id,omitempty"`
	WorkspaceName       string     `json:"workspace_name,omitempty"`
	ConversationID      *string    `json:"conversation_id,omitempty"`
	Subject             string     `json:"subject"`
	SenderEmail         string     `json:"sender_email"`
	SenderName          string     `json:"sender_name"`
	Recipient           string     `json:"recipient"`
	ReceivedAt          time.Time  `json:"received_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	AvailableAt         time.Time  `json:"available_at"`
	Attempts            int        `json:"attempts"`
	RequiresDestination bool       `json:"requires_destination"`
	RecoveryRequestedAt *time.Time `json:"recovery_requested_at,omitempty"`
	RecoveryActorName   string     `json:"recovery_actor_name,omitempty"`
}

// AdminInboundEmailList is the paginated operator queue.
type AdminInboundEmailList struct {
	Entries        []AdminInboundEmail `json:"entries"`
	Total          int64               `json:"total"`
	AttentionCount int64               `json:"attention_count"`
	Page           int                 `json:"page"`
	PerPage        int                 `json:"per_page"`
}

// AdminInboundAttachment contains display metadata and a receipt-local download ID.
type AdminInboundAttachment struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

// AdminInboundEmailDetail exposes only the email content needed for review.
type AdminInboundEmailDetail struct {
	AdminInboundEmail
	Body        string                    `json:"body"`
	ProviderID  string                    `json:"provider_id"`
	InReplyTo   string                    `json:"in_reply_to,omitempty"`
	LastError   string                    `json:"last_error,omitempty"`
	Attachments []AdminInboundAttachment  `json:"attachments"`
	Destination *AdminInboundConversation `json:"destination,omitempty"`
}

// AdminInboundConversation is a possible destination, never an automatic match.
type AdminInboundConversation struct {
	ID            string `json:"id"`
	DisplayID     int    `json:"display_id"`
	Subject       string `json:"subject"`
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	WorkspaceSlug string `json:"workspace_slug"`
	InboxName     string `json:"inbox_name"`
	CustomerName  string `json:"customer_name"`
	CustomerEmail string `json:"customer_email"`
	LastTeamReply string `json:"last_team_reply,omitempty"`
}

// RecoverInboundEmailRequest optionally confirms a destination; empty means retry routing.
type RecoverInboundEmailRequest struct {
	ConversationID string `json:"conversation_id,omitempty"`
}
