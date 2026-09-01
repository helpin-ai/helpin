package model

import (
	"encoding/json"
	"time"
)

// CRM email providers.
const (
	CRMEmailProviderGmail     = "gmail"
	CRMEmailProviderMicrosoft = "microsoft"
)

// CRM email directions.
const (
	CRMEmailDirectionInbound  = "inbound"
	CRMEmailDirectionOutbound = "outbound"
)

// CRM email account statuses.
const (
	CRMEmailAccountStatusPendingOAuth = "pending_oauth"
	CRMEmailAccountStatusConnected    = "connected"
	CRMEmailAccountStatusDisconnected = "disconnected"
	CRMEmailAccountStatusError        = "error"
)

// CRM email sync command modes.
const (
	CRMEmailSyncModeIncremental = "incremental"
	CRMEmailSyncModeHistorical  = "historical"
)

// CRM email participant roles.
const (
	CRMEmailParticipantRoleFrom   = "from"
	CRMEmailParticipantRoleTo     = "to"
	CRMEmailParticipantRoleCC     = "cc"
	CRMEmailParticipantRoleManual = "manual"
)

// CRMEmailAccount represents a connected email account for CRM sync.
type CRMEmailAccount struct {
	ID                     string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID            string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	MemberID               string     `json:"member_id" gorm:"type:uuid;not null;index"`
	Provider               string     `json:"provider" gorm:"not null;default:'gmail'"` // gmail, microsoft
	EmailAddress           string     `json:"email_address" gorm:"not null"`
	NormalizedEmailAddress *string    `json:"normalized_email_address"`
	AccessTokenEncrypted   *string    `json:"-" gorm:"column:access_token_encrypted"`
	RefreshTokenEncrypted  *string    `json:"-" gorm:"column:refresh_token_encrypted"`
	SyncState              JSONB      `json:"sync_state" gorm:"type:jsonb;default:'{}'"`
	LastHistoryID          *string    `json:"last_history_id"`
	LastSyncedAt           *time.Time `json:"last_synced_at"`
	IsActive               bool       `json:"is_active" gorm:"not null;default:true"`
	Status                 string     `json:"status" gorm:"not null;default:'connected'"`
	DisconnectedAt         *time.Time `json:"disconnected_at"`
	OAuthState             *string    `json:"-" gorm:"column:oauth_state"`
	TokenExpiresAt         *time.Time `json:"-" gorm:"column:token_expires_at"`
	HasSyncedData          bool       `json:"has_synced_data" gorm:"-"`
	CanSend                bool       `json:"can_send" gorm:"-"`
	CreatedAt              time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt              time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMEmailAccount) TableName() string { return "crm_email_accounts" }

// CRMEmailThread represents an email thread linked to CRM objects.
type CRMEmailThread struct {
	ID                  string           `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID         string           `json:"workspace_id" gorm:"type:uuid;not null;index"`
	EmailAccountID      string           `json:"email_account_id" gorm:"type:uuid;not null;index"`
	ThreadExternalID    string           `json:"thread_external_id" gorm:"not null"`
	Subject             string           `json:"subject" gorm:"not null"`
	LastMessageAt       time.Time        `json:"last_message_at" gorm:"not null"`
	MessageCount        int              `json:"message_count" gorm:"not null;default:0"`
	ContactIDs          json.RawMessage  `json:"contact_ids" gorm:"type:jsonb;default:'[]'"`
	DealID              *string          `json:"deal_id" gorm:"type:uuid;index"`
	CreatedAt           time.Time        `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time        `json:"updated_at" gorm:"autoUpdateTime"`
	LatestMessage       *CRMEmailMessage `json:"latest_message,omitempty" gorm:"-"`
	MailboxEmail        string           `json:"mailbox_email,omitempty" gorm:"-"`
	MailboxProvider     string           `json:"mailbox_provider,omitempty" gorm:"-"`
	MailboxStatus       string           `json:"mailbox_status,omitempty" gorm:"-"`
	MailboxLastSync     *time.Time       `json:"mailbox_last_synced_at,omitempty" gorm:"-"`
	CanReply            bool             `json:"can_reply" gorm:"-"`
	NeedsReply          bool             `json:"needs_reply" gorm:"-"`
	NeedsReplyDismissed bool             `json:"needs_reply_dismissed" gorm:"-"`
}

func (CRMEmailThread) TableName() string { return "crm_email_threads" }

