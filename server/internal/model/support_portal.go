package model

import (
	"strings"
	"time"
)

const (
	SupportPortalAuditRequestCreated     = "request_created"
	SupportPortalAuditReplyCreated       = "reply_created"
	SupportPortalAuditAttachmentUploaded = "attachment_uploaded"
	SupportPortalAuditRequestReopened    = "request_reopened"
	SupportPortalAuditVisibilityChanged  = "visibility_changed"
	SupportPortalAuditConfirmationResent = "confirmation_resent"

	SupportPortalActorCustomer = "customer"
	SupportPortalActorUser     = "user"
	SupportPortalActorSystem   = "system"

	// PortalAccessAllowed and PortalAccessBlocked are CRM contact portal
	// decisions; a nil decision means none was made.
	PortalAccessAllowed = "allowed"
	PortalAccessBlocked = "blocked"

	// Portal access modes decide who may sign in to the customer portal.
	SupportPortalAccessModeApprovedContacts = "approved_contacts"
	SupportPortalAccessModeAnyVerifiedEmail = "any_verified_email"
)

// SupportPortalIdentity is the stable customer identity used to authorize
// portal requests in one workspace. Authentication providers can bind their
// subject to AuthSubject without changing a request's conversation ownership.
type SupportPortalIdentity struct {
	ID          string  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string  `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_support_portal_identity_email,priority:1;uniqueIndex:idx_support_portal_identity_subject,priority:1"`
	Email       string  `json:"email" gorm:"not null;uniqueIndex:idx_support_portal_identity_email,priority:2"`
	DisplayName *string `json:"display_name,omitempty"`
	AuthSubject *string `json:"-" gorm:"uniqueIndex:idx_support_portal_identity_subject,priority:2,where:auth_subject IS NOT NULL"`
	// CRMContactID is the approved contact this identity signed in as.
	CRMContactID *string   `json:"-" gorm:"type:uuid"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportPortalIdentity) TableName() string { return "support_portal_identities" }

// SupportPortalRequestReference maps an opaque, workspace-scoped public
// reference to the canonical support conversation. The reference is the only
// request identifier exposed to portal clients.
type SupportPortalRequestReference struct {
	ID               string `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_support_portal_request_reference,priority:1"`
	ConversationID   string `json:"-" gorm:"type:uuid;not null;uniqueIndex:idx_support_portal_request_conversation,priority:1"`
	PortalIdentityID string `json:"-" gorm:"type:uuid;not null;index"`
	Reference        string `json:"reference" gorm:"not null;uniqueIndex:idx_support_portal_request_reference,priority:2"`
	// CustomerLastReadAt is when the customer last opened the request.
	CustomerLastReadAt *time.Time `json:"-"`
	CreatedAt          time.Time  `json:"created_at" gorm:"autoCreateTime"`
}

func (SupportPortalRequestReference) TableName() string { return "support_portal_request_references" }

// SupportPortalAuditEvent is an immutable portal-specific audit ledger. It
// preserves who performed a customer-facing action independently of the
// operational support event stream.
type SupportPortalAuditEvent struct {
	ID               string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string    `json:"workspace_id" gorm:"type:uuid;not null;index:idx_support_portal_audit_request_time,priority:1"`
	ConversationID   string    `json:"conversation_id" gorm:"type:uuid;not null;index:idx_support_portal_audit_conversation_time,priority:1"`
	PortalIdentityID *string   `json:"portal_identity_id,omitempty" gorm:"type:uuid;index"`
	ActorType        string    `json:"actor_type" gorm:"not null"`
	ActorUserID      *string   `json:"actor_user_id,omitempty" gorm:"type:uuid"`
	EventType        string    `json:"event_type" gorm:"not null;index"`
	Metadata         string    `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	OccurredAt       time.Time `json:"occurred_at" gorm:"not null;index:idx_support_portal_audit_request_time,priority:2,sort:desc;index:idx_support_portal_audit_conversation_time,priority:2,sort:desc"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (SupportPortalAuditEvent) TableName() string { return "support_portal_audit_events" }

// SupportPortalRequest is a deliberately customer-safe projection of a
// support conversation. It has no internal ID, assignment, notes, or customer
// contact data.
type SupportPortalRequest struct {
	Reference           string     `json:"reference"`
	Subject             string     `json:"subject"`
	Status              string     `json:"status"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	LastActivityAt      *time.Time `json:"last_activity_at,omitempty"`
	ResolvedAt          *time.Time `json:"resolved_at,omitempty"`
	LastPublicMessageAt *time.Time `json:"-"`
	// Number is the short ticket number agents also see (display_id).
	Number int `json:"number" gorm:"column:display_id"`
	// LastMessagePreview is a plain-text excerpt of the latest public message.
	LastMessagePreview string `json:"last_message_preview,omitempty" gorm:"-"`
	// LastMessageFrom is "customer" or "support".
	LastMessageFrom string `json:"last_message_from,omitempty" gorm:"-"`
	// Unread is true when support replied after the customer last opened it.
	Unread bool `json:"unread" gorm:"-"`

	LastPublicSenderType     *string    `json:"-"`
	CustomerAwaitingResponse bool       `json:"-"`
	LastMessageContent       *string    `json:"-"`
	CustomerLastReadAt       *time.Time `json:"-"`
}

// PortalRequestStatus exposes the customer-facing lifecycle. Open requests
// awaiting a customer reply share the inbox's derived Waiting view.
func PortalRequestStatus(status, lastPublicSenderType string, customerAwaitingResponse bool) string {
	switch NormalizeSupportConversationStatus(status) {
	case SupportConversationStatusResolved:
		return SupportConversationStatusResolved
	case SupportConversationStatusWaitingOnCustomer:
		return SupportConversationStatusWaitingOnCustomer
	case SupportConversationStatusOpen:
		if lastPublicSenderType == "user" && !customerAwaitingResponse {
			return SupportConversationStatusWaitingOnCustomer
		}
	default:
	}
	return "active"
}

// PortalMessagePreview turns message text (often Markdown) into a short
// plain-text excerpt for the request list.
func PortalMessagePreview(content string) string {
	replacer := strings.NewReplacer("**", "", "__", "", "`", "", "#", "", "> ", "", "\r", " ", "\n", " ", "\t", " ")
	text := strings.Join(strings.Fields(replacer.Replace(content)), " ")
	for _, bullet := range []string{"- ", "* "} {
		text = strings.TrimPrefix(text, bullet)
	}
	const limit = 140
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return strings.TrimSpace(string(runes[:limit])) + "…"
}

// PortalSenderAvatar is a teammate's avatar as the app draws it: an uploaded
// image, or a generated one from its style, seed and background.
type PortalSenderAvatar struct {
	URL             *string `json:"url,omitempty" gorm:"column:avatar_url"`
	Style           *string `json:"style,omitempty" gorm:"column:avatar_style"`
	Seed            *string `json:"seed,omitempty" gorm:"column:avatar_seed"`
	BackgroundMode  *string `json:"background_mode,omitempty" gorm:"column:avatar_background_mode"`
	BackgroundColor *string `json:"background_color,omitempty" gorm:"column:avatar_background_color"`
}
