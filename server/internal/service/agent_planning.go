package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

const (
	productSpecsSpaceSlug = "product-specs"
	productSpecsSpaceName = "Product Specs"
)

type planningRunInput struct {
	Stage               string `json:"stage,omitempty"`
	AdditionalContext   string `json:"additional_context,omitempty"`
	SpecDocumentID      string `json:"spec_document_id,omitempty"`
	SpecVersionID       string `json:"spec_version_id,omitempty"`
	PlanningMethodology string `json:"planning_methodology,omitempty"`
}

type epicPlanningRunSummary struct {
	Stage               string                        `json:"stage"`
	SpecDocumentID      string                        `json:"spec_document_id,omitempty"`
	SpecVersionID       string                        `json:"spec_version_id,omitempty"`
	PlanningMethodology string                        `json:"planning_methodology,omitempty"`
	Summary             string                        `json:"summary,omitempty"`
	Risks               []string                      `json:"risks,omitempty"`
	Assumptions         []string                      `json:"assumptions,omitempty"`
	OpenQuestions       []string                      `json:"open_questions,omitempty"`
	Clarifications      []model.SpecClarificationItem `json:"clarifications,omitempty"`
	Proposal            *model.OrchestrationProposal  `json:"proposal,omitempty"`
	CreatedTaskIDs      []string                      `json:"created_task_ids,omitempty"`
	CreatedTasks        []createdPlanningTask         `json:"created_tasks,omitempty"`
}

type createdPlanningTask struct {
	TaskID              string                    `json:"task_id"`
	Ref                 string                    `json:"ref,omitempty"`
	Name                string                    `json:"name"`
	TaskType            string                    `json:"task_type"`
	Estimate            *int                      `json:"estimate,omitempty"`
	Priority            *string                   `json:"priority,omitempty"`
	AcceptanceCriteria  []string                  `json:"acceptance_criteria,omitempty"`
	DependencyRefs      []string                  `json:"dependency_refs,omitempty"`
	SourceRefs          []model.PlanningSourceRef `json:"source_refs,omitempty"`
	ImplementationBrief json.RawMessage           `json:"implementation_brief,omitempty"`
}

// ApproveEpicSpec approves the current or specified spec version and clears any pending draft-spec run.
func (s *AgentService) ApproveEpicSpec(ctx context.Context, workspaceID, epicID, actorID string, req model.ApproveEpicSpecRequest) (*model.ApprovedSpecSummary, error) {
	epicWithStats, err := s.epicRepo.GetByID(ctx, epicID)
	if err != nil {
		return nil, fmt.Errorf("get epic: %w", err)
	}
	if epicWithStats == nil {
		return nil, fmt.Errorf("epic not found")
	}
	epic := &epicWithStats.Epic
	if epic.SpecDocumentID == nil || strings.TrimSpace(*epic.SpecDocumentID) == "" {
		return nil, fmt.Errorf("epic does not have a product spec document yet")
	}

	clarifications := model.ParseSpecClarifications(epic.SpecClarifications)
	pendingClarifyCount := countPendingSpecClarifications(clarifications)
	if pendingClarifyCount > 0 {
		return nil, fmt.Errorf("resolve all open questions and assumptions before approving the spec")
	}
	if req.VersionID != nil && strings.TrimSpace(*req.VersionID) != "" && len(clarifications) > 0 {
		return nil, fmt.Errorf("approve the current spec version after clarifications are synced into Docs")
	}

	if len(clarifications) > 0 {
		if err := s.syncClarificationsIntoSpecDoc(ctx, *epic.SpecDocumentID, actorID, clarifications); err != nil {
			return nil, err
		}
	}

	var version *model.DocsVersion
	if req.VersionID != nil && strings.TrimSpace(*req.VersionID) != "" {
		version, err = s.docsVersionRepo.GetByID(ctx, *req.VersionID)
		if err != nil {
			return nil, err
		}
		if version == nil || version.DocumentID != *epic.SpecDocumentID {
			return nil, fmt.Errorf("version does not belong to the epic product spec")
		}
	} else {
		content, err := s.docsContentRepo.GetByDocumentID(ctx, *epic.SpecDocumentID)
		if err != nil {
			return nil, err
		}
		if content == nil {
			return nil, fmt.Errorf("spec document has no content to approve")
		}
		label := "Approved Spec"
		version, err = s.docsVersionRepo.Create(ctx, *epic.SpecDocumentID, actorID, content.Content, content.ContentText, &label, "manual", len(strings.Fields(content.ContentText)))
		if err != nil {
			return nil, err
		}
	}

	epic.ApprovedSpecVersionID = &version.ID
	if err := s.epicRepo.Update(ctx, epic); err != nil {
		return nil, err
	}

	if err := s.approveActiveEpicPlanningRun(ctx, workspaceID, epicID, actorID, model.PlanningStageDraftSpec); err != nil {
		return nil, err
	}

	_ = s.activitySvc.Log(ctx, workspaceID, "epic", epicID, &actorID, "updated", strPtr("approved_spec_version_id"), nil, &version.ID, nil)
	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "epic",
		EntityID:    epicID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
	})

	return &model.ApprovedSpecSummary{
		Stage:               model.PlanningStageDraftSpec,
		SpecDocumentID:      *epic.SpecDocumentID,
		SpecVersionID:       version.ID,
		Clarifications:      clarifications,
		PendingClarifyCount: 0,
	}, nil
}

