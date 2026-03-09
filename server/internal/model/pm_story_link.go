package model

import "time"

const (
	PMStoryLinkTypeBlocks     = "blocks"
	PMStoryLinkTypeRelatesTo  = "relates_to"
	PMStoryLinkTypeDuplicates = "duplicates"
)

// PMStoryLink stores explicit relationships between stories.
type PMStoryLink struct {
	ID            string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID   string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	SourceStoryID string    `json:"source_story_id" gorm:"type:uuid;not null;index"`
	TargetStoryID string    `json:"target_story_id" gorm:"type:uuid;not null;index"`
	LinkType      string    `json:"link_type" gorm:"not null;index"`
	CreatedBy     string    `json:"created_by" gorm:"type:uuid;not null"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMStoryLink) TableName() string { return "pm_story_links" }

// StoryDependencyEdge is a simplified dependency graph edge.
type StoryDependencyEdge struct {
	SourceStoryID string `json:"source_story_id"`
	TargetStoryID string `json:"target_story_id"`
	LinkType      string `json:"link_type"`
}
