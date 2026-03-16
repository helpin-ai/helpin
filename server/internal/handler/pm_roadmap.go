package handler

import (
	"net/http"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// PMRoadmapHandler handles the roadmap endpoint.
type PMRoadmapHandler struct {
	roadmapService *service.PMRoadmapService
}

// NewPMRoadmapHandler creates a new PMRoadmapHandler.
func NewPMRoadmapHandler(roadmapService *service.PMRoadmapService) *PMRoadmapHandler {
	return &PMRoadmapHandler{roadmapService: roadmapService}
}

// Get handles GET /api/pm/roadmap.
func (h *PMRoadmapHandler) Get(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	filters := model.RoadmapFilters{
		TeamID:        queryStringPtr(r, "team_id"),
		ObjectiveID:   queryStringPtr(r, "objective_id"),
		Health:        queryStringPtr(r, "health"),
		ShowCompleted: r.URL.Query().Get("show_completed") == "true",
	}

	data, err := h.roadmapService.GetData(r.Context(), workspaceID, filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, data)
}
