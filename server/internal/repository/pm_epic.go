package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PMEpicRepository handles DB operations for epics.
type PMEpicRepository struct {
	db *gorm.DB
}

// NewPMEpicRepository creates a new PMEpicRepository.
func NewPMEpicRepository(db *gorm.DB) *PMEpicRepository {
	return &PMEpicRepository{db: db}
}

// List returns epics in a workspace with optional filters.
func (r *PMEpicRepository) List(ctx context.Context, workspaceID string, filters model.PMEpicListFilters) ([]model.PMEpic, error) {
	query := r.listQuery(ctx, workspaceID, filters)

	var epics []model.PMEpic
	if err := query.Order("pm_epics.position ASC, pm_epics.created_at DESC").Find(&epics).Error; err != nil {
		return nil, fmt.Errorf("list epics: %w", err)
	}
	return epics, nil
}

// ListPage returns one bounded page of epics and the total matching count.
func (r *PMEpicRepository) ListPage(ctx context.Context, workspaceID string, filters model.PMEpicListFilters, pagination model.PMPagination) ([]model.PMEpic, int64, error) {
	var total int64
	if err := r.listQuery(ctx, workspaceID, filters).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count epics: %w", err)
	}

	page := pagination.Page
	perPage := pagination.PerPage
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 50
	}
	var epics []model.PMEpic
	if err := r.listQuery(ctx, workspaceID, filters).
		Order("pm_epics.position ASC, pm_epics.created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&epics).Error; err != nil {
		return nil, 0, fmt.Errorf("list epics page: %w", err)
	}
	return epics, total, nil
}

func (r *PMEpicRepository) listQuery(ctx context.Context, workspaceID string, filters model.PMEpicListFilters) *gorm.DB {
	query := r.db.WithContext(ctx).Model(&model.PMEpic{}).Where("pm_epics.workspace_id = ?", workspaceID)

	if filters.TeamID != nil && *filters.TeamID != "" {
		query = query.Where("pm_epics.team_id = ?", *filters.TeamID)
	}
	if filters.StateID != nil && *filters.StateID != "" {
		query = query.Where("pm_epics.epic_state_id = ?", *filters.StateID)
	}
	if filters.Archived != nil {
		query = query.Where("pm_epics.archived = ?", *filters.Archived)
	}
	if filters.LabelID != nil && *filters.LabelID != "" {
		query = query.Joins("JOIN pm_epic_labels pel ON pel.epic_id = pm_epics.id").Where("pel.label_id = ?", *filters.LabelID)
	}
	if filters.AccessibleTeamIDs != nil {
		if len(filters.AccessibleTeamIDs) == 0 {
			query = query.Where("1 = 0")
		} else {
			query = query.Where("pm_epics.team_id IN ?", filters.AccessibleTeamIDs)
		}
	}
	return query
}

// ListByIDs returns epics by ID for a workspace.
func (r *PMEpicRepository) ListByIDs(ctx context.Context, workspaceID string, ids []string) ([]model.PMEpic, error) {
	if len(ids) == 0 {
		return []model.PMEpic{}, nil
	}
	var epics []model.PMEpic
	if err := r.db.WithContext(ctx).
		Select("id", "workspace_id", "name").
		Where("workspace_id = ? AND id IN ?", workspaceID, ids).
		Find(&epics).Error; err != nil {
		return nil, fmt.Errorf("list epics by ids: %w", err)
	}
	return epics, nil
}

// GetByID returns an epic with labels and computed stats.
func (r *PMEpicRepository) GetByID(ctx context.Context, id string) (*model.EpicWithStats, error) {
	return r.GetWithStats(ctx, id)
}

// GetWithStats returns an epic with labels and computed stats.
func (r *PMEpicRepository) GetWithStats(ctx context.Context, id string) (*model.EpicWithStats, error) {
	var epic model.PMEpic
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&epic).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get epic: %w", err)
	}

	labels, err := r.listLabels(ctx, id)
	if err != nil {
		return nil, err
	}
	objectives, err := r.listObjectives(ctx, id)
	if err != nil {
		return nil, err
	}
	stats, err := r.ComputeStats(ctx, id)
	if err != nil {
		return nil, err
	}

	return &model.EpicWithStats{Epic: epic, Labels: labels, Objectives: objectives, Stats: stats}, nil
}

