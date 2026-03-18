package repository

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

type FlowRepository struct {
	db *gorm.DB
}

func NewFlowRepository(db *gorm.DB) *FlowRepository {
	return &FlowRepository{db: db}
}

func (r *FlowRepository) CreateRun(ctx context.Context, run *model.FlowRun) error {
	if err := r.db.WithContext(ctx).Create(run).Error; err != nil {
		return fmt.Errorf("create flow run: %w", err)
	}
	return nil
}

func (r *FlowRepository) UpdateRun(ctx context.Context, run *model.FlowRun) error {
	if err := r.db.WithContext(ctx).Save(run).Error; err != nil {
		return fmt.Errorf("update flow run: %w", err)
	}
	return nil
}

func (r *FlowRepository) GetRunByID(ctx context.Context, workspaceID, id string) (*model.FlowRun, error) {
	var run model.FlowRun
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&run).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get flow run: %w", err)
	}
	return &run, nil
}

func (r *FlowRepository) GetRunByIDAny(ctx context.Context, id string) (*model.FlowRun, error) {
	var run model.FlowRun
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&run).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get flow run by id: %w", err)
	}
	return &run, nil
}

func (r *FlowRepository) GetActiveRunByTarget(ctx context.Context, workspaceID, templateID, targetType, targetID string) (*model.FlowRun, error) {
	var run model.FlowRun
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND template_id = ? AND target_type = ? AND target_id = ? AND status IN ?", workspaceID, templateID, targetType, targetID,
			[]string{model.FlowStatusRunning, model.FlowStatusAwaitingApproval, model.FlowStatusAwaitingInput}).
		Order("created_at DESC").
		First(&run).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get active flow run by target: %w", err)
	}
	return &run, nil
}

func (r *FlowRepository) ListRuns(ctx context.Context, workspaceID string, limit, offset int) ([]model.FlowRun, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.FlowRun{}).Where("workspace_id = ?", workspaceID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count flow runs: %w", err)
	}
	var items []model.FlowRun
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list flow runs: %w", err)
	}
	return items, total, nil
}

func (r *FlowRepository) CreateNodeRun(ctx context.Context, nodeRun *model.FlowNodeRun) error {
	if err := r.db.WithContext(ctx).Create(nodeRun).Error; err != nil {
		return fmt.Errorf("create flow node run: %w", err)
	}
	return nil
}

func (r *FlowRepository) UpdateNodeRun(ctx context.Context, nodeRun *model.FlowNodeRun) error {
	if err := r.db.WithContext(ctx).Save(nodeRun).Error; err != nil {
		return fmt.Errorf("update flow node run: %w", err)
	}
	return nil
}

func (r *FlowRepository) ListNodeRuns(ctx context.Context, flowRunID string) ([]model.FlowNodeRun, error) {
	var items []model.FlowNodeRun
	if err := r.db.WithContext(ctx).
		Where("flow_run_id = ?", flowRunID).
		Order("created_at ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list flow node runs: %w", err)
	}
	return items, nil
}

func (r *FlowRepository) GetNodeRunByID(ctx context.Context, flowRunID, nodeRunID string) (*model.FlowNodeRun, error) {
	var nodeRun model.FlowNodeRun
	if err := r.db.WithContext(ctx).Where("flow_run_id = ? AND id = ?", flowRunID, nodeRunID).First(&nodeRun).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get flow node run: %w", err)
	}
	return &nodeRun, nil
}

func (r *FlowRepository) GetNodeRunByIDAny(ctx context.Context, nodeRunID string) (*model.FlowNodeRun, error) {
	var nodeRun model.FlowNodeRun
	if err := r.db.WithContext(ctx).Where("id = ?", nodeRunID).First(&nodeRun).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get flow node run by id: %w", err)
	}
	return &nodeRun, nil
}

func (r *FlowRepository) GetLatestNodeRunByNodeID(ctx context.Context, flowRunID, nodeID string) (*model.FlowNodeRun, error) {
	var nodeRun model.FlowNodeRun
	if err := r.db.WithContext(ctx).
		Where("flow_run_id = ? AND node_id = ?", flowRunID, nodeID).
		Order("created_at DESC").
		First(&nodeRun).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest node run: %w", err)
	}
	return &nodeRun, nil
}

func (r *FlowRepository) CreateTrigger(ctx context.Context, trigger *model.FlowTrigger) error {
	if err := r.db.WithContext(ctx).Create(trigger).Error; err != nil {
		return fmt.Errorf("create flow trigger: %w", err)
	}
	return nil
}
