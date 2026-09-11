package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// SupportConversation represents a support conversation (renamed from SupportTicket).
type SupportConversation struct {
	ID                             string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID                    string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	MailboxID                      *string         `json:"mailbox_id" gorm:"type:uuid;index"`
	DisplayID                      int             `json:"display_id" gorm:"not null;index"`
	Subject                        string          `json:"subject" gorm:"not null"`
	Status                         string          `json:"status" gorm:"not null;default:'open'"`     // open, waiting_on_customer, resolved, spam
	FlowState                      *string         `json:"flow_state" gorm:"index"`                   // ai_handling, waiting_for_human, queued_for_human, after_hours_queue, assigned_to_human, resolved_by_ai, resolved_by_human
	HandoffState                   *string         `json:"handoff_state" gorm:"column:handoff_state"` // live, busy, after_hours — customer-facing availability at handoff time
	Priority                       string          `json:"priority" gorm:"not null;default:'medium'"` // low, medium, high, urgent
	Channel                        string          `json:"channel" gorm:"not null;default:'widget'"`  // widget, internal, email, api
	CustomerName                   *string         `json:"customer_name"`
	CustomerEmail                  *string         `json:"customer_email"`
	CustomerPhone                  *string         `json:"customer_phone"`
	PrimaryRecipientState          string          `json:"primary_recipient_state" gorm:"not null;default:'confirmed'"`
	SuggestedPrimaryRecipientEmail *string         `json:"suggested_primary_recipient_email,omitempty"`
	SuggestedPrimaryRecipientName  *string         `json:"suggested_primary_recipient_name,omitempty"`
	EmailCC                        DocsStringArray `json:"email_cc,omitempty" gorm:"type:text[]"`
	EmailThreadParticipants        DocsStringArray `json:"email_thread_participants,omitempty" gorm:"type:text[]"`
	OpenedByUserID                 *string         `json:"opened_by_user_id" gorm:"type:uuid"`
	AssignedUserID                 *string         `json:"assigned_user_id" gorm:"type:uuid"`
	AssignedAgentID                *string         `json:"assigned_agent_id" gorm:"type:uuid"`
	LinkedTaskID                   *string         `json:"linked_task_id" gorm:"column:linked_task_id;type:uuid"`
	Source                         string          `json:"source" gorm:"not null;default:'internal'"` // widget, internal, email, api - kept for backward compat
	AnonymousID                    *string         `json:"anonymous_id" gorm:"index"`
	CRMContactID                   *string         `json:"crm_contact_id" gorm:"type:uuid;index"`
	CRMCompanyID                   *string         `json:"crm_company_id" gorm:"type:uuid;index"`
	ResolvedAt                     *time.Time      `json:"resolved_at"`
	ClosedAt                       *time.Time      `json:"closed_at"`
	TeamLastSeenAt                 *time.Time      `json:"team_last_seen_at" gorm:"type:timestamptz"`
	ContactLastSeenAt              *time.Time      `json:"contact_last_seen_at" gorm:"type:timestamptz"`
	EmailUnsubscribed              bool            `json:"email_unsubscribed" gorm:"not null;default:false"`
	ListLastMessageID              *string         `json:"list_last_message_id,omitempty" gorm:"type:uuid"`
	ListLastMessageAt              *time.Time      `json:"list_last_message_at,omitempty" gorm:"type:timestamptz"`
	ListLastMessagePreview         *string         `json:"list_last_message_preview,omitempty"`
	ListLastMessageIsInternal      bool            `json:"list_last_message_is_internal" gorm:"not null;default:false"`
	LastPublicMessageID            *string         `json:"last_public_message_id,omitempty" gorm:"type:uuid"`
	LastPublicMessageAt            *time.Time      `json:"last_public_message_at,omitempty" gorm:"type:timestamptz"`
	LastPublicSenderType           *string         `json:"last_public_sender_type,omitempty"`
	LastPublicSenderDisplayName    *string         `json:"last_public_sender_display_name,omitempty"`
	LastCustomerMessageID          *string         `json:"last_customer_message_id,omitempty" gorm:"type:uuid"`
	LastCustomerMessageAt          *time.Time      `json:"last_customer_message_at,omitempty" gorm:"type:timestamptz"`
	UnansweredCustomerMessageCount int             `json:"unanswered_customer_message_count" gorm:"not null;default:0"`
	CustomerAwaitingResponse       bool            `json:"customer_awaiting_response" gorm:"not null;default:false"`
	NeedsHumanReply                bool            `json:"needs_human_reply" gorm:"not null;default:false"`
	SupportStateVersion            int64           `json:"state_version" gorm:"column:support_state_version;not null;default:0"`
	VisitorCountryCode             *string         `json:"visitor_country_code,omitempty"`
	VisitorCountryName             *string         `json:"visitor_country_name,omitempty"`
	ViewSearchDocument             *string         `json:"-"`

	// AI State — separate from human Status. Null when AI is not involved.
	AIState                  *string    `json:"ai_state" gorm:"index"` // null, "pending", "resolved", "escalated"
	AIResolvedAt             *time.Time `json:"ai_resolved_at" gorm:"type:timestamptz"`
	DelayedTeamReplySentFor  *time.Time `json:"-" gorm:"type:timestamptz;default:null"`
	AIEscalatedAt            *time.Time `json:"ai_escalated_at" gorm:"type:timestamptz"`
	AIResolutionType         *string    `json:"ai_resolution_type"` // "confirmed", "assumed", null
	AITurnCount              int        `json:"ai_turn_count" gorm:"not null;default:0"`
	CustomerRequestedHumanAt *time.Time `json:"customer_requested_human_at" gorm:"type:timestamptz"`
	// AIActiveRunID points at the agent-runtime chat run currently backing
	// this conversation's AI turns (nil before the first AI turn; repointed
	// when an idle-expired run gets a successor).
	AIFollowUp    *SupportAIFollowUp `json:"ai_follow_up,omitempty" gorm:"-"`
	AIActiveRunID *string            `json:"ai_active_run_id,omitempty" gorm:"type:uuid;index"`
	HumanTakeover *bool              `json:"human_takeover" gorm:"default:false;index"`

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	// Virtual fields — populated by SELECT subqueries, not stored as columns.
	LastMessage                  *string                    `json:"last_message,omitempty" gorm:"->"`
	LastMessageSenderType        *string                    `json:"last_message_sender_type,omitempty" gorm:"->"`
	LastMessageSenderDisplayName *string                    `json:"last_message_sender_display_name,omitempty" gorm:"->"`
	UnreadCount                  int                        `json:"unread_count" gorm:"->"`
	AwaitingReply                bool                       `json:"awaiting_reply" gorm:"->"`
	PersonalStateVersion         int64                      `json:"personal_state_version" gorm:"->"`
	CountryCode                  *string                    `json:"country_code,omitempty" gorm:"->"`
	CountryName                  *string                    `json:"country_name,omitempty" gorm:"->"`
	OpenedByDisplayName          *string                    `json:"opened_by_display_name,omitempty" gorm:"-"`
	OpenedByAvatarURL            *string                    `json:"opened_by_avatar_url,omitempty" gorm:"-"`
	OpenedByStatus               *string                    `json:"opened_by_status,omitempty" gorm:"-"`
	MailboxName                  *string                    `json:"mailbox_name,omitempty" gorm:"->"`
	MailboxHandle                *string                    `json:"mailbox_handle,omitempty" gorm:"->"`
	MailboxIcon                  *string                    `json:"mailbox_icon,omitempty" gorm:"->"`
	Triage                       *SupportConversationTriage `json:"triage,omitempty" gorm:"-"`
	Tags                         []SupportTag               `json:"tags,omitempty" gorm:"-"`
	SystemTags                   []string                   `json:"system_tags,omitempty" gorm:"-"`

	// ListLastActivityAt is derived from timeline entries, including notes and system events.
	// It must not replace message timestamps used for previews, unread state, or reply tracking.
	ListLastActivityAt *time.Time `json:"list_last_activity_at,omitempty" gorm:"->;-:migration"`
}

func (SupportConversation) TableName() string { return "support_conversations" }

const (
	SupportConversationStatusOpen              = "open"
	SupportConversationStatusWaitingOnCustomer = "waiting_on_customer"
	SupportConversationStatusResolved          = "resolved"
	SupportConversationStatusSpam              = "spam"

	SupportConversationFlowStateAIHandling      = "ai_handling"
	SupportConversationFlowStateWaitingForHuman = "waiting_for_human"
	SupportConversationFlowStateQueuedForHuman  = "queued_for_human"
	SupportConversationFlowStateAfterHoursQueue = "after_hours_queue"
	SupportConversationFlowStateAssignedToHuman = "assigned_to_human"
	SupportConversationFlowStateResolvedByAI    = "resolved_by_ai"
	SupportConversationFlowStateResolvedByHuman = "resolved_by_human"
)

const (
	SupportPrimaryRecipientStateConfirmed   = "confirmed"
	SupportPrimaryRecipientStateUnconfirmed = "unconfirmed"
)

// Handoff states describe availability at the moment the AI hands off.
const (
	HandoffStateLive       = "live"
	HandoffStateBusy       = "busy"
	HandoffStateAfterHours = "after_hours"
)

const (
	SupportConversationListFilterWaiting  = "waiting"
	SupportConversationListFilterInbox    = "inbox"
	SupportConversationListFilterMine     = "mine"
	SupportConversationListFilterMentions = "mentions"
	SupportConversationListFilterResolved = "resolved"
)

func NormalizeSupportConversationStatus(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "in_progress":
		return SupportConversationStatusOpen
	case "waiting":
		return SupportConversationStatusWaitingOnCustomer
	case "closed":
		return SupportConversationStatusResolved
	default:
		return strings.TrimSpace(strings.ToLower(status))
	}
}

func IsValidSupportConversationStatus(status string) bool {
	switch NormalizeSupportConversationStatus(status) {
	case SupportConversationStatusOpen, SupportConversationStatusWaitingOnCustomer, SupportConversationStatusResolved, SupportConversationStatusSpam:
		return true
	default:
		return false
	}
}

const (
	SupportTeammateStatusOnline  = "online"
	SupportTeammateStatusAway    = "away"
	SupportTeammateStatusOffline = "offline"
)

const (
	SupportConversationTriageStatusNotRun     = "not_run"
	SupportConversationTriageStatusSuggested  = "suggested"
	SupportConversationTriageStatusAutoMoved  = "auto_moved"
	SupportConversationTriageStatusDismissed  = "dismissed"
	SupportConversationTriageStatusOverridden = "overridden"
)

const (
	SupportConversationTriageSourceRule = "rule"
	SupportConversationTriageSourceAI   = "ai"
)

const (
	SupportConversationTriageFeedbackAccepted  = "accepted"
	SupportConversationTriageFeedbackDismissed = "dismissed"
	SupportConversationTriageFeedbackCorrected = "corrected"
)

const (
	SupportTeammateStatusSourceAuto   = "auto"
	SupportTeammateStatusSourceManual = "manual"
)

// SupportTeammatePresenceStatus represents a teammate's live support availability.
type SupportTeammatePresenceStatus struct {
	UserID       string     `json:"user_id"`
	Status       string     `json:"status"`
	Source       string     `json:"source"` // auto | manual
	ManualStatus *string    `json:"manual_status,omitempty"`
	LastSeenAt   *time.Time `json:"last_seen_at,omitempty"`
}

type UpdateSupportTeammatePresenceRequest struct {
	ManualStatus *string `json:"manual_status"`
}

