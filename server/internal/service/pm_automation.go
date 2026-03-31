package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// PMAutomationService handles automation CRUD and trigger logic.
type PMAutomationService struct {
	automationRepo  *repository.PMAutomationRepository
	epicRepo        *repository.PMEpicRepository
	storyRepo       *repository.PMStoryRepository
	sprintRepo      *repository.PMSprintRepository
	workflowRepo    *repository.PMWorkflowRepository
	activityService *PMActivityService
	wsPublisher     *websocket.Publisher
	logger          *slog.Logger
	healthObserver  AutomationHealthObserver
}

// NewPMAutomationService creates a new PMAutomationService.
func NewPMAutomationService(
	automationRepo *repository.PMAutomationRepository,
	epicRepo *repository.PMEpicRepository,
	storyRepo *repository.PMStoryRepository,
	sprintRepo *repository.PMSprintRepository,
	workflowRepo *repository.PMWorkflowRepository,
	activityService *PMActivityService,
	wsPublisher *websocket.Publisher,
) *PMAutomationService {
	return &PMAutomationService{
		automationRepo:  automationRepo,
		epicRepo:        epicRepo,
		storyRepo:       storyRepo,
		sprintRepo:      sprintRepo,
		workflowRepo:    workflowRepo,
		activityService: activityService,
		wsPublisher:     wsPublisher,
		logger:          slog.Default().With("service", "pm_automation"),
	}
}

func (s *PMAutomationService) SetHealthObserver(observer AutomationHealthObserver) *PMAutomationService {
	s.healthObserver = observer
	return s
}

// SeedWorkspaceDefaults enables the default workspace-level epic automations for
// newly created workspaces, targeting the seeded started/done epic states.
func (s *PMAutomationService) SeedWorkspaceDefaults(ctx context.Context, workspaceID, actorID string) error {
	if workspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if s.automationRepo == nil || s.workflowRepo == nil {
		return fmt.Errorf("pm automation defaults require automation and workflow repositories")
	}

	epicStates, err := s.workflowRepo.ListEpicStates(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list epic workflow states: %w", err)
	}

	var startedStateID string
	var doneStateID string
	for _, state := range epicStates {
		if startedStateID == "" && state.StateType == model.PMStateTypeStarted {
			startedStateID = state.ID
		}
		if doneStateID == "" && state.StateType == model.PMStateTypeDone {
			doneStateID = state.ID
		}
	}

	if startedStateID == "" {
		return fmt.Errorf("started epic workflow state is required")
	}
	if doneStateID == "" {
		return fmt.Errorf("done epic workflow state is required")
	}

	defaults := []*model.PMAutomation{
		{
			WorkspaceID:    workspaceID,
			AutomationType: model.PMAutomationTypeEpicAutoStart,
			Enabled:        true,
			ConfigStateID:  &startedStateID,
		},
		{
			WorkspaceID:    workspaceID,
			AutomationType: model.PMAutomationTypeEpicAutoComplete,
			Enabled:        true,
			ConfigStateID:  &doneStateID,
		},
	}

	for _, automation := range defaults {
		if err := s.automationRepo.Upsert(ctx, automation); err != nil {
			return fmt.Errorf("seed %s: %w", automation.AutomationType, err)
		}
	}

	s.logger.InfoContext(ctx, "seeded default epic automations", "workspace_id", workspaceID)
	return nil
}

// List returns all automations for a workspace.
func (s *PMAutomationService) List(ctx context.Context, workspaceID string) ([]model.PMAutomation, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.automationRepo.ListByWorkspace(ctx, workspaceID)
}

// Upsert creates or updates an automation.
func (s *PMAutomationService) Upsert(ctx context.Context, req model.UpsertAutomationRequest) (*model.PMAutomation, error) {
	if req.WorkspaceID == "" || req.AutomationType == "" {
		return nil, fmt.Errorf("workspace_id and automation_type are required")
	}
	if !isValidAutomationType(req.AutomationType) {
		return nil, fmt.Errorf("invalid automation_type")
	}

	automation := &model.PMAutomation{
		WorkspaceID:    req.WorkspaceID,
		AutomationType: req.AutomationType,
		Enabled:        req.Enabled,
		TeamID:         req.TeamID,
		ConfigStateID:  req.ConfigStateID,
		ConfigInt:      req.ConfigInt,
		ConfigInt2:     req.ConfigInt2,
		ConfigInt3:     req.ConfigInt3,
	}
	if err := s.automationRepo.Upsert(ctx, automation); err != nil {
		return nil, err
	}
	s.logger.InfoContext(ctx, "automation upserted", "automation_type", req.AutomationType, "workspace_id", req.WorkspaceID, "enabled", req.Enabled)
	return automation, nil
}

