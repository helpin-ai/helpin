package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// PMObjectiveService contains objective business logic.
type PMObjectiveService struct {
	objectiveRepo       *repository.PMObjectiveRepository
	krRepo              *repository.PMKeyResultRepository
	labelRepo           *repository.PMLabelRepository
	attachmentRepo      *repository.PMAttachmentRepository
	workspaceRepo       *repository.WorkspaceRepository
	activitySvc         *PMActivityService
	wsPublisher         *websocket.Publisher
	notificationService *NotificationService
	logger              *slog.Logger
}

// NewPMObjectiveService creates a new PMObjectiveService.
func NewPMObjectiveService(
	objectiveRepo *repository.PMObjectiveRepository,
	krRepo *repository.PMKeyResultRepository,
	labelRepo *repository.PMLabelRepository,
	attachmentRepo *repository.PMAttachmentRepository,
	workspaceRepo *repository.WorkspaceRepository,
	activitySvc *PMActivityService,
	wsPublisher *websocket.Publisher,
	notificationService *NotificationService,
) *PMObjectiveService {
	return &PMObjectiveService{
		objectiveRepo:       objectiveRepo,
		krRepo:              krRepo,
		labelRepo:           labelRepo,
		attachmentRepo:      attachmentRepo,
		workspaceRepo:       workspaceRepo,
		activitySvc:         activitySvc,
		wsPublisher:         wsPublisher,
		notificationService: notificationService,
		logger:              slog.Default().With("service", "pm_objective"),
	}
}

// requireAdmin checks that the actor has owner or admin role.
func (s *PMObjectiveService) requireAdmin(ctx context.Context, workspaceID, actorID string) error {
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

// List returns objectives with details for a workspace.
func (s *PMObjectiveService) List(ctx context.Context, workspaceID string, filters model.PMObjectiveListFilters) ([]model.ObjectiveWithDetails, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	// Objectives are strategic — readable workspace-wide for all roles.
	// Team filtering is NOT applied to objective lists.
	objectives, err := s.objectiveRepo.List(ctx, workspaceID, filters)
	if err != nil {
		return nil, err
	}

	result := make([]model.ObjectiveWithDetails, 0, len(objectives))
	for _, obj := range objectives {
		details, err := s.objectiveRepo.GetByID(ctx, obj.ID)
		if err != nil {
			return nil, err
		}
		if details != nil {
			enrichSuggestedHealth(details)
			result = append(result, *details)
		}
	}
	return result, nil
}

// ListPage loads and enriches only the requested objective page.
func (s *PMObjectiveService) ListPage(ctx context.Context, workspaceID string, filters model.PMObjectiveListFilters, pagination model.PMPagination) ([]model.ObjectiveWithDetails, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	objectives, total, err := s.objectiveRepo.ListPage(ctx, workspaceID, filters, pagination)
	if err != nil {
		return nil, 0, err
	}
	result := make([]model.ObjectiveWithDetails, 0, len(objectives))
	for _, obj := range objectives {
		details, err := s.objectiveRepo.GetByID(ctx, obj.ID)
		if err != nil {
			return nil, 0, err
		}
		if details != nil {
			enrichSuggestedHealth(details)
			result = append(result, *details)
		}
	}
	return result, total, nil
}

// ListByIDs returns objective rows for bounded run-target title enrichment.
func (s *PMObjectiveService) ListByIDs(ctx context.Context, workspaceID string, ids []string) ([]model.PMObjective, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.objectiveRepo.ListByIDs(ctx, workspaceID, ids)
}

// GetByID returns a single objective with details.
// When workspaceID is provided, the query is scoped to that workspace
// to prevent cross-workspace data access.
func (s *PMObjectiveService) GetByID(ctx context.Context, id string, workspaceID ...string) (*model.ObjectiveWithDetails, error) {
	obj, err := s.objectiveRepo.GetByID(ctx, id, workspaceID...)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, fmt.Errorf("objective not found")
	}
	// Objectives are strategic — readable workspace-wide for all roles.
	enrichSuggestedHealth(obj)
	return obj, nil
}