// UnreadStats holds aggregate unread conversation counts for sidebar badges.
type UnreadStats struct {
	Inbox                   int `json:"inbox"`
	Mine                    int `json:"mine"`
	Waiting                 int `json:"waiting"`
	AIActive                int `json:"ai_active"`
	Total                   int `json:"total"`
	MyInbox                 int `json:"my_inbox"`
	Unassigned              int `json:"unassigned"`
	InboxTotal              int `json:"inbox_total"`
	MineTotal               int `json:"mine_total"`
	WaitingTotal            int `json:"waiting_total"`
	AIActiveTotal           int `json:"ai_active_total"`
	InboxNeedsHumanReply    int `json:"inbox_needs_human_reply"`
	MineNeedsHumanReply     int `json:"mine_needs_human_reply"`
	WaitingNeedsHumanReply  int `json:"waiting_needs_human_reply"`
	AIActiveNeedsHumanReply int `json:"ai_active_needs_human_reply"`
}

type SupportInboxScope struct {
	ID                   string  `json:"id"`
	Name                 string  `json:"name"`
	Handle               string  `json:"handle"`
	Icon                 string  `json:"icon"`
	IsShared             bool    `json:"is_shared"`
	IsDefault            bool    `json:"is_default"`
	TotalCount           int     `json:"total_count"`
	UnreadCount          int     `json:"unread_count"`
	NeedsHumanReplyCount int     `json:"needs_human_reply_count"`
	Active               bool    `json:"active"`
	LinkedTeamID         *string `json:"linked_team_id,omitempty"`
}

type SupportInboxScopeListResponse struct {
	SharedInbox SupportInboxScope   `json:"shared_inbox"`
	Mailboxes   []SupportInboxScope `json:"mailboxes"`
}

// SupportWorkspaceUnreadCount is the per-workspace unread aggregate returned by
// the workspace-switcher badge endpoint.
type SupportWorkspaceUnreadCount struct {
	WorkspaceID string `json:"workspace_id"`
	UnreadCount int    `json:"unread_count"`
}

// ConversationListMeta holds metadata returned alongside paginated conversation lists.
type ConversationListMeta struct {
	Unread UnreadStats `json:"unread"`
}

// ConversationListResponse is the paginated conversation list with unread metadata.
type ConversationListResponse struct {
	Data       []SupportConversation `json:"data"`
	Total      int                   `json:"total"`
	Page       int                   `json:"page"`
	PerPage    int                   `json:"per_page"`
	TotalPages int                   `json:"total_pages"`
	Meta       ConversationListMeta  `json:"meta"`
}

type SupportSearchHighlightRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type SupportSearchHighlight struct {
	Field  string                        `json:"field"`
	Text   string                        `json:"text"`
	Ranges []SupportSearchHighlightRange `json:"ranges"`
}

type SupportConversationSearchResult struct {
	Conversation  SupportConversation      `json:"conversation"`
	DisplayID     int                      `json:"display_id"`
	MatchedFields []string                 `json:"matched_fields"`
	Snippet       string                   `json:"snippet"`
	Highlights    []SupportSearchHighlight `json:"highlights"`
	Score         float64                  `json:"score"`
}

type SupportConversationSearchMeta struct {
	Sort        string `json:"sort"`
	Query       string `json:"query"`
	TotalCapped bool   `json:"total_capped"`
	TotalCap    int    `json:"total_cap"`
}

type SupportConversationSearchResponse struct {
	Data       []SupportConversationSearchResult `json:"data"`
	Total      int                               `json:"total"`
	Page       int                               `json:"page"`
	PerPage    int                               `json:"per_page"`
	TotalPages int                               `json:"total_pages"`
	Meta       SupportConversationSearchMeta     `json:"meta"`
}

type SupportConversationSearchParams struct {
	WorkspaceID   string
	UserID        string
	Query         string
	Sort          string
	Pagination    PMPagination
	AssignedTo    []string
	MailboxIDs    []string
	TagIDs        []string
	CustomerEmail string
	CreatedFrom   *time.Time
	CreatedTo     *time.Time
	Statuses      []string
	Priorities    []string
	Title         string
	AI            []string
}

// SupportMessage represents a message within a support conversation.
type SupportMessage struct {
	ID                string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID       string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ConversationID    string     `json:"conversation_id" gorm:"type:uuid;index"`
	SenderType        string     `json:"sender_type" gorm:"not null"`                  // customer, user, agent, ai
	MessageType       string     `json:"message_type" gorm:"not null;default:'reply'"` // reply, csat_survey, system
	SystemEventType   *string    `json:"system_event_type,omitempty" gorm:"size:40;index:idx_support_messages_system_event,where:system_event_type IS NOT NULL"`
	SenderUserID      *string    `json:"sender_user_id" gorm:"type:uuid"`
	SenderAgentID     *string    `json:"sender_agent_id" gorm:"type:uuid"`
	SenderDisplayName *string    `json:"sender_display_name"`
	SenderAvatarURL   *string    `json:"sender_avatar_url"`
	Content           string     `json:"content" gorm:"not null"`
	IsInternal        bool       `json:"is_internal" gorm:"not null;default:false"`
	Metadata          string     `json:"metadata" gorm:"type:jsonb;default:'{}'"` // JSONB for CSAT ratings, AI sources, etc.
	ViaChannel        *string    `json:"via_channel,omitempty" gorm:"size:20"`
	EmailNotifiedAt   *time.Time `json:"email_notified_at,omitempty"`
	EmailReadAt       *time.Time `json:"email_read_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	// CancellableUntil is the moment the email-fallback timer fires for an
	// outbound agent reply. Until this passes, the agent can soft-delete the
	// message and the queued email is removed from the per-conversation Redis
	// outbox. NULL for messages that aren't subject to email fallback.
	CancellableUntil *time.Time `json:"cancellable_until,omitempty" gorm:"index"`
	// DeletedAt enables GORM soft-delete: removed messages keep their row
	// (auditability) but are filtered out of every read path automatically.
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Virtual fields — populated by service layer, not stored in DB.
	Attachments     []SupportAttachmentPayload `json:"attachments,omitempty" gorm:"-"`
	ClientMessageID string                     `json:"client_message_id,omitempty" gorm:"-"`
	// HTMLBody is the sanitized HTML variant of an inbound email's body, loaded
	// from the linked support_email_logs row. Only populated for messages
	// where ViaChannel == "email" and an email log exists.
	HTMLBody string `json:"html_body,omitempty" gorm:"-"`
	// StrippedText is the markdown-friendly plaintext variant of an inbound
	// email's body. Same population rules as HTMLBody.
	StrippedText              string `json:"stripped_text,omitempty" gorm:"-"`
	EmailVisibleText          string `json:"email_visible_text,omitempty" gorm:"-"`
	EmailQuotedText           string `json:"email_quoted_text,omitempty" gorm:"-"`
	EmailHasQuotedContent     *bool  `json:"email_has_quoted_content,omitempty" gorm:"-"`
	EmailProjectionConfidence string `json:"email_projection_confidence,omitempty" gorm:"-"`
	EmailProjectionVersion    int    `json:"email_projection_version,omitempty" gorm:"-"`
	// EmailDeliveryStatus mirrors the linked outbound support_email_log's status
	// ("sent", "delivered", "opened", "bounced", "spam_complaint"). Only set
	// when an email log exists for the message.
	EmailDeliveryStatus string `json:"email_delivery_status,omitempty" gorm:"-"`
	// EmailDeliveryError surfaces the bounce/complaint description when the
	// email's delivery failed. Empty otherwise.
	EmailDeliveryError string          `json:"email_delivery_error,omitempty" gorm:"-"`
	EmailFrom          string          `json:"email_from,omitempty" gorm:"-"`
	EmailTo            string          `json:"email_to,omitempty" gorm:"-"`
	EmailReplyTo       string          `json:"email_reply_to,omitempty" gorm:"-"`
	EmailCC            DocsStringArray `json:"email_cc,omitempty" gorm:"-"`
	EmailBCC           DocsStringArray `json:"email_bcc,omitempty" gorm:"-"`
}

func (SupportMessage) TableName() string { return "support_messages" }

// SupportMessagePage is a cursor-paginated slice of a conversation transcript.
type SupportMessagePage struct {
	Data       []SupportMessage `json:"data"`
	HasMore    bool             `json:"has_more"`
	NextCursor *string          `json:"next_cursor,omitempty"`
}

// SupportLinkPreview represents an unfurled link card attached to a support message.
type SupportLinkPreview struct {
	URL         string  `json:"url"`
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
	SiteName    *string `json:"site_name,omitempty"`
	ImageURL    *string `json:"image_url,omitempty"`
	Host        string  `json:"host"`
}

// SupportLinkSecurity records a time-bounded reputation lookup for a support-message URL.
type SupportLinkSecurity struct {
	URL         string    `json:"url"`
	Status      string    `json:"status"`
	ThreatTypes []string  `json:"threat_types,omitempty"`
	CheckedAt   time.Time `json:"checked_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// SupportCannedResponse represents a canned response for quick replies.
type SupportCannedResponse struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ShortCode   string    `json:"short_code" gorm:"not null"` // e.g., "!greeting", "!thanks"
	Content     string    `json:"content" gorm:"not null"`
	Tag         string    `json:"tag" gorm:"not null;default:'General'"`
	CreatedByID string    `json:"created_by_id" gorm:"type:uuid"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportCannedResponse) TableName() string { return "support_canned_responses" }

// SupportWidgetInstallation holds workspace-level widget configuration.
type SupportWidgetInstallation struct {
	ID                       string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID              string          `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex"`
	WidgetKey                string          `json:"widget_key" gorm:"not null"` // public key for embedding
	SecretKey                string          `json:"-" gorm:"not null"`          // S2S auth and identity signing
	AllowedOrigins           DocsStringArray `json:"allowed_origins" gorm:"type:text[];not null;default:'{}'"`
	IdentityVerificationMode string          `json:"identity_verification_mode" gorm:"not null;default:'report_only'"`
	Settings                 string          `json:"settings" gorm:"type:jsonb;not null;default:'{}'"`
	Active                   bool            `json:"active" gorm:"not null;default:true"`
	CreatedAt                time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportWidgetInstallation) TableName() string { return "support_widget_installations" }

const (
	IdentityVerificationModeOff        = "off"
	IdentityVerificationModeReportOnly = "report_only"
	IdentityVerificationModeEnforced   = "enforced"

	IdentityMethodAnonymous        = "anonymous"
	IdentityMethodBrowserClaim     = "browser_claim"
	IdentityMethodSignedWidget     = "signed_widget"
	IdentityMethodVerifiedSupport  = "verified_support"
	IdentityMethodConnectedMailbox = "connected_mailbox"
	IdentityMethodServerEvent      = "server_event"
	IdentityMethodManualEntry      = "manual_entry"

	IdentityTrustUntrusted     = "untrusted"
	IdentityTrustProbabilistic = "probabilistic"
	IdentityTrustVerified      = "verified"

	// CompanyMatchMethodDeclared means the company was supplied by the
	// customer's own identify call and is as trustworthy as that call.
	CompanyMatchMethodDeclared = "declared"
	// CompanyMatchMethodEmailDomain means the company was inferred from the
	// visitor's email domain. This is probabilistic regardless of how strongly
	// the person themselves was identified.
	CompanyMatchMethodEmailDomain = "email_domain"
)

// WorkspaceEventProjectAlias maps a historical ClickHouse project to its workspace.
// Legacy server-secret aliases are read compatibility data and must never be exposed.
type WorkspaceEventProjectAlias struct {
	ID          string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string     `json:"workspace_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_event_project_workspace"`
	ProjectID   string     `json:"-" gorm:"not null;uniqueIndex;uniqueIndex:idx_event_project_workspace"`
	Source      string     `json:"source" gorm:"not null"`
	ValidFrom   *time.Time `json:"valid_from,omitempty" gorm:"type:timestamptz"`
	ValidTo     *time.Time `json:"valid_to,omitempty" gorm:"type:timestamptz"`
	CreatedAt   time.Time  `json:"created_at" gorm:"autoCreateTime"`
}

