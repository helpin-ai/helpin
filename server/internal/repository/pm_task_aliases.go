package repository

import (
	"gorm.io/gorm"
)

// Backward-compat aliases: Story-era names → Task-era canonical types.
type PMStoryRepository = PMTaskRepository
type PMStoryTemplateRepository = PMTaskTemplateRepository
type PMStoryLinkRepository = PMTaskLinkRepository
type StoryDeliveryTargetRepository = TaskDeliveryTargetRepository
type StoryGitLinkRepository = TaskGitLinkRepository
type PMStoryTemplateListOptions = PMTaskTemplateListOptions

func NewPMStoryRepository(db *gorm.DB) *PMStoryRepository {
	return NewPMTaskRepository(db)
}

func NewPMStoryTemplateRepository(db *gorm.DB) *PMStoryTemplateRepository {
	return NewPMTaskTemplateRepository(db)
}

func NewPMStoryLinkRepository(db *gorm.DB) *PMStoryLinkRepository {
	return NewPMTaskLinkRepository(db)
}

func NewStoryDeliveryTargetRepository(db *gorm.DB) *StoryDeliveryTargetRepository {
	return NewTaskDeliveryTargetRepository(db)
}

func NewStoryGitLinkRepository(db *gorm.DB) *StoryGitLinkRepository {
	return NewTaskGitLinkRepository(db)
}