// Create creates an objective.
func (s *PMObjectiveService) Create(ctx context.Context, req model.CreateObjectiveRequest, actorID string) (*model.ObjectiveWithDetails, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}
	if err := requireCanManageTeams(ctx, req.TeamIDs); err != nil {
		return nil, err
	}

	objType := req.ObjectiveType
	if objType == "" {
		objType = model.PMObjectiveTypeTactical
	}
	if !isValidObjectiveType(objType) {
		return nil, fmt.Errorf("invalid objective_type")
	}

	state := model.PMObjectiveStateNotStarted
	if req.State != nil && *req.State != "" {
		normalized, ok := normalizeObjectiveState(*req.State)
		if !ok {
			return nil, fmt.Errorf("invalid state")
		}
		state = normalized
	}

	health := model.PMObjectiveHealthOnTrack
	if req.Health != nil && *req.Health != "" {
		health = *req.Health
	}
	if !isValidObjectiveHealth(health) {
		return nil, fmt.Errorf("invalid health")
	}

	obj := &model.PMObjective{
		WorkspaceID:      req.WorkspaceID,
		Name:             strings.TrimSpace(req.Name),
		Description:      req.Description,
		ObjectiveType:    objType,
		State:            state,
		PlannedStartDate: req.PlannedStartDate,
		Deadline:         req.Deadline,
		Health:           health,
		HealthComment:    req.HealthComment,
	}
	if req.Position != nil {
		obj.Position = *req.Position
	}
	if actorID != "" {
		obj.CreatedBy = &actorID
	}

	var resolvedOwnerMemberIDs []string
	if len(req.OwnerIDs) > 0 || len(req.OwnerMemberIDs) > 0 {
		owners, err := resolveWorkspaceMemberReferences(ctx, s.workspaceRepo, req.WorkspaceID, req.OwnerMemberIDs, req.OwnerIDs)
		if err != nil {
			return nil, err
		}
		resolvedOwnerMemberIDs = memberIDs(owners)
	}
	if len(req.LabelIDs) > 0 {
		if err := validateLabelScope(ctx, s.labelRepo, req.WorkspaceID, req.LabelIDs, req.TeamIDs); err != nil {
			return nil, err
		}
	}
	if err := s.objectiveRepo.Transaction(ctx, func(repo *repository.PMObjectiveRepository) error {
		if err := repo.Create(ctx, obj); err != nil {
			return err
		}
		if len(req.TeamIDs) > 0 {
			if err := repo.ReplaceTeams(ctx, obj.ID, req.TeamIDs); err != nil {
				return err
			}
		}
		if len(resolvedOwnerMemberIDs) > 0 {
			if err := repo.ReplaceOwners(ctx, obj.ID, resolvedOwnerMemberIDs); err != nil {
				return err
			}
		}
		if len(req.LabelIDs) > 0 {
			if err := repo.ReplaceLabels(ctx, obj.ID, req.LabelIDs); err != nil {
				return err
			}
		}
		if len(req.EpicIDs) > 0 {
			if err := repo.ReplaceEpics(ctx, obj.ID, req.EpicIDs); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if len(req.AttachmentIDs) > 0 && s.attachmentRepo != nil {
		if err := s.attachmentRepo.ReassignToEntity(ctx, req.AttachmentIDs, "objective", obj.ID); err != nil {
			s.logger.ErrorContext(ctx, "failed to reassign attachments to objective", "error", err, "objective_id", obj.ID, "attachment_ids", req.AttachmentIDs)
		}
	}

	if err := s.activitySvc.Log(ctx, obj.WorkspaceID, "objective", obj.ID, optionalActor(actorID), "created", nil, nil, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for objective create", "error", err, "objective_id", obj.ID, "workspace_id", obj.WorkspaceID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "objective", EntityID: obj.ID, WorkspaceID: obj.WorkspaceID, ActorID: actorID})

	if s.notificationService != nil {
		if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
			WorkspaceID: obj.WorkspaceID,
			ActorID:     actorID,
			EventType:   "objective.created",
			EntityType:  "objective",
			EntityID:    obj.ID,
			Title:       "created objective " + obj.Name,
			Category:    "activity",
			Priority:    "normal",
			EntitySnapshot: model.JSONB{
				"title": obj.Name,
				"type":  obj.ObjectiveType,
			},
		}); err != nil {
			s.logger.ErrorContext(ctx, "failed to emit notification for objective create", "error", err, "objective_id", obj.ID, "workspace_id", obj.WorkspaceID)
		}
		if obj.Description != nil {
			if _, err := emitMentionNotification(ctx, s.notificationService, s.workspaceRepo, pmMentionNotificationInput{
				WorkspaceID:     obj.WorkspaceID,
				ActorID:         actorID,
				Body:            *obj.Description,
				EventType:       "objective.mention",
				EntityType:      "objective",
				EntityID:        obj.ID,
				Title:           "mentioned you in objective " + obj.Name,
				ReadableTeamIDs: req.TeamIDs,
				EntitySnapshot: model.JSONB{
					"title": obj.Name,
					"type":  obj.ObjectiveType,
				},
			}); err != nil {
				s.logger.ErrorContext(ctx, "failed to emit notification for objective mention", "error", err, "objective_id", obj.ID, "workspace_id", obj.WorkspaceID)
			}
		}
	}

	s.logger.InfoContext(ctx, "objective created", "objective_id", obj.ID, "workspace_id", obj.WorkspaceID, "actor_id", actorID)
	return s.GetByID(ctx, obj.ID)
}

// Update updates an objective.
func (s *PMObjectiveService) Update(ctx context.Context, id string, req model.UpdateObjectiveRequest, actorID string) (*model.ObjectiveWithDetails, error) {
	current, err := s.objectiveRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("objective not found")
	}
	if err := requireCanManageTeams(ctx, current.Teams); err != nil {
		return nil, fmt.Errorf("objective not found")
	}
	obj := current.Objective

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		obj.Name = name
	}
	if req.Description != nil {
		obj.Description = req.Description
	}
	if req.ObjectiveType != nil {
		if !isValidObjectiveType(*req.ObjectiveType) {
			return nil, fmt.Errorf("invalid objective_type")
		}
		obj.ObjectiveType = *req.ObjectiveType
	}
	if req.State != nil {
		normalized, ok := normalizeObjectiveState(*req.State)
		if !ok {
			return nil, fmt.Errorf("invalid state")
		}
		obj.State = normalized
	}
	if req.Health != nil {
		if !isValidObjectiveHealth(*req.Health) {
			return nil, fmt.Errorf("invalid health")
		}
		obj.Health = *req.Health
	}
	if req.HealthComment != nil {
		obj.HealthComment = req.HealthComment
	}
	if req.PlannedStartDate != nil || req.PlannedStartDateSet {
		obj.PlannedStartDate = req.PlannedStartDate
	}
	if req.Deadline != nil || req.DeadlineSet {
		obj.Deadline = req.Deadline
	}
	if req.Position != nil {
		obj.Position = *req.Position
	}
	if req.Archived != nil {
		obj.Archived = *req.Archived
	}

	var resolvedOwnerMemberIDs []string
	if req.OwnerIDs != nil || req.OwnerMemberIDs != nil {
		owners, err := resolveWorkspaceMemberReferences(ctx, s.workspaceRepo, obj.WorkspaceID, req.OwnerMemberIDs, req.OwnerIDs)
		if err != nil {
			return nil, err
		}
		resolvedOwnerMemberIDs = memberIDs(owners)
	}
	if req.LabelIDs != nil {
		teamIDs := current.Teams
		if req.TeamIDs != nil {
			teamIDs = req.TeamIDs
		}
		if err := validateLabelScope(ctx, s.labelRepo, obj.WorkspaceID, req.LabelIDs, teamIDs); err != nil {
			return nil, err
		}
	}
	if err := s.objectiveRepo.Transaction(ctx, func(repo *repository.PMObjectiveRepository) error {
		if err := repo.Update(ctx, &obj); err != nil {
			return err
		}
		if req.TeamIDs != nil {
			if err := repo.ReplaceTeams(ctx, obj.ID, req.TeamIDs); err != nil {
				return err
			}
		}
		if req.OwnerIDs != nil || req.OwnerMemberIDs != nil {
			if err := repo.ReplaceOwners(ctx, obj.ID, resolvedOwnerMemberIDs); err != nil {
				return err
			}
		}
		if req.LabelIDs != nil {
			if err := repo.ReplaceLabels(ctx, obj.ID, req.LabelIDs); err != nil {
				return err
			}
		}
		if req.EpicIDs != nil {
			if err := repo.ReplaceEpics(ctx, obj.ID, req.EpicIDs); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if err := s.activitySvc.Log(ctx, obj.WorkspaceID, "objective", obj.ID, optionalActor(actorID), "updated", nil, nil, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for objective update", "error", err, "objective_id", obj.ID, "workspace_id", obj.WorkspaceID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "objective", EntityID: obj.ID, WorkspaceID: obj.WorkspaceID, ActorID: actorID})

	if s.notificationService != nil {
		if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
			WorkspaceID: obj.WorkspaceID,
			ActorID:     actorID,
			EventType:   "objective.updated",
			EntityType:  "objective",
			EntityID:    obj.ID,
			Title:       "updated objective " + obj.Name,
			Category:    "activity",
			Priority:    "normal",
			EntitySnapshot: model.JSONB{
				"title": obj.Name,
				"type":  obj.ObjectiveType,
			},
		}); err != nil {
			s.logger.ErrorContext(ctx, "failed to emit notification for objective update", "error", err, "objective_id", obj.ID, "workspace_id", obj.WorkspaceID)
		}
		if req.Description != nil {
			readableTeamIDs := current.Teams
			if req.TeamIDs != nil {
				readableTeamIDs = req.TeamIDs
			}
			if _, err := emitMentionNotification(ctx, s.notificationService, s.workspaceRepo, pmMentionNotificationInput{
				WorkspaceID:     obj.WorkspaceID,
				ActorID:         actorID,
				Body:            *req.Description,
				EventType:       "objective.mention",
				EntityType:      "objective",
				EntityID:        obj.ID,
				Title:           "mentioned you in objective " + obj.Name,
				ReadableTeamIDs: readableTeamIDs,
				EntitySnapshot: model.JSONB{
					"title": obj.Name,
					"type":  obj.ObjectiveType,
				},
			}); err != nil {
				s.logger.ErrorContext(ctx, "failed to emit notification for objective mention", "error", err, "objective_id", obj.ID, "workspace_id", obj.WorkspaceID)
			}
		}
	}

	s.logger.InfoContext(ctx, "objective updated", "objective_id", obj.ID, "workspace_id", obj.WorkspaceID, "actor_id", actorID)
	return s.GetByID(ctx, obj.ID)
}

// Delete archives an objective.
func (s *PMObjectiveService) Delete(ctx context.Context, id string, actorID string) error {
	obj, err := s.objectiveRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if obj == nil {
		return fmt.Errorf("objective not found")
	}
	if err := s.requireAdmin(ctx, obj.Objective.WorkspaceID, actorID); err != nil {
		return err
	}
	if err := s.objectiveRepo.Delete(ctx, id); err != nil {
		return err
	}
	if err := s.activitySvc.Log(ctx, obj.Objective.WorkspaceID, "objective", id, optionalActor(actorID), "archived", nil, nil, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for objective delete", "error", err, "objective_id", id, "workspace_id", obj.Objective.WorkspaceID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "deleted", Entity: "objective", EntityID: id, WorkspaceID: obj.Objective.WorkspaceID, ActorID: actorID})

	if s.notificationService != nil {
		if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
			WorkspaceID: obj.Objective.WorkspaceID,
			ActorID:     actorID,
			EventType:   "objective.deleted",
			EntityType:  "objective",
			EntityID:    id,
			Title:       "archived objective " + obj.Objective.Name,
			Category:    "activity",
			Priority:    "normal",
			EntitySnapshot: model.JSONB{
				"title": obj.Objective.Name,
			},
		}); err != nil {
			s.logger.ErrorContext(ctx, "failed to emit notification for objective delete", "error", err, "objective_id", id, "workspace_id", obj.Objective.WorkspaceID)
		}
	}

	s.logger.InfoContext(ctx, "objective deleted", "objective_id", id, "workspace_id", obj.Objective.WorkspaceID, "actor_id", actorID)
	return nil
}

