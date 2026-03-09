package model

import "time"

// PMComment represents comments for stories/epics/docs.
type PMComment struct {
	ID         string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	EntityType string    `json:"entity_type" gorm:"not null"`
	EntityID   string    `json:"entity_id" gorm:"type:uuid;not null;index"`
	AuthorID   string    `json:"author_id" gorm:"type:uuid;not null;index"`
	Body       string    `json:"body" gorm:"not null"`
	ParentID   *string   `json:"parent_id" gorm:"type:uuid;index"`
	CreatedAt  time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMComment) TableName() string { return "pm_comments" }

// CreateCommentRequest is the payload for creating comments.
type CreateCommentRequest struct {
	EntityType string  `json:"entity_type"`
	EntityID   string  `json:"entity_id"`
	Body       string  `json:"body"`
	ParentID   *string `json:"parent_id"`
}

// UpdateCommentRequest is the payload for updating comments.
type UpdateCommentRequest struct {
	Body string `json:"body"`
}

// CommentWithAuthor is a comment enriched with author info.
type CommentWithAuthor struct {
	Comment    PMComment          `json:"comment"`
	Author     User               `json:"author"`
	ReplyCount int                `json:"reply_count"`
	Replies    []CommentWithAuthor `json:"replies,omitempty"`
}
