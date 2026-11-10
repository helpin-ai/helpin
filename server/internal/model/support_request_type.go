package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// SupportRequestTypeLimit bounds the request types one workspace keeps.
const SupportRequestTypeLimit = 30

// SupportRequestTypeFieldLimit bounds the fields one request type carries.
const SupportRequestTypeFieldLimit = 30

// SupportRequestTypeField places a conversation field on a request type.
// Internal fields (CustomerVisible false) are filled in by teammates.
type SupportRequestTypeField struct {
	Key             string `json:"key"`
	Required        bool   `json:"required"`
	CustomerVisible bool   `json:"customer_visible"`
	HelpText        string `json:"help_text"`
}

// SupportRequestTypeFields is the JSON list of a request type's fields.
type SupportRequestTypeFields []SupportRequestTypeField

// Value implements driver.Valuer.
func (f SupportRequestTypeFields) Value() (driver.Value, error) {
	if f == nil {
		return "[]", nil
	}
	data, err := json.Marshal(f)
	return string(data), err
}

// Scan implements sql.Scanner.
func (f *SupportRequestTypeFields) Scan(value any) error {
	var data []byte
	switch v := value.(type) {
	case nil:
		*f = SupportRequestTypeFields{}
		return nil
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("unsupported request type fields value")
	}
	return json.Unmarshal(data, f)
}

// SupportRequestType is a kind of request customers pick on the portal,
// such as "Bug report", with the fields it asks for.
type SupportRequestType struct {
	ID string `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	// The unique (workspace_id, lower(name)) index serves workspace lookups.
	WorkspaceID string                   `json:"workspace_id" gorm:"type:uuid;not null"`
	Name        string                   `json:"name" gorm:"not null"`
	Description string                   `json:"description" gorm:"not null;default:''"`
	Fields      SupportRequestTypeFields `json:"fields" gorm:"type:jsonb;not null;default:'[]'"`
	// DefaultPriority and MailboxID apply to new requests of this type;
	// routing rules that match still win.
	DefaultPriority *string   `json:"default_priority"`
	MailboxID       *string   `json:"mailbox_id" gorm:"type:uuid"`
	Enabled         bool      `json:"enabled" gorm:"not null"`
	Position        int       `json:"position" gorm:"type:integer;not null;default:0"`
	CreatedAt       time.Time `json:"created_at" gorm:"type:timestamptz;not null;default:now();autoCreateTime"`
	UpdatedAt       time.Time `json:"updated_at" gorm:"type:timestamptz;not null;default:now();autoUpdateTime"`
	// ConversationCount shows how many conversations carry the type.
	ConversationCount int64 `json:"conversation_count" gorm:"-"`
}

func (SupportRequestType) TableName() string { return "support_request_types" }

// SupportRequestTypeRequest creates or updates a request type.
type SupportRequestTypeRequest struct {
	Name            string                    `json:"name"`
	Description     string                    `json:"description"`
	Fields          []SupportRequestTypeField `json:"fields"`
	DefaultPriority *string                   `json:"default_priority"`
	MailboxID       *string                   `json:"mailbox_id"`
	Enabled         bool                      `json:"enabled"`
}

// ReorderSupportRequestTypesRequest sets the display order of request types.
type ReorderSupportRequestTypesRequest struct {
	IDs []string `json:"ids"`
}
