package model

// ── Backward-compatibility aliases: Story → Task ──────────────────────────────
// The canonical types are now PMTask, CreateTaskRequest, etc.
// These aliases ensure existing code that references Story-era names still compiles.

const (
	PMStoryTypeFeature = PMTaskTypeFeature
	PMStoryTypeBug     = PMTaskTypeBug
	PMStoryTypeChore   = PMTaskTypeChore

	PMStoryPriorityNone   = PMTaskPriorityNone
	PMStoryPriorityLow    = PMTaskPriorityLow
	PMStoryPriorityMedium = PMTaskPriorityMedium
	PMStoryPriorityHigh   = PMTaskPriorityHigh
	PMStoryPriorityUrgent = PMTaskPriorityUrgent

	PMStorySeverityNone     = PMTaskSeverityNone
	PMStorySeverityMinor    = PMTaskSeverityMinor
	PMStorySeverityMajor    = PMTaskSeverityMajor
	PMStorySeverityCritical = PMTaskSeverityCritical

	PMStoryLinkTypeBlocks     = PMTaskLinkTypeBlocks
	PMStoryLinkTypeRelatesTo  = PMTaskLinkTypeRelatesTo
	PMStoryLinkTypeDuplicates = PMTaskLinkTypeDuplicates
)

// Core model aliases.
type PMStory = PMTask
type PMStoryOwner = PMTaskOwner
type PMStoryFollower = PMTaskFollower
type PMStoryLabel = PMTaskLabel
type PMStoryFilters = PMTaskFilters

// Request/response aliases.
type CreateStoryRequest = CreateTaskRequest
type UpdateStoryRequest = UpdateTaskRequest
type MoveStoryRequest = MoveTaskRequest
type ReorderStoryRequest = ReorderTaskRequest
type StoryUserLinkRequest = TaskUserLinkRequest
type StoryLabelLinkRequest = TaskLabelLinkRequest

// Detail and board aliases.
type StoryDetail = TaskDetail
type StoryDependencyStory = TaskDependencyTask
type BoardStory = BoardTask
type StoryGroup = TaskGroup
type StoryStateColumn = TaskStateColumn
type StoryMemberColumn = TaskMemberColumn
type StoryStateCount = TaskStateCount
type ColumnStoriesResponse = ColumnTasksResponse

// Link aliases.
type PMStoryLink = PMTaskLink
type StoryDependencyEdge = TaskDependencyEdge

// Template aliases.
type PMStoryTemplate = PMTaskTemplate
type CreateStoryTemplateRequest = CreateTaskTemplateRequest
type UpdateStoryTemplateRequest = UpdateTaskTemplateRequest

// Git aliases.
type StoryDeliveryTarget = TaskDeliveryTarget
type StoryGitLink = TaskGitLink
type UpdateStoryDeliveryTargetRequest = UpdateTaskDeliveryTargetRequest

// Agent planning aliases.
type ProposedStory = ProposedTask
type StoryImplementationBrief = TaskImplementationBrief
var NormalizeProposedStories = NormalizeProposedTasks

// Agent runtime aliases.
type StoryCompletionFollowupProposal = TaskCompletionFollowupProposal
type StoryCompletionAssessment = TaskCompletionAssessment

// Recurring template aliases.
type PMRecurringStorySeed = PMRecurringTaskSeed
type StoryRecurringSummary = TaskRecurringSummary

// Association aliases.
type CreateStoryRelationshipRequest = CreateTaskRelationshipRequest
type StoryRelationshipSummary = TaskRelationshipSummary
type StoryRelationshipGroups = TaskRelationshipGroups

// Sprint planning aliases.
type SprintPlanningStoryPreview = SprintPlanningTaskPreview
