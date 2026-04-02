package service

import (
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

type PMTaskService = PMStoryService
type PMTaskTemplateService = PMStoryTemplateService

func NewPMTaskService(taskRepo *repository.PMTaskRepository, workspaceRepo *repository.WorkspaceRepository, workflowRepo *repository.PMWorkflowRepository, epicRepo *repository.PMEpicRepository, sprintRepo *repository.PMSprintRepository, labelRepo *repository.PMLabelRepository, checklistRepo *repository.PMChecklistItemRepository, externalLinkRepo *repository.PMExternalLinkRepository, attachmentRepo *repository.PMAttachmentRepository, activityService *PMActivityService, wsPublisher *websocket.Publisher, automationService *PMAutomationService, notificationService *NotificationService, followerService *FollowerService) *PMTaskService {
	return NewPMStoryService(taskRepo, workspaceRepo, workflowRepo, epicRepo, sprintRepo, labelRepo, checklistRepo, externalLinkRepo, attachmentRepo, activityService, wsPublisher, automationService, notificationService, followerService)
}

func NewPMTaskTemplateService(templateRepo *repository.PMTaskTemplateRepository, wsPublisher *websocket.Publisher) *PMTaskTemplateService {
	return NewPMStoryTemplateService(templateRepo, wsPublisher)
}
