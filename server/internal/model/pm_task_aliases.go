package model

const (
	PMTaskTypeFeature = PMStoryTypeFeature
	PMTaskTypeBug     = PMStoryTypeBug
	PMTaskTypeChore   = PMStoryTypeChore

	PMTaskPriorityNone   = PMStoryPriorityNone
	PMTaskPriorityLow    = PMStoryPriorityLow
	PMTaskPriorityMedium = PMStoryPriorityMedium
	PMTaskPriorityHigh   = PMStoryPriorityHigh
	PMTaskPriorityUrgent = PMStoryPriorityUrgent

	PMTaskSeverityNone     = PMStorySeverityNone
	PMTaskSeverityMinor    = PMStorySeverityMinor
	PMTaskSeverityMajor    = PMStorySeverityMajor
	PMTaskSeverityCritical = PMStorySeverityCritical

	PMTaskLinkTypeBlocks     = PMStoryLinkTypeBlocks
	PMTaskLinkTypeRelatesTo  = PMStoryLinkTypeRelatesTo
	PMTaskLinkTypeDuplicates = PMStoryLinkTypeDuplicates
)

type PMTask = PMStory
type PMTaskOwner = PMStoryOwner
type PMTaskFollower = PMStoryFollower
type PMTaskLabel = PMStoryLabel
type PMTaskFilters = PMStoryFilters

type CreateTaskRequest = CreateStoryRequest
type UpdateTaskRequest = UpdateStoryRequest
type MoveTaskRequest = MoveStoryRequest
type ReorderTaskRequest = ReorderStoryRequest
type TaskUserLinkRequest = StoryUserLinkRequest
type TaskLabelLinkRequest = StoryLabelLinkRequest

type TaskDetail = StoryDetail
type TaskDependencyTask = StoryDependencyStory
type BoardTask = BoardStory
type TaskGroup = StoryGroup
type TaskStateColumn = StoryStateColumn
type TaskMemberColumn = StoryMemberColumn
type TaskStateCount = StoryStateCount

type PMTaskLink = PMStoryLink
type CreateTaskRelationshipRequest = CreateStoryRelationshipRequest

type PMTaskTemplate = PMStoryTemplate
type CreateTaskTemplateRequest = CreateStoryTemplateRequest
type UpdateTaskTemplateRequest = UpdateStoryTemplateRequest

type TaskDeliveryTarget = StoryDeliveryTarget
type TaskGitLink = StoryGitLink
type UpdateTaskDeliveryTargetRequest = UpdateStoryDeliveryTargetRequest
