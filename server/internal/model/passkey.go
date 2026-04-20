package model

import (
	"encoding/json"
	"time"
)

// UserPasskey stores a WebAuthn credential that can be used for passwordless login.
type UserPasskey struct {
	ID              string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID          string    `json:"user_id" gorm:"type:uuid;not null;index"`
	CredentialID    []byte    `json:"-" gorm:"column:credential_id;type:bytea;not null;uniqueIndex"`
	PublicKey       []byte    `json:"-" gorm:"column:public_key;type:bytea;not null"`
	AttestationType string    `json:"attestation_type" gorm:"type:text;not null"`
	Transport       JSONBlob  `json:"transport" gorm:"type:jsonb;not null;default:'[]'"`
	SignCount       int64     `json:"sign_count" gorm:"not null;default:0"`
	Name            string    `json:"name" gorm:"type:text;not null"`
	AAGUID          []byte    `json:"-" gorm:"column:aaguid;type:bytea"`
	Flags           int       `json:"-" gorm:"column:flags;not null;default:0"`
	Verified        bool      `json:"verified" gorm:"not null;default:false"`
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (UserPasskey) TableName() string { return "user_passkeys" }

// PasskeyResponse is the public representation used by passkey management UIs.
type PasskeyResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Verified  bool      `json:"verified"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PasskeyOptionsResponse wraps WebAuthn creation or assertion options with the opaque challenge key.
type PasskeyOptionsResponse struct {
	Challenge string          `json:"challenge"`
	Options   json.RawMessage `json:"options"`
}

// PasskeyRegisterRequest completes a registration ceremony.
type PasskeyRegisterRequest struct {
	Challenge  string          `json:"challenge"`
	Credential json.RawMessage `json:"credential"`
	Name       *string         `json:"name,omitempty"`
}

// PasskeyAuthenticationOptionsRequest starts an authentication ceremony.
type PasskeyAuthenticationOptionsRequest struct {
	EmailHint *string `json:"email_hint,omitempty"`
}

// PasskeyAuthenticateRequest completes a passkey authentication ceremony.
type PasskeyAuthenticateRequest struct {
	Challenge  string          `json:"challenge"`
	Credential json.RawMessage `json:"credential"`
	RememberMe bool            `json:"remember_me,omitempty"`
}

// PasskeyListResponse returns the current user's registered passkeys.
type PasskeyListResponse struct {
	Passkeys []PasskeyResponse `json:"passkeys"`
}

func (p UserPasskey) Public() PasskeyResponse {
	return PasskeyResponse{
		ID:        p.ID,
		Name:      p.Name,
		Verified:  p.Verified,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}
