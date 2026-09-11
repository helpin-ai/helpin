package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMPipelineUpdateOptions controls atomic stage edits and stale-write protection.
type CRMPipelineUpdateOptions struct {
	Stages            []model.CRMPipelineStage
	StageMigrations   map[string]string
	ExpectedUpdatedAt *time.Time
}

// ListPipelines returns workspace pipelines with stage and pipeline deal counts.
func (r *CRMDealRepository) ListPipelines(ctx context.Context, workspaceID string) ([]model.CRMPipeline, error) {
	var pipelines []model.CRMPipeline
	if err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Preload("Stages", func(db *gorm.DB) *gorm.DB { return db.Order("position ASC").Order("id ASC") }).Order("position ASC").Find(&pipelines).Error; err != nil {
		return nil, fmt.Errorf("list pipelines: %w", err)
	}
	if err := populatePipelineDealCounts(r.db.WithContext(ctx), pipelines); err != nil {
		return nil, err
	}
	return pipelines, nil
}

// GetPipeline returns a pipeline with stages and deal counts, or nil if absent.
func (r *CRMDealRepository) GetPipeline(ctx context.Context, id string) (*model.CRMPipeline, error) {
	var pipeline model.CRMPipeline
	if err := r.db.WithContext(ctx).Where("id = ?", id).Preload("Stages", func(db *gorm.DB) *gorm.DB { return db.Order("position ASC").Order("id ASC") }).First(&pipeline).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get pipeline: %w", err)
	}
	rows := []model.CRMPipeline{pipeline}
	if err := populatePipelineDealCounts(r.db.WithContext(ctx), rows); err != nil {
		return nil, err
	}
	return &rows[0], nil
}

// CreatePipeline inserts stages and establishes the workspace default atomically.
func (r *CRMDealRepository) CreatePipeline(ctx context.Context, pipeline *model.CRMPipeline) error {
	if err := model.ValidateCRMPipelineStages(pipeline.Stages); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockPipelineWorkspace(tx, pipeline.WorkspaceID); err != nil {
			return err
		}
		var defaults int64
		if err := tx.Model(&model.CRMPipeline{}).Where("workspace_id = ? AND is_default = ?", pipeline.WorkspaceID, true).Count(&defaults).Error; err != nil {
			return err
		}
		if defaults == 0 {
			pipeline.IsDefault = true
		}
		if pipeline.IsDefault {
			if err := clearOtherPipelineDefaults(tx, pipeline.WorkspaceID, pipeline.ID); err != nil {
				return err
			}
		}
		if err := tx.Create(pipeline).Error; err != nil {
			return fmt.Errorf("create pipeline: %w", err)
		}
		return nil
	})
}

// UpdatePipeline saves metadata and applies an in-place stage diff in one transaction.
// Existing callers without options update metadata only.
func (r *CRMDealRepository) UpdatePipeline(ctx context.Context, pipeline *model.CRMPipeline, options ...CRMPipelineUpdateOptions) error {
	var opts CRMPipelineUpdateOptions
	if len(options) > 0 {
		opts = options[0]
	}
	if opts.ExpectedUpdatedAt == nil {
		expected := pipeline.UpdatedAt
		opts.ExpectedUpdatedAt = &expected
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockPipelineWorkspace(tx, pipeline.WorkspaceID); err != nil {
			return err
		}
		var current model.CRMPipeline
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND workspace_id = ?", pipeline.ID, pipeline.WorkspaceID).First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.ErrCRMPipelineNotFound
			}
			return err
		}
		if !current.UpdatedAt.Equal(*opts.ExpectedUpdatedAt) {
			return model.ErrCRMPipelineConflict
		}
		if current.IsDefault && !pipeline.IsDefault {
			if err := requireOtherPipelineDefault(tx, current); err != nil {
				return err
			}
		}
		if opts.Stages != nil {
			if err := updatePipelineStages(tx, current.ID, opts.Stages, opts.StageMigrations); err != nil {
				return err
			}
		} else if len(opts.StageMigrations) > 0 {
			return &model.CRMPipelineValidationError{Message: "include the updated stages when moving deals"}
		}
		if pipeline.IsDefault {
			if err := clearOtherPipelineDefaults(tx, current.WorkspaceID, current.ID); err != nil {
				return err
			}
		}
		// Omit associations: saving preloaded stages would overwrite edits made above.
		return tx.Model(&model.CRMPipeline{}).Where("id = ?", current.ID).Updates(map[string]interface{}{
			"name": pipeline.Name, "is_default": pipeline.IsDefault, "default_commercial_motion": pipeline.DefaultCommercialMotion, "position": pipeline.Position,
		}).Error
	})
}

// ReplaceStages applies an in-place diff, retaining stage IDs and deal references.
func (r *CRMDealRepository) ReplaceStages(ctx context.Context, pipelineID string, stages []model.CRMPipelineStage) error {
	pipeline, err := r.GetPipeline(ctx, pipelineID)
	if err != nil {
		return err
	}
	if pipeline == nil {
		return model.ErrCRMPipelineNotFound
	}
	if stages == nil {
		stages = []model.CRMPipelineStage{}
	}
	return r.UpdatePipeline(ctx, pipeline, CRMPipelineUpdateOptions{Stages: stages})
}

