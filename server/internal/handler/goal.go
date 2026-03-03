package handler

import (
	"net/http"

	"github.com/d4interactive/teampulse/server/internal/middleware"
	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/service"
)

// GoalHandler handles goal HTTP requests.
type GoalHandler struct {
	goalService *service.GoalService
}

// NewGoalHandler creates a new GoalHandler.
func NewGoalHandler(goalService *service.GoalService) *GoalHandler {
	return &GoalHandler{goalService: goalService}
}

// List handles GET /api/goals?workspace_id=xxx&quarter_id=xxx.
func (h *GoalHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	quarterID := r.URL.Query().Get("quarter_id")
	if workspaceID == "" || quarterID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id and quarter_id are required")
		return
	}

	goals, err := h.goalService.List(r.Context(), workspaceID, quarterID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if goals == nil {
		goals = []model.CompanyGoalWithContributions{}
	}

	writeJSON(w, http.StatusOK, goals)
}

// ListSprintGoals handles GET /api/goals/sprint?sprint_id=xxx[&team_id=yyy].
func (h *GoalHandler) ListSprintGoals(w http.ResponseWriter, r *http.Request) {
	sprintID := r.URL.Query().Get("sprint_id")
	if sprintID == "" {
		writeError(w, http.StatusBadRequest, "sprint_id is required")
		return
	}

	teamID := r.URL.Query().Get("team_id")
	var teamFilter *string
	if teamID != "" {
		teamFilter = &teamID
	}

	goals, err := h.goalService.ListSprintGoals(r.Context(), sprintID, teamFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if goals == nil {
		goals = []model.SprintGoal{}
	}

	writeJSON(w, http.StatusOK, goals)
}

// Create handles POST /api/goals.
func (h *GoalHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.CreateGoalRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	goal, err := h.goalService.Create(r.Context(), req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, goal)
}

// UpsertSprintGoal handles POST /api/goals/sprint.
func (h *GoalHandler) UpsertSprintGoal(w http.ResponseWriter, r *http.Request) {
	var req model.UpsertSprintGoalRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	goal, err := h.goalService.UpsertSprintGoal(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, goal)
}
