package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

type PMRecurringTemplateService struct {
	recurringRepo    *repository.PMRecurringTemplateRepository
	taskRepo         *repository.PMTaskRepository
	workflowRepo     *repository.PMWorkflowRepository
	sprintRepo       *repository.PMSprintRepository
	workspaceRepo    *repository.WorkspaceRepository
	checklistRepo    *repository.PMChecklistItemRepository
	externalLinkRepo *repository.PMExternalLinkRepository
	activityService  *PMActivityService
	taskService      *PMTaskService
	workflowRunner   recurringTemplateWorkflowRunner
	wsPublisher      *websocket.Publisher
	logger           *slog.Logger
}

func NewPMRecurringTemplateService(
	recurringRepo *repository.PMRecurringTemplateRepository,
	taskRepo *repository.PMTaskRepository,
	workflowRepo *repository.PMWorkflowRepository,
	sprintRepo *repository.PMSprintRepository,
	workspaceRepo *repository.WorkspaceRepository,
	checklistRepo *repository.PMChecklistItemRepository,
	externalLinkRepo *repository.PMExternalLinkRepository,
	activityService *PMActivityService,
	wsPublisher *websocket.Publisher,
) *PMRecurringTemplateService {
	return &PMRecurringTemplateService{
		recurringRepo:    recurringRepo,
		taskRepo:         taskRepo,
		workflowRepo:     workflowRepo,
		sprintRepo:       sprintRepo,
		workspaceRepo:    workspaceRepo,
		checklistRepo:    checklistRepo,
		externalLinkRepo: externalLinkRepo,
		activityService:  activityService,
		wsPublisher:      wsPublisher,
		logger:           slog.Default().With("service", "pm_recurring_template"),
	}
}

func (s *PMRecurringTemplateService) SetTaskService(taskService *PMTaskService) {
	s.taskService = taskService
}

func (s *PMRecurringTemplateService) requireCanEdit(ctx context.Context, workspaceID, actorID string) error {
	if workspaceID == "" || actorID == "" {
		return &model.ErrForbidden{Message: "workspace_id and user_id are required"}
	}
	role, err := s.workspaceRepo.GetMemberRole(ctx, workspaceID, actorID)
	if err != nil {
		return err
	}
	if role == model.RoleViewer {
		return &model.ErrForbidden{Message: "edit access required"}
	}
	return nil
}

func (s *PMRecurringTemplateService) List(ctx context.Context, workspaceID string, status, teamID, search *string) ([]model.RecurringTemplateDetail, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	templates, err := s.recurringRepo.ListByWorkspace(ctx, workspaceID, status, teamID, search)
	if err != nil {
		return nil, err
	}
	result := make([]model.RecurringTemplateDetail, 0, len(templates))
	for i := range templates {
		detail, err := s.buildDetail(ctx, &templates[i], false)
		if err != nil {
			return nil, err
		}
		result = append(result, detail)
	}
	return result, nil
}

func (s *PMRecurringTemplateService) GetByID(ctx context.Context, id string) (*model.RecurringTemplateDetail, error) {
	tmpl, err := s.recurringRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tmpl == nil {
		return nil, fmt.Errorf("recurring template not found")
	}
	detail, err := s.buildDetail(ctx, tmpl, true)
	if err != nil {
		return nil, err
	}
	return &detail, nil
}

func (s *PMRecurringTemplateService) GetByStoryID(ctx context.Context, storyID string) (*model.TaskRecurringSummary, error) {
	if storyID == "" {
		return nil, fmt.Errorf("story_id is required")
	}
	story, err := s.taskRepo.GetRawByID(ctx, storyID)
	if err != nil {
		return nil, err
	}
	if story == nil || story.RecurringTemplateID == nil || *story.RecurringTemplateID == "" {
		return nil, nil
	}
	tmpl, err := s.recurringRepo.GetByID(ctx, *story.RecurringTemplateID)
	if err != nil {
		return nil, err
	}
	if tmpl == nil {
		return nil, nil
	}
	cfg, err := parseRecurringConfig(tmpl.Config)
	if err != nil {
		return nil, err
	}
	var lastGenerated *model.PMTask
	if tmpl.LastGeneratedTaskID != nil {
		lastGenerated, err = s.taskRepo.GetRawByID(ctx, *tmpl.LastGeneratedTaskID)
		if err != nil {
			return nil, err
		}
	}
	summary := &model.TaskRecurringSummary{
		TemplateID:        tmpl.ID,
		TemplateTitle:     tmpl.Title,
		Status:            tmpl.Status,
		GeneratedCount:    tmpl.GeneratedCount,
		RuleSummary:       recurringRuleSummary(cfg),
		NextRunAt:         tmpl.NextRunAt,
		LastError:         tmpl.LastError,
		Config:            cfg,
		LastGeneratedTask: lastGenerated,
	}
	if story.RecurringOccurrenceNumber != nil {
		summary.OccurrenceNumber = *story.RecurringOccurrenceNumber
	}
	return summary, nil
}

