package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// PMChecklistItem represents a checklist (todo) item on a task.
type PMChecklistItem struct {
	ID         string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TaskID     string     `json:"task_id" gorm:"column:task_id;type:uuid;not null;index"`
	Text       string     `json:"text" gorm:"not null"`
	Completed  bool       `json:"completed" gorm:"default:false"`
	Position   int        `json:"position" gorm:"default:0"`
	AssigneeID *string    `json:"assignee_id" gorm:"type:uuid;index"`
	DueDate    *time.Time `json:"due_date" gorm:"type:date"`
	CreatedAt  time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMChecklistItem) TableName() string { return "pm_checklist_items" }

// CreateChecklistItemRequest is the payload for creating a checklist item.
type CreateChecklistItemRequest struct {
	Text       string     `json:"text"`
	Position   *int       `json:"position,omitempty"`
	AssigneeID *string    `json:"assignee_id,omitempty"`
	DueDate    *time.Time `json:"due_date,omitempty"`
}

// UpdateChecklistItemRequest is the payload for updating a checklist item.
type UpdateChecklistItemRequest struct {
	Text       *string    `json:"text,omitempty"`
	Completed  *bool      `json:"completed,omitempty"`
	Position   *int       `json:"position,omitempty"`
	AssigneeID *string    `json:"assignee_id,omitempty"`
	DueDate    *time.Time `json:"due_date,omitempty"`
	DueDateSet bool       `json:"-"`
}

func parseChecklistRequestDueDate(raw json.RawMessage) (*time.Time, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	var value string
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return nil, fmt.Errorf("due_date must be a date string or null")
	}
	if value == "" {
		return nil, nil
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return &parsed, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("due_date must use YYYY-MM-DD or RFC3339 format")
	}
	return &parsed, nil
}

// UnmarshalJSON accepts date-only and RFC3339 due dates for REST compatibility.
func (r *CreateChecklistItemRequest) UnmarshalJSON(data []byte) error {
	var wire struct {
		Text       string          `json:"text"`
		Position   *int            `json:"position"`
		AssigneeID *string         `json:"assignee_id"`
		DueDate    json.RawMessage `json:"due_date"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	dueDate, err := parseChecklistRequestDueDate(wire.DueDate)
	if err != nil {
		return err
	}
	r.Text = wire.Text
	r.Position = wire.Position
	r.AssigneeID = wire.AssigneeID
	r.DueDate = dueDate
	return nil
}

// UnmarshalJSON records due_date presence so omission preserves while null or
// an empty string explicitly clears the stored value.
func (r *UpdateChecklistItemRequest) UnmarshalJSON(data []byte) error {
	var wire struct {
		Text       *string         `json:"text"`
		Completed  *bool           `json:"completed"`
		Position   *int            `json:"position"`
		AssigneeID *string         `json:"assignee_id"`
		DueDate    json.RawMessage `json:"due_date"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	dueDate, err := parseChecklistRequestDueDate(wire.DueDate)
	if err != nil {
		return err
	}
	r.Text = wire.Text
	r.Completed = wire.Completed
	r.Position = wire.Position
	r.AssigneeID = wire.AssigneeID
	r.DueDate = dueDate
	r.DueDateSet = wire.DueDate != nil
	return nil
}