// ConfirmEpicRun confirms a task plan, creates tasks, and writes dependency links.
func (s *AgentService) ConfirmEpicRun(ctx context.Context, workspaceID, epicID, runID, actorID string, req model.ConfirmPlanningRequest) ([]model.PMTask, error) {
	// Interactive path: stories provided directly, no agent run to validate.
	if runID == "" && len(req.ProposedTasks) > 0 {
		return s.createStoriesFromProposal(ctx, workspaceID, epicID, actorID, req.ProposedTasks)
	}

	run, err := s.GetAgentRun(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	if run.TargetType != "epic" || run.TargetID != epicID {
		return nil, fmt.Errorf("run does not belong to this epic")
	}

	stage := parsePlanningRunStage(run.Input)
	if stage == "" {
		stage = model.PlanningStagePlanTasks
	}
	if stage != model.PlanningStagePlanTasks {
		return nil, fmt.Errorf("run is not a task planning run")
	}

	existingSummary, _ := decodePlanningRunSummary(run.OutputSummary)
	if run.ApprovalState == "approved" && len(existingSummary.CreatedTaskIDs) > 0 {
		return s.loadCreatedTasks(ctx, existingSummary.CreatedTaskIDs)
	}
	if run.ApprovalState != "pending" {
		return nil, fmt.Errorf("run does not have a pending task plan")
	}
	if s.taskService == nil {
		return nil, fmt.Errorf("task service is not configured")
	}

	epicWithStats, err := s.epicRepo.GetByID(ctx, epicID)
	if err != nil {
		return nil, fmt.Errorf("get epic: %w", err)
	}
	if epicWithStats == nil {
		return nil, fmt.Errorf("epic not found")
	}
	epic := &epicWithStats.Epic
	teamID, err := plannerTaskTeamID(epic)
	if err != nil {
		return nil, err
	}
	workflowID, workflowStateID, err := s.resolvePlanningTaskWorkflow(ctx, workspaceID, teamID)
	if err != nil {
		return nil, err
	}

	var proposal model.OrchestrationProposal
	if existingSummary.Proposal != nil {
		proposal = *existingSummary.Proposal
	} else if err := json.Unmarshal(run.OutputSummary, &proposal); err != nil {
		return nil, fmt.Errorf("task plan output is invalid; regenerate the plan as JSON with \"summary\" and \"proposed_tasks\" or the legacy \"proposed_stories\"")
	}

	proposedStories := req.ProposedTasks
	if len(proposedStories) == 0 {
		proposedStories = proposal.ProposedTasks
	}
	if len(proposedStories) == 0 {
		return nil, fmt.Errorf("task plan must include at least one item in \"proposed_tasks\" or the legacy \"proposed_stories\"")
	}
	if err := validatePlanningTasks(proposedStories); err != nil {
		return nil, err
	}

	enablerCount := 0
	for _, s := range proposedStories {
		if s.SliceType == "enabler" {
			enablerCount++
		}
	}
	if enablerCount > 2 {
		slog.Warn("high enabler count in task plan",
			"enabler_count", enablerCount,
			"total_count", len(proposedStories),
			"epic_id", epicID)
	}

	// Warn on overlapping file modifications across non-dependent tasks.
	fileOwners := map[string]string{} // path -> task ref
	for _, ps := range proposedStories {
		if ps.ImplementationBrief == nil {
			continue
		}
		depSet := make(map[string]bool, len(ps.DependencyRefs))
		for _, d := range ps.DependencyRefs {
			depSet[d] = true
		}
		for _, f := range ps.ImplementationBrief.FilesToModify {
			if owner, exists := fileOwners[f.Path]; exists && !depSet[owner] {
				slog.Warn("overlapping file modification without dependency edge",
					"file", f.Path, "task_a", owner, "task_b", ps.Ref, "epic_id", epicID)
			}
			if _, exists := fileOwners[f.Path]; !exists {
				fileOwners[f.Path] = ps.Ref
			}
		}
	}

	externalIDs := make([]string, 0, len(proposedStories))
	for idx, ps := range proposedStories {
		externalIDs = append(externalIDs, planningTaskExternalID(runID, idx, ps.Ref))
	}
	existingTasks, err := s.taskRepo.ListByEpicAndExternalIDs(ctx, workspaceID, epicID, externalIDs)
	if err != nil {
		return nil, fmt.Errorf("lookup existing planned tasks: %w", err)
	}
	existingByExternalID := make(map[string]model.PMTask, len(existingTasks))
	for _, task := range existingTasks {
		if task.ExternalID == nil || strings.TrimSpace(*task.ExternalID) == "" {
			continue
		}
		existingByExternalID[*task.ExternalID] = task
	}

	created := make([]model.PMTask, 0, len(proposedStories))
	createdIDs := make([]string, 0, len(proposedStories))
	createdDetails := make([]createdPlanningTask, 0, len(proposedStories))
	refToTask := make(map[string]model.PMTask, len(proposedStories))

	for idx, ps := range proposedStories {
		taskType := strings.ToLower(strings.TrimSpace(ps.TaskType))
		if !isValidTaskType(taskType) {
			taskType = model.PMTaskTypeFeature
		}

		externalID := planningTaskExternalID(runID, idx, ps.Ref)
		var detail *model.TaskDetail
		if existing, ok := existingByExternalID[externalID]; ok {
			detail, err = s.taskService.GetByID(ctx, existing.ID)
			if err != nil {
				return nil, fmt.Errorf("reload existing task %d: %w", idx+1, err)
			}
		} else {
			desc := renderPlannedTaskDescription(ps)
			detail, err = s.taskService.Create(ctx, model.CreateTaskRequest{
				WorkspaceID:     workspaceID,
				Name:            strings.TrimSpace(ps.Name),
				Description:     strPtr(desc),
				TaskType:       taskType,
				WorkflowID:      workflowID,
				WorkflowStateID: workflowStateID,
				EpicID:          &epicID,
				TeamID:          teamID,
				Estimate:        ps.Estimate,
				Priority:        ps.Priority,
				ExternalID:      strPtr(externalID),
			}, actorID)
			if err != nil {
				return nil, fmt.Errorf("create task %d: %w", idx+1, err)
			}
		}

		var briefJSON json.RawMessage
		briefFields := map[string]interface{}{}
		if ps.SliceType != "" {
			briefFields["slice_type"] = ps.SliceType
		}
		if ps.ImplementationBrief != nil {
			if b, marshalErr := json.Marshal(ps.ImplementationBrief); marshalErr != nil {
				slog.WarnContext(ctx, "marshal implementation brief", "error", marshalErr, "task_ref", ps.Ref)
			} else {
				briefFields["implementation_brief"] = b
				briefJSON = b
			}
		}
		if len(briefFields) > 0 {
			if err := s.taskRepo.UpdateFields(ctx, detail.Task.ID, briefFields); err != nil {
				slog.WarnContext(ctx, "failed to persist task brief fields",
					"task_id", detail.Task.ID, "error", err)
			}
		}

		if ps.AssignAgentID != nil && strings.TrimSpace(*ps.AssignAgentID) != "" && isValidUUID(*ps.AssignAgentID) {
			if err := s.AssignAgentToTask(ctx, workspaceID, detail.Task.ID, *ps.AssignAgentID, actorID); err != nil {
				slog.WarnContext(ctx, "skipping agent assignment for planned task",
					"task", detail.Task.Name, "agent_id", *ps.AssignAgentID, "error", err)
			} else {
				detail, err = s.taskService.GetByID(ctx, detail.Task.ID)
				if err != nil {
					return nil, fmt.Errorf("reload task %q: %w", ps.Name, err)
				}
			}
		}

		created = append(created, detail.Task)
		createdIDs = append(createdIDs, detail.Task.ID)
		createdDetails = append(createdDetails, createdPlanningTask{
			TaskID:              detail.Task.ID,
			Ref:                 ps.Ref,
			Name:                detail.Task.Name,
			TaskType:            detail.Task.TaskType,
			Estimate:            detail.Task.Estimate,
			Priority:            ps.Priority,
			AcceptanceCriteria:  slices.Clone(ps.AcceptanceCriteria),
			DependencyRefs:      slices.Clone(ps.DependencyRefs),
			SourceRefs:          append([]model.PlanningSourceRef(nil), ps.SourceRefs...),
			ImplementationBrief: briefJSON,
		})
		if strings.TrimSpace(ps.Ref) != "" {
			refToTask[ps.Ref] = detail.Task
		}
	}

	for _, ps := range proposedStories {
		targetTask, ok := refToTask[ps.Ref]
		if !ok || len(ps.DependencyRefs) == 0 {
			continue
		}
		for _, depRef := range ps.DependencyRefs {
			sourceTask, exists := refToTask[depRef]
			if !exists {
				return nil, fmt.Errorf("task %q references unknown dependency ref %q in dependency_refs", strings.TrimSpace(ps.Name), depRef)
			}
			if err := s.taskLinkRepo.Create(ctx, &model.PMTaskLink{
				WorkspaceID:   workspaceID,
				SourceTaskID: sourceTask.ID,
				TargetTaskID: targetTask.ID,
				LinkType:      model.PMTaskLinkTypeBlocks,
				CreatedBy:     actorID,
			}); err != nil {
				return nil, err
			}
		}
	}

	handoffContext, _ := json.Marshal(map[string]any{
		"created_task_ids":   createdIDs,
		"created_task_count": len(created),
		"epic_id":            epicID,
		"run_id":             runID,
	})
	handoff := &model.AgentHandoff{
		WorkspaceID: workspaceID,
		FromAgentID: &run.AgentID,
		EpicID:      &epicID,
		RunID:       &run.ID,
		HandoffType: "agent_to_human",
		Reason:      fmt.Sprintf("Confirmed task plan and created %d tasks", len(created)),
		Context:     handoffContext,
	}
	_ = s.handoffRepo.Create(ctx, handoff)

	proposal.EpicID = epicID
	if proposal.SpecVersionID == "" && epic.ApprovedSpecVersionID != nil {
		proposal.SpecVersionID = *epic.ApprovedSpecVersionID
	}
	proposal.ProposedTasks = proposedStories
	runSummary := epicPlanningRunSummary{
		Stage:               model.PlanningStagePlanTasks,
		SpecDocumentID:      derefString(epic.SpecDocumentID),
		SpecVersionID:       derefString(epic.ApprovedSpecVersionID),
		PlanningMethodology: parsePlanningMethodology(run.Input),
		Summary:             proposal.Summary,
		OpenQuestions:       proposal.OpenQuestions,
		Risks:               proposal.Risks,
		Proposal:            &proposal,
		CreatedTaskIDs:      createdIDs,
		CreatedTasks:        createdDetails,
	}
	outputSummary, _ := json.Marshal(runSummary)
	run.OutputSummary = outputSummary
	run.ApprovalState = "approved"
	if model.IsAgentRunPausedStatus(run.Status) && run.PauseReason == model.AgentRunPauseReasonHumanApproval {
		now := time.Now()
		run.Status = "completed"
		run.PauseReason = model.AgentRunPauseReasonNone
		run.CompletedAt = &now
		run.ExecutionStage = strPtr("approved")
	}
	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, err
	}

	epic.LastPlanningRunID = &run.ID
	if err := s.epicRepo.Update(ctx, epic); err != nil {
		return nil, err
	}

	summary := fmt.Sprintf("Created %d tasks from the approved plan.", len(created))
	_ = s.saveArtifact(ctx, run, "handoff_note", "markdown", summary, 999998)
	_ = s.runEngine.SignalApprove(ctx, derefString(run.WorkflowID), derefString(run.WorkflowRunID))
	s.publishRunEvent(run, actorID)

	return created, nil
}

