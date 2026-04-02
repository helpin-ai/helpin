package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
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
	if filters.AccessibleTeamIDs != nil {
		if len(filters.AccessibleTeamIDs) == 0 {
			query = query.Where("1 = 0")
		} else {
			query = query.Where("team_id IN ?", filters.AccessibleTeamIDs)
		}
	}

	var sprints []model.PMSprint
	if err := query.Order("COALESCE(start_date, created_at) DESC").Find(&sprints).Error; err != nil {
		return nil, fmt.Errorf("list sprints: %w", err)
	}

	if filters.Status != nil && *filters.Status != "" {
		filtered := make([]model.PMSprint, 0, len(sprints))
		now := time.Now().UTC()
		for _, it := range sprints {
			status := computeSprintStatus(it.StartDate, it.EndDate, now)
			if status == *filters.Status {
				it.Status = status
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

// ListPlanningWorkspace returns grouped sprint cards plus an unassigned backlog preview for the planning page.
func (r *PMSprintRepository) ListPlanningWorkspace(ctx context.Context, workspaceID string, filters model.PMSprintPlanningFilters) (*model.SprintPlanningWorkspace, error) {
	const (
		defaultPreviewLimit = 5
		defaultBacklogLimit = 50
	)

	previewLimit := filters.PreviewStoryLimit
	if previewLimit <= 0 {
		previewLimit = defaultPreviewLimit
	}
	backlogLimit := filters.BacklogLimit
	if backlogLimit <= 0 {
		backlogLimit = defaultBacklogLimit
	}

	query := r.db.WithContext(ctx).Model(&model.PMSprint{}).
		Where("workspace_id = ?", workspaceID).
		Where("archived = ?", false)
	if filters.TeamID != nil && *filters.TeamID != "" {
		query = query.Where("team_id = ?", *filters.TeamID)
	}
	if filters.AccessibleTeamIDs != nil {
		if len(filters.AccessibleTeamIDs) == 0 {
			query = query.Where("1 = 0")
		} else {
			query = query.Where("team_id IN ?", filters.AccessibleTeamIDs)
		}
	}

	var sprints []model.PMSprint
	if err := query.Order("COALESCE(start_date, created_at) ASC, created_at ASC").Find(&sprints).Error; err != nil {
		return nil, fmt.Errorf("list planning sprints: %w", err)
	}

	now := time.Now().UTC()
	buckets := map[string]*model.SprintPlanningBucket{
		"active":    {Key: "active", Label: "Active"},
		"upcoming":  {Key: "upcoming", Label: "Upcoming"},
		"completed": {Key: "completed", Label: "Completed"},
	}

	sprintIDs := make([]string, 0, len(sprints))
	cardBySprintID := make(map[string]*model.SprintPlanningCard, len(sprints))
	for i := range sprints {
		sprints[i].Status = computeSprintStatus(sprints[i].StartDate, sprints[i].EndDate, now)
		bucketKey := planningBucketKey(sprints[i].Status)
		if bucketKey == "completed" && !filters.IncludeCompleted {
			continue
		}
		card := model.SprintPlanningCard{
			Sprint:         sprints[i],
			PreviewStories: []model.SprintPlanningTaskPreview{},
		}
		buckets[bucketKey].Sprints = append(buckets[bucketKey].Sprints, card)
		cardBySprintID[sprints[i].ID] = &buckets[bucketKey].Sprints[len(buckets[bucketKey].Sprints)-1]
		sprintIDs = append(sprintIDs, sprints[i].ID)
	}

	if len(sprintIDs) > 0 {
		statsBySprintID, err := r.computePlanningStats(ctx, sprintIDs)
		if err != nil {
			return nil, err
		}
		previewStoriesBySprintID, err := r.listPlanningPreviewStories(ctx, sprintIDs, previewLimit)
		if err != nil {
			return nil, err
		}
		for _, sprintID := range sprintIDs {
			card := cardBySprintID[sprintID]
			if card == nil {
				continue
			}
			card.Stats = statsBySprintID[sprintID]
			card.PreviewStories = previewStoriesBySprintID[sprintID]
			if overflow := card.Stats.TaskCount - len(card.PreviewStories); overflow > 0 {
				card.TaskPreviewOverflow = overflow
			}
		}
	}

	backlogStories, backlogTotal, err := r.listPlanningBacklogStories(ctx, workspaceID, filters, backlogLimit)
	if err != nil {
		return nil, err
	}

	orderedBuckets := []model.SprintPlanningBucket{
		*buckets["active"],
		*buckets["upcoming"],
		*buckets["completed"],
	}
	if !filters.IncludeCompleted {
		orderedBuckets = orderedBuckets[:2]
	}

	return &model.SprintPlanningWorkspace{
		Buckets:        orderedBuckets,
		BacklogStories: backlogStories,
		BacklogTotal:   backlogTotal,
	}, nil
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
		Where("workspace_id = ? AND archived = false AND start_date IS NOT NULL AND end_date IS NOT NULL AND start_date <= ? AND end_date >= ?", workspaceID, nowDate, nowDate)
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
		Table("pm_tasks s").
		Select("ws.state_type AS state_type, COUNT(*) AS count, COALESCE(SUM(COALESCE(s.estimate, 0)), 0) AS points").
		Joins("JOIN pm_workflow_states ws ON ws.id = s.workflow_state_id").
		Where("s.sprint_id = ? AND s.archived = false", sprintID).
		Group("ws.state_type").
		Scan(&rows).Error; err != nil {
		return stats, fmt.Errorf("compute sprint stats: %w", err)
	}

	for _, row := range rows {
		stats.TaskCount += row.Count
		stats.TotalPoints += row.Points
		if row.StateType == model.PMStateTypeDone {
			stats.DoneTaskCount += row.Count
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
func (r *PMSprintRepository) ListStories(ctx context.Context, sprintID string) ([]model.PMTask, error) {
	var stories []model.PMTask
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
		Where("start_date IS NOT NULL AND end_date IS NOT NULL").
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

func computeSprintStatus(startDate, endDate *time.Time, now time.Time) string {
	if startDate == nil || endDate == nil {
		return model.PMSprintStatusUnstarted
	}
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

func planningBucketKey(status string) string {
	switch status {
	case model.PMSprintStatusStarted:
		return "active"
	case model.PMSprintStatusDone:
		return "completed"
	default:
		return "upcoming"
	}
}

func (r *PMSprintRepository) computePlanningStats(ctx context.Context, sprintIDs []string) (map[string]model.PMSprintStats, error) {
	statsBySprintID := make(map[string]model.PMSprintStats, len(sprintIDs))
	if len(sprintIDs) == 0 {
		return statsBySprintID, nil
	}

	var rows []struct {
		SprintID  string
		StateType string
		Count     int
		Points    int
	}
	if err := r.db.WithContext(ctx).
		Table("pm_tasks s").
		Select("s.sprint_id AS sprint_id, ws.state_type AS state_type, COUNT(*) AS count, COALESCE(SUM(COALESCE(s.estimate, 0)), 0) AS points").
		Joins("JOIN pm_workflow_states ws ON ws.id = s.workflow_state_id").
		Where("s.sprint_id IN ? AND s.archived = false", sprintIDs).
		Group("s.sprint_id, ws.state_type").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("compute planning stats: %w", err)
	}

	for _, sprintID := range sprintIDs {
		statsBySprintID[sprintID] = model.PMSprintStats{}
	}
	for _, row := range rows {
		stats := statsBySprintID[row.SprintID]
		stats.TaskCount += row.Count
		stats.TotalPoints += row.Points
		if row.StateType == model.PMStateTypeDone {
			stats.DoneTaskCount += row.Count
			stats.DonePoints += row.Points
		}
		statsBySprintID[row.SprintID] = stats
	}
	return statsBySprintID, nil
}

func (r *PMSprintRepository) listPlanningPreviewStories(ctx context.Context, sprintIDs []string, limitPerSprint int) (map[string][]model.SprintPlanningTaskPreview, error) {
	bySprint := make(map[string][]model.SprintPlanningTaskPreview, len(sprintIDs))
	if len(sprintIDs) == 0 {
		return bySprint, nil
	}

	var rows []model.SprintPlanningTaskPreview
	if err := r.db.WithContext(ctx).
		Table("pm_tasks s").
		Select(`
			s.id,
			s.display_id,
			s.name,
			s.workflow_state_id,
			ws.name AS state_name,
			ws.state_type AS state_type,
			s.owner_member_id,
			s.estimate,
			s.priority,
			s.sprint_id,
			s.team_id
		`).
		Joins("JOIN pm_workflow_states ws ON ws.id = s.workflow_state_id").
		Where("s.sprint_id IN ? AND s.archived = false", sprintIDs).
		Order("s.sprint_id ASC, s.position ASC, s.created_at DESC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list planning preview stories: %w", err)
	}

	for _, row := range rows {
		if row.SprintID == nil {
			continue
		}
		current := bySprint[*row.SprintID]
		if len(current) >= limitPerSprint {
			continue
		}
		bySprint[*row.SprintID] = append(current, row)
	}
	return bySprint, nil
}

func (r *PMSprintRepository) listPlanningBacklogStories(ctx context.Context, workspaceID string, filters model.PMSprintPlanningFilters, limit int) ([]model.SprintPlanningTaskPreview, int, error) {
	base := r.db.WithContext(ctx).
		Table("pm_tasks s").
		Joins("JOIN pm_workflow_states ws ON ws.id = s.workflow_state_id").
		Where("s.workspace_id = ? AND s.archived = false AND s.sprint_id IS NULL", workspaceID).
		Where("ws.state_type <> ?", model.PMStateTypeDone)
	if filters.TeamID != nil && *filters.TeamID != "" {
		base = base.Where("s.team_id = ?", *filters.TeamID)
	}
	if filters.AccessibleTeamIDs != nil {
		if len(filters.AccessibleTeamIDs) == 0 {
			base = base.Where("1 = 0")
		} else {
			base = base.Where("s.team_id IN ?", filters.AccessibleTeamIDs)
		}
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count planning backlog stories: %w", err)
	}

	var stories []model.SprintPlanningTaskPreview
	if err := base.
		Select(`
			s.id,
			s.display_id,
			s.name,
			s.workflow_state_id,
			ws.name AS state_name,
			ws.state_type AS state_type,
			s.owner_member_id,
			s.estimate,
			s.priority,
			s.sprint_id,
			s.team_id
		`).
		Order("s.position ASC, s.updated_at DESC, s.created_at DESC").
		Limit(limit).
		Scan(&stories).Error; err != nil {
		return nil, 0, fmt.Errorf("list planning backlog stories: %w", err)
	}
	return stories, int(total), nil
}
