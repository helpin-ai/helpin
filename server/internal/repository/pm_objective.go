package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PMObjectiveRepository handles DB operations for objectives.
type PMObjectiveRepository struct {
	db *gorm.DB
}

// NewPMObjectiveRepository creates a new PMObjectiveRepository.
func NewPMObjectiveRepository(db *gorm.DB) *PMObjectiveRepository {
	return &PMObjectiveRepository{db: db}
}

// List returns objectives in a workspace with optional filters.
func (r *PMObjectiveRepository) List(ctx context.Context, workspaceID string, filters model.PMObjectiveListFilters) ([]model.PMObjective, error) {
	query := r.db.WithContext(ctx).Model(&model.PMObjective{}).Where("workspace_id = ?", workspaceID)

	if filters.ObjectiveType != nil && *filters.ObjectiveType != "" {
		query = query.Where("objective_type = ?", *filters.ObjectiveType)
	}
	if filters.State != nil && *filters.State != "" {
		query = query.Where("state = ?", *filters.State)
	}
	if filters.Archived != nil {
		query = query.Where("archived = ?", *filters.Archived)
	}
	if filters.TeamID != nil && *filters.TeamID != "" {
		query = query.Joins("JOIN pm_objective_teams pot ON pot.objective_id = pm_objectives.id").Where("pot.team_id = ?", *filters.TeamID)
	}
	if filters.LabelID != nil && *filters.LabelID != "" {
		query = query.Joins("JOIN pm_objective_labels pol ON pol.objective_id = pm_objectives.id").Where("pol.label_id = ?", *filters.LabelID)
	}

	var objectives []model.PMObjective
	if err := query.Order("position ASC, created_at DESC").Find(&objectives).Error; err != nil {
		return nil, fmt.Errorf("list objectives: %w", err)
	}
	return objectives, nil
}

// GetByID returns an objective with all details.
func (r *PMObjectiveRepository) GetByID(ctx context.Context, id string) (*model.ObjectiveWithDetails, error) {
	var obj model.PMObjective
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&obj).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get objective: %w", err)
	}

	teamIDs, err := r.listTeamIDs(ctx, id)
	if err != nil {
		return nil, err
	}
	ownerIDs, err := r.listOwnerIDs(ctx, id)
	if err != nil {
		return nil, err
	}
	ownerMemberIDs, err := r.listOwnerMemberIDs(ctx, id)
	if err != nil {
		return nil, err
	}
	labels, err := r.listLabels(ctx, id)
	if err != nil {
		return nil, err
	}
	keyResults, err := r.listKeyResults(ctx, id)
	if err != nil {
		return nil, err
	}
	epics, err := r.listEpics(ctx, id)
	if err != nil {
		return nil, err
	}
	stats, err := r.ComputeStats(ctx, id)
	if err != nil {
		return nil, err
	}

	return &model.ObjectiveWithDetails{
		Objective:      obj,
		Teams:          teamIDs,
		Owners:         ownerIDs,
		OwnerMemberIDs: ownerMemberIDs,
		Labels:         labels,
		KeyResults:     keyResults,
		Epics:          epics,
		Stats:          stats,
	}, nil
}

// Create inserts an objective.
func (r *PMObjectiveRepository) Create(ctx context.Context, obj *model.PMObjective) error {
	if err := r.db.WithContext(ctx).Create(obj).Error; err != nil {
		return fmt.Errorf("create objective: %w", err)
	}
	return nil
}

// Update saves an objective.
func (r *PMObjectiveRepository) Update(ctx context.Context, obj *model.PMObjective) error {
	if err := r.db.WithContext(ctx).Save(obj).Error; err != nil {
		return fmt.Errorf("update objective: %w", err)
	}
	return nil
}

// Delete archives an objective.
func (r *PMObjectiveRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.PMObjective{}).
		Where("id = ?", id).
		Update("archived", true).Error; err != nil {
		return fmt.Errorf("archive objective: %w", err)
	}
	return nil
}

// ── Teams ──────────────────────────────────────────────────────────

func (r *PMObjectiveRepository) AddTeam(ctx context.Context, objectiveID, teamID string) error {
	link := model.PMObjectiveTeam{ObjectiveID: objectiveID, TeamID: teamID}
	if err := r.db.WithContext(ctx).Create(&link).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil
		}
		return fmt.Errorf("add objective team: %w", err)
	}
	return nil
}

