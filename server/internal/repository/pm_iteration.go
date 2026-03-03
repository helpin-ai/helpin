package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/d4interactive/teampulse/server/internal/model"
)

// PMIterationRepository handles DB operations for iterations.
type PMIterationRepository struct {
	db *gorm.DB
}

// NewPMIterationRepository creates a new PMIterationRepository.
func NewPMIterationRepository(db *gorm.DB) *PMIterationRepository {
	return &PMIterationRepository{db: db}
}

// List returns iterations with filters.
func (r *PMIterationRepository) List(ctx context.Context, workspaceID string, filters model.PMIterationListFilters) ([]model.PMIteration, error) {
	query := r.db.WithContext(ctx).Model(&model.PMIteration{}).Where("workspace_id = ?", workspaceID)
	if filters.TeamID != nil && *filters.TeamID != "" {
		query = query.Where("team_id = ?", *filters.TeamID)
	}
	if filters.Archived != nil {
		query = query.Where("archived = ?", *filters.Archived)
	}

	var iterations []model.PMIteration
	if err := query.Order("start_date DESC").Find(&iterations).Error; err != nil {
		return nil, fmt.Errorf("list iterations: %w", err)
	}

	if filters.Status != nil && *filters.Status != "" {
		filtered := make([]model.PMIteration, 0, len(iterations))
		now := time.Now().UTC()
		for _, it := range iterations {
			if computeIterationStatus(it.StartDate, it.EndDate, now) == *filters.Status {
				it.Status = *filters.Status
				filtered = append(filtered, it)
			}
		}
		return filtered, nil
	}

	now := time.Now().UTC()
	for i := range iterations {
		iterations[i].Status = computeIterationStatus(iterations[i].StartDate, iterations[i].EndDate, now)
	}
	return iterations, nil
}

// GetByID returns an iteration with labels/stats.
func (r *PMIterationRepository) GetByID(ctx context.Context, id string) (*model.IterationWithStats, error) {
	return r.GetWithStats(ctx, id)
}

// GetWithStats returns an iteration with labels/stats.
func (r *PMIterationRepository) GetWithStats(ctx context.Context, id string) (*model.IterationWithStats, error) {
	var iteration model.PMIteration
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&iteration).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get iteration: %w", err)
	}

	labels, err := r.listLabels(ctx, id)
	if err != nil {
		return nil, err
	}
	stats, err := r.ComputeStats(ctx, id)
	if err != nil {
		return nil, err
	}
	iteration.Status = computeIterationStatus(iteration.StartDate, iteration.EndDate, time.Now().UTC())

	return &model.IterationWithStats{Iteration: iteration, Labels: labels, Stats: stats}, nil
}

// Create inserts an iteration.
func (r *PMIterationRepository) Create(ctx context.Context, iteration *model.PMIteration) error {
	if err := r.db.WithContext(ctx).Create(iteration).Error; err != nil {
		return fmt.Errorf("create iteration: %w", err)
	}
	return nil
}

// Update updates an iteration.
func (r *PMIterationRepository) Update(ctx context.Context, iteration *model.PMIteration) error {
	if err := r.db.WithContext(ctx).Save(iteration).Error; err != nil {
		return fmt.Errorf("update iteration: %w", err)
	}
	return nil
}

// Delete hard-deletes an iteration.
func (r *PMIterationRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&model.PMIteration{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete iteration: %w", err)
	}
	return nil
}

// GetCurrentIteration returns active iteration by workspace and optional team.
func (r *PMIterationRepository) GetCurrentIteration(ctx context.Context, workspaceID string, teamID *string) (*model.PMIteration, error) {
	nowDate := time.Now().UTC().Format("2006-01-02")
	query := r.db.WithContext(ctx).
		Where("workspace_id = ? AND archived = false AND start_date <= ? AND end_date >= ?", workspaceID, nowDate, nowDate)
	if teamID != nil && *teamID != "" {
		query = query.Where("team_id = ?", *teamID)
	}

	var iteration model.PMIteration
	if err := query.Order("start_date DESC").First(&iteration).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get current iteration: %w", err)
	}
	iteration.Status = model.PMIterationStatusStarted
	return &iteration, nil
}

