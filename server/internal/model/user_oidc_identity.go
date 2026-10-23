package model

import "time"

// UserOIDCIdentity links a Helpin user to an account at an OpenID Connect
// provider. Identity is the provider (issuer) plus its stable subject claim,
// never the email address.
type UserOIDCIdentity struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID    string    `json:"user_id" gorm:"type:uuid;not null"`
	Issuer    string    `json:"issuer" gorm:"not null;uniqueIndex:idx_user_oidc_identities_issuer_subject,priority:1"`
	Subject   string    `json:"subject" gorm:"not null;uniqueIndex:idx_user_oidc_identities_issuer_subject,priority:2"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`

	User *User `json:"-" gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
}

func (UserOIDCIdentity) TableName() string { return "user_oidc_identities" }
