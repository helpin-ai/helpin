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
	CreatedStoryIDs     []string                      `json:"created_story_ids,omitempty"`
	CreatedStories      []createdPlanningStory        `json:"created_stories,omitempty"`
}

type createdPlanningStory struct {
	StoryID             string                    `json:"story_id"`
	Ref                 string                    `json:"ref,omitempty"`
	Name                string                    `json:"name"`
	StoryType           string                    `json:"story_type"`
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
		Stage:               model.PlanningStageDraftSpec,
		SpecDocumentID:      *epic.SpecDocumentID,
		SpecVersionID:       version.ID,
		Clarifications:      clarifications,
		PendingClarifyCount: 0,
	}, nil
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

	enablerCount := 0
	for _, s := range proposedStories {
		if s.SliceType == "enabler" {
			enablerCount++
		}
	}
	if enablerCount > 2 {
		slog.Warn("high enabler count in story plan",
			"enabler_count", enablerCount,
			"total_count", len(proposedStories),
			"epic_id", epicID)
	}

	// Warn on overlapping file modifications across non-dependent stories.
	fileOwners := map[string]string{} // path -> story ref
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
					"file", f.Path, "story_a", owner, "story_b", ps.Ref, "epic_id", epicID)
			}
			if _, exists := fileOwners[f.Path]; !exists {
				fileOwners[f.Path] = ps.Ref
			}
		}
	}

	externalIDs := make([]string, 0, len(proposedStories))
	for idx, ps := range proposedStories {
		externalIDs = append(externalIDs, planningStoryExternalID(runID, idx, ps.Ref))
	}
	existingStories, err := s.storyRepo.ListByEpicAndExternalIDs(ctx, workspaceID, epicID, externalIDs)
	if err != nil {
		return nil, fmt.Errorf("lookup existing planned stories: %w", err)
	}
	existingByExternalID := make(map[string]model.PMStory, len(existingStories))
	for _, story := range existingStories {
		if story.ExternalID == nil || strings.TrimSpace(*story.ExternalID) == "" {
			continue
		}
		existingByExternalID[*story.ExternalID] = story
	}

	created := make([]model.PMStory, 0, len(proposedStories))
	createdIDs := make([]string, 0, len(proposedStories))
	createdDetails := make([]createdPlanningStory, 0, len(proposedStories))
	refToStory := make(map[string]model.PMStory, len(proposedStories))

	for idx, ps := range proposedStories {
		storyType := strings.ToLower(strings.TrimSpace(ps.StoryType))
		if !isValidStoryType(storyType) {
			storyType = model.PMStoryTypeFeature
		}

		externalID := planningStoryExternalID(runID, idx, ps.Ref)
		var detail *model.StoryDetail
		if existing, ok := existingByExternalID[externalID]; ok {
			detail, err = s.storyService.GetByID(ctx, existing.ID)
			if err != nil {
				return nil, fmt.Errorf("reload existing story %d: %w", idx+1, err)
			}
		} else {
			desc := renderPlannedStoryDescription(ps)
			detail, err = s.storyService.Create(ctx, model.CreateStoryRequest{
				WorkspaceID: workspaceID,
				Name:        strings.TrimSpace(ps.Name),
				Description: strPtr(desc),
				StoryType:   storyType,
				EpicID:      &epicID,
				Estimate:    ps.Estimate,
				Priority:    ps.Priority,
				ExternalID:  strPtr(externalID),
			}, actorID)
			if err != nil {
				return nil, fmt.Errorf("create story %d: %w", idx+1, err)
			}
		}

		var briefJSON json.RawMessage
		briefFields := map[string]interface{}{}
		if ps.SliceType != "" {
			briefFields["slice_type"] = ps.SliceType
		}
		if ps.ImplementationBrief != nil {
			if b, marshalErr := json.Marshal(ps.ImplementationBrief); marshalErr != nil {
				slog.WarnContext(ctx, "marshal implementation brief", "error", marshalErr, "story_ref", ps.Ref)
			} else {
				briefFields["implementation_brief"] = b
				briefJSON = b
			}
		}
		if len(briefFields) > 0 {
			if err := s.storyRepo.UpdateFields(ctx, detail.Story.ID, briefFields); err != nil {
				slog.WarnContext(ctx, "failed to persist story brief fields",
					"story_id", detail.Story.ID, "error", err)
			}
		}

		if ps.AssignAgentID != nil && strings.TrimSpace(*ps.AssignAgentID) != "" && isValidUUID(*ps.AssignAgentID) {
			if err := s.AssignAgentToStory(ctx, workspaceID, detail.Story.ID, *ps.AssignAgentID, actorID); err != nil {
				slog.WarnContext(ctx, "skipping agent assignment for planned story",
					"story", detail.Story.Name, "agent_id", *ps.AssignAgentID, "error", err)
			} else {
				detail, err = s.storyService.GetByID(ctx, detail.Story.ID)
				if err != nil {
					return nil, fmt.Errorf("reload story %q: %w", ps.Name, err)
				}
			}
		}

		created = append(created, detail.Story)
		createdIDs = append(createdIDs, detail.Story.ID)
		createdDetails = append(createdDetails, createdPlanningStory{
			StoryID:             detail.Story.ID,
			Ref:                 ps.Ref,
			Name:                detail.Story.Name,
			StoryType:           detail.Story.StoryType,
			Estimate:            detail.Story.Estimate,
			Priority:            ps.Priority,
			AcceptanceCriteria:  slices.Clone(ps.AcceptanceCriteria),
			DependencyRefs:      slices.Clone(ps.DependencyRefs),
			SourceRefs:          append([]model.PlanningSourceRef(nil), ps.SourceRefs...),
			ImplementationBrief: briefJSON,
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

func planningStoryExternalID(runID string, index int, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref != "" {
		return fmt.Sprintf("planning:%s:%s", runID, ref)
	}
	return fmt.Sprintf("planning:%s:%03d", runID, index+1)
}

func (s *AgentService) StartEpicStoryPlanningForFlow(ctx context.Context, workspaceID, epicID, actorID, agentID, additionalContext, flowRunID, flowNodeRunID string) (*model.AgentRun, error) {
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
	if epic.ApprovedSpecVersionID == nil || strings.TrimSpace(*epic.ApprovedSpecVersionID) == "" {
		return nil, fmt.Errorf("planning requires an approved spec version")
	}
	if epic.PlanningRepositoryID == nil || strings.TrimSpace(*epic.PlanningRepositoryID) == "" {
		return nil, fmt.Errorf("planning requires an epic planning repository")
	}
	return s.startEpicPlanningRun(ctx, workspaceID, epic, actorID, agentID, planningRunInput{
		Stage:             model.PlanningStagePlanStories,
		AdditionalContext: strings.TrimSpace(additionalContext),
		SpecDocumentID:    *epic.SpecDocumentID,
		SpecVersionID:     *epic.ApprovedSpecVersionID,
	}, &flowRunID, &flowNodeRunID)
}

func (s *AgentService) startEpicPlanningRun(ctx context.Context, workspaceID string, epic *model.PMEpic, actorID, agentID string, input planningRunInput, flowRunID, flowNodeRunID *string) (*model.AgentRun, error) {
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
		workspaceID:   workspaceID,
		agent:         agent,
		profile:       profile,
		targetType:    "epic",
		targetID:      epic.ID,
		flowRunID:     flowRunID,
		flowNodeRunID: flowNodeRunID,
		actorID:       &actorID,
		input:         payload,
	})
	if err != nil {
		return nil, err
	}
	return run, nil
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
		filteredCriteria := make([]string, 0, len(story.AcceptanceCriteria))
		for _, item := range story.AcceptanceCriteria {
			item = strings.TrimSpace(item)
			if item != "" {
				filteredCriteria = append(filteredCriteria, item)
			}
		}
		stories[idx].AcceptanceCriteria = filteredCriteria
		if len(filteredCriteria) == 0 {
			return fmt.Errorf("story %d must include at least one acceptance criterion", idx+1)
		}

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
	savedContent, err := s.docsContentRepo.Upsert(ctx, documentID, serviceMarkdownToDocsJSON(updatedMarkdown))
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

func serviceMarkdownToDocsJSON(markdown string) json.RawMessage {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	nodes := make([]map[string]interface{}, 0, len(lines))
	paragraphLines := make([]string, 0)
	bulletLines := make([]string, 0)

	flushParagraph := func() {
		if len(paragraphLines) == 0 {
			return
		}
		text := strings.TrimSpace(strings.Join(paragraphLines, " "))
		paragraphLines = paragraphLines[:0]
		if text == "" {
			return
		}
		nodes = append(nodes, map[string]interface{}{
			"type": "paragraph",
			"content": []map[string]interface{}{
				{"type": "text", "text": text},
			},
		})
	}

	flushBullets := func() {
		if len(bulletLines) == 0 {
			return
		}
		items := make([]map[string]interface{}, 0, len(bulletLines))
		for _, item := range bulletLines {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			items = append(items, map[string]interface{}{
				"type": "listItem",
				"content": []map[string]interface{}{
					{
						"type": "paragraph",
						"content": []map[string]interface{}{
							{"type": "text", "text": item},
						},
					},
				},
			})
		}
		bulletLines = bulletLines[:0]
		if len(items) == 0 {
			return
		}
		nodes = append(nodes, map[string]interface{}{
			"type":    "bulletList",
			"content": items,
		})
	}

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			flushParagraph()
			flushBullets()
			continue
		}
		if level, headingText, ok := parseServiceMarkdownHeading(line); ok {
			flushParagraph()
			flushBullets()
			nodes = append(nodes, map[string]interface{}{
				"type": "heading",
				"attrs": map[string]interface{}{
					"level": level,
				},
				"content": []map[string]interface{}{
					{"type": "text", "text": headingText},
				},
			})
			continue
		}
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			flushParagraph()
			bulletLines = append(bulletLines, strings.TrimSpace(line[2:]))
			continue
		}
		flushBullets()
		paragraphLines = append(paragraphLines, line)
	}

	flushParagraph()
	flushBullets()

	if len(nodes) == 0 {
		nodes = append(nodes, map[string]interface{}{"type": "paragraph"})
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"type":    "doc",
		"content": nodes,
	})
	return payload
}

func parseServiceMarkdownHeading(line string) (int, string, bool) {
	if !strings.HasPrefix(line, "#") {
		return 0, "", false
	}
	level := 0
	for level < len(line) && line[level] == '#' && level < 6 {
		level++
	}
	if level == 0 || level >= len(line) || line[level] != ' ' {
		return 0, "", false
	}
	text := strings.TrimSpace(line[level+1:])
	if text == "" {
		return 0, "", false
	}
	return level, text, true
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
