package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// JSONB is a flexible JSON column type for GORM.
type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return "{}", nil
	}
	b, err := json.Marshal(j)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = JSONB{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("failed to unmarshal JSONB value: %v", value)
	}
	return json.Unmarshal(bytes, j)
}

// Notification represents a single notification for a recipient about an entity.
// Entity-centric: one notification per (recipient, entity) pair, updated on new events.
type Notification struct {
	ID          string  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string  `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_notif_recipient_entity,priority:4"`
	RecipientID string  `json:"recipient_id" gorm:"type:uuid;not null;uniqueIndex:idx_notif_recipient_entity,priority:1"`
	ActorID     *string `json:"actor_id" gorm:"type:uuid"`

	EntityType string `json:"entity_type" gorm:"type:varchar(50);not null;uniqueIndex:idx_notif_recipient_entity,priority:2"`
	EntityID   string `json:"entity_id" gorm:"type:uuid;not null;uniqueIndex:idx_notif_recipient_entity,priority:3"`

	EventType           string `json:"event_type" gorm:"type:varchar(100);not null"`
	Title               string `json:"title" gorm:"not null"`
	Body                *string `json:"body"`
	Metadata            JSONB  `json:"metadata" gorm:"type:jsonb"`
	LatestEventCategory string `json:"latest_event_category" gorm:"type:varchar(50);not null"`

	// Denormalized render snapshots for inbox rendering without joins.
	ActorSnapshot        JSONB `json:"actor_snapshot" gorm:"type:jsonb"`
	EntitySnapshot       JSONB `json:"entity_snapshot" gorm:"type:jsonb"`
	ParentEntitySnapshot JSONB `json:"parent_entity_snapshot,omitempty" gorm:"type:jsonb"`

	EventCount  int       `json:"event_count" gorm:"default:1"`
	LastEventAt time.Time `json:"last_event_at" gorm:"autoCreateTime"`

	Status       string     `json:"status" gorm:"type:varchar(20);default:'unread';index:idx_notifications_recipient_status"`
	SnoozedUntil *time.Time `json:"snoozed_until"`
	ReadAt       *time.Time `json:"read_at"`
	ArchivedAt   *time.Time `json:"archived_at"`

	Priority string `json:"priority" gorm:"type:varchar(10);default:'normal'"`

	Events []NotificationEvent `json:"events,omitempty" gorm:"foreignKey:NotificationID"`

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Notification) TableName() string { return "notifications" }

// NotificationEvent tracks individual events that contribute to a notification.
type NotificationEvent struct {
	ID             string  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	NotificationID string  `json:"notification_id" gorm:"type:uuid;not null;index"`
	ActorID        *string `json:"actor_id" gorm:"type:uuid"`
	EventType      string  `json:"event_type" gorm:"type:varchar(100);not null"`
	Title          string  `json:"title" gorm:"not null"`
	Metadata       JSONB   `json:"metadata" gorm:"type:jsonb;default:'{}'"`
	Category       string  `json:"category" gorm:"type:varchar(50);not null"`
	ActorSnapshot  JSONB   `json:"actor_snapshot" gorm:"type:jsonb;default:'{}'"`
	Priority       string  `json:"priority" gorm:"type:varchar(10);default:'normal'"`

	Deliveries []NotificationDelivery `json:"deliveries,omitempty" gorm:"foreignKey:NotificationEventID"`
	CreatedAt  time.Time              `json:"created_at" gorm:"autoCreateTime"`
}

func (NotificationEvent) TableName() string { return "notification_events" }