// Delete removes an automation.
func (s *PMAutomationService) Delete(ctx context.Context, req model.DeleteAutomationRequest) error {
	if req.WorkspaceID == "" || req.AutomationType == "" {
		return fmt.Errorf("workspace_id and automation_type are required")
	}
	if err := s.automationRepo.Delete(ctx, req.WorkspaceID, req.AutomationType, req.TeamID); err != nil {
		return err
	}
	s.logger.InfoContext(ctx, "automation deleted", "automation_type", req.AutomationType, "workspace_id", req.WorkspaceID)
	return nil
}

// OnStoryStateChange is called after a story's workflow state changes.
// It evaluates epic automations (auto-start, auto-complete).
func (s *PMAutomationService) OnStoryStateChange(ctx context.Context, story *model.PMStory, newStateID string) {
	if story.EpicID == nil || *story.EpicID == "" {
		return
	}

	epicID := *story.EpicID

	// Resolve the new state's type
	newState, err := s.workflowRepo.GetStateByID(ctx, newStateID)
	if err != nil || newState == nil {
		return
	}

	// Only check epic automations relevant to the new state type
	if newState.StateType == model.PMStateTypeStarted {
		if auto, _ := s.automationRepo.GetByType(ctx, story.WorkspaceID, model.PMAutomationTypeEpicAutoStart, nil); auto != nil && auto.Enabled {
			mutated, err := s.handleEpicAutoStart(ctx, *auto, epicID, newState)
			if err != nil {
				s.observeFailure(ctx, story.WorkspaceID, "pm.epic_auto_start", model.AutomationScopeWorkspace, story.WorkspaceID, err, model.JSONB{
					"epic_id":   epicID,
					"story_id":  story.ID,
					"state_id":  newState.ID,
					"mutated":   false,
					"triggered": true,
				})
			} else {
				s.observeSuccess(ctx, story.WorkspaceID, "pm.epic_auto_start", model.AutomationScopeWorkspace, story.WorkspaceID, model.JSONB{
					"epic_id":   epicID,
					"story_id":  story.ID,
					"state_id":  newState.ID,
					"mutated":   mutated,
					"triggered": true,
				})
			}
		}
	} else if newState.StateType == model.PMStateTypeDone {
		if auto, _ := s.automationRepo.GetByType(ctx, story.WorkspaceID, model.PMAutomationTypeEpicAutoComplete, nil); auto != nil && auto.Enabled {
			mutated, err := s.handleEpicAutoComplete(ctx, *auto, epicID, newState)
			if err != nil {
				s.observeFailure(ctx, story.WorkspaceID, "pm.epic_auto_complete", model.AutomationScopeWorkspace, story.WorkspaceID, err, model.JSONB{
					"epic_id":   epicID,
					"story_id":  story.ID,
					"state_id":  newState.ID,
					"mutated":   false,
					"triggered": true,
				})
			} else {
				s.observeSuccess(ctx, story.WorkspaceID, "pm.epic_auto_complete", model.AutomationScopeWorkspace, story.WorkspaceID, model.JSONB{
					"epic_id":   epicID,
					"story_id":  story.ID,
					"state_id":  newState.ID,
					"mutated":   mutated,
					"triggered": true,
				})
			}
		}
	}
}

