package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// PMWorkflowService contains workflow business logic.
type PMWorkflowService struct {
	workflowRepo *repository.PMWorkflowRepository
	storyRepo    *repository.PMStoryRepository
	labelRepo    *repository.PMLabelRepository
	logger       *slog.Logger
}

// NewPMWorkflowService creates a new PMWorkflowService.
func NewPMWorkflowService(workflowRepo *repository.PMWorkflowRepository, storyRepo *repository.PMStoryRepository, labelRepo *repository.PMLabelRepository) *PMWorkflowService {
	return &PMWorkflowService{
		workflowRepo: workflowRepo,
		storyRepo:    storyRepo,
		labelRepo:    labelRepo,
		logger:       slog.Default().With("service", "pm_workflow"),
	}
}

// ListByWorkspace lists workflows in a workspace.
func (s *PMWorkflowService) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.WorkflowWithStates, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.workflowRepo.ListByWorkspace(ctx, workspaceID)
}

// GetByID returns a workflow by ID.
func (s *PMWorkflowService) GetByID(ctx context.Context, id string) (*model.WorkflowWithStates, error) {
	wf, err := s.workflowRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if wf == nil {
		return nil, fmt.Errorf("workflow not found")
	}
	return wf, nil
}

// ListEpicStates returns epic workflow states for workspace, seeding defaults if none exist.
func (s *PMWorkflowService) ListEpicStates(ctx context.Context, workspaceID string) ([]model.PMEpicWorkflowState, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	states, err := s.workflowRepo.ListEpicStates(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if len(states) == 0 {
		return s.workflowRepo.SeedDefaultEpicStates(ctx, workspaceID)
	}
	return states, nil
}

// Create creates a workflow and seeds base states.
func (s *PMWorkflowService) Create(ctx context.Context, req model.CreateWorkflowRequest) (*model.WorkflowWithStates, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}

	workflow := &model.PMWorkflow{
		WorkspaceID:     req.WorkspaceID,
		Name:            strings.TrimSpace(req.Name),
		Description:     req.Description,
		TeamID:          req.TeamID,
		AutoAssignOwner: req.AutoAssignOwner,
	}
	if err := s.workflowRepo.Create(ctx, workflow); err != nil {
		s.logger.ErrorContext(ctx, "failed to create workflow", "error", err, "workspace_id", req.WorkspaceID)
		return nil, err
	}

	// New workflows start with the baseline 4-state layout.
	states := []model.PMWorkflowState{
		{WorkflowID: workflow.ID, Name: "Backlog", StateType: model.PMStateTypeBacklog, Position: 0},
		{WorkflowID: workflow.ID, Name: "To Do", StateType: model.PMStateTypeUnstarted, Position: 1, IsDefault: true},
		{WorkflowID: workflow.ID, Name: "In Progress", StateType: model.PMStateTypeStarted, Position: 2},
		{WorkflowID: workflow.ID, Name: "Done", StateType: model.PMStateTypeDone, Position: 3},
	}
	for i := range states {
		if err := s.workflowRepo.CreateState(ctx, &states[i]); err != nil {
			return nil, err
		}
	}
	defaultStateID := states[1].ID
	workflow.DefaultStateID = &defaultStateID
	if err := s.workflowRepo.Update(ctx, workflow); err != nil {
		return nil, err
	}

	s.logger.InfoContext(ctx, "workflow created", "workflow_id", workflow.ID, "workspace_id", req.WorkspaceID, "name", workflow.Name)
	return s.workflowRepo.GetByID(ctx, workflow.ID)
}

