package model

import "time"

// PMExternalLink represents an external link attached to a task.
type PMExternalLink struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	StoryID     string    `json:"story_id" gorm:"column:task_id;type:uuid;not null;index"`
	Title       string    `json:"title" gorm:"not null"`
	URL         string    `json:"url" gorm:"not null"`
	CreatedByID string    `json:"created_by_id" gorm:"type:uuid;not null"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMExternalLink) TableName() string { return "pm_external_links" }

// CreateExternalLinkRequest is the payload for creating an external link.
type CreateExternalLinkRequest struct {
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
}

// UpdateExternalLinkRequest is the payload for updating an external link.
type UpdateExternalLinkRequest struct {
	URL   *string `json:"url,omitempty"`
	Title *string `json:"title,omitempty"`
}
