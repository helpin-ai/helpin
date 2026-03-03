package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/d4interactive/teampulse/server/internal/middleware"
	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/service"
)

// SprintHandler handles sprint HTTP requests.
type SprintHandler struct {
	sprintService *service.SprintService
}

// NewSprintHandler creates a new SprintHandler.
func NewSprintHandler(sprintService *service.SprintService) *SprintHandler {
	return &SprintHandler{sprintService: sprintService}
}

// List handles GET /api/sprints?quarter_id=xxx.
func (h *SprintHandler) List(w http.ResponseWriter, r *http.Request) {
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
		sprints = []model.Sprint{}
	}

	writeJSON(w, http.StatusOK, sprints)
}

// Get handles GET /api/sprints/{id}.
func (h *SprintHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sprint, err := h.sprintService.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, sprint)
}

// GetIndividualChecks handles GET /api/sprints/{id}/checks.
func (h *SprintHandler) GetIndividualChecks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	checks, err := h.sprintService.GetIndividualChecks(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if checks == nil {
		checks = []model.IndividualCheck{}
	}

	writeJSON(w, http.StatusOK, checks)
}

// UpsertIndividualCheck handles POST /api/sprints/{id}/checks.
func (h *SprintHandler) UpsertIndividualCheck(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.UpsertCheckRequest
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

// Lock handles POST /api/sprints/{id}/lock.
func (h *SprintHandler) Lock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	sprint, err := h.sprintService.Lock(r.Context(), id, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, sprint)
}

// Unlock handles POST /api/sprints/{id}/unlock.
func (h *SprintHandler) Unlock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sprint, err := h.sprintService.Unlock(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, sprint)
}
