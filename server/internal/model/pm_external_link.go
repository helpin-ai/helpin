package model

import "time"

// PMExternalLink represents an external link attached to an entity.
type PMExternalLink struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TaskID      *string   `json:"task_id,omitempty" gorm:"column:task_id;type:uuid;index"`
	EntityType  string    `json:"entity_type" gorm:"type:varchar(32);default:'task'"`
	EntityID    string    `json:"entity_id" gorm:"type:uuid;index:idx_ext_links_entity"`
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
