package model

import (
	"gorm.io/gorm"
	"time"
)

// PMComment represents comments for tasks/epics/docs.
type PMComment struct {
	SlackPermalink         *string    `json:"slack_permalink,omitempty" gorm:"<-:create"`
	SlackDiscussionChannel *string    `json:"slack_discussion_channel,omitempty" gorm:"->"`
	SlackSourceKey         *string    `json:"-" gorm:"<-:create"`
	ID                     string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	EntityType             string     `json:"entity_type" gorm:"not null"`
	EntityID               string     `json:"entity_id" gorm:"type:uuid;not null;index"`
	AuthorID               string     `json:"author_id" gorm:"type:uuid;not null;index"`
	AgentID                *string    `json:"agent_id,omitempty" gorm:"type:uuid;index"`
	AgentName              string     `json:"agent_name,omitempty"`
	AgentRunID             *string    `json:"agent_run_id,omitempty" gorm:"type:uuid;index"`
	Body                   string     `json:"body" gorm:"not null"`
	ParentID               *string    `json:"parent_id" gorm:"type:uuid;index"`
	BlockID                *string    `json:"block_id,omitempty" gorm:"type:uuid;index"`
	Range                  JSONB      `json:"range,omitempty" gorm:"column:block_range;type:jsonb"`
	AnchorText             string     `json:"anchor_text,omitempty"`
	ResolvedAt             *time.Time `json:"resolved_at,omitempty"`
	ResolvedBy             *string    `json:"resolved_by,omitempty" gorm:"type:uuid"`
	CreatedAt              time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt              time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMComment) TableName() string { return "pm_comments" }

// Ordinary comments also come from bulk imports and older schemas.
func (comment *PMComment) BeforeCreate(tx *gorm.DB) error {
	if comment.SlackPermalink == nil {
		tx.Statement.Omits = append(tx.Statement.Omits, "SlackPermalink")
	}
	if comment.SlackSourceKey == nil {
		tx.Statement.Omits = append(tx.Statement.Omits, "SlackSourceKey")
	}
	return nil
}

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
	SlackSourceKey *string  `json:"-"`
	EntityType     string   `json:"entity_type"`
	EntityID       string   `json:"entity_id"`
	Body           string   `json:"body"`
	ParentID       *string  `json:"parent_id"`
	BlockID        *string  `json:"block_id,omitempty"`
	Range          JSONB    `json:"range,omitempty"`
	AnchorText     string   `json:"anchor_text,omitempty"`
	AttachmentIDs  []string `json:"attachment_ids,omitempty"`
	AgentID        *string  `json:"-"`
	AgentName      string   `json:"-"`
	AgentRunID     *string  `json:"-"`
}

// UpdateCommentRequest is the payload for updating comments.
type UpdateCommentRequest struct {
	Body          string   `json:"body"`
	AttachmentIDs []string `json:"attachment_ids,omitempty"`
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