// NotificationDelivery tracks per-event, per-channel delivery state.
type NotificationDelivery struct {
	ID                  string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	NotificationEventID string     `json:"notification_event_id" gorm:"type:uuid;not null;index"`
	Channel             string     `json:"channel" gorm:"type:varchar(30);not null"`
	Status              string     `json:"status" gorm:"type:varchar(20);default:'pending'"`
	DeliveredAt         *time.Time `json:"delivered_at"`
	Error               *string    `json:"error"`
	ExternalMessageID   *string    `json:"external_message_id" gorm:"type:varchar(255)"`
	CreatedAt           time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (NotificationDelivery) TableName() string { return "notification_deliveries" }

// NotificationPreference stores per-user, per-workspace notification settings.
type NotificationPreference struct {
	ID                   string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID               string     `json:"user_id" gorm:"type:uuid;not null;uniqueIndex:uq_notification_pref_user_workspace"`
	WorkspaceID          string     `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:uq_notification_pref_user_workspace"`
	DoNotDisturb         bool       `json:"do_not_disturb" gorm:"default:false"`
	DNDUntil             *time.Time `json:"dnd_until"`
	EmailEnabled         bool       `json:"email_enabled" gorm:"default:true"`
	EmailDigestFrequency string     `json:"email_digest_frequency" gorm:"type:varchar(20);default:'daily'"`
	EmailDigestTime      string     `json:"email_digest_time" gorm:"type:varchar(10);default:'09:00'"`
	EmailDigestDay       int        `json:"email_digest_day" gorm:"default:1"`
	Timezone             string     `json:"timezone" gorm:"type:varchar(50);default:'UTC'"`
	ChannelPreferences   JSONB      `json:"channel_preferences" gorm:"type:jsonb;default:'{}'"`
	BadgeMode            string     `json:"badge_mode" gorm:"type:varchar(20);default:'all'"`
	CreatedAt            time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt            time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (NotificationPreference) TableName() string { return "notification_preferences" }

// EntityFollower tracks who follows which entity for notification purposes.
type EntityFollower struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID      string    `json:"user_id" gorm:"type:uuid;not null;uniqueIndex:uq_entity_follower"`
	EntityType  string    `json:"entity_type" gorm:"type:varchar(50);not null;uniqueIndex:uq_entity_follower"`
	EntityID    string    `json:"entity_id" gorm:"type:uuid;not null;uniqueIndex:uq_entity_follower"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Reason      string    `json:"reason" gorm:"type:varchar(30);default:'manual'"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (EntityFollower) TableName() string { return "entity_followers" }

// NotificationEventInput is the input to NotificationService.Emit().
type NotificationEventInput struct {
	WorkspaceID        string
	ActorID            string
	EventType          string // e.g. "story.assigned"
	EntityType         string // e.g. "story"
	EntityID           string
	Title              string
	Body               string
	Category           string // e.g. "assignment", "comment", "status_change"
	Priority           string // "urgent", "high", "normal", "low"
	Metadata           JSONB
	ActorSnapshot      JSONB
	EntitySnapshot     JSONB
	ParentEntitySnapshot JSONB
	ExplicitRecipients []string // Additional recipients beyond followers
}

// UpdateNotificationRequest is the payload for updating a notification.
type UpdateNotificationRequest struct {
	Status       *string    `json:"status"`        // "read", "unread", "archived"
	SnoozedUntil *time.Time `json:"snoozed_until"`
}

// UpdateNotificationPreferenceRequest is the payload for updating preferences.
type UpdateNotificationPreferenceRequest struct {
	DoNotDisturb         *bool              `json:"do_not_disturb"`
	DNDUntil             *time.Time         `json:"dnd_until"`
	EmailEnabled         *bool              `json:"email_enabled"`
	EmailDigestFrequency *string            `json:"email_digest_frequency"`
	EmailDigestTime      *string            `json:"email_digest_time"`
	EmailDigestDay       *int               `json:"email_digest_day"`
	Timezone             *string            `json:"timezone"`
	ChannelPreferences   map[string]interface{} `json:"channel_preferences"`
	BadgeMode            *string            `json:"badge_mode"`
}

// NotificationListResponse is the paginated response for listing notifications.
type NotificationListResponse struct {
	Data        []Notification `json:"data"`
	NextCursor  *string        `json:"next_cursor"`
	UnreadCount int            `json:"unread_count"`
}

// UnreadCountResponse is the response for getting unread count.
type UnreadCountResponse struct {
	Count int `json:"count"`
}

// FollowRequest is the payload for following an entity.
type FollowRequest struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
}