func (WorkspaceEventProjectAlias) TableName() string {
	return "workspace_event_project_aliases"
}

// CRMIdentityLink records the provenance of a browser-to-CRM identity mutation.
type CRMIdentityLink struct {
	ID             string  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string  `json:"workspace_id" gorm:"type:uuid;not null;index"`
	AnonymousID    string  `json:"anonymous_id" gorm:"not null;index"`
	ExternalUserID *string `json:"external_user_id,omitempty" gorm:"index"`
	ContactID      *string `json:"contact_id,omitempty" gorm:"type:uuid;index"`
	CompanyID      *string `json:"company_id,omitempty" gorm:"type:uuid;index"`
	IdentityMethod string  `json:"identity_method" gorm:"not null"`
	IdentityTrust  string  `json:"identity_trust" gorm:"not null"`
	// CompanyMatchMethod records how CompanyID was resolved. IdentityMethod and
	// IdentityTrust describe proof of the *person's* identity, which says
	// nothing about how their account was attached: a HMAC-verified visitor may
	// still have had their company inferred from an email domain. Consumers that
	// act on company-scoped evidence must check this rather than assuming
	// IdentityTrust covers it.
	CompanyMatchMethod *string    `json:"company_match_method,omitempty"`
	VerifiedAt         *time.Time `json:"verified_at,omitempty" gorm:"type:timestamptz"`
	VerifierVersion    *string    `json:"verifier_version,omitempty"`
	CreatedAt          time.Time  `json:"created_at" gorm:"autoCreateTime"`
}

func (CRMIdentityLink) TableName() string { return "crm_identity_links" }

// SupportCredentialRotationAudit records who rotated an installation secret.
type SupportCredentialRotationAudit struct {
	ID             string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	InstallationID string    `json:"installation_id" gorm:"type:uuid;not null;index"`
	ActorUserID    string    `json:"actor_user_id" gorm:"type:uuid;not null"`
	RotationKind   string    `json:"rotation_kind" gorm:"not null"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (SupportCredentialRotationAudit) TableName() string {
	return "support_credential_rotation_audits"
}

// SupportWidgetSession represents an external widget chat session (30-day TTL).
type SupportWidgetSession struct {
	ID                      string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID             string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ConversationID          *string    `json:"conversation_id" gorm:"type:uuid;index"`
	SessionToken            string     `json:"-" gorm:"not null;uniqueIndex"`
	AnonymousID             string     `json:"anonymous_id" gorm:"not null"`
	IsAnonymous             bool       `json:"is_anonymous" gorm:"default:true"`
	CustomerName            *string    `json:"customer_name"`
	CustomerEmail           *string    `json:"customer_email"`
	CustomerPhone           *string    `json:"customer_phone"`
	UserAgent               *string    `json:"-"`
	LastPageURL             *string    `json:"last_page_url"`
	Timezone                *string    `json:"timezone"`
	Locale                  *string    `json:"locale"`
	IPAddress               *string    `json:"ip_address,omitempty" gorm:"size:64"`
	CountryCode             *string    `json:"country_code,omitempty" gorm:"size:8"`
	CountryName             *string    `json:"country_name,omitempty" gorm:"size:128"`
	RegionName              *string    `json:"region_name,omitempty" gorm:"size:128"`
	CityName                *string    `json:"city_name,omitempty" gorm:"size:128"`
	CRMCompanyID            *string    `json:"crm_company_id" gorm:"type:uuid;index"`
	IdentityMethod          string     `json:"identity_method" gorm:"not null;default:'anonymous';index"`
	IdentityTrust           string     `json:"identity_trust" gorm:"not null;default:'untrusted';index"`
	IdentityVerifiedAt      *time.Time `json:"identity_verified_at,omitempty" gorm:"type:timestamptz"`
	IdentityVerifierVersion *string    `json:"identity_verifier_version,omitempty"`
	LastActiveAt            *time.Time `json:"last_active_at,omitempty" gorm:"type:timestamptz;index"`
	RevokedAt               *time.Time `json:"-" gorm:"index"`
	ExpiresAt               time.Time  `json:"expires_at" gorm:"not null"`
	CreatedAt               time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt               time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportWidgetSession) TableName() string { return "support_widget_sessions" }

type SupportMailbox struct {
	ID             string  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string  `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Name           string  `json:"name" gorm:"not null"`
	Handle         string  `json:"handle" gorm:"not null"`
	Icon           string  `json:"icon" gorm:"not null;default:'inbox'"`
	Description    *string `json:"description"`
	RoutingPrompt  *string `json:"routing_prompt"`
	TriageEligible bool    `json:"triage_eligible" gorm:"not null;default:true"`
	LinkedTeamID   *string `json:"linked_team_id" gorm:"type:uuid"`
	VisibilityMode string  `json:"visibility_mode" gorm:"not null;default:'members_only'"`
	AssignmentMode string  `json:"assignment_mode" gorm:"not null;default:'manual'"`
	// ReplyTimePreset / ReplyTimeCustomMinutes override the workspace-wide
	// reply-time expectation for conversations routed into this mailbox.
	// Nil preset means "inherit workspace default".
	ReplyTimePreset        *string   `json:"reply_time_preset,omitempty" gorm:"size:20;default:null"`
	ReplyTimeCustomMinutes *int      `json:"reply_time_custom_minutes,omitempty" gorm:"default:null"`
	Position               int       `json:"position" gorm:"not null;default:0"`
	Active                 bool      `json:"active" gorm:"not null;default:true"`
	CreatedByID            string    `json:"created_by_id" gorm:"type:uuid;not null"`
	CreatedAt              time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt              time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	LinkedTeamName *string `json:"linked_team_name,omitempty" gorm:"->"`
	MemberCount    int     `json:"member_count,omitempty" gorm:"->"`
	UnreadCount    int     `json:"unread_count,omitempty" gorm:"->"`
}

func (SupportMailbox) TableName() string { return "support_mailboxes" }

type SupportConversationTriage struct {
	ID                 string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID        string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ConversationID     string     `json:"conversation_id" gorm:"type:uuid;not null;uniqueIndex"`
	Status             string     `json:"status" gorm:"not null;default:'suggested'"`
	Intent             *string    `json:"intent,omitempty"`
	Confidence         *float64   `json:"confidence,omitempty"`
	Reason             *string    `json:"reason,omitempty"`
	ClassifierSource   string     `json:"classifier_source" gorm:"column:source;not null"`
	SuggestedMailboxID *string    `json:"suggested_mailbox_id,omitempty" gorm:"type:uuid;index"`
	AutoMoved          bool       `json:"auto_moved" gorm:"not null;default:false"`
	LockedAt           *time.Time `json:"locked_at,omitempty"`
	EvaluatedAt        *time.Time `json:"evaluated_at,omitempty"`
	FeedbackAction     *string    `json:"feedback_action,omitempty"`
	InputHash          string     `json:"-" gorm:"index"`
	CreatedAt          time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportConversationTriage) TableName() string { return "support_conversation_triage" }

type SupportConversationTriageEvent struct {
	ID             string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ConversationID string    `json:"conversation_id" gorm:"type:uuid;not null;index"`
	TriageID       *string   `json:"triage_id,omitempty" gorm:"type:uuid;index"`
	EventType      string    `json:"event_type" gorm:"not null;index"`
	Source         *string   `json:"source,omitempty" gorm:"index"`
	Cached         bool      `json:"cached,omitempty" gorm:"not null;default:false"`
	FromMailboxID  *string   `json:"from_mailbox_id,omitempty" gorm:"type:uuid"`
	ToMailboxID    *string   `json:"to_mailbox_id,omitempty" gorm:"type:uuid"`
	ActorUserID    *string   `json:"actor_user_id,omitempty" gorm:"type:uuid"`
	InputHash      string    `json:"-" gorm:"index"`
	Payload        JSONB     `json:"payload" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (SupportConversationTriageEvent) TableName() string { return "support_conversation_triage_events" }

type SupportTriageRuleConditions struct {
	ConditionLogic      string   `json:"condition_logic,omitempty"`
	PhraseContains      []string `json:"phrase_contains,omitempty"`
	EmailDomainEquals   []string `json:"email_domain_equals,omitempty"`
	SenderEmailContains []string `json:"sender_email_contains,omitempty"`
}

func (c SupportTriageRuleConditions) Value() (driver.Value, error) {
	if len(c.PhraseContains) == 0 && len(c.EmailDomainEquals) == 0 && len(c.SenderEmailContains) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (c *SupportTriageRuleConditions) Scan(value interface{}) error {
	if value == nil {
		*c = SupportTriageRuleConditions{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("failed to scan SupportTriageRuleConditions: %T", value)
	}
	if len(bytes) == 0 {
		*c = SupportTriageRuleConditions{}
		return nil
	}
	return json.Unmarshal(bytes, c)
}

type SupportTriageRule struct {
	ID              string                      `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string                      `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Priority        int                         `json:"priority" gorm:"not null;default:0"`
	Active          bool                        `json:"active" gorm:"not null;default:true"`
	Name            string                      `json:"name" gorm:"not null"`
	Channels        DocsStringArray             `json:"channels" gorm:"type:text[];not null;default:'{}'"`
	Conditions      SupportTriageRuleConditions `json:"conditions" gorm:"type:jsonb;not null;default:'{}'"`
	TargetMailboxID string                      `json:"target_mailbox_id" gorm:"type:uuid;not null;index"`
	CreatedByID     string                      `json:"created_by_id" gorm:"type:uuid;not null"`
	CreatedAt       time.Time                   `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time                   `json:"updated_at" gorm:"autoUpdateTime"`

	TargetMailboxName   *string `json:"target_mailbox_name,omitempty" gorm:"->"`
	TargetMailboxHandle *string `json:"target_mailbox_handle,omitempty" gorm:"->"`
}

func (SupportTriageRule) TableName() string { return "support_triage_rules" }

type SupportMailboxMembership struct {
	ID                string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	MailboxID         string    `json:"mailbox_id" gorm:"type:uuid;not null;index"`
	WorkspaceMemberID string    `json:"workspace_member_id" gorm:"type:uuid;not null;index"`
	CreatedAt         time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportMailboxMembership) TableName() string { return "support_mailbox_memberships" }

type SupportMailboxMember struct {
	WorkspaceMemberID     string  `json:"workspace_member_id"`
	UserID                *string `json:"user_id,omitempty"`
	Email                 string  `json:"email"`
	DisplayName           string  `json:"display_name"`
	AvatarURL             *string `json:"avatar_url,omitempty"`
	AvatarStyle           *string `json:"avatar_style,omitempty"`
	AvatarSeed            *string `json:"avatar_seed,omitempty"`
	AvatarBackgroundMode  *string `json:"avatar_background_mode,omitempty"`
	AvatarBackgroundColor *string `json:"avatar_background_color,omitempty"`
	Role                  string  `json:"role"`
}

