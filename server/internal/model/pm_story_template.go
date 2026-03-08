package model

import "time"

// PMStoryTemplate represents reusable story templates scoped to workspace or team.
type PMStoryTemplate struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	TeamID      *string   `json:"team_id" gorm:"type:uuid;index"`
	Name        string    `json:"name" gorm:"not null"`
	Description *string   `json:"description"`
	StoryType   *string   `json:"story_type"`
	Priority    *string   `json:"priority"`
	Severity    *string   `json:"severity"`
	Estimate    *int      `json:"estimate"`
	LabelIDs    *string   `json:"label_ids" gorm:"type:text"`
	Archived    bool      `json:"archived" gorm:"not null;default:false"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMStoryTemplate) TableName() string { return "pm_story_templates" }

// CreateStoryTemplateRequest is the payload for creating a story template.
type CreateStoryTemplateRequest struct {
	WorkspaceID string  `json:"workspace_id"`
	TeamID      *string `json:"team_id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	StoryType   *string `json:"story_type"`
	Priority    *string `json:"priority"`
	Severity    *string `json:"severity"`
	Estimate    *int    `json:"estimate"`
	LabelIDs    *string `json:"label_ids"`
}

// UpdateStoryTemplateRequest is the payload for updating a story template.
type UpdateStoryTemplateRequest struct {
	TeamID      *string `json:"team_id"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	StoryType   *string `json:"story_type"`
	Priority    *string `json:"priority"`
	Severity    *string `json:"severity"`
	Estimate    *int    `json:"estimate"`
	LabelIDs    *string `json:"label_ids"`
	Archived    *bool   `json:"archived"`
}
