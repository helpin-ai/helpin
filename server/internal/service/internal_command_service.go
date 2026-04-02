package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

type InternalCommandDefinition struct {
	Name                 string
	Module               string
	Mutating             bool
	SupportedTargetTypes []string
	Tool                 *commandtools.RuntimeToolMetadata
	Execute              func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error)
}

type InternalCommandService struct {
	agentService        *AgentService
	taskService        *PMTaskService
	crmDealService      *CRMDealService
	crmActivityService  *CRMActivityService
	docsContentService  *DocsContentService
	docsLinkService     *DocsLinkService
	pmAutomationService *PMAutomationService
	gitService          *GitService
	taskRepo           *repository.PMTaskRepository
	storyLinkRepo       *repository.PMTaskLinkRepository
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
	taskService *PMTaskService,
	crmDealService *CRMDealService,
	crmActivityService *CRMActivityService,
	docsContentService *DocsContentService,
	docsLinkService *DocsLinkService,
	taskRepo *repository.PMTaskRepository,
	storyLinkRepo *repository.PMTaskLinkRepository,
) *InternalCommandService {
	svc := &InternalCommandService{
		agentService:       agentService,
		taskService:       taskService,
		crmDealService:     crmDealService,
		crmActivityService: crmActivityService,
		docsContentService: docsContentService,
		docsLinkService:    docsLinkService,
		taskRepo:          taskRepo,
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

func (d InternalCommandDefinition) ExposesTool() bool {
	return d.Tool != nil && strings.TrimSpace(d.Tool.Alias) != ""
}

func (s *InternalCommandService) ToolDefinitions() []InternalCommandDefinition {
	if s == nil {
		return nil
	}
	defs := make([]InternalCommandDefinition, 0, len(s.definitions))
	for _, def := range s.definitions {
		if def.ExposesTool() {
			defs = append(defs, def)
		}
	}
	return defs
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
		Tool:                 mustCommandToolMetadata("docs.ensure_spec_doc"),
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			doc, err := s.agentService.EnsureEpicSpecDocument(ctx, meta.WorkspaceID, meta.TargetID, fallbackActor(meta))
			if err != nil {
				return nil, err
			}
			return mustJSON(map[string]any{"document_id": doc.ID, "title": doc.Title}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "docs.ensure_task_plan_doc",
		Module:               "docs",
		Mutating:             true,
		SupportedTargetTypes: []string{"task", "story"},
		Tool:                 mustCommandToolMetadata("docs.ensure_task_plan_doc"),
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			doc, err := s.agentService.EnsureTaskPlanDocument(ctx, meta.WorkspaceID, meta.TargetID, fallbackActor(meta))
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
		SupportedTargetTypes: []string{"task", "story"},
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			return s.Execute(ctx, meta, "docs.ensure_task_plan_doc", input)
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.approve_epic_spec",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"epic"},
		Tool:                 mustCommandToolMetadata("pm.approve_epic_spec"),
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
		Name:                 "pm.create_task_batch",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"epic"},
		Tool:                 mustCommandToolMetadata("pm.create_task_batch"),
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				Stories       []model.ProposedTask `json:"stories"`
				Tasks         []model.ProposedTask `json:"tasks"`
				ProposedTasks []model.ProposedTask `json:"proposed_tasks"`
				RunID         string                `json:"run_id,omitempty"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse task batch input: %w", err)
			}
			if len(req.Stories) == 0 {
				req.Stories = req.Tasks
			}
			if len(req.Stories) == 0 {
				req.Stories = req.ProposedTasks
			}
			if len(req.Stories) == 0 {
				var legacy model.ConfirmPlanningRequest
				if err := json.Unmarshal(input, &legacy); err != nil {
					return nil, fmt.Errorf("tasks is required")
				}
				req.Stories = legacy.ProposedTasks
				req.RunID = legacy.RunID
			}
			if len(req.Stories) == 0 {
				return nil, fmt.Errorf("tasks is required")
			}

			var tasks []model.PMTask
			var err error
			if strings.TrimSpace(req.RunID) != "" {
				legacy := model.ConfirmPlanningRequest{
					RunID:          strings.TrimSpace(req.RunID),
					ProposedTasks:  req.Stories,
				}
				tasks, err = s.agentService.ConfirmEpicRun(ctx, meta.WorkspaceID, meta.TargetID, legacy.RunID, fallbackActor(meta), legacy)
			} else {
				tasks, err = s.agentService.CreateEpicTaskBatch(ctx, meta.WorkspaceID, meta.TargetID, fallbackActor(meta), req.Stories)
			}
			if err != nil {
				return nil, err
			}
			results := make([]map[string]any, 0, len(tasks))
			for idx, task := range tasks {
				ref := strings.TrimSpace(req.Stories[idx].Ref)
				results = append(results, map[string]any{
					"ref":      ref,
					"task_id":  task.ID,
					"story_id": task.ID,
					"name":     task.Name,
				})
			}
			return mustJSON(map[string]any{"tasks": results, "stories": results}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.create_story_batch",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"epic"},
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			return s.Execute(ctx, meta, "pm.create_task_batch", input)
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.set_task_dependencies",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"epic", "task", "story"},
		Tool:                 mustCommandToolMetadata("pm.set_task_dependencies"),
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				Dependencies []struct {
					SourceStoryID string `json:"source_story_id"`
					TargetStoryID string `json:"target_story_id"`
					SourceTaskID  string `json:"source_task_id"`
					TargetTaskID  string `json:"target_task_id"`
				} `json:"dependencies"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse dependency input: %w", err)
			}
			for _, dep := range req.Dependencies {
				sourceID := strings.TrimSpace(firstNonEmptyCommand(dep.SourceTaskID, dep.SourceStoryID))
				targetID := strings.TrimSpace(firstNonEmptyCommand(dep.TargetTaskID, dep.TargetStoryID))
				if sourceID == "" || targetID == "" {
					return nil, fmt.Errorf("source_task_id and target_task_id are required")
				}
				source, err := s.taskRepo.GetRawByID(ctx, sourceID)
				if err != nil {
					return nil, err
				}
				target, err := s.taskRepo.GetRawByID(ctx, targetID)
				if err != nil {
					return nil, err
				}
				if source == nil || target == nil || source.WorkspaceID != meta.WorkspaceID || target.WorkspaceID != meta.WorkspaceID {
					return nil, fmt.Errorf("tasks must belong to the current workspace")
				}
				if err := s.storyLinkRepo.Create(ctx, &model.PMTaskLink{
					WorkspaceID:   meta.WorkspaceID,
					SourceStoryID: sourceID,
					TargetStoryID: targetID,
					LinkType:      model.PMTaskLinkTypeBlocks,
					CreatedBy:     fallbackActor(meta),
				}); err != nil {
					return nil, err
				}
			}
			return mustJSON(map[string]any{"dependency_count": len(req.Dependencies)}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.set_story_dependencies",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"epic", "task", "story"},
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			return s.Execute(ctx, meta, "pm.set_task_dependencies", input)
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.assign_task_agent",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"epic", "task", "story"},
		Tool:                 mustCommandToolMetadata("pm.assign_task_agent"),
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				StoryID string `json:"story_id"`
				TaskID  string `json:"task_id"`
				AgentID string `json:"agent_id"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse assign input: %w", err)
			}
			targetID := strings.TrimSpace(firstNonEmptyCommand(req.TaskID, req.StoryID, meta.TargetID))
			if targetID == "" || strings.TrimSpace(req.AgentID) == "" {
				return nil, fmt.Errorf("task_id and agent_id are required")
			}
			if err := s.agentService.AssignAgentToTask(ctx, meta.WorkspaceID, targetID, req.AgentID, fallbackActor(meta)); err != nil {
				return nil, err
			}
			return mustJSON(map[string]any{"task_id": targetID, "story_id": targetID, "agent_id": req.AgentID}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.assign_story_agent",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"epic", "task", "story"},
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			return s.Execute(ctx, meta, "pm.assign_task_agent", input)
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.create_followup_tasks",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"task", "story"},
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				Followups []model.TaskCompletionFollowupProposal `json:"followups"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse followup input: %w", err)
			}
			task, err := s.taskRepo.GetRawByID(ctx, meta.TargetID)
			if err != nil {
				return nil, err
			}
			if task == nil {
				return nil, fmt.Errorf("task not found")
			}
			createdIDs := make([]string, 0, len(req.Followups))
			for idx, followup := range req.Followups {
				title := strings.TrimSpace(followup.Title)
				if title == "" {
					return nil, fmt.Errorf("followup %d is missing a title", idx+1)
				}
				taskType := strings.TrimSpace(followup.TaskType)
				if taskType == "" {
					taskType = model.PMTaskTypeChore
				}
				description := strings.TrimSpace(followup.Description)
				createReq := model.CreateTaskRequest{
					WorkspaceID: meta.WorkspaceID,
					Name:        title,
					Description: stringPtrOrNil(description),
					TaskType:   taskType,
					EpicID:      task.EpicID,
					TeamID:      task.TeamID,
					Priority:    followup.Priority,
				}
				detail, err := s.taskService.Create(ctx, createReq, fallbackActor(meta))
				if err != nil {
					return nil, err
				}
				createdIDs = append(createdIDs, detail.Task.ID)
			}
			return mustJSON(map[string]any{"created_task_ids": createdIDs, "created_story_ids": createdIDs}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.create_followup_stories",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"task", "story"},
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			return s.Execute(ctx, meta, "pm.create_followup_tasks", input)
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.update_task_state",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"task", "story"},
		Tool:                 mustCommandToolMetadata("pm.update_task_state"),
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				StoryID  string `json:"story_id"`
				TaskID   string `json:"task_id"`
				StateID  string `json:"state_id"`
				Position *int   `json:"position,omitempty"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse move task input: %w", err)
			}
			taskID := strings.TrimSpace(firstNonEmptyCommand(req.TaskID, req.StoryID, meta.TargetID))
			if taskID == "" || strings.TrimSpace(req.StateID) == "" {
				return nil, fmt.Errorf("task_id and state_id are required")
			}
			_, err := s.taskService.MoveToState(ctx, taskID, model.MoveTaskRequest{
				StateID:  req.StateID,
				Position: req.Position,
			}, fallbackActor(meta))
			if err != nil {
				return nil, err
			}
			return mustJSON(map[string]any{"task_id": taskID, "story_id": taskID, "state_id": req.StateID}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.update_story_state",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"task", "story"},
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			return s.Execute(ctx, meta, "pm.update_task_state", input)
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "docs.write_document_content",
		Module:               "docs",
		Mutating:             true,
		SupportedTargetTypes: []string{"document", "epic", "task", "story", "crm_deal"},
		Tool:                 mustCommandToolMetadata("docs.write_document_content"),
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
		SupportedTargetTypes: []string{"epic", "task", "story", "crm_deal"},
		Tool:                 mustCommandToolMetadata("docs.link_document_to_object"),
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
		Tool:                 mustCommandToolMetadata("crm.update_deal_stage"),
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
		Tool:                 mustCommandToolMetadata("crm.add_deal_note"),
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
		SupportedTargetTypes: []string{"epic", "task", "story"},
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
		SupportedTargetTypes: []string{"epic", "task", "story"},
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
		SupportedTargetTypes: []string{"task", "story"},
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			if s.gitService == nil {
				return nil, fmt.Errorf("git service not configured")
			}
			var req struct {
				StoryID      string `json:"story_id"`
				TaskID       string `json:"task_id"`
				TargetBranch string `json:"target_branch"`
			}
			if len(input) > 0 {
				if err := json.Unmarshal(input, &req); err != nil {
					return nil, fmt.Errorf("parse merge_branch input: %w", err)
				}
			}
			storyID := firstNonEmptyCommand(req.TaskID, req.StoryID, meta.TargetID)
			if storyID == "" {
				return nil, fmt.Errorf("task_id is required")
			}
			if strings.TrimSpace(req.TargetBranch) == "" {
				return nil, fmt.Errorf("target_branch is required")
			}
			if err := s.gitService.MergeBranch(ctx, meta.WorkspaceID, storyID, req.TargetBranch); err != nil {
				return nil, err
			}
			return mustJSON(map[string]any{"task_id": storyID, "story_id": storyID, "target_branch": req.TargetBranch}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "crm.apply_deal_actions",
		Module:               "crm",
		Mutating:             true,
		SupportedTargetTypes: []string{"crm_deal"},
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

func mustCommandToolMetadata(commandName string) *commandtools.RuntimeToolMetadata {
	meta, ok := commandtools.ToolMetadataForCommand(commandName)
	if !ok {
		panic("missing runtime tool metadata for command " + commandName)
	}
	return meta
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