type SupportEmailRoute struct {
	ID                          string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID                 string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	MailboxID                   *string    `json:"mailbox_id,omitempty" gorm:"type:uuid;index"`
	RouteKey                    string     `json:"route_key" gorm:"not null;uniqueIndex"`
	InboundAddress              string     `json:"inbound_address" gorm:"not null;uniqueIndex"`
	SourceAddress               *string    `json:"source_address,omitempty"`
	ProviderType                string     `json:"provider_type" gorm:"not null;default:'forwarding'"`
	Active                      bool       `json:"active" gorm:"not null;default:true"`
	LastInboundAt               *time.Time `json:"last_inbound_at,omitempty"`
	ConfirmationReceivedAt      *time.Time `json:"confirmation_received_at,omitempty"`
	ConfirmationConversationID  *string    `json:"confirmation_conversation_id,omitempty" gorm:"type:uuid;index"`
	VerificationSentAt          *time.Time `json:"verification_sent_at,omitempty"`
	ForwardingVerifiedAt        *time.Time `json:"forwarding_verified_at,omitempty"`
	ForwardingVerificationToken string     `json:"-"`
	ForwardingLastError         *string    `json:"forwarding_last_error,omitempty"`
	CreatedByID                 string     `json:"created_by_id" gorm:"type:uuid;not null"`
	CreatedAt                   time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                   time.Time  `json:"updated_at" gorm:"autoUpdateTime"`

	MailboxName   *string `json:"mailbox_name,omitempty" gorm:"->"`
	MailboxHandle *string `json:"mailbox_handle,omitempty" gorm:"->"`
	MailboxIcon   *string `json:"mailbox_icon,omitempty" gorm:"->"`
}

func (SupportEmailRoute) TableName() string { return "support_email_routes" }

type CreateSupportEmailRouteRequest struct {
	MailboxID     *string `json:"mailbox_id"`
	SourceAddress *string `json:"source_address"`
}

// SendSupportEmailRouteTestRequest identifies the external inbox Helpin should test.
type SendSupportEmailRouteTestRequest struct {
	SourceAddress string `json:"source_address"`
}

type DisableSupportEmailRouteRequest struct{}

type SupportEmailSenderDomain struct {
	ID                         string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID                string     `json:"workspace_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_support_email_sender_domain_workspace_domain"`
	Domain                     string     `json:"domain" gorm:"not null;uniqueIndex:idx_support_email_sender_domain_workspace_domain"`
	FromLocalPart              string     `json:"from_local_part" gorm:"not null;default:'support'"`
	PostmarkDomainID           *int       `json:"postmark_domain_id,omitempty" gorm:"uniqueIndex"`
	ReturnPathDomain           string     `json:"return_path_domain"`
	ReturnPathDomainCNAMEValue string     `json:"return_path_domain_cname_value"`
	ReturnPathDomainVerified   bool       `json:"return_path_domain_verified" gorm:"not null;default:false"`
	DKIMHost                   string     `json:"dkim_host"`
	DKIMTextValue              string     `json:"dkim_text_value"`
	DKIMPendingHost            string     `json:"dkim_pending_host"`
	DKIMPendingTextValue       string     `json:"dkim_pending_text_value"`
	DKIMVerified               bool       `json:"dkim_verified" gorm:"not null;default:false"`
	DKIMUpdateStatus           string     `json:"dkim_update_status"`
	Status                     string     `json:"status" gorm:"not null;default:'pending_dns';index"`
	Active                     bool       `json:"active" gorm:"not null;default:false;index"`
	LastCheckedAt              *time.Time `json:"last_checked_at,omitempty"`
	LastError                  *string    `json:"last_error,omitempty"`
	CreatedByID                string     `json:"created_by_id" gorm:"type:uuid;not null"`
	CreatedAt                  time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                  time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportEmailSenderDomain) TableName() string { return "support_email_sender_domains" }

type CreateSupportEmailSenderDomainRequest struct {
	Domain        string `json:"domain"`
	FromLocalPart string `json:"from_local_part"`
}

type SupportEmailSender struct {
	ID                          string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID                 string     `json:"workspace_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_support_email_sender_workspace_email"`
	MailboxID                   *string    `json:"mailbox_id,omitempty" gorm:"type:uuid;index"`
	Email                       string     `json:"email" gorm:"not null;uniqueIndex:idx_support_email_sender_workspace_email"`
	LocalPart                   string     `json:"local_part" gorm:"not null"`
	Domain                      string     `json:"domain" gorm:"not null;index"`
	DisplayName                 string     `json:"display_name"`
	PostmarkDomainID            *int       `json:"postmark_domain_id,omitempty" gorm:"index"`
	ReturnPathDomain            string     `json:"return_path_domain"`
	ReturnPathDomainCNAMEValue  string     `json:"return_path_domain_cname_value"`
	ReturnPathDomainVerified    bool       `json:"return_path_domain_verified" gorm:"not null;default:false"`
	DKIMHost                    string     `json:"dkim_host"`
	DKIMTextValue               string     `json:"dkim_text_value"`
	DKIMPendingHost             string     `json:"dkim_pending_host"`
	DKIMPendingTextValue        string     `json:"dkim_pending_text_value"`
	DKIMVerified                bool       `json:"dkim_verified" gorm:"not null;default:false"`
	DKIMUpdateStatus            string     `json:"dkim_update_status"`
	DMARCHost                   string     `json:"dmarc_host"`
	DMARCPolicy                 string     `json:"dmarc_policy"`
	DMARCRecordPresent          bool       `json:"dmarc_record_present" gorm:"not null;default:false"`
	DMARCLastCheckedAt          *time.Time `json:"dmarc_last_checked_at,omitempty"`
	DomainStatus                string     `json:"domain_status" gorm:"not null;default:'pending_dns';index"`
	ForwardingStatus            string     `json:"forwarding_status" gorm:"not null;default:'not_started';index"`
	ForwardingVerificationToken string     `json:"-" gorm:"uniqueIndex"`
	ForwardingAddress           string     `json:"forwarding_address"`
	ForwardingVerifiedAt        *time.Time `json:"forwarding_verified_at,omitempty"`
	ForwardingLastCheckedAt     *time.Time `json:"forwarding_last_checked_at,omitempty"`
	ForwardingLastError         *string    `json:"forwarding_last_error,omitempty"`
	EmailRouteID                *string    `json:"email_route_id,omitempty" gorm:"type:uuid;index"`
	VerificationStatus          string     `json:"verification_status" gorm:"not null;default:'pending_dns';index"`
	DefaultScope                string     `json:"default_scope" gorm:"not null;default:'none';index"`
	Active                      bool       `json:"active" gorm:"not null;default:false;index"`
	LastCheckedAt               *time.Time `json:"last_checked_at,omitempty"`
	LastError                   *string    `json:"last_error,omitempty"`
	CreatedByID                 string     `json:"created_by_id" gorm:"type:uuid;not null"`
	CreatedAt                   time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                   time.Time  `json:"updated_at" gorm:"autoUpdateTime"`

	MailboxName   *string  `json:"mailbox_name,omitempty" gorm:"->"`
	MailboxHandle *string  `json:"mailbox_handle,omitempty" gorm:"->"`
	MailboxIcon   *string  `json:"mailbox_icon,omitempty" gorm:"->"`
	MailboxIDs    []string `json:"mailbox_ids,omitempty" gorm:"-"`
}

func (SupportEmailSender) TableName() string { return "support_email_senders" }

