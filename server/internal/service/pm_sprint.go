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

// PMSprintService contains sprint business logic.
type PMSprintService struct {
	sprintRepo          *repository.PMSprintRepository
	closeoutRepo        *repository.PMSprintCloseoutRepository
	labelRepo           *repository.PMLabelRepository
	attachmentRepo      *repository.PMAttachmentRepository
	workspaceRepo       *repository.WorkspaceRepository
	settingsRepo        *repository.SettingsRepository
	activityService     *PMActivityService
	wsPublisher         *websocket.Publisher
	notificationService *NotificationService
	logger              *slog.Logger
}

// NewPMSprintService creates a new PMSprintService.
func NewPMSprintService(sprintRepo *repository.PMSprintRepository, labelRepo *repository.PMLabelRepository, attachmentRepo *repository.PMAttachmentRepository, workspaceRepo *repository.WorkspaceRepository, settingsRepo *repository.SettingsRepository, activityService *PMActivityService, wsPublisher *websocket.Publisher, notificationService *NotificationService, closeoutRepo *repository.PMSprintCloseoutRepository) *PMSprintService {
	return &PMSprintService{sprintRepo: sprintRepo, closeoutRepo: closeoutRepo, labelRepo: labelRepo, attachmentRepo: attachmentRepo, workspaceRepo: workspaceRepo, settingsRepo: settingsRepo, activityService: activityService, wsPublisher: wsPublisher, notificationService: notificationService, logger: slog.Default().With("service", "pm_sprint")}
}

// List returns sprints with filters.
func (s *PMSprintService) List(ctx context.Context, workspaceID string, filters model.PMSprintListFilters) ([]model.SprintWithStats, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	filters.AccessibleTeamIDs = accessibleTeamIDs(ctx)
	sprints, err := s.sprintRepo.List(ctx, workspaceID, filters)
	if err != nil {
		return nil, err
	}
	return s.sprintRepo.EnrichSprints(ctx, sprints)
}

// ListPage returns a repository-bounded sprint page with batch enrichment.
func (s *PMSprintService) ListPage(ctx context.Context, workspaceID string, filters model.PMSprintListFilters, pagination model.PMPagination) ([]model.SprintWithStats, int, int, int, error) {
	if workspaceID == "" {
		return nil, 0, 0, 0, fmt.Errorf("workspace_id is required")
	}
	filters.AccessibleTeamIDs = accessibleTeamIDs(ctx)
	return s.sprintRepo.ListPage(ctx, workspaceID, filters, pagination)
}

// ListByIDs returns accessible sprint rows for batch target-title enrichment.
func (s *PMSprintService) ListByIDs(ctx context.Context, workspaceID string, ids []string) ([]model.PMSprint, error) {
	sprints, err := s.sprintRepo.ListByIDs(ctx, workspaceID, ids)
	if err != nil {
		return nil, err
	}
	accessible := sprints[:0]
	for _, sprint := range sprints {
		if canAccessTeam(ctx, sprint.TeamID) {
			accessible = append(accessible, sprint)
		}
	}
	return accessible, nil
}

// ListPlanningWorkspace returns grouped sprints and an unassigned backlog for the planning page.
func (s *PMSprintService) ListPlanningWorkspace(ctx context.Context, workspaceID string, filters model.PMSprintPlanningFilters) (*model.SprintPlanningWorkspace, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	filters.AccessibleTeamIDs = accessibleTeamIDs(ctx)
	return s.sprintRepo.ListPlanningWorkspace(ctx, workspaceID, filters)
}

// GetByID returns sprint with stats.
func (s *PMSprintService) GetByID(ctx context.Context, id string) (*model.SprintWithStats, error) {
	sprint, err := s.sprintRepo.GetWithStats(ctx, id)
	if err != nil {
		return nil, err
	}
	if sprint == nil {
		return nil, fmt.Errorf("sprint not found")
	}
	if err := requireTeamAccess(ctx, sprint.Sprint.TeamID); err != nil {
		return nil, fmt.Errorf("sprint not found")
	}
	return sprint, nil
}

