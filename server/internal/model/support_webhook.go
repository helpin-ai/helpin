package model

import (
	"encoding/json"
	"time"
)

// Webhook event types a workspace can subscribe to.
const (
	WebhookEventConversationCreated         = "conversation.created"
	WebhookEventConversationStatusChanged   = "conversation.status_changed"
	WebhookEventConversationAssigned        = "conversation.assigned"
	WebhookEventConversationPriorityChanged = "conversation.priority_changed"
	WebhookEventConversationSnoozed         = "conversation.snoozed"
	WebhookEventMessageCreated              = "message.created"
	WebhookEventSLABreached                 = "sla.breached"
	WebhookEventCSATSubmitted               = "csat.submitted"
	// WebhookEventPing is sent only by "Send test event".
	WebhookEventPing = "ping"
)

// WebhookEventTypes lists the subscribable events in display order.
var WebhookEventTypes = []string{
	WebhookEventConversationCreated, WebhookEventConversationStatusChanged, WebhookEventConversationAssigned,
	WebhookEventConversationPriorityChanged, WebhookEventConversationSnoozed, WebhookEventMessageCreated,
	WebhookEventSLABreached, WebhookEventCSATSubmitted,
}

// IsWebhookEventType reports whether event is subscribable.
func IsWebhookEventType(event string) bool {
	for _, known := range WebhookEventTypes {
		if known == event {
			return true
		}
	}
	return false
}

// Webhook delivery states.
const (
	WebhookDeliveryPending   = "pending"
	WebhookDeliverySucceeded = "succeeded"
	WebhookDeliveryFailed    = "failed"
)

// WebhookEndpointLimit bounds endpoints per workspace.
const WebhookEndpointLimit = 10