// HandleEpicAutoStart checks whether the given epic should be auto-started and updates it.
// Returns true if the epic was mutated.
func (s *PMAutomationService) HandleEpicAutoStart(ctx context.Context, workspaceID, epicID, targetStateID string) (bool, error) {
	if targetStateID == "" {
		return false, nil
	}

	epicWithStats, err := s.epicRepo.GetWithStats(ctx, epicID)
	if err != nil || epicWithStats == nil {
		if err != nil {
			return false, err
		}
		return false, nil
	}
	epic := epicWithStats.Epic

	if epic.Started {
		return false, nil
	}

	now := time.Now().UTC()
	epic.EpicStateID = &targetStateID
	epic.Started = true
	epic.StartedAt = &now

	if err := s.epicRepo.Update(ctx, &epic); err != nil {
		return false, err
	}

	if err := s.activityService.Log(ctx, epic.WorkspaceID, "epic", epic.ID, nil, "auto-started by automation", nil, nil, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for epic auto-start", "error", err, "epic_id", epic.ID, "workspace_id", epic.WorkspaceID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "epic", EntityID: epic.ID, WorkspaceID: epic.WorkspaceID})
	return true, nil
}

// HandleEpicAutoComplete checks whether the given epic should be auto-completed and updates it.
// Returns true if the epic was mutated.
func (s *PMAutomationService) HandleEpicAutoComplete(ctx context.Context, workspaceID, epicID, targetStateID string) (bool, error) {
	if targetStateID == "" {
		return false, nil
	}

	epicWithStats, err := s.epicRepo.GetWithStats(ctx, epicID)
	if err != nil || epicWithStats == nil {
		if err != nil {
			return false, err
		}
		return false, nil
	}
	epic := epicWithStats.Epic
	stats := epicWithStats.Stats

	if stats.StoryCount == 0 || stats.DoneStoryCount != stats.StoryCount {
		return false, nil
	}
	if epic.Completed {
		return false, nil
	}

	now := time.Now().UTC()
	epic.EpicStateID = &targetStateID
	epic.Completed = true
	epic.CompletedAt = &now
	epic.Started = true
	if epic.StartedAt == nil {
		epic.StartedAt = &now
	}

	if err := s.epicRepo.Update(ctx, &epic); err != nil {
		return false, err
	}

	if err := s.activityService.Log(ctx, epic.WorkspaceID, "epic", epic.ID, nil, "auto-completed by automation", nil, nil, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for epic auto-complete", "error", err, "epic_id", epic.ID, "workspace_id", epic.WorkspaceID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "epic", EntityID: epic.ID, WorkspaceID: epic.WorkspaceID})
	return true, nil
}

func (s *PMAutomationService) handleEpicAutoStart(ctx context.Context, auto model.PMAutomation, epicID string, newState *model.PMWorkflowState) (bool, error) {
	if newState.StateType != model.PMStateTypeStarted {
		return false, nil
	}
	if auto.ConfigStateID == nil || *auto.ConfigStateID == "" {
		return false, nil
	}
	return s.HandleEpicAutoStart(ctx, auto.WorkspaceID, epicID, *auto.ConfigStateID)
}

func (s *PMAutomationService) handleEpicAutoComplete(ctx context.Context, auto model.PMAutomation, epicID string, newState *model.PMWorkflowState) (bool, error) {
	if newState.StateType != model.PMStateTypeDone {
		return false, nil
	}
	if auto.ConfigStateID == nil || *auto.ConfigStateID == "" {
		return false, nil
	}
	return s.HandleEpicAutoComplete(ctx, auto.WorkspaceID, epicID, *auto.ConfigStateID)
}

// RunSprintAutomations runs background sprint automations.
// Called periodically (e.g., hourly). Looks at sprints that ended recently.
func (s *PMAutomationService) RunSprintAutomations(ctx context.Context) {
	s.logger.InfoContext(ctx, "running sprint automations")
	s.runSprintAutoCreate(ctx)
	s.runSprintMoveUnfinished(ctx)
	s.logger.InfoContext(ctx, "sprint automations completed")
}

// RunSprintAutoCreate runs only the sprint auto-create logic.
func (s *PMAutomationService) RunSprintAutoCreate(ctx context.Context) {
	s.runSprintAutoCreate(ctx)
}

// RunSprintMoveUnfinished runs only the move-unfinished-stories logic.
func (s *PMAutomationService) RunSprintMoveUnfinished(ctx context.Context) {
	s.runSprintMoveUnfinished(ctx)
}

func (s *PMAutomationService) runSprintAutoCreate(ctx context.Context) {
	configs, err := s.automationRepo.ListEnabledByType(ctx, model.PMAutomationTypeSprintAutoCreate)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to list sprint auto-create configs", "error", err)
		return
	}

	for _, cfg := range configs {
		if cfg.TeamID == nil || cfg.ConfigInt == nil || cfg.ConfigInt2 == nil {
			continue
		}
		teamID := *cfg.TeamID
		metrics := model.JSONB{
			"evaluated":     true,
			"created_count": 0,
		}
		failed := false
		targetCount := *cfg.ConfigInt
		weekLength := *cfg.ConfigInt2
		startDay := 1 // Monday default
		if cfg.ConfigInt3 != nil {
			startDay = *cfg.ConfigInt3
		}
		if targetCount <= 0 || weekLength <= 0 {
			continue
		}

		// Count future (unstarted) sprints for this team
		filters := model.PMSprintListFilters{
			TeamID: &teamID,
		}
		sprints, err := s.sprintRepo.List(ctx, cfg.WorkspaceID, filters)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to list sprints for auto-create", "error", err, "workspace_id", cfg.WorkspaceID, "team_id", teamID)
			s.observeFailure(ctx, cfg.WorkspaceID, "pm.sprint_auto_create", model.AutomationScopeTeam, teamID, err, metrics)
			failed = true
			continue
		}

		now := toDay(time.Now().UTC())
		upcomingCount := 0
		var latestEnd time.Time
		for _, sp := range sprints {
			if sp.Archived {
				continue
			}
			if sp.EndDate == nil || sp.StartDate == nil {
				continue
			}
			spStart := toDay(*sp.StartDate)
			spEnd := toDay(*sp.EndDate)
			// Only count truly unstarted sprints (start date in the future)
			if spStart.After(now) {
				upcomingCount++
			}
			if spEnd.After(latestEnd) {
				latestEnd = spEnd
			}
		}

		needed := targetCount - upcomingCount
		if needed <= 0 {
			continue
		}

		// Start creating from the day after the latest end date
		nextStart := latestEnd.AddDate(0, 0, 1)
		if latestEnd.IsZero() {
			// No existing sprints, start from next occurrence of startDay
			nextStart = nextWeekday(now, time.Weekday(startDay))
		}

		for i := 0; i < needed; i++ {
			// Align to start day of week
			nextStart = nextWeekday(nextStart, time.Weekday(startDay))
			endDate := nextStart.AddDate(0, 0, weekLength*7-1)

			overlap, err := s.sprintRepo.HasDateOverlap(ctx, cfg.WorkspaceID, &teamID, nextStart, endDate, nil)
			if err != nil {
				s.logger.ErrorContext(ctx, "failed to check sprint date overlap", "error", err, "workspace_id", cfg.WorkspaceID, "team_id", teamID)
				s.observeFailure(ctx, cfg.WorkspaceID, "pm.sprint_auto_create", model.AutomationScopeTeam, teamID, err, metrics)
				failed = true
				break
			}
			if overlap {
				nextStart = endDate.AddDate(0, 0, 1)
				continue
			}

			_, startWeek := nextStart.ISOWeek()
			_, endWeek := endDate.ISOWeek()
			name := fmt.Sprintf("Week %d-%d, %d", startWeek, endWeek, nextStart.Year())

			sprint := &model.PMSprint{
				WorkspaceID: cfg.WorkspaceID,
				Name:        name,
				StartDate:   &nextStart,
				EndDate:     &endDate,
				TeamID:      &teamID,
			}
			if err := s.sprintRepo.Create(ctx, sprint); err != nil {
				s.logger.ErrorContext(ctx, "failed to auto-create sprint", "error", err, "workspace_id", cfg.WorkspaceID, "team_id", teamID)
				s.observeFailure(ctx, cfg.WorkspaceID, "pm.sprint_auto_create", model.AutomationScopeTeam, teamID, err, metrics)
				failed = true
				break
			}
			metrics["created_count"] = metrics["created_count"].(int) + 1
			s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "sprint", EntityID: sprint.ID, WorkspaceID: cfg.WorkspaceID})
			nextStart = endDate.AddDate(0, 0, 1)
		}
		if !failed {
			s.observeSuccess(ctx, cfg.WorkspaceID, "pm.sprint_auto_create", model.AutomationScopeTeam, teamID, metrics)
		}
	}
}

