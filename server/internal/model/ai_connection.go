package model

import "time"

// AIConnection is personal to one workspace member. Secret material is never
// serialized; device sessions and OAuth refresh tokens remain app-owned.
type AIConnection struct {
	ID              string     `json:"id" gorm:"type:uuid;primaryKey"`
	WorkspaceID     string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	UserID          string     `json:"user_id" gorm:"type:uuid;not null;index"`
	Name            string     `json:"name" gorm:"not null"`
	Provider        string     `json:"provider" gorm:"not null"`
	Status          string     `json:"status" gorm:"not null"`
	AccountID       string     `json:"account_id,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	EncryptedSecret []byte     `json:"-"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type CreateAIConnectionRequest struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	APIKey   string `json:"api_key,omitempty"`
}

type AIConnectionLogin struct {
	Connection      AIConnection `json:"connection"`
	VerificationURL string       `json:"verification_url,omitempty"`
	UserCode        string       `json:"user_code,omitempty"`
	ExpiresAt       *time.Time   `json:"expires_at,omitempty"`
	IntervalSeconds int          `json:"interval_seconds,omitempty"`
}

func (AIConnection) TableName() string { return "ai_connections" }
