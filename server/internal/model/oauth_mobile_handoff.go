package model

import "time"

// OAuthMobileHandoff stores a hashed, short-lived code used to transfer an
// OAuth login from the system browser back into a native app session.
type OAuthMobileHandoff struct {
	ID        string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID    string     `json:"user_id" gorm:"type:uuid;not null;index"`
	CodeHash  string     `json:"-" gorm:"not null;uniqueIndex"`
	ExpiresAt time.Time  `json:"expires_at" gorm:"not null;index"`
	UsedAt    *time.Time `json:"used_at"`
	CreatedAt time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (OAuthMobileHandoff) TableName() string { return "oauth_mobile_handoffs" }

// OAuthMobileExchangeRequest exchanges a single-use deep-link code for a
// normal Helpin auth session.
type OAuthMobileExchangeRequest struct {
	Code string `json:"code"`
}
