package model

import "time"

// PortalMagicLink stores only a digest of the emailed secret. ConversationID is
// set on anonymous intake confirmations: exchanging the link proves the address
// and publishes that request to the address owner's portal.
type PortalMagicLink struct {
	ID             string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string    `gorm:"type:uuid;not null;index"`
	Email          string    `gorm:"not null"`
	TokenHash      string    `gorm:"uniqueIndex;not null"`
	ConversationID *string   `gorm:"type:uuid"`
	ExpiresAt      time.Time `gorm:"not null"`
	UsedAt         *time.Time
	CreatedAt      time.Time `gorm:"autoCreateTime"`
}

func (PortalMagicLink) TableName() string { return "support_portal_magic_links" }

// PortalSession is independent from staff authentication and can be revoked.
// ReconciledAt records the last request-continuity pass for the session.
// CRMContactID is the approved contact the session was issued for; every
// request in approved-contacts mode must still resolve to it.
type PortalSession struct {
	ID           string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID  string    `gorm:"type:uuid;not null;index"`
	IdentityID   string    `gorm:"type:uuid;not null"`
	CRMContactID *string   `gorm:"type:uuid"`
	TokenHash    string    `gorm:"uniqueIndex;not null"`
	ExpiresAt    time.Time `gorm:"not null"`
	RevokedAt    *time.Time
	ReconciledAt *time.Time
	CreatedAt    time.Time `gorm:"autoCreateTime"`
}

func (PortalSession) TableName() string { return "support_portal_sessions" }

// PortalIntakeSession scopes unverified uploads and can create one request only.
// It never grants read or reply access to an existing conversation.
type PortalIntakeSession struct {
	ID          string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `gorm:"type:uuid;not null;index"`
	Email       string    `gorm:"not null"`
	TokenHash   string    `gorm:"uniqueIndex;not null"`
	ExpiresAt   time.Time `gorm:"not null"`
	UsedAt      *time.Time
	CreatedAt   time.Time `gorm:"autoCreateTime"`
}

func (PortalIntakeSession) TableName() string { return "support_portal_intake_sessions" }