// Update updates a workflow.
func (s *PMWorkflowService) Update(ctx context.Context, id string, req model.UpdateWorkflowRequest) (*model.WorkflowWithStates, error) {
	wf, err := s.workflowRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if wf == nil {
		return nil, fmt.Errorf("workflow not found")
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		wf.Workflow.Name = name
	}
	if req.Description != nil {
		wf.Workflow.Description = req.Description
	}
	if req.TeamID != nil {
		wf.Workflow.TeamID = req.TeamID
	}
	if req.DefaultStateID != nil {
		ok, err := s.workflowRepo.StateBelongsToWorkflow(ctx, *req.DefaultStateID, wf.Workflow.ID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("default_state_id must belong to this workflow")
		}
		wf.Workflow.DefaultStateID = req.DefaultStateID
	}
	if req.AutoAssignOwner != nil {
		wf.Workflow.AutoAssignOwner = *req.AutoAssignOwner
	}

	if err := s.workflowRepo.Update(ctx, &wf.Workflow); err != nil {
		s.logger.ErrorContext(ctx, "failed to update workflow", "error", err, "workflow_id", id)
		return nil, err
	}
	s.logger.InfoContext(ctx, "workflow updated", "workflow_id", id)
	return s.workflowRepo.GetByID(ctx, id)
}

// Delete deletes a workflow.
func (s *PMWorkflowService) Delete(ctx context.Context, id string) error {
	wf, err := s.workflowRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if wf == nil {
		return fmt.Errorf("workflow not found")
	}
	if err := s.workflowRepo.Delete(ctx, id); err != nil {
		s.logger.ErrorContext(ctx, "failed to delete workflow", "error", err, "workflow_id", id)
		return err
	}
	s.logger.InfoContext(ctx, "workflow deleted", "workflow_id", id)
	return nil
}

// CreateState adds a state to a workflow.
func (s *PMWorkflowService) CreateState(ctx context.Context, workflowID string, req model.CreateWorkflowStateRequest) (*model.PMWorkflowState, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	if !isValidStateType(req.StateType) {
		return nil, fmt.Errorf("invalid state_type")
	}

	wf, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	if wf == nil {
		return nil, fmt.Errorf("workflow not found")
	}

	// Temporarily assign end position; normalizePositions will fix it.
	position := len(wf.States)
	if req.Position != nil {
		position = *req.Position
	}

	state := &model.PMWorkflowState{
		WorkflowID:  workflowID,
		Name:        strings.TrimSpace(req.Name),
		StateType:   req.StateType,
		Position:    position,
		Color:       req.Color,
		Description: req.Description,
		WIPLimit:    req.WIPLimit,
		IsDefault:   req.IsDefault,
	}
	if err := s.workflowRepo.CreateState(ctx, state); err != nil {
		s.logger.ErrorContext(ctx, "failed to create workflow state", "error", err, "workflow_id", workflowID)
		return nil, err
	}
	s.logger.InfoContext(ctx, "workflow state created", "state_id", state.ID, "workflow_id", workflowID, "name", state.Name)

	if state.IsDefault {
		wf.Workflow.DefaultStateID = &state.ID
		if err := s.workflowRepo.Update(ctx, &wf.Workflow); err != nil {
			return nil, err
		}
	}

	updated, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	if err := ensureWorkflowStateTypeCoverage(updated.States); err != nil {
		return nil, err
	}

	// Normalize positions so states are ordered by type group, then by position within each group.
	if err := s.normalizePositions(ctx, workflowID, updated.States); err != nil {
		return nil, err
	}

	return state, nil
}

