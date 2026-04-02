package handler

import "github.com/helpin-ai/helpin/server/internal/service"

type PMTaskHandler = PMStoryHandler
type PMTaskTemplateHandler = PMStoryTemplateHandler

func NewPMTaskHandler(taskService *service.PMTaskService) *PMTaskHandler {
	return NewPMStoryHandler(taskService)
}

func NewPMTaskTemplateHandler(templateService *service.PMTaskTemplateService) *PMTaskTemplateHandler {
	return NewPMStoryTemplateHandler(templateService)
}
