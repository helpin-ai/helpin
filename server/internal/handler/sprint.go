package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/d4interactive/teampulse/server/internal/middleware"
	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/service"
)

// RewardSprintHandler handles sprint HTTP requests.
type RewardSprintHandler struct {
	sprintService *service.RewardSprintService
}

// NewRewardSprintHandler creates a new RewardSprintHandler.
func NewRewardSprintHandler(sprintService *service.RewardSprintService) *RewardSprintHandler {
	return &RewardSprintHandler{sprintService: sprintService}
}

// List handles GET /api/rewards/sprints?quarter_id=xxx.
func (h *RewardSprintHandler) List(w http.ResponseWriter, r *http.Request) {
	quarterID := r.URL.Query().Get("quarter_id")
	if quarterID == "" {
		writeError(w, http.StatusBadRequest, "quarter_id is required")
		return
	}

	sprints, err := h.sprintService.List(r.Context(), quarterID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if sprints == nil {
		sprints = []model.RewardSprint{}
	}

	writeJSON(w, http.StatusOK, sprints)
}

// Get handles GET /api/rewards/sprints/{id}.
func (h *RewardSprintHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sprint, err := h.sprintService.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, sprint)
}

// GetIndividualChecks handles GET /api/rewards/sprints/{id}/checks.
func (h *RewardSprintHandler) GetIndividualChecks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	checks, err := h.sprintService.GetIndividualChecks(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if checks == nil {
		checks = []model.RewardIndividualCheck{}
	}

	writeJSON(w, http.StatusOK, checks)
}

// UpsertIndividualCheck handles POST /api/rewards/sprints/{id}/checks.
func (h *RewardSprintHandler) UpsertIndividualCheck(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.UpsertRewardCheckRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Override sprint_id from URL param.
	req.SprintID = chi.URLParam(r, "id")

	check, err := h.sprintService.UpsertIndividualCheck(r.Context(), req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, check)
}

// Lock handles POST /api/rewards/sprints/{id}/lock.
func (h *RewardSprintHandler) Lock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	sprint, err := h.sprintService.Lock(r.Context(), id, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, sprint)
}

// Unlock handles POST /api/rewards/sprints/{id}/unlock.
func (h *RewardSprintHandler) Unlock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sprint, err := h.sprintService.Unlock(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, sprint)
}
