package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// PMObjectiveService contains objective business logic.
type PMObjectiveService struct {
	objectiveRepo *repository.PMObjectiveRepository
	krRepo        *repository.PMKeyResultRepository
	labelRepo     *repository.PMLabelRepository
	activitySvc   *PMActivityService
	wsPublisher   *websocket.Publisher
}

// NewPMObjectiveService creates a new PMObjectiveService.
func NewPMObjectiveService(
	objectiveRepo *repository.PMObjectiveRepository,
	krRepo *repository.PMKeyResultRepository,
	labelRepo *repository.PMLabelRepository,
	activitySvc *PMActivityService,
	wsPublisher *websocket.Publisher,
) *PMObjectiveService {
	return &PMObjectiveService{
		objectiveRepo: objectiveRepo,
		krRepo:        krRepo,
		labelRepo:     labelRepo,
		activitySvc:   activitySvc,
		wsPublisher:   wsPublisher,
	}
}

// List returns objectives with details for a workspace.
func (s *PMObjectiveService) List(ctx context.Context, workspaceID string, filters model.PMObjectiveListFilters) ([]model.ObjectiveWithDetails, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
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

// GetByID returns a single objective with details.
func (s *PMObjectiveService) GetByID(ctx context.Context, id string) (*model.ObjectiveWithDetails, error) {
	obj, err := s.objectiveRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, fmt.Errorf("objective not found")
	}
	enrichSuggestedHealth(obj)
	return obj, nil
}

// Create creates an objective.
func (s *PMObjectiveService) Create(ctx context.Context, req model.CreateObjectiveRequest, actorID string) (*model.ObjectiveWithDetails, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
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

	if err := s.objectiveRepo.Create(ctx, obj); err != nil {
		return nil, err
	}

	// Sync many-to-many
	if len(req.TeamIDs) > 0 {
		if err := s.objectiveRepo.ReplaceTeams(ctx, obj.ID, req.TeamIDs); err != nil {
			return nil, err
		}
	}
	if len(req.OwnerIDs) > 0 {
		if err := s.objectiveRepo.ReplaceOwners(ctx, obj.ID, req.OwnerIDs); err != nil {
			return nil, err
		}
	}
	if len(req.LabelIDs) > 0 {
		if err := validateLabelScope(ctx, s.labelRepo, req.WorkspaceID, req.LabelIDs, req.TeamIDs); err != nil {
			return nil, err
		}
		if err := s.objectiveRepo.ReplaceLabels(ctx, obj.ID, req.LabelIDs); err != nil {
			return nil, err
		}
	}
	if len(req.EpicIDs) > 0 {
		if err := s.objectiveRepo.ReplaceEpics(ctx, obj.ID, req.EpicIDs); err != nil {
			return nil, err
		}
	}

	_ = s.activitySvc.Log(ctx, obj.WorkspaceID, "objective", obj.ID, optionalActor(actorID), "created", nil, nil, nil, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "objective", EntityID: obj.ID, WorkspaceID: obj.WorkspaceID, ActorID: actorID})
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
	if req.PlannedStartDate != nil {
		obj.PlannedStartDate = req.PlannedStartDate
	}
	if req.Deadline != nil {
		obj.Deadline = req.Deadline
	}
	if req.Position != nil {
		obj.Position = *req.Position
	}
	if req.Archived != nil {
		obj.Archived = *req.Archived
	}

	if err := s.objectiveRepo.Update(ctx, &obj); err != nil {
		return nil, err
	}

	if req.TeamIDs != nil {
		if err := s.objectiveRepo.ReplaceTeams(ctx, obj.ID, req.TeamIDs); err != nil {
			return nil, err
		}
	}
	if req.OwnerIDs != nil {
		if err := s.objectiveRepo.ReplaceOwners(ctx, obj.ID, req.OwnerIDs); err != nil {
			return nil, err
		}
	}
	if req.LabelIDs != nil {
		teamIDs := current.Teams
		if req.TeamIDs != nil {
			teamIDs = req.TeamIDs
		}
		if err := validateLabelScope(ctx, s.labelRepo, obj.WorkspaceID, req.LabelIDs, teamIDs); err != nil {
			return nil, err
		}
		if err := s.objectiveRepo.ReplaceLabels(ctx, obj.ID, req.LabelIDs); err != nil {
			return nil, err
		}
	}
	if req.EpicIDs != nil {
		if err := s.objectiveRepo.ReplaceEpics(ctx, obj.ID, req.EpicIDs); err != nil {
			return nil, err
		}
	}

	_ = s.activitySvc.Log(ctx, obj.WorkspaceID, "objective", obj.ID, optionalActor(actorID), "updated", nil, nil, nil, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "objective", EntityID: obj.ID, WorkspaceID: obj.WorkspaceID, ActorID: actorID})
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
	if err := s.objectiveRepo.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.activitySvc.Log(ctx, obj.Objective.WorkspaceID, "objective", id, optionalActor(actorID), "archived", nil, nil, nil, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "deleted", Entity: "objective", EntityID: id, WorkspaceID: obj.Objective.WorkspaceID, ActorID: actorID})
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
	if err := s.objectiveRepo.RemoveTeam(ctx, objectiveID, teamID); err != nil {
		return err
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "objective", EntityID: objectiveID, WorkspaceID: obj.Objective.WorkspaceID, ActorID: actorID})
	return nil
}