// CRMEmailMessage represents a single email message in CRM.
type CRMEmailMessage struct {
	ID                string               `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID       string               `json:"workspace_id" gorm:"type:uuid;not null;index"`
	EmailAccountID    string               `json:"email_account_id" gorm:"type:uuid;not null;index"`
	ThreadID          *string              `json:"thread_id" gorm:"type:uuid;index"`
	MessageExternalID string               `json:"message_external_id"`
	RFCMessageID      *string              `json:"rfc_message_id,omitempty"`
	InReplyTo         *string              `json:"in_reply_to,omitempty"`
	ReferencesHeader  *string              `json:"references_header,omitempty"`
	FromAddress       string               `json:"from_address" gorm:"not null"`
	FromName          *string              `json:"from_name"`
	ToAddresses       json.RawMessage      `json:"to_addresses" gorm:"type:jsonb;default:'[]'"`
	CCAddresses       json.RawMessage      `json:"cc_addresses" gorm:"type:jsonb;default:'[]'"`
	Subject           string               `json:"subject"`
	BodyText          *string              `json:"body_text"`
	BodyHTML          *string              `json:"body_html"`
	Direction         string               `json:"direction" gorm:"not null;default:'inbound'"` // inbound, outbound
	SentAt            time.Time            `json:"sent_at" gorm:"not null"`
	ContactID         *string              `json:"contact_id" gorm:"type:uuid;index"`
	ContactIDs        []string             `json:"contact_ids" gorm:"-"`
	DealID            *string              `json:"deal_id" gorm:"type:uuid;index"`
	CreatedAt         time.Time            `json:"created_at" gorm:"autoCreateTime"`
	Attachments       []CRMEmailAttachment `json:"attachments,omitempty" gorm:"-"`
}

func (CRMEmailMessage) TableName() string { return "crm_email_messages" }

// CRMEmailMessageContact links email messages to all external CRM contacts that
// participated in the conversation.
type CRMEmailMessageContact struct {
	MessageID       string    `json:"message_id" gorm:"type:uuid;not null;primaryKey"`
	ContactID       string    `json:"contact_id" gorm:"type:uuid;not null;primaryKey"`
	ParticipantRole string    `json:"participant_role" gorm:"type:varchar(20);not null;primaryKey"`
	WorkspaceID     string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (CRMEmailMessageContact) TableName() string { return "crm_email_message_contacts" }

// CRMEmailThreadDismissal records a user's dismissal of an actionable inbound
// thread. A later inbound message naturally supersedes the dismissal.
type CRMEmailThreadDismissal struct {
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;primaryKey"`
	ThreadID    string    `json:"thread_id" gorm:"type:uuid;not null;primaryKey"`
	UserID      string    `json:"user_id" gorm:"type:uuid;not null;primaryKey"`
	DismissedAt time.Time `json:"dismissed_at" gorm:"not null"`
}

func (CRMEmailThreadDismissal) TableName() string { return "crm_email_thread_dismissals" }

type CRMEmailParticipant struct {
	Email       string  `json:"email"`
	Name        string  `json:"name,omitempty"`
	Role        string  `json:"role"`
	ContactID   *string `json:"contact_id,omitempty"`
	ContactName string  `json:"contact_name,omitempty"`
	CompanyID   *string `json:"company_id,omitempty"`
	CompanyName string  `json:"company_name,omitempty"`
}

type CRMEmailThreadDetail struct {
	Thread       CRMEmailThread        `json:"thread"`
	Messages     []CRMEmailMessage     `json:"messages"`
	Participants []CRMEmailParticipant `json:"participants"`
}

// CreateCRMEmailAccountRequest is the payload for connecting an email account.
type CreateCRMEmailAccountRequest struct {
	WorkspaceID  string `json:"workspace_id"`
	MemberID     string `json:"member_id"`
	Provider     string `json:"provider"`
	EmailAddress string `json:"email_address"`
}

// CreateCRMEmailMessageRequest is the payload for creating an email message.
type CreateCRMEmailMessageRequest struct {
	WorkspaceID    string          `json:"workspace_id"`
	EmailAccountID string          `json:"email_account_id"`
	ThreadID       *string         `json:"thread_id"`
	FromAddress    string          `json:"from_address"`
	FromName       *string         `json:"from_name"`
	ToAddresses    json.RawMessage `json:"to_addresses"`
	CCAddresses    json.RawMessage `json:"cc_addresses"`
	Subject        string          `json:"subject"`
	BodyText       *string         `json:"body_text"`
	BodyHTML       *string         `json:"body_html"`
	Direction      string          `json:"direction"`
	SentAt         *time.Time      `json:"sent_at"`
	ContactID      *string         `json:"contact_id"`
	DealID         *string         `json:"deal_id"`
}

