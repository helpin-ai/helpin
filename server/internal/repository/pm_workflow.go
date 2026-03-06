package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PMWorkflowRepository handles DB operations for workflows and workflow states.
type PMWorkflowRepository struct {
	db *gorm.DB
}

// NewPMWorkflowRepository creates a new PMWorkflowRepository.
func NewPMWorkflowRepository(db *gorm.DB) *PMWorkflowRepository {
	return &PMWorkflowRepository{db: db}
}

// ListByWorkspace returns all workflows in a workspace, each with its states.
func (r *PMWorkflowRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.WorkflowWithStates, error) {
	var workflows []model.PMWorkflow
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("created_at ASC").
		Find(&workflows).Error; err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}

	result := make([]model.WorkflowWithStates, 0, len(workflows))
	for _, wf := range workflows {
		states, err := r.listStates(ctx, wf.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, model.WorkflowWithStates{
			Workflow: wf,
			States:   states,
		})
	}
	return result, nil
}

// GetByID returns a workflow and its states.
func (r *PMWorkflowRepository) GetByID(ctx context.Context, id string) (*model.WorkflowWithStates, error) {
	var wf model.PMWorkflow
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&wf).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get workflow: %w", err)
	}

	states, err := r.listStates(ctx, wf.ID)
	if err != nil {
		return nil, err
	}
	return &model.WorkflowWithStates{Workflow: wf, States: states}, nil
}

// GetByTeamID returns a team-specific workflow with states.
func (r *PMWorkflowRepository) GetByTeamID(ctx context.Context, teamID string) (*model.WorkflowWithStates, error) {
	var wf model.PMWorkflow
	if err := r.db.WithContext(ctx).
		Where("team_id = ?", teamID).
		Order("created_at ASC").
		First(&wf).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get workflow by team: %w", err)
	}

	states, err := r.listStates(ctx, wf.ID)
	if err != nil {
		return nil, err
	}
	return &model.WorkflowWithStates{Workflow: wf, States: states}, nil
}

// Create inserts a workflow.
func (r *PMWorkflowRepository) Create(ctx context.Context, workflow *model.PMWorkflow) error {
	if err := r.db.WithContext(ctx).Create(workflow).Error; err != nil {
		return fmt.Errorf("create workflow: %w", err)
	}
	return nil
}

// Update updates a workflow.
func (r *PMWorkflowRepository) Update(ctx context.Context, workflow *model.PMWorkflow) error {
	if err := r.db.WithContext(ctx).Save(workflow).Error; err != nil {
		return fmt.Errorf("update workflow: %w", err)
	}
	return nil
}

// Delete removes a workflow.
func (r *PMWorkflowRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&model.PMWorkflow{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete workflow: %w", err)
	}
	return nil
}

// CreateState inserts a workflow state.
func (r *PMWorkflowRepository) CreateState(ctx context.Context, state *model.PMWorkflowState) error {
	if err := r.db.WithContext(ctx).Create(state).Error; err != nil {
		return fmt.Errorf("create workflow state: %w", err)
	}
	return nil
}

// UpdateState updates a workflow state.
func (r *PMWorkflowRepository) UpdateState(ctx context.Context, state *model.PMWorkflowState) error {
	if err := r.db.WithContext(ctx).Save(state).Error; err != nil {
		return fmt.Errorf("update workflow state: %w", err)
	}
	return nil
}

// DeleteState removes a workflow state.
func (r *PMWorkflowRepository) DeleteState(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&model.PMWorkflowState{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete workflow state: %w", err)
	}
	return nil
}

// ReorderStates updates state positions in order.
func (r *PMWorkflowRepository) ReorderStates(ctx context.Context, workflowID string, stateIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for idx, stateID := range stateIDs {
			if err := tx.Model(&model.PMWorkflowState{}).
				Where("id = ? AND workflow_id = ?", stateID, workflowID).
				Update("position", idx).Error; err != nil {
				return fmt.Errorf("reorder states: %w", err)
			}
		}
		return nil
	})
}

// GetDefaultWorkflow returns the default/general workflow for a workspace.
func (r *PMWorkflowRepository) GetDefaultWorkflow(ctx context.Context, workspaceID string) (*model.WorkflowWithStates, error) {
	var wf model.PMWorkflow
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("CASE WHEN team_id IS NULL THEN 0 ELSE 1 END, created_at ASC").
		First(&wf).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get default workflow: %w", err)
	}
	states, err := r.listStates(ctx, wf.ID)
	if err != nil {
		return nil, err
	}
	return &model.WorkflowWithStates{Workflow: wf, States: states}, nil
}

