package model

// StoryRelationshipAction values drive user-facing relationship creation.
const (
	StoryRelationshipActionRelatesTo      = "relates_to"
	StoryRelationshipActionBlocks         = "blocks"
	StoryRelationshipActionIsBlockedBy    = "is_blocked_by"
	StoryRelationshipActionDuplicates     = "duplicates"
	StoryRelationshipActionIsDuplicatedBy = "is_duplicated_by"
)

// CreateStoryRelationshipRequest creates a directional story relationship from a story detail surface.
type CreateStoryRelationshipRequest struct {
	RelationshipType string `json:"relationship_type"`
	OtherStoryID     string `json:"other_story_id"`
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
	StoryType       *string `json:"story_type,omitempty"`
}

// StoryRelationshipSummary represents one relationship edge rendered from the point of view of the current story.
type StoryRelationshipSummary struct {
	RelationshipID string                   `json:"relationship_id"`
	LinkType       string                   `json:"link_type"`
	IsActive       bool                     `json:"is_active"`
	Story          AssociationObjectSummary `json:"story"`
}

// StoryRelationshipGroups contains story-to-story relationships grouped for Shortcut-style presentation.
type StoryRelationshipGroups struct {
	BlockedBy    []StoryRelationshipSummary `json:"blocked_by"`
	Blocking     []StoryRelationshipSummary `json:"blocking"`
	RelatesTo    []StoryRelationshipSummary `json:"relates_to"`
	RelatedBy    []StoryRelationshipSummary `json:"related_by"`
	Duplicates   []StoryRelationshipSummary `json:"duplicates"`
	DuplicatedBy []StoryRelationshipSummary `json:"duplicated_by"`
}

// GroupedAssociationsResponse is the umbrella associations payload for PM and support surfaces.
type GroupedAssociationsResponse struct {
	StoryRelationships StoryRelationshipGroups    `json:"story_relationships"`
	Stories            []AssociationObjectSummary `json:"stories"`
	SupportTickets     []AssociationObjectSummary `json:"support_tickets"`
	CRMRecords         []AssociationObjectSummary `json:"crm_records"`
	Docs               []AssociationObjectSummary `json:"docs"`
}