func (s *PMRecurringTemplateService) Create(ctx context.Context, req model.CreateRecurringTemplateRequest, actorID string) (*model.RecurringTemplateDetail, error) {
	if req.WorkspaceID == "" || req.TaskID == "" || strings.TrimSpace(req.Title) == "" {
		return nil, fmt.Errorf("workspace_id, story_id, and title are required")
	}
	if err := s.requireCanEdit(ctx, req.WorkspaceID, actorID); err != nil {
		return nil, err
	}
	cfg, err := normalizeRecurringConfig(req.Config)
	if err != nil {
		return nil, err
	}

	rawStory, err := s.taskRepo.GetRawByID(ctx, req.TaskID)
	if err != nil {
		return nil, err
	}
	if rawStory == nil || rawStory.WorkspaceID != req.WorkspaceID {
		return nil, fmt.Errorf("story not found")
	}
	if rawStory.RecurringTemplateID != nil && *rawStory.RecurringTemplateID != "" {
		return nil, fmt.Errorf("story is already linked to a recurring template")
	}
	if err := requireTeamAccess(ctx, rawStory.TeamID); err != nil {
		return nil, fmt.Errorf("story not found")
	}

	story, err := s.taskRepo.GetByID(ctx, req.TaskID)
	if err != nil {
		return nil, err
	}
	if story == nil {
		return nil, fmt.Errorf("story not found")
	}
	seed, err := s.buildSeedFromTask(ctx, story)
	if err != nil {
		return nil, err
	}
	seedJSON, err := json.Marshal(seed)
	if err != nil {
		return nil, fmt.Errorf("marshal recurring seed: %w", err)
	}
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal recurring config: %w", err)
	}

	tmpl := &model.PMRecurringTemplate{
		WorkspaceID:          req.WorkspaceID,
		TeamID:               rawStory.TeamID,
		Title:                strings.TrimSpace(req.Title),
		Description:          req.Description,
		Status:               model.PMRecurringTemplateStatusActive,
		OwnerMemberID:        rawStory.OwnerMemberID,
		CreatedFromTaskID:    &rawStory.ID,
		SeedPayload:          seedJSON,
		Config:               cfgJSON,
		StartDate:            cfg.StartsOn,
		EndDate:              cfg.EndsOn,
		EndsAfterOccurrences: cfg.EndsAfterOccurrences,
		CreatedByID:          optionalString(actorID),
		UpdatedByID:          optionalString(actorID),
		GeneratedCount:       1,
		LastGeneratedTaskID:  &rawStory.ID,
	}
	now := time.Now().UTC()
	tmpl.LastRunAt = &now
	if reachedRecurringEnd(cfg, 1) {
		tmpl.Status = model.PMRecurringTemplateStatusStopped
	} else if cfg.ScheduleType == model.PMRecurringScheduleTypeTime {
		nextRun, err := computeNextRecurringRun(rawStory.CreatedAt, cfg)
		if err != nil {
			return nil, err
		}
		tmpl.NextRunAt = nextRun
	}
	if err := s.recurringRepo.Create(ctx, tmpl); err != nil {
		return nil, err
	}

	run := &model.PMRecurringRun{
		WorkspaceID:      req.WorkspaceID,
		TemplateID:       tmpl.ID,
		OccurrenceNumber: 1,
		TriggerType:      model.PMRecurringRunTriggerManualSeed,
		Status:           model.PMRecurringRunStatusSucceeded,
		GeneratedTaskID:  &rawStory.ID,
		DedupeKey:        recurringRunDedupeKey(tmpl.ID, model.PMRecurringRunTriggerManualSeed, 1),
	}
	run.StartedAt = &now
	run.FinishedAt = &now
	if err := s.recurringRepo.CreateRun(ctx, run); err != nil {
		return nil, err
	}

	rawStory.RecurringTemplateID = &tmpl.ID
	rawStory.RecurringRunID = &run.ID
	rawStory.RecurringOccurrenceNumber = recurringIntPtr(1)
	if err := s.taskRepo.Update(ctx, rawStory); err != nil {
		return nil, err
	}

	if s.activityService != nil {
		_ = s.activityService.Log(ctx, req.WorkspaceID, "task", rawStory.ID, optionalActor(actorID), "made this task recurring", nil, nil, nil, nil)
	}

	detail, err := s.buildDetail(ctx, tmpl, true)
	if err != nil {
		return nil, err
	}
	publishWorkspaceEvent(s.wsPublisher, "created", "recurring_template", tmpl.ID, tmpl.WorkspaceID, actorID)
	return &detail, nil
}

func (s *PMRecurringTemplateService) Update(ctx context.Context, id string, req model.UpdateRecurringTemplateRequest, actorID string) (*model.RecurringTemplateDetail, error) {
	tmpl, err := s.recurringRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tmpl == nil {
		return nil, fmt.Errorf("recurring template not found")
	}
	if err := s.requireCanEdit(ctx, tmpl.WorkspaceID, actorID); err != nil {
		return nil, err
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, fmt.Errorf("title cannot be empty")
		}
		tmpl.Title = title
	}
	if req.Description != nil {
		tmpl.Description = req.Description
	}
	if req.TaskID != nil && strings.TrimSpace(*req.TaskID) != "" {
		rawStory, err := s.taskRepo.GetRawByID(ctx, strings.TrimSpace(*req.TaskID))
		if err != nil {
			return nil, err
		}
		if rawStory == nil || rawStory.WorkspaceID != tmpl.WorkspaceID {
			return nil, fmt.Errorf("story not found")
		}
		story, err := s.taskRepo.GetByID(ctx, rawStory.ID)
		if err != nil {
			return nil, err
		}
		if story == nil {
			return nil, fmt.Errorf("story not found")
		}
		seed, err := s.buildSeedFromTask(ctx, story)
		if err != nil {
			return nil, err
		}
		seedJSON, err := json.Marshal(seed)
		if err != nil {
			return nil, fmt.Errorf("marshal recurring seed: %w", err)
		}
		tmpl.SeedPayload = seedJSON
		tmpl.TeamID = rawStory.TeamID
		tmpl.OwnerMemberID = rawStory.OwnerMemberID
		tmpl.CreatedFromTaskID = &rawStory.ID
	}
	if req.Config != nil {
		cfg, err := normalizeRecurringConfig(*req.Config)
		if err != nil {
			return nil, err
		}
		cfgJSON, err := json.Marshal(cfg)
		if err != nil {
			return nil, err
		}
		tmpl.Config = cfgJSON
		tmpl.StartDate = cfg.StartsOn
		tmpl.EndDate = cfg.EndsOn
		tmpl.EndsAfterOccurrences = cfg.EndsAfterOccurrences
		if tmpl.Status == model.PMRecurringTemplateStatusActive && cfg.ScheduleType == model.PMRecurringScheduleTypeTime && !reachedRecurringEnd(cfg, tmpl.GeneratedCount) {
			ref := time.Now().UTC()
			if tmpl.LastRunAt != nil {
				ref = *tmpl.LastRunAt
			}
			nextRun, err := computeNextRecurringRun(ref, cfg)
			if err != nil {
				return nil, err
			}
			tmpl.NextRunAt = nextRun
		} else if cfg.ScheduleType == model.PMRecurringScheduleTypeCompletion {
			tmpl.NextRunAt = nil
		}
	}
	tmpl.UpdatedByID = optionalString(actorID)
	if err := s.recurringRepo.Update(ctx, tmpl); err != nil {
		return nil, err
	}
	detail, err := s.buildDetail(ctx, tmpl, true)
	if err != nil {
		return nil, err
	}
	publishWorkspaceEvent(s.wsPublisher, "updated", "recurring_template", tmpl.ID, tmpl.WorkspaceID, actorID)
	return &detail, nil
}

