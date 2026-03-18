package model

import (
	"encoding/json"
	"strings"
)

const (
	PlanningStageDraftSpec   = "draft_spec"
	PlanningStagePlanStories = "plan_stories"

	EpicPlanningStateNotStarted            = "not_started"
	EpicPlanningStateAwaitingClarification = "awaiting_spec_clarification"
	EpicPlanningStateAwaitingSpecApproval  = "awaiting_spec_approval"
	EpicPlanningStateReadyForStoryPlanning = "ready_for_story_planning"
	EpicPlanningStateAwaitingPlanApproval  = "awaiting_plan_approval"
	EpicPlanningStateStoriesCreated        = "stories_created"
	EpicPlanningStateExecutionStarted      = "execution_started"
	EpicPlanningStateReadyForExecution     = "ready_for_execution"

	SpecClarificationKindOpenQuestion = "open_question"
	SpecClarificationKindAssumption   = "assumption"

	SpecClarificationDispositionPending  = "pending"
	SpecClarificationDispositionAnswered = "answered"
	SpecClarificationDispositionAccepted = "accepted"
	SpecClarificationDispositionRejected = "rejected"
)

type ApproveEpicSpecRequest struct {
	VersionID *string `json:"version_id,omitempty"`
}

type SpecClarificationItem struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Prompt      string `json:"prompt"`
	Disposition string `json:"disposition,omitempty"`
	Response    string `json:"response,omitempty"`
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
	AssignAgentID       *string                  `json:"assign_agent_id,omitempty"`
	SliceType           string                   `json:"slice_type,omitempty"`
	ImplementationBrief *StoryImplementationBrief `json:"implementation_brief,omitempty"`
}

// StoryImplementationBrief gives the coding agent a concrete build plan for a story.
type StoryImplementationBrief struct {
	Approach       string       `json:"approach"`
	FilesToModify  []FileChange `json:"files_to_modify"`
	TestStrategy   string       `json:"test_strategy"`
	VerticalLayers []string     `json:"vertical_layers,omitempty"`
	DependsOnFiles []string     `json:"depends_on_files,omitempty"`
}

// FileChange describes a single file-level action in an implementation brief.
type FileChange struct {
	Path        string `json:"path"`
	Action      string `json:"action"`
	Description string `json:"description"`
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
	Risks            []string                `json:"risks,omitempty"`
	VerticalCoverage []VerticalCoverageEntry `json:"vertical_coverage,omitempty"`
	TokensUsed       int                     `json:"tokens_used"`
}

// VerticalCoverageEntry maps a user-facing behavior to the stories that deliver it.
type VerticalCoverageEntry struct {
	Behavior  string   `json:"behavior"`
	StoryRefs []string `json:"story_refs"`
	FullSlice bool     `json:"full_slice"`
}

type ProductSpecDraft struct {
	Title         string                   `json:"title"`
	Summary       string                   `json:"summary"`
	SpecMarkdown  string                   `json:"spec_markdown"`
	Risks         []string                 `json:"risks,omitempty"`
	Assumptions   []string                 `json:"assumptions,omitempty"`
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

type ApprovedSpecSummary struct {
	Stage               string                  `json:"stage"`
	SpecDocumentID      string                  `json:"spec_document_id"`
	SpecVersionID       string                  `json:"spec_version_id"`
	Summary             string                  `json:"summary,omitempty"`
	Clarifications      []SpecClarificationItem `json:"clarifications,omitempty"`
	PendingClarifyCount int                     `json:"pending_clarify_count,omitempty"`
}

func NormalizeSpecClarificationKind(value string) string {
	switch strings.TrimSpace(value) {
	case SpecClarificationKindOpenQuestion:
		return SpecClarificationKindOpenQuestion
	case SpecClarificationKindAssumption:
		return SpecClarificationKindAssumption
	default:
		return ""
	}
}

func NormalizeSpecClarificationDisposition(kind, value string) string {
	kind = NormalizeSpecClarificationKind(kind)
	value = strings.TrimSpace(value)

	switch kind {
	case SpecClarificationKindOpenQuestion:
		if value == SpecClarificationDispositionAnswered {
			return SpecClarificationDispositionAnswered
		}
	case SpecClarificationKindAssumption:
		if value == SpecClarificationDispositionAccepted || value == SpecClarificationDispositionRejected {
			return value
		}
	}
	return SpecClarificationDispositionPending
}

func SpecClarificationResolved(item SpecClarificationItem) bool {
	switch NormalizeSpecClarificationKind(item.Kind) {
	case SpecClarificationKindOpenQuestion:
		return NormalizeSpecClarificationDisposition(item.Kind, item.Disposition) == SpecClarificationDispositionAnswered && strings.TrimSpace(item.Response) != ""
	case SpecClarificationKindAssumption:
		disposition := NormalizeSpecClarificationDisposition(item.Kind, item.Disposition)
		if disposition == SpecClarificationDispositionAccepted {
			return true
		}
		if disposition == SpecClarificationDispositionRejected {
			return strings.TrimSpace(item.Response) != ""
		}
	}
	return false
}

func ParseSpecClarifications(raw json.RawMessage) []SpecClarificationItem {
	if len(raw) == 0 {
		return []SpecClarificationItem{}
	}

	var items []SpecClarificationItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return []SpecClarificationItem{}
	}

	normalized := make([]SpecClarificationItem, 0, len(items))
	for _, item := range items {
		item.ID = strings.TrimSpace(item.ID)
		item.Kind = NormalizeSpecClarificationKind(item.Kind)
		item.Prompt = strings.TrimSpace(item.Prompt)
		item.Disposition = NormalizeSpecClarificationDisposition(item.Kind, item.Disposition)
		item.Response = strings.TrimSpace(item.Response)
		if item.ID == "" || item.Kind == "" || item.Prompt == "" {
			continue
		}
		normalized = append(normalized, item)
	}
	return normalized
}

func MarshalSpecClarifications(items []SpecClarificationItem) json.RawMessage {
	if len(items) == 0 {
		return json.RawMessage("[]")
	}
	payload, err := json.Marshal(items)
	if err != nil {
		return json.RawMessage("[]")
	}
	return payload
}
