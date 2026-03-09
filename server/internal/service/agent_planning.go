package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	"github.com/helpin-ai/helpin/server/internal/worker"
)

const (
	productSpecsSpaceSlug = "product-specs"
	productSpecsSpaceName = "Product Specs"
)

type planningRunInput struct {
	Stage                     string `json:"stage,omitempty"`
	AdditionalContext         string `json:"additional_context,omitempty"`
	SpecDocumentID            string `json:"spec_document_id,omitempty"`
	SpecVersionID             string `json:"spec_version_id,omitempty"`
	PlanningMethodology       string `json:"planning_methodology,omitempty"`
	PlanningWebSearchEnabled  bool   `json:"planning_web_search_enabled,omitempty"`
	PlanningWebSearchProvider string `json:"planning_web_search_provider,omitempty"`
}

type epicPlanningRunSummary struct {
	Stage               string                       `json:"stage"`
	SpecDocumentID      string                       `json:"spec_document_id,omitempty"`
	SpecVersionID       string                       `json:"spec_version_id,omitempty"`
	PlanningMethodology string                       `json:"planning_methodology,omitempty"`
	Summary             string                       `json:"summary,omitempty"`
	Risks               []string                     `json:"risks,omitempty"`
	OpenQuestions       []string                     `json:"open_questions,omitempty"`
	Proposal            *model.OrchestrationProposal `json:"proposal,omitempty"`
	CreatedStoryIDs     []string                     `json:"created_story_ids,omitempty"`
	CreatedStories      []createdPlanningStory       `json:"created_stories,omitempty"`
}

type createdPlanningStory struct {
	StoryID            string                    `json:"story_id"`
	Ref                string                    `json:"ref,omitempty"`
	Name               string                    `json:"name"`
	StoryType          string                    `json:"story_type"`
	Estimate           *int                      `json:"estimate,omitempty"`
	Priority           *string                   `json:"priority,omitempty"`
	AcceptanceCriteria []string                  `json:"acceptance_criteria,omitempty"`
	DependencyRefs     []string                  `json:"dependency_refs,omitempty"`
	SourceRefs         []model.PlanningSourceRef `json:"source_refs,omitempty"`
}

// RunEpicAgent retains backward compatibility and defaults epic planning to draft_spec.
func (s *AgentService) RunEpicAgent(ctx context.Context, workspaceID, epicID, actorID, additionalContext string) (*model.AgentRun, error) {
	return s.DraftEpicSpec(ctx, workspaceID, epicID, actorID, additionalContext)
}

// DraftEpicSpec starts a staged planning run that drafts or refreshes the canonical spec doc.
func (s *AgentService) DraftEpicSpec(ctx context.Context, workspaceID, epicID, actorID, additionalContext string) (*model.AgentRun, error) {
	epicWithStats, err := s.epicRepo.GetByID(ctx, epicID)
	if err != nil {
		return nil, fmt.Errorf("get epic: %w", err)
	}
	if epicWithStats == nil {
		return nil, fmt.Errorf("epic not found")
	}
	epic := &epicWithStats.Epic
	if epic.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("epic not found")
	}
	if epic.OrchestratorAgentID == nil || strings.TrimSpace(*epic.OrchestratorAgentID) == "" {
		return nil, fmt.Errorf("no product planner assigned to this epic")
	}
	if epic.PlanningRepositoryID == nil || strings.TrimSpace(*epic.PlanningRepositoryID) == "" {
		return nil, fmt.Errorf("drafting a product spec requires an epic planning repository")
	}

	specDoc, err := s.ensureEpicSpecDocument(ctx, workspaceID, epic, actorID)
	if err != nil {
		return nil, err
	}

	run, err := s.startEpicPlanningRun(ctx, workspaceID, epic, actorID, *epic.OrchestratorAgentID, planningRunInput{
		Stage:             model.PlanningStageDraftSpec,
		AdditionalContext: strings.TrimSpace(additionalContext),
		SpecDocumentID:    specDoc.ID,
	})
	if err != nil {
		return nil, err
	}

	_ = s.activitySvc.Log(ctx, workspaceID, "epic", epicID, &actorID, "updated", strPtr("planning_stage"), nil, strPtr(model.PlanningStageDraftSpec), nil)
	s.publishRunEvent(run, actorID)

	return run, nil
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
		version, err = s.docsVersionRepo.Create(ctx, *epic.SpecDocumentID, actorID, content.Content, content.ContentText, &label)
		if err != nil {
			return nil, err
		}
	}

	epic.ApprovedSpecVersionID = &version.ID
	epic.PlanningState = model.EpicPlanningStateReadyForStoryPlanning
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
		Stage:          model.PlanningStageDraftSpec,
		SpecDocumentID: *epic.SpecDocumentID,
		SpecVersionID:  version.ID,
	}, nil
}

