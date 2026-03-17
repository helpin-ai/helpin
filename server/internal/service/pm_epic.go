package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// PMEpicService contains epic business logic.
type PMEpicService struct {
	epicRepo            *repository.PMEpicRepository
	storyRepo           *repository.PMStoryRepository
	labelRepo           *repository.PMLabelRepository
	gitRepo             *repository.GitRepositoryRepository
	workspaceRepo       *repository.WorkspaceRepository
	activityService     *PMActivityService
	wsPublisher         *websocket.Publisher
	notificationService *NotificationService
	logger              *slog.Logger
}

// NewPMEpicService creates a new PMEpicService.
func NewPMEpicService(epicRepo *repository.PMEpicRepository, storyRepo *repository.PMStoryRepository, labelRepo *repository.PMLabelRepository, gitRepo *repository.GitRepositoryRepository, workspaceRepo *repository.WorkspaceRepository, activityService *PMActivityService, wsPublisher *websocket.Publisher, notificationService *NotificationService) *PMEpicService {
	return &PMEpicService{
		epicRepo:            epicRepo,
		storyRepo:           storyRepo,
		labelRepo:           labelRepo,
		gitRepo:             gitRepo,
		workspaceRepo:       workspaceRepo,
		activityService:     activityService,
		wsPublisher:         wsPublisher,
		notificationService: notificationService,
		logger:              slog.Default().With("service", "pm_epic"),
	}
}

// requireAdmin checks that the actor has owner or admin role.
func (s *PMEpicService) requireAdmin(ctx context.Context, workspaceID, actorID string) error {
	if workspaceID == "" || actorID == "" {
		return &model.ErrForbidden{Message: "workspace_id and user_id are required"}
	}
	role, err := s.workspaceRepo.GetMemberRole(ctx, workspaceID, actorID)
	if err != nil {
		return err
	}
	if role != model.RoleOwner && role != model.RoleAdmin {
		return &model.ErrForbidden{Message: "admin access or above required"}
	}
	return nil
}

// List returns epics and computed stats.
func (s *PMEpicService) List(ctx context.Context, workspaceID string, filters model.PMEpicListFilters) ([]model.EpicWithStats, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	filters.AccessibleTeamIDs = accessibleTeamIDs(ctx)
	epics, err := s.epicRepo.List(ctx, workspaceID, filters)
	if err != nil {
		return nil, err
	}

	result := make([]model.EpicWithStats, 0, len(epics))
	epicIDs := make([]string, 0, len(epics))
	for _, epic := range epics {
		withStats, err := s.epicRepo.GetWithStats(ctx, epic.ID)
		if err != nil {
			return nil, err
		}
		if withStats != nil {
			enrichEpicSuggestedHealth(withStats)
			result = append(result, *withStats)
			epicIDs = append(epicIDs, epic.ID)
		}
	}

	// Batch-load objectives for all epics
	objMap, err := s.epicRepo.ListObjectivesBatch(ctx, epicIDs)
	if err != nil {
		return nil, err
	}
	for i := range result {
		if objs, ok := objMap[result[i].Epic.ID]; ok {
			result[i].Objectives = objs
		}
	}

	return result, nil
}

// GetByID returns one epic with stats.
func (s *PMEpicService) GetByID(ctx context.Context, id string) (*model.EpicWithStats, error) {
	epic, err := s.epicRepo.GetWithStats(ctx, id)
	if err != nil {
		return nil, err
	}
	if epic == nil {
		return nil, fmt.Errorf("epic not found")
	}
	if err := requireTeamAccess(ctx, epic.Epic.TeamID); err != nil {
		return nil, fmt.Errorf("epic not found")
	}
	enrichEpicSuggestedHealth(epic)
	return epic, nil
}

