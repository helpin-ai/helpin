package model

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	PlanningStageDraftSpec    = "draft_spec"
	PlanningStagePlanStories   = "plan_stories"
	PlanningStageStoryPlanDoc  = "story_plan_doc"
	PlanningStageTaskPlanDoc   = "task_plan_doc"

	EpicPlanningStateNotStarted          = "not_started"
	EpicPlanningStateAwaitingClarification = "awaiting_spec_clarification"
	EpicPlanningStateAwaitingSpecApproval  = "awaiting_spec_approval"
	EpicPlanningStateReadyForTaskPlanning  = "ready_for_task_planning"
	EpicPlanningStateReadyForStoryPlanning = "ready_for_story_planning" // compat alias
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

// ProposedTask is a reviewable planning output before confirmation.
type ProposedTask struct {
	Ref                 string                   `json:"ref,omitempty"`
	Name                string                   `json:"name"`
	Description         string                   `json:"description"`
	TaskType            string                   `json:"task_type"`
	Estimate            *int                     `json:"estimate"`
	Priority            *string                  `json:"priority,omitempty"`
	AcceptanceCriteria  []string                 `json:"acceptance_criteria,omitempty"`
	DependencyRefs      []string                 `json:"dependency_refs,omitempty"`
	SourceRefs          []PlanningSourceRef      `json:"source_refs,omitempty"`
	AssignAgentID       *string                  `json:"assign_agent_id,omitempty"`
	SliceType           string                   `json:"slice_type,omitempty"`
	ImplementationBrief *TaskImplementationBrief `json:"implementation_brief,omitempty"`
}

func (p *ProposedTask) UnmarshalJSON(data []byte) error {
	type rawProposedTask struct {
		Ref                 string                   `json:"ref,omitempty"`
		Name                string                   `json:"name"`
		Title               string                   `json:"title"`
		Description         string                   `json:"description"`
		TaskType            string                   `json:"task_type"`
		StoryType           string                   `json:"story_type"`
		Type                string                   `json:"type"`
		Estimate            *int                     `json:"estimate"`
		Priority            *string                  `json:"priority,omitempty"`
		AcceptanceCriteria  []string                 `json:"acceptance_criteria,omitempty"`
		DependencyRefs      []string                 `json:"dependency_refs,omitempty"`
		SourceRefs          []PlanningSourceRef      `json:"source_refs,omitempty"`
		AssignAgentID       *string                  `json:"assign_agent_id,omitempty"`
		SliceType           string                   `json:"slice_type,omitempty"`
		ImplementationBrief *TaskImplementationBrief `json:"implementation_brief,omitempty"`
	}

	var raw rawProposedTask
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	p.Ref = strings.TrimSpace(raw.Ref)
	p.Name = strings.TrimSpace(firstNonEmpty(raw.Name, raw.Title))
	p.Description = strings.TrimSpace(raw.Description)
	p.TaskType = strings.TrimSpace(firstNonEmpty(raw.TaskType, raw.StoryType, raw.Type))
	p.Estimate = raw.Estimate
	p.Priority = raw.Priority
	p.AcceptanceCriteria = raw.AcceptanceCriteria
	p.DependencyRefs = raw.DependencyRefs
	p.SourceRefs = raw.SourceRefs
	p.AssignAgentID = raw.AssignAgentID
	p.SliceType = strings.TrimSpace(raw.SliceType)
	p.ImplementationBrief = raw.ImplementationBrief
	return nil
}

// TaskImplementationBrief gives the coding agent a concrete build plan for a task.
type TaskImplementationBrief struct {
	Approach       string       `json:"approach"`
	FilesToModify  []FileChange `json:"files_to_modify"`
	TestStrategy   string       `json:"test_strategy"`
	VerticalLayers []string     `json:"vertical_layers,omitempty"`
	DependsOnFiles []string     `json:"depends_on_files,omitempty"`
}

func (b *TaskImplementationBrief) UnmarshalJSON(data []byte) error {
	type rawTaskImplementationBrief struct {
		Approach       string          `json:"approach"`
		FilesToModify  []FileChange    `json:"files_to_modify"`
		TestStrategy   json.RawMessage `json:"test_strategy"`
		VerticalLayers []string        `json:"vertical_layers,omitempty"`
		DependsOnFiles []string        `json:"depends_on_files,omitempty"`
	}

	var raw rawTaskImplementationBrief
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	b.Approach = strings.TrimSpace(raw.Approach)
	b.FilesToModify = raw.FilesToModify
	b.VerticalLayers = raw.VerticalLayers
	b.DependsOnFiles = raw.DependsOnFiles

	if len(raw.TestStrategy) == 0 {
		b.TestStrategy = ""
		return nil
	}

	var single string
	if err := json.Unmarshal(raw.TestStrategy, &single); err == nil {
		b.TestStrategy = strings.TrimSpace(single)
		return nil
	}

	var multiple []string
	if err := json.Unmarshal(raw.TestStrategy, &multiple); err == nil {
		filtered := make([]string, 0, len(multiple))
		for _, item := range multiple {
			item = strings.TrimSpace(item)
			if item != "" {
				filtered = append(filtered, item)
			}
		}
		b.TestStrategy = strings.Join(filtered, "\n")
		return nil
	}

	return fmt.Errorf("implementation_brief.test_strategy must be a string or array of strings")
}

// FileChange describes a single file-level action in an implementation brief.
type FileChange struct {
	Path        string `json:"path"`
	Action      string `json:"action"`
	Description string `json:"description"`
}

