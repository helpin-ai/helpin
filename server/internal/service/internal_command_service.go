package service

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
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
	agentService         *AgentService
	taskService          *PMTaskService
	labelService         *PMLabelService
	commentService       *PMCommentService
	crmDealService       *CRMDealService
	crmActivityService   *CRMActivityService
	crmEnrichmentService *CRMEnrichmentService
	docsDocumentService  *DocsDocumentService
	docsContentService   *DocsContentService
	docsBlockService     *DocsBlockService
	docsContentRepo      *repository.DocsContentRepository
	docsLinkService      *DocsLinkService
	pmAutomationService  *PMAutomationService
	gitService           *GitService
	taskRepo             *repository.PMTaskRepository
	taskLinkRepo         *repository.PMTaskLinkRepository
	definitions          map[string]InternalCommandDefinition
}

// SetPMAutomationService sets the PM automation service (breaks circular dependency).
func (s *InternalCommandService) SetPMAutomationService(svc *PMAutomationService) {
	s.pmAutomationService = svc
}

// SetPMLabelService sets the PM label service for command-backed label tools.
func (s *InternalCommandService) SetPMLabelService(svc *PMLabelService) {
	s.labelService = svc
}

// SetPMCommentService sets the PM comment service for command-backed comment tools.
func (s *InternalCommandService) SetPMCommentService(svc *PMCommentService) {
	s.commentService = svc
}

// SetGitService sets the git service for delivery commands.
func (s *InternalCommandService) SetGitService(svc *GitService) {
	s.gitService = svc
}

// SetCRMEnrichmentService sets guarded CRM enrichment dependencies.
func (s *InternalCommandService) SetCRMEnrichmentService(svc *CRMEnrichmentService) {
	s.crmEnrichmentService = svc
}

// SetDocsCreateDependencies wires document creation dependencies after service
// construction so callers can avoid circular startup ordering.
func (s *InternalCommandService) SetDocsCreateDependencies(documentSvc *DocsDocumentService, contentRepo *repository.DocsContentRepository) {
	if s == nil {
		return
	}
	s.docsDocumentService = documentSvc
	s.docsContentRepo = contentRepo
}

func (s *InternalCommandService) SetDocsBlockService(blockSvc *DocsBlockService) {
	if s == nil {
		return
	}
	s.docsBlockService = blockSvc
}