// Create creates an epic.
func (s *PMEpicService) Create(ctx context.Context, req model.CreateEpicRequest, actorID string) (*model.EpicWithStats, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}
	if err := requireCanManage(ctx, req.TeamID); err != nil {
		return nil, err
	}
	health := model.PMEpicHealthNone
	if req.Health != nil && *req.Health != "" {
		health = *req.Health
	}
	if !isValidEpicHealth(health) {
		return nil, fmt.Errorf("invalid health value")
	}
	ownerMember, err := resolveWorkspaceMember(ctx, s.workspaceRepo, req.WorkspaceID, req.OwnerMemberID, req.OwnerID)
	if err != nil {
		return nil, err
	}

	epic := &model.PMEpic{
		WorkspaceID:          req.WorkspaceID,
		Name:                 strings.TrimSpace(req.Name),
		Description:          req.Description,
		EpicStateID:          req.EpicStateID,
		OwnerID:              memberUserIDPtr(ownerMember),
		OwnerMemberID:        memberIDPtr(ownerMember),
		TeamID:               req.TeamID,
		PlannedStartDate:     req.PlannedStartDate,
		Deadline:             req.Deadline,
		Color:                req.Color,
		Health:               health,
		HealthComment:        req.HealthComment,
		PlanningRepositoryID: req.PlanningRepositoryID,
	}
	if err := s.validatePlanningRepository(ctx, req.WorkspaceID, epic.PlanningRepositoryID); err != nil {
		return nil, err
	}
	if req.Position != nil {
		epic.Position = *req.Position
	}
	if actorID != "" {
		epic.CreatedBy = &actorID
	}

	if err := s.epicRepo.Create(ctx, epic); err != nil {
		s.logger.ErrorContext(ctx, "failed to create epic", "error", err, "workspace_id", req.WorkspaceID)
		return nil, err
	}
	if len(req.LabelIDs) > 0 {
		if err := validateLabelScope(ctx, s.labelRepo, req.WorkspaceID, req.LabelIDs, allowedTeamIDs(req.TeamID)); err != nil {
			return nil, err
		}
		if err := s.epicRepo.ReplaceLabels(ctx, epic.ID, req.LabelIDs); err != nil {
			return nil, err
		}
	}

	if err := s.syncProgress(ctx, epic.ID); err != nil {
		return nil, err
	}
	s.logger.InfoContext(ctx, "epic created", "epic_id", epic.ID, "workspace_id", epic.WorkspaceID, "name", epic.Name)
	if err := s.activityService.Log(ctx, epic.WorkspaceID, "epic", epic.ID, optionalActor(actorID), "created", nil, nil, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log epic created activity", "error", err, "epic_id", epic.ID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "epic", EntityID: epic.ID, WorkspaceID: epic.WorkspaceID, ActorID: actorID})

	if s.notificationService != nil {
		if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
			WorkspaceID: epic.WorkspaceID,
			ActorID:     actorID,
			EventType:   "epic.created",
			EntityType:  "epic",
			EntityID:    epic.ID,
			Title:       "created epic " + epic.Name,
			Category:    "activity",
			Priority:    "normal",
			EntitySnapshot: model.JSONB{
				"title": epic.Name,
			},
		}); err != nil {
			s.logger.ErrorContext(ctx, "failed to emit epic created notification", "error", err, "epic_id", epic.ID)
		}
	}

	return s.epicRepo.GetWithStats(ctx, epic.ID)
}