// PlanEpicStories starts a staged planning run from an approved product spec.
func (s *AgentService) PlanEpicStories(ctx context.Context, workspaceID, epicID, actorID, additionalContext string) (*model.AgentRun, error) {
	epicWithStats, err := s.epicRepo.GetByID(ctx, epicID)
	if err != nil {
		return nil, fmt.Errorf("get epic: %w", err)
	}
	if epicWithStats == nil {
		return nil, fmt.Errorf("epic not found")
	}
	epic := &epicWithStats.Epic
	if epic.OrchestratorAgentID == nil || strings.TrimSpace(*epic.OrchestratorAgentID) == "" {
		return nil, fmt.Errorf("no product planner assigned to this epic")
	}
	if epic.SpecDocumentID == nil || strings.TrimSpace(*epic.SpecDocumentID) == "" {
		return nil, fmt.Errorf("epic does not have a product spec document yet")
	}
	if epic.ApprovedSpecVersionID == nil || strings.TrimSpace(*epic.ApprovedSpecVersionID) == "" {
		return nil, fmt.Errorf("planning requires an approved spec version")
	}
	if epic.PlanningRepositoryID == nil || strings.TrimSpace(*epic.PlanningRepositoryID) == "" {
		return nil, fmt.Errorf("planning requires an epic planning repository")
	}

	run, err := s.startEpicPlanningRun(ctx, workspaceID, epic, actorID, *epic.OrchestratorAgentID, planningRunInput{
		Stage:             model.PlanningStagePlanStories,
		AdditionalContext: strings.TrimSpace(additionalContext),
		SpecDocumentID:    *epic.SpecDocumentID,
		SpecVersionID:     *epic.ApprovedSpecVersionID,
	})
	if err != nil {
		return nil, err
	}

	_ = s.activitySvc.Log(ctx, workspaceID, "epic", epicID, &actorID, "updated", strPtr("planning_stage"), nil, strPtr(model.PlanningStagePlanStories), nil)
	s.publishRunEvent(run, actorID)
	return run, nil
}