// DeletePipeline removes an empty pipeline after protecting the workspace default.
func (r *CRMDealRepository) DeletePipeline(ctx context.Context, id string) error {
	pipeline, err := r.GetPipeline(ctx, id)
	if err != nil {
		return err
	}
	if pipeline == nil {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockPipelineWorkspace(tx, pipeline.WorkspaceID); err != nil {
			return err
		}
		var current model.CRMPipeline
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&current).Error; err != nil {
			return err
		}
		if current.IsDefault {
			if err := requireOtherPipelineDefault(tx, current); err != nil {
				return err
			}
		}
		var stages []model.CRMPipelineStage
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("pipeline_id = ?", id).Order("id ASC").Find(&stages).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&model.CRMDeal{}).Where("pipeline_id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return &model.CRMPipelineValidationError{Message: "move all deals to another pipeline before deleting this pipeline"}
		}
		if err := tx.Where("pipeline_id = ?", id).Delete(&model.CRMPipelineStage{}).Error; err != nil {
			return fmt.Errorf("delete pipeline stages: %w", err)
		}
		return tx.Where("id = ?", id).Delete(&model.CRMPipeline{}).Error
	})
}

func updatePipelineStages(tx *gorm.DB, pipelineID string, stages []model.CRMPipelineStage, migrations map[string]string) error {
	var existing []model.CRMPipelineStage
	// Lock before counting so a concurrent deal insert cannot slip past deletion checks.
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("pipeline_id = ?", pipelineID).Order("id ASC").Find(&existing).Error; err != nil {
		return err
	}
	rows := []model.CRMPipeline{{ID: pipelineID, Stages: existing}}
	if err := populatePipelineDealCounts(tx, rows); err != nil {
		return err
	}
	if err := model.ValidateCRMPipelineStageChanges(existing, stages, migrations); err != nil {
		return err
	}
	old := make(map[string]model.CRMPipelineStage, len(existing))
	retained := make(map[string]bool, len(stages))
	for _, stage := range existing {
		old[stage.ID] = stage
	}
	for i := range stages {
		stage := &stages[i]
		stage.PipelineID = pipelineID
		if stage.ID == "" {
			if err := tx.Create(stage).Error; err != nil {
				return fmt.Errorf("create pipeline stage: %w", err)
			}
			continue
		}
		retained[stage.ID] = true
		previous := old[stage.ID]
		if previous.Name == stage.Name && previous.StageType == stage.StageType && previous.Position == stage.Position && previous.Probability == stage.Probability {
			continue
		}
		if err := tx.Model(&model.CRMPipelineStage{}).Where("id = ? AND pipeline_id = ?", stage.ID, pipelineID).Updates(map[string]interface{}{
			"name": stage.Name, "stage_type": stage.StageType, "position": stage.Position, "probability": stage.Probability,
		}).Error; err != nil {
			return fmt.Errorf("update pipeline stage: %w", err)
		}
	}
	for _, stage := range existing {
		if retained[stage.ID] {
			continue
		}
		if destination := migrations[stage.ID]; destination != "" {
			if err := tx.Model(&model.CRMDeal{}).Where("pipeline_id = ? AND stage_id = ?", pipelineID, stage.ID).Updates(map[string]interface{}{"stage_id": destination, "updated_at": time.Now().UTC()}).Error; err != nil {
				return fmt.Errorf("move pipeline stage deals: %w", err)
			}
		}
		if err := tx.Where("id = ? AND pipeline_id = ?", stage.ID, pipelineID).Delete(&model.CRMPipelineStage{}).Error; err != nil {
			return fmt.Errorf("delete pipeline stage: %w", err)
		}
	}
	return nil
}

func populatePipelineDealCounts(db *gorm.DB, pipelines []model.CRMPipeline) error {
	if len(pipelines) == 0 {
		return nil
	}
	ids := make([]string, 0, len(pipelines))
	for _, pipeline := range pipelines {
		ids = append(ids, pipeline.ID)
	}
	var rows []struct {
		PipelineID string
		StageID    string
		DealCount  int64
	}
	if err := db.Model(&model.CRMDeal{}).Select("pipeline_id, stage_id, COUNT(*) AS deal_count").Where("pipeline_id IN ?", ids).Group("pipeline_id, stage_id").Scan(&rows).Error; err != nil {
		return fmt.Errorf("count pipeline stage deals: %w", err)
	}
	stageCounts := make(map[string]int64, len(rows))
	pipelineCounts := make(map[string]int64, len(pipelines))
	for _, row := range rows {
		stageCounts[row.StageID] = row.DealCount
		pipelineCounts[row.PipelineID] += row.DealCount
	}
	for i := range pipelines {
		pipelines[i].DealCount = pipelineCounts[pipelines[i].ID]
		for j := range pipelines[i].Stages {
			pipelines[i].Stages[j].DealCount = stageCounts[pipelines[i].Stages[j].ID]
		}
	}
	return nil
}

func lockPipelineWorkspace(tx *gorm.DB, workspaceID string) error {
	// All pipeline writers lock the same workspace first, including creates, to
	// serialize default selection without a schema change or lock-order deadlock.
	var workspace struct{ ID string }
	return tx.Table("workspaces").Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", workspaceID).Take(&workspace).Error
}

func clearOtherPipelineDefaults(tx *gorm.DB, workspaceID, id string) error {
	query := tx.Model(&model.CRMPipeline{}).Where("workspace_id = ? AND is_default = ?", workspaceID, true)
	// New pipelines receive their UUID on insert, so there is no ID to exclude yet.
	if id != "" {
		query = query.Where("id <> ?", id)
	}
	return query.Update("is_default", false).Error
}

func requireOtherPipelineDefault(tx *gorm.DB, pipeline model.CRMPipeline) error {
	var count int64
	if err := tx.Model(&model.CRMPipeline{}).Where("workspace_id = ? AND id <> ? AND is_default = ?", pipeline.WorkspaceID, pipeline.ID, true).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return &model.CRMPipelineValidationError{Message: "choose another pipeline as default before removing this default pipeline"}
	}
	return nil
}