// Update updates an epic.
func (s *PMEpicService) Update(ctx context.Context, id string, req model.UpdateEpicRequest, actorID string) (*model.EpicWithStats, error) {
	current, err := s.epicRepo.GetWithStats(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("epic not found")
	}
	if err := requireCanManage(ctx, current.Epic.TeamID); err != nil {
		return nil, fmt.Errorf("epic not found")
	}
	epic := current.Epic

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		epic.Name = name
	}
	if req.Description != nil {
		epic.Description = req.Description
	}
	if req.EpicStateID != nil {
		epic.EpicStateID = req.EpicStateID
	}
	if req.OwnerID != nil || req.OwnerMemberID != nil {
		ownerMember, err := resolveWorkspaceMember(ctx, s.workspaceRepo, current.Epic.WorkspaceID, req.OwnerMemberID, req.OwnerID)
		if err != nil {
			return nil, err
		}
		epic.OwnerID = memberUserIDPtr(ownerMember)
		epic.OwnerMemberID = memberIDPtr(ownerMember)
	}
	if req.TeamID != nil {
		epic.TeamID = req.TeamID
	}
	if req.PlannedStartDate != nil {
		epic.PlannedStartDate = req.PlannedStartDate
	}
	if req.Deadline != nil {
		epic.Deadline = req.Deadline
	}
	if req.Position != nil {
		epic.Position = *req.Position
	}
	if req.Color != nil {
		epic.Color = req.Color
	}
	if req.Archived != nil {
		epic.Archived = *req.Archived
	}
	if req.Health != nil {
		if !isValidEpicHealth(*req.Health) {
			return nil, fmt.Errorf("invalid health value")
		}
		epic.Health = *req.Health
	}
	if req.HealthComment != nil {
		epic.HealthComment = req.HealthComment
	}
	if req.PlanningRepositoryID != nil {
		if err := s.validatePlanningRepository(ctx, epic.WorkspaceID, req.PlanningRepositoryID); err != nil {
			return nil, err
		}
		epic.PlanningRepositoryID = req.PlanningRepositoryID
	}

	if err := s.epicRepo.Update(ctx, &epic); err != nil {
		s.logger.ErrorContext(ctx, "failed to update epic", "error", err, "epic_id", id)
		return nil, err
	}
	if req.LabelIDs != nil {
		if err := validateLabelScope(ctx, s.labelRepo, epic.WorkspaceID, req.LabelIDs, allowedTeamIDs(epic.TeamID)); err != nil {
			return nil, err
		}
		if err := s.epicRepo.ReplaceLabels(ctx, epic.ID, req.LabelIDs); err != nil {
			return nil, err
		}
	}

	if err := s.syncProgress(ctx, epic.ID); err != nil {
		return nil, err
	}
	s.logger.InfoContext(ctx, "epic updated", "epic_id", epic.ID, "workspace_id", epic.WorkspaceID)
	if err := s.activityService.Log(ctx, epic.WorkspaceID, "epic", epic.ID, optionalActor(actorID), "updated", nil, nil, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log epic updated activity", "error", err, "epic_id", epic.ID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "epic", EntityID: epic.ID, WorkspaceID: epic.WorkspaceID, ActorID: actorID})

	if s.notificationService != nil {
		if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
			WorkspaceID: epic.WorkspaceID,
			ActorID:     actorID,
			EventType:   "epic.updated",
			EntityType:  "epic",
			EntityID:    epic.ID,
			Title:       "updated epic " + epic.Name,
			Category:    "activity",
			Priority:    "normal",
			EntitySnapshot: model.JSONB{
				"title": epic.Name,
			},
		}); err != nil {
			s.logger.ErrorContext(ctx, "failed to emit epic updated notification", "error", err, "epic_id", epic.ID)
		}
	}

	return s.epicRepo.GetWithStats(ctx, epic.ID)
}

func (s *PMEpicService) validatePlanningRepository(ctx context.Context, workspaceID string, repositoryID *string) error {
	if repositoryID == nil || strings.TrimSpace(*repositoryID) == "" {
		return nil
	}
	if s.gitRepo == nil {
		return fmt.Errorf("git repository catalog is not configured")
	}
	repo, err := s.gitRepo.GetByID(ctx, workspaceID, *repositoryID)
	if err != nil {
		return err
	}
	if repo == nil {
		return fmt.Errorf("planning repository not found")
	}
	if repo.Archived || !repo.Selected {
		return fmt.Errorf("planning repository is not available")
	}
	return nil
}

