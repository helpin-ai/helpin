package repository

import (
	"gorm.io/gorm"
)

type PMTaskRepository = PMStoryRepository
type PMTaskTemplateRepository = PMStoryTemplateRepository
type PMTaskLinkRepository = PMStoryLinkRepository
type TaskDeliveryTargetRepository = StoryDeliveryTargetRepository
type TaskGitLinkRepository = StoryGitLinkRepository

func NewPMTaskRepository(db *gorm.DB) *PMTaskRepository {
	return NewPMStoryRepository(db)
}

func NewPMTaskTemplateRepository(db *gorm.DB) *PMTaskTemplateRepository {
	return NewPMStoryTemplateRepository(db)
}

func NewPMTaskLinkRepository(db *gorm.DB) *PMTaskLinkRepository {
	return NewPMStoryLinkRepository(db)
}

func NewTaskDeliveryTargetRepository(db *gorm.DB) *TaskDeliveryTargetRepository {
	return NewStoryDeliveryTargetRepository(db)
}

func NewTaskGitLinkRepository(db *gorm.DB) *TaskGitLinkRepository {
	return NewStoryGitLinkRepository(db)
}
