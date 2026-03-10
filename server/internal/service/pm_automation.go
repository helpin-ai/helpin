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
			s.handleEpicAutoStart(ctx, *auto, epicID, newState)
		}
	} else if newState.StateType == model.PMStateTypeDone {
		if auto, _ := s.automationRepo.GetByType(ctx, story.WorkspaceID, model.PMAutomationTypeEpicAutoComplete, nil); auto != nil && auto.Enabled {
			s.handleEpicAutoComplete(ctx, *auto, epicID, newState)
		}
	}
}

func (s *PMAutomationService) handleEpicAutoStart(ctx context.Context, auto model.PMAutomation, epicID string, newState *model.PMWorkflowState) {
	if newState.StateType != model.PMStateTypeStarted {
		return
	}
	if auto.ConfigStateID == nil || *auto.ConfigStateID == "" {
		return
	}

	epicWithStats, err := s.epicRepo.GetWithStats(ctx, epicID)
	if err != nil || epicWithStats == nil {
		return
	}
	epic := epicWithStats.Epic

	// Only auto-start if epic hasn't started yet
	if epic.Started {
		return
	}

	// Resolve the target epic state to verify it exists
	// (ConfigStateID points to a pm_epic_workflow_states row)
	now := time.Now().UTC()
	epic.EpicStateID = auto.ConfigStateID
	epic.Started = true
	epic.StartedAt = &now

	if err := s.epicRepo.Update(ctx, &epic); err != nil {
		return
	}

	if err := s.activityService.Log(ctx, epic.WorkspaceID, "epic", epic.ID, nil, "auto-started by automation", nil, nil, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for epic auto-start", "error", err, "epic_id", epic.ID, "workspace_id", epic.WorkspaceID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "epic", EntityID: epic.ID, WorkspaceID: epic.WorkspaceID})
}

func (s *PMAutomationService) handleEpicAutoComplete(ctx context.Context, auto model.PMAutomation, epicID string, newState *model.PMWorkflowState) {
	if newState.StateType != model.PMStateTypeDone {
		return
	}
	if auto.ConfigStateID == nil || *auto.ConfigStateID == "" {
		return
	}

	epicWithStats, err := s.epicRepo.GetWithStats(ctx, epicID)
	if err != nil || epicWithStats == nil {
		return
	}
	epic := epicWithStats.Epic
	stats := epicWithStats.Stats

	// Only auto-complete if ALL stories are done
	if stats.StoryCount == 0 || stats.DoneStoryCount != stats.StoryCount {
		return
	}

	// Already completed
	if epic.Completed {
		return
	}

	now := time.Now().UTC()
	epic.EpicStateID = auto.ConfigStateID
	epic.Completed = true
	epic.CompletedAt = &now
	epic.Started = true
	if epic.StartedAt == nil {
		epic.StartedAt = &now
	}

	if err := s.epicRepo.Update(ctx, &epic); err != nil {
		return
	}

	if err := s.activityService.Log(ctx, epic.WorkspaceID, "epic", epic.ID, nil, "auto-completed by automation", nil, nil, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for epic auto-complete", "error", err, "epic_id", epic.ID, "workspace_id", epic.WorkspaceID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "epic", EntityID: epic.ID, WorkspaceID: epic.WorkspaceID})
}

// RunSprintAutomations runs background sprint automations.
// Called periodically (e.g., hourly). Looks at sprints that ended recently.
func (s *PMAutomationService) RunSprintAutomations(ctx context.Context) {
	s.logger.InfoContext(ctx, "running sprint automations")
	s.runSprintAutoCreate(ctx)
	s.runSprintMoveUnfinished(ctx)
	s.logger.InfoContext(ctx, "sprint automations completed")
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
			continue
		}

		now := toDay(time.Now().UTC())
		futureCount := 0
		var latestEnd time.Time
		for _, sp := range sprints {
			if sp.Archived {
				continue
			}
			if sp.EndDate == nil {
				continue
			}
			spEnd := toDay(*sp.EndDate)
			if spEnd.After(now) || spEnd.Equal(now) {
				futureCount++
			}
			if spEnd.After(latestEnd) {
				latestEnd = spEnd
			}
		}

		needed := targetCount - futureCount
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
				break
			}
			s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "sprint", EntityID: sprint.ID, WorkspaceID: cfg.WorkspaceID})
			nextStart = endDate.AddDate(0, 0, 1)
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

		// Find sprints that just ended (within catch window)
		filters := model.PMSprintListFilters{TeamID: &teamID}
		sprints, err := s.sprintRepo.List(ctx, cfg.WorkspaceID, filters)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to list sprints for move-unfinished", "error", err, "workspace_id", cfg.WorkspaceID, "team_id", teamID)
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
			s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "story", EntityID: story.ID, WorkspaceID: cfg.WorkspaceID})
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
