package model

import (
	"encoding/json"
	"time"
)

// PMActivityLog represents a persisted activity event for PM entities.
type PMActivityLog struct {
	ID          string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	EntityType  string          `json:"entity_type" gorm:"not null"`
	EntityID    string          `json:"entity_id" gorm:"type:uuid;not null;index"`
	ActorID     *string         `json:"actor_id" gorm:"type:uuid;index"`
	EventType   *string         `json:"event_type,omitempty" gorm:"index"`
	Action      string          `json:"action" gorm:"not null"`
	FieldName   *string         `json:"field_name"`
	OldValue    *string         `json:"old_value"`
	NewValue    *string         `json:"new_value"`
	Metadata    json.RawMessage `json:"metadata" gorm:"type:jsonb"`
	CreatedAt   time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

func (PMActivityLog) TableName() string { return "pm_activity_log" }

// PMActivityFilters applies workspace feed filters.
type PMActivityFilters struct {
	ActorID    *string
	EntityType *string
	Action     *string
}

// ActivityLogEntry is an activity item enriched with actor details.
type ActivityLogEntry struct {
	Activity PMActivityLog `json:"activity"`
	Actor    *User         `json:"actor,omitempty"`
}