type SupportEmailSenderMailbox struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	SenderID    string    `json:"sender_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_support_email_sender_mailbox_sender_mailbox"`
	MailboxID   string    `json:"mailbox_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_support_email_sender_mailbox_sender_mailbox;uniqueIndex:idx_support_email_sender_mailbox_mailbox"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportEmailSenderMailbox) TableName() string { return "support_email_sender_mailboxes" }

type CreateSupportEmailSenderRequest struct {
	Email       string  `json:"email"`
	DisplayName string  `json:"display_name"`
	MailboxID   *string `json:"mailbox_id"`
}

type SetSupportEmailSenderDefaultRequest struct {
	DefaultScope string   `json:"default_scope"`
	MailboxID    *string  `json:"mailbox_id"`
	MailboxIDs   []string `json:"mailbox_ids,omitempty"`
}

type UpdateSupportEmailSenderRequest struct {
	DisplayName  *string  `json:"display_name,omitempty"`
	DefaultScope *string  `json:"default_scope,omitempty"`
	MailboxID    *string  `json:"mailbox_id,omitempty"`
	MailboxIDs   []string `json:"mailbox_ids,omitempty"`
}

type CreateSupportMailboxRequest struct {
	Name               string   `json:"name"`
	Handle             string   `json:"handle"`
	Icon               string   `json:"icon"`
	Description        *string  `json:"description"`
	RoutingPrompt      *string  `json:"routing_prompt"`
	TriageEligible     *bool    `json:"triage_eligible"`
	LinkedTeamID       *string  `json:"linked_team_id"`
	WorkspaceMemberIDs []string `json:"workspace_member_ids"`
	AssignmentMode     string   `json:"assignment_mode"`
	ImportLinkedTeam   bool     `json:"import_linked_team"`
}

type OptionalNullableString struct {
	Set   bool
	Value *string
}

func (s *OptionalNullableString) UnmarshalJSON(data []byte) error {
	s.Set = true
	if strings.TrimSpace(string(data)) == "null" {
		s.Value = nil
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	s.Value = &value
	return nil
}

type UpdateSupportMailboxRequest struct {
	Name               *string                `json:"name,omitempty"`
	Handle             *string                `json:"handle,omitempty"`
	Icon               *string                `json:"icon,omitempty"`
	Description        *string                `json:"description,omitempty"`
	RoutingPrompt      *string                `json:"routing_prompt,omitempty"`
	TriageEligible     *bool                  `json:"triage_eligible,omitempty"`
	LinkedTeamID       OptionalNullableString `json:"linked_team_id,omitempty"`
	WorkspaceMemberIDs []string               `json:"workspace_member_ids,omitempty"`
	AssignmentMode     *string                `json:"assignment_mode,omitempty"`
	Active             *bool                  `json:"active,omitempty"`
	ImportLinkedTeam   bool                   `json:"import_linked_team,omitempty"`

	// ReplyTimePreset overrides the workspace default for conversations in
	// this mailbox. Pass an explicit value to set; set ClearReplyTimePreset
	// to true to clear the override and inherit from workspace again.
	ReplyTimePreset             *string `json:"reply_time_preset,omitempty"`
	ReplyTimeCustomMinutes      *int    `json:"reply_time_custom_minutes,omitempty"`
	ClearReplyTimePreset        *bool   `json:"clear_reply_time_preset,omitempty"`
	ClearReplyTimeCustomMinutes *bool   `json:"clear_reply_time_custom_minutes,omitempty"`
}

type ReorderSupportMailboxesRequest struct {
	MailboxIDs []string `json:"mailbox_ids"`
}

type MoveSupportConversationRequest struct {
	MailboxID *string `json:"mailbox_id"`
}

type CreateSupportTriageRuleRequest struct {
	Priority        int                         `json:"priority"`
	Active          *bool                       `json:"active"`
	Name            string                      `json:"name"`
	Channels        []string                    `json:"channels"`
	Conditions      SupportTriageRuleConditions `json:"conditions"`
	TargetMailboxID string                      `json:"target_mailbox_id"`
}

type UpdateSupportTriageRuleRequest struct {
	Priority        *int                         `json:"priority,omitempty"`
	Active          *bool                        `json:"active,omitempty"`
	Name            *string                      `json:"name,omitempty"`
	Channels        []string                     `json:"channels,omitempty"`
	Conditions      *SupportTriageRuleConditions `json:"conditions,omitempty"`
	TargetMailboxID *string                      `json:"target_mailbox_id,omitempty"`
}

// CreateConversationRequest is the payload for creating a support conversation.
type CreateConversationRequest struct {
	WorkspaceID   string  `json:"workspace_id"`
	MailboxID     *string `json:"mailbox_id,omitempty"`
	Subject       string  `json:"subject"`
	Priority      string  `json:"priority"`
	CustomerName  *string `json:"customer_name"`
	CustomerEmail *string `json:"customer_email"`
	Source        string  `json:"source"`
}

// CreateConversationWithMessageRequest creates a support conversation and its
// first public message in one operation.
type CreateConversationWithMessageRequest struct {
	WorkspaceID   string   `json:"workspace_id"`
	MailboxID     *string  `json:"mailbox_id,omitempty"`
	Subject       string   `json:"subject"`
	CustomerName  *string  `json:"customer_name,omitempty"`
	CustomerEmail *string  `json:"customer_email,omitempty"`
	CRMContactID  *string  `json:"crm_contact_id,omitempty"`
	Channels      []string `json:"channels,omitempty"`
	Content       string   `json:"content"`
	AttachmentIDs []string `json:"attachment_ids,omitempty"`
	TagIDs        []string `json:"tag_ids,omitempty"`
	CCEmails      []string `json:"cc_emails,omitempty"`
	BCCEmails     []string `json:"bcc_emails,omitempty"`
}

type CreateConversationWithMessageResponse struct {
	Conversation *SupportConversation `json:"conversation"`
	Message      *SupportMessage      `json:"message"`
}

// CreateMessageRequest is the payload for creating a support message.
type CreateMessageRequest struct {
	Content         string   `json:"content"`
	ClientMessageID string   `json:"client_message_id,omitempty"`
	IsInternal      bool     `json:"is_internal"`
	AIAssisted      bool     `json:"ai_assisted,omitempty"`
	MessageType     string   `json:"message_type"` // reply, csat_survey, system
	AttachmentIDs   []string `json:"attachment_ids,omitempty"`
	Channels        []string `json:"channels,omitempty"`
	CCEmails        []string `json:"cc_emails,omitempty"`
	BCCEmails       []string `json:"bcc_emails,omitempty"`
}

// LinkTaskRequest links a conversation to a task.
type LinkTaskRequest struct {
	TaskID string `json:"task_id"`
}

// CreateTaskFromConversationRequest creates a PM task from the current support conversation.
type CreateTaskFromConversationRequest struct {
	Name              *string                      `json:"name,omitempty"`
	Description       *string                      `json:"description,omitempty"`
	TaskType          *string                      `json:"task_type,omitempty"`
	WorkflowID        *string                      `json:"workflow_id,omitempty"`
	WorkflowStateID   *string                      `json:"workflow_state_id,omitempty"`
	EpicID            *string                      `json:"epic_id,omitempty"`
	SprintID          *string                      `json:"sprint_id,omitempty"`
	TeamID            *string                      `json:"team_id,omitempty"`
	OwnerMemberID     *string                      `json:"owner_member_id,omitempty"`
	RequesterMemberID *string                      `json:"requester_member_id,omitempty"`
	Estimate          *int                         `json:"estimate,omitempty"`
	Priority          *string                      `json:"priority,omitempty"`
	Severity          *string                      `json:"severity,omitempty"`
	Deadline          *time.Time                   `json:"deadline,omitempty"`
	Position          *int                         `json:"position,omitempty"`
	Blocked           *bool                        `json:"blocked,omitempty"`
	Blocker           *string                      `json:"blocker,omitempty"`
	TemplateID        *string                      `json:"template_id,omitempty"`
	ExternalID        *string                      `json:"external_id,omitempty"`
	OwnerIDs          []string                     `json:"owner_ids,omitempty"`
	FollowerIDs       []string                     `json:"follower_ids,omitempty"`
	LabelIDs          []string                     `json:"label_ids,omitempty"`
	AttachmentIDs     []string                     `json:"attachment_ids,omitempty"`
	ChecklistItems    []CreateChecklistItemRequest `json:"checklist_items,omitempty"`
	ExternalLinks     []CreateExternalLinkRequest  `json:"external_links,omitempty"`
}

// CreateTaskFromConversationResponse summarizes the created PM task and copied associations.
type CreateTaskFromConversationResponse struct {
	TaskID                    string `json:"task_id"`
	DisplayID                 int    `json:"display_id,omitempty"`
	TaskKey                   string `json:"task_key,omitempty"`
	TaskName                  string `json:"task_name"`
	Summary                   string `json:"summary,omitempty"`
	CopiedContactAssociations int    `json:"copied_contact_associations"`
	CopiedCompanyAssociations int    `json:"copied_company_associations"`
	CopiedDealAssociations    int    `json:"copied_deal_associations"`
}

// AssignConversationAgentRequest assigns an agent to a conversation.
type AssignConversationAgentRequest struct {
	AgentID string `json:"agent_id"`
}

// AssignConversationUserRequest assigns a teammate to a conversation.
type AssignConversationUserRequest struct {
	UserID *string `json:"user_id"`
}

// UpdateConversationCRMContactRequest sets or clears the primary CRM contact link.
type UpdateConversationCRMContactRequest struct {
	CRMContactID *string `json:"crm_contact_id"`
}

// UpdateConversationCRMCompanyRequest sets or clears explicit company context.
type UpdateConversationCRMCompanyRequest struct {
	CRMCompanyID *string `json:"crm_company_id"`
}

// UpdateConversationCustomerNameRequest sets the support conversation customer display name.
type UpdateConversationCustomerNameRequest struct {
	CustomerName string `json:"customer_name"`
}

// UpdateConversationEmailRecipientsRequest confirms or updates who receives email replies.
type UpdateConversationEmailRecipientsRequest struct {
	PrimaryRecipientEmail *string  `json:"primary_recipient_email,omitempty"`
	PrimaryRecipientName  *string  `json:"primary_recipient_name,omitempty"`
	CCEmails              []string `json:"cc_emails,omitempty"`
	ConfirmPrimary        *bool    `json:"confirm_primary,omitempty"`
}

// UpdateConversationStatusRequest changes conversation status.
type UpdateConversationStatusRequest struct {
	Status string `json:"status"`
}

// WidgetSessionRequest creates a new widget session (legacy HTTP).
type WidgetSessionRequest struct {
	WorkspaceSlug string  `json:"workspace_slug"`
	WidgetKey     string  `json:"widget_key"`
	CustomerName  *string `json:"customer_name"`
	CustomerEmail *string `json:"customer_email"`
}

// WidgetMessageRequest sends a message via widget (legacy HTTP).
type WidgetMessageRequest struct {
	SessionToken string `json:"session_token"`
	Content      string `json:"content"`
}

// WidgetTypingRequest sends a typing indicator via widget HTTP fallback.
type WidgetTypingRequest struct {
	SessionToken string `json:"session_token"`
	IsTyping     bool   `json:"is_typing"`
}

// WidgetSessionRevokeRequest revokes a widget session (HTTP fallback for shutdown).
type WidgetSessionRevokeRequest struct {
	SessionToken string `json:"session_token"`
}

type WidgetTranscriptRequest struct {
	SessionToken string `json:"session_token"`
	Email        string `json:"email,omitempty"`
}

type WidgetTranscriptResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type SendSupportConversationTranscriptRequest struct {
	Email               string `json:"email"`
	UpdateCustomerEmail bool   `json:"update_customer_email"`
}

type SendSupportConversationTranscriptResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Email   string `json:"email"`
}

// ── WS Message Types ─────────────────────────────────────────────────

// WidgetWSMessage is the envelope for all widget WS messages.
type WidgetWSMessage struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

// WidgetSessionCreateData is the payload for session:create.
type WidgetSessionCreateData struct {
	AnonymousID string `json:"anonymous_id"`
	PageURL     string `json:"page_url"`
	PageTitle   string `json:"page_title"`
	UserAgent   string `json:"user_agent"`
	Timezone    string `json:"timezone"`
	Locale      string `json:"locale"`
}

// WidgetSessionRestoreData is the payload for session:restore.
type WidgetSessionRestoreData struct {
	SessionToken string `json:"session_token"`
}

// WidgetSessionUpgradeData is the payload for session:upgrade.
type WidgetIdentityPayload struct {
	Email                string                      `json:"email"`
	Name                 string                      `json:"name"` // backward-compatible full-name input only
	FirstName            string                      `json:"first_name,omitempty"`
	LastName             string                      `json:"last_name,omitempty"`
	Phone                string                      `json:"phone,omitempty"`
	JobTitle             string                      `json:"job_title,omitempty"`
	ExternalUserID       string                      `json:"external_user_id,omitempty"`
	Source               string                      `json:"source"` // "widget_prechat", "sdk_identify", or "sdk_lead"
	Company              JSONB                       `json:"company,omitempty"`
	IdentityVerification *WidgetIdentityVerification `json:"identity_verification,omitempty"`
}

// WidgetIdentityVerification is a short-lived customer-server HMAC proof.
type WidgetIdentityVerification struct {
	Version   string `json:"version"`
	IssuedAt  int64  `json:"issued_at"`
	ExpiresAt int64  `json:"expires_at"`
	Signature string `json:"signature"`
}

func (p WidgetIdentityPayload) DisplayName() string {
	first := strings.TrimSpace(p.FirstName)
	last := strings.TrimSpace(p.LastName)
	if first != "" || last != "" {
		return strings.TrimSpace(strings.Join([]string{first, last}, " "))
	}
	return strings.TrimSpace(p.Name)
}

// WidgetSessionUpgradeData is the payload for session:upgrade.
type WidgetSessionUpgradeData = WidgetIdentityPayload

// WidgetIdentifyRequest is the HTTP payload for POST /api/widget/identify (headless SDK path).
type WidgetIdentifyRequest struct {
	APIKey      string `json:"api_key"`
	AnonymousID string `json:"anonymous_id"`
	WidgetIdentityPayload
}

// WidgetMessageSendData is the payload for message:send.
type WidgetMessageSendData struct {
	Content       string   `json:"content"`
	AttachmentIDs []string `json:"attachment_ids,omitempty"`
}

// WidgetTypingData is the payload for typing:start / typing:stop.
type WidgetTypingData struct {
	Content string `json:"content,omitempty"`
}

// WidgetConversationSelectData is the payload for conversation:select.
type WidgetConversationSelectData struct {
	ConversationID string `json:"conversation_id"`
}

type WidgetActiveTeammate struct {
	UserID    string  `json:"user_id"`
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatar_url,omitempty"`
	Status    string  `json:"status,omitempty"`
}

// WidgetSessionJoinedPayload is sent to the client after session:create or session:restore.
type WidgetSessionJoinedPayload struct {
	SessionToken   string                `json:"session_token"`
	ExpiresAt      string                `json:"expires_at"`
	IsAnonymous    bool                  `json:"is_anonymous"`
	CustomerEmail  string                `json:"customer_email,omitempty"`
	Conversations  []SupportConversation `json:"conversations"`
	Messages       []SupportMessage      `json:"messages"`
	ActiveTeammate *WidgetActiveTeammate `json:"active_teammate,omitempty"`
}

// WidgetMessageReceivedPayload is sent to widget clients for new messages.
type WidgetMessageReceivedPayload struct {
	ID                        string                     `json:"id"`
	ClientMessageID           string                     `json:"client_message_id,omitempty"`
	ConversationID            string                     `json:"conversation_id"`
	Content                   string                     `json:"content"`
	SenderType                string                     `json:"sender_type"`
	MessageType               string                     `json:"message_type,omitempty"`
	SystemEventType           *string                    `json:"system_event_type,omitempty"`
	SenderName                *string                    `json:"sender_name"`
	SenderAvatar              *string                    `json:"sender_avatar"`
	Metadata                  *string                    `json:"metadata,omitempty"`
	ViaChannel                string                     `json:"via_channel,omitempty"`
	Attachments               []SupportAttachmentPayload `json:"attachments,omitempty"`
	EmailVisibleText          string                     `json:"email_visible_text,omitempty"`
	EmailQuotedText           string                     `json:"email_quoted_text,omitempty"`
	EmailHasQuotedContent     *bool                      `json:"email_has_quoted_content,omitempty"`
	EmailProjectionConfidence string                     `json:"email_projection_confidence,omitempty"`
	EmailProjectionVersion    int                        `json:"email_projection_version,omitempty"`
	CreatedAt                 string                     `json:"created_at"`
}