// AddTeam links a team to an objective.
func (s *PMObjectiveService) AddTeam(ctx context.Context, objectiveID, teamID, actorID string) error {
	obj, err := s.objectiveRepo.GetByID(ctx, objectiveID)
	if err != nil {
		return err
	}
	if obj == nil {
		return fmt.Errorf("objective not found")
	}
	if err := requireCanManageTeams(ctx, obj.Teams); err != nil {
		return err
	}
	if err := s.objectiveRepo.AddTeam(ctx, objectiveID, teamID); err != nil {
		return err
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "objective", EntityID: objectiveID, WorkspaceID: obj.Objective.WorkspaceID, ActorID: actorID})
	return nil
}

// RemoveTeam unlinks a team from an objective.
func (s *PMObjectiveService) RemoveTeam(ctx context.Context, objectiveID, teamID, actorID string) error {
	obj, err := s.objectiveRepo.GetByID(ctx, objectiveID)
	if err != nil {
		return err
	}
	if obj == nil {
		return fmt.Errorf("objective not found")
	}
	if err := requireCanManageTeams(ctx, obj.Teams); err != nil {
		return err
	}
	if err := s.objectiveRepo.RemoveTeam(ctx, objectiveID, teamID); err != nil {
		return err
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "objective", EntityID: objectiveID, WorkspaceID: obj.Objective.WorkspaceID, ActorID: actorID})
	return nil
}

