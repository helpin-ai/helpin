package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

type PMSprintCloseoutRepository struct {
	db *gorm.DB
}

func NewPMSprintCloseoutRepository(db *gorm.DB) *PMSprintCloseoutRepository {
	return &PMSprintCloseoutRepository{db: db}
}

func (r *PMSprintCloseoutRepository) CreateCloseout(ctx context.Context, closeout *model.PMSprintCloseout, tasks []model.PMSprintCloseoutTask) (*model.PMSprintCloseout, error) {
	if closeout == nil {
		return nil, fmt.Errorf("closeout is required")
	}

	var persisted model.PMSprintCloseout
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("sprint_id = ?", closeout.SprintID).First(&persisted).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("lookup sprint closeout: %w", err)
		}

		if closeout.ID == "" {
			closeout.ID = uuid.NewString()
		}
		if err := tx.Create(closeout).Error; err != nil {
			return fmt.Errorf("create sprint closeout: %w", err)
		}
		persisted = *closeout

		if len(tasks) == 0 {
			return nil
		}

		rows := make([]model.PMSprintCloseoutTask, len(tasks))
		for i := range tasks {
			rows[i] = tasks[i]
			if rows[i].ID == "" {
				rows[i].ID = uuid.NewString()
			}
			rows[i].CloseoutID = persisted.ID
		}
		if err := tx.Create(&rows).Error; err != nil {
			return fmt.Errorf("create sprint closeout tasks: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &persisted, nil
}

func (r *PMSprintCloseoutRepository) GetCloseoutBySprintID(ctx context.Context, sprintID string) (*model.PMSprintCloseout, []model.PMSprintCloseoutTask, error) {
	var closeout model.PMSprintCloseout
	if err := r.db.WithContext(ctx).Where("sprint_id = ?", sprintID).First(&closeout).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("get sprint closeout: %w", err)
	}

	var tasks []model.PMSprintCloseoutTask
	if err := r.db.WithContext(ctx).
		Where("closeout_id = ?", closeout.ID).
		Order("created_at ASC, id ASC").
		Find(&tasks).Error; err != nil {
		return nil, nil, fmt.Errorf("list sprint closeout tasks: %w", err)
	}
	return &closeout, tasks, nil
}

func (r *PMSprintCloseoutRepository) ListCloseoutSummaries(ctx context.Context, workspaceID string, teamID *string) ([]model.SprintCloseoutListItem, error) {
	query := r.db.WithContext(ctx).
		Table("pm_sprint_closeouts c").
		Select(`
			c.id AS closeout_id,
			c.sprint_id AS sprint_id,
			s.name AS sprint_name,
			c.team_id AS team_id,
			wt.name AS team_name,
			s.start_date AS start_date,
			s.end_date AS end_date,
			c.committed_count AS committed_count,
			c.completed_count AS completed_count,
			c.unfinished_count AS unfinished_count,
			c.rolled_over_count AS rolled_over_count,
			c.committed_points AS committed_points,
			c.completed_points AS completed_points,
			c.unfinished_points AS unfinished_points,
			c.rolled_over_points AS rolled_over_points,
			CASE
				WHEN c.committed_count = 0 THEN 0
				ELSE CAST(c.completed_count AS REAL) / CAST(c.committed_count AS REAL)
			END AS completion_rate,
			c.rolled_to_sprint_id AS rolled_to_sprint_id,
			rs.name AS rolled_to_sprint_name,
			c.closed_at AS closed_at
		`).
		Joins("JOIN pm_sprints s ON s.id = c.sprint_id").
		Joins("LEFT JOIN workspace_teams wt ON wt.id = c.team_id").
		Joins("LEFT JOIN pm_sprints rs ON rs.id = c.rolled_to_sprint_id").
		Where("c.workspace_id = ?", workspaceID)
	if teamID != nil && *teamID != "" {
		query = query.Where("c.team_id = ?", *teamID)
	}

	var items []model.SprintCloseoutListItem
	if err := query.Order("c.closed_at DESC, c.created_at DESC").Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("list sprint closeout summaries: %w", err)
	}
	return items, nil
}

func (r *PMSprintCloseoutRepository) ListInboundRolloverSummaries(ctx context.Context, sprintID string) ([]model.SprintInboundRolloverSummary, error) {
	var items []model.SprintInboundRolloverSummary
	if err := r.db.WithContext(ctx).
		Table("pm_sprint_closeouts c").
		Select(`
			c.sprint_id AS source_sprint_id,
			s.name AS source_sprint_name,
			c.rolled_over_count AS rolled_over_count,
			c.rolled_over_points AS rolled_over_points
		`).
		Joins("JOIN pm_sprints s ON s.id = c.sprint_id").
		Where("c.rolled_to_sprint_id = ?", sprintID).
		Order("c.closed_at DESC, c.created_at DESC").
		Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("list inbound sprint rollover summaries: %w", err)
	}
	return items, nil
}