// Create inserts an epic.
func (r *PMEpicRepository) Create(ctx context.Context, epic *model.PMEpic) error {
	if err := r.db.WithContext(ctx).Create(epic).Error; err != nil {
		return fmt.Errorf("create epic: %w", err)
	}
	return nil
}

// CreateWithLabels inserts an epic and its label links atomically.
func (r *PMEpicRepository) CreateWithLabels(ctx context.Context, epic *model.PMEpic, labelIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txRepo := NewPMEpicRepository(tx)
		if err := txRepo.Create(ctx, epic); err != nil {
			return err
		}
		return txRepo.replaceLabels(ctx, epic.ID, labelIDs)
	})
}

// Update updates an epic.
func (r *PMEpicRepository) Update(ctx context.Context, epic *model.PMEpic) error {
	if err := r.db.WithContext(ctx).Save(epic).Error; err != nil {
		return fmt.Errorf("update epic: %w", err)
	}
	return nil
}

// UpdateWithLabels updates an epic and, when labelIDs is non-nil, replaces its
// label links in the same transaction. A nil labelIDs slice preserves labels.
func (r *PMEpicRepository) UpdateWithLabels(ctx context.Context, epic *model.PMEpic, labelIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txRepo := NewPMEpicRepository(tx)
		if err := txRepo.Update(ctx, epic); err != nil {
			return err
		}
		if labelIDs == nil {
			return nil
		}
		return txRepo.replaceLabels(ctx, epic.ID, labelIDs)
	})
}

// Delete archives an epic.
func (r *PMEpicRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.PMEpic{}).
		Where("id = ?", id).
		Update("archived", true).Error; err != nil {
		return fmt.Errorf("archive epic: %w", err)
	}
	return nil
}

// UpdateHealth updates health fields on an epic.
func (r *PMEpicRepository) UpdateHealth(ctx context.Context, id, health string, comment *string) error {
	updates := map[string]interface{}{"health": health, "health_comment": comment}
	if err := r.db.WithContext(ctx).
		Model(&model.PMEpic{}).
		Where("id = ?", id).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("update epic health: %w", err)
	}
	return nil
}

// AddLabel links a label to an epic.
func (r *PMEpicRepository) AddLabel(ctx context.Context, epicID, labelID string) error {
	link := model.PMEpicLabel{EpicID: epicID, LabelID: labelID}
	if err := r.db.WithContext(ctx).Create(&link).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil
		}
		return fmt.Errorf("add epic label: %w", err)
	}
	return nil
}

// RemoveLabel unlinks a label from an epic.
func (r *PMEpicRepository) RemoveLabel(ctx context.Context, epicID, labelID string) error {
	if err := r.db.WithContext(ctx).
		Delete(&model.PMEpicLabel{}, "epic_id = ? AND label_id = ?", epicID, labelID).Error; err != nil {
		return fmt.Errorf("remove epic label: %w", err)
	}
	return nil
}

// ReplaceLabels replaces all labels linked to an epic.
func (r *PMEpicRepository) ReplaceLabels(ctx context.Context, epicID string, labelIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return NewPMEpicRepository(tx).replaceLabels(ctx, epicID, labelIDs)
	})
}

// replaceLabels performs label writes on the repository's current database
// handle. Callers that need atomicity must bind the repository to a transaction.
func (r *PMEpicRepository) replaceLabels(ctx context.Context, epicID string, labelIDs []string) error {
	db := r.db.WithContext(ctx)
	if err := db.Delete(&model.PMEpicLabel{}, "epic_id = ?", epicID).Error; err != nil {
		return fmt.Errorf("clear epic labels: %w", err)
	}
	for _, labelID := range labelIDs {
		link := model.PMEpicLabel{EpicID: epicID, LabelID: labelID}
		if err := db.Create(&link).Error; err != nil {
			return fmt.Errorf("set epic labels: %w", err)
		}
	}
	return nil
}

