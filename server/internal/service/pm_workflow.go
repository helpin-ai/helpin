package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// PMWorkflowService contains workflow business logic.
type PMWorkflowService struct {
	workflowRepo *repository.PMWorkflowRepository
	storyRepo    *repository.PMTaskRepository
	labelRepo    *repository.PMLabelRepository
	wsPublisher  *websocket.Publisher
	logger       *slog.Logger
}

// NewPMWorkflowService creates a new PMWorkflowService.
func NewPMWorkflowService(workflowRepo *repository.PMWorkflowRepository, storyRepo *repository.PMTaskRepository, labelRepo *repository.PMLabelRepository, wsPublisher *websocket.Publisher) *PMWorkflowService {
	return &PMWorkflowService{
		workflowRepo: workflowRepo,
		storyRepo:    storyRepo,
		labelRepo:    labelRepo,
		wsPublisher:  wsPublisher,
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
	publishWorkspaceEvent(s.wsPublisher, "created", "workflow", workflow.ID, req.WorkspaceID, "")
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
	publishWorkspaceEvent(s.wsPublisher, "updated", "workflow", id, wf.Workflow.WorkspaceID, "")
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
	all, err := s.workflowRepo.ListByWorkspace(ctx, wf.Workflow.WorkspaceID)
	if err != nil {
		return err
	}
	if len(all) <= 1 {
		return fmt.Errorf("cannot delete the last workflow")
	}
	var totalStories int64
	for _, state := range wf.States {
		count, err := s.storyRepo.CountByWorkflowState(ctx, state.ID)
		if err != nil {
			return err
		}
		totalStories += count
	}
	if totalStories > 0 {
		return fmt.Errorf("cannot delete workflow with %d active stories — move or archive them first", totalStories)
	}
	if err := s.workflowRepo.Delete(ctx, id); err != nil {
		s.logger.ErrorContext(ctx, "failed to delete workflow", "error", err, "workflow_id", id)
		return err
	}
	s.logger.InfoContext(ctx, "workflow deleted", "workflow_id", id)
	publishWorkspaceEvent(s.wsPublisher, "deleted", "workflow", id, wf.Workflow.WorkspaceID, "")
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
	if state.IsDefault {
		if err := s.workflowRepo.ClearDefaultStates(ctx, workflowID); err != nil {
			return nil, err
		}
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
	publishWorkspaceEvent(s.wsPublisher, "updated", "workflow", workflowID, wf.Workflow.WorkspaceID, "")

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
			if err := s.workflowRepo.ClearDefaultStates(ctx, workflowID); err != nil {
				return nil, err
			}
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
			publishWorkspaceEvent(s.wsPublisher, "updated", "workflow", workflowID, wf.Workflow.WorkspaceID, "")
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
	publishWorkspaceEvent(s.wsPublisher, "updated", "workflow", workflowID, wf.Workflow.WorkspaceID, "")
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
	publishWorkspaceEvent(s.wsPublisher, "updated", "workflow", workflowID, wf.Workflow.WorkspaceID, "")
	return nil
}

// ResolveTeamWorkflow returns the workflow for a team, falling back to the workspace default.
func (s *PMWorkflowService) ResolveTeamWorkflow(ctx context.Context, workspaceID, teamID string) (*model.WorkflowWithStates, error) {
	if workspaceID == "" || teamID == "" {
		return nil, fmt.Errorf("workspace_id and team_id are required")
	}

	// Try team-specific workflow first.
	wf, err := s.workflowRepo.GetByTeamID(ctx, workspaceID, teamID)
	if err != nil {
		return nil, err
	}
	if wf != nil {
		s.logger.DebugContext(ctx, "resolved team workflow", "team_id", teamID, "workflow_id", wf.Workflow.ID)
		return wf, nil
	}

	// No team workflow found — auto-seed one from the workspace default.
	s.logger.InfoContext(ctx, "auto-seeding workflow for team", "team_id", teamID, "workspace_id", workspaceID)
	if err := s.SeedTeamWorkflow(ctx, workspaceID, teamID, "Team"); err != nil {
		return nil, fmt.Errorf("auto-seed team workflow: %w", err)
	}

	// Re-fetch the newly created team workflow.
	wf, err = s.workflowRepo.GetByTeamID(ctx, workspaceID, teamID)
	if err != nil {
		return nil, err
	}
	if wf == nil {
		return nil, fmt.Errorf("team workflow not found after seeding")
	}
	s.logger.InfoContext(ctx, "resolved auto-seeded team workflow", "team_id", teamID, "workflow_id", wf.Workflow.ID)
	return wf, nil
}

// CopyToTeam copies a source workflow to a target team.
func (s *PMWorkflowService) CopyToTeam(ctx context.Context, sourceWorkflowID, targetTeamID, workspaceID string) (*model.WorkflowWithStates, error) {
	if sourceWorkflowID == "" || targetTeamID == "" || workspaceID == "" {
		return nil, fmt.Errorf("source_workflow_id, target_team_id, and workspace_id are required")
	}

	// Validate source workflow exists.
	source, err := s.workflowRepo.GetByID(ctx, sourceWorkflowID)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, fmt.Errorf("source workflow not found")
	}

	// Check target team doesn't already have a workflow.
	existing, err := s.workflowRepo.GetByTeamID(ctx, workspaceID, targetTeamID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("team already has a workflow")
	}

	newName := source.Workflow.Name + " (Copy)"
	result, err := s.workflowRepo.CopyWorkflow(ctx, source, newName, &targetTeamID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to copy workflow to team", "error", err, "source_workflow_id", sourceWorkflowID, "team_id", targetTeamID)
		return nil, err
	}
	s.logger.InfoContext(ctx, "workflow copied to team", "source_workflow_id", sourceWorkflowID, "new_workflow_id", result.Workflow.ID, "team_id", targetTeamID)

	// Migrate existing team stories from source workflow to new workflow.
	s.migrateTeamStories(ctx, targetTeamID, source, result)

	return result, nil
}

// SeedTeamWorkflow creates a default workflow for a newly created team.
func (s *PMWorkflowService) SeedTeamWorkflow(ctx context.Context, workspaceID, teamID, teamName string) error {
	// Check if team already has a workflow.
	existing, err := s.workflowRepo.GetByTeamID(ctx, workspaceID, teamID)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	// Try to copy from the workspace default workflow.
	defaultWf, err := s.workflowRepo.GetDefaultWorkflow(ctx, workspaceID)
	if err != nil {
		return err
	}
	if defaultWf != nil && len(defaultWf.States) > 0 {
		name := teamName + " Workflow"
		newWf, err := s.workflowRepo.CopyWorkflow(ctx, defaultWf, name, &teamID)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to seed team workflow from default", "error", err, "team_id", teamID)
			return err
		}
		s.logger.InfoContext(ctx, "team workflow seeded from default", "team_id", teamID, "workspace_id", workspaceID)

		// Migrate existing team stories from old workflow to new workflow.
		s.migrateTeamStories(ctx, teamID, defaultWf, newWf)
		return nil
	}

	// No default workflow exists; create a fresh one.
	_, err = s.Create(ctx, model.CreateWorkflowRequest{
		WorkspaceID: workspaceID,
		Name:        teamName + " Workflow",
		TeamID:      &teamID,
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to seed fresh team workflow", "error", err, "team_id", teamID)
		return err
	}
	s.logger.InfoContext(ctx, "team workflow seeded with defaults", "team_id", teamID, "workspace_id", workspaceID)
	return nil
}

// migrateTeamStories remaps stories from an old workflow to a new one by matching states.
func (s *PMWorkflowService) migrateTeamStories(ctx context.Context, teamID string, oldWf, newWf *model.WorkflowWithStates) {
	stateMap := buildStateMapping(oldWf.States, newWf.States, newWf.Workflow.DefaultStateID)
	migrated, err := s.storyRepo.MigrateStoriesToWorkflow(ctx, teamID, oldWf.Workflow.ID, newWf.Workflow.ID, stateMap)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to migrate team stories to new workflow",
			"error", err, "team_id", teamID,
			"old_workflow_id", oldWf.Workflow.ID, "new_workflow_id", newWf.Workflow.ID)
		return
	}
	if migrated > 0 {
		s.logger.InfoContext(ctx, "migrated team stories to new workflow",
			"team_id", teamID, "count", migrated,
			"old_workflow_id", oldWf.Workflow.ID, "new_workflow_id", newWf.Workflow.ID)
	}
}

// buildStateMapping creates a mapping from old state IDs to new state IDs,
// matching by (state_type, position) first, then state_type only, then default.
func buildStateMapping(oldStates, newStates []model.PMWorkflowState, defaultStateID *string) map[string]string {
	byTypeAndPos := map[string]string{}
	byType := map[string]string{}
	for _, ns := range newStates {
		key := fmt.Sprintf("%s:%d", ns.StateType, ns.Position)
		byTypeAndPos[key] = ns.ID
		if _, exists := byType[ns.StateType]; !exists {
			byType[ns.StateType] = ns.ID
		}
	}

	fallback := newStates[0].ID
	if defaultStateID != nil {
		fallback = *defaultStateID
	}

	result := make(map[string]string, len(oldStates))
	for _, os := range oldStates {
		key := fmt.Sprintf("%s:%d", os.StateType, os.Position)
		if newID, ok := byTypeAndPos[key]; ok {
			result[os.ID] = newID
		} else if newID, ok := byType[os.StateType]; ok {
			result[os.ID] = newID
		} else {
			result[os.ID] = fallback
		}
	}
	return result
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