// ConfirmEpicRun confirms a story plan, creates stories, and writes dependency links.
func (s *AgentService) ConfirmEpicRun(ctx context.Context, workspaceID, epicID, runID, actorID string, req model.ConfirmPlanningRequest) ([]model.PMStory, error) {
	run, err := s.GetAgentRun(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	if run.TargetType != "epic" || run.TargetID != epicID {
		return nil, fmt.Errorf("run does not belong to this epic")
	}

	stage := parsePlanningRunStage(run.Input)
	if stage == "" {
		stage = model.PlanningStagePlanStories
	}
	if stage != model.PlanningStagePlanStories {
		return nil, fmt.Errorf("run is not a story planning run")
	}

	existingSummary, _ := decodePlanningRunSummary(run.OutputSummary)
	if run.ApprovalState == "approved" && len(existingSummary.CreatedStoryIDs) > 0 {
		return s.loadCreatedStories(ctx, existingSummary.CreatedStoryIDs)
	}
	if run.ApprovalState != "pending" {
		return nil, fmt.Errorf("run does not have a pending story plan")
	}
	if s.storyService == nil {
		return nil, fmt.Errorf("story service is not configured")
	}

	epicWithStats, err := s.epicRepo.GetByID(ctx, epicID)
	if err != nil {
		return nil, fmt.Errorf("get epic: %w", err)
	}
	if epicWithStats == nil {
		return nil, fmt.Errorf("epic not found")
	}
	epic := &epicWithStats.Epic

	var proposal model.OrchestrationProposal
	if existingSummary.Proposal != nil {
		proposal = *existingSummary.Proposal
	} else if err := json.Unmarshal(run.OutputSummary, &proposal); err != nil {
		return nil, fmt.Errorf("run does not contain a valid planning proposal")
	}

	proposedStories := req.ProposedStories
	if len(proposedStories) == 0 {
		proposedStories = proposal.ProposedStories
	}
	if len(proposedStories) == 0 {
		return nil, fmt.Errorf("at least one proposed story is required")
	}
	if err := validatePlanningStories(proposedStories); err != nil {
		return nil, err
	}

	created := make([]model.PMStory, 0, len(proposedStories))
	createdIDs := make([]string, 0, len(proposedStories))
	createdDetails := make([]createdPlanningStory, 0, len(proposedStories))
	refToStory := make(map[string]model.PMStory, len(proposedStories))

	for idx, ps := range proposedStories {
		storyType := strings.TrimSpace(ps.StoryType)
		if storyType == "" {
			storyType = model.PMStoryTypeFeature
		}

		desc := renderPlannedStoryDescription(ps)
		detail, err := s.storyService.Create(ctx, model.CreateStoryRequest{
			WorkspaceID: workspaceID,
			Name:        strings.TrimSpace(ps.Name),
			Description: strPtr(desc),
			StoryType:   storyType,
			EpicID:      &epicID,
			Estimate:    ps.Estimate,
			Priority:    ps.Priority,
		}, actorID)
		if err != nil {
			return nil, fmt.Errorf("create story %d: %w", idx+1, err)
		}

		if ps.AssignAgentID != nil && strings.TrimSpace(*ps.AssignAgentID) != "" {
			if err := s.AssignAgentToStory(ctx, workspaceID, detail.Story.ID, *ps.AssignAgentID, actorID); err != nil {
				return nil, fmt.Errorf("assign agent to story %q: %w", detail.Story.Name, err)
			}
			detail, err = s.storyService.GetByID(ctx, detail.Story.ID)
			if err != nil {
				return nil, fmt.Errorf("reload story %q: %w", ps.Name, err)
			}
		}

		created = append(created, detail.Story)
		createdIDs = append(createdIDs, detail.Story.ID)
		createdDetails = append(createdDetails, createdPlanningStory{
			StoryID:            detail.Story.ID,
			Ref:                ps.Ref,
			Name:               detail.Story.Name,
			StoryType:          detail.Story.StoryType,
			Estimate:           detail.Story.Estimate,
			Priority:           ps.Priority,
			AcceptanceCriteria: slices.Clone(ps.AcceptanceCriteria),
			DependencyRefs:     slices.Clone(ps.DependencyRefs),
			SourceRefs:         append([]model.PlanningSourceRef(nil), ps.SourceRefs...),
		})
		if strings.TrimSpace(ps.Ref) != "" {
			refToStory[ps.Ref] = detail.Story
		}
	}

	for _, ps := range proposedStories {
		targetStory, ok := refToStory[ps.Ref]
		if !ok || len(ps.DependencyRefs) == 0 {
			continue
		}
		for _, depRef := range ps.DependencyRefs {
			sourceStory, exists := refToStory[depRef]
			if !exists {
				return nil, fmt.Errorf("dependency %q does not reference a known story ref", depRef)
			}
			if err := s.storyLinkRepo.Create(ctx, &model.PMStoryLink{
				WorkspaceID:   workspaceID,
				SourceStoryID: sourceStory.ID,
				TargetStoryID: targetStory.ID,
				LinkType:      model.PMStoryLinkTypeBlocks,
				CreatedBy:     actorID,
			}); err != nil {
				return nil, err
			}
			target, err := s.storyRepo.GetRawByID(ctx, targetStory.ID)
			if err == nil && target != nil && !target.Blocked {
				target.Blocked = true
				_ = s.storyRepo.Update(ctx, target)
			}
		}
	}

	handoffContext, _ := json.Marshal(map[string]any{
		"created_story_ids":   createdIDs,
		"created_story_count": len(created),
		"epic_id":             epicID,
		"run_id":              runID,
	})
	handoff := &model.AgentHandoff{
		WorkspaceID: workspaceID,
		FromAgentID: &run.AgentID,
		EpicID:      &epicID,
		RunID:       &run.ID,
		HandoffType: "agent_to_human",
		Reason:      fmt.Sprintf("Confirmed story plan and created %d stories", len(created)),
		Context:     handoffContext,
	}
	_ = s.handoffRepo.Create(ctx, handoff)

	proposal.EpicID = epicID
	if proposal.SpecVersionID == "" && epic.ApprovedSpecVersionID != nil {
		proposal.SpecVersionID = *epic.ApprovedSpecVersionID
	}
	proposal.ProposedStories = proposedStories
	runSummary := epicPlanningRunSummary{
		Stage:               model.PlanningStagePlanStories,
		SpecDocumentID:      derefString(epic.SpecDocumentID),
		SpecVersionID:       derefString(epic.ApprovedSpecVersionID),
		PlanningMethodology: parsePlanningMethodology(run.Input),
		Summary:             proposal.Summary,
		OpenQuestions:       proposal.OpenQuestions,
		Risks:               proposal.Risks,
		Proposal:            &proposal,
		CreatedStoryIDs:     createdIDs,
		CreatedStories:      createdDetails,
	}
	outputSummary, _ := json.Marshal(runSummary)
	run.OutputSummary = outputSummary
	run.ApprovalState = "approved"
	if run.Status == "awaiting_approval" {
		now := time.Now()
		run.Status = "completed"
		run.CompletedAt = &now
		run.ExecutionStage = strPtr("approved")
	}
	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, err
	}

	epic.PlanningState = model.EpicPlanningStateStoriesCreated
	epic.LastPlanningRunID = &run.ID
	if err := s.epicRepo.Update(ctx, epic); err != nil {
		return nil, err
	}

	summary := fmt.Sprintf("Created %d stories from the approved plan.", len(created))
	_ = s.saveArtifact(ctx, run, "handoff_note", "markdown", summary, 999998)
	_ = s.runEngine.SignalApprove(ctx, derefString(run.WorkflowID), derefString(run.WorkflowRunID))
	s.publishRunEvent(run, actorID)

	return created, nil
}