// SeedDefaultWorkflow creates a default workflow and default states if missing.
func (r *PMWorkflowRepository) SeedDefaultWorkflow(ctx context.Context, workspaceID string) (*model.WorkflowWithStates, error) {
	existing, err := r.GetDefaultWorkflow(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if existing != nil && len(existing.States) > 0 {
		return existing, nil
	}

	var seeded *model.WorkflowWithStates
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		wf := model.PMWorkflow{
			WorkspaceID: workspaceID,
			Name:        "Default Workflow",
		}
		if err := tx.Create(&wf).Error; err != nil {
			return fmt.Errorf("seed workflow: %w", err)
		}

		defaultColor := func(c string) *string { return &c }
		states := []model.PMWorkflowState{
			{WorkflowID: wf.ID, Name: "Backlog", StateType: model.PMStateTypeBacklog, Position: 0, IsDefault: false, Color: defaultColor("#9ca3af")},
			{WorkflowID: wf.ID, Name: "Ready for Dev", StateType: model.PMStateTypeUnstarted, Position: 1, IsDefault: true, Color: defaultColor("#f59e0b")},
			{WorkflowID: wf.ID, Name: "In Development", StateType: model.PMStateTypeStarted, Position: 2, IsDefault: false, Color: defaultColor("#3b82f6")},
			{WorkflowID: wf.ID, Name: "In Review", StateType: model.PMStateTypeStarted, Position: 3, IsDefault: false, Color: defaultColor("#8b5cf6")},
			{WorkflowID: wf.ID, Name: "Done", StateType: model.PMStateTypeDone, Position: 4, IsDefault: false, Color: defaultColor("#22c55e")},
		}
		if err := tx.Create(&states).Error; err != nil {
			return fmt.Errorf("seed workflow states: %w", err)
		}

		defaultStateID := states[1].ID
		if err := tx.Model(&model.PMWorkflow{}).
			Where("id = ?", wf.ID).
			Update("default_state_id", defaultStateID).Error; err != nil {
			return fmt.Errorf("seed workflow default state: %w", err)
		}

		wf.DefaultStateID = &defaultStateID
		seeded = &model.WorkflowWithStates{Workflow: wf, States: states}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return seeded, nil
}

// ListEpicStates lists epic workflow states for a workspace.
func (r *PMWorkflowRepository) ListEpicStates(ctx context.Context, workspaceID string) ([]model.PMEpicWorkflowState, error) {
	var states []model.PMEpicWorkflowState
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("position ASC").
		Find(&states).Error; err != nil {
		return nil, fmt.Errorf("list epic workflow states: %w", err)
	}
	return states, nil
}

// SeedDefaultEpicStates creates default epic states when missing.
func (r *PMWorkflowRepository) SeedDefaultEpicStates(ctx context.Context, workspaceID string) ([]model.PMEpicWorkflowState, error) {
	existing, err := r.ListEpicStates(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		return existing, nil
	}

	color := func(c string) *string { return &c }
	states := []model.PMEpicWorkflowState{
		{WorkspaceID: workspaceID, Name: "To Do", StateType: model.PMStateTypeUnstarted, Position: 0, Color: color("#9ca3af"), IsDefault: true},
		{WorkspaceID: workspaceID, Name: "In Progress", StateType: model.PMStateTypeStarted, Position: 1, Color: color("#3b82f6"), IsDefault: false},
		{WorkspaceID: workspaceID, Name: "Done", StateType: model.PMStateTypeDone, Position: 2, Color: color("#22c55e"), IsDefault: false},
	}
	if err := r.db.WithContext(ctx).Create(&states).Error; err != nil {
		return nil, fmt.Errorf("seed default epic states: %w", err)
	}
	return states, nil
}

// GetStateByID returns a workflow state by ID.
func (r *PMWorkflowRepository) GetStateByID(ctx context.Context, id string) (*model.PMWorkflowState, error) {
	var state model.PMWorkflowState
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&state).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get workflow state: %w", err)
	}
	return &state, nil
}

// StateBelongsToWorkflow checks if state is part of a workflow.
func (r *PMWorkflowRepository) StateBelongsToWorkflow(ctx context.Context, stateID, workflowID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.PMWorkflowState{}).
		Where("id = ? AND workflow_id = ?", stateID, workflowID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check workflow state ownership: %w", err)
	}
	return count > 0, nil
}

func (r *PMWorkflowRepository) listStates(ctx context.Context, workflowID string) ([]model.PMWorkflowState, error) {
	var states []model.PMWorkflowState
	if err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Order("CASE state_type WHEN 'backlog' THEN 0 WHEN 'unstarted' THEN 1 WHEN 'started' THEN 2 WHEN 'done' THEN 3 ELSE 4 END, position ASC").
		Find(&states).Error; err != nil {
		return nil, fmt.Errorf("list workflow states: %w", err)
	}
	return states, nil
}