// AddOwner links an owner to an objective.
func (s *PMObjectiveService) AddOwner(ctx context.Context, objectiveID, ownerRef, actorID string) error {
	obj, err := s.objectiveRepo.GetByID(ctx, objectiveID)
	if err != nil {
		return err
	}
	if obj == nil {
		return fmt.Errorf("objective not found")
	}
	if err := requireCanManageTeams(ctx, obj.Teams); err != nil {
		return err
	}
	owner, err := resolveWorkspaceMemberReference(ctx, s.workspaceRepo, obj.Objective.WorkspaceID, ownerRef)
	if err != nil {
		return err
	}
	if err := s.objectiveRepo.AddOwner(ctx, objectiveID, owner.ID); err != nil {
		return err
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "objective", EntityID: objectiveID, WorkspaceID: obj.Objective.WorkspaceID, ActorID: actorID})

	if s.notificationService != nil && owner.UserID != nil {
		if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
			WorkspaceID:        obj.Objective.WorkspaceID,
			ActorID:            actorID,
			EventType:          "objective.assigned",
			EntityType:         "objective",
			EntityID:           objectiveID,
			Title:              "assigned you to objective " + obj.Objective.Name,
			Category:           "assignment",
			Priority:           "normal",
			ExplicitRecipients: []string{*owner.UserID},
			EntitySnapshot: model.JSONB{
				"title": obj.Objective.Name,
				"type":  obj.Objective.ObjectiveType,
			},
		}); err != nil {
			s.logger.ErrorContext(ctx, "failed to emit notification for objective assign", "error", err, "objective_id", objectiveID, "workspace_id", obj.Objective.WorkspaceID)
		}
	}

	return nil
}