// Delete archives an epic.
func (s *PMEpicService) Delete(ctx context.Context, id string, actorID string) error {
	epic, err := s.epicRepo.GetWithStats(ctx, id)
	if err != nil {
		return err
	}
	if epic == nil {
		return fmt.Errorf("epic not found")
	}
	if err := s.requireAdmin(ctx, epic.Epic.WorkspaceID, actorID); err != nil {
		return err
	}
	if err := s.epicRepo.Delete(ctx, id); err != nil {
		s.logger.ErrorContext(ctx, "failed to delete epic", "error", err, "epic_id", id)
		return err
	}
	s.logger.InfoContext(ctx, "epic deleted", "epic_id", id, "workspace_id", epic.Epic.WorkspaceID)
	if err := s.activityService.Log(ctx, epic.Epic.WorkspaceID, "epic", id, optionalActor(actorID), "archived", nil, nil, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log epic deleted activity", "error", err, "epic_id", id)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "deleted", Entity: "epic", EntityID: id, WorkspaceID: epic.Epic.WorkspaceID, ActorID: actorID})

	if s.notificationService != nil {
		if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
			WorkspaceID: epic.Epic.WorkspaceID,
			ActorID:     actorID,
			EventType:   "epic.deleted",
			EntityType:  "epic",
			EntityID:    id,
			Title:       "archived epic " + epic.Epic.Name,
			Category:    "activity",
			Priority:    "normal",
			EntitySnapshot: model.JSONB{
				"title": epic.Epic.Name,
			},
		}); err != nil {
			s.logger.ErrorContext(ctx, "failed to emit epic deleted notification", "error", err, "epic_id", id)
		}
	}

	return nil
}

// UpdateHealth updates epic health fields.
func (s *PMEpicService) UpdateHealth(ctx context.Context, id string, req model.UpdateEpicHealthRequest, actorID string) error {
	if !isValidEpicHealth(req.Health) {
		return fmt.Errorf("invalid health value")
	}
	epic, err := s.epicRepo.GetWithStats(ctx, id)
	if err != nil {
		return err
	}
	if epic == nil {
		return fmt.Errorf("epic not found")
	}
	if err := requireCanManage(ctx, epic.Epic.TeamID); err != nil {
		return err
	}
	if err := s.epicRepo.UpdateHealth(ctx, id, req.Health, req.Comment); err != nil {
		s.logger.ErrorContext(ctx, "failed to update epic health", "error", err, "epic_id", id)
		return err
	}
	if err := s.activityService.Log(ctx, epic.Epic.WorkspaceID, "epic", id, optionalActor(actorID), "health_updated", stringPtr("health"), nil, &req.Health, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log epic health_updated activity", "error", err, "epic_id", id)
	}
	return nil
}

// AddLabel attaches a label to an epic.
func (s *PMEpicService) AddLabel(ctx context.Context, epicID, labelID, actorID string) error {
	epic, err := s.epicRepo.GetWithStats(ctx, epicID)
	if err != nil {
		return err
	}
	if epic == nil {
		return fmt.Errorf("epic not found")
	}
	if err := requireCanManage(ctx, epic.Epic.TeamID); err != nil {
		return err
	}
	if err := validateLabelScope(ctx, s.labelRepo, epic.Epic.WorkspaceID, []string{labelID}, allowedTeamIDs(epic.Epic.TeamID)); err != nil {
		return err
	}
	if err := s.epicRepo.AddLabel(ctx, epicID, labelID); err != nil {
		return err
	}
	if err := s.activityService.Log(ctx, epic.Epic.WorkspaceID, "epic", epicID, optionalActor(actorID), "label_added", stringPtr("label"), nil, &labelID, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log epic label_added activity", "error", err, "epic_id", epicID)
	}
	return nil
}

// RemoveLabel detaches a label from an epic.
func (s *PMEpicService) RemoveLabel(ctx context.Context, epicID, labelID, actorID string) error {
	epic, err := s.epicRepo.GetWithStats(ctx, epicID)
	if err != nil {
		return err
	}
	if epic == nil {
		return fmt.Errorf("epic not found")
	}
	if err := requireCanManage(ctx, epic.Epic.TeamID); err != nil {
		return err
	}
	if err := s.epicRepo.RemoveLabel(ctx, epicID, labelID); err != nil {
		return err
	}
	if err := s.activityService.Log(ctx, epic.Epic.WorkspaceID, "epic", epicID, optionalActor(actorID), "label_removed", stringPtr("label"), &labelID, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log epic label_removed activity", "error", err, "epic_id", epicID)
	}
	return nil
}