// ComputeStats computes derived story/point metrics for an iteration.
func (r *PMIterationRepository) ComputeStats(ctx context.Context, iterationID string) (model.PMIterationStats, error) {
	stats := model.PMIterationStats{}
	var rows []struct {
		StateType string
		Count     int
		Points    int
	}
	if err := r.db.WithContext(ctx).
		Table("pm_stories s").
		Select("ws.state_type AS state_type, COUNT(*) AS count, COALESCE(SUM(COALESCE(s.estimate, 0)), 0) AS points").
		Joins("JOIN pm_workflow_states ws ON ws.id = s.workflow_state_id").
		Where("s.iteration_id = ? AND s.archived = false", iterationID).
		Group("ws.state_type").
		Scan(&rows).Error; err != nil {
		return stats, fmt.Errorf("compute iteration stats: %w", err)
	}

	for _, row := range rows {
		stats.StoryCount += row.Count
		stats.TotalPoints += row.Points
		if row.StateType == model.PMStateTypeDone {
			stats.DoneStoryCount += row.Count
			stats.DonePoints += row.Points
		}
	}
	return stats, nil
}

// ReplaceLabels replaces all labels linked to an iteration.
func (r *PMIterationRepository) ReplaceLabels(ctx context.Context, iterationID string, labelIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.PMIterationLabel{}, "iteration_id = ?", iterationID).Error; err != nil {
			return fmt.Errorf("clear iteration labels: %w", err)
		}
		for _, labelID := range labelIDs {
			link := model.PMIterationLabel{IterationID: iterationID, LabelID: labelID}
			if err := tx.Create(&link).Error; err != nil {
				return fmt.Errorf("set iteration labels: %w", err)
			}
		}
		return nil
	})
}

// ListStories returns stories in an iteration.
func (r *PMIterationRepository) ListStories(ctx context.Context, iterationID string) ([]model.PMStory, error) {
	var stories []model.PMStory
	if err := r.db.WithContext(ctx).
		Where("iteration_id = ? AND archived = false", iterationID).
		Order("position ASC, created_at DESC").
		Find(&stories).Error; err != nil {
		return nil, fmt.Errorf("list iteration stories: %w", err)
	}
	return stories, nil
}

// HasDateOverlap checks whether another iteration overlaps date range for workspace/team.
func (r *PMIterationRepository) HasDateOverlap(ctx context.Context, workspaceID string, teamID *string, startDate, endDate time.Time, excludeID *string) (bool, error) {
	query := r.db.WithContext(ctx).
		Model(&model.PMIteration{}).
		Where("workspace_id = ? AND archived = false", workspaceID).
		Where("start_date < ? AND end_date > ?", endDate.Format("2006-01-02"), startDate.Format("2006-01-02"))

	if teamID != nil && *teamID != "" {
		query = query.Where("team_id = ?", *teamID)
	} else {
		query = query.Where("team_id IS NULL")
	}
	if excludeID != nil && *excludeID != "" {
		query = query.Where("id <> ?", *excludeID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, fmt.Errorf("check iteration overlap: %w", err)
	}
	return count > 0, nil
}

func (r *PMIterationRepository) listLabels(ctx context.Context, iterationID string) ([]model.PMLabel, error) {
	var labels []model.PMLabel
	if err := r.db.WithContext(ctx).
		Table("pm_labels l").
		Select("l.*").
		Joins("JOIN pm_iteration_labels pil ON pil.label_id = l.id").
		Where("pil.iteration_id = ?", iterationID).
		Order("l.name ASC").
		Find(&labels).Error; err != nil {
		return nil, fmt.Errorf("list iteration labels: %w", err)
	}
	return labels, nil
}

func computeIterationStatus(startDate, endDate time.Time, now time.Time) string {
	current := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	start := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, time.UTC)
	end := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, time.UTC)
	if current.Before(start) {
		return model.PMIterationStatusUnstarted
	}
	if current.After(end) {
		return model.PMIterationStatusDone
	}
	return model.PMIterationStatusStarted
}