// UpdateState updates a workflow state.
func (s *PMWorkflowService) UpdateState(ctx context.Context, workflowID, stateID string, req model.UpdateWorkflowStateRequest) (*model.PMWorkflowState, error) {
	wf, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	if wf == nil {
		return nil, fmt.Errorf("workflow not found")
	}

	var target *model.PMWorkflowState
	for i := range wf.States {
		if wf.States[i].ID == stateID {
			target = &wf.States[i]
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("state not found")
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		target.Name = name
	}
	if req.StateType != nil {
		if !isValidStateType(*req.StateType) {
			return nil, fmt.Errorf("invalid state_type")
		}
		target.StateType = *req.StateType
	}
	if req.Position != nil {
		target.Position = *req.Position
	}
	if req.Color != nil {
		target.Color = req.Color
	}
	if req.Description != nil {
		target.Description = req.Description
	}
	if req.WIPLimit != nil {
		target.WIPLimit = req.WIPLimit
	}
	if req.IsDefault != nil {
		target.IsDefault = *req.IsDefault
		if *req.IsDefault {
			wf.Workflow.DefaultStateID = &target.ID
		}
	}

	if err := s.workflowRepo.UpdateState(ctx, target); err != nil {
		s.logger.ErrorContext(ctx, "failed to update workflow state", "error", err, "state_id", stateID, "workflow_id", workflowID)
		return nil, err
	}
	s.logger.InfoContext(ctx, "workflow state updated", "state_id", stateID, "workflow_id", workflowID)
	if target.IsDefault {
		if err := s.workflowRepo.Update(ctx, &wf.Workflow); err != nil {
			return nil, err
		}
	}

	updated, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	if err := validateWorkflowStateOrder(updated.States); err != nil {
		return nil, err
	}
	if err := ensureWorkflowStateTypeCoverage(updated.States); err != nil {
		return nil, err
	}
	for _, state := range updated.States {
		if state.ID == stateID {
			return &state, nil
		}
	}
	return nil, fmt.Errorf("state not found")
}

// DeleteState deletes a state if no active stories and workflow remains valid.
func (s *PMWorkflowService) DeleteState(ctx context.Context, workflowID, stateID string) error {
	wf, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return err
	}
	if wf == nil {
		return fmt.Errorf("workflow not found")
	}

	count, err := s.storyRepo.CountByWorkflowState(ctx, stateID)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("cannot delete state with active stories")
	}

	remaining := make([]model.PMWorkflowState, 0, len(wf.States)-1)
	for _, state := range wf.States {
		if state.ID != stateID {
			remaining = append(remaining, state)
		}
	}
	if len(remaining) == len(wf.States) {
		return fmt.Errorf("state not found")
	}
	if err := ensureWorkflowStateTypeCoverage(remaining); err != nil {
		return err
	}

	if err := s.workflowRepo.DeleteState(ctx, stateID); err != nil {
		return err
	}
	if wf.Workflow.DefaultStateID != nil && *wf.Workflow.DefaultStateID == stateID {
		newDefault := remaining[0].ID
		wf.Workflow.DefaultStateID = &newDefault
		if err := s.workflowRepo.Update(ctx, &wf.Workflow); err != nil {
			return err
		}
	}
	return nil
}

// ReorderStates reorders workflow states with ordering validation.
func (s *PMWorkflowService) ReorderStates(ctx context.Context, workflowID string, req model.ReorderStatesRequest) error {
	if len(req.StateIDs) == 0 {
		return fmt.Errorf("state_ids is required")
	}
	wf, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return err
	}
	if wf == nil {
		return fmt.Errorf("workflow not found")
	}

	stateByID := make(map[string]model.PMWorkflowState, len(wf.States))
	for _, state := range wf.States {
		stateByID[state.ID] = state
	}
	reordered := make([]model.PMWorkflowState, 0, len(req.StateIDs))
	for idx, id := range req.StateIDs {
		state, ok := stateByID[id]
		if !ok {
			return fmt.Errorf("state %s does not belong to workflow", id)
		}
		state.Position = idx
		reordered = append(reordered, state)
	}
	if len(reordered) != len(wf.States) {
		return fmt.Errorf("state_ids must include all workflow states")
	}

	if err := validateWorkflowStateOrder(reordered); err != nil {
		return err
	}
	if err := s.workflowRepo.ReorderStates(ctx, workflowID, req.StateIDs); err != nil {
		return err
	}
	return nil
}

// SeedWorkspaceDefaults seeds default workflow, epic states, and labels.
func (s *PMWorkflowService) SeedWorkspaceDefaults(ctx context.Context, workspaceID, actorID string) error {
	if workspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if _, err := s.workflowRepo.SeedDefaultWorkflow(ctx, workspaceID); err != nil {
		return err
	}
	if _, err := s.workflowRepo.SeedDefaultEpicStates(ctx, workspaceID); err != nil {
		return err
	}
	if err := s.seedDefaultLabels(ctx, workspaceID); err != nil {
		return err
	}
	return nil
}

