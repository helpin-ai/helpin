package model

import "time"

const (
	SupportPortalAuditRequestCreated     = "request_created"
	SupportPortalAuditReplyCreated       = "reply_created"
	SupportPortalAuditAttachmentUploaded = "attachment_uploaded"
	SupportPortalAuditRequestReopened    = "request_reopened"
	SupportPortalAuditVisibilityChanged  = "visibility_changed"

	SupportPortalActorCustomer = "customer"
	SupportPortalActorUser     = "user"
	SupportPortalActorSystem   = "system"
)

// SupportPortalIdentity is the stable customer identity used to authorize
// portal requests in one workspace. Authentication providers can bind their
// subject to AuthSubject without changing a request's conversation ownership.
type SupportPortalIdentity struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_support_portal_identity_email,priority:1;uniqueIndex:idx_support_portal_identity_subject,priority:1"`
	Email       string    `json:"email" gorm:"not null;uniqueIndex:idx_support_portal_identity_email,priority:2"`
	DisplayName *string   `json:"display_name,omitempty"`
	AuthSubject *string   `json:"-" gorm:"uniqueIndex:idx_support_portal_identity_subject,priority:2,where:auth_subject IS NOT NULL"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportPortalIdentity) TableName() string { return "support_portal_identities" }

// SupportPortalRequestReference maps an opaque, workspace-scoped public
// reference to the canonical support conversation. The reference is the only
// request identifier exposed to portal clients.
type SupportPortalRequestReference struct {
	ID               string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string    `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_support_portal_request_reference,priority:1"`
	ConversationID   string    `json:"-" gorm:"type:uuid;not null;uniqueIndex:idx_support_portal_request_conversation,priority:1"`
	PortalIdentityID string    `json:"-" gorm:"type:uuid;not null;index"`
	Reference        string    `json:"reference" gorm:"not null;uniqueIndex:idx_support_portal_request_reference,priority:2"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`
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
	ListLastActivityAt  *time.Time `json:"-"`
	LastPublicMessageAt *time.Time `json:"-"`
}

// PortalRequestStatus exposes only the customer-facing lifecycle, never internal states.
func PortalRequestStatus(status string) string {
	switch NormalizeSupportConversationStatus(status) {
	case SupportConversationStatusResolved:
		return SupportConversationStatusResolved
	case SupportConversationStatusWaitingOnCustomer:
		return SupportConversationStatusWaitingOnCustomer
	default:
		return "active"
	}
}
