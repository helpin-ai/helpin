package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PMLabelRepository handles DB operations for labels.
type PMLabelRepository struct {
	db *gorm.DB
}

// PMLabelListOptions aliases ScopeFilterOptions for labels.
type PMLabelListOptions = ScopeFilterOptions

// NewPMLabelRepository creates a new PMLabelRepository.
func NewPMLabelRepository(db *gorm.DB) *PMLabelRepository {
	return &PMLabelRepository{db: db}
}

// ListByWorkspace lists labels by workspace.
func (r *PMLabelRepository) ListByWorkspace(ctx context.Context, workspaceID string, opts PMLabelListOptions) ([]model.PMLabel, error) {
	var labels []model.PMLabel
	query := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	query = ApplyScopeFilter(query, opts)
	if err := query.Order("team_id ASC, name ASC").Find(&labels).Error; err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}
	return labels, nil
}

// GetByID returns a label by ID.
func (r *PMLabelRepository) GetByID(ctx context.Context, id string) (*model.PMLabel, error) {
	var label model.PMLabel
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&label).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get label: %w", err)
	}
	return &label, nil
}

// ListByIDs returns labels matching the requested IDs in one query.
func (r *PMLabelRepository) ListByIDs(ctx context.Context, ids []string) ([]model.PMLabel, error) {
	if len(ids) == 0 {
		return []model.PMLabel{}, nil
	}
	var labels []model.PMLabel
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&labels).Error; err != nil {
		return nil, fmt.Errorf("list labels by ids: %w", err)
	}
	return labels, nil
}

// GetByName returns a label by workspace/name.
func (r *PMLabelRepository) GetByName(ctx context.Context, workspaceID string, teamID *string, name string) (*model.PMLabel, error) {
	var label model.PMLabel
	query := r.db.WithContext(ctx).
		Where("workspace_id = ? AND LOWER(name) = LOWER(?)", workspaceID, strings.TrimSpace(name))
	if teamID == nil || strings.TrimSpace(*teamID) == "" {
		query = query.Where("team_id IS NULL")
	} else {
		query = query.Where("team_id = ?", *teamID)
	}
	if err := query.First(&label).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get label by name: %w", err)
	}
	return &label, nil
}

// Create inserts a label.
func (r *PMLabelRepository) Create(ctx context.Context, label *model.PMLabel) error {
	if err := r.db.WithContext(ctx).Create(label).Error; err != nil {
		return fmt.Errorf("create label: %w", err)
	}
	return nil
}

// Update updates a label.
func (r *PMLabelRepository) Update(ctx context.Context, label *model.PMLabel) error {
	if err := r.db.WithContext(ctx).Save(label).Error; err != nil {
		return fmt.Errorf("update label: %w", err)
	}
	return nil
}

// ListWithStats returns all labels in a workspace with task/epic completion stats.
func (r *PMLabelRepository) ListWithStats(ctx context.Context, workspaceID string, opts PMLabelListOptions) ([]model.LabelWithStats, error) {
	// Fetch labels
	query := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	query = ApplyScopeFilter(query, opts)
	var labels []model.PMLabel
	if err := query.Order("team_id ASC, name ASC").Find(&labels).Error; err != nil {
		return nil, fmt.Errorf("list labels with stats: %w", err)
	}

	if len(labels) == 0 {
		return []model.LabelWithStats{}, nil
	}

	// Collect label IDs
	labelIDs := make([]string, len(labels))
	for i, l := range labels {
		labelIDs[i] = l.ID
	}

	// Query task stats per label
	type taskStatRow struct {
		LabelID string `gorm:"column:label_id"`
		Total   int    `gorm:"column:total"`
		Done    int    `gorm:"column:done"`
		Points  int    `gorm:"column:points"`
		DonePts int    `gorm:"column:done_pts"`
	}
	var taskRows []taskStatRow
	if err := r.db.WithContext(ctx).
		Table("pm_task_labels sl").
		Select(`sl.label_id,
			COUNT(*) AS total,
			SUM(CASE WHEN s.completed = true THEN 1 ELSE 0 END) AS done,
			COALESCE(SUM(COALESCE(s.estimate, 0)), 0) AS points,
			COALESCE(SUM(CASE WHEN s.completed = true THEN COALESCE(s.estimate, 0) ELSE 0 END), 0) AS done_pts`).
		Joins("JOIN pm_tasks s ON s.id = sl.task_id").
		Where("sl.label_id IN ? AND s.archived = false", labelIDs).
		Group("sl.label_id").
		Scan(&taskRows).Error; err != nil {
		return nil, fmt.Errorf("label task stats: %w", err)
	}

	// Query epic stats per label
	type epicStatRow struct {
		LabelID string `gorm:"column:label_id"`
		Total   int    `gorm:"column:total"`
		Done    int    `gorm:"column:done"`
	}
	var epicRows []epicStatRow
	if err := r.db.WithContext(ctx).
		Table("pm_epic_labels el").
		Select(`el.label_id,
			COUNT(*) AS total,
			SUM(CASE WHEN e.completed = true THEN 1 ELSE 0 END) AS done`).
		Joins("JOIN pm_epics e ON e.id = el.epic_id").
		Where("el.label_id IN ? AND e.archived = false", labelIDs).
		Group("el.label_id").
		Scan(&epicRows).Error; err != nil {
		return nil, fmt.Errorf("label epic stats: %w", err)
	}

	// Index stats by label ID
	taskMap := make(map[string]taskStatRow, len(taskRows))
	for _, r := range taskRows {
		taskMap[r.LabelID] = r
	}
	epicMap := make(map[string]epicStatRow, len(epicRows))
	for _, r := range epicRows {
		epicMap[r.LabelID] = r
	}

	// Build result
	results := make([]model.LabelWithStats, len(labels))
	for i, label := range labels {
		stats := model.LabelStats{}
		if ss, ok := taskMap[label.ID]; ok {
			stats.TaskCount = ss.Total
			stats.DoneTaskCount = ss.Done
			stats.TotalPoints = ss.Points
			stats.DonePoints = ss.DonePts
		}
		if es, ok := epicMap[label.ID]; ok {
			stats.EpicCount = es.Total
			stats.DoneEpicCount = es.Done
		}
		results[i] = model.LabelWithStats{Label: label, Stats: stats}
	}
	return results, nil
}

// Delete hard-deletes a label.
func (r *PMLabelRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&model.PMLabel{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete label: %w", err)
	}
	return nil
}