func (s *PMAutomationService) runSprintMoveUnfinished(ctx context.Context) {
	configs, err := s.automationRepo.ListEnabledByType(ctx, model.PMAutomationTypeSprintMoveUnfinished)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to list sprint move-unfinished configs", "error", err)
		return
	}

	today := toDay(time.Now().UTC())
	twoDaysAgo := today.AddDate(0, 0, -2)

	for _, cfg := range configs {
		if cfg.TeamID == nil {
			continue
		}
		teamID := *cfg.TeamID
		metrics := model.JSONB{
			"evaluated":     true,
			"stories_moved": 0,
		}
		failed := false

		// Find sprints that just ended (within catch window)
		filters := model.PMSprintListFilters{TeamID: &teamID}
		sprints, err := s.sprintRepo.List(ctx, cfg.WorkspaceID, filters)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to list sprints for move-unfinished", "error", err, "workspace_id", cfg.WorkspaceID, "team_id", teamID)
			s.observeFailure(ctx, cfg.WorkspaceID, "pm.sprint_move_unfinished", model.AutomationScopeTeam, teamID, err, metrics)
			failed = true
			continue
		}

		var endedSprint *model.PMSprint
		var nextSprint *model.PMSprint
		for i := range sprints {
			sp := &sprints[i]
			if sp.Archived || sp.EndDate == nil {
				continue
			}
			spEnd := toDay(*sp.EndDate)
			if spEnd.Before(today) && (spEnd.After(twoDaysAgo) || spEnd.Equal(twoDaysAgo)) {
				if endedSprint == nil || endedSprint.EndDate == nil || spEnd.After(toDay(*endedSprint.EndDate)) {
					endedSprint = sp
				}
			}
		}

		if endedSprint == nil {
			continue
		}

		// Find the next sprint (earliest start_date after endedSprint.EndDate)
		if endedSprint.EndDate == nil {
			continue
		}
		endedEnd := toDay(*endedSprint.EndDate)
		for i := range sprints {
			sp := &sprints[i]
			if sp.Archived || sp.ID == endedSprint.ID || sp.StartDate == nil {
				continue
			}
			spStart := toDay(*sp.StartDate)
			if spStart.After(endedEnd) {
				if nextSprint == nil || nextSprint.StartDate == nil || spStart.Before(toDay(*nextSprint.StartDate)) {
					nextSprint = sp
				}
			}
		}

		if nextSprint == nil {
			continue
		}

		// Move non-done stories from ended sprint to next sprint
		stories, err := s.sprintRepo.ListStories(ctx, endedSprint.ID)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to list stories for move-unfinished", "error", err, "sprint_id", endedSprint.ID, "workspace_id", cfg.WorkspaceID)
			s.observeFailure(ctx, cfg.WorkspaceID, "pm.sprint_move_unfinished", model.AutomationScopeTeam, teamID, err, metrics)
			failed = true
			continue
		}

		for _, story := range stories {
			state, err := s.workflowRepo.GetStateByID(ctx, story.WorkflowStateID)
			if err != nil || state == nil {
				continue
			}
			if state.StateType == model.PMStateTypeDone {
				continue
			}
			if err := s.storyRepo.UpdateSprintID(ctx, story.ID, &nextSprint.ID); err != nil {
				s.logger.ErrorContext(ctx, "failed to move unfinished story to next sprint", "error", err, "story_id", story.ID, "next_sprint_id", nextSprint.ID)
				continue
			}
			metrics["stories_moved"] = metrics["stories_moved"].(int) + 1
			s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "story", EntityID: story.ID, WorkspaceID: cfg.WorkspaceID})
		}
		if !failed {
			s.observeSuccess(ctx, cfg.WorkspaceID, "pm.sprint_move_unfinished", model.AutomationScopeTeam, teamID, metrics)
		}
	}
}