// KickoffEpicExecution starts story-level execution runs for selected stories from an approved plan.
func (s *AgentService) KickoffEpicExecution(ctx context.Context, workspaceID, epicID, actorID string, req model.KickoffPlanningExecutionRequest) (*model.KickoffExecutionResult, error) {
	run, err := s.GetAgentRun(ctx, workspaceID, req.RunID)
	if err != nil {
		return nil, err
	}
	if run.TargetType != "epic" || run.TargetID != epicID {
		return nil, fmt.Errorf("run does not belong to this epic")
	}

	summary, err := decodePlanningRunSummary(run.OutputSummary)
	if err != nil {
		return nil, fmt.Errorf("run does not contain a valid planning summary")
	}
	if len(summary.CreatedStoryIDs) == 0 {
		return nil, fmt.Errorf("run has not created any stories yet")
	}

	storyIDs := req.StoryIDs
	if len(storyIDs) == 0 {
		storyIDs = append([]string(nil), summary.CreatedStoryIDs...)
	}
	selected := make(map[string]createdPlanningStory, len(summary.CreatedStories))
	for _, item := range summary.CreatedStories {
		selected[item.StoryID] = item
	}

	epicWithStats, err := s.epicRepo.GetByID(ctx, epicID)
	if err != nil {
		return nil, err
	}
	if epicWithStats == nil {
		return nil, fmt.Errorf("epic not found")
	}
	epic := &epicWithStats.Epic

	var specVersion *model.DocsVersion
	if epic.ApprovedSpecVersionID != nil && *epic.ApprovedSpecVersionID != "" {
		specVersion, _ = s.docsVersionRepo.GetByID(ctx, *epic.ApprovedSpecVersionID)
	}

	result := &model.KickoffExecutionResult{
		Started: make([]model.PlanningExecutionStart, 0, len(storyIDs)),
		Skipped: make([]model.PlanningExecutionSkip, 0),
	}

	for _, storyID := range storyIDs {
		story, err := s.storyRepo.GetRawByID(ctx, storyID)
		if err != nil || story == nil || story.EpicID == nil || *story.EpicID != epicID {
			result.Skipped = append(result.Skipped, model.PlanningExecutionSkip{StoryID: storyID, Reason: "story_not_found"})
			continue
		}
		if story.AssignedAgentID == nil || strings.TrimSpace(*story.AssignedAgentID) == "" {
			result.Skipped = append(result.Skipped, model.PlanningExecutionSkip{StoryID: storyID, Reason: "story_has_no_assigned_agent"})
			continue
		}

		agent, err := s.requireRunnableAgent(ctx, workspaceID, *story.AssignedAgentID, "story")
		if err != nil {
			result.Skipped = append(result.Skipped, model.PlanningExecutionSkip{StoryID: storyID, Reason: err.Error()})
			continue
		}
		profile := worker.GetRuntimeProfile(agent.CapabilityProfile)
		deliveryTarget, err := s.gitService.ResolveStoryDeliveryTargetForRun(ctx, workspaceID, storyID, profile)
		if err != nil {
			if errors.Is(err, ErrStoryDeliveryTargetRequired) {
				result.Skipped = append(result.Skipped, model.PlanningExecutionSkip{StoryID: storyID, Reason: "story_has_no_delivery_target"})
				continue
			}
			result.Skipped = append(result.Skipped, model.PlanningExecutionSkip{StoryID: storyID, Reason: err.Error()})
			continue
		}

		planned := selected[storyID]
		input, _ := json.Marshal(map[string]any{
			"story_id":           storyID,
			"additional_context": buildStoryExecutionBrief(epic, specVersion, planned),
		})

		run, err := s.createRun(ctx, createRunParams{
			workspaceID: workspaceID,
			agent:       agent,
			profile:     profile,
			targetType:  "story",
			targetID:    storyID,
			storyID:     &storyID,
			actorID:     actorID,
			input:       input,
			delivery:    deliveryTarget,
		})
		if err != nil {
			result.Skipped = append(result.Skipped, model.PlanningExecutionSkip{StoryID: storyID, Reason: err.Error()})
			continue
		}
		s.publishRunEvent(run, actorID)
		result.Started = append(result.Started, model.PlanningExecutionStart{
			StoryID:   storyID,
			RunID:     run.ID,
			StartedAt: run.CreatedAt,
		})
	}

	if len(result.Started) > 0 {
		epic.PlanningState = model.EpicPlanningStateExecutionStarted
	} else {
		epic.PlanningState = model.EpicPlanningStateReadyForExecution
	}
	if err := s.epicRepo.Update(ctx, epic); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *AgentService) startEpicPlanningRun(ctx context.Context, workspaceID string, epic *model.PMEpic, actorID, agentID string, input planningRunInput) (*model.AgentRun, error) {
	agent, err := s.requireRunnableAgent(ctx, workspaceID, agentID, "epic")
	if err != nil {
		return nil, err
	}
	profile := worker.GetRuntimeProfile(agent.CapabilityProfile)
	settings := s.resolvePlanningWorkspaceAISettings(ctx, workspaceID)
	if strings.TrimSpace(input.PlanningMethodology) != "" {
		input.PlanningMethodology = normalizePlanningMethodology(input.PlanningMethodology)
	} else {
		input.PlanningMethodology = settings.methodology
	}
	input.PlanningWebSearchEnabled = input.PlanningWebSearchEnabled || settings.webSearchEnabled
	if strings.TrimSpace(input.PlanningWebSearchProvider) != "" {
		input.PlanningWebSearchProvider = model.NormalizePlanningWebSearchProvider(input.PlanningWebSearchProvider)
	} else {
		input.PlanningWebSearchProvider = settings.webSearchProvider
	}

	payload, _ := json.Marshal(input)
	run, err := s.createRun(ctx, createRunParams{
		workspaceID: workspaceID,
		agent:       agent,
		profile:     profile,
		targetType:  "epic",
		targetID:    epic.ID,
		actorID:     actorID,
		input:       payload,
	})
	if err != nil {
		return nil, err
	}
	return run, nil
}