// CannedResponseRequest is the payload for CRUD operations on canned responses.
type CannedResponseRequest struct {
	ShortCode string  `json:"short_code"`
	Content   string  `json:"content"`
	Tag       *string `json:"tag,omitempty"`
}

// TypingIndicatorRequest represents a typing indicator event.
type TypingIndicatorRequest struct {
	IsTyping bool   `json:"is_typing"`
	Content  string `json:"content,omitempty"`
}

// ViewingPresenceRequest represents a viewing presence event.
type ViewingPresenceRequest struct {
	Viewing bool `json:"viewing"`
}

// CsatSurveyRequest represents a CSAT rating submission.
type CsatSurveyRequest struct {
	Rating   int    `json:"rating"`
	Feedback string `json:"feedback"`
}

// BusinessHoursDay defines a single day's business hours.
type BusinessHoursDay struct {
	Start   string `json:"start"` // "09:00"
	End     string `json:"end"`   // "17:00"
	Enabled bool   `json:"enabled"`
}

const (
	// SupportAIPreRouterModeOff is retained for persisted-settings compatibility.
	SupportAIPreRouterModeOff = "off"
	// SupportAIPreRouterModeShadow is retained for rollout telemetry compatibility.
	SupportAIPreRouterModeShadow = "shadow"
	// SupportAIPreRouterModeEnabled runs the LLM pre-router for every AI-handled message.
	SupportAIPreRouterModeEnabled = "enabled"
)

// SupportInboxSettings holds all widget configuration stored as JSONB.
type SupportInboxSettings struct {
	// Identity Capture
	RequireEmailBeforeChat bool   `json:"require_email_before_chat"`
	RequirePhoneAfterEmail bool   `json:"require_phone_after_email"`
	WelcomeMessage         string `json:"welcome_message"`

	// CRM Integration
	AutoCreateCRMContact  bool   `json:"auto_create_crm_contact"`
	DefaultLifecycleStage string `json:"default_lifecycle_stage"` // subscriber, lead, opportunity
	AutoPromoteToLead     bool   `json:"auto_promote_to_lead"`

	// AI Auto-Reply
	AIEnabled                    bool    `json:"ai_enabled"`
	AIAgentID                    *string `json:"ai_agent_id"`
	AIConfidenceThreshold        float64 `json:"ai_confidence_threshold"` // 0.0–1.0
	AIResponseMode               string  `json:"ai_response_mode"`        // "ai_first" | "internal_note" | "off"
	AIPreRouterMode              string  `json:"ai_pre_router_mode"`      // "off" | "shadow" | "enabled"
	AIMaxFollowups               int     `json:"ai_max_followups"`        // max stalled same-issue AI attempts before forced handoff (default: 5)
	AIFollowUpEnabled            bool    `json:"ai_follow_up_enabled"`
	AIFollowUpSecondDelayHours   int     `json:"ai_follow_up_second_delay_hours"`
	AIFollowUpDelayHours         int     `json:"ai_follow_up_delay_hours"`
	AIFollowUpCloseHours         int     `json:"ai_follow_up_close_hours"`
	AIFollowUpMaxPerConversation int     `json:"ai_follow_up_max_per_conversation"`
	AIAutoResolveTimeout         int     `json:"ai_auto_resolve_timeout"` // hours before assumed resolution (default: 24, 0 = disabled)
	ShowTalkToHuman              bool    `json:"show_talk_to_human"`

	// Escalation
	DelayedTeamReplyMinutes        int    `json:"delayed_team_reply_minutes"`
	DelayedTeamReplyMessage        string `json:"delayed_team_reply_message"`
	DelayedTeamReplyMessageNoEmail string `json:"delayed_team_reply_message_no_email"`
	EscalationMessage              string `json:"escalation_message"`             // message shown when AI hands off to human (live)
	EscalationMessageBusy          string `json:"escalation_message_busy"`        // message shown when AI hands off while team is busy
	EscalationMessageAfterHours    string `json:"escalation_message_after_hours"` // message shown when AI hands off after hours

	// Handoff Routing
	HandoffBehavior    string  `json:"handoff_behavior"` // unassigned, assign_to_team, round_robin
	HandoffTeamID      *string `json:"handoff_team_id"`
	DefaultMailboxID   *string `json:"default_mailbox_id"`
	AIHandoffMailboxID *string `json:"ai_handoff_mailbox_id"`

	// Conversation triage
	TriageEnabled                 bool    `json:"triage_enabled"`
	TriageAutoMoveEnabled         bool    `json:"triage_auto_move_enabled"`
	TriageConfidenceThreshold     float64 `json:"triage_confidence_threshold"`
	TriageWidgetEnabled           bool    `json:"triage_widget_enabled"`
	TriageEmailEnabled            bool    `json:"triage_email_enabled"`
	TriageInternalEnabled         bool    `json:"triage_internal_enabled"`
	TriageFallbackBehavior        string  `json:"triage_fallback_behavior"` // shared, default
	TriageRerunOnMeaningChange    bool    `json:"triage_rerun_on_meaning_change"`
	TriageDailyBudget             int     `json:"triage_daily_budget"`
	TriageSkipSpamConversations   bool    `json:"triage_skip_spam_conversations"`
	TriageDeduplicateFirstMessage bool    `json:"triage_deduplicate_first_message"`

	// Business Hours
	BusinessHoursEnabled  bool                        `json:"business_hours_enabled"`
	BusinessHoursTimezone string                      `json:"business_hours_timezone"` // IANA
	BusinessHoursSchedule map[string]BusinessHoursDay `json:"business_hours_schedule"` // mon-sun
	OutsideHoursMessage   string                      `json:"outside_hours_message"`

	// Reply-time expectations rendered on the widget during business hours.
	// Preset drives the copy; ReplyTimeCustomMinutes is only honored when
	// ReplyTimePreset == "custom". See SupportReplyTimePreset* constants.
	ReplyTimePreset        string `json:"reply_time_preset"`
	ReplyTimeCustomMinutes *int   `json:"reply_time_custom_minutes,omitempty"`

	// Optional workspace-wide notice rendered as a slim banner above
	// conversation surfaces (outages, backlog, maintenance). Empty string
	// or nil means the banner is hidden.
	SpecialNoticeText *string `json:"special_notice_text,omitempty"`

	// Offline email fallback
	EmailFallbackEnabled            bool   `json:"email_fallback_enabled"`
	EmailFallbackDelaySecs          int    `json:"email_fallback_delay_secs"`
	EmailFallbackFromName           string `json:"email_fallback_from_name"`
	EmailFallbackMaxDeliveryAgeSecs int    `json:"email_fallback_max_delivery_age_secs"`
	ForwardedEmailDetectionEnabled  bool   `json:"forwarded_email_detection_enabled"`
	ForwardedEmailDetectionMode     string `json:"forwarded_email_detection_mode"`
	ForwardedEmailMinConfidence     int    `json:"forwarded_email_min_confidence"`

	// Widget Identity
	WidgetName         string   `json:"widget_name"`           // display name in widget header (defaults to workspace name)
	WidgetAvatarURL    string   `json:"widget_avatar_url"`     // custom avatar URL for the widget
	WidgetHelpSpaceIDs []string `json:"widget_help_space_ids"` // selected external-capable docs spaces shown in widget help

	// Branding
	BrandColor      string `json:"brand_color"` // hex "#6366F1"
	ShowBranding    bool   `json:"show_branding"`
	ColorScheme     string `json:"color_scheme"`      // system, light, dark
	ButtonColor     string `json:"button_color"`      // hex "#000000"
	ButtonIconColor string `json:"button_icon_color"` // hex "#FFFFFF"
	LogoURL         string `json:"logo_url"`          // URL to workspace logo for widget header

	// Launcher
	LauncherPosition string `json:"launcher_position"` // bottom_right, bottom_left
	LauncherIcon     string `json:"launcher_icon"`     // chat_bubble, question_mark, help

	// CSAT
	CSATEnabled bool `json:"csat_enabled"`

	// File Uploads
	FileUploadsEnabled bool `json:"file_uploads_enabled"`

	// Email Transcript
	ForceVisitorIdentity bool `json:"force_visitor_identity"`
}

type SupportRoutingUsageStatus struct {
	TriageEnabled  bool      `json:"triage_enabled"`
	DailyBudget    int       `json:"daily_budget"`
	UsedToday      int       `json:"used_today"`
	RemainingToday *int      `json:"remaining_today,omitempty"`
	ResetAt        time.Time `json:"reset_at"`
	Exhausted      bool      `json:"exhausted"`
}

// DefaultSupportInboxSettings returns settings with sensible defaults.
func DefaultSupportInboxSettings() SupportInboxSettings {
	return SupportInboxSettings{
		RequireEmailBeforeChat:         true,
		RequirePhoneAfterEmail:         false,
		WelcomeMessage:                 "Hi there! How can we help you today?",
		AutoCreateCRMContact:           true,
		DefaultLifecycleStage:          "subscriber",
		AutoPromoteToLead:              false,
		AIEnabled:                      false,
		AIAgentID:                      nil,
		AIConfidenceThreshold:          0.7,
		AIResponseMode:                 "ai_first",
		AIPreRouterMode:                SupportAIPreRouterModeEnabled,
		AIMaxFollowups:                 5,
		AIFollowUpEnabled:              true,
		AIFollowUpSecondDelayHours:     24,
		AIFollowUpDelayHours:           24,
		AIFollowUpCloseHours:           1,
		AIFollowUpMaxPerConversation:   2,
		AIAutoResolveTimeout:           24,
		ShowTalkToHuman:                true,
		DelayedTeamReplyMinutes:        5,
		DelayedTeamReplyMessage:        "Looks like our team needs a little more time. We’ll reply in this chat and email you if you miss it. Thanks for your patience.",
		DelayedTeamReplyMessageNoEmail: "Our team hasn’t been able to reply yet. Leave your email and we’ll notify you when someone responds, so you don’t have to wait here.",
		EscalationMessage:              "Let me connect you with a team member — they typically reply in {reply_time}.",
		EscalationMessageBusy:          "I've notified the team. Everyone's helping other customers right now — expect a reply within {reply_time}.",
		EscalationMessageAfterHours:    "I've passed this on to the team. We're away right now and back {next_open}.",
		HandoffBehavior:                "unassigned",
		HandoffTeamID:                  nil,
		DefaultMailboxID:               nil,
		AIHandoffMailboxID:             nil,
		TriageEnabled:                  true,
		TriageAutoMoveEnabled:          true,
		TriageConfidenceThreshold:      0.8,
		TriageWidgetEnabled:            true,
		TriageEmailEnabled:             true,
		TriageInternalEnabled:          false,
		TriageFallbackBehavior:         "shared",
		TriageRerunOnMeaningChange:     false,
		TriageDailyBudget:              250,
		TriageSkipSpamConversations:    true,
		TriageDeduplicateFirstMessage:  true,
		BusinessHoursEnabled:           false,
		BusinessHoursTimezone:          "America/New_York",
		BusinessHoursSchedule: map[string]BusinessHoursDay{
			"mon": {Start: "09:00", End: "17:00", Enabled: true},
			"tue": {Start: "09:00", End: "17:00", Enabled: true},
			"wed": {Start: "09:00", End: "17:00", Enabled: true},
			"thu": {Start: "09:00", End: "17:00", Enabled: true},
			"fri": {Start: "09:00", End: "17:00", Enabled: true},
			"sat": {Start: "09:00", End: "17:00", Enabled: false},
			"sun": {Start: "09:00", End: "17:00", Enabled: false},
		},
		OutsideHoursMessage:             "We're currently offline. Leave a message and we'll get back to you!",
		ReplyTimePreset:                 SupportReplyTimePresetFewMinutes,
		ReplyTimeCustomMinutes:          nil,
		SpecialNoticeText:               nil,
		EmailFallbackEnabled:            true,
		EmailFallbackDelaySecs:          10,
		EmailFallbackFromName:           "",
		EmailFallbackMaxDeliveryAgeSecs: 600,
		ForwardedEmailDetectionEnabled:  true,
		ForwardedEmailDetectionMode:     "high_confidence_any_sender",
		ForwardedEmailMinConfidence:     80,
		WidgetName:                      "",
		WidgetAvatarURL:                 "",
		WidgetHelpSpaceIDs:              []string{},
		BrandColor:                      "#6366F1",
		ShowBranding:                    true,
		ColorScheme:                     "light",
		ButtonColor:                     "#000000",
		ButtonIconColor:                 "#FFFFFF",
		LogoURL:                         "",
		LauncherPosition:                "bottom_right",
		LauncherIcon:                    "chat_bubble",
		CSATEnabled:                     false,
		FileUploadsEnabled:              true,
		ForceVisitorIdentity:            false,
	}
}

