package model

// TaskRelationshipAction values drive user-facing relationship creation.
const (
	TaskRelationshipActionRelatesTo      = "relates_to"
	TaskRelationshipActionBlocks         = "blocks"
	TaskRelationshipActionIsBlockedBy    = "is_blocked_by"
	TaskRelationshipActionDuplicates     = "duplicates"
	TaskRelationshipActionIsDuplicatedBy = "is_duplicated_by"

)

// CreateTaskRelationshipRequest creates a directional task relationship from a task detail surface.
type CreateTaskRelationshipRequest struct {
	RelationshipType string `json:"relationship_type"`
	OtherTaskID      string `json:"other_task_id"`
}

// AssociationObjectSummary is the lightweight cross-object shape returned by grouped association APIs.
type AssociationObjectSummary struct {
	AssociationID   string  `json:"association_id,omitempty"`
	ObjectType      string  `json:"object_type"`
	ObjectID        string  `json:"object_id"`
	DisplayID       *string `json:"display_id,omitempty"`
	Title           string  `json:"title"`
	Status          *string `json:"status,omitempty"`
	WorkflowStateID *string `json:"workflow_state_id,omitempty"`
	Completed       bool    `json:"completed,omitempty"`
	TaskType        *string `json:"task_type,omitempty"`
}

// TaskRelationshipSummary represents one relationship edge rendered from the point of view of the current task.
type TaskRelationshipSummary struct {
	RelationshipID string                   `json:"relationship_id"`
	LinkType       string                   `json:"link_type"`
	IsActive       bool                     `json:"is_active"`
	Task           AssociationObjectSummary `json:"task"`
}

// TaskRelationshipGroups contains task-to-task relationships grouped for presentation.
type TaskRelationshipGroups struct {
	BlockedBy    []TaskRelationshipSummary `json:"blocked_by"`
	Blocking     []TaskRelationshipSummary `json:"blocking"`
	RelatesTo    []TaskRelationshipSummary `json:"relates_to"`
	RelatedBy    []TaskRelationshipSummary `json:"related_by"`
	Duplicates   []TaskRelationshipSummary `json:"duplicates"`
	DuplicatedBy []TaskRelationshipSummary `json:"duplicated_by"`
}

// GroupedAssociationsResponse is the umbrella associations payload for PM and support surfaces.
type GroupedAssociationsResponse struct {
	TaskRelationships    TaskRelationshipGroups     `json:"task_relationships"`
	Tasks                []AssociationObjectSummary `json:"tasks"`
	SupportConversations []AssociationObjectSummary `json:"support_conversations"`
	CRMRecords           []AssociationObjectSummary `json:"crm_records"`
	Docs                 []AssociationObjectSummary `json:"docs"`
}