func (s *PMWorkflowService) seedDefaultLabels(ctx context.Context, workspaceID string) error {
	defaults := []struct {
		Name  string
		Color string
	}{
		{Name: "frontend", Color: "#3b82f6"},
		{Name: "backend", Color: "#16a34a"},
		{Name: "design", Color: "#ec4899"},
		{Name: "infrastructure", Color: "#64748b"},
	}

	for _, def := range defaults {
		existing, err := s.labelRepo.GetByName(ctx, workspaceID, nil, def.Name)
		if err != nil {
			return err
		}
		if existing != nil {
			continue
		}
		color := def.Color
		label := &model.PMLabel{WorkspaceID: workspaceID, Name: def.Name, Color: &color}
		if err := s.labelRepo.Create(ctx, label); err != nil {
			return err
		}
	}
	return nil
}

// normalizePositions reorders all states so they are grouped by type
// (backlog → unstarted → started → done), preserving relative order within each group,
// then persists the new positions via ReorderStates.
func (s *PMWorkflowService) normalizePositions(ctx context.Context, workflowID string, states []model.PMWorkflowState) error {
	typeRank := map[string]int{
		model.PMStateTypeBacklog:   0,
		model.PMStateTypeUnstarted: 1,
		model.PMStateTypeStarted:   2,
		model.PMStateTypeDone:      3,
	}

	sorted := make([]model.PMWorkflowState, len(states))
	copy(sorted, states)
	sort.SliceStable(sorted, func(i, j int) bool {
		ri := typeRank[sorted[i].StateType]
		rj := typeRank[sorted[j].StateType]
		if ri != rj {
			return ri < rj
		}
		return sorted[i].Position < sorted[j].Position
	})

	ids := make([]string, len(sorted))
	for i, state := range sorted {
		ids[i] = state.ID
	}
	return s.workflowRepo.ReorderStates(ctx, workflowID, ids)
}

func isValidStateType(stateType string) bool {
	switch stateType {
	case model.PMStateTypeBacklog, model.PMStateTypeUnstarted, model.PMStateTypeStarted, model.PMStateTypeDone:
		return true
	default:
		return false
	}
}

func validateWorkflowStateOrder(states []model.PMWorkflowState) error {
	typeRank := map[string]int{
		model.PMStateTypeBacklog:   0,
		model.PMStateTypeUnstarted: 1,
		model.PMStateTypeStarted:   2,
		model.PMStateTypeDone:      3,
	}

	sorted := make([]model.PMWorkflowState, len(states))
	copy(sorted, states)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Position < sorted[j].Position
	})

	prev := -1
	for _, state := range sorted {
		rank, ok := typeRank[state.StateType]
		if !ok {
			return fmt.Errorf("invalid state_type %s", state.StateType)
		}
		if rank < prev {
			return fmt.Errorf("state type ordering must be backlog < unstarted < started < done")
		}
		prev = rank
	}
	return nil
}

func ensureWorkflowStateTypeCoverage(states []model.PMWorkflowState) error {
	type hasType struct {
		Backlog   bool
		Unstarted bool
		Started   bool
		Done      bool
	}
	flags := hasType{}
	for _, state := range states {
		switch state.StateType {
		case model.PMStateTypeBacklog:
			flags.Backlog = true
		case model.PMStateTypeUnstarted:
			flags.Unstarted = true
		case model.PMStateTypeStarted:
			flags.Started = true
		case model.PMStateTypeDone:
			flags.Done = true
		}
	}
	if !flags.Backlog || !flags.Unstarted || !flags.Started || !flags.Done {
		return fmt.Errorf("workflow must include at least one backlog, unstarted, started, and done state")
	}
	return nil
}