// RemoveOwner unlinks an owner from an objective.
func (s *PMObjectiveService) RemoveOwner(ctx context.Context, objectiveID, ownerRef, actorID string) error {
	obj, err := s.objectiveRepo.GetByID(ctx, objectiveID)
	if err != nil {
		return err
	}
	if obj == nil {
		return fmt.Errorf("objective not found")
	}
	if err := requireCanManageTeams(ctx, obj.Teams); err != nil {
		return err
	}
	owner, err := resolveWorkspaceMemberReference(ctx, s.workspaceRepo, obj.Objective.WorkspaceID, ownerRef)
	if err != nil {
		return err
	}
	if err := s.objectiveRepo.RemoveOwner(ctx, objectiveID, owner.ID); err != nil {
		return err
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "objective", EntityID: objectiveID, WorkspaceID: obj.Objective.WorkspaceID, ActorID: actorID})
	return nil
}

// AddEpic links an epic to an objective.
func (s *PMObjectiveService) AddEpic(ctx context.Context, objectiveID, epicID, actorID string) error {
	obj, err := s.objectiveRepo.GetByID(ctx, objectiveID)
	if err != nil {
		return err
	}
	if obj == nil {
		return fmt.Errorf("objective not found")
	}
	if err := requireCanManageTeams(ctx, obj.Teams); err != nil {
		return err
	}
	if err := s.objectiveRepo.AddEpic(ctx, objectiveID, epicID); err != nil {
		return err
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "objective", EntityID: objectiveID, WorkspaceID: obj.Objective.WorkspaceID, ActorID: actorID})
	return nil
}

// RemoveEpic unlinks an epic from an objective.
func (s *PMObjectiveService) RemoveEpic(ctx context.Context, objectiveID, epicID, actorID string) error {
	obj, err := s.objectiveRepo.GetByID(ctx, objectiveID)
	if err != nil {
		return err
	}
	if obj == nil {
		return fmt.Errorf("objective not found")
	}
	if err := requireCanManageTeams(ctx, obj.Teams); err != nil {
		return err
	}
	if err := s.objectiveRepo.RemoveEpic(ctx, objectiveID, epicID); err != nil {
		return err
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "objective", EntityID: objectiveID, WorkspaceID: obj.Objective.WorkspaceID, ActorID: actorID})
	return nil
}