func (r *PMObjectiveRepository) RemoveTeam(ctx context.Context, objectiveID, teamID string) error {
	if err := r.db.WithContext(ctx).
		Delete(&model.PMObjectiveTeam{}, "objective_id = ? AND team_id = ?", objectiveID, teamID).Error; err != nil {
		return fmt.Errorf("remove objective team: %w", err)
	}
	return nil
}

func (r *PMObjectiveRepository) ReplaceTeams(ctx context.Context, objectiveID string, teamIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.PMObjectiveTeam{}, "objective_id = ?", objectiveID).Error; err != nil {
			return fmt.Errorf("clear objective teams: %w", err)
		}
		for _, teamID := range teamIDs {
			link := model.PMObjectiveTeam{ObjectiveID: objectiveID, TeamID: teamID}
			if err := tx.Create(&link).Error; err != nil {
				return fmt.Errorf("set objective teams: %w", err)
			}
		}
		return nil
	})
}

// ── Owners ─────────────────────────────────────────────────────────

func (r *PMObjectiveRepository) AddOwner(ctx context.Context, objectiveID, workspaceMemberID string) error {
	link := model.PMObjectiveOwner{ObjectiveID: objectiveID, WorkspaceMemberID: workspaceMemberID}
	if err := r.db.WithContext(ctx).Create(&link).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil
		}
		return fmt.Errorf("add objective owner: %w", err)
	}
	return nil
}

func (r *PMObjectiveRepository) RemoveOwner(ctx context.Context, objectiveID, workspaceMemberID string) error {
	if err := r.db.WithContext(ctx).
		Delete(&model.PMObjectiveOwner{}, "objective_id = ? AND workspace_member_id = ?", objectiveID, workspaceMemberID).Error; err != nil {
		return fmt.Errorf("remove objective owner: %w", err)
	}
	return nil
}

func (r *PMObjectiveRepository) ReplaceOwners(ctx context.Context, objectiveID string, workspaceMemberIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.PMObjectiveOwner{}, "objective_id = ?", objectiveID).Error; err != nil {
			return fmt.Errorf("clear objective owners: %w", err)
		}
		for _, workspaceMemberID := range workspaceMemberIDs {
			link := model.PMObjectiveOwner{ObjectiveID: objectiveID, WorkspaceMemberID: workspaceMemberID}
			if err := tx.Create(&link).Error; err != nil {
				return fmt.Errorf("set objective owners: %w", err)
			}
		}
		return nil
	})
}

// ── Labels ─────────────────────────────────────────────────────────

func (r *PMObjectiveRepository) AddLabel(ctx context.Context, objectiveID, labelID string) error {
	link := model.PMObjectiveLabel{ObjectiveID: objectiveID, LabelID: labelID}
	if err := r.db.WithContext(ctx).Create(&link).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil
		}
		return fmt.Errorf("add objective label: %w", err)
	}
	return nil
}

func (r *PMObjectiveRepository) RemoveLabel(ctx context.Context, objectiveID, labelID string) error {
	if err := r.db.WithContext(ctx).
		Delete(&model.PMObjectiveLabel{}, "objective_id = ? AND label_id = ?", objectiveID, labelID).Error; err != nil {
		return fmt.Errorf("remove objective label: %w", err)
	}
	return nil
}

func (r *PMObjectiveRepository) ReplaceLabels(ctx context.Context, objectiveID string, labelIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.PMObjectiveLabel{}, "objective_id = ?", objectiveID).Error; err != nil {
			return fmt.Errorf("clear objective labels: %w", err)
		}
		for _, labelID := range labelIDs {
			link := model.PMObjectiveLabel{ObjectiveID: objectiveID, LabelID: labelID}
			if err := tx.Create(&link).Error; err != nil {
				return fmt.Errorf("set objective labels: %w", err)
			}
		}
		return nil
	})
}

// ── Epics ──────────────────────────────────────────────────────────

func (r *PMObjectiveRepository) AddEpic(ctx context.Context, objectiveID, epicID string) error {
	link := model.PMEpicObjective{EpicID: epicID, ObjectiveID: objectiveID}
	if err := r.db.WithContext(ctx).Create(&link).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil
		}
		return fmt.Errorf("add objective epic: %w", err)
	}
	return nil
}