func (s *PMRecurringTemplateService) Pause(ctx context.Context, id, actorID string) (*model.RecurringTemplateDetail, error) {
	return s.updateStatus(ctx, id, actorID, model.PMRecurringTemplateStatusPaused, false)
}

func (s *PMRecurringTemplateService) Resume(ctx context.Context, id, actorID string) (*model.RecurringTemplateDetail, error) {
	return s.updateStatus(ctx, id, actorID, model.PMRecurringTemplateStatusActive, false)
}

func (s *PMRecurringTemplateService) Stop(ctx context.Context, id, actorID string) (*model.RecurringTemplateDetail, error) {
	return s.updateStatus(ctx, id, actorID, model.PMRecurringTemplateStatusStopped, true)
}

func (s *PMRecurringTemplateService) SkipNext(ctx context.Context, id, actorID string) (*model.RecurringTemplateDetail, error) {
	tmpl, err := s.recurringRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tmpl == nil {
		return nil, fmt.Errorf("recurring template not found")
	}
	if err := s.requireCanEdit(ctx, tmpl.WorkspaceID, actorID); err != nil {
		return nil, err
	}
	tmpl.SkipNextRun = true
	tmpl.UpdatedByID = optionalString(actorID)
	if err := s.recurringRepo.Update(ctx, tmpl); err != nil {
		return nil, err
	}
	detail, err := s.buildDetail(ctx, tmpl, true)
	if err != nil {
		return nil, err
	}
	return &detail, nil
}

func (s *PMRecurringTemplateService) GenerateNow(ctx context.Context, id, actorID string) (*model.RecurringTemplateDetail, error) {
	tmpl, err := s.recurringRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tmpl == nil {
		return nil, fmt.Errorf("recurring template not found")
	}
	if err := s.requireCanEdit(ctx, tmpl.WorkspaceID, actorID); err != nil {
		return nil, err
	}
	cfg, err := parseRecurringConfig(tmpl.Config)
	if err != nil {
		return nil, err
	}
	if cfg.ScheduleType == model.PMRecurringScheduleTypeCompletion && tmpl.LastGeneratedTaskID != nil {
		lastStory, err := s.taskRepo.GetRawByID(ctx, *tmpl.LastGeneratedTaskID)
		if err != nil {
			return nil, err
		}
		if lastStory != nil && !lastStory.Completed {
			return nil, fmt.Errorf("complete the current occurrence before generating another one")
		}
	}
	now := time.Now().UTC()
	if _, err := s.generateTaskFromTemplate(ctx, tmpl, cfg, model.PMRecurringRunTriggerManualNow, &now, actorID); err != nil {
		return nil, err
	}
	detail, err := s.buildDetail(ctx, tmpl, true)
	if err != nil {
		return nil, err
	}
	return &detail, nil
}

func (s *PMRecurringTemplateService) Duplicate(ctx context.Context, id, actorID string) (*model.RecurringTemplateDetail, error) {
	tmpl, err := s.recurringRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tmpl == nil {
		return nil, fmt.Errorf("recurring template not found")
	}
	if err := s.requireCanEdit(ctx, tmpl.WorkspaceID, actorID); err != nil {
		return nil, err
	}

	clone := *tmpl
	clone.ID = ""
	clone.Title = tmpl.Title + " (copy)"
	clone.Status = model.PMRecurringTemplateStatusPaused
	clone.GeneratedCount = 0
	clone.LastGeneratedTaskID = nil
	clone.LastRunAt = nil
	clone.NextRunAt = nil
	clone.LastError = nil
	clone.FailureCount = 0
	clone.SkipNextRun = false
	clone.CreatedByID = optionalString(actorID)
	clone.UpdatedByID = optionalString(actorID)
	now := time.Now().UTC()
	clone.CreatedAt = now
	clone.UpdatedAt = now
	if err := s.recurringRepo.Create(ctx, &clone); err != nil {
		return nil, err
	}
	detail, err := s.buildDetail(ctx, &clone, true)
	if err != nil {
		return nil, err
	}
	publishWorkspaceEvent(s.wsPublisher, "created", "recurring_template", clone.ID, clone.WorkspaceID, actorID)
	return &detail, nil
}

