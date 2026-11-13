package model

import "time"

// Support channel account statuses.
const (
	SupportChannelAccountActive         = "active"
	SupportChannelAccountReauthRequired = "reauthorization_required"
	SupportChannelAccountDisabled       = "disabled"
)

// SupportChannelAccount is a connected provider account (a Telegram bot) that
// feeds a mailbox. Owned by versioned SQL only (202611130001).
type SupportChannelAccount struct {
	ID                string     `json:"id" gorm:"type:uuid;primaryKey"`
	WorkspaceID       string     `json:"workspace_id" gorm:"type:uuid;not null"`
	Provider          string     `json:"provider" gorm:"not null"`
	ExternalAccountID string     `json:"external_account_id" gorm:"not null"`
	DisplayName       string     `json:"display_name" gorm:"not null;default:''"`
	Handle            string     `json:"handle" gorm:"not null;default:''"`
	MailboxID         *string    `json:"mailbox_id" gorm:"type:uuid"`
	AIRepliesDisabled bool       `json:"ai_replies_disabled" gorm:"not null;default:false"`
	TokenEncrypted    string     `json:"-" gorm:"not null"`
	WebhookSecretHash string     `json:"-" gorm:"not null"`
	Status            string     `json:"status" gorm:"not null;default:active"`
	AuthErrorCount    int        `json:"-" gorm:"not null;default:0"`
	LastErrorCode     *string    `json:"last_error_code"`
	LastErrorMessage  *string    `json:"last_error_message"`
	LastEventAt       *time.Time `json:"last_event_at"`
	ConnectedBy       *string    `json:"connected_by" gorm:"type:uuid"`
	DisabledAt        *time.Time `json:"disabled_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (SupportChannelAccount) TableName() string { return "support_channel_accounts" }

// SupportChannelAccountView adds delivery health to an account for settings.
type SupportChannelAccountView struct {
	SupportChannelAccount
	FailedDeliveries      int64 `json:"failed_deliveries"`
	UnconfirmedDeliveries int64 `json:"unconfirmed_deliveries"`
}

// ConnectSupportChannelRequest connects an account.
type ConnectSupportChannelRequest struct {
	Provider  string  `json:"provider"`
	Token     string  `json:"token"`
	MailboxID *string `json:"mailbox_id"`
}

// UpdateSupportChannelRequest changes an account's routing and AI setting.
// A nil field is left unchanged; an empty MailboxID clears it.
type UpdateSupportChannelRequest struct {
	MailboxID         *string `json:"mailbox_id"`
	AIRepliesDisabled *bool   `json:"ai_replies_disabled"`
}

// ReconnectSupportChannelRequest re-registers the webhook, optionally with a
// new token after the old one was revoked.
type ReconnectSupportChannelRequest struct {
	Token string `json:"token"`
}

// SupportChannelIdentity is a customer of a connected account (a Telegram
// user). Owned by versioned SQL only (202611130001).
type SupportChannelIdentity struct {
	ID               string     `json:"id" gorm:"type:uuid;primaryKey"`
	WorkspaceID      string     `json:"workspace_id" gorm:"type:uuid;not null"`
	ChannelAccountID string     `json:"channel_account_id" gorm:"type:uuid;not null"`
	ExternalUserID   string     `json:"external_user_id" gorm:"not null"`
	DisplayName      string     `json:"display_name" gorm:"not null;default:''"`
	Username         string     `json:"username" gorm:"not null;default:''"`
	AvatarURL        string     `json:"avatar_url" gorm:"not null;default:''"`
	CRMContactID     *string    `json:"crm_contact_id" gorm:"type:uuid"`
	LastInboundAt    *time.Time `json:"last_inbound_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func (SupportChannelIdentity) TableName() string { return "support_channel_identities" }
