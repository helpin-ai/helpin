package model

import (
	"encoding/json"
	"strings"
	"time"
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

type ClarifyEpicSpecRequest struct {
	Clarifications []SpecClarificationItem `json:"clarifications"`
}

type PlanEpicStoriesRequest struct {
	AdditionalContext string `json:"additional_context"`
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