// ComputeStats computes derived story/point metrics for an epic.
func (r *PMEpicRepository) ComputeStats(ctx context.Context, epicID string) (model.PMEpicStats, error) {
	stats := model.PMEpicStats{}

	var rows []struct {
		StateType string
		Count     int
		Points    int
	}
	if err := r.db.WithContext(ctx).
		Table("pm_tasks s").
		Select("ws.state_type AS state_type, COUNT(*) AS count, COALESCE(SUM(COALESCE(s.estimate, 0)), 0) AS points").
		Joins("JOIN pm_workflow_states ws ON ws.id = s.workflow_state_id").
		Where("s.epic_id = ? AND s.archived = false", epicID).
		Group("ws.state_type").
		Scan(&rows).Error; err != nil {
		return stats, fmt.Errorf("compute epic stats: %w", err)
	}

	for _, row := range rows {
		stats.TaskCount += row.Count
		stats.TotalPoints += row.Points
		switch row.StateType {
		case model.PMStateTypeDone:
			stats.DoneTaskCount += row.Count
			stats.DonePoints += row.Points
		case model.PMStateTypeStarted:
			stats.InProgressCount += row.Count
		case model.PMStateTypeUnstarted, model.PMStateTypeBacklog:
			stats.UnstartedCount += row.Count
		}
	}
	return stats, nil
}

// ComputeStatsBatch computes derived task/point metrics for multiple epics.
func (r *PMEpicRepository) ComputeStatsBatch(ctx context.Context, epicIDs []string) (map[string]model.PMEpicStats, error) {
	result := make(map[string]model.PMEpicStats, len(epicIDs))
	if len(epicIDs) == 0 {
		return result, nil
	}
	type row struct {
		EpicID    string
		StateType string
		Count     int
		Points    int
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Table("pm_tasks s").
		Select("s.epic_id AS epic_id, ws.state_type AS state_type, COUNT(*) AS count, COALESCE(SUM(COALESCE(s.estimate, 0)), 0) AS points").
		Joins("JOIN pm_workflow_states ws ON ws.id = s.workflow_state_id").
		Where("s.epic_id IN ? AND s.archived = false", epicIDs).
		Group("s.epic_id, ws.state_type").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("batch compute epic stats: %w", err)
	}
	for _, row := range rows {
		stats := result[row.EpicID]
		stats.TaskCount += row.Count
		stats.TotalPoints += row.Points
		switch row.StateType {
		case model.PMStateTypeDone:
			stats.DoneTaskCount += row.Count
			stats.DonePoints += row.Points
		case model.PMStateTypeStarted:
			stats.InProgressCount += row.Count
		case model.PMStateTypeUnstarted, model.PMStateTypeBacklog:
			stats.UnstartedCount += row.Count
		}
		result[row.EpicID] = stats
	}
	return result, nil
}

// ListTasks returns non-archived tasks in an epic.
func (r *PMEpicRepository) ListTasks(ctx context.Context, epicID string) ([]model.PMTask, error) {
	var stories []model.PMTask
	if err := r.db.WithContext(ctx).
		Joins("JOIN pm_workflow_states ws ON ws.id = pm_tasks.workflow_state_id").
		Where("epic_id = ? AND archived = false", epicID).
		Order(`CASE ws.state_type WHEN 'backlog' THEN 0 WHEN 'unstarted' THEN 1 WHEN 'started' THEN 2 WHEN 'done' THEN 3 ELSE 4 END ASC`).
		Order("ws.position ASC").
		Order("ws.id ASC").
		Order(`CASE WHEN ws.state_type = 'done' THEN COALESCE(pm_tasks.completed_at, pm_tasks.moved_at, pm_tasks.updated_at) END DESC`).
		Order(`CASE WHEN ws.state_type = 'done' THEN pm_tasks.updated_at END DESC`).
		Order("pm_tasks.position ASC").
		Order("pm_tasks.updated_at DESC").
		Find(&stories).Error; err != nil {
		return nil, fmt.Errorf("list epic stories: %w", err)
	}
	return stories, nil
}

