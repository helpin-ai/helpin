package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

type InternalCommandDefinition struct {
	Name                 string
	Module               string
	Mutating             bool
	SupportedTargetTypes []string
	ExposeAsTool         bool
	Execute              func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error)
}

type InternalCommandService struct {
	agentService        *AgentService
	storyService        *PMStoryService
	crmDealService      *CRMDealService
	crmActivityService  *CRMActivityService
	docsContentService  *DocsContentService
	docsLinkService     *DocsLinkService
	pmAutomationService *PMAutomationService
	gitService          *GitService
	storyRepo           *repository.PMStoryRepository
	storyLinkRepo       *repository.PMStoryLinkRepository
	definitions         map[string]InternalCommandDefinition
}

// SetPMAutomationService sets the PM automation service (breaks circular dependency).
func (s *InternalCommandService) SetPMAutomationService(svc *PMAutomationService) {
	s.pmAutomationService = svc
}

// SetGitService sets the git service for delivery commands.
func (s *InternalCommandService) SetGitService(svc *GitService) {
	s.gitService = svc
}

func NewInternalCommandService(
	agentService *AgentService,
	storyService *PMStoryService,
	crmDealService *CRMDealService,
	crmActivityService *CRMActivityService,
	docsContentService *DocsContentService,
	docsLinkService *DocsLinkService,
	storyRepo *repository.PMStoryRepository,
	storyLinkRepo *repository.PMStoryLinkRepository,
) *InternalCommandService {
	svc := &InternalCommandService{
		agentService:       agentService,
		storyService:       storyService,
		crmDealService:     crmDealService,
		crmActivityService: crmActivityService,
		docsContentService: docsContentService,
		docsLinkService:    docsLinkService,
		storyRepo:          storyRepo,
		storyLinkRepo:      storyLinkRepo,
		definitions:        make(map[string]InternalCommandDefinition),
	}
	svc.registerDefaults()
	return svc
}

func (s *InternalCommandService) Definition(name string) (InternalCommandDefinition, bool) {
	if s == nil {
		return InternalCommandDefinition{}, false
	}
	def, ok := s.definitions[strings.TrimSpace(name)]
	return def, ok
}

func (s *InternalCommandService) Execute(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
	if s == nil {
		return nil, fmt.Errorf("internal command service is not configured")
	}
	def, ok := s.Definition(name)
	if !ok {
		return nil, fmt.Errorf("unknown command %q", name)
	}
	if len(def.SupportedTargetTypes) > 0 && meta.TargetType != "" {
		supported := false
		for _, targetType := range def.SupportedTargetTypes {
			if targetType == meta.TargetType {
				supported = true
				break
			}
		}
		if !supported {
			return nil, fmt.Errorf("command %q does not support target type %q", name, meta.TargetType)
		}
	}
	output, err := def.Execute(ctx, meta, input)
	if err != nil {
		return nil, err
	}
	if len(output) == 0 {
		return json.RawMessage("{}"), nil
	}
	return output, nil
}

func (s *InternalCommandService) register(def InternalCommandDefinition) {
	s.definitions[def.Name] = def
}