func (r *PMObjectiveRepository) RemoveEpic(ctx context.Context, objectiveID, epicID string) error {
	if err := r.db.WithContext(ctx).
		Delete(&model.PMEpicObjective{}, "objective_id = ? AND epic_id = ?", objectiveID, epicID).Error; err != nil {
		return fmt.Errorf("remove objective epic: %w", err)
	}
	return nil
}

func (r *PMObjectiveRepository) ReplaceEpics(ctx context.Context, objectiveID string, epicIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.PMEpicObjective{}, "objective_id = ?", objectiveID).Error; err != nil {
			return fmt.Errorf("clear objective epics: %w", err)
		}
		for _, epicID := range epicIDs {
			link := model.PMEpicObjective{EpicID: epicID, ObjectiveID: objectiveID}
			if err := tx.Create(&link).Error; err != nil {
				return fmt.Errorf("set objective epics: %w", err)
			}
		}
		return nil
	})
}

// ── Stats ──────────────────────────────────────────────────────────

// ComputeStats computes aggregate stats for an objective.
func (r *PMObjectiveRepository) ComputeStats(ctx context.Context, objectiveID string) (model.PMObjectiveStats, error) {
	stats := model.PMObjectiveStats{}

	// Key result stats
	var krRows []struct {
		Count  int
		AvgPct float64
	}
	if err := r.db.WithContext(ctx).
		Table("pm_key_results").
		Select("COUNT(*) AS count, COALESCE(AVG(progress), 0) AS avg_pct").
		Where("objective_id = ?", objectiveID).
		Scan(&krRows).Error; err != nil {
		return stats, fmt.Errorf("compute kr stats: %w", err)
	}
	if len(krRows) > 0 {
		stats.KeyResultCount = krRows[0].Count
		stats.KeyResultAvgPct = krRows[0].AvgPct
	}

	// Epic stats — count epics linked + aggregate their story counts
	epicIDs, err := r.listEpicIDs(ctx, objectiveID)
	if err != nil {
		return stats, err
	}
	stats.EpicCount = len(epicIDs)

	if len(epicIDs) > 0 {
		// Count completed epics
		var doneCount int64
		if err := r.db.WithContext(ctx).
			Model(&model.PMEpic{}).
			Where("id IN ? AND completed = true", epicIDs).
			Count(&doneCount).Error; err != nil {
			return stats, fmt.Errorf("count done epics: %w", err)
		}
		stats.EpicDoneCount = int(doneCount)

		// Aggregate story counts across linked epics
		var storyRow struct {
			Total int
			Done  int
		}
		if err := r.db.WithContext(ctx).
			Table("pm_stories s").
			Select("COUNT(*) AS total, COUNT(CASE WHEN ws.state_type = 'done' THEN 1 END) AS done").
			Joins("JOIN pm_workflow_states ws ON ws.id = s.workflow_state_id").
			Where("s.epic_id IN ? AND s.archived = false", epicIDs).
			Scan(&storyRow).Error; err != nil {
			return stats, fmt.Errorf("compute epic story stats: %w", err)
		}
		stats.EpicStoryCount = storyRow.Total
		stats.EpicDoneStories = storyRow.Done
		if storyRow.Total > 0 {
			stats.EpicProgressPct = float64(storyRow.Done) / float64(storyRow.Total) * 100
		}
	}

	return stats, nil
}

// ── Private helpers ────────────────────────────────────────────────

func (r *PMObjectiveRepository) listTeamIDs(ctx context.Context, objectiveID string) ([]string, error) {
	var ids []string
	if err := r.db.WithContext(ctx).
		Model(&model.PMObjectiveTeam{}).
		Where("objective_id = ?", objectiveID).
		Pluck("team_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("list objective teams: %w", err)
	}
	return ids, nil
}

func (r *PMObjectiveRepository) listOwnerIDs(ctx context.Context, objectiveID string) ([]string, error) {
	var rows []struct {
		OwnerRef string
	}
	if err := r.db.WithContext(ctx).
		Table("pm_objective_owners pmo").
		Select("COALESCE(CAST(wm.user_id AS TEXT), CAST(pmo.workspace_member_id AS TEXT)) AS owner_ref").
		Joins("JOIN workspace_members wm ON wm.id = pmo.workspace_member_id").
		Where("pmo.objective_id = ?", objectiveID).
		Order("pmo.created_at ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list objective owners: %w", err)
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.OwnerRef)
	}
	return ids, nil
}