func (s *AgentService) resolvePlanningMethodology(ctx context.Context, workspaceID, requested string) string {
	settings := s.resolvePlanningWorkspaceAISettings(ctx, workspaceID)
	if strings.TrimSpace(requested) != "" {
		return normalizePlanningMethodology(requested)
	}
	return settings.methodology
}

type planningWorkspaceAISettings struct {
	methodology       string
	webSearchEnabled  bool
	webSearchProvider string
}

func (s *AgentService) resolvePlanningWorkspaceAISettings(ctx context.Context, workspaceID string) planningWorkspaceAISettings {
	resolved := planningWorkspaceAISettings{
		methodology:       model.PlanningMethodologyStructuredV1,
		webSearchProvider: model.PlanningWebSearchProviderBrave,
	}
	if s.settingsRepo == nil {
		return resolved
	}

	settings, err := s.settingsRepo.GetWorkspaceSettings(ctx, workspaceID)
	if err != nil || settings == nil {
		return resolved
	}

	resolved.methodology = normalizePlanningMethodology(settings.PlanningMethodology)
	resolved.webSearchEnabled = settings.PlanningWebSearchEnabled
	resolved.webSearchProvider = model.NormalizePlanningWebSearchProvider(settings.PlanningWebSearchProvider)
	return resolved
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

	doc, err := s.docsDocumentRepo.Create(ctx, &model.DocsDocument{
		WorkspaceID: workspaceID,
		SpaceID:     space.ID,
		Title:       strings.TrimSpace(epic.Name) + " Product Spec",
		DocType:     model.DocTypeProductSpec,
		Status:      model.DocStatusDraft,
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		OwnerID:     strPtr(actorID),
		TemplateKey: strPtr(model.DocTypeProductSpec),
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

func (s *AgentService) approveActiveEpicPlanningRun(ctx context.Context, workspaceID, epicID, actorID, stage string) error {
	activeRun, err := s.runRepo.FindActiveByTarget(ctx, workspaceID, "epic", epicID)
	if err != nil || activeRun == nil {
		return err
	}
	if activeRun.Status != "awaiting_approval" || activeRun.ApprovalState != "pending" {
		return nil
	}
	if parsePlanningRunStage(activeRun.Input) != stage {
		return nil
	}

	now := time.Now()
	activeRun.ApprovalState = "approved"
	activeRun.Status = "completed"
	activeRun.CompletedAt = &now
	activeRun.ExecutionStage = strPtr("approved")
	if err := s.runRepo.Update(ctx, activeRun); err != nil {
		return err
	}
	_ = s.runEngine.SignalApprove(ctx, derefString(activeRun.WorkflowID), derefString(activeRun.WorkflowRunID))
	s.publishRunEvent(activeRun, actorID)
	return nil
}

func loadPlanningStoriesByID(ctx context.Context, storyRepo interface {
	GetRawByID(ctx context.Context, id string) (*model.PMStory, error)
}, ids []string) ([]model.PMStory, error) {
	stories := make([]model.PMStory, 0, len(ids))
	for _, id := range ids {
		story, err := storyRepo.GetRawByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if story != nil {
			stories = append(stories, *story)
		}
	}
	return stories, nil
}

func (s *AgentService) loadCreatedStories(ctx context.Context, ids []string) ([]model.PMStory, error) {
	return loadPlanningStoriesByID(ctx, s.storyRepo, ids)
}

func validatePlanningStories(stories []model.ProposedStory) error {
	refToIdx := make(map[string]int, len(stories))
	for idx := range stories {
		stories[idx].Name = strings.TrimSpace(stories[idx].Name)
		if stories[idx].Name == "" {
			return fmt.Errorf("proposed story %d is missing a name", idx+1)
		}
		if strings.TrimSpace(stories[idx].Ref) == "" {
			stories[idx].Ref = fmt.Sprintf("story_%d", idx+1)
		}
		if prev, exists := refToIdx[stories[idx].Ref]; exists {
			return fmt.Errorf("story refs must be unique; stories %d and %d both use %q", prev+1, idx+1, stories[idx].Ref)
		}
		refToIdx[stories[idx].Ref] = idx
	}
	for idx, story := range stories {
		for _, depRef := range story.DependencyRefs {
			if _, ok := refToIdx[depRef]; !ok {
				return fmt.Errorf("story %d references unknown dependency ref %q", idx+1, depRef)
			}
			if depRef == story.Ref {
				return fmt.Errorf("story %d cannot depend on itself", idx+1)
			}
		}
	}

	visited := make(map[string]uint8, len(stories))
	var visit func(ref string) error
	visit = func(ref string) error {
		switch visited[ref] {
		case 1:
			return fmt.Errorf("circular dependency detected involving %q", ref)
		case 2:
			return nil
		}
		visited[ref] = 1
		story := stories[refToIdx[ref]]
		for _, depRef := range story.DependencyRefs {
			if err := visit(depRef); err != nil {
				return err
			}
		}
		visited[ref] = 2
		return nil
	}
	for _, story := range stories {
		if err := visit(story.Ref); err != nil {
			return err
		}
	}
	return nil
}

func renderPlannedStoryDescription(story model.ProposedStory) string {
	var sections []string
	if summary := strings.TrimSpace(story.Description); summary != "" {
		sections = append(sections, "## Summary\n"+summary)
	}
	if len(story.AcceptanceCriteria) > 0 {
		var lines []string
		for _, item := range story.AcceptanceCriteria {
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
	if len(story.DependencyRefs) > 0 {
		lines := make([]string, 0, len(story.DependencyRefs))
		for _, dep := range story.DependencyRefs {
			lines = append(lines, "- Depends on `"+dep+"`")
		}
		sections = append(sections, "## Dependencies\n"+strings.Join(lines, "\n"))
	}
	if len(story.SourceRefs) > 0 {
		lines := make([]string, 0, len(story.SourceRefs))
		for _, ref := range story.SourceRefs {
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
	return strings.Join(sections, "\n\n")
}

func buildStoryExecutionBrief(epic *model.PMEpic, specVersion *model.DocsVersion, story createdPlanningStory) string {
	var sections []string
	sections = append(sections, "This story was created from an approved epic planning run. Use the context below while implementing it.")
	if epic != nil {
		sections = append(sections, fmt.Sprintf("Epic: %s", epic.Name))
	}
	if strings.TrimSpace(story.Name) != "" {
		sections = append(sections, fmt.Sprintf("Planned story: %s", story.Name))
	}
	if len(story.AcceptanceCriteria) > 0 {
		sections = append(sections, "Acceptance criteria:\n- "+strings.Join(story.AcceptanceCriteria, "\n- "))
	}
	if len(story.DependencyRefs) > 0 {
		sections = append(sections, "Dependencies:\n- "+strings.Join(story.DependencyRefs, "\n- "))
	}
	if len(story.SourceRefs) > 0 {
		lines := make([]string, 0, len(story.SourceRefs))
		for _, ref := range story.SourceRefs {
			label := strings.TrimSpace(ref.Title)
			if label == "" {
				label = ref.Type
			}
			lines = append(lines, "- "+label)
		}
		sections = append(sections, "Traceability:\n"+strings.Join(lines, "\n"))
	}
	if specVersion != nil && strings.TrimSpace(specVersion.ContentText) != "" {
		sections = append(sections, "Approved spec snapshot:\n"+truncateString(specVersion.ContentText, 12000))
	}
	return strings.Join(sections, "\n\n")
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