// WebhookEndpoint receives signed support events over HTTPS.
type WebhookEndpoint struct {
	ID          string `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string `json:"workspace_id" gorm:"type:uuid;not null;index"`
	URL         string `json:"url" gorm:"not null"`
	Description string `json:"description" gorm:"not null;default:''"`
	// Events lists subscribed event types; empty means every event.
	Events DocsStringArray `json:"events" gorm:"type:text[];not null;default:'{}'"`
	// MailboxIDs limits events to these inboxes ("shared" for the main
	// inbox); empty means every inbox.
	MailboxIDs DocsStringArray `json:"mailbox_ids" gorm:"type:text[];not null;default:'{}'"`
	// EncryptedSecret is the signing secret, encrypted at rest.
	EncryptedSecret string     `json:"-" gorm:"not null"`
	SecretHint      string     `json:"secret_hint" gorm:"not null;default:''"`
	Enabled         bool       `json:"enabled" gorm:"not null"`
	DisabledReason  string     `json:"disabled_reason" gorm:"not null;default:''"`
	FailureCount    int        `json:"failure_count" gorm:"type:integer;not null;default:0"`
	LastSuccessAt   *time.Time `json:"last_success_at,omitempty" gorm:"type:timestamptz"`
	LastFailureAt   *time.Time `json:"last_failure_at,omitempty" gorm:"type:timestamptz"`
	CreatedByID     *string    `json:"created_by_id,omitempty" gorm:"type:uuid"`
	CreatedAt       time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (WebhookEndpoint) TableName() string { return "webhook_endpoints" }

// WebhookEvent is an outbox row written by database triggers or the server
// when something subscribable happens; the dispatcher turns it into one
// delivery per subscribed endpoint.
type WebhookEvent struct {
	ID             string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string          `json:"workspace_id" gorm:"type:uuid;not null"`
	EventType      string          `json:"event_type" gorm:"not null"`
	ConversationID *string         `json:"conversation_id,omitempty" gorm:"type:uuid"`
	MessageID      *string         `json:"message_id,omitempty" gorm:"type:uuid"`
	Data           json.RawMessage `json:"data" gorm:"type:jsonb;not null;default:'{}'"`
	// AvailableAt delays events, such as a reply inside its undo window.
	AvailableAt time.Time `json:"available_at" gorm:"type:timestamptz;not null;index"`
	CreatedAt   time.Time `json:"created_at" gorm:"type:timestamptz;not null"`
}

func (WebhookEvent) TableName() string { return "webhook_events" }

// WebhookDelivery is one event sent, or to be sent, to one endpoint.
type WebhookDelivery struct {
	ID             string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string          `json:"workspace_id" gorm:"type:uuid;not null"`
	EndpointID     string          `json:"endpoint_id" gorm:"type:uuid;not null;index:idx_webhook_deliveries_endpoint,priority:1"`
	EventID        string          `json:"event_id" gorm:"type:uuid;not null"`
	EventType      string          `json:"event_type" gorm:"not null"`
	Payload        json.RawMessage `json:"payload" gorm:"type:jsonb;not null"`
	Status         string          `json:"status" gorm:"not null;default:'pending';index:idx_webhook_deliveries_due,priority:1"`
	Attempts       int             `json:"attempts" gorm:"type:integer;not null;default:0"`
	NextAttemptAt  time.Time       `json:"next_attempt_at" gorm:"type:timestamptz;not null;index:idx_webhook_deliveries_due,priority:2"`
	LastStatusCode *int            `json:"last_status_code,omitempty" gorm:"type:integer"`
	LastError      string          `json:"last_error" gorm:"not null;default:''"`
	LastAttemptAt  *time.Time      `json:"last_attempt_at,omitempty" gorm:"type:timestamptz"`
	DurationMS     *int            `json:"duration_ms,omitempty" gorm:"type:integer"`
	DeliveredAt    *time.Time      `json:"delivered_at,omitempty" gorm:"type:timestamptz"`
	CreatedAt      time.Time       `json:"created_at" gorm:"type:timestamptz;not null;index:idx_webhook_deliveries_endpoint,priority:2"`
}

func (WebhookDelivery) TableName() string { return "webhook_deliveries" }

// WebhookEndpointRequest creates or updates an endpoint.
type WebhookEndpointRequest struct {
	URL         string   `json:"url"`
	Description string   `json:"description"`
	Events      []string `json:"events"`
	MailboxIDs  []string `json:"mailbox_ids"`
	Enabled     *bool    `json:"enabled"`
}

// WebhookEndpointSecret is returned once when a secret is created or rotated.
type WebhookEndpointSecret struct {
	Endpoint *WebhookEndpoint `json:"endpoint"`
	Secret   string           `json:"secret"`
}

// WebhookEnvelope is the JSON body every delivery carries.
type WebhookEnvelope struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	CreatedAt   time.Time      `json:"created_at"`
	WorkspaceID string         `json:"workspace_id"`
	Data        map[string]any `json:"data"`
}

// WebhookConversation is the conversation snapshot in event payloads.
type WebhookConversation struct {
	ID           string                  `json:"id"`
	Number       int                     `json:"number"`
	Subject      string                  `json:"subject"`
	Status       string                  `json:"status"`
	Priority     string                  `json:"priority"`
	Channel      string                  `json:"channel"`
	InboxID      *string                 `json:"inbox_id"`
	AssigneeID   *string                 `json:"assignee_id"`
	Customer     WebhookCustomer         `json:"customer"`
	Tags         []string                `json:"tags"`
	SnoozedUntil *time.Time              `json:"snoozed_until"`
	MergedIntoID *string                 `json:"merged_into_id,omitempty"`
	SLA          *SupportConversationSLA `json:"sla,omitempty"`
	CreatedAt    time.Time               `json:"created_at"`
	UpdatedAt    time.Time               `json:"updated_at"`
	ResolvedAt   *time.Time              `json:"resolved_at"`
	URL          string                  `json:"url,omitempty"`
}

// WebhookCustomer identifies the customer in event payloads.
type WebhookCustomer struct {
	Name         *string `json:"name"`
	Email        *string `json:"email"`
	CRMContactID *string `json:"crm_contact_id"`
}

// WebhookMessage is a public message in event payloads.
type WebhookMessage struct {
	ID         string    `json:"id"`
	SenderType string    `json:"sender_type"`
	SenderID   *string   `json:"sender_user_id"`
	SenderName *string   `json:"sender_name"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}