// nextWeekday returns d if it's already the target weekday, otherwise the next occurrence.
func nextWeekday(d time.Time, target time.Weekday) time.Time {
	days := (int(target) - int(d.Weekday()) + 7) % 7
	if days == 0 {
		return d
	}
	return d.AddDate(0, 0, days)
}

func isValidAutomationType(t string) bool {
	switch t {
	case model.PMAutomationTypeEpicAutoStart,
		model.PMAutomationTypeEpicAutoComplete,
		model.PMAutomationTypeSprintAutoCreate,
		model.PMAutomationTypeSprintMoveUnfinished:
		return true
	default:
		return false
	}
}

func (s *PMAutomationService) observeSuccess(ctx context.Context, workspaceID, catalogID, scopeType, scopeID string, metrics model.JSONB) {
	if s == nil || s.healthObserver == nil {
		return
	}
	if err := s.healthObserver.ObserveSuccess(ctx, workspaceID, catalogID, scopeType, scopeID, metrics); err != nil {
		s.logger.WarnContext(ctx, "failed to record automation health success", "error", err, "catalog_id", catalogID, "workspace_id", workspaceID)
	}
}

func (s *PMAutomationService) observeFailure(ctx context.Context, workspaceID, catalogID, scopeType, scopeID string, cause error, metrics model.JSONB) {
	if s == nil || s.healthObserver == nil || cause == nil {
		return
	}
	if err := s.healthObserver.ObserveFailure(ctx, workspaceID, catalogID, scopeType, scopeID, cause.Error(), metrics); err != nil {
		s.logger.WarnContext(ctx, "failed to record automation health failure", "error", err, "catalog_id", catalogID, "workspace_id", workspaceID)
	}
}
