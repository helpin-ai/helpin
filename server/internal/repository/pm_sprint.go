package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/d4interactive/teampulse/server/internal/model"
)

// PMSprintRepository handles DB operations for sprints.
type PMSprintRepository struct {
	db *gorm.DB
}

// NewPMSprintRepository creates a new PMSprintRepository.
func NewPMSprintRepository(db *gorm.DB) *PMSprintRepository {
	return &PMSprintRepository{db: db}
}

// List returns sprints with filters.
func (r *PMSprintRepository) List(ctx context.Context, workspaceID string, filters model.PMSprintListFilters) ([]model.PMSprint, error) {
	query := r.db.WithContext(ctx).Model(&model.PMSprint{}).Where("workspace_id = ?", workspaceID)
	if filters.TeamID != nil && *filters.TeamID != "" {
		query = query.Where("team_id = ?", *filters.TeamID)
	}
	if filters.Archived != nil {
		query = query.Where("archived = ?", *filters.Archived)
	}

	var sprints []model.PMSprint
	if err := query.Order("start_date DESC").Find(&sprints).Error; err != nil {
		return nil, fmt.Errorf("list sprints: %w", err)
	}

	if filters.Status != nil && *filters.Status != "" {
		filtered := make([]model.PMSprint, 0, len(sprints))
		now := time.Now().UTC()
		for _, it := range sprints {
			if computeSprintStatus(it.StartDate, it.EndDate, now) == *filters.Status {
				it.Status = *filters.Status
				filtered = append(filtered, it)
			}
		}
		return filtered, nil
	}

	now := time.Now().UTC()
	for i := range sprints {
		sprints[i].Status = computeSprintStatus(sprints[i].StartDate, sprints[i].EndDate, now)
	}
	return sprints, nil
}

// GetByID returns a sprint with labels/stats.
func (r *PMSprintRepository) GetByID(ctx context.Context, id string) (*model.SprintWithStats, error) {
	return r.GetWithStats(ctx, id)
}

// GetWithStats returns a sprint with labels/stats.
func (r *PMSprintRepository) GetWithStats(ctx context.Context, id string) (*model.SprintWithStats, error) {
	var sprint model.PMSprint
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&sprint).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get sprint: %w", err)
	}

	labels, err := r.listLabels(ctx, id)
	if err != nil {
		return nil, err
	}
	stats, err := r.ComputeStats(ctx, id)
	if err != nil {
		return nil, err
	}
	sprint.Status = computeSprintStatus(sprint.StartDate, sprint.EndDate, time.Now().UTC())

	return &model.SprintWithStats{Sprint: sprint, Labels: labels, Stats: stats}, nil
}

// Create inserts a sprint.
func (r *PMSprintRepository) Create(ctx context.Context, sprint *model.PMSprint) error {
	if err := r.db.WithContext(ctx).Create(sprint).Error; err != nil {
		return fmt.Errorf("create sprint: %w", err)
	}
	return nil
}

// Update updates a sprint.
func (r *PMSprintRepository) Update(ctx context.Context, sprint *model.PMSprint) error {
	if err := r.db.WithContext(ctx).Save(sprint).Error; err != nil {
		return fmt.Errorf("update sprint: %w", err)
	}
	return nil
}

// Delete hard-deletes a sprint.
func (r *PMSprintRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&model.PMSprint{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete sprint: %w", err)
	}
	return nil
}

// GetCurrentSprint returns active sprint by workspace and optional team.
func (r *PMSprintRepository) GetCurrentSprint(ctx context.Context, workspaceID string, teamID *string) (*model.PMSprint, error) {
	nowDate := time.Now().UTC().Format("2006-01-02")
	query := r.db.WithContext(ctx).
		Where("workspace_id = ? AND archived = false AND start_date <= ? AND end_date >= ?", workspaceID, nowDate, nowDate)
	if teamID != nil && *teamID != "" {
		query = query.Where("team_id = ?", *teamID)
	}

	var sprint model.PMSprint
	if err := query.Order("start_date DESC").First(&sprint).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get current sprint: %w", err)
	}
	sprint.Status = model.PMSprintStatusStarted
	return &sprint, nil
}

// ComputeStats computes derived story/point metrics for a sprint.
func (r *PMSprintRepository) ComputeStats(ctx context.Context, sprintID string) (model.PMSprintStats, error) {
	stats := model.PMSprintStats{}
	var rows []struct {
		StateType string
		Count     int
		Points    int
	}
	if err := r.db.WithContext(ctx).
		Table("pm_stories s").
		Select("ws.state_type AS state_type, COUNT(*) AS count, COALESCE(SUM(COALESCE(s.estimate, 0)), 0) AS points").
		Joins("JOIN pm_workflow_states ws ON ws.id = s.workflow_state_id").
		Where("s.sprint_id = ? AND s.archived = false", sprintID).
		Group("ws.state_type").
		Scan(&rows).Error; err != nil {
		return stats, fmt.Errorf("compute sprint stats: %w", err)
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

// ReplaceLabels replaces all labels linked to a sprint.
func (r *PMSprintRepository) ReplaceLabels(ctx context.Context, sprintID string, labelIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.PMSprintLabel{}, "sprint_id = ?", sprintID).Error; err != nil {
			return fmt.Errorf("clear sprint labels: %w", err)
		}
		for _, labelID := range labelIDs {
			link := model.PMSprintLabel{SprintID: sprintID, LabelID: labelID}
			if err := tx.Create(&link).Error; err != nil {
				return fmt.Errorf("set sprint labels: %w", err)
			}
		}
		return nil
	})
}

// ListStories returns stories in a sprint.
func (r *PMSprintRepository) ListStories(ctx context.Context, sprintID string) ([]model.PMStory, error) {
	var stories []model.PMStory
	if err := r.db.WithContext(ctx).
		Where("sprint_id = ? AND archived = false", sprintID).
		Order("position ASC, created_at DESC").
		Find(&stories).Error; err != nil {
		return nil, fmt.Errorf("list sprint stories: %w", err)
	}
	return stories, nil
}

// HasDateOverlap checks whether another sprint overlaps date range for workspace/team.
func (r *PMSprintRepository) HasDateOverlap(ctx context.Context, workspaceID string, teamID *string, startDate, endDate time.Time, excludeID *string) (bool, error) {
	query := r.db.WithContext(ctx).
		Model(&model.PMSprint{}).
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
		return false, fmt.Errorf("check sprint overlap: %w", err)
	}
	return count > 0, nil
}

func (r *PMSprintRepository) listLabels(ctx context.Context, sprintID string) ([]model.PMLabel, error) {
	var labels []model.PMLabel
	if err := r.db.WithContext(ctx).
		Table("pm_labels l").
		Select("l.*").
		Joins("JOIN pm_sprint_labels psl ON psl.label_id = l.id").
		Where("psl.sprint_id = ?", sprintID).
		Order("l.name ASC").
		Find(&labels).Error; err != nil {
		return nil, fmt.Errorf("list sprint labels: %w", err)
	}
	return labels, nil
}

func computeSprintStatus(startDate, endDate time.Time, now time.Time) string {
	current := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	start := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, time.UTC)
	end := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, time.UTC)
	if current.Before(start) {
		return model.PMSprintStatusUnstarted
	}
	if current.After(end) {
		return model.PMSprintStatusDone
	}
	return model.PMSprintStatusStarted
}
