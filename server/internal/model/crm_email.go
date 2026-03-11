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

// CRMEmailAccount represents a connected email account for CRM sync.
type CRMEmailAccount struct {
	ID                    string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID           string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	MemberID              string     `json:"member_id" gorm:"type:uuid;not null;index"`
	Provider              string     `json:"provider" gorm:"not null;default:'gmail'"` // gmail, microsoft
	EmailAddress          string     `json:"email_address" gorm:"not null"`
	AccessTokenEncrypted  *string    `json:"-" gorm:"column:access_token_encrypted"`
	RefreshTokenEncrypted *string    `json:"-" gorm:"column:refresh_token_encrypted"`
	SyncState             JSONB      `json:"sync_state" gorm:"type:jsonb;default:'{}'"`
	LastSyncedAt          *time.Time `json:"last_synced_at"`
	IsActive              bool       `json:"is_active" gorm:"not null;default:true"`
	OAuthState            *string    `json:"-" gorm:"column:oauth_state"`
	TokenExpiresAt        *time.Time `json:"-" gorm:"column:token_expires_at"`
	CreatedAt             time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt             time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMEmailAccount) TableName() string { return "crm_email_accounts" }

// CRMEmailThread represents an email thread linked to CRM objects.
type CRMEmailThread struct {
	ID               string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	EmailAccountID   string    `json:"email_account_id" gorm:"type:uuid;not null;index"`
	ThreadExternalID string    `json:"thread_external_id" gorm:"not null"`
	Subject          string    `json:"subject" gorm:"not null"`
	LastMessageAt    time.Time `json:"last_message_at" gorm:"not null"`
	MessageCount     int       `json:"message_count" gorm:"not null;default:0"`
	ContactIDs       json.RawMessage `json:"contact_ids" gorm:"type:jsonb;default:'[]'"`
	DealID           *string   `json:"deal_id" gorm:"type:uuid;index"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMEmailThread) TableName() string { return "crm_email_threads" }

// CRMEmailMessage represents a single email message in CRM.
type CRMEmailMessage struct {
	ID                string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID       string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	EmailAccountID    string    `json:"email_account_id" gorm:"type:uuid;not null;index"`
	ThreadID          *string   `json:"thread_id" gorm:"type:uuid;index"`
	MessageExternalID string    `json:"message_external_id"`
	FromAddress       string    `json:"from_address" gorm:"not null"`
	FromName          *string   `json:"from_name"`
	ToAddresses       json.RawMessage `json:"to_addresses" gorm:"type:jsonb;default:'[]'"`
	CCAddresses       json.RawMessage `json:"cc_addresses" gorm:"type:jsonb;default:'[]'"`
	Subject           string    `json:"subject"`
	BodyText          *string   `json:"body_text"`
	BodyHTML          *string   `json:"body_html"`
	Direction         string    `json:"direction" gorm:"not null;default:'inbound'"` // inbound, outbound
	SentAt            time.Time `json:"sent_at" gorm:"not null"`
	ContactID         *string   `json:"contact_id" gorm:"type:uuid;index"`
	DealID            *string   `json:"deal_id" gorm:"type:uuid;index"`
	CreatedAt         time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (CRMEmailMessage) TableName() string { return "crm_email_messages" }

// CreateCRMEmailAccountRequest is the payload for connecting an email account.
type CreateCRMEmailAccountRequest struct {
	WorkspaceID  string `json:"workspace_id"`
	MemberID     string `json:"member_id"`
	Provider     string `json:"provider"`
	EmailAddress string `json:"email_address"`
}

// CreateCRMEmailMessageRequest is the payload for creating an email message.
type CreateCRMEmailMessageRequest struct {
	WorkspaceID    string                 `json:"workspace_id"`
	EmailAccountID string                 `json:"email_account_id"`
	ThreadID       *string                `json:"thread_id"`
	FromAddress    string                 `json:"from_address"`
	FromName       *string                `json:"from_name"`
	ToAddresses    json.RawMessage `json:"to_addresses"`
	CCAddresses    json.RawMessage `json:"cc_addresses"`
	Subject        string                 `json:"subject"`
	BodyText       *string                `json:"body_text"`
	BodyHTML       *string                `json:"body_html"`
	Direction      string                 `json:"direction"`
	SentAt         *time.Time             `json:"sent_at"`
	ContactID      *string                `json:"contact_id"`
	DealID         *string                `json:"deal_id"`
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
	DealID         *string
	Search         *string
}

// CRMEmailMessageListFilters applies filters when listing email messages.
type CRMEmailMessageListFilters struct {
	ThreadID       *string
	EmailAccountID *string
	ContactID      *string
	DealID         *string
	Direction      *string
}