// createStoriesFromProposal creates tasks from proposed items without requiring an agent run.
// Used by the interactive task planning path.
func (s *AgentService) createStoriesFromProposal(ctx context.Context, workspaceID, epicID, actorID string, proposedStories []model.ProposedTask) ([]model.PMTask, error) {
	if s.taskService == nil {
		return nil, fmt.Errorf("task service is not configured")
	}
	if len(proposedStories) == 0 {
		return nil, fmt.Errorf("task plan must include at least one item in \"proposed_tasks\" or the legacy \"proposed_stories\"")
	}
	if err := validatePlanningTasks(proposedStories); err != nil {
		return nil, err
	}

	epicWithStats, err := s.epicRepo.GetByID(ctx, epicID)
	if err != nil {
		return nil, fmt.Errorf("get epic: %w", err)
	}
	if epicWithStats == nil {
		return nil, fmt.Errorf("epic not found")
	}
	epic := &epicWithStats.Epic
	teamID, err := plannerTaskTeamID(epic)
	if err != nil {
		return nil, err
	}
	workflowID, workflowStateID, err := s.resolvePlanningTaskWorkflow(ctx, workspaceID, teamID)
	if err != nil {
		return nil, err
	}

	// Use a stable key for external IDs in the interactive path.
	syntheticRunID := "interactive-" + epicID

	externalIDs := make([]string, 0, len(proposedStories))
	for idx, ps := range proposedStories {
		externalIDs = append(externalIDs, planningTaskExternalID(syntheticRunID, idx, ps.Ref))
	}
	existingTasks, err := s.taskRepo.ListByEpicAndExternalIDs(ctx, workspaceID, epicID, externalIDs)
	if err != nil {
		return nil, fmt.Errorf("lookup existing planned tasks: %w", err)
	}
	existingByExternalID := make(map[string]model.PMTask, len(existingTasks))
	for _, task := range existingTasks {
		if task.ExternalID == nil || strings.TrimSpace(*task.ExternalID) == "" {
			continue
		}
		existingByExternalID[*task.ExternalID] = task
	}

	created := make([]model.PMTask, 0, len(proposedStories))
	refToTask := make(map[string]model.PMTask, len(proposedStories))

	for idx, ps := range proposedStories {
		taskType := strings.ToLower(strings.TrimSpace(ps.TaskType))
		if !isValidTaskType(taskType) {
			taskType = model.PMTaskTypeFeature
		}

		externalID := planningTaskExternalID(syntheticRunID, idx, ps.Ref)
		var detail *model.TaskDetail
		if existing, ok := existingByExternalID[externalID]; ok {
			detail, err = s.taskService.GetByID(ctx, existing.ID)
			if err != nil {
				return nil, fmt.Errorf("reload existing task %d: %w", idx+1, err)
			}
		} else {
			desc := renderPlannedTaskDescription(ps)
			detail, err = s.taskService.Create(ctx, model.CreateTaskRequest{
				WorkspaceID:     workspaceID,
				Name:            strings.TrimSpace(ps.Name),
				Description:     strPtr(desc),
				TaskType:       taskType,
				WorkflowID:      workflowID,
				WorkflowStateID: workflowStateID,
				EpicID:          &epicID,
				TeamID:          teamID,
				Estimate:        ps.Estimate,
				Priority:        ps.Priority,
				ExternalID:      strPtr(externalID),
			}, actorID)
			if err != nil {
				return nil, fmt.Errorf("create task %d: %w", idx+1, err)
			}
		}

		briefFields := map[string]interface{}{}
		if ps.SliceType != "" {
			briefFields["slice_type"] = ps.SliceType
		}
		if ps.ImplementationBrief != nil {
			if b, marshalErr := json.Marshal(ps.ImplementationBrief); marshalErr != nil {
				slog.WarnContext(ctx, "marshal implementation brief", "error", marshalErr, "task_ref", ps.Ref)
			} else {
				briefFields["implementation_brief"] = b
			}
		}
		if len(briefFields) > 0 {
			if err := s.taskRepo.UpdateFields(ctx, detail.Task.ID, briefFields); err != nil {
				slog.WarnContext(ctx, "failed to persist task brief fields",
					"task_id", detail.Task.ID, "error", err)
			}
		}

		if ps.AssignAgentID != nil && strings.TrimSpace(*ps.AssignAgentID) != "" && isValidUUID(*ps.AssignAgentID) {
			if err := s.AssignAgentToTask(ctx, workspaceID, detail.Task.ID, *ps.AssignAgentID, actorID); err != nil {
				slog.WarnContext(ctx, "skipping agent assignment for planned task",
					"task", detail.Task.Name, "agent_id", *ps.AssignAgentID, "error", err)
			} else {
				detail, err = s.taskService.GetByID(ctx, detail.Task.ID)
				if err != nil {
					return nil, fmt.Errorf("reload task %q: %w", ps.Name, err)
				}
			}
		}

		created = append(created, detail.Task)
		if strings.TrimSpace(ps.Ref) != "" {
			refToTask[ps.Ref] = detail.Task
		}
	}

	for _, ps := range proposedStories {
		targetTask, ok := refToTask[ps.Ref]
		if !ok || len(ps.DependencyRefs) == 0 {
			continue
		}
		for _, depRef := range ps.DependencyRefs {
			sourceTask, exists := refToTask[depRef]
			if !exists {
				return nil, fmt.Errorf("task %q references unknown dependency ref %q in dependency_refs", strings.TrimSpace(ps.Name), depRef)
			}
			if err := s.taskLinkRepo.Create(ctx, &model.PMTaskLink{
				WorkspaceID:   workspaceID,
				SourceTaskID: sourceTask.ID,
				TargetTaskID: targetTask.ID,
				LinkType:      model.PMTaskLinkTypeBlocks,
				CreatedBy:     actorID,
			}); err != nil {
				return nil, err
			}
		}
	}

	if err := s.epicRepo.Update(ctx, epic); err != nil {
		return nil, err
	}

	return created, nil
}