// ListStories returns stories that belong to an epic.
func (s *PMEpicService) ListStories(ctx context.Context, epicID string) ([]model.PMStory, error) {
	return s.epicRepo.ListStories(ctx, epicID)
}

func (s *PMEpicService) syncProgress(ctx context.Context, epicID string) error {
	withStats, err := s.epicRepo.GetWithStats(ctx, epicID)
	if err != nil {
		return err
	}
	if withStats == nil {
		return fmt.Errorf("epic not found")
	}
	epic := withStats.Epic
	stats := withStats.Stats
	now := time.Now().UTC()

	started := false
	completed := false
	if stats.StoryCount > 0 {
		if stats.DoneStoryCount == stats.StoryCount {
			started = true
			completed = true
		} else if stats.DoneStoryCount > 0 || stats.InProgressCount > 0 {
			started = true
		}
	}

	epic.Started = started
	epic.Completed = completed
	if started {
		if epic.StartedAt == nil {
			epic.StartedAt = &now
		}
	} else {
		epic.StartedAt = nil
	}
	if completed {
		if epic.CompletedAt == nil {
			epic.CompletedAt = &now
		}
	} else {
		epic.CompletedAt = nil
	}

	if err := s.epicRepo.Update(ctx, &epic); err != nil {
		return err
	}
	return nil
}

func isValidEpicHealth(value string) bool {
	switch value {
	case model.PMEpicHealthNone, model.PMEpicHealthOnTrack, model.PMEpicHealthAtRisk, model.PMEpicHealthOffTrack:
		return true
	default:
		return false
	}
}

func optionalActor(actorID string) *string {
	if actorID == "" {
		return nil
	}
	return &actorID
}

func stringPtr(value string) *string { return &value }

// computeEpicSuggestedHealth calculates health based on story progress vs the planned schedule.
func computeEpicSuggestedHealth(epic *model.EpicWithStats) string {
	return computeEpicSuggestedHealthAt(epic, time.Now())
}

func computeEpicSuggestedHealthAt(epic *model.EpicWithStats, now time.Time) string {
	if epic.Epic.PlannedStartDate == nil || epic.Epic.Deadline == nil {
		return model.PMEpicHealthNone
	}
	if epic.Stats.StoryCount == 0 {
		return model.PMEpicHealthNone
	}

	start := startOfDayUTC(*epic.Epic.PlannedStartDate)
	end := startOfDayUTC(*epic.Epic.Deadline)
	today := startOfDayUTC(now)
	if end.Before(start) {
		return model.PMEpicHealthNone
	}
	if today.Before(start) {
		return model.PMEpicHealthNone
	}

	totalDays := int(end.Sub(start).Hours()/24) + 1
	if totalDays <= 0 {
		return model.PMEpicHealthNone
	}

	// Past deadline with incomplete work
	if today.After(end) && epic.Stats.DoneStoryCount < epic.Stats.StoryCount {
		return model.PMEpicHealthOffTrack
	}

	// Compare actual progress against completed schedule days. This keeps date-only
	// plans from looking overdue at midnight on the start or deadline date.
	elapsedDays := int(today.Sub(start).Hours() / 24)
	if elapsedDays < 0 {
		elapsedDays = 0
	}
	if elapsedDays > totalDays {
		elapsedDays = totalDays
	}

	expectedPct := (float64(elapsedDays) / float64(totalDays)) * 100
	if expectedPct > 100 {
		expectedPct = 100
	}
	actualPct := float64(0)
	if epic.Stats.StoryCount > 0 {
		actualPct = float64(epic.Stats.DoneStoryCount) / float64(epic.Stats.StoryCount) * 100
	}
	gap := expectedPct - actualPct

	if gap <= 10 {
		return model.PMEpicHealthOnTrack
	} else if gap <= 25 {
		return model.PMEpicHealthAtRisk
	}
	return model.PMEpicHealthOffTrack
}

func enrichEpicSuggestedHealth(epic *model.EpicWithStats) {
	epic.SuggestedHealth = computeEpicSuggestedHealth(epic)
}

func startOfDayUTC(value time.Time) time.Time {
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}