// ── Key Results ────────────────────────────────────────────────────

// CreateKeyResult creates a key result for an objective.
func (s *PMObjectiveService) CreateKeyResult(ctx context.Context, objectiveID string, req model.CreateKeyResultRequest, actorID string) (*model.PMKeyResult, error) {
	obj, err := s.objectiveRepo.GetByID(ctx, objectiveID)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, fmt.Errorf("objective not found")
	}
	if err := requireCanManageTeams(ctx, obj.Teams); err != nil {
		return nil, err
	}

	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	resultType := req.ResultType
	if resultType == "" {
		resultType = model.PMKeyResultTypeBoolean
	}
	if !isValidKeyResultType(resultType) {
		return nil, fmt.Errorf("invalid result_type")
	}

	kr := &model.PMKeyResult{
		ObjectiveID:  objectiveID,
		Name:         strings.TrimSpace(req.Name),
		ResultType:   resultType,
		InitialValue: req.InitialValue,
		CurrentValue: req.CurrentValue,
		TargetValue:  req.TargetValue,
		Note:         req.Note,
		UpdatedBy:    optionalActor(actorID),
	}
	if req.Note != nil && *req.Note != "" {
		now := time.Now()
		kr.NoteUpdatedBy = optionalActor(actorID)
		kr.NoteUpdatedAt = &now
	}
	if req.Position != nil {
		kr.Position = *req.Position
	}
	kr.Progress = computeKeyResultProgress(kr)

	if err := s.krRepo.Create(ctx, kr); err != nil {
		return nil, err
	}

	if err := s.activitySvc.Log(ctx, obj.Objective.WorkspaceID, "objective", objectiveID, optionalActor(actorID), "key_result_created", nil, nil, &kr.Name, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for key result create", "error", err, "key_result_id", kr.ID, "objective_id", objectiveID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "key_result", EntityID: kr.ID, WorkspaceID: obj.Objective.WorkspaceID, ActorID: actorID, ParentType: "objective", ParentID: objectiveID})
	s.logger.InfoContext(ctx, "key result created", "key_result_id", kr.ID, "objective_id", objectiveID, "workspace_id", obj.Objective.WorkspaceID, "actor_id", actorID)
	return kr, nil
}

// UpdateKeyResult updates a key result.
func (s *PMObjectiveService) UpdateKeyResult(ctx context.Context, id string, req model.UpdateKeyResultRequest, actorID string) (*model.PMKeyResult, error) {
	kr, err := s.krRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if kr == nil {
		return nil, fmt.Errorf("key result not found")
	}
	parentObj, err := s.objectiveRepo.GetByID(ctx, kr.ObjectiveID)
	if err != nil {
		return nil, err
	}
	if parentObj == nil {
		return nil, fmt.Errorf("objective not found")
	}
	if err := requireCanManageTeams(ctx, parentObj.Teams); err != nil {
		return nil, err
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		kr.Name = name
	}
	if req.ResultType != nil {
		if !isValidKeyResultType(*req.ResultType) {
			return nil, fmt.Errorf("invalid result_type")
		}
		kr.ResultType = *req.ResultType
	}
	if req.InitialValue != nil {
		kr.InitialValue = *req.InitialValue
	}
	if req.CurrentValue != nil {
		kr.CurrentValue = *req.CurrentValue
	}
	if req.TargetValue != nil {
		kr.TargetValue = *req.TargetValue
	}
	if req.Note != nil {
		kr.Note = req.Note
		now := time.Now()
		kr.NoteUpdatedBy = optionalActor(actorID)
		kr.NoteUpdatedAt = &now
	}
	if req.Position != nil {
		kr.Position = *req.Position
	}
	kr.Progress = computeKeyResultProgress(kr)
	kr.UpdatedBy = optionalActor(actorID)

	if err := s.krRepo.Update(ctx, kr); err != nil {
		return nil, err
	}
	if parentObj != nil {
		s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "key_result", EntityID: id, WorkspaceID: parentObj.Objective.WorkspaceID, ActorID: actorID, ParentType: "objective", ParentID: kr.ObjectiveID})
	}
	return kr, nil
}