// CreateEpicTaskBatch creates tasks from planner tool input.
func (s *AgentService) CreateEpicTaskBatch(ctx context.Context, workspaceID, epicID, actorID string, proposedStories []model.ProposedTask) ([]model.PMTask, error) {
	return s.createStoriesFromProposal(ctx, workspaceID, epicID, actorID, proposedStories)
}

func planningTaskExternalID(runID string, index int, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref != "" {
		return fmt.Sprintf("planning:%s:%s", runID, ref)
	}
	return fmt.Sprintf("planning:%s:%03d", runID, index+1)
}

// EnsureEpicSpecDocument ensures the epic has a canonical product spec doc and returns it.
func (s *AgentService) EnsureEpicSpecDocument(ctx context.Context, workspaceID, epicID, actorID string) (*model.DocsDocument, error) {
	epicWithStats, err := s.epicRepo.GetByID(ctx, epicID)
	if err != nil {
		return nil, fmt.Errorf("get epic: %w", err)
	}
	if epicWithStats == nil || epicWithStats.Epic.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("epic not found")
	}
	return s.ensureEpicSpecDocument(ctx, workspaceID, &epicWithStats.Epic, actorID)
}

func (s *AgentService) ensureEpicSpecDocument(ctx context.Context, workspaceID string, epic *model.PMEpic, actorID string) (*model.DocsDocument, error) {
	if epic.SpecDocumentID != nil && strings.TrimSpace(*epic.SpecDocumentID) != "" {
		doc, err := s.docsDocumentRepo.GetByID(ctx, *epic.SpecDocumentID)
		if err != nil {
			return nil, err
		}
		if doc != nil {
			return doc, nil
		}
	}

	space, err := s.docsSpaceRepo.GetBySlug(ctx, workspaceID, productSpecsSpaceSlug)
	if err != nil {
		return nil, err
	}
	if space == nil {
		space, err = s.docsSpaceRepo.Create(ctx, &model.DocsSpace{
			WorkspaceID: workspaceID,
			Name:        productSpecsSpaceName,
			Slug:        productSpecsSpaceSlug,
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeInternal,
			IsSystem:    true,
			Position:    2,
			CreatedBy:   actorID,
		})
		if err != nil {
			return nil, fmt.Errorf("create product specs space: %w", err)
		}
	}

	teamID := epic.TeamID
	if teamID == nil {
		teamID = strPtr(actorID)
	}

	doc, err := s.docsDocumentRepo.Create(ctx, &model.DocsDocument{
		WorkspaceID: workspaceID,
		SpaceID:     space.ID,
		Title:       strings.TrimSpace(epic.Name) + " Product Spec",
		Status:      model.DocStatusDraft,
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		OwnerID:     strPtr(actorID),
		TeamID:      teamID,
		TemplateKey: strPtr("product_spec"),
		Tags:        model.DocsStringArray{"product-spec", "epic"},
		CreatedBy:   actorID,
	})
	if err != nil {
		return nil, err
	}

	epic.SpecDocumentID = &doc.ID
	if err := s.epicRepo.Update(ctx, epic); err != nil {
		return nil, err
	}

	if err := s.ensureEpicSpecLink(ctx, workspaceID, doc.ID, epic.ID, actorID); err != nil {
		return nil, err
	}

	return doc, nil
}

