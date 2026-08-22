package model

import "time"

// PushDevice represents a mobile device registered to receive push
// notifications for a user. Registrations are keyed by device token: a
// re-registration of the same token reassigns the row to the current user,
// since phones change accounts.
type PushDevice struct {
	ID         string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID     string    `json:"user_id" gorm:"type:uuid;index;not null"`
	Platform   string    `json:"platform" gorm:"not null"`
	Token      string    `json:"token" gorm:"uniqueIndex;not null"`
	AppVersion string    `json:"app_version"`
	LastSeenAt time.Time `json:"last_seen_at"`
	CreatedAt  time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PushDevice) TableName() string { return "push_devices" }

// RegisterPushDeviceRequest is the request body for POST /user/push-devices.
type RegisterPushDeviceRequest struct {
	Platform   string `json:"platform"`
	Token      string `json:"token"`
	AppVersion string `json:"app_version"`
}

// UnregisterPushDeviceRequest is the request body for DELETE /user/push-devices.
type UnregisterPushDeviceRequest struct {
	Token string `json:"token"`
}
