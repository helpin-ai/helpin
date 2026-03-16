package repository

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// PMRoadmapRepository handles roadmap-specific queries.
type PMRoadmapRepository struct {
	db *gorm.DB
}

// NewPMRoadmapRepository creates a new PMRoadmapRepository.
func NewPMRoadmapRepository(db *gorm.DB) *PMRoadmapRepository {
	return &PMRoadmapRepository{db: db}
}

// epicObjectiveRow is an internal struct for the join query result.
type epicObjectiveRow struct {
	EpicID        string `gorm:"column:epic_id"`
	ObjectiveID   string `gorm:"column:objective_id"`
	ObjectiveName string `gorm:"column:objective_name"`
}

// ListEpicObjectives returns a map of epic_id → objective references.
func (r *PMRoadmapRepository) ListEpicObjectives(ctx context.Context, epicIDs []string) (map[string][]model.RoadmapObjectiveRef, error) {
	if len(epicIDs) == 0 {
		return map[string][]model.RoadmapObjectiveRef{}, nil
	}

	var rows []epicObjectiveRow
	err := r.db.WithContext(ctx).
		Table("pm_epic_objectives peo").
		Select("peo.epic_id, peo.objective_id, o.name AS objective_name").
		Joins("JOIN pm_objectives o ON o.id = peo.objective_id").
		Where("peo.epic_id IN ?", epicIDs).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string][]model.RoadmapObjectiveRef, len(epicIDs))
	for _, row := range rows {
		result[row.EpicID] = append(result[row.EpicID], model.RoadmapObjectiveRef{
			ID:   row.ObjectiveID,
			Name: row.ObjectiveName,
		})
	}
	return result, nil
}

// ListObjectives returns all non-archived objectives for a workspace.
func (r *PMRoadmapRepository) ListObjectives(ctx context.Context, workspaceID string) ([]model.PMObjective, error) {
	var objectives []model.PMObjective
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND archived = false", workspaceID).
		Order("position ASC, created_at DESC").
		Find(&objectives).Error
	return objectives, err
}