// EnsureTaskPlanDocument ensures the task has a canonical planning doc and returns it.
func (s *AgentService) EnsureTaskPlanDocument(ctx context.Context, workspaceID, taskID, actorID string) (*model.DocsDocument, error) {
	task, err := s.taskRepo.GetRawByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	if task == nil || task.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("task not found")
	}
	var epic *model.PMEpic
	if task.EpicID != nil && strings.TrimSpace(*task.EpicID) != "" {
		epicWithStats, err := s.epicRepo.GetByID(ctx, *task.EpicID)
		if err != nil {
			return nil, fmt.Errorf("get parent epic: %w", err)
		}
		if epicWithStats != nil {
			epic = &epicWithStats.Epic
		}
	}
	return s.ensureTaskPlanDocument(ctx, workspaceID, task, epic, actorID)
}

func (s *AgentService) ensureTaskPlanDocument(ctx context.Context, workspaceID string, task *model.PMTask, epic *model.PMEpic, actorID string) (*model.DocsDocument, error) {
	if task.PlanDocumentID != nil && strings.TrimSpace(*task.PlanDocumentID) != "" {
		doc, err := s.docsDocumentRepo.GetByID(ctx, *task.PlanDocumentID)
		if err != nil {
			return nil, err
		}
		if doc != nil {
			if err := s.ensureTaskPlanLink(ctx, workspaceID, doc.ID, task.ID, actorID); err != nil {
				return nil, err
			}
			return doc, nil
		}
	}

	space, err := s.docsSpaceRepo.GetBySlug(ctx, workspaceID, productSpecsSpaceSlug)
	if err != nil {
		return nil, err
	}
	if space == nil {
		space, err = s.docsSpaceRepo.Create(ctx, &model.DocsSpace{
			WorkspaceID: workspaceID,
			Name:        productSpecsSpaceName,
			Slug:        productSpecsSpaceSlug,
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeInternal,
			IsSystem:    true,
			Position:    2,
			CreatedBy:   actorID,
		})
		if err != nil {
			return nil, fmt.Errorf("create product specs space: %w", err)
		}
	}

	teamID := task.TeamID
	if teamID == nil && epic != nil {
		teamID = epic.TeamID
	}
	if teamID == nil {
		teamID = strPtr(actorID)
	}

	title := strings.TrimSpace(task.Name) + " Plan"
	doc, err := s.docsDocumentRepo.Create(ctx, &model.DocsDocument{
		WorkspaceID: workspaceID,
		SpaceID:     space.ID,
		Title:       title,
		Status:      model.DocStatusDraft,
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		OwnerID:     strPtr(actorID),
		TeamID:      teamID,
		TemplateKey: strPtr("task_plan"),
		Tags:        model.DocsStringArray{"task-plan", "task"},
		CreatedBy:   actorID,
	})
	if err != nil {
		return nil, err
	}

	task.PlanDocumentID = &doc.ID
	if err := s.taskRepo.Update(ctx, task); err != nil {
		return nil, err
	}

	if err := s.ensureTaskPlanLink(ctx, workspaceID, doc.ID, task.ID, actorID); err != nil {
		return nil, err
	}

	return doc, nil
}

func (s *AgentService) ensureEpicSpecLink(ctx context.Context, workspaceID, documentID, epicID, actorID string) error {
	links, err := s.docsLinkRepo.ListByObject(ctx, workspaceID, model.LinkedObjectEpic, epicID)
	if err != nil {
		return err
	}
	for _, link := range links {
		if link.DocumentID == documentID {
			return nil
		}
	}
	_, err = s.docsLinkRepo.Create(ctx, &model.DocsLink{
		WorkspaceID:      workspaceID,
		DocumentID:       documentID,
		LinkedObjectType: model.LinkedObjectEpic,
		LinkedObjectID:   epicID,
		LinkContext:      model.LinkContextCreatedFrom,
		CreatedBy:        actorID,
	})
	return err
}