func (s *PMRecurringTemplateService) ProcessDueTemplates(ctx context.Context, limit int) (int, error) {
	now := time.Now().UTC()
	templates, err := s.recurringRepo.ListDue(ctx, now, limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	for i := range templates {
		cfg, err := parseRecurringConfig(templates[i].Config)
		if err != nil {
			return processed, err
		}
		if cfg.ScheduleType != model.PMRecurringScheduleTypeTime {
			continue
		}
		if templates[i].SkipNextRun {
			if err := s.recordSkippedRun(ctx, &templates[i], cfg, now); err != nil {
				return processed, err
			}
			processed++
			continue
		}
		if _, err := s.generateTaskFromTemplate(ctx, &templates[i], cfg, model.PMRecurringRunTriggerSchedule, templates[i].NextRunAt, derefString(templates[i].CreatedByID)); err != nil {
			msg := err.Error()
			templates[i].Status = model.PMRecurringTemplateStatusFailed
			templates[i].LastError = &msg
			templates[i].FailureCount++
			templates[i].UpdatedByID = optionalString(derefString(templates[i].CreatedByID))
			_ = s.recurringRepo.Update(ctx, &templates[i])
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func (s *PMRecurringTemplateService) HandleStoryProgress(ctx context.Context, storyID string) error {
	if storyID == "" {
		return nil
	}
	story, err := s.taskRepo.GetRawByID(ctx, storyID)
	if err != nil || story == nil || story.RecurringTemplateID == nil || *story.RecurringTemplateID == "" {
		return err
	}
	tmpl, err := s.recurringRepo.GetByID(ctx, *story.RecurringTemplateID)
	if err != nil || tmpl == nil {
		return err
	}
	if tmpl.Status != model.PMRecurringTemplateStatusActive {
		return nil
	}
	if tmpl.LastGeneratedTaskID == nil || *tmpl.LastGeneratedTaskID != story.ID {
		return nil
	}
	cfg, err := parseRecurringConfig(tmpl.Config)
	if err != nil {
		return err
	}
	if cfg.ScheduleType != model.PMRecurringScheduleTypeCompletion {
		return nil
	}
	matched, err := s.storyMatchesCompletionEvent(ctx, story, cfg)
	if err != nil || !matched {
		return err
	}

	now := time.Now().UTC()
	if tmpl.SkipNextRun {
		return s.recordCompletionSkip(ctx, tmpl, now)
	}
	if reachedRecurringEnd(cfg, tmpl.GeneratedCount) {
		tmpl.Status = model.PMRecurringTemplateStatusStopped
		tmpl.NextRunAt = nil
		tmpl.UpdatedByID = optionalString(derefString(tmpl.CreatedByID))
		return s.recurringRepo.Update(ctx, tmpl)
	}
	_, err = s.generateTaskFromTemplate(ctx, tmpl, cfg, model.PMRecurringRunTriggerCompletion, &now, derefString(tmpl.CreatedByID))
	return err
}

func (s *PMRecurringTemplateService) buildDetail(ctx context.Context, tmpl *model.PMRecurringTemplate, includeRuns bool) (model.RecurringTemplateDetail, error) {
	cfg, err := parseRecurringConfig(tmpl.Config)
	if err != nil {
		return model.RecurringTemplateDetail{}, err
	}
	seed, err := parseRecurringSeed(tmpl.SeedPayload)
	if err != nil {
		return model.RecurringTemplateDetail{}, err
	}
	detail := model.RecurringTemplateDetail{
		Template:    *tmpl,
		Config:      cfg,
		Seed:        seed,
		RuleSummary: recurringRuleSummary(cfg),
	}
	if tmpl.LastGeneratedTaskID != nil && *tmpl.LastGeneratedTaskID != "" {
		lastStory, err := s.taskRepo.GetRawByID(ctx, *tmpl.LastGeneratedTaskID)
		if err != nil {
			return model.RecurringTemplateDetail{}, err
		}
		detail.LastGeneratedTask = lastStory
	}
	if includeRuns {
		runs, err := s.recurringRepo.ListRuns(ctx, tmpl.ID, 20)
		if err != nil {
			return model.RecurringTemplateDetail{}, err
		}
		detail.Runs = runs
	}
	return detail, nil
}

func (s *PMRecurringTemplateService) buildSeedFromTask(ctx context.Context, story *model.TaskDetail) (model.PMRecurringTaskSeed, error) {
	checklistItems, err := s.checklistRepo.List(ctx, story.Task.ID)
	if err != nil {
		return model.PMRecurringTaskSeed{}, err
	}
	externalLinks, err := s.externalLinkRepo.List(ctx, story.Task.ID)
	if err != nil {
		return model.PMRecurringTaskSeed{}, err
	}

	stateID := story.Task.WorkflowStateID
	if story.State != nil && story.State.StateType == model.PMStateTypeDone {
		if workflow, err := s.workflowRepo.GetByID(ctx, story.Task.WorkflowID); err == nil && workflow != nil && workflow.Workflow.DefaultStateID != nil {
			stateID = *workflow.Workflow.DefaultStateID
		}
	}

	ownerIDs := make([]string, 0, len(story.Owners))
	for _, owner := range story.Owners {
		ownerIDs = append(ownerIDs, owner.ID)
	}
	followerIDs := make([]string, 0, len(story.Followers))
	for _, follower := range story.Followers {
		followerIDs = append(followerIDs, follower.ID)
	}
	labelIDs := make([]string, 0, len(story.Labels))
	for _, label := range story.Labels {
		labelIDs = append(labelIDs, label.ID)
	}
	checklist := make([]model.CreateChecklistItemRequest, 0, len(checklistItems))
	for _, item := range checklistItems {
		checklist = append(checklist, model.CreateChecklistItemRequest{
			Text:       item.Text,
			Position:   &item.Position,
			AssigneeID: item.AssigneeID,
		})
	}
	links := make([]model.CreateExternalLinkRequest, 0, len(externalLinks))
	for _, link := range externalLinks {
		links = append(links, model.CreateExternalLinkRequest{
			URL:   link.URL,
			Title: link.Title,
		})
	}
	return model.PMRecurringTaskSeed{
		Name:              story.Task.Name,
		Description:       story.Task.Description,
		TaskType:          story.Task.TaskType,
		WorkflowID:        story.Task.WorkflowID,
		WorkflowStateID:   stateID,
		EpicID:            story.Task.EpicID,
		TeamID:            story.Task.TeamID,
		OwnerMemberID:     story.Task.OwnerMemberID,
		RequesterMemberID: story.Task.RequesterMemberID,
		Estimate:          story.Task.Estimate,
		Priority:          optionalString(story.Task.Priority),
		Severity:          optionalString(story.Task.Severity),
		OwnerIDs:          dedupeIDs(ownerIDs),
		FollowerIDs:       dedupeIDs(followerIDs),
		LabelIDs:          dedupeIDs(labelIDs),
		ChecklistItems:    checklist,
		ExternalLinks:     links,
	}, nil
}

func (s *PMRecurringTemplateService) generateTaskFromTemplate(ctx context.Context, tmpl *model.PMRecurringTemplate, cfg model.PMRecurringTemplateConfig, triggerType string, scheduledFor *time.Time, actorID string) (*model.TaskDetail, error) {
	if s.taskService == nil {
		return nil, fmt.Errorf("task service is not configured")
	}
	if reachedRecurringEnd(cfg, tmpl.GeneratedCount) {
		tmpl.Status = model.PMRecurringTemplateStatusStopped
		tmpl.NextRunAt = nil
		return nil, s.recurringRepo.Update(ctx, tmpl)
	}

	seed, err := parseRecurringSeed(tmpl.SeedPayload)
	if err != nil {
		return nil, err
	}
	occurrenceNumber := tmpl.GeneratedCount + 1
	dedupeKey := recurringRunDedupeKey(tmpl.ID, triggerType, occurrenceNumber)
	if existing, err := s.recurringRepo.GetRunByDedupeKey(ctx, dedupeKey); err != nil {
		return nil, err
	} else if existing != nil && existing.GeneratedTaskID != nil {
		story, err := s.taskRepo.GetByID(ctx, *existing.GeneratedTaskID)
		if err != nil {
			return nil, err
		}
		if story == nil {
			return nil, nil
		}
		return story, nil
	}

	now := time.Now().UTC()
	run := &model.PMRecurringRun{
		WorkspaceID:      tmpl.WorkspaceID,
		TemplateID:       tmpl.ID,
		OccurrenceNumber: occurrenceNumber,
		TriggerType:      triggerType,
		ScheduledFor:     scheduledFor,
		Status:           model.PMRecurringRunStatusFailed,
		DedupeKey:        dedupeKey,
	}
	run.StartedAt = &now
	if err := s.recurringRepo.CreateRun(ctx, run); err != nil {
		return nil, err
	}

	createReq, err := s.seedToCreateRequest(ctx, seed, cfg, tmpl, scheduledFor)
	if err != nil {
		return nil, err
	}
	story, err := s.taskService.Create(ctx, createReq, actorID)
	finishedAt := time.Now().UTC()
	run.FinishedAt = &finishedAt
	if err != nil {
		msg := err.Error()
		run.ErrorMessage = &msg
		_ = s.recurringRepo.UpdateRun(ctx, run)
		return nil, err
	}

	rawStory, err := s.taskRepo.GetRawByID(ctx, story.Task.ID)
	if err != nil {
		return nil, err
	}
	rawStory.RecurringTemplateID = &tmpl.ID
	rawStory.RecurringRunID = &run.ID
	rawStory.RecurringOccurrenceNumber = &occurrenceNumber
	if err := s.taskRepo.Update(ctx, rawStory); err != nil {
		return nil, err
	}

	run.Status = model.PMRecurringRunStatusSucceeded
	run.GeneratedTaskID = &story.Task.ID
	run.ErrorMessage = nil
	if err := s.recurringRepo.UpdateRun(ctx, run); err != nil {
		return nil, err
	}

	tmpl.GeneratedCount = occurrenceNumber
	tmpl.LastGeneratedTaskID = &story.Task.ID
	tmpl.LastRunAt = &finishedAt
	tmpl.LastError = nil
	tmpl.FailureCount = 0
	tmpl.SkipNextRun = false
	if reachedRecurringEnd(cfg, tmpl.GeneratedCount) {
		tmpl.Status = model.PMRecurringTemplateStatusStopped
		tmpl.NextRunAt = nil
	} else if cfg.ScheduleType == model.PMRecurringScheduleTypeTime {
		ref := finishedAt
		if scheduledFor != nil {
			ref = *scheduledFor
		}
		nextRun, err := computeNextRecurringRun(ref, cfg)
		if err != nil {
			return nil, err
		}
		tmpl.NextRunAt = nextRun
	} else {
		tmpl.NextRunAt = nil
	}
	if err := s.recurringRepo.Update(ctx, tmpl); err != nil {
		return nil, err
	}
	return story, nil
}

func (s *PMRecurringTemplateService) seedToCreateRequest(ctx context.Context, seed model.PMRecurringTaskSeed, cfg model.PMRecurringTemplateConfig, tmpl *model.PMRecurringTemplate, scheduledFor *time.Time) (model.CreateTaskRequest, error) {
	ownerMemberIDs := []string{}
	if seed.OwnerMemberID != nil && strings.TrimSpace(*seed.OwnerMemberID) != "" {
		ownerMemberIDs = []string{strings.TrimSpace(*seed.OwnerMemberID)}
	}
	req := model.CreateTaskRequest{
		WorkspaceID:       tmpl.WorkspaceID,
		Name:              seed.Name,
		Description:       seed.Description,
		TaskType:          seed.TaskType,
		WorkflowID:        seed.WorkflowID,
		WorkflowStateID:   seed.WorkflowStateID,
		EpicID:            seed.EpicID,
		TeamID:            tmpl.TeamID,
		OwnerMemberIDs:    ownerMemberIDs,
		RequesterMemberID: seed.RequesterMemberID,
		Estimate:          seed.Estimate,
		OwnerIDs:          append([]string{}, seed.OwnerIDs...),
		FollowerIDs:       append([]string{}, seed.FollowerIDs...),
		LabelIDs:          append([]string{}, seed.LabelIDs...),
		ChecklistItems:    append([]model.CreateChecklistItemRequest{}, seed.ChecklistItems...),
		ExternalLinks:     append([]model.CreateExternalLinkRequest{}, seed.ExternalLinks...),
	}
	if seed.Priority != nil {
		req.Priority = seed.Priority
	}
	if seed.Severity != nil {
		req.Severity = seed.Severity
	}

	if scheduledFor != nil {
		switch cfg.DueDateMode {
		case model.PMRecurringDueDateModeScheduled:
			deadline := normalizeRecurringDate(*scheduledFor)
			req.Deadline = &deadline
		case model.PMRecurringDueDateModeOffsetDays:
			offset := 0
			if cfg.DueOffsetDays != nil {
				offset = *cfg.DueOffsetDays
			}
			deadline := normalizeRecurringDate((*scheduledFor).AddDate(0, 0, offset))
			req.Deadline = &deadline
		}
	}

	if cfg.SprintAssignmentMode != model.PMRecurringSprintAssignmentNone && s.sprintRepo != nil {
		switch cfg.SprintAssignmentMode {
		case model.PMRecurringSprintAssignmentCurrent:
			sprint, err := s.sprintRepo.GetCurrentSprint(ctx, tmpl.WorkspaceID, tmpl.TeamID)
			if err != nil {
				return model.CreateTaskRequest{}, err
			}
			if sprint != nil {
				req.SprintID = &sprint.ID
			}
		case model.PMRecurringSprintAssignmentDueDate:
			date := time.Now().UTC()
			if req.Deadline != nil {
				date = *req.Deadline
			}
			sprint, err := s.findSprintForDate(ctx, tmpl.WorkspaceID, tmpl.TeamID, date)
			if err != nil {
				return model.CreateTaskRequest{}, err
			}
			if sprint != nil {
				req.SprintID = &sprint.ID
			}
		}
	}

	return req, nil
}

func (s *PMRecurringTemplateService) findSprintForDate(ctx context.Context, workspaceID string, teamID *string, date time.Time) (*model.PMSprint, error) {
	sprints, err := s.sprintRepo.List(ctx, workspaceID, model.PMSprintListFilters{TeamID: teamID})
	if err != nil {
		return nil, err
	}
	target := normalizeRecurringDate(date)
	for i := range sprints {
		if sprints[i].Archived || sprints[i].StartDate == nil || sprints[i].EndDate == nil {
			continue
		}
		start := normalizeRecurringDate(*sprints[i].StartDate)
		end := normalizeRecurringDate(*sprints[i].EndDate)
		if !target.Before(start) && !target.After(end) {
			return &sprints[i], nil
		}
	}
	return nil, nil
}

func (s *PMRecurringTemplateService) updateStatus(ctx context.Context, id, actorID, status string, clearNext bool) (*model.RecurringTemplateDetail, error) {
	tmpl, err := s.recurringRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tmpl == nil {
		return nil, fmt.Errorf("recurring template not found")
	}
	if err := s.requireCanEdit(ctx, tmpl.WorkspaceID, actorID); err != nil {
		return nil, err
	}
	tmpl.Status = status
	if clearNext {
		tmpl.NextRunAt = nil
	}
	if status == model.PMRecurringTemplateStatusActive {
		cfg, err := parseRecurringConfig(tmpl.Config)
		if err != nil {
			return nil, err
		}
		if cfg.ScheduleType == model.PMRecurringScheduleTypeTime && !reachedRecurringEnd(cfg, tmpl.GeneratedCount) {
			ref := time.Now().UTC()
			if tmpl.LastRunAt != nil {
				ref = *tmpl.LastRunAt
			}
			nextRun, err := computeNextRecurringRun(ref, cfg)
			if err != nil {
				return nil, err
			}
			tmpl.NextRunAt = nextRun
		}
	}
	tmpl.UpdatedByID = optionalString(actorID)
	if err := s.recurringRepo.Update(ctx, tmpl); err != nil {
		return nil, err
	}
	detail, err := s.buildDetail(ctx, tmpl, true)
	if err != nil {
		return nil, err
	}
	return &detail, nil
}

func (s *PMRecurringTemplateService) recordSkippedRun(ctx context.Context, tmpl *model.PMRecurringTemplate, cfg model.PMRecurringTemplateConfig, now time.Time) error {
	occurrenceNumber := tmpl.GeneratedCount + 1
	run := &model.PMRecurringRun{
		WorkspaceID:      tmpl.WorkspaceID,
		TemplateID:       tmpl.ID,
		OccurrenceNumber: occurrenceNumber,
		TriggerType:      model.PMRecurringRunTriggerSchedule,
		ScheduledFor:     tmpl.NextRunAt,
		Status:           model.PMRecurringRunStatusSkipped,
		DedupeKey:        recurringRunDedupeKey(tmpl.ID, "skip", occurrenceNumber),
	}
	run.StartedAt = &now
	run.FinishedAt = &now
	if err := s.recurringRepo.CreateRun(ctx, run); err != nil {
		return err
	}
	tmpl.SkipNextRun = false
	tmpl.LastRunAt = &now
	nextRun, err := computeNextRecurringRun(now, cfg)
	if err != nil {
		return err
	}
	tmpl.NextRunAt = nextRun
	tmpl.UpdatedByID = optionalString(derefString(tmpl.CreatedByID))
	return s.recurringRepo.Update(ctx, tmpl)
}

func (s *PMRecurringTemplateService) recordCompletionSkip(ctx context.Context, tmpl *model.PMRecurringTemplate, now time.Time) error {
	run := &model.PMRecurringRun{
		WorkspaceID:      tmpl.WorkspaceID,
		TemplateID:       tmpl.ID,
		OccurrenceNumber: tmpl.GeneratedCount + 1,
		TriggerType:      model.PMRecurringRunTriggerCompletion,
		Status:           model.PMRecurringRunStatusSkipped,
		DedupeKey:        recurringRunDedupeKey(tmpl.ID, "completion-skip", tmpl.GeneratedCount+1),
	}
	run.StartedAt = &now
	run.FinishedAt = &now
	if err := s.recurringRepo.CreateRun(ctx, run); err != nil {
		return err
	}
	tmpl.SkipNextRun = false
	tmpl.Status = model.PMRecurringTemplateStatusPaused
	tmpl.LastRunAt = &now
	tmpl.NextRunAt = nil
	return s.recurringRepo.Update(ctx, tmpl)
}

func parseRecurringConfig(raw json.RawMessage) (model.PMRecurringTemplateConfig, error) {
	cfg := model.PMRecurringTemplateConfig{Interval: 1, DueDateMode: model.PMRecurringDueDateModeNone, SprintAssignmentMode: model.PMRecurringSprintAssignmentNone}
	if len(raw) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return model.PMRecurringTemplateConfig{}, fmt.Errorf("unmarshal recurring config: %w", err)
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 1
	}
	if cfg.DueDateMode == "" {
		cfg.DueDateMode = model.PMRecurringDueDateModeNone
	}
	if cfg.SprintAssignmentMode == "" {
		cfg.SprintAssignmentMode = model.PMRecurringSprintAssignmentNone
	}
	return cfg, nil
}

func parseRecurringSeed(raw json.RawMessage) (model.PMRecurringTaskSeed, error) {
	var seed model.PMRecurringTaskSeed
	if len(raw) == 0 {
		return seed, nil
	}
	if err := json.Unmarshal(raw, &seed); err != nil {
		return model.PMRecurringTaskSeed{}, fmt.Errorf("unmarshal recurring seed: %w", err)
	}
	return seed, nil
}

func normalizeRecurringConfig(cfg model.PMRecurringTemplateConfig) (model.PMRecurringTemplateConfig, error) {
	if cfg.Interval <= 0 {
		cfg.Interval = 1
	}
	switch cfg.ScheduleType {
	case model.PMRecurringScheduleTypeTime:
		switch cfg.Frequency {
		case model.PMRecurringFrequencyDaily, model.PMRecurringFrequencyWeekly, model.PMRecurringFrequencyMonthly, model.PMRecurringFrequencyYearly:
		default:
			return cfg, fmt.Errorf("invalid recurring frequency")
		}
	case model.PMRecurringScheduleTypeCompletion:
		switch cfg.CompletionEvent {
		case model.PMRecurringCompletionEventCompleted, model.PMRecurringCompletionEventDoneState:
		default:
			return cfg, fmt.Errorf("invalid completion_event")
		}
	default:
		return cfg, fmt.Errorf("invalid schedule_type")
	}

	switch cfg.DueDateMode {
	case "", model.PMRecurringDueDateModeNone:
		cfg.DueDateMode = model.PMRecurringDueDateModeNone
	case model.PMRecurringDueDateModeScheduled, model.PMRecurringDueDateModeOffsetDays:
	default:
		return cfg, fmt.Errorf("invalid due_date_mode")
	}
	if cfg.DueDateMode == model.PMRecurringDueDateModeOffsetDays {
		if cfg.DueOffsetDays == nil {
			return cfg, fmt.Errorf("due_offset_days is required")
		}
	}
	switch cfg.SprintAssignmentMode {
	case "", model.PMRecurringSprintAssignmentNone:
		cfg.SprintAssignmentMode = model.PMRecurringSprintAssignmentNone
	case model.PMRecurringSprintAssignmentCurrent, model.PMRecurringSprintAssignmentDueDate:
	default:
		return cfg, fmt.Errorf("invalid sprint_assignment_mode")
	}
	if cfg.StartsOn != nil {
		start := normalizeRecurringDate(*cfg.StartsOn)
		cfg.StartsOn = &start
	}
	if cfg.EndsOn != nil {
		end := normalizeRecurringDate(*cfg.EndsOn)
		cfg.EndsOn = &end
	}
	if cfg.StartsOn != nil && cfg.EndsOn != nil && cfg.EndsOn.Before(*cfg.StartsOn) {
		return cfg, fmt.Errorf("ends_on must be on or after starts_on")
	}
	if cfg.EndsAfterOccurrences != nil && *cfg.EndsAfterOccurrences <= 0 {
		return cfg, fmt.Errorf("ends_after_occurrences must be greater than zero")
	}
	if cfg.DayOfMonth != nil && (*cfg.DayOfMonth < 1 || *cfg.DayOfMonth > 31) {
		return cfg, fmt.Errorf("day_of_month must be between 1 and 31")
	}
	for _, weekday := range cfg.Weekdays {
		if weekday < 0 || weekday > 6 {
			return cfg, fmt.Errorf("weekdays must use 0-6")
		}
	}
	return cfg, nil
}

func computeNextRecurringRun(reference time.Time, cfg model.PMRecurringTemplateConfig) (*time.Time, error) {
	if cfg.ScheduleType != model.PMRecurringScheduleTypeTime {
		return nil, nil
	}
	ref := normalizeRecurringDate(reference)
	if cfg.StartsOn != nil && ref.Before(normalizeRecurringDate(*cfg.StartsOn)) {
		ref = normalizeRecurringDate(*cfg.StartsOn).AddDate(0, 0, -1)
	}

	var next time.Time
	switch cfg.Frequency {
	case model.PMRecurringFrequencyDaily:
		next = ref.AddDate(0, 0, cfg.Interval)
	case model.PMRecurringFrequencyWeekly:
		next = nextWeeklyOccurrence(ref, cfg)
	case model.PMRecurringFrequencyMonthly:
		next = nextMonthlyOccurrence(ref, cfg)
	case model.PMRecurringFrequencyYearly:
		next = nextYearlyOccurrence(ref, cfg)
	default:
		return nil, fmt.Errorf("invalid recurring frequency")
	}
	next = normalizeRecurringDate(next)
	if cfg.EndsOn != nil && next.After(normalizeRecurringDate(*cfg.EndsOn)) {
		return nil, nil
	}
	return &next, nil
}

func nextWeeklyOccurrence(reference time.Time, cfg model.PMRecurringTemplateConfig) time.Time {
	if len(cfg.Weekdays) == 0 {
		return reference.AddDate(0, 0, 7*cfg.Interval)
	}

	// Use the first weekday (single-select in UI).
	targetWeekday := time.Weekday(cfg.Weekdays[0])

	// Jump forward by interval weeks from reference.
	candidate := reference.AddDate(0, 0, 7*cfg.Interval)

	// Find the target weekday in the landing week.
	// First, rewind to the Monday of that week.
	weekStart := candidate
	for weekStart.Weekday() != time.Monday {
		weekStart = weekStart.AddDate(0, 0, -1)
	}

	// Advance to the target weekday within that week.
	target := weekStart
	for target.Weekday() != targetWeekday {
		target = target.AddDate(0, 0, 1)
	}

	// If target landed before or on reference (same week edge case), jump another interval.
	if !target.After(reference) {
		return nextWeeklyOccurrence(target, cfg)
	}

	return target
}

func nextMonthlyOccurrence(reference time.Time, cfg model.PMRecurringTemplateConfig) time.Time {
	day := reference.Day()
	if cfg.DayOfMonth != nil {
		day = *cfg.DayOfMonth
	}
	firstOfTarget := time.Date(reference.Year(), reference.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, cfg.Interval, 0)
	lastDay := daysInMonth(firstOfTarget.Year(), firstOfTarget.Month())
	if day > lastDay {
		day = lastDay
	}
	return time.Date(firstOfTarget.Year(), firstOfTarget.Month(), day, 0, 0, 0, 0, time.UTC)
}

func nextYearlyOccurrence(reference time.Time, cfg model.PMRecurringTemplateConfig) time.Time {
	month := reference.Month()
	day := reference.Day()
	if cfg.StartsOn != nil {
		month = cfg.StartsOn.Month()
		day = cfg.StartsOn.Day()
	}
	year := reference.Year() + cfg.Interval
	lastDay := daysInMonth(year, month)
	if day > lastDay {
		day = lastDay
	}
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func recurringRuleSummary(cfg model.PMRecurringTemplateConfig) string {
	switch cfg.ScheduleType {
	case model.PMRecurringScheduleTypeCompletion:
		if cfg.CompletionEvent == model.PMRecurringCompletionEventDoneState {
			return "Generate next when moved to done"
		}
		return "Generate next after completion"
	case model.PMRecurringScheduleTypeTime:
		switch cfg.Frequency {
		case model.PMRecurringFrequencyDaily:
			if cfg.Interval <= 1 {
				return "Daily"
			}
			return fmt.Sprintf("Every %d days", cfg.Interval)
		case model.PMRecurringFrequencyWeekly:
			dayName := ""
			if len(cfg.Weekdays) > 0 {
				dayName = time.Weekday(cfg.Weekdays[0]).String()
			}
			if cfg.Interval <= 1 {
				if dayName != "" {
					return "Every week on " + dayName
				}
				return "Weekly"
			}
			if dayName != "" {
				return fmt.Sprintf("Every %d weeks on %s", cfg.Interval, dayName)
			}
			return fmt.Sprintf("Every %d weeks", cfg.Interval)
		case model.PMRecurringFrequencyMonthly:
			if cfg.DayOfMonth != nil {
				if cfg.Interval <= 1 {
					return fmt.Sprintf("Monthly on day %d", *cfg.DayOfMonth)
				}
				return fmt.Sprintf("Every %d months on day %d", cfg.Interval, *cfg.DayOfMonth)
			}
			return "Monthly"
		case model.PMRecurringFrequencyYearly:
			if cfg.Interval <= 1 {
				return "Yearly"
			}
			return fmt.Sprintf("Every %d years", cfg.Interval)
		}
	}
	return "Recurring"
}

func recurringRunDedupeKey(templateID, triggerType string, occurrenceNumber int) string {
	return fmt.Sprintf("%s:%s:%d", templateID, triggerType, occurrenceNumber)
}

func reachedRecurringEnd(cfg model.PMRecurringTemplateConfig, generatedCount int) bool {
	return cfg.EndsAfterOccurrences != nil && generatedCount >= *cfg.EndsAfterOccurrences
}

func normalizeRecurringDate(t time.Time) time.Time {
	utc := t.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func recurringIntPtr(value int) *int {
	return &value
}

func optionalString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(value)
	return &trimmed
}

func (s *PMRecurringTemplateService) storyMatchesCompletionEvent(ctx context.Context, story *model.PMTask, cfg model.PMRecurringTemplateConfig) (bool, error) {
	// If specific state IDs are configured, check if the story's current state matches any.
	if len(cfg.CompletionStateIDs) > 0 {
		for _, id := range cfg.CompletionStateIDs {
			if story.WorkflowStateID == id {
				return true, nil
			}
		}
		return false, nil
	}

	// Fallback to legacy completion event matching.
	switch cfg.CompletionEvent {
	case model.PMRecurringCompletionEventCompleted:
		return story.Completed, nil
	case model.PMRecurringCompletionEventDoneState:
		state, err := s.workflowRepo.GetStateByID(ctx, story.WorkflowStateID)
		if err != nil {
			return false, err
		}
		return state != nil && state.StateType == model.PMStateTypeDone, nil
	default:
		return false, nil
	}
}
