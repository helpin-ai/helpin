package model

import "time"

// PortalMagicLink stores only a digest of the emailed secret.
type PortalMagicLink struct {
	ID          string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `gorm:"type:uuid;not null;index"`
	Email       string    `gorm:"not null"`
	TokenHash   string    `gorm:"uniqueIndex;not null"`
	ExpiresAt   time.Time `gorm:"not null"`
	UsedAt      *time.Time
	CreatedAt   time.Time `gorm:"autoCreateTime"`
}

func (PortalMagicLink) TableName() string { return "support_portal_magic_links" }

// PortalSession is independent from staff authentication and can be revoked.
type PortalSession struct {
	ID          string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `gorm:"type:uuid;not null;index"`
	IdentityID  string    `gorm:"type:uuid;not null"`
	TokenHash   string    `gorm:"uniqueIndex;not null"`
	ExpiresAt   time.Time `gorm:"not null"`
	RevokedAt   *time.Time
	CreatedAt   time.Time `gorm:"autoCreateTime"`
}

func (PortalSession) TableName() string { return "support_portal_sessions" }