func (s *AgentService) ensureTaskPlanLink(ctx context.Context, workspaceID, documentID, taskID, actorID string) error {
	links, err := s.docsLinkRepo.ListByObject(ctx, workspaceID, model.LinkedObjectTask, taskID)
	if err != nil {
		return err
	}
	for _, link := range links {
		if link.DocumentID == documentID {
			return nil
		}
	}
	_, err = s.docsLinkRepo.Create(ctx, &model.DocsLink{
		WorkspaceID:      workspaceID,
		DocumentID:       documentID,
		LinkedObjectType: model.LinkedObjectTask,
		LinkedObjectID:   taskID,
		LinkContext:      model.LinkContextCreatedFrom,
		CreatedBy:        actorID,
	})
	return err
}

func (s *AgentService) approveActiveEpicPlanningRun(ctx context.Context, workspaceID, epicID, actorID, stage string) error {
	activeRun, err := s.runRepo.FindActiveByTarget(ctx, workspaceID, "epic", epicID)
	if err != nil || activeRun == nil {
		return err
	}
	model.NormalizeAgentRunPauseState(activeRun)
	if activeRun.Status != model.AgentRunStatusPaused || activeRun.PauseReason != model.AgentRunPauseReasonHumanApproval || activeRun.ApprovalState != "pending" {
		return nil
	}
	if parsePlanningRunStage(activeRun.Input) != stage {
		return nil
	}

	now := time.Now()
	activeRun.ApprovalState = "approved"
	activeRun.Status = "completed"
	activeRun.PauseReason = model.AgentRunPauseReasonNone
	activeRun.CompletedAt = &now
	activeRun.ExecutionStage = strPtr("approved")
	if err := s.runRepo.Update(ctx, activeRun); err != nil {
		return err
	}
	_ = s.runEngine.SignalApprove(ctx, derefString(activeRun.WorkflowID), derefString(activeRun.WorkflowRunID))
	s.publishRunEvent(activeRun, actorID)
	return nil
}

func loadPlanningTasksByID(ctx context.Context, taskRepo interface {
	GetRawByID(ctx context.Context, id string) (*model.PMTask, error)
}, ids []string) ([]model.PMTask, error) {
	tasks := make([]model.PMTask, 0, len(ids))
	for _, id := range ids {
		task, err := taskRepo.GetRawByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if task != nil {
			tasks = append(tasks, *task)
		}
	}
	return tasks, nil
}

func (s *AgentService) loadCreatedTasks(ctx context.Context, ids []string) ([]model.PMTask, error) {
	return loadPlanningTasksByID(ctx, s.taskRepo, ids)
}

func validatePlanningTasks(stories []model.ProposedTask) error {
	if err := model.NormalizeProposedTasks(stories); err != nil {
		return err
	}
	for idx := range stories {
		stories[idx].TaskType = normalizePlannedTaskType(stories[idx].TaskType)
		if stories[idx].Priority != nil {
			normalizedPriority := normalizePlannedTaskPriority(*stories[idx].Priority)
			stories[idx].Priority = &normalizedPriority
		}
	}
	return nil
}

func plannerTaskTeamID(epic *model.PMEpic) (*string, error) {
	if epic == nil {
		return nil, fmt.Errorf("epic is required")
	}
	if epic.TeamID == nil || strings.TrimSpace(*epic.TeamID) == "" {
		return nil, fmt.Errorf("epic %q must have a team before tasks can be created", strings.TrimSpace(epic.Name))
	}
	teamID := strings.TrimSpace(*epic.TeamID)
	return &teamID, nil
}

func (s *AgentService) resolvePlanningTaskWorkflow(ctx context.Context, workspaceID string, teamID *string) (string, string, error) {
	if workspaceID == "" {
		return "", "", fmt.Errorf("workspace_id is required")
	}

	if teamID != nil && strings.TrimSpace(*teamID) != "" {
		normalizedTeamID := strings.TrimSpace(*teamID)
		if s.workflowService != nil {
			workflow, err := s.workflowService.ResolveTeamWorkflow(ctx, workspaceID, normalizedTeamID)
			if err != nil {
				return "", "", fmt.Errorf("resolve team workflow: %w", err)
			}
			if workflow != nil {
				stateID := ""
				if workflow.Workflow.DefaultStateID != nil {
					stateID = strings.TrimSpace(*workflow.Workflow.DefaultStateID)
				}
				if stateID == "" && len(workflow.States) > 0 {
					stateID = workflow.States[0].ID
				}
				if strings.TrimSpace(workflow.Workflow.ID) == "" || stateID == "" {
					return "", "", fmt.Errorf("resolved team workflow is missing a default state")
				}
				return workflow.Workflow.ID, stateID, nil
			}
		}

		// Defensive fallback for worker paths that forgot to inject PMWorkflowService.
		if s.taskService != nil && s.taskService.workflowRepo != nil {
			workflow, err := s.taskService.workflowRepo.GetByTeamID(ctx, workspaceID, normalizedTeamID)
			if err != nil {
				return "", "", fmt.Errorf("lookup team workflow: %w", err)
			}
			if workflow != nil {
				stateID := ""
				if workflow.Workflow.DefaultStateID != nil {
					stateID = strings.TrimSpace(*workflow.Workflow.DefaultStateID)
				}
				if stateID == "" && len(workflow.States) > 0 {
					stateID = workflow.States[0].ID
				}
				if strings.TrimSpace(workflow.Workflow.ID) == "" || stateID == "" {
					return "", "", fmt.Errorf("team workflow is missing a default state")
				}
				slog.WarnContext(ctx, "resolved planning workflow via repository fallback",
					"workspace_id", workspaceID,
					"team_id", normalizedTeamID,
					"workflow_id", workflow.Workflow.ID)
				return workflow.Workflow.ID, stateID, nil
			}
		}
	}

	if s.taskService == nil || s.taskService.workflowRepo == nil {
		return "", "", fmt.Errorf("workflow service is not configured")
	}

	defaultWorkflow, err := s.taskService.workflowRepo.GetDefaultWorkflow(ctx, workspaceID)
	if err != nil {
		return "", "", fmt.Errorf("resolve default workflow: %w", err)
	}
	if defaultWorkflow == nil {
		defaultWorkflow, err = s.taskService.workflowRepo.SeedDefaultWorkflow(ctx, workspaceID)
		if err != nil {
			return "", "", fmt.Errorf("seed default workflow: %w", err)
		}
	}
	if defaultWorkflow == nil {
		return "", "", fmt.Errorf("default workflow not found")
	}

	stateID := ""
	if defaultWorkflow.Workflow.DefaultStateID != nil {
		stateID = strings.TrimSpace(*defaultWorkflow.Workflow.DefaultStateID)
	}
	if stateID == "" && len(defaultWorkflow.States) > 0 {
		stateID = defaultWorkflow.States[0].ID
	}
	if strings.TrimSpace(defaultWorkflow.Workflow.ID) == "" || stateID == "" {
		return "", "", fmt.Errorf("default workflow is missing a default state")
	}
	return defaultWorkflow.Workflow.ID, stateID, nil
}

