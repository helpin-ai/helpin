package handler

import "github.com/helpin-ai/helpin/server/internal/service"

// Backward-compat aliases: Story-era names → Task-era canonical types.
type PMStoryHandler = PMTaskHandler
type PMStoryTemplateHandler = PMTaskTemplateHandler

func NewPMStoryHandler(taskService *service.PMTaskService) *PMStoryHandler {
	return NewPMTaskHandler(taskService)
}

func NewPMStoryTemplateHandler(templateService *service.PMTaskTemplateService) *PMStoryTemplateHandler {
	return NewPMTaskTemplateHandler(templateService)
}