// GetCloseout returns frozen sprint results plus inbound rollover context.
func (s *PMSprintService) GetCloseout(ctx context.Context, sprintID string) (*model.SprintCloseoutResponse, error) {
	sprint, err := s.sprintRepo.GetWithStats(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	if sprint == nil {
		return nil, fmt.Errorf("sprint not found")
	}
	if err := requireTeamAccess(ctx, sprint.Sprint.TeamID); err != nil {
		return nil, fmt.Errorf("sprint not found")
	}
	if s.closeoutRepo == nil {
		return &model.SprintCloseoutResponse{RolledInFrom: []model.SprintInboundRolloverSummary{}}, nil
	}

	closeout, _, err := s.closeoutRepo.GetCloseoutBySprintID(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	rolledInFrom, err := s.closeoutRepo.ListInboundRolloverSummaries(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	if rolledInFrom == nil {
		rolledInFrom = []model.SprintInboundRolloverSummary{}
	}
	return &model.SprintCloseoutResponse{
		Closeout:     closeout,
		RolledInFrom: rolledInFrom,
	}, nil
}

// ListCloseouts returns frozen closeout summaries for PM reports.
func (s *PMSprintService) ListCloseouts(ctx context.Context, workspaceID string, teamID *string) ([]model.SprintCloseoutListItem, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if s.closeoutRepo == nil {
		return []model.SprintCloseoutListItem{}, nil
	}
	if teamID != nil && *teamID != "" && !canAccessTeam(ctx, teamID) {
		return []model.SprintCloseoutListItem{}, nil
	}

	items, err := s.closeoutRepo.ListCloseoutSummaries(ctx, workspaceID, teamID)
	if err != nil {
		return nil, err
	}

	accessible := accessibleTeamIDs(ctx)
	if accessible == nil {
		return items, nil
	}
	if len(accessible) == 0 {
		return []model.SprintCloseoutListItem{}, nil
	}

	allowed := make(map[string]struct{}, len(accessible))
	for _, id := range accessible {
		allowed[id] = struct{}{}
	}

	filtered := make([]model.SprintCloseoutListItem, 0, len(items))
	for _, item := range items {
		if item.TeamID == nil || *item.TeamID == "" {
			continue
		}
		if _, ok := allowed[*item.TeamID]; ok {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

// Create creates a sprint.
func (s *PMSprintService) Create(ctx context.Context, req model.CreateSprintRequest, actorID string) (*model.SprintWithStats, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}
	if err := requireCanManage(ctx, req.TeamID); err != nil {
		return nil, err
	}
	// Check if sprints are enabled for this team
	if req.TeamID != nil && *req.TeamID != "" && s.settingsRepo != nil {
		team, err := s.settingsRepo.GetTeamByID(ctx, *req.TeamID)
		if err == nil && team != nil && !team.SprintsEnabled {
			return nil, fmt.Errorf("sprints are disabled for this team")
		}
	}
	if !req.EndDate.After(req.StartDate) {
		return nil, fmt.Errorf("end_date must be after start_date")
	}
	overlap, err := s.sprintRepo.HasDateOverlap(ctx, req.WorkspaceID, req.TeamID, req.StartDate, req.EndDate, nil)
	if err != nil {
		return nil, err
	}
	if overlap {
		return nil, fmt.Errorf("sprint date range overlaps with another sprint for the same team")
	}
	labelIDs := dedupeIDs(req.LabelIDs)
	if len(labelIDs) > 0 {
		if err := validateLabelScope(ctx, s.labelRepo, req.WorkspaceID, labelIDs, allowedTeamIDs(req.TeamID)); err != nil {
			return nil, err
		}
	}

	sprint := &model.PMSprint{
		WorkspaceID: req.WorkspaceID,
		Name:        strings.TrimSpace(req.Name),
		Description: req.Description,
		StartDate:   &req.StartDate,
		EndDate:     &req.EndDate,
		TeamID:      req.TeamID,
	}
	if actorID != "" {
		sprint.CreatedBy = &actorID
	}

	if err := s.sprintRepo.CreateWithLabels(ctx, sprint, labelIDs); err != nil {
		s.logger.ErrorContext(ctx, "failed to create sprint", "error", err, "workspace_id", req.WorkspaceID)
		return nil, err
	}
	if len(req.AttachmentIDs) > 0 && s.attachmentRepo != nil {
		if err := s.attachmentRepo.ReassignToEntity(ctx, req.AttachmentIDs, "sprint", sprint.ID); err != nil {
			s.logger.ErrorContext(ctx, "failed to reassign attachments to sprint", "error", err, "sprint_id", sprint.ID, "attachment_ids", req.AttachmentIDs)
		}
	}
	s.logger.InfoContext(ctx, "sprint created", "sprint_id", sprint.ID, "workspace_id", sprint.WorkspaceID, "name", sprint.Name)
	if err := s.activityService.Log(ctx, sprint.WorkspaceID, "sprint", sprint.ID, optionalActor(actorID), "created", nil, nil, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log sprint created activity", "error", err, "sprint_id", sprint.ID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "sprint", EntityID: sprint.ID, WorkspaceID: sprint.WorkspaceID, ActorID: actorID})

	if s.notificationService != nil {
		if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
			WorkspaceID: sprint.WorkspaceID,
			ActorID:     actorID,
			EventType:   "sprint.created",
			EntityType:  "sprint",
			EntityID:    sprint.ID,
			Title:       "created sprint " + sprint.Name,
			Category:    "activity",
			Priority:    "normal",
			TeamID:      derefString(sprint.TeamID),
			EntitySnapshot: model.JSONB{
				"title": sprint.Name,
			},
		}); err != nil {
			s.logger.ErrorContext(ctx, "failed to emit sprint created notification", "error", err, "sprint_id", sprint.ID)
		}
		if sprint.Description != nil {
			if _, err := emitMentionNotification(ctx, s.notificationService, s.workspaceRepo, pmMentionNotificationInput{
				WorkspaceID:     sprint.WorkspaceID,
				ActorID:         actorID,
				Body:            *sprint.Description,
				EventType:       "sprint.mention",
				EntityType:      "sprint",
				EntityID:        sprint.ID,
				Title:           "mentioned you in sprint " + sprint.Name,
				TeamID:          derefString(sprint.TeamID),
				ReadableTeamIDs: mentionScopeForTeamID(sprint.TeamID),
				EntitySnapshot:  model.JSONB{"title": sprint.Name},
			}); err != nil {
				s.logger.ErrorContext(ctx, "failed to emit sprint mention notification", "error", err, "sprint_id", sprint.ID)
			}
		}
	}

	return s.sprintRepo.GetWithStats(ctx, sprint.ID)
}

// Update updates a sprint.
func (s *PMSprintService) Update(ctx context.Context, id string, req model.UpdateSprintRequest, actorID string) (*model.SprintWithStats, error) {
	current, err := s.sprintRepo.GetWithStats(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("sprint not found")
	}
	if err := requireCanManage(ctx, current.Sprint.TeamID); err != nil {
		return nil, fmt.Errorf("sprint not found")
	}
	sprint := current.Sprint

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		sprint.Name = name
	}
	if req.Description != nil {
		sprint.Description = req.Description
	}
	if req.StartDate != nil {
		sprint.StartDate = req.StartDate
	}
	if req.EndDate != nil {
		sprint.EndDate = req.EndDate
	}
	if req.TeamID != nil {
		sprint.TeamID = req.TeamID
		if err := requireCanManage(ctx, sprint.TeamID); err != nil {
			return nil, err
		}
	}
	if req.Archived != nil {
		sprint.Archived = *req.Archived
	}

	if sprint.StartDate == nil || sprint.EndDate == nil {
		return nil, fmt.Errorf("start_date and end_date are required")
	}
	if !sprint.EndDate.After(*sprint.StartDate) {
		return nil, fmt.Errorf("end_date must be after start_date")
	}
	excludeID := id
	overlap, err := s.sprintRepo.HasDateOverlap(ctx, sprint.WorkspaceID, sprint.TeamID, *sprint.StartDate, *sprint.EndDate, &excludeID)
	if err != nil {
		return nil, err
	}
	if overlap {
		return nil, fmt.Errorf("sprint date range overlaps with another sprint for the same team")
	}
	normalizedLabelIDs := req.LabelIDs
	if req.LabelIDs != nil {
		normalizedLabelIDs = dedupeIDs(req.LabelIDs)
	}
	labelIDsToValidate := normalizedLabelIDs
	if req.TeamID != nil && req.LabelIDs == nil {
		labelIDsToValidate = make([]string, 0, len(current.Labels))
		for _, label := range current.Labels {
			labelIDsToValidate = append(labelIDsToValidate, label.ID)
		}
	}
	if len(labelIDsToValidate) > 0 {
		if err := validateLabelScope(ctx, s.labelRepo, sprint.WorkspaceID, labelIDsToValidate, allowedTeamIDs(sprint.TeamID)); err != nil {
			return nil, err
		}
	}

	if err := s.sprintRepo.UpdateWithLabels(ctx, &sprint, normalizedLabelIDs); err != nil {
		s.logger.ErrorContext(ctx, "failed to update sprint", "error", err, "sprint_id", id)
		return nil, err
	}

	s.logger.InfoContext(ctx, "sprint updated", "sprint_id", sprint.ID, "workspace_id", sprint.WorkspaceID)
	if err := s.activityService.Log(ctx, sprint.WorkspaceID, "sprint", sprint.ID, optionalActor(actorID), "updated", nil, nil, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log sprint updated activity", "error", err, "sprint_id", sprint.ID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "sprint", EntityID: sprint.ID, WorkspaceID: sprint.WorkspaceID, ActorID: actorID})

	if s.notificationService != nil {
		if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
			WorkspaceID: sprint.WorkspaceID,
			ActorID:     actorID,
			EventType:   "sprint.updated",
			EntityType:  "sprint",
			EntityID:    sprint.ID,
			Title:       "updated sprint " + sprint.Name,
			Category:    "activity",
			Priority:    "normal",
			TeamID:      derefString(sprint.TeamID),
			EntitySnapshot: model.JSONB{
				"title": sprint.Name,
			},
		}); err != nil {
			s.logger.ErrorContext(ctx, "failed to emit sprint updated notification", "error", err, "sprint_id", sprint.ID)
		}
		if req.Description != nil {
			if _, err := emitMentionNotification(ctx, s.notificationService, s.workspaceRepo, pmMentionNotificationInput{
				WorkspaceID:     sprint.WorkspaceID,
				ActorID:         actorID,
				Body:            *req.Description,
				EventType:       "sprint.mention",
				EntityType:      "sprint",
				EntityID:        sprint.ID,
				Title:           "mentioned you in sprint " + sprint.Name,
				TeamID:          derefString(sprint.TeamID),
				ReadableTeamIDs: mentionScopeForTeamID(sprint.TeamID),
				EntitySnapshot:  model.JSONB{"title": sprint.Name},
			}); err != nil {
				s.logger.ErrorContext(ctx, "failed to emit sprint mention notification", "error", err, "sprint_id", sprint.ID)
			}
		}
	}

	return s.sprintRepo.GetWithStats(ctx, sprint.ID)
}

// Delete deletes a sprint.
func (s *PMSprintService) Delete(ctx context.Context, id string, actorID string) error {
	current, err := s.sprintRepo.GetWithStats(ctx, id)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("sprint not found")
	}
	if err := requireCanManage(ctx, current.Sprint.TeamID); err != nil {
		return fmt.Errorf("sprint not found")
	}
	if err := s.sprintRepo.Delete(ctx, id); err != nil {
		s.logger.ErrorContext(ctx, "failed to delete sprint", "error", err, "sprint_id", id)
		return err
	}
	s.logger.InfoContext(ctx, "sprint deleted", "sprint_id", id, "workspace_id", current.Sprint.WorkspaceID)
	if err := s.activityService.Log(ctx, current.Sprint.WorkspaceID, "sprint", id, optionalActor(actorID), "deleted", nil, nil, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log sprint deleted activity", "error", err, "sprint_id", id)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "deleted", Entity: "sprint", EntityID: id, WorkspaceID: current.Sprint.WorkspaceID, ActorID: actorID})
	return nil
}

// GetCurrentSprint returns active sprint for workspace/team.
func (s *PMSprintService) GetCurrentSprint(ctx context.Context, workspaceID string, teamID *string) (*model.PMSprint, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.sprintRepo.GetCurrentSprint(ctx, workspaceID, teamID)
}

// ListTasks returns tasks in a sprint, with TaskKey and table-facing computed fields populated.
func (s *PMSprintService) ListTasks(ctx context.Context, sprintID string) ([]model.BoardTask, error) {
	tasks, err := s.sprintRepo.ListEnrichedTasks(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	return s.populateTaskKeys(ctx, tasks), nil
}

// ListTasksPage returns a repository-bounded sprint task page with batch enrichment.
func (s *PMSprintService) ListTasksPage(ctx context.Context, sprintID string, search *string, pagination model.PMPagination) ([]model.BoardTask, int, int, int, error) {
	tasks, total, page, perPage, err := s.sprintRepo.ListEnrichedTasksPage(ctx, sprintID, search, pagination)
	if err != nil {
		return nil, 0, page, perPage, err
	}
	return s.populateTaskKeys(ctx, tasks), total, page, perPage, nil
}

func (s *PMSprintService) populateTaskKeys(ctx context.Context, tasks []model.BoardTask) []model.BoardTask {
	if len(tasks) == 0 {
		return tasks
	}
	ws, err := s.workspaceRepo.GetByID(ctx, tasks[0].WorkspaceID)
	if err != nil || ws == nil {
		return tasks
	}
	for i := range tasks {
		tasks[i].TaskKey = model.FormatTaskKey(ws.WorkspaceKey, tasks[i].DisplayID)
		tasks[i].PMTask.TaskKey = tasks[i].TaskKey
	}
	return tasks
}

// ListPreviewTasksPage returns lightweight task previews for a sprint page.
func (s *PMSprintService) ListPreviewTasksPage(ctx context.Context, sprintID string, pagination model.PMPagination) (*model.PaginatedResponse, error) {
	return s.sprintRepo.ListPreviewTasksPage(ctx, sprintID, pagination)
}

// ComputeStats returns computed story/point stats for a sprint.
func (s *PMSprintService) ComputeStats(ctx context.Context, sprintID string) (model.PMSprintStats, error) {
	return s.sprintRepo.ComputeStats(ctx, sprintID)
}

func toDay(ts time.Time) time.Time {
	return time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, time.UTC)
}