// AddOwner links an owner to an objective.
func (s *PMObjectiveService) AddOwner(ctx context.Context, objectiveID, userID, actorID string) error {
	obj, err := s.objectiveRepo.GetByID(ctx, objectiveID)
	if err != nil {
		return err
	}
	if obj == nil {
		return fmt.Errorf("objective not found")
	}
	if err := s.objectiveRepo.AddOwner(ctx, objectiveID, userID); err != nil {
		return err
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "objective", EntityID: objectiveID, WorkspaceID: obj.Objective.WorkspaceID, ActorID: actorID})
	return nil
}

// RemoveOwner unlinks an owner from an objective.
func (s *PMObjectiveService) RemoveOwner(ctx context.Context, objectiveID, userID, actorID string) error {
	obj, err := s.objectiveRepo.GetByID(ctx, objectiveID)
	if err != nil {
		return err
	}
	if obj == nil {
		return fmt.Errorf("objective not found")
	}
	if err := s.objectiveRepo.RemoveOwner(ctx, objectiveID, userID); err != nil {
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
	}
	if req.Position != nil {
		kr.Position = *req.Position
	}
	kr.Progress = computeKeyResultProgress(kr)

	if err := s.krRepo.Create(ctx, kr); err != nil {
		return nil, err
	}

	_ = s.activitySvc.Log(ctx, obj.Objective.WorkspaceID, "objective", objectiveID, optionalActor(actorID), "key_result_created", nil, nil, &kr.Name, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "key_result", EntityID: kr.ID, WorkspaceID: obj.Objective.WorkspaceID, ActorID: actorID, ParentType: "objective", ParentID: objectiveID})
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
	if req.Position != nil {
		kr.Position = *req.Position
	}
	kr.Progress = computeKeyResultProgress(kr)

	if err := s.krRepo.Update(ctx, kr); err != nil {
		return nil, err
	}
	if obj, _ := s.objectiveRepo.GetByID(ctx, kr.ObjectiveID); obj != nil {
		s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "key_result", EntityID: id, WorkspaceID: obj.Objective.WorkspaceID, ActorID: actorID, ParentType: "objective", ParentID: kr.ObjectiveID})
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
	if err := s.krRepo.Delete(ctx, id); err != nil {
		return err
	}
	if obj, _ := s.objectiveRepo.GetByID(ctx, kr.ObjectiveID); obj != nil {
		s.wsPublisher.Publish(websocket.Event{Action: "deleted", Entity: "key_result", EntityID: id, WorkspaceID: obj.Objective.WorkspaceID, ActorID: actorID, ParentType: "objective", ParentID: kr.ObjectiveID})
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

func isValidObjectiveState(v string) bool {
	return v == model.PMObjectiveStateNotStarted || v == model.PMObjectiveStateActive || v == model.PMObjectiveStateClosed
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

// computeSuggestedHealth calculates health based on KR progress vs time elapsed.
func computeSuggestedHealth(obj *model.ObjectiveWithDetails) string {
	// Need both dates and at least one KR to compute
	if obj.Objective.PlannedStartDate == nil || obj.Objective.Deadline == nil {
		return model.PMObjectiveHealthOnTrack
	}
	if obj.Stats.KeyResultCount == 0 {
		return model.PMObjectiveHealthOnTrack
	}

	now := time.Now()
	start := *obj.Objective.PlannedStartDate
	end := *obj.Objective.Deadline
	totalDays := end.Sub(start).Hours() / 24
	if totalDays <= 0 {
		return model.PMObjectiveHealthOnTrack
	}

	// Past deadline with incomplete work
	if now.After(end) && obj.Stats.KeyResultAvgPct < 100 {
		return model.PMObjectiveHealthOffTrack
	}

	elapsedDays := now.Sub(start).Hours() / 24
	if elapsedDays < 0 {
		// Not started yet (before planned start)
		return model.PMObjectiveHealthOnTrack
	}

	expectedPct := (elapsedDays / totalDays) * 100
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