// CRMEmailAccountListFilters applies filters when listing email accounts.
type CRMEmailAccountListFilters struct {
	MemberID *string
	Provider *string
	IsActive *bool
}

// CRMEmailThreadListFilters applies filters when listing email threads.
type CRMEmailThreadListFilters struct {
	EmailAccountID *string
	ContactID      *string
	CompanyID      *string
	DealID         *string
	Search         *string
	Scope          *string
	Sort           *string
	UserID         *string
}

// CRMEmailMessageListFilters applies filters when listing email messages.
type CRMEmailMessageListFilters struct {
	ThreadID       *string
	EmailAccountID *string
	ContactID      *string
	CompanyID      *string
	DealID         *string
	Direction      *string
}

// CRMEmailSyncError captures the last sync failure recorded for a mailbox.
type CRMEmailSyncError struct {
	Operation string `json:"operation"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

// CRMEmailSyncCycleStats captures aggregate stats for the most recent sync cycle.
type CRMEmailSyncCycleStats struct {
	Mode                string     `json:"mode"`
	StartedAt           *time.Time `json:"started_at"`
	CompletedAt         *time.Time `json:"completed_at"`
	MessagesSeen        int        `json:"messages_seen"`
	MessagesStored      int        `json:"messages_stored"`
	DuplicatesSkipped   int        `json:"duplicates_skipped"`
	FilteredSkipped     int        `json:"filtered_skipped"`
	InternalSkipped     int        `json:"internal_skipped"`
	ContactsCreated     int        `json:"contacts_created"`
	AssociationsWritten int        `json:"associations_written"`
	ThreadsTouched      int        `json:"threads_touched"`
	RecoveryTriggered   bool       `json:"recovery_triggered"`
}

// CRMEmailSyncDiagnostics is the normalized read model derived from sync_state.
type CRMEmailSyncDiagnostics struct {
	Status              string                  `json:"status"`
	Phase               string                  `json:"phase"`
	LastAttemptAt       *time.Time              `json:"last_attempt_at"`
	LastSuccessAt       *time.Time              `json:"last_success_at"`
	LastFailureAt       *time.Time              `json:"last_failure_at"`
	ConsecutiveFailures int                     `json:"consecutive_failures"`
	LastHistoryID       *string                 `json:"last_history_id"`
	LastError           *CRMEmailSyncError      `json:"last_error"`
	LastCycle           *CRMEmailSyncCycleStats `json:"last_cycle"`
}

// CRMEmailSyncedDataCounts summarizes stored mailbox records.
type CRMEmailSyncedDataCounts struct {
	Threads        int64 `json:"threads"`
	Messages       int64 `json:"messages"`
	CalendarEvents int64 `json:"calendar_events"`
}

// CRMEmailAssociationHealth summarizes email association consistency.
type CRMEmailAssociationHealth struct {
	MessagesMissingAssociations int64 `json:"messages_missing_associations"`
	ThreadsWithEmptyContactIDs  int64 `json:"threads_with_empty_contact_ids"`
}

// CRMEmailAccountDiagnostics is the admin diagnostics read model for a mailbox.
type CRMEmailAccountDiagnostics struct {
	AccountID              string                    `json:"account_id"`
	WorkspaceID            string                    `json:"workspace_id"`
	MemberID               string                    `json:"member_id"`
	Provider               string                    `json:"provider"`
	EmailAddress           string                    `json:"email_address"`
	NormalizedEmailAddress *string                   `json:"normalized_email_address"`
	AccountStatus          string                    `json:"account_status"`
	IsActive               bool                      `json:"is_active"`
	DisconnectedAt         *time.Time                `json:"disconnected_at"`
	LastSyncedAt           *time.Time                `json:"last_synced_at"`
	LastHistoryID          *string                   `json:"last_history_id"`
	HasSyncedData          bool                      `json:"has_synced_data"`
	Sync                   CRMEmailSyncDiagnostics   `json:"sync"`
	Counts                 CRMEmailSyncedDataCounts  `json:"counts"`
	AssociationHealth      CRMEmailAssociationHealth `json:"association_health"`
}

// CRMEmailRebuildAssociationsResult summarizes the association repair job.
type CRMEmailRebuildAssociationsResult struct {
	MessagesScanned     int `json:"messages_scanned"`
	MessagesRepaired    int `json:"messages_repaired"`
	AssociationsWritten int `json:"associations_written"`
	ThreadsRefreshed    int `json:"threads_refreshed"`
	ContactsCreated     int `json:"contacts_created"`
}
