package service

import (
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// Backward-compat aliases: Story-era names → Task-era canonical types.
type PMStoryService = PMTaskService
type PMStoryTemplateService = PMTaskTemplateService

func NewPMStoryService(taskRepo *repository.PMTaskRepository, workspaceRepo *repository.WorkspaceRepository, workflowRepo *repository.PMWorkflowRepository, epicRepo *repository.PMEpicRepository, sprintRepo *repository.PMSprintRepository, labelRepo *repository.PMLabelRepository, checklistRepo *repository.PMChecklistItemRepository, externalLinkRepo *repository.PMExternalLinkRepository, attachmentRepo *repository.PMAttachmentRepository, activityService *PMActivityService, wsPublisher *websocket.Publisher, automationService *PMAutomationService, notificationService *NotificationService, followerService *FollowerService) *PMStoryService {
	return NewPMTaskService(taskRepo, workspaceRepo, workflowRepo, epicRepo, sprintRepo, labelRepo, checklistRepo, externalLinkRepo, attachmentRepo, activityService, wsPublisher, automationService, notificationService, followerService)
}

func NewPMStoryTemplateService(templateRepo *repository.PMTaskTemplateRepository, wsPublisher *websocket.Publisher) *PMStoryTemplateService {
	return NewPMTaskTemplateService(templateRepo, wsPublisher)
}