func (f *FileChange) UnmarshalJSON(data []byte) error {
	var path string
	if err := json.Unmarshal(data, &path); err == nil {
		f.Path = strings.TrimSpace(path)
		f.Action = normalizeFileChangeAction("")
		f.Description = ""
		return nil
	}

	type alias FileChange
	var raw alias
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	f.Path = strings.TrimSpace(raw.Path)
	f.Action = normalizeFileChangeAction(raw.Action)
	f.Description = strings.TrimSpace(raw.Description)
	return nil
}

type PlanningSourceRef struct {
	Type  string `json:"type"`
	ID    string `json:"id,omitempty"`
	Title string `json:"title,omitempty"`
}

type OrchestrationProposal struct {
	EpicID           string                  `json:"epic_id"`
	Summary          string                  `json:"summary"`
	SpecVersionID    string                  `json:"spec_version_id,omitempty"`
	ProposedTasks    []ProposedTask          `json:"proposed_tasks"`
	OpenQuestions    []string                `json:"open_questions,omitempty"`
	Risks            []string                `json:"risks,omitempty"`
	VerticalCoverage []VerticalCoverageEntry `json:"vertical_coverage,omitempty"`
	TokensUsed       int                     `json:"tokens_used"`
}

func NormalizeProposedTasks(tasks []ProposedTask) error {
	refToIdx := make(map[string]int, len(tasks))
	for idx := range tasks {
		tasks[idx].Ref = strings.TrimSpace(tasks[idx].Ref)
		tasks[idx].Name = strings.TrimSpace(tasks[idx].Name)
		tasks[idx].Description = strings.TrimSpace(tasks[idx].Description)
		tasks[idx].TaskType = strings.TrimSpace(tasks[idx].TaskType)
		tasks[idx].SliceType = strings.TrimSpace(tasks[idx].SliceType)

		if tasks[idx].Name == "" {
			return fmt.Errorf("story %d is missing name; use field \"name\" for the story title", idx+1)
		}

		filteredCriteria := make([]string, 0, len(tasks[idx].AcceptanceCriteria))
		for _, item := range tasks[idx].AcceptanceCriteria {
			item = strings.TrimSpace(item)
			if item != "" {
				filteredCriteria = append(filteredCriteria, item)
			}
		}
		tasks[idx].AcceptanceCriteria = filteredCriteria
		if len(filteredCriteria) == 0 {
			return fmt.Errorf("story %d is missing acceptance_criteria; provide at least one acceptance criterion", idx+1)
		}

		tasks[idx].DependencyRefs = filterNonEmptyPlannerStrings(tasks[idx].DependencyRefs)
		if tasks[idx].Ref == "" {
			tasks[idx].Ref = fmt.Sprintf("story_%d", idx+1)
		}

		if brief := tasks[idx].ImplementationBrief; brief != nil {
			brief.Approach = strings.TrimSpace(brief.Approach)
			brief.TestStrategy = strings.TrimSpace(brief.TestStrategy)
			brief.VerticalLayers = filterNonEmptyPlannerStrings(brief.VerticalLayers)
			brief.DependsOnFiles = filterNonEmptyPlannerStrings(brief.DependsOnFiles)
			files := make([]FileChange, 0, len(brief.FilesToModify))
			for _, change := range brief.FilesToModify {
				change.Path = strings.TrimSpace(change.Path)
				if change.Path == "" {
					continue
				}
				change.Description = strings.TrimSpace(change.Description)
				change.Action = normalizeFileChangeAction(change.Action)
				files = append(files, change)
			}
			brief.FilesToModify = files
		}

		if prev, exists := refToIdx[tasks[idx].Ref]; exists {
			return fmt.Errorf("story refs must be unique; stories %d and %d both use ref %q", prev+1, idx+1, tasks[idx].Ref)
		}
		refToIdx[tasks[idx].Ref] = idx
	}

	visited := make(map[string]uint8, len(tasks))
	var visit func(ref string) error
	visit = func(ref string) error {
		switch visited[ref] {
		case 1:
			return fmt.Errorf("dependency_refs contain a cycle involving ref %q", ref)
		case 2:
			return nil
		}
		visited[ref] = 1
		task := tasks[refToIdx[ref]]
		for _, depRef := range task.DependencyRefs {
			if _, ok := refToIdx[depRef]; !ok {
				return fmt.Errorf("story %d references unknown dependency ref %q in dependency_refs", refToIdx[ref]+1, depRef)
			}
			if depRef == ref {
				return fmt.Errorf("story %d cannot list its own ref in dependency_refs", refToIdx[ref]+1)
			}
			if err := visit(depRef); err != nil {
				return err
			}
		}
		visited[ref] = 2
		return nil
	}

	for _, task := range tasks {
		if err := visit(task.Ref); err != nil {
			return err
		}
	}
	return nil
}

// VerticalCoverageEntry maps a user-facing behavior to the tasks that deliver it.
type VerticalCoverageEntry struct {
	Behavior  string   `json:"behavior"`
	TaskRefs []string `json:"task_refs"`
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

// ConfirmPlanningRequest confirms a task plan and optionally edits proposed tasks first.
type ConfirmPlanningRequest struct {
	RunID          string         `json:"run_id"`
	ProposedTasks []ProposedTask `json:"proposed_tasks"`
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

func normalizeFileChangeAction(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "create":
		return "create"
	case "delete":
		return "delete"
	default:
		return "modify"
	}
}

func filterNonEmptyPlannerStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			filtered = append(filtered, value)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