func (r *PMObjectiveRepository) listOwnerMemberIDs(ctx context.Context, objectiveID string) ([]string, error) {
	var ids []string
	if err := r.db.WithContext(ctx).
		Model(&model.PMObjectiveOwner{}).
		Where("objective_id = ?", objectiveID).
		Order("created_at ASC").
		Pluck("workspace_member_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("list objective owner member ids: %w", err)
	}
	return ids, nil
}

func (r *PMObjectiveRepository) listLabels(ctx context.Context, objectiveID string) ([]model.PMLabel, error) {
	var labels []model.PMLabel
	if err := r.db.WithContext(ctx).
		Table("pm_labels l").
		Select("l.*").
		Joins("JOIN pm_objective_labels pol ON pol.label_id = l.id").
		Where("pol.objective_id = ?", objectiveID).
		Order("l.name ASC").
		Find(&labels).Error; err != nil {
		return nil, fmt.Errorf("list objective labels: %w", err)
	}
	return labels, nil
}

func (r *PMObjectiveRepository) listKeyResults(ctx context.Context, objectiveID string) ([]model.PMKeyResult, error) {
	var results []model.PMKeyResult
	if err := r.db.WithContext(ctx).
		Where("objective_id = ?", objectiveID).
		Order("position ASC, created_at ASC").
		Find(&results).Error; err != nil {
		return nil, fmt.Errorf("list key results: %w", err)
	}
	return results, nil
}

func (r *PMObjectiveRepository) listEpicIDs(ctx context.Context, objectiveID string) ([]string, error) {
	var ids []string
	if err := r.db.WithContext(ctx).
		Model(&model.PMEpicObjective{}).
		Where("objective_id = ?", objectiveID).
		Pluck("epic_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("list objective epic ids: %w", err)
	}
	return ids, nil
}

func (r *PMObjectiveRepository) listEpics(ctx context.Context, objectiveID string) ([]model.EpicWithStats, error) {
	epicIDs, err := r.listEpicIDs(ctx, objectiveID)
	if err != nil {
		return nil, err
	}
	if len(epicIDs) == 0 {
		return []model.EpicWithStats{}, nil
	}

	var epics []model.PMEpic
	if err := r.db.WithContext(ctx).Where("id IN ?", epicIDs).Find(&epics).Error; err != nil {
		return nil, fmt.Errorf("list objective epics: %w", err)
	}

	result := make([]model.EpicWithStats, 0, len(epics))
	for _, epic := range epics {
		// Compute simple stats per epic
		stats := model.PMEpicStats{}
		var rows []struct {
			StateType string
			Count     int
			Points    int
		}
		if err := r.db.WithContext(ctx).
			Table("pm_stories s").
			Select("ws.state_type AS state_type, COUNT(*) AS count, COALESCE(SUM(COALESCE(s.estimate, 0)), 0) AS points").
			Joins("JOIN pm_workflow_states ws ON ws.id = s.workflow_state_id").
			Where("s.epic_id = ? AND s.archived = false", epic.ID).
			Group("ws.state_type").
			Scan(&rows).Error; err != nil {
			return nil, fmt.Errorf("compute epic stats: %w", err)
		}
		for _, row := range rows {
			stats.StoryCount += row.Count
			stats.TotalPoints += row.Points
			switch row.StateType {
			case model.PMStateTypeDone:
				stats.DoneStoryCount += row.Count
				stats.DonePoints += row.Points
			case model.PMStateTypeStarted:
				stats.InProgressCount += row.Count
			case model.PMStateTypeUnstarted, model.PMStateTypeBacklog:
				stats.UnstartedCount += row.Count
			}
		}

		// Get epic labels
		var labels []model.PMLabel
		r.db.WithContext(ctx).
			Table("pm_labels l").
			Select("l.*").
			Joins("JOIN pm_epic_labels pel ON pel.label_id = l.id").
			Where("pel.epic_id = ?", epic.ID).
			Order("l.name ASC").
			Find(&labels)

		result = append(result, model.EpicWithStats{Epic: epic, Labels: labels, Stats: stats})
	}

	return result, nil
}