func NewInternalCommandService(
	agentService *AgentService,
	taskService *PMTaskService,
	crmDealService *CRMDealService,
	crmActivityService *CRMActivityService,
	docsContentService *DocsContentService,
	docsLinkService *DocsLinkService,
	taskRepo *repository.PMTaskRepository,
	taskLinkRepo *repository.PMTaskLinkRepository,
) *InternalCommandService {
	svc := &InternalCommandService{
		agentService:       agentService,
		taskService:        taskService,
		crmDealService:     crmDealService,
		crmActivityService: crmActivityService,
		docsContentService: docsContentService,
		docsLinkService:    docsLinkService,
		taskRepo:           taskRepo,
		taskLinkRepo:       taskLinkRepo,
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
				Tasks         []model.ProposedTask `json:"tasks"`
				ProposedTasks []model.ProposedTask `json:"proposed_tasks"`
				RunID         string               `json:"run_id,omitempty"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse task batch input: %w", err)
			}
			if len(req.Tasks) == 0 {
				req.Tasks = req.ProposedTasks
			}
			if len(req.Tasks) == 0 {
				var legacy model.ConfirmPlanningRequest
				if err := json.Unmarshal(input, &legacy); err != nil {
					return nil, fmt.Errorf("tasks is required")
				}
				req.Tasks = legacy.ProposedTasks
				req.RunID = legacy.RunID
			}
			if len(req.Tasks) == 0 {
				return nil, fmt.Errorf("tasks is required")
			}

			var tasks []model.PMTask
			var err error
			if strings.TrimSpace(req.RunID) != "" {
				legacy := model.ConfirmPlanningRequest{
					RunID:         strings.TrimSpace(req.RunID),
					ProposedTasks: req.Tasks,
				}
				tasks, err = s.agentService.ConfirmEpicRun(ctx, meta.WorkspaceID, meta.TargetID, legacy.RunID, fallbackActor(meta), legacy)
			} else {
				tasks, err = s.agentService.CreateEpicTaskBatch(ctx, meta.WorkspaceID, meta.TargetID, fallbackActor(meta), req.Tasks)
			}
			if err != nil {
				return nil, err
			}
			results := make([]map[string]any, 0, len(tasks))
			for idx, task := range tasks {
				ref := strings.TrimSpace(req.Tasks[idx].Ref)
				results = append(results, map[string]any{
					"ref":     ref,
					"task_id": task.ID,
					"name":    task.Name,
				})
			}
			return mustJSON(map[string]any{"tasks": results}), nil
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
					SourceTaskID string `json:"source_task_id"`
					TargetTaskID string `json:"target_task_id"`
				} `json:"dependencies"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse dependency input: %w", err)
			}
			for _, dep := range req.Dependencies {
				sourceID := strings.TrimSpace(dep.SourceTaskID)
				targetID := strings.TrimSpace(dep.TargetTaskID)
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
				if err := s.taskLinkRepo.Create(ctx, &model.PMTaskLink{
					WorkspaceID:  meta.WorkspaceID,
					SourceTaskID: sourceID,
					TargetTaskID: targetID,
					LinkType:     model.PMTaskLinkTypeBlocks,
					CreatedBy:    fallbackActor(meta),
				}); err != nil {
					return nil, err
				}
			}
			return mustJSON(map[string]any{"dependency_count": len(req.Dependencies)}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.create_task",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "epic"},
		Tool:                 mustCommandToolMetadata("pm.create_task"),
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				Name           string   `json:"name"`
				Description    *string  `json:"description"`
				TaskType       string   `json:"task_type"`
				Estimate       *int     `json:"estimate"`
				Priority       *string  `json:"priority"`
				EpicID         *string  `json:"epic_id"`
				TeamID         string   `json:"team_id"`
				WorkflowID     *string  `json:"workflow_id"`
				StateID        *string  `json:"state_id"`
				OwnerMemberIDs []string `json:"owner_member_ids"`
				LabelIDs       []string `json:"label_ids"`
				Deadline       *string  `json:"deadline"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse create task input: %w", err)
			}

			req.Name = strings.TrimSpace(req.Name)
			req.TeamID = strings.TrimSpace(req.TeamID)
			if req.Name == "" || req.TeamID == "" {
				return nil, fmt.Errorf("name and team_id are required")
			}

			if req.EpicID == nil && strings.TrimSpace(meta.TargetType) == "epic" && strings.TrimSpace(meta.TargetID) != "" {
				req.EpicID = stringPtrOrNil(meta.TargetID)
			}
			req.TaskType = strings.TrimSpace(req.TaskType)
			req.Description = normalizeTaskDescriptionRichText(stringPtrOrNil(commandDerefString(req.Description)))
			req.Priority = stringPtrOrNil(commandDerefString(req.Priority))
			req.WorkflowID = stringPtrOrNil(commandDerefString(req.WorkflowID))
			req.StateID = stringPtrOrNil(commandDerefString(req.StateID))
			req.OwnerMemberIDs = commandTrimStringSlice(req.OwnerMemberIDs)

			var deadline *time.Time
			if req.Deadline != nil {
				parsed, err := parseInternalCommandTaskDeadline(*req.Deadline)
				if err != nil {
					return nil, err
				}
				deadline = parsed
			}

			workflowID, stateID, err := s.resolveTaskCreationWorkflow(ctx, meta.WorkspaceID, req.TeamID, req.WorkflowID, req.StateID)
			if err != nil {
				return nil, err
			}

			createReq := model.CreateTaskRequest{
				WorkspaceID:     meta.WorkspaceID,
				Name:            req.Name,
				Description:     req.Description,
				TaskType:        req.TaskType,
				WorkflowID:      workflowID,
				WorkflowStateID: stateID,
				EpicID:          req.EpicID,
				TeamID:          stringPtrOrNil(req.TeamID),
				OwnerMemberIDs:  req.OwnerMemberIDs,
				Estimate:        req.Estimate,
				Priority:        req.Priority,
				Deadline:        deadline,
				LabelIDs:        req.LabelIDs,
			}
			detail, err := s.taskService.Create(ctx, createReq, fallbackActor(meta))
			if err != nil {
				return nil, err
			}
			stateName := ""
			if detail.State != nil {
				stateName = strings.TrimSpace(detail.State.Name)
			}
			return mustJSON(map[string]any{
				"task_id":      detail.Task.ID,
				"display_id":   detail.Task.DisplayID,
				"task_key":     detail.Task.TaskKey,
				"name":         detail.Task.Name,
				"team_id":      detail.Task.TeamID,
				"workflow_id":  detail.Task.WorkflowID,
				"state_id":     detail.Task.WorkflowStateID,
				"state_name":   stateName,
				"workspace_id": detail.Task.WorkspaceID,
				"epic_id":      detail.Task.EpicID,
			}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.assign_task_agent",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"epic", "task", "story"},
		Tool:                 mustCommandToolMetadata("pm.assign_task_agent"),
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			return nil, fmt.Errorf("task agent assignment was removed; use a workflow automation rule or start a run explicitly with an agent")
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.ensure_label",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "epic", "task", "story"},
		Tool:                 mustCommandToolMetadata("pm.ensure_label"),
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			if s.labelService == nil {
				return nil, fmt.Errorf("label service is not configured")
			}
			var req struct {
				Name        string  `json:"name"`
				TeamID      *string `json:"team_id"`
				Description *string `json:"description"`
				Color       *string `json:"color"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse ensure label input: %w", err)
			}
			name := strings.TrimSpace(req.Name)
			if name == "" {
				return nil, fmt.Errorf("name is required")
			}
			teamID := stringPtrOrNil(commandDerefString(req.TeamID))
			existing, err := s.labelService.labelRepo.GetByName(ctx, meta.WorkspaceID, teamID, name)
			if err != nil {
				return nil, err
			}
			created := false
			label := existing
			if label == nil {
				label, err = s.labelService.Create(ctx, model.CreateLabelRequest{
					WorkspaceID: meta.WorkspaceID,
					TeamID:      teamID,
					Name:        name,
					Description: stringPtrOrNil(commandDerefString(req.Description)),
					Color:       stringPtrOrNil(commandDerefString(req.Color)),
				})
				if err != nil {
					return nil, err
				}
				created = true
			} else if label.Archived {
				archived := false
				label, err = s.labelService.Update(ctx, label.ID, model.UpdateLabelRequest{Archived: &archived})
				if err != nil {
					return nil, err
				}
			}
			return mustJSON(map[string]any{
				"label_id":     label.ID,
				"name":         label.Name,
				"team_id":      label.TeamID,
				"color":        label.Color,
				"workspace_id": label.WorkspaceID,
				"created":      created,
			}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.list_tasks",
		Module:               "pm",
		Mutating:             false,
		SupportedTargetTypes: []string{"workspace", "epic", "task", "story"},
		Tool:                 mustCommandToolMetadata("pm.list_tasks"),
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			if s.taskService == nil {
				return nil, fmt.Errorf("task service is not configured")
			}
			var req struct {
				LabelID             string `json:"label_id"`
				TeamID              string `json:"team_id"`
				OpenOnly            bool   `json:"open_only"`
				IncludeDescriptions bool   `json:"include_descriptions"`
				IncludeComments     bool   `json:"include_comments"`
				Limit               int    `json:"limit"`
				DetailLevel         string `json:"detail_level"`
			}
			if len(input) > 0 {
				if err := json.Unmarshal(input, &req); err != nil {
					return nil, fmt.Errorf("parse list tasks input: %w", err)
				}
			}
			req.DetailLevel = strings.ToLower(strings.TrimSpace(req.DetailLevel))
			if req.DetailLevel != "" && req.DetailLevel != "summary" && req.DetailLevel != "compact" && req.DetailLevel != "full" {
				return nil, fmt.Errorf("detail_level must be summary, compact, or full")
			}
			limit := req.Limit
			if limit <= 0 {
				limit = 50
			}
			if limit > 100 {
				limit = 100
			}
			archived := false
			filters := model.PMTaskFilters{
				LabelID:  stringPtrOrNil(req.LabelID),
				TeamID:   stringPtrOrNil(req.TeamID),
				Archived: &archived,
			}
			if req.OpenOnly {
				completed := false
				filters.Completed = &completed
			}
			tasks, total, err := s.taskService.List(ctx, meta.WorkspaceID, filters, model.PMPagination{Page: 1, PerPage: limit})
			if err != nil {
				return nil, err
			}
			commentsByTask := map[string][]model.CommentWithAuthor{}
			if req.DetailLevel == "compact" || req.IncludeComments {
				if s.commentService == nil {
					return nil, fmt.Errorf("comment service is not configured")
				}
				taskIDs := make([]string, 0, len(tasks))
				for _, task := range tasks {
					taskIDs = append(taskIDs, task.ID)
				}
				commentsByTask, err = s.commentService.ListByEntityIDs(ctx, "task", taskIDs)
				if err != nil {
					return nil, err
				}
			}
			results := make([]map[string]any, 0, len(tasks))
			for _, task := range tasks {
				if req.DetailLevel == "compact" {
					results = append(results, buildCompactTaskItem(task, commentsByTask[task.ID]))
					continue
				}
				item := map[string]any{
					"task_id":     task.ID,
					"display_id":  task.DisplayID,
					"task_key":    task.TaskKey,
					"name":        task.Name,
					"team_id":     task.TeamID,
					"state_id":    task.WorkflowStateID,
					"state_name":  task.StateName,
					"completed":   task.Completed,
					"priority":    task.Priority,
					"severity":    task.Severity,
					"external_id": task.ExternalID,
					"updated_at":  task.UpdatedAt,
					"labels":      task.Labels,
				}
				if req.IncludeDescriptions {
					item["description"] = task.Description
				}
				if req.IncludeComments {
					item["comments"] = compactTaskComments(commentsByTask[task.ID], 10)
				}
				results = append(results, item)
			}
			if req.DetailLevel == "compact" {
				return marshalCompactTaskResponse(results, total, limit)
			}
			response := map[string]any{
				"tasks":        results,
				"total":        total,
				"limit":        limit,
				"detail_level": req.DetailLevel,
			}
			return mustJSON(response), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.add_task_comment",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "task", "story"},
		Tool:                 mustCommandToolMetadata("pm.add_task_comment"),
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			if s.commentService == nil || s.taskService == nil {
				return nil, fmt.Errorf("comment service is not configured")
			}
			var req struct {
				TaskID  string `json:"task_id"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse add task comment input: %w", err)
			}
			taskID := strings.TrimSpace(firstNonEmptyCommand(req.TaskID, meta.TargetID))
			content := strings.TrimSpace(req.Content)
			if taskID == "" || content == "" {
				return nil, fmt.Errorf("task_id and content are required")
			}
			detail, err := s.taskService.GetByID(ctx, taskID)
			if err != nil {
				return nil, err
			}
			if detail.Task.WorkspaceID != meta.WorkspaceID {
				return nil, fmt.Errorf("task not found")
			}
			if normalized := normalizeTaskDescriptionRichText(&content); normalized != nil {
				content = *normalized
			}
			comment, err := s.commentService.Create(ctx, model.CreateCommentRequest{
				EntityType: "task",
				EntityID:   taskID,
				Body:       content,
			}, fallbackActor(meta), meta.WorkspaceID)
			if err != nil {
				return nil, err
			}
			return mustJSON(map[string]any{
				"task_id":    taskID,
				"comment_id": comment.Comment.ID,
			}), nil
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
				description := normalizeTaskDescriptionRichText(stringPtrOrNil(strings.TrimSpace(followup.Description)))
				createReq := model.CreateTaskRequest{
					WorkspaceID: meta.WorkspaceID,
					Name:        title,
					Description: description,
					TaskType:    taskType,
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
		Name:                 "docs.update_document_block",
		Module:               "docs",
		Mutating:             true,
		SupportedTargetTypes: []string{"document"},
		Tool:                 mustCommandToolMetadata("docs.update_document_block"),
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			if s.docsBlockService == nil {
				return nil, fmt.Errorf("docs block service is not available")
			}
			var req struct {
				DocumentID string          `json:"document_id"`
				BlockID    string          `json:"block_id"`
				Revision   int             `json:"revision"`
				Content    json.RawMessage `json:"content"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse document block input: %w", err)
			}
			if strings.TrimSpace(req.DocumentID) == "" {
				return nil, fmt.Errorf("document_id is required")
			}
			if strings.TrimSpace(req.BlockID) == "" {
				return nil, fmt.Errorf("block_id is required")
			}
			if req.Revision <= 0 {
				return nil, fmt.Errorf("revision is required")
			}
			if len(req.Content) == 0 || strings.TrimSpace(string(req.Content)) == "" || strings.TrimSpace(string(req.Content)) == "null" {
				return nil, fmt.Errorf("content is required")
			}
			content, err := s.docsBlockService.Patch(ctx, req.DocumentID, req.BlockID, req.Revision, req.Content, meta.ActorID)
			if err != nil {
				return nil, err
			}
			return mustJSON(map[string]any{"document_id": req.DocumentID, "block_id": req.BlockID, "content_id": content.ID}), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "docs.create_document",
		Module:               "docs",
		Mutating:             true,
		SupportedTargetTypes: []string{"epic", "task", "story", "crm_deal", "workspace"},
		Tool:                 mustCommandToolMetadata("docs.create_document"),
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				SpaceID      string          `json:"space_id"`
				Title        string          `json:"title"`
				CollectionID *string         `json:"collection_id,omitempty"`
				Content      json.RawMessage `json:"content,omitempty"`
				Icon         *string         `json:"icon,omitempty"`
				Tags         []string        `json:"tags,omitempty"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse create document input: %w", err)
			}
			req.SpaceID = strings.TrimSpace(req.SpaceID)
			req.Title = strings.TrimSpace(req.Title)
			if req.SpaceID == "" {
				return nil, fmt.Errorf("space_id is required")
			}
			if req.Title == "" {
				return nil, fmt.Errorf("title is required")
			}
			if s.docsDocumentService == nil {
				return nil, fmt.Errorf("docs document service is not available")
			}

			doc, err := s.docsDocumentService.Create(ctx, meta.WorkspaceID, model.CreateDocsDocumentRequest{
				SpaceID:      req.SpaceID,
				CollectionID: req.CollectionID,
				Title:        req.Title,
				Icon:         req.Icon,
				Tags:         req.Tags,
			}, fallbackActor(meta))
			if err != nil {
				return nil, err
			}

			docContent := normalizeInternalCommandDocumentContent(req.Content)
			if !documentContentIsEffectivelyEmpty(docContent) {
				if s.docsContentRepo == nil {
					return nil, fmt.Errorf("docs content repository is not available")
				}
				if _, err := s.docsContentRepo.Upsert(ctx, doc.ID, docContent); err != nil {
					return nil, err
				}
			}

			return mustJSON(map[string]any{
				"id":       doc.ID,
				"title":    doc.Title,
				"status":   doc.Status,
				"space_id": doc.SpaceID,
			}), nil
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
		Name:                 "crm.enrich_contact",
		Module:               "crm",
		Mutating:             true,
		SupportedTargetTypes: []string{"crm_contact"},
		Tool:                 mustCommandToolMetadata("crm.enrich_contact"),
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			if s.crmEnrichmentService == nil {
				return nil, fmt.Errorf("CRM enrichment service is not configured")
			}
			var req model.EnrichCRMContactRequest
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse contact enrichment input: %w", err)
			}
			req.ContactID = strings.TrimSpace(firstNonEmptyCommand(req.ContactID, meta.TargetID))
			if req.ContactID == "" {
				return nil, fmt.Errorf("contact_id is required")
			}
			result, err := s.crmEnrichmentService.EnrichContact(ctx, meta.WorkspaceID, req)
			if err != nil {
				return nil, err
			}
			return mustJSON(result), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "crm.enrich_company",
		Module:               "crm",
		Mutating:             true,
		SupportedTargetTypes: []string{"crm_company"},
		Tool:                 mustCommandToolMetadata("crm.enrich_company"),
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			if s.crmEnrichmentService == nil {
				return nil, fmt.Errorf("CRM enrichment service is not configured")
			}
			var req model.EnrichCRMCompanyRequest
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse company enrichment input: %w", err)
			}
			req.CompanyID = strings.TrimSpace(firstNonEmptyCommand(req.CompanyID, meta.TargetID))
			if req.CompanyID == "" {
				return nil, fmt.Errorf("company_id is required")
			}
			result, err := s.crmEnrichmentService.EnrichCompany(ctx, meta.WorkspaceID, req)
			if err != nil {
				return nil, err
			}
			return mustJSON(result), nil
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "crm.ensure_contact_company",
		Module:               "crm",
		Mutating:             true,
		SupportedTargetTypes: []string{"crm_contact"},
		Tool:                 mustCommandToolMetadata("crm.ensure_contact_company"),
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			if s.crmEnrichmentService == nil {
				return nil, fmt.Errorf("CRM enrichment service is not configured")
			}
			var req model.EnsureCRMContactCompanyRequest
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse contact company input: %w", err)
			}
			req.ContactID = strings.TrimSpace(firstNonEmptyCommand(req.ContactID, meta.TargetID))
			if req.ContactID == "" {
				return nil, fmt.Errorf("contact_id is required")
			}
			result, err := s.crmEnrichmentService.EnsureContactCompany(ctx, meta.WorkspaceID, req)
			if err != nil {
				return nil, err
			}
			return mustJSON(result), nil
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
			if err := s.gitService.UpdateDeliveryStatusAfterMerge(ctx, meta.WorkspaceID, storyID, "merged"); err != nil {
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

func normalizeInternalCommandDocumentContent(raw json.RawMessage) json.RawMessage {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	content := json.RawMessage(trimmed)
	if len(content) > 0 && content[0] == '"' {
		var markdown string
		if err := json.Unmarshal(content, &markdown); err == nil {
			if strings.TrimSpace(markdown) == "" {
				return nil
			}
			return tiptap.MarkdownToJSON(markdown)
		}
	}
	return content
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

func (s *InternalCommandService) resolveTaskCreationWorkflow(ctx context.Context, workspaceID, teamID string, requestedWorkflowID, requestedStateID *string) (string, string, error) {
	if s == nil || s.taskService == nil || s.taskService.workflowRepo == nil {
		return "", "", fmt.Errorf("workflow service is not configured")
	}
	teamID = strings.TrimSpace(teamID)
	if workspaceID == "" || teamID == "" {
		return "", "", fmt.Errorf("workspace_id and team_id are required")
	}

	workflowID := commandDerefString(requestedWorkflowID)
	stateID := commandDerefString(requestedStateID)

	var workflow *model.WorkflowWithStates
	if workflowID != "" {
		loaded, err := s.taskService.workflowRepo.GetByID(ctx, workflowID)
		if err != nil {
			return "", "", fmt.Errorf("get workflow: %w", err)
		}
		if loaded == nil || loaded.Workflow.WorkspaceID != workspaceID {
			return "", "", fmt.Errorf("workflow not found")
		}
		if loaded.Workflow.TeamID != nil && strings.TrimSpace(*loaded.Workflow.TeamID) != "" && strings.TrimSpace(*loaded.Workflow.TeamID) != teamID {
			return "", "", fmt.Errorf("workflow_id does not belong to team_id")
		}
		workflow = loaded
	} else {
		resolved, err := s.taskService.workflowRepo.GetByTeamID(ctx, workspaceID, teamID)
		if err != nil {
			return "", "", fmt.Errorf("resolve team workflow: %w", err)
		}
		if resolved == nil {
			resolved, err = s.taskService.workflowRepo.GetDefaultWorkflow(ctx, workspaceID)
			if err != nil {
				return "", "", fmt.Errorf("resolve default workflow: %w", err)
			}
			if resolved == nil {
				resolved, err = s.taskService.workflowRepo.SeedDefaultWorkflow(ctx, workspaceID)
				if err != nil {
					return "", "", fmt.Errorf("seed default workflow: %w", err)
				}
			}
		}
		workflow = resolved
	}
	if workflow == nil {
		return "", "", fmt.Errorf("workflow not found")
	}

	if workflowID == "" {
		workflowID = strings.TrimSpace(workflow.Workflow.ID)
	}
	if workflowID == "" {
		return "", "", fmt.Errorf("workflow_id could not be resolved")
	}
	if stateID == "" && workflow.Workflow.DefaultStateID != nil {
		stateID = strings.TrimSpace(*workflow.Workflow.DefaultStateID)
	}
	if stateID == "" {
		for _, state := range workflow.States {
			if state.IsDefault {
				stateID = strings.TrimSpace(state.ID)
				break
			}
		}
	}
	if stateID == "" && len(workflow.States) > 0 {
		stateID = strings.TrimSpace(workflow.States[0].ID)
	}
	if stateID == "" {
		return "", "", fmt.Errorf("workflow has no usable default state")
	}
	var matchedState *model.PMWorkflowState
	for idx := range workflow.States {
		if strings.TrimSpace(workflow.States[idx].ID) == stateID {
			matchedState = &workflow.States[idx]
			break
		}
	}
	if matchedState == nil {
		return "", "", fmt.Errorf("state_id does not belong to workflow_id")
	}
	return workflowID, stateID, nil
}

func parseInternalCommandTaskDeadline(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339, time.RFC3339Nano} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return &parsed, nil
		}
	}
	return nil, fmt.Errorf("deadline must be YYYY-MM-DD or RFC3339")
}

func commandDerefString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func commandTrimStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		trimmed = append(trimmed, value)
	}
	return trimmed
}

func compactTaskComments(comments []model.CommentWithAuthor, limit int) []map[string]any {
	if limit <= 0 || limit > len(comments) {
		limit = len(comments)
	}
	start := len(comments) - limit
	out := make([]map[string]any, 0, limit)
	for i := start; i < len(comments); i++ {
		comment := comments[i].Comment
		out = append(out, map[string]any{
			"comment_id": comment.ID,
			"author_id":  comment.AuthorID,
			"body":       comment.Body,
			"created_at": comment.CreatedAt,
			"updated_at": comment.UpdatedAt,
		})
	}
	return out
}

func helpinCommandCompactionHint() map[string]any {
	return map[string]any{
		"exempt":    true,
		"max_runes": 30000,
		"mode":      "bounded_index",
	}
}

func buildCompactTaskItem(task model.BoardTask, comments []model.CommentWithAuthor) map[string]any {
	description := commandDerefString(task.Description)
	item := map[string]any{
		"task_id":             task.ID,
		"display_id":          task.DisplayID,
		"task_key":            task.TaskKey,
		"name":                task.Name,
		"team_id":             task.TeamID,
		"state_id":            task.WorkflowStateID,
		"state_name":          task.StateName,
		"completed":           task.Completed,
		"priority":            task.Priority,
		"severity":            task.Severity,
		"external_id":         task.ExternalID,
		"updated_at":          task.UpdatedAt,
		"labels":              compactTaskLabels(task.Labels),
		"description_excerpt": richTextPlainExcerpt(description, 500),
		"comment_excerpts":    compactCommentExcerpts(comments, 3, 300),
	}
	return item
}

func marshalCompactTaskResponse(tasks []map[string]any, total int64, limit int) (json.RawMessage, error) {
	response := map[string]any{
		"_helpin_compaction": helpinCommandCompactionHint(),
		"tasks":              tasks,
		"total":              total,
		"limit":              limit,
		"returned_tasks":     len(tasks),
		"detail_level":       "compact",
	}
	out, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	if len([]rune(string(out))) <= 30000 {
		return out, nil
	}

	response["bounded"] = true
	response["has_more"] = true
	response["warnings"] = []string{"compact_task_excerpts_bounded; load selected task details if more context is required"}
	for _, task := range tasks {
		if excerpt, ok := task["description_excerpt"].(string); ok {
			task["description_excerpt"] = truncatePlainRunes(excerpt, 160)
		}
		task["comment_excerpts"] = []map[string]any{}
	}
	out, err = json.Marshal(response)
	if err != nil {
		return nil, err
	}
	if len([]rune(string(out))) <= 30000 {
		return out, nil
	}

	for _, task := range tasks {
		task["description_excerpt"] = ""
	}
	out, err = json.Marshal(response)
	if err != nil {
		return nil, err
	}
	if len([]rune(string(out))) <= 30000 {
		return out, nil
	}

	for count := len(tasks); count >= 0; count-- {
		response["tasks"] = tasks[:count]
		response["returned_tasks"] = count
		out, err = json.Marshal(response)
		if err != nil {
			return nil, err
		}
		if len([]rune(string(out))) <= 30000 {
			return out, nil
		}
	}
	return out, nil
}

func compactTaskLabels(labels []model.PMLabel) []map[string]any {
	out := make([]map[string]any, 0, len(labels))
	for _, label := range labels {
		out = append(out, map[string]any{
			"label_id": label.ID,
			"name":     label.Name,
		})
	}
	return out
}

func compactCommentExcerpts(comments []model.CommentWithAuthor, limit int, charBudget int) []map[string]any {
	if limit <= 0 || limit > len(comments) {
		limit = len(comments)
	}
	start := len(comments) - limit
	out := make([]map[string]any, 0, limit)
	for i := start; i < len(comments); i++ {
		comment := comments[i].Comment
		out = append(out, map[string]any{
			"comment_id": comment.ID,
			"author_id":  comment.AuthorID,
			"excerpt":    richTextPlainExcerpt(comment.Body, charBudget),
			"created_at": comment.CreatedAt,
			"updated_at": comment.UpdatedAt,
		})
	}
	return out
}

func richTextPlainExcerpt(value string, limit int) string {
	plain := strings.Join(strings.Fields(stripHTMLPreservingComments(value)), " ")
	if plain == "" {
		plain = strings.Join(strings.Fields(value), " ")
	}
	return truncatePlainRunes(plain, limit)
}

func stripHTMLPreservingComments(value string) string {
	if value == "" {
		return ""
	}
	comments := extractHTMLComments(value)
	plain := html.UnescapeString(tiptap.StripHTML(value))
	if len(comments) == 0 {
		return plain
	}
	return strings.Join(comments, " ") + " " + plain
}

func extractHTMLComments(value string) []string {
	var comments []string
	for {
		start := strings.Index(value, "<!--")
		if start < 0 {
			break
		}
		remaining := value[start+4:]
		relativeEnd := strings.Index(remaining, "-->")
		if relativeEnd < 0 {
			break
		}
		end := start + 4 + relativeEnd + len("-->")
		comment := strings.TrimSpace(html.UnescapeString(value[start:end]))
		if comment != "" {
			comments = append(comments, comment)
		}
		value = value[end:]
	}
	return comments
}

func truncatePlainRunes(value string, limit int) string {
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 1 {
		return string(runes[:limit])
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}
