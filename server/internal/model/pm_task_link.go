package model

import "time"

const (
	PMTaskLinkTypeBlocks     = "blocks"
	PMTaskLinkTypeRelatesTo  = "relates_to"
	PMTaskLinkTypeDuplicates = "duplicates"
)

// PMTaskLink stores explicit relationships between tasks.
type PMTaskLink struct {
	ID           string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID  string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	SourceStoryID string    `json:"source_task_id" gorm:"column:source_task_id;type:uuid;not null;index"`
	TargetStoryID string    `json:"target_task_id" gorm:"column:target_task_id;type:uuid;not null;index"`
	LinkType     string    `json:"link_type" gorm:"not null;index"`
	CreatedBy    string    `json:"created_by" gorm:"type:uuid;not null"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMTaskLink) TableName() string { return "pm_task_links" }

// TaskDependencyEdge is a simplified dependency graph edge.
type TaskDependencyEdge struct {
	SourceStoryID string `json:"source_task_id"`
	TargetStoryID string `json:"target_task_id"`
	LinkType     string `json:"link_type"`
}