// UpdateInstallationSettingsRequest is a PATCH payload with pointer fields.
type UpdateInstallationSettingsRequest struct {
	AllowedOrigins                  *[]string                   `json:"allowed_origins,omitempty"`
	IdentityVerificationMode        *string                     `json:"identity_verification_mode,omitempty"`
	RequireEmailBeforeChat          *bool                       `json:"require_email_before_chat,omitempty"`
	RequirePhoneAfterEmail          *bool                       `json:"require_phone_after_email,omitempty"`
	WelcomeMessage                  *string                     `json:"welcome_message,omitempty"`
	AutoCreateCRMContact            *bool                       `json:"auto_create_crm_contact,omitempty"`
	DefaultLifecycleStage           *string                     `json:"default_lifecycle_stage,omitempty"`
	AutoPromoteToLead               *bool                       `json:"auto_promote_to_lead,omitempty"`
	AIEnabled                       *bool                       `json:"ai_enabled,omitempty"`
	AIAgentID                       *string                     `json:"ai_agent_id,omitempty"`
	AIConfidenceThreshold           *float64                    `json:"ai_confidence_threshold,omitempty"`
	AIResponseMode                  *string                     `json:"ai_response_mode,omitempty"`
	AIPreRouterMode                 *string                     `json:"ai_pre_router_mode,omitempty"`
	AIMaxFollowups                  *int                        `json:"ai_max_followups,omitempty"`
	AIFollowUpEnabled               *bool                       `json:"ai_follow_up_enabled,omitempty"`
	AIFollowUpSecondDelayHours      *int                        `json:"ai_follow_up_second_delay_hours,omitempty"`
	AIFollowUpDelayHours            *int                        `json:"ai_follow_up_delay_hours,omitempty"`
	AIFollowUpCloseHours            *int                        `json:"ai_follow_up_close_hours,omitempty"`
	AIFollowUpMaxPerConversation    *int                        `json:"ai_follow_up_max_per_conversation,omitempty"`
	AIAutoResolveTimeout            *int                        `json:"ai_auto_resolve_timeout,omitempty"`
	ShowTalkToHuman                 *bool                       `json:"show_talk_to_human,omitempty"`
	DelayedTeamReplyMinutes         *int                        `json:"delayed_team_reply_minutes,omitempty"`
	DelayedTeamReplyMessage         *string                     `json:"delayed_team_reply_message,omitempty"`
	DelayedTeamReplyMessageNoEmail  *string                     `json:"delayed_team_reply_message_no_email,omitempty"`
	EscalationMessage               *string                     `json:"escalation_message,omitempty"`
	EscalationMessageBusy           *string                     `json:"escalation_message_busy,omitempty"`
	EscalationMessageAfterHours     *string                     `json:"escalation_message_after_hours,omitempty"`
	HandoffBehavior                 *string                     `json:"handoff_behavior,omitempty"`
	HandoffTeamID                   *string                     `json:"handoff_team_id,omitempty"`
	DefaultMailboxID                *string                     `json:"default_mailbox_id,omitempty"`
	AIHandoffMailboxID              *string                     `json:"ai_handoff_mailbox_id,omitempty"`
	TriageEnabled                   *bool                       `json:"triage_enabled,omitempty"`
	TriageAutoMoveEnabled           *bool                       `json:"triage_auto_move_enabled,omitempty"`
	TriageConfidenceThreshold       *float64                    `json:"triage_confidence_threshold,omitempty"`
	TriageWidgetEnabled             *bool                       `json:"triage_widget_enabled,omitempty"`
	TriageEmailEnabled              *bool                       `json:"triage_email_enabled,omitempty"`
	TriageInternalEnabled           *bool                       `json:"triage_internal_enabled,omitempty"`
	TriageFallbackBehavior          *string                     `json:"triage_fallback_behavior,omitempty"`
	TriageRerunOnMeaningChange      *bool                       `json:"triage_rerun_on_meaning_change,omitempty"`
	TriageDailyBudget               *int                        `json:"triage_daily_budget,omitempty"`
	TriageSkipSpamConversations     *bool                       `json:"triage_skip_spam_conversations,omitempty"`
	TriageDeduplicateFirstMessage   *bool                       `json:"triage_deduplicate_first_message,omitempty"`
	BusinessHoursEnabled            *bool                       `json:"business_hours_enabled,omitempty"`
	BusinessHoursTimezone           *string                     `json:"business_hours_timezone,omitempty"`
	BusinessHoursSchedule           map[string]BusinessHoursDay `json:"business_hours_schedule,omitempty"`
	OutsideHoursMessage             *string                     `json:"outside_hours_message,omitempty"`
	ReplyTimePreset                 *string                     `json:"reply_time_preset,omitempty"`
	ReplyTimeCustomMinutes          *int                        `json:"reply_time_custom_minutes,omitempty"`
	SpecialNoticeText               *string                     `json:"special_notice_text,omitempty"`
	ClearSpecialNotice              *bool                       `json:"clear_special_notice,omitempty"`
	ClearReplyTimeCustomMinutes     *bool                       `json:"clear_reply_time_custom_minutes,omitempty"`
	EmailFallbackEnabled            *bool                       `json:"email_fallback_enabled,omitempty"`
	EmailFallbackDelaySecs          *int                        `json:"email_fallback_delay_secs,omitempty"`
	EmailFallbackFromName           *string                     `json:"email_fallback_from_name,omitempty"`
	EmailFallbackMaxDeliveryAgeSecs *int                        `json:"email_fallback_max_delivery_age_secs,omitempty"`
	ForwardedEmailDetectionEnabled  *bool                       `json:"forwarded_email_detection_enabled,omitempty"`
	ForwardedEmailDetectionMode     *string                     `json:"forwarded_email_detection_mode,omitempty"`
	ForwardedEmailMinConfidence     *int                        `json:"forwarded_email_min_confidence,omitempty"`
	WidgetName                      *string                     `json:"widget_name,omitempty"`
	WidgetAvatarURL                 *string                     `json:"widget_avatar_url,omitempty"`
	WidgetHelpSpaceIDs              []string                    `json:"widget_help_space_ids,omitempty"`
	BrandColor                      *string                     `json:"brand_color,omitempty"`
	ShowBranding                    *bool                       `json:"show_branding,omitempty"`
	ColorScheme                     *string                     `json:"color_scheme,omitempty"`
	ButtonColor                     *string                     `json:"button_color,omitempty"`
	ButtonIconColor                 *string                     `json:"button_icon_color,omitempty"`
	LogoURL                         *string                     `json:"logo_url,omitempty"`
	LauncherPosition                *string                     `json:"launcher_position,omitempty"`
	LauncherIcon                    *string                     `json:"launcher_icon,omitempty"`
	CSATEnabled                     *bool                       `json:"csat_enabled,omitempty"`
	FileUploadsEnabled              *bool                       `json:"file_uploads_enabled,omitempty"`
	ForceVisitorIdentity            *bool                       `json:"force_visitor_identity,omitempty"`
}

// SupportAIPreviewRequest is a dry-run request for the support AI planner + RAG pipeline.
type SupportAIPreviewRequest struct {
	Message        string                        `json:"message"`
	ConversationID *string                       `json:"conversation_id,omitempty"`
	History        []SupportAIPreviewHistoryTurn `json:"history,omitempty"`
	IncludeAnswer  *bool                         `json:"include_answer,omitempty"`
	MaxResults     *int                          `json:"max_results,omitempty"`
}

// SupportAIRewriteDraftRequest rewrites a human-authored support draft with a targeted transform.
type SupportAIRewriteDraftRequest struct {
	Content   string `json:"content"`
	Operation string `json:"operation"`
}