// DeleteKeyResult hard-deletes a key result.
func (s *PMObjectiveService) DeleteKeyResult(ctx context.Context, id string, actorID string) error {
	kr, err := s.krRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if kr == nil {
		return fmt.Errorf("key result not found")
	}
	parentObj, _ := s.objectiveRepo.GetByID(ctx, kr.ObjectiveID)
	if parentObj != nil {
		if err := requireCanManageTeams(ctx, parentObj.Teams); err != nil {
			return err
		}
	}
	if err := s.krRepo.Delete(ctx, id); err != nil {
		return err
	}
	if parentObj != nil {
		s.wsPublisher.Publish(websocket.Event{Action: "deleted", Entity: "key_result", EntityID: id, WorkspaceID: parentObj.Objective.WorkspaceID, ActorID: actorID, ParentType: "objective", ParentID: kr.ObjectiveID})
	}
	return nil
}

// ── Helpers ────────────────────────────────────────────────────────

func computeKeyResultProgress(kr *model.PMKeyResult) float64 {
	switch kr.ResultType {
	case model.PMKeyResultTypeBoolean:
		if kr.CurrentValue >= kr.TargetValue && kr.TargetValue > 0 {
			return 100
		}
		return 0
	case model.PMKeyResultTypePercent:
		return math.Min(math.Max(kr.CurrentValue, 0), 100)
	case model.PMKeyResultTypeNumeric:
		denom := kr.TargetValue - kr.InitialValue
		if denom == 0 {
			return 0
		}
		pct := (kr.CurrentValue - kr.InitialValue) / denom * 100
		return math.Min(math.Max(pct, 0), 100)
	default:
		return 0
	}
}

func isValidObjectiveType(v string) bool {
	return v == model.PMObjectiveTypeTactical || v == model.PMObjectiveTypeStrategic
}

func normalizeObjectiveState(v string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "not_started", "not started", "todo", "to_do", "to do":
		return model.PMObjectiveStateNotStarted, true
	case "active", "in_progress", "in progress", "started", "on_track", "behind", "at_risk":
		return model.PMObjectiveStateActive, true
	case "closed", "done", "complete", "completed":
		return model.PMObjectiveStateClosed, true
	default:
		return "", false
	}
}

func isValidObjectiveHealth(v string) bool {
	return v == model.PMObjectiveHealthOnTrack || v == model.PMObjectiveHealthAtRisk || v == model.PMObjectiveHealthOffTrack
}

func isValidKeyResultType(v string) bool {
	return v == model.PMKeyResultTypeBoolean || v == model.PMKeyResultTypePercent || v == model.PMKeyResultTypeNumeric
}

// computeSuggestedHealth calculates health based on KR progress vs the planned schedule.
func computeSuggestedHealth(obj *model.ObjectiveWithDetails) string {
	return computeSuggestedHealthAt(obj, time.Now())
}

func computeSuggestedHealthAt(obj *model.ObjectiveWithDetails, now time.Time) string {
	// Need both dates and at least one KR to compute
	if obj.Objective.PlannedStartDate == nil || obj.Objective.Deadline == nil {
		return model.PMObjectiveHealthOnTrack
	}
	if obj.Stats.KeyResultCount == 0 {
		return model.PMObjectiveHealthOnTrack
	}

	start := startOfDayUTC(*obj.Objective.PlannedStartDate)
	end := startOfDayUTC(*obj.Objective.Deadline)
	today := startOfDayUTC(now)
	if end.Before(start) {
		return model.PMObjectiveHealthOnTrack
	}
	if today.Before(start) {
		return model.PMObjectiveHealthOnTrack
	}

	totalDays := int(end.Sub(start).Hours()/24) + 1
	if totalDays <= 0 {
		return model.PMObjectiveHealthOnTrack
	}

	// Past deadline with incomplete work
	if today.After(end) && obj.Stats.KeyResultAvgPct < 100 {
		return model.PMObjectiveHealthOffTrack
	}

	// Compare actual progress against completed schedule days so date-only plans
	// don't become late at midnight on their start or deadline date.
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
	actualPct := obj.Stats.KeyResultAvgPct
	gap := expectedPct - actualPct

	if gap <= 10 {
		return model.PMObjectiveHealthOnTrack
	} else if gap <= 25 {
		return model.PMObjectiveHealthAtRisk
	}
	return model.PMObjectiveHealthOffTrack
}

// enrichSuggestedHealth fills the SuggestedHealth field on a details object.
func enrichSuggestedHealth(obj *model.ObjectiveWithDetails) {
	obj.SuggestedHealth = computeSuggestedHealth(obj)
}