func (s *InternalCommandService) registerDefaults() {
	s.register(InternalCommandDefinition{
		Name:                 "docs.ensure_spec_doc",
		Module:               "docs",
		Mutating:             true,
		SupportedTargetTypes: []string{"epic"},
		ExposeAsTool:         true,
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			doc, err := s.agentService.EnsureEpicSpecDocument(ctx, meta.WorkspaceID, meta.TargetID, fallbackActor(meta))
			if err != nil {
				return nil, err
			}
			return mustJSON(map[string]any{"document_id": doc.ID, "title": doc.Title}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "docs.ensure_story_plan_doc",
		Module:               "docs",
		Mutating:             true,
		SupportedTargetTypes: []string{"story"},
		ExposeAsTool:         true,
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			doc, err := s.agentService.EnsureStoryPlanDocument(ctx, meta.WorkspaceID, meta.TargetID, fallbackActor(meta))
			if err != nil {
				return nil, err
			}
			return mustJSON(map[string]any{"document_id": doc.ID, "title": doc.Title}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.approve_epic_spec",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"epic"},
		ExposeAsTool:         false,
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req model.ApproveEpicSpecRequest
			if len(input) > 0 {
				if err := json.Unmarshal(input, &req); err != nil {
					return nil, fmt.Errorf("parse approve spec input: %w", err)
				}
			}
			summary, err := s.agentService.ApproveEpicSpec(ctx, meta.WorkspaceID, meta.TargetID, fallbackActor(meta), req)
			if err != nil {
				return nil, err
			}
			return mustJSON(summary), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.create_story_batch",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"epic"},
		ExposeAsTool:         true,
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				Stories []model.ProposedStory `json:"stories"`
				RunID   string                `json:"run_id,omitempty"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse story batch input: %w", err)
			}
			if len(req.Stories) == 0 {
				var legacy model.ConfirmPlanningRequest
				if err := json.Unmarshal(input, &legacy); err != nil {
					return nil, fmt.Errorf("stories is required")
				}
				req.Stories = legacy.ProposedStories
				req.RunID = legacy.RunID
			}
			if len(req.Stories) == 0 {
				return nil, fmt.Errorf("stories is required")
			}

			var stories []model.PMStory
			var err error
			if strings.TrimSpace(req.RunID) != "" {
				legacy := model.ConfirmPlanningRequest{
					RunID:           strings.TrimSpace(req.RunID),
					ProposedStories: req.Stories,
				}
				stories, err = s.agentService.ConfirmEpicRun(ctx, meta.WorkspaceID, meta.TargetID, legacy.RunID, fallbackActor(meta), legacy)
			} else {
				stories, err = s.agentService.CreateEpicStoryBatch(ctx, meta.WorkspaceID, meta.TargetID, fallbackActor(meta), req.Stories)
			}
			if err != nil {
				return nil, err
			}
			results := make([]map[string]any, 0, len(stories))
			for idx, story := range stories {
				ref := strings.TrimSpace(req.Stories[idx].Ref)
				results = append(results, map[string]any{
					"ref":      ref,
					"story_id": story.ID,
					"name":     story.Name,
				})
			}
			return mustJSON(map[string]any{"stories": results}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.set_story_dependencies",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"epic", "story"},
		ExposeAsTool:         true,
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				Dependencies []struct {
					SourceStoryID string `json:"source_story_id"`
					TargetStoryID string `json:"target_story_id"`
				} `json:"dependencies"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse dependency input: %w", err)
			}
			for _, dep := range req.Dependencies {
				if strings.TrimSpace(dep.SourceStoryID) == "" || strings.TrimSpace(dep.TargetStoryID) == "" {
					return nil, fmt.Errorf("source_story_id and target_story_id are required")
				}
				source, err := s.storyRepo.GetRawByID(ctx, dep.SourceStoryID)
				if err != nil {
					return nil, err
				}
				target, err := s.storyRepo.GetRawByID(ctx, dep.TargetStoryID)
				if err != nil {
					return nil, err
				}
				if source == nil || target == nil || source.WorkspaceID != meta.WorkspaceID || target.WorkspaceID != meta.WorkspaceID {
					return nil, fmt.Errorf("stories must belong to the current workspace")
				}
				if err := s.storyLinkRepo.Create(ctx, &model.PMStoryLink{
					WorkspaceID:   meta.WorkspaceID,
					SourceStoryID: dep.SourceStoryID,
					TargetStoryID: dep.TargetStoryID,
					LinkType:      model.PMStoryLinkTypeBlocks,
					CreatedBy:     fallbackActor(meta),
				}); err != nil {
					return nil, err
				}
			}
			return mustJSON(map[string]any{"dependency_count": len(req.Dependencies)}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.assign_story_agent",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"epic", "story"},
		ExposeAsTool:         true,
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				StoryID string `json:"story_id"`
				AgentID string `json:"agent_id"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse assign input: %w", err)
			}
			if err := s.agentService.AssignAgentToStory(ctx, meta.WorkspaceID, req.StoryID, req.AgentID, fallbackActor(meta)); err != nil {
				return nil, err
			}
			return mustJSON(map[string]any{"story_id": req.StoryID, "agent_id": req.AgentID}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.create_followup_stories",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"story"},
		ExposeAsTool:         true,
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				Followups []model.StoryCompletionFollowupProposal `json:"followups"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse followup input: %w", err)
			}
			story, err := s.storyRepo.GetRawByID(ctx, meta.TargetID)
			if err != nil {
				return nil, err
			}
			if story == nil {
				return nil, fmt.Errorf("story not found")
			}
			createdIDs := make([]string, 0, len(req.Followups))
			for idx, followup := range req.Followups {
				title := strings.TrimSpace(followup.Title)
				if title == "" {
					return nil, fmt.Errorf("followup %d is missing a title", idx+1)
				}
				storyType := strings.TrimSpace(followup.StoryType)
				if storyType == "" {
					storyType = model.PMStoryTypeChore
				}
				description := strings.TrimSpace(followup.Description)
				createReq := model.CreateStoryRequest{
					WorkspaceID: meta.WorkspaceID,
					Name:        title,
					Description: stringPtrOrNil(description),
					StoryType:   storyType,
					EpicID:      story.EpicID,
					TeamID:      story.TeamID,
					Priority:    followup.Priority,
				}
				detail, err := s.storyService.Create(ctx, createReq, fallbackActor(meta))
				if err != nil {
					return nil, err
				}
				createdIDs = append(createdIDs, detail.Story.ID)
			}
			return mustJSON(map[string]any{"created_story_ids": createdIDs}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.update_story_state",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"story"},
		ExposeAsTool:         true,
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				StoryID  string `json:"story_id"`
				StateID  string `json:"state_id"`
				Position *int   `json:"position,omitempty"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse move story input: %w", err)
			}
			storyID := strings.TrimSpace(firstNonEmptyCommand(req.StoryID, meta.TargetID))
			if storyID == "" || strings.TrimSpace(req.StateID) == "" {
				return nil, fmt.Errorf("story_id and state_id are required")
			}
			_, err := s.storyService.MoveToState(ctx, storyID, model.MoveStoryRequest{
				StateID:  req.StateID,
				Position: req.Position,
			}, fallbackActor(meta))
			if err != nil {
				return nil, err
			}
			return mustJSON(map[string]any{"story_id": storyID, "state_id": req.StateID}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "docs.write_document_content",
		Module:               "docs",
		Mutating:             true,
		SupportedTargetTypes: []string{"document", "epic", "story", "crm_deal"},
		ExposeAsTool:         true,
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				DocumentID string          `json:"document_id"`
				Content    json.RawMessage `json:"content"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse document content input: %w", err)
			}
			if strings.TrimSpace(req.DocumentID) == "" {
				return nil, fmt.Errorf("document_id is required")
			}
			if len(req.Content) == 0 || strings.TrimSpace(string(req.Content)) == "" || strings.TrimSpace(string(req.Content)) == "null" {
				return nil, fmt.Errorf("content is required")
			}
			// Auto-convert markdown to TipTap JSON when the agent sends a
			// plain string instead of a structured document object.
			docContent := req.Content
			if len(docContent) > 0 && docContent[0] == '"' {
				var markdown string
				if err := json.Unmarshal(docContent, &markdown); err == nil {
					if strings.TrimSpace(markdown) == "" {
						return nil, fmt.Errorf("content must not be empty")
					}
					docContent = tiptap.MarkdownToJSON(markdown)
				}
			}
			if documentContentIsEffectivelyEmpty(docContent) {
				return nil, fmt.Errorf("content must not be empty")
			}
			content, err := s.docsContentService.Save(ctx, req.DocumentID, docContent, meta.ActorID)
			if err != nil {
				return nil, err
			}
			return mustJSON(map[string]any{"document_id": req.DocumentID, "content_id": content.ID}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "docs.link_document_to_object",
		Module:               "docs",
		Mutating:             true,
		SupportedTargetTypes: []string{"epic", "story", "crm_deal"},
		ExposeAsTool:         true,
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				DocumentID       string  `json:"document_id"`
				LinkedObjectType string  `json:"linked_object_type"`
				LinkedObjectID   string  `json:"linked_object_id"`
				LinkContext      *string `json:"link_context,omitempty"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse document link input: %w", err)
			}
			linkContext := model.LinkContextAttached
			if req.LinkContext != nil && strings.TrimSpace(*req.LinkContext) != "" {
				linkContext = strings.TrimSpace(*req.LinkContext)
			}
			link, err := s.docsLinkService.Create(ctx, meta.WorkspaceID, req.DocumentID, model.CreateDocsLinkRequest{
				LinkedObjectType: req.LinkedObjectType,
				LinkedObjectID:   req.LinkedObjectID,
				LinkContext:      linkContext,
			}, fallbackActor(meta))
			if err != nil {
				return nil, err
			}
			return mustJSON(map[string]any{"link_id": link.ID, "document_id": link.DocumentID}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "crm.update_deal_stage",
		Module:               "crm",
		Mutating:             true,
		SupportedTargetTypes: []string{"crm_deal"},
		ExposeAsTool:         true,
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				DealID  string `json:"deal_id"`
				StageID string `json:"stage_id"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse deal stage input: %w", err)
			}
			dealID := strings.TrimSpace(firstNonEmptyCommand(req.DealID, meta.TargetID))
			if dealID == "" || strings.TrimSpace(req.StageID) == "" {
				return nil, fmt.Errorf("deal_id and stage_id are required")
			}
			deal, err := s.crmDealService.Update(ctx, dealID, model.UpdateCRMDealRequest{StageID: &req.StageID})
			if err != nil {
				return nil, err
			}
			return mustJSON(map[string]any{"deal_id": deal.ID, "stage_id": deal.StageID}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "crm.add_deal_note",
		Module:               "crm",
		Mutating:             true,
		SupportedTargetTypes: []string{"crm_deal"},
		ExposeAsTool:         true,
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				DealID  string `json:"deal_id"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse deal note input: %w", err)
			}
			dealID := strings.TrimSpace(firstNonEmptyCommand(req.DealID, meta.TargetID))
			if dealID == "" || strings.TrimSpace(req.Content) == "" {
				return nil, fmt.Errorf("deal_id and content are required")
			}
			activity, err := s.crmActivityService.Create(ctx, model.CreateCRMActivityRequest{
				WorkspaceID:  meta.WorkspaceID,
				ActivityType: model.CRMActivityNote,
				DealID:       &dealID,
				Body:         stringPtrOrNil(req.Content),
				OccurredAt:   commandTimePtr(time.Now()),
			})
			if err != nil {
				return nil, err
			}
			return mustJSON(map[string]any{"activity_id": activity.ID, "deal_id": dealID}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.auto_start_epic",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"epic", "story"},
		ExposeAsTool:         false,
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			if s.pmAutomationService == nil {
				return nil, fmt.Errorf("pm automation service not configured")
			}
			var req struct {
				EpicID        string `json:"epic_id"`
				TargetStateID string `json:"target_state_id"`
			}
			if len(input) > 0 {
				if err := json.Unmarshal(input, &req); err != nil {
					return nil, fmt.Errorf("parse auto_start_epic input: %w", err)
				}
			}
			epicID := firstNonEmptyCommand(req.EpicID, meta.TargetID)
			if epicID == "" {
				return nil, fmt.Errorf("epic_id is required")
			}
			if req.TargetStateID == "" {
				return nil, fmt.Errorf("target_state_id is required")
			}
			mutated, err := s.pmAutomationService.HandleEpicAutoStart(ctx, meta.WorkspaceID, epicID, req.TargetStateID)
			if err != nil {
				return nil, err
			}
			return mustJSON(map[string]any{"epic_id": epicID, "mutated": mutated}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.auto_complete_epic",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"epic", "story"},
		ExposeAsTool:         false,
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			if s.pmAutomationService == nil {
				return nil, fmt.Errorf("pm automation service not configured")
			}
			var req struct {
				EpicID        string `json:"epic_id"`
				TargetStateID string `json:"target_state_id"`
			}
			if len(input) > 0 {
				if err := json.Unmarshal(input, &req); err != nil {
					return nil, fmt.Errorf("parse auto_complete_epic input: %w", err)
				}
			}
			epicID := firstNonEmptyCommand(req.EpicID, meta.TargetID)
			if epicID == "" {
				return nil, fmt.Errorf("epic_id is required")
			}
			if req.TargetStateID == "" {
				return nil, fmt.Errorf("target_state_id is required")
			}
			mutated, err := s.pmAutomationService.HandleEpicAutoComplete(ctx, meta.WorkspaceID, epicID, req.TargetStateID)
			if err != nil {
				return nil, err
			}
			return mustJSON(map[string]any{"epic_id": epicID, "mutated": mutated}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.sprint_auto_create",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"sprint"},
		ExposeAsTool:         false,
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			if s.pmAutomationService == nil {
				return nil, fmt.Errorf("pm automation service not configured")
			}
			s.pmAutomationService.RunSprintAutoCreate(ctx)
			return mustJSON(map[string]any{"status": "completed"}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.sprint_move_unfinished",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"sprint"},
		ExposeAsTool:         false,
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			if s.pmAutomationService == nil {
				return nil, fmt.Errorf("pm automation service not configured")
			}
			s.pmAutomationService.RunSprintMoveUnfinished(ctx)
			return mustJSON(map[string]any{"status": "completed"}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "delivery.merge_branch",
		Module:               "delivery",
		Mutating:             true,
		SupportedTargetTypes: []string{"story"},
		ExposeAsTool:         false,
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			if s.gitService == nil {
				return nil, fmt.Errorf("git service not configured")
			}
			var req struct {
				StoryID      string `json:"story_id"`
				TargetBranch string `json:"target_branch"`
			}
			if len(input) > 0 {
				if err := json.Unmarshal(input, &req); err != nil {
					return nil, fmt.Errorf("parse merge_branch input: %w", err)
				}
			}
			storyID := firstNonEmptyCommand(req.StoryID, meta.TargetID)
			if storyID == "" {
				return nil, fmt.Errorf("story_id is required")
			}
			if strings.TrimSpace(req.TargetBranch) == "" {
				return nil, fmt.Errorf("target_branch is required")
			}
			if err := s.gitService.MergeBranch(ctx, meta.WorkspaceID, storyID, req.TargetBranch); err != nil {
				return nil, err
			}
			return mustJSON(map[string]any{"story_id": storyID, "target_branch": req.TargetBranch}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "crm.apply_deal_actions",
		Module:               "crm",
		Mutating:             true,
		SupportedTargetTypes: []string{"crm_deal"},
		ExposeAsTool:         false,
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				RecommendedStageID *string `json:"recommended_stage_id,omitempty"`
				Note               *string `json:"note,omitempty"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse apply deal actions input: %w", err)
			}
			result := map[string]any{
				"deal_id": meta.TargetID,
			}
			if req.RecommendedStageID != nil && strings.TrimSpace(*req.RecommendedStageID) != "" {
				output, err := s.Execute(ctx, meta, "crm.update_deal_stage", mustJSON(map[string]any{
					"deal_id":  meta.TargetID,
					"stage_id": *req.RecommendedStageID,
				}))
				if err != nil {
					return nil, err
				}
				var decoded map[string]any
				_ = json.Unmarshal(output, &decoded)
				result["stage_update"] = decoded
			}
			if req.Note != nil && strings.TrimSpace(*req.Note) != "" {
				output, err := s.Execute(ctx, meta, "crm.add_deal_note", mustJSON(map[string]any{
					"deal_id": meta.TargetID,
					"content": *req.Note,
				}))
				if err != nil {
					return nil, err
				}
				var decoded map[string]any
				_ = json.Unmarshal(output, &decoded)
				result["note"] = decoded
			}
			return mustJSON(result), nil
		},
	})
}

func fallbackActor(meta model.InternalCommandContext) string {
	if strings.TrimSpace(meta.ActorID) != "" {
		return strings.TrimSpace(meta.ActorID)
	}
	if strings.TrimSpace(meta.AgentID) != "" {
		return strings.TrimSpace(meta.AgentID)
	}
	return ""
}

func commandTimePtr(value time.Time) *time.Time {
	return &value
}

func stringPtrOrNil(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func documentContentIsEffectivelyEmpty(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return true
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return true
	}

	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) == ""
	case map[string]any:
		return !documentNodeHasText(typed)
	default:
		return false
	}
}

func documentNodeHasText(node map[string]any) bool {
	if text, ok := node["text"].(string); ok && strings.TrimSpace(text) != "" {
		return true
	}

	content, ok := node["content"].([]any)
	if !ok {
		return false
	}
	for _, child := range content {
		childNode, ok := child.(map[string]any)
		if !ok {
			continue
		}
		if documentNodeHasText(childNode) {
			return true
		}
	}
	return false
}

func mustJSON(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("{}")
	}
	return payload
}

func firstNonEmptyCommand(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