// SupportAIRewriteDraftResponse returns the rewritten draft plus model metadata.
type SupportAIRewriteDraftResponse struct {
	Content   string `json:"content"`
	Operation string `json:"operation"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
}

// SupportAIPreviewHistoryTurn is a simplified conversation turn used for preview requests.
type SupportAIPreviewHistoryTurn struct {
	SenderType  string `json:"sender_type"`
	MessageType string `json:"message_type,omitempty"`
	Content     string `json:"content"`
}

// SupportAIPreviewResponse is the structured dry-run response for support AI previewing.
type SupportAIPreviewResponse struct {
	ConversationSource  string                    `json:"conversation_source"`
	ConfidenceThreshold float64                   `json:"confidence_threshold"`
	TotalTokensUsed     int                       `json:"total_tokens_used"`
	FinalDecision       string                    `json:"final_decision"`
	FinalReason         string                    `json:"final_reason"`
	QueryPlan           SupportAIPreviewQueryPlan `json:"query_plan"`
	Retrieval           SupportAIPreviewRetrieval `json:"retrieval"`
	Answer              *SupportAIPreviewAnswer   `json:"answer,omitempty"`
}

type SupportAIPreviewQueryPlan struct {
	Route              string   `json:"route"`
	Decision           string   `json:"decision"`
	Intent             string   `json:"intent"`
	Subject            string   `json:"subject"`
	Language           string   `json:"language"`
	Risk               string   `json:"risk"`
	RequiredEvidence   []string `json:"required_evidence"`
	EvidenceMode       string   `json:"evidence_mode"`
	RegistryVersion    int      `json:"registry_version"`
	ContextAction      string   `json:"context_action"`
	IssueKey           string   `json:"issue_key"`
	IssueSummary       string   `json:"issue_summary"`
	ProgressSignal     string   `json:"progress_signal"`
	StandaloneQuery    string   `json:"standalone_query"`
	SearchQueries      []string `json:"search_queries"`
	ClarifyingQuestion string   `json:"clarifying_question"`
	GreetingReply      string   `json:"greeting_reply"`
	Reason             string   `json:"reason"`
	TokensUsed         int      `json:"tokens_used"`
	FallbackUsed       bool     `json:"fallback_used"`
	Error              string   `json:"error,omitempty"`
}

type SupportAIPreviewRetrieval struct {
	QueryCount      int                            `json:"query_count"`
	ResultCount     int                            `json:"result_count"`
	EvidenceFound   map[string][]string            `json:"evidence_found"`
	EvidenceMissing []string                       `json:"evidence_missing"`
	TargetedRetry   bool                           `json:"targeted_retry"`
	Results         []SupportAIPreviewSearchResult `json:"results"`
	Error           string                         `json:"error,omitempty"`
}

type SupportAIPreviewSearchResult struct {
	ReferenceID   string  `json:"reference_id"`
	SourceType    string  `json:"source_type"`
	Title         string  `json:"title"`
	URL           string  `json:"url,omitempty"`
	ChunkIndex    int     `json:"chunk_index"`
	CombinedScore float64 `json:"combined_score"`
	VectorScore   float64 `json:"vector_score"`
	LexicalScore  float64 `json:"lexical_score"`
	Snippet       string  `json:"snippet"`
}

type SupportAIPreviewAnswer struct {
	Content            string   `json:"content"`
	CanAnswer          bool     `json:"can_answer"`
	SourceDocIDs       []string `json:"source_doc_ids"`
	LLMConfidence      float64  `json:"llm_confidence"`
	GroundedConfidence float64  `json:"grounded_confidence"`
	TokensUsed         int      `json:"tokens_used"`
	Provider           string   `json:"provider"`
	Model              string   `json:"model"`
	ValidationOutcome  string   `json:"validation_outcome"`
	ValidationReasons  []string `json:"validation_reasons"`
	MaterialClaims     int      `json:"material_claims"`
	SupportedClaims    int      `json:"supported_claims"`
}

// WidgetToken is the token format expected by the events-pipeline rust-capture service.
type WidgetToken struct {
	ID                       string   `json:"id"`
	WorkspaceID              string   `json:"workspace_id"`
	ClientSecret             string   `json:"client_secret"`
	ServerSecret             string   `json:"server_secret"`
	Origins                  []string `json:"origins"`
	IdentityVerificationMode string   `json:"identity_verification_mode"`
}

// WidgetTokensResponse wraps the token list for the HTTP response.
type WidgetTokensResponse struct {
	Tokens []WidgetToken `json:"tokens"`
}

// WidgetConfigBranding matches the widget-core WidgetConfig.branding shape.
type WidgetConfigBranding struct {
	PrimaryColor    string `json:"primaryColor"`
	LogoURL         string `json:"logoUrl,omitempty"`
	WelcomeMessage  string `json:"welcomeMessage"`
	WidgetPosition  string `json:"widgetPosition"`
	ShowBranding    bool   `json:"showBranding"`
	LauncherIcon    string `json:"launcherIcon,omitempty"`
	ColorScheme     string `json:"colorScheme,omitempty"`
	ButtonColor     string `json:"buttonColor,omitempty"`
	ButtonIconColor string `json:"buttonIconColor,omitempty"`
}

// WidgetConfigFeatures matches the widget-core WidgetConfig.features shape.
type WidgetConfigFeatures struct {
	AIEnabled         bool   `json:"aiEnabled"`
	AIFirst           bool   `json:"aiFirst"`
	ShowTalkToHuman   bool   `json:"showTalkToHuman"`
	EscalationMessage string `json:"escalationMessage,omitempty"`
	FileUploads       bool   `json:"fileUploads"`
	PreChatForm       bool   `json:"preChatForm"`
	RequirePhone      bool   `json:"requirePhone"`
	CSATRating        bool   `json:"csatRating"`
	ForceIdentify     bool   `json:"forceIdentify"`
}

// WidgetConfigAvailability matches the widget-core WidgetConfig.availability shape.
type WidgetConfigAvailability struct {
	IsOnline            bool    `json:"isOnline"`
	StatusText          string  `json:"statusText"`
	ReplyTimeText       string  `json:"replyTimeText"`
	OutsideHoursMessage *string `json:"outsideHoursMessage,omitempty"`
	NextOnlineAt        *string `json:"nextOnlineAt,omitempty"`

	// Structured reply-time expectation so the widget can choose to render
	// its own copy (e.g. localized) instead of relying on ReplyTimeText.
	// ReplyTimeMinutes is set only when the preset is "custom".
	ReplyTimePreset  string `json:"replyTimePreset,omitempty"`
	ReplyTimeMinutes *int   `json:"replyTimeMinutes,omitempty"`

	// SpecialNoticeText drives the slim amber banner above conversation
	// surfaces. Nil or empty means the banner is hidden.
	SpecialNoticeText *string `json:"specialNoticeText,omitempty"`

	// MailboxID is set when the effective preset came from a mailbox
	// override rather than the workspace default. Widget can surface this
	// for debugging / preview purposes but does not render it today.
	MailboxID *string `json:"mailboxId,omitempty"`
}

// WidgetHelpSpace is an external-capable docs space exposed to the widget help tab.
type WidgetHelpSpace struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	Slug string  `json:"slug"`
	Icon *string `json:"icon,omitempty"`
}

// WidgetHelpCollection is a widget help collection row. ParentCollectionID
// and Depth let the widget renderer build a nested drilldown from a flat
// response. ArticleCount counts only direct articles in this collection,
// not descendants.
type WidgetHelpCollection struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Slug               string  `json:"slug"`
	PublicID           string  `json:"public_id"`
	Icon               *string `json:"icon,omitempty"`
	ParentCollectionID *string `json:"parent_collection_id"`
	Depth              int     `json:"depth"`
	ArticleCount       int     `json:"article_count"`
}

// WidgetHelpArticleSummary is a widget help article list row.
type WidgetHelpArticleSummary struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Slug       string  `json:"slug"`
	PublicID   string  `json:"public_id"`
	ArticleKey string  `json:"article_key"`
	Excerpt    *string `json:"excerpt,omitempty"`
	Icon       *string `json:"icon,omitempty"`
}

// WidgetHelpSearchResult is a widget-scoped help center search hit.
type WidgetHelpSearchResult struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	Slug           string  `json:"slug"`
	PublicID       string  `json:"public_id"`
	ArticleKey     string  `json:"article_key"`
	Excerpt        *string `json:"excerpt,omitempty"`
	CollectionName *string `json:"collection_name,omitempty"`
	SpaceName      string  `json:"space_name,omitempty"`
}

// WidgetHelpArticle is a widget help article detail response.
type WidgetHelpArticle struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Slug        string  `json:"slug"`
	PublicID    string  `json:"public_id"`
	ArticleKey  string  `json:"article_key"`
	Excerpt     *string `json:"excerpt,omitempty"`
	Icon        *string `json:"icon,omitempty"`
	ContentHTML *string `json:"content_html"`
	PublicPath  *string `json:"public_path,omitempty"`
}

// WidgetConfigResponse is the public-facing widget config matching the
// TypeScript WidgetConfig interface in packages/shared/src/types/widget-config.ts.
type WidgetConfigResponse struct {
	WorkspaceID        string                   `json:"workspaceId"`
	WorkspaceName      string                   `json:"workspaceName,omitempty"`
	Branding           WidgetConfigBranding     `json:"branding"`
	Features           WidgetConfigFeatures     `json:"features"`
	Availability       WidgetConfigAvailability `json:"availability"`
	AvailableTeammates []WidgetActiveTeammate   `json:"availableTeammates,omitempty"`
	HelpSpaces         []WidgetHelpSpace        `json:"helpSpaces"`
}

// ── Visitor Context DTOs ─────────────────────────────────────────────

// VisitorDeviceInfo holds parsed user-agent data for visitor context.
type VisitorDeviceInfo struct {
	Browser        string `json:"browser"`
	BrowserVersion string `json:"browser_version"`
	OS             string `json:"os"`
	OSVersion      string `json:"os_version"`
	DeviceType     string `json:"device_type"` // desktop, mobile, tablet
}

// VisitorLocation holds geographic/locale data for visitor context.
type VisitorLocation struct {
	Timezone    *string `json:"timezone"`
	Locale      *string `json:"locale"`
	LastPageURL *string `json:"last_page_url"`
	CountryCode *string `json:"country_code,omitempty"`
	CountryName *string `json:"country_name,omitempty"`
	RegionName  *string `json:"region_name,omitempty"`
	CityName    *string `json:"city_name,omitempty"`
}

// VisitorContactData holds CRM contact details for visitor context.
type VisitorContactData struct {
	ID               string         `json:"id"`
	Name             *string        `json:"name"`
	Email            *string        `json:"email"`
	Phone            *string        `json:"phone"`
	JobTitle         *string        `json:"job_title"`
	LifecycleStage   string         `json:"lifecycle_stage"`
	LeadStatus       string         `json:"lead_status"`
	Source           string         `json:"source"`
	CustomProperties map[string]any `json:"custom_properties,omitempty"`
}

type VisitorCompanyContextStatus string

const (
	VisitorCompanyContextOK       VisitorCompanyContextStatus = "ok"
	VisitorCompanyContextUnlinked VisitorCompanyContextStatus = "unlinked"
	VisitorCompanyContextError    VisitorCompanyContextStatus = "error"
)

// VisitorCompanyData is the live CRM company linked directly to a conversation.
type VisitorCompanyData struct {
	ID               string         `json:"id"`
	DisplayID        string         `json:"display_id"`
	ExternalID       *string        `json:"external_id,omitempty"`
	Name             string         `json:"name"`
	Domain           *string        `json:"domain,omitempty"`
	Industry         *string        `json:"industry,omitempty"`
	EmployeeCount    *int           `json:"employee_count,omitempty"`
	AnnualRevenue    *float64       `json:"annual_revenue,omitempty"`
	Description      *string        `json:"description,omitempty"`
	LogoURL          *string        `json:"logo_url,omitempty"`
	CustomProperties map[string]any `json:"custom_properties"`
	UpdatedAt        string         `json:"updated_at"`
}

// VisitorCompanyOption is a compact company membership selector option.
type VisitorCompanyOption struct {
	ID        string  `json:"id"`
	DisplayID string  `json:"display_id"`
	Name      string  `json:"name"`
	Domain    *string `json:"domain,omitempty"`
	LogoURL   *string `json:"logo_url,omitempty"`
}

// VisitorOtherConversation is a compact summary for other conversations in visitor context.
type VisitorOtherConversation struct {
	ID        string `json:"id"`
	DisplayID int    `json:"display_id"`
	Subject   string `json:"subject"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// VisitorContextResponse assembles all visitor intelligence for a conversation.
type VisitorContextResponse struct {
	Device               *VisitorDeviceInfo          `json:"device,omitempty"`
	Location             *VisitorLocation            `json:"location,omitempty"`
	Contact              *VisitorContactData         `json:"contact,omitempty"`
	Company              *VisitorCompanyData         `json:"company,omitempty"`
	CompanyOptions       []VisitorCompanyOption      `json:"company_options,omitempty"`
	CompanyContextStatus VisitorCompanyContextStatus `json:"company_context_status"`
	OtherConversations   []VisitorOtherConversation  `json:"other_conversations"`
	TotalConversations   int                         `json:"total_conversations"`
	SessionCreatedAt     *string                     `json:"session_created_at,omitempty"`
	LastActiveAt         *string                     `json:"last_active_at,omitempty"`
	LastActiveSource     *string                     `json:"last_active_source,omitempty"`
}

// InstallationSettingsResponse wraps installation + parsed settings for the admin API.
type InstallationSettingsResponse struct {
	ID                       string               `json:"id"`
	WorkspaceID              string               `json:"workspace_id"`
	WidgetKey                string               `json:"widget_key"`
	AllowedOrigins           []string             `json:"allowed_origins"`
	IdentityVerificationMode string               `json:"identity_verification_mode"`
	Settings                 SupportInboxSettings `json:"settings"`
	Active                   bool                 `json:"active"`
	CreatedAt                string               `json:"created_at"`
	UpdatedAt                string               `json:"updated_at"`
}

// RotateWidgetSecretResponse returns a newly rotated secret exactly once.
type RotateWidgetSecretResponse struct {
	SecretKey string `json:"secret_key"`
	RotatedAt string `json:"rotated_at"`
}