// ListEnrichedTasks returns non-archived tasks in an epic with table-facing
// computed fields such as owner member IDs, labels, state info, and latest run.
func (r *PMEpicRepository) ListEnrichedTasks(ctx context.Context, epicID string) ([]model.BoardTask, error) {
	tasks, err := r.ListTasks(ctx, epicID)
	if err != nil {
		return nil, err
	}
	return NewPMTaskRepository(r.db).EnrichTasksForList(ctx, tasks), nil
}

func (r *PMEpicRepository) listObjectives(ctx context.Context, epicID string) ([]model.RoadmapObjectiveRef, error) {
	var refs []model.RoadmapObjectiveRef
	if err := r.db.WithContext(ctx).
		Table("pm_objectives o").
		Select("o.id, o.name").
		Joins("JOIN pm_epic_objectives peo ON peo.objective_id = o.id").
		Where("peo.epic_id = ?", epicID).
		Order("o.name ASC").
		Find(&refs).Error; err != nil {
		return nil, fmt.Errorf("list epic objectives: %w", err)
	}
	return refs, nil
}

// ListObjectivesBatch returns a map of epic_id → objective references for multiple epics.
func (r *PMEpicRepository) ListObjectivesBatch(ctx context.Context, epicIDs []string) (map[string][]model.RoadmapObjectiveRef, error) {
	if len(epicIDs) == 0 {
		return map[string][]model.RoadmapObjectiveRef{}, nil
	}

	type row struct {
		EpicID        string `gorm:"column:epic_id"`
		ObjectiveID   string `gorm:"column:objective_id"`
		ObjectiveName string `gorm:"column:objective_name"`
	}

	var rows []row
	if err := r.db.WithContext(ctx).
		Table("pm_epic_objectives peo").
		Select("peo.epic_id, peo.objective_id, o.name AS objective_name").
		Joins("JOIN pm_objectives o ON o.id = peo.objective_id").
		Where("peo.epic_id IN ?", epicIDs).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("batch list epic objectives: %w", err)
	}

	result := make(map[string][]model.RoadmapObjectiveRef, len(epicIDs))
	for _, r := range rows {
		result[r.EpicID] = append(result[r.EpicID], model.RoadmapObjectiveRef{
			ID:   r.ObjectiveID,
			Name: r.ObjectiveName,
		})
	}
	return result, nil
}

// ListLabelsBatch returns labels grouped by epic ID for the requested epics.
func (r *PMEpicRepository) ListLabelsBatch(ctx context.Context, epicIDs []string) (map[string][]model.PMLabel, error) {
	result := make(map[string][]model.PMLabel, len(epicIDs))
	if len(epicIDs) == 0 {
		return result, nil
	}
	type row struct {
		EpicRefID string `gorm:"column:epic_ref_id"`
		model.PMLabel
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Table("pm_labels l").
		Select("pel.epic_id AS epic_ref_id, l.*").
		Joins("JOIN pm_epic_labels pel ON pel.label_id = l.id").
		Where("pel.epic_id IN ?", epicIDs).
		Order("pel.epic_id ASC, l.name ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("batch list epic labels: %w", err)
	}
	for _, row := range rows {
		result[row.EpicRefID] = append(result[row.EpicRefID], row.PMLabel)
	}
	return result, nil
}

func (r *PMEpicRepository) listLabels(ctx context.Context, epicID string) ([]model.PMLabel, error) {
	var labels []model.PMLabel
	if err := r.db.WithContext(ctx).
		Table("pm_labels l").
		Select("l.*").
		Joins("JOIN pm_epic_labels pel ON pel.label_id = l.id").
		Where("pel.epic_id = ?", epicID).
		Order("l.name ASC").
		Find(&labels).Error; err != nil {
		return nil, fmt.Errorf("list epic labels: %w", err)
	}
	return labels, nil
}
