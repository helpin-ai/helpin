package model

import "time"

// PMComment represents comments for tasks/epics/docs.
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

// PMCommentReaction represents an emoji reaction on a comment.
type PMCommentReaction struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CommentID string    `json:"comment_id" gorm:"type:uuid;not null;index:idx_reaction_unique,unique"`
	UserID    string    `json:"user_id" gorm:"type:uuid;not null;index:idx_reaction_unique,unique"`
	Emoji     string    `json:"emoji" gorm:"not null;index:idx_reaction_unique,unique"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (PMCommentReaction) TableName() string { return "pm_comment_reactions" }

// ReactionSummary is the aggregated reaction info per emoji.
type ReactionSummary struct {
	Emoji   string   `json:"emoji"`
	Count   int      `json:"count"`
	UserIDs []string `json:"user_ids"`
}

// CreateCommentRequest is the payload for creating comments.
type CreateCommentRequest struct {
	EntityType    string   `json:"entity_type"`
	EntityID      string   `json:"entity_id"`
	Body          string   `json:"body"`
	ParentID      *string  `json:"parent_id"`
	AttachmentIDs []string `json:"attachment_ids,omitempty"`
}

// UpdateCommentRequest is the payload for updating comments.
type UpdateCommentRequest struct {
	Body string `json:"body"`
}

// ToggleReactionRequest is the payload for toggling a reaction.
type ToggleReactionRequest struct {
	Emoji string `json:"emoji"`
}

// CommentWithAuthor is a comment enriched with author info.
type CommentWithAuthor struct {
	Comment     PMComment            `json:"comment"`
	Author      User                 `json:"author"`
	ReplyCount  int                  `json:"reply_count"`
	Replies     []CommentWithAuthor  `json:"replies,omitempty"`
	Reactions   []ReactionSummary    `json:"reactions"`
	Attachments []AttachmentResponse `json:"attachments,omitempty"`
}
