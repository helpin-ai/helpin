package model

import "time"

const (
	PlanningStageDraftSpec   = "draft_spec"
	PlanningStagePlanStories = "plan_stories"

	EpicPlanningStateNotStarted            = "not_started"
	EpicPlanningStateAwaitingSpecApproval  = "awaiting_spec_approval"
	EpicPlanningStateReadyForStoryPlanning = "ready_for_story_planning"
	EpicPlanningStateAwaitingPlanApproval  = "awaiting_plan_approval"
	EpicPlanningStateStoriesCreated        = "stories_created"
	EpicPlanningStateExecutionStarted      = "execution_started"
	EpicPlanningStateReadyForExecution     = "ready_for_execution"
)

// OrchestrateRequest is the legacy request to decompose an epic into stories.
// Stage defaults to draft_spec for the new staged planner.
type OrchestrateRequest struct {
	AdditionalContext string `json:"additional_context"`
	Stage             string `json:"stage,omitempty"`
}

type DraftEpicSpecRequest struct {
	AdditionalContext string `json:"additional_context"`
}

type ApproveEpicSpecRequest struct {
	VersionID *string `json:"version_id,omitempty"`
}

type PlanEpicStoriesRequest struct {
	AdditionalContext string `json:"additional_context"`
}

// ProposedStory is a reviewable planning output before confirmation.
type ProposedStory struct {
	Ref                string              `json:"ref,omitempty"`
	Name               string              `json:"name"`
	Description        string              `json:"description"`
	StoryType          string              `json:"story_type"`
	Estimate           *int                `json:"estimate"`
	Priority           *string             `json:"priority,omitempty"`
	AcceptanceCriteria []string            `json:"acceptance_criteria,omitempty"`
	DependencyRefs     []string            `json:"dependency_refs,omitempty"`
	SourceRefs         []PlanningSourceRef `json:"source_refs,omitempty"`
	AssignAgentID      *string             `json:"assign_agent_id,omitempty"`
}

type PlanningSourceRef struct {
	Type  string `json:"type"`
	ID    string `json:"id,omitempty"`
	Title string `json:"title,omitempty"`
}

type OrchestrationProposal struct {
	EpicID          string          `json:"epic_id"`
	Summary         string          `json:"summary"`
	SpecVersionID   string          `json:"spec_version_id,omitempty"`
	ProposedStories []ProposedStory `json:"proposed_stories"`
	OpenQuestions   []string        `json:"open_questions,omitempty"`
	Risks           []string        `json:"risks,omitempty"`
	TokensUsed      int             `json:"tokens_used"`
}

type ProductSpecDraft struct {
	Title         string                   `json:"title"`
	Summary       string                   `json:"summary"`
	SpecMarkdown  string                   `json:"spec_markdown"`
	Risks         []string                 `json:"risks,omitempty"`
	OpenQuestions []string                 `json:"open_questions,omitempty"`
	Sources       []PlanningResearchSource `json:"sources,omitempty"`
}

type PlanningResearchSource struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Note        string `json:"note,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
}

// ConfirmPlanningRequest confirms a story plan and optionally edits proposed stories first.
type ConfirmPlanningRequest struct {
	RunID           string          `json:"run_id"`
	ProposedStories []ProposedStory `json:"proposed_stories"`
}

// ConfirmOrchestrationRequest remains as a backward-compatible alias.
type ConfirmOrchestrationRequest = ConfirmPlanningRequest

type KickoffPlanningExecutionRequest struct {
	RunID    string   `json:"run_id"`
	StoryIDs []string `json:"story_ids,omitempty"`
}

type KickoffExecutionResult struct {
	Started []PlanningExecutionStart `json:"started"`
	Skipped []PlanningExecutionSkip  `json:"skipped"`
}

type PlanningExecutionStart struct {
	StoryID   string    `json:"story_id"`
	RunID     string    `json:"run_id"`
	StartedAt time.Time `json:"started_at"`
}

type PlanningExecutionSkip struct {
	StoryID string `json:"story_id"`
	Reason  string `json:"reason"`
}

type ApprovedSpecSummary struct {
	Stage          string `json:"stage"`
	SpecDocumentID string `json:"spec_document_id"`
	SpecVersionID  string `json:"spec_version_id"`
	Summary        string `json:"summary,omitempty"`
}