func normalizePlannedTaskType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "feature", "feat", "user_story", "story", "task":
		return model.PMTaskTypeFeature
	case "bug", "fix", "defect":
		return model.PMTaskTypeBug
	case "chore", "maintenance", "infra", "infrastructure", "ops":
		return model.PMTaskTypeChore
	default:
		return model.PMTaskTypeFeature
	}
}

func normalizePlannedTaskPriority(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "none", "no priority", "n/a", "na", "unset":
		return model.PMTaskPriorityNone
	case "low", "p3", "minor":
		return model.PMTaskPriorityLow
	case "medium", "med", "normal", "default", "p2":
		return model.PMTaskPriorityMedium
	case "high", "important", "p1":
		return model.PMTaskPriorityHigh
	case "urgent", "critical", "blocker", "highest", "p0":
		return model.PMTaskPriorityUrgent
	default:
		return model.PMTaskPriorityNone
	}
}

func filterNonEmptyStrings(values []string) []string {
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

func renderPlannedTaskDescription(proposed model.ProposedTask) string {
	var sections []string
	if summary := strings.TrimSpace(proposed.Description); summary != "" {
		sections = append(sections, "## Summary\n"+summary)
	}
	if len(proposed.AcceptanceCriteria) > 0 {
		var lines []string
		for _, item := range proposed.AcceptanceCriteria {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			lines = append(lines, "- "+item)
		}
		if len(lines) > 0 {
			sections = append(sections, "## Acceptance Criteria\n"+strings.Join(lines, "\n"))
		}
	}
	if len(proposed.DependencyRefs) > 0 {
		lines := make([]string, 0, len(proposed.DependencyRefs))
		for _, dep := range proposed.DependencyRefs {
			lines = append(lines, "- Depends on `"+dep+"`")
		}
		sections = append(sections, "## Dependencies\n"+strings.Join(lines, "\n"))
	}
	if len(proposed.SourceRefs) > 0 {
		lines := make([]string, 0, len(proposed.SourceRefs))
		for _, ref := range proposed.SourceRefs {
			label := strings.TrimSpace(ref.Title)
			if label == "" {
				label = strings.TrimSpace(ref.Type)
			}
			if ref.ID != "" {
				label = fmt.Sprintf("%s (%s)", label, ref.ID)
			}
			lines = append(lines, "- "+label)
		}
		sections = append(sections, "## Spec Traceability\n"+strings.Join(lines, "\n"))
	}
	if len(sections) == 0 {
		return ""
	}
	markdown := strings.Join(sections, "\n\n")
	rendered, err := tiptap.RenderHTML(tiptap.MarkdownToJSON(markdown))
	if err != nil {
		slog.Warn("failed to render planned task markdown to html", "error", err)
		return markdown
	}
	return strings.TrimSpace(rendered)
}

func validateSpecClarifications(updated, current []model.SpecClarificationItem) ([]model.SpecClarificationItem, int, error) {
	if len(current) == 0 {
		return []model.SpecClarificationItem{}, 0, nil
	}
	if len(updated) != len(current) {
		return nil, 0, fmt.Errorf("clarification responses must include every open question and assumption")
	}

	currentByID := make(map[string]model.SpecClarificationItem, len(current))
	for _, item := range current {
		currentByID[item.ID] = item
	}

	normalized := make([]model.SpecClarificationItem, 0, len(updated))
	pendingCount := 0
	for _, item := range updated {
		base, ok := currentByID[strings.TrimSpace(item.ID)]
		if !ok {
			return nil, 0, fmt.Errorf("clarification %q is not part of the current spec draft", item.ID)
		}

		next := model.SpecClarificationItem{
			ID:          base.ID,
			Kind:        base.Kind,
			Prompt:      base.Prompt,
			Disposition: model.NormalizeSpecClarificationDisposition(base.Kind, item.Disposition),
			Response:    strings.TrimSpace(item.Response),
		}

		switch next.Kind {
		case model.SpecClarificationKindOpenQuestion:
			if next.Disposition != model.SpecClarificationDispositionAnswered {
				next.Disposition = model.SpecClarificationDispositionPending
			}
			if next.Disposition == model.SpecClarificationDispositionAnswered && next.Response == "" {
				return nil, 0, fmt.Errorf("answer is required for open question %q", next.Prompt)
			}
		case model.SpecClarificationKindAssumption:
			if next.Disposition == model.SpecClarificationDispositionRejected && next.Response == "" {
				return nil, 0, fmt.Errorf("explanation is required when rejecting assumption %q", next.Prompt)
			}
		default:
			return nil, 0, fmt.Errorf("clarification %q has an invalid kind", next.ID)
		}

		if !model.SpecClarificationResolved(next) {
			pendingCount++
		}
		normalized = append(normalized, next)
	}

	return normalized, pendingCount, nil
}

func countPendingSpecClarifications(items []model.SpecClarificationItem) int {
	count := 0
	for _, item := range items {
		if !model.SpecClarificationResolved(item) {
			count++
		}
	}
	return count
}

func resolvedSpecClarifications(items []model.SpecClarificationItem) []model.SpecClarificationItem {
	resolved := make([]model.SpecClarificationItem, 0, len(items))
	for _, item := range items {
		if model.SpecClarificationResolved(item) {
			resolved = append(resolved, item)
		}
	}
	return resolved
}

func renderSpecClarificationsBrief(items []model.SpecClarificationItem) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		switch item.Kind {
		case model.SpecClarificationKindOpenQuestion:
			lines = append(lines, fmt.Sprintf("- %s -> %s", item.Prompt, item.Response))
		case model.SpecClarificationKindAssumption:
			if item.Disposition == model.SpecClarificationDispositionAccepted {
				lines = append(lines, fmt.Sprintf("- %s -> accepted", item.Prompt))
			} else {
				lines = append(lines, fmt.Sprintf("- %s -> rejected: %s", item.Prompt, item.Response))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func (s *AgentService) syncClarificationsIntoSpecDoc(ctx context.Context, documentID, actorID string, clarifications []model.SpecClarificationItem) error {
	if len(clarifications) == 0 {
		return nil
	}

	content, err := s.docsContentRepo.GetByDocumentID(ctx, documentID)
	if err != nil {
		return err
	}
	if content == nil || strings.TrimSpace(content.ContentText) == "" {
		return fmt.Errorf("spec document has no content to clarify")
	}

	updatedMarkdown := upsertSpecClarificationsSection(content.ContentText, clarifications)
	savedContent, err := s.docsContentRepo.Upsert(ctx, documentID, tiptap.MarkdownToJSON(updatedMarkdown))
	if err != nil {
		return err
	}

	label := "Clarified Spec"
	_, err = s.docsVersionRepo.Create(ctx, documentID, actorID, savedContent.Content, savedContent.ContentText, &label, "manual", len(strings.Fields(savedContent.ContentText)))
	return err
}

func upsertSpecClarificationsSection(markdown string, clarifications []model.SpecClarificationItem) string {
	markdown = strings.TrimSpace(markdown)
	if idx := strings.Index(markdown, "\n## Clarifications"); idx >= 0 {
		markdown = strings.TrimSpace(markdown[:idx])
	}

	section := renderSpecClarificationsSection(clarifications)
	if section == "" {
		return markdown
	}
	if markdown == "" {
		return section
	}
	return markdown + "\n\n" + section
}

func renderSpecClarificationsSection(clarifications []model.SpecClarificationItem) string {
	resolved := resolvedSpecClarifications(clarifications)
	if len(resolved) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "## Clarifications")

	var answered []string
	var assumptions []string
	for _, item := range resolved {
		switch item.Kind {
		case model.SpecClarificationKindOpenQuestion:
			answered = append(answered, fmt.Sprintf("- %s -> %s", item.Prompt, item.Response))
		case model.SpecClarificationKindAssumption:
			if item.Disposition == model.SpecClarificationDispositionAccepted {
				assumptions = append(assumptions, fmt.Sprintf("- %s -> accepted", item.Prompt))
			} else {
				assumptions = append(assumptions, fmt.Sprintf("- %s -> rejected: %s", item.Prompt, item.Response))
			}
		}
	}

	if len(answered) > 0 {
		lines = append(lines, "### Open Questions Resolved")
		lines = append(lines, answered...)
	}
	if len(assumptions) > 0 {
		lines = append(lines, "### Assumptions Reviewed")
		lines = append(lines, assumptions...)
	}

	return strings.Join(lines, "\n")
}

func truncateString(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "\n... (truncated)"
}

func parsePlanningRunStage(input json.RawMessage) string {
	var payload planningRunInput
	if err := json.Unmarshal(input, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Stage)
}

func parsePlanningMethodology(input json.RawMessage) string {
	var payload planningRunInput
	if err := json.Unmarshal(input, &payload); err != nil {
		return model.PlanningMethodologyStructuredV1
	}
	return normalizePlanningMethodology(payload.PlanningMethodology)
}

func decodePlanningRunSummary(raw json.RawMessage) (epicPlanningRunSummary, error) {
	var summary epicPlanningRunSummary
	if len(raw) == 0 {
		return summary, fmt.Errorf("empty planning run summary")
	}
	if err := json.Unmarshal(raw, &summary); err != nil {
		return summary, err
	}
	return summary, nil
}

// isValidUUID checks whether s is a valid UUID string.
func isValidUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

// ClearEpicSpecForDocument resets spec approval state on any epic
// whose spec document matches the given document ID.
func (s *AgentService) ClearEpicSpecForDocument(ctx context.Context, documentID string) error {
	links, err := s.docsLinkRepo.ListByDocument(ctx, documentID)
	if err != nil {
		return fmt.Errorf("list links for document %s: %w", documentID, err)
	}
	for _, link := range links {
		if link.LinkedObjectType != "epic" {
			continue
		}
		epic, err := s.epicRepo.GetByID(ctx, link.LinkedObjectID)
		if err != nil {
			slog.ErrorContext(ctx, "fetch epic for spec cleanup", "error", err, "epic_id", link.LinkedObjectID)
			continue
		}
		if epic == nil || epic.Epic.SpecDocumentID == nil || *epic.Epic.SpecDocumentID != documentID {
			continue
		}
		epic.Epic.SpecDocumentID = nil
		epic.Epic.ApprovedSpecVersionID = nil
		if err := s.epicRepo.Update(ctx, &epic.Epic); err != nil {
			slog.ErrorContext(ctx, "clear epic spec state", "error", err, "epic_id", epic.Epic.ID)
			continue
		}
		slog.InfoContext(ctx, "cleared epic spec state after document deletion",
			"epic_id", epic.Epic.ID, "document_id", documentID)
	}
	return nil
}
