package model

// RoadmapObjectiveRef is a lightweight objective reference for roadmap grouping.
type RoadmapObjectiveRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// RoadmapEpic is an alias for EpicWithStats (which now includes objectives).
type RoadmapEpic = EpicWithStats

// RoadmapData is the full roadmap response.
type RoadmapData struct {
	Epics      []EpicWithStats `json:"epics"`
	Objectives []PMObjective   `json:"objectives"`
}

// RoadmapFilters defines query filters for the roadmap endpoint.
type RoadmapFilters struct {
	TeamID        *string
	ObjectiveID   *string
	Health        *string
	ShowCompleted bool
}
