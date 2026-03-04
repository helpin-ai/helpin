package model

import "time"

// PMChecklistItem represents a checklist (todo) item on a story.
type PMChecklistItem struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	StoryID   string    `json:"story_id" gorm:"type:uuid;not null;index"`
	Text      string    `json:"text" gorm:"not null"`
	Completed bool      `json:"completed" gorm:"default:false"`
	Position  int       `json:"position" gorm:"default:0"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMChecklistItem) TableName() string { return "pm_checklist_items" }

// CreateChecklistItemRequest is the payload for creating a checklist item.
type CreateChecklistItemRequest struct {
	Text     string `json:"text"`
	Position *int   `json:"position,omitempty"`
}

// UpdateChecklistItemRequest is the payload for updating a checklist item.
type UpdateChecklistItemRequest struct {
	Text      *string `json:"text,omitempty"`
	Completed *bool   `json:"completed,omitempty"`
	Position  *int    `json:"position,omitempty"`
}
