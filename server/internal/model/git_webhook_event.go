package model

import "time"

const (
	GitWebhookStatusReceived  = "received"
	GitWebhookStatusProcessed = "processed"
	GitWebhookStatusIgnored   = "ignored"
	GitWebhookStatusFailed    = "failed"
)

// GitWebhookEvent stores raw git provider webhook requests for audit/debugging.
type GitWebhookEvent struct {
	ID                 string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Provider           string    `json:"provider" gorm:"size:30;not null;index"`
	EventType          string    `json:"event_type" gorm:"size:60;not null;index"`
	DeliveryID         *string   `json:"delivery_id,omitempty" gorm:"size:120;index"`
	IntegrationID      *string   `json:"integration_id,omitempty" gorm:"type:uuid;index"`
	WorkspaceID        *string   `json:"workspace_id,omitempty" gorm:"type:uuid;index"`
	RepositoryFullName *string   `json:"repository_full_name,omitempty" gorm:"size:255;index"`
	Action             *string   `json:"action,omitempty" gorm:"size:80;index"`
	Status             string    `json:"status" gorm:"size:30;not null;default:'received';index"`
	StatusCode         *int      `json:"status_code,omitempty"`
	ErrorMessage       *string   `json:"error_message,omitempty"`
	RawPayload         string    `json:"raw_payload" gorm:"type:jsonb;not null"`
	ReceivedAt         time.Time `json:"received_at" gorm:"not null;index"`
	CreatedAt          time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (GitWebhookEvent) TableName() string { return "git_webhook_events" }
