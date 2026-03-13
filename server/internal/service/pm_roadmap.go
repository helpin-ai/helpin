package service

import (
	"context"
	"log/slog"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// PMRoadmapService provides roadmap data by combining epics with objectives.
type PMRoadmapService struct {
	epicService *PMEpicService
	roadmapRepo *repository.PMRoadmapRepository
	logger      *slog.Logger
}

// NewPMRoadmapService creates a new PMRoadmapService.
func NewPMRoadmapService(epicService *PMEpicService, roadmapRepo *repository.PMRoadmapRepository) *PMRoadmapService {
	return &PMRoadmapService{
		epicService: epicService,
		roadmapRepo: roadmapRepo,
		logger:      slog.Default().With("service", "pm_roadmap"),
	}
}

// GetData returns roadmap data: epics with objective refs + all objectives.
func (s *PMRoadmapService) GetData(ctx context.Context, workspaceID string, filters model.RoadmapFilters) (*model.RoadmapData, error) {
	// 1. List epics (non-archived) using existing epic service.
	notArchived := false
	epicFilters := model.PMEpicListFilters{
		TeamID:   filters.TeamID,
		Archived: &notArchived,
	}

	allEpics, err := s.epicService.List(ctx, workspaceID, epicFilters)
	if err != nil {
		return nil, err
	}

	// 2. Apply roadmap-specific filters (health, completed).
	var filteredEpics []model.EpicWithStats
	for _, e := range allEpics {
		if !filters.ShowCompleted && e.Epic.Completed {
			continue
		}
		if filters.Health != nil && e.Epic.Health != *filters.Health {
			continue
		}
		filteredEpics = append(filteredEpics, e)
	}

	// 3. Get epic → objective mappings from join table.
	epicIDs := make([]string, len(filteredEpics))
	for i, e := range filteredEpics {
		epicIDs[i] = e.Epic.ID
	}

	epicObjMap, err := s.roadmapRepo.ListEpicObjectives(ctx, epicIDs)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to load epic-objective mappings", "error", err)
		return nil, err
	}

	// 4. Assemble roadmap epics; filter by objective if requested.
	var roadmapEpics []model.RoadmapEpic
	for _, e := range filteredEpics {
		objs := epicObjMap[e.Epic.ID]
		if objs == nil {
			objs = []model.RoadmapObjectiveRef{}
		}

		if filters.ObjectiveID != nil {
			linked := false
			for _, o := range objs {
				if o.ID == *filters.ObjectiveID {
					linked = true
					break
				}
			}
			if !linked {
				continue
			}
		}

		roadmapEpics = append(roadmapEpics, model.RoadmapEpic{
			Epic:            e.Epic,
			Labels:          e.Labels,
			Stats:           e.Stats,
			SuggestedHealth: e.SuggestedHealth,
			Objectives:      objs,
		})
	}
	if roadmapEpics == nil {
		roadmapEpics = []model.RoadmapEpic{}
	}

	// 5. List all objectives for grouping/filtering UI.
	objectives, err := s.roadmapRepo.ListObjectives(ctx, workspaceID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to load objectives", "error", err)
		return nil, err
	}
	if objectives == nil {
		objectives = []model.PMObjective{}
	}

	return &model.RoadmapData{
		Epics:      roadmapEpics,
		Objectives: objectives,
	}, nil
}
