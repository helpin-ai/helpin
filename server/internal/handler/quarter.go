package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/d4interactive/teampulse/server/internal/middleware"
	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/service"
)

// RewardQuarterHandler handles quarter HTTP requests.
type RewardQuarterHandler struct {
	quarterService *service.RewardQuarterService
}

// NewRewardQuarterHandler creates a new RewardQuarterHandler.
func NewRewardQuarterHandler(quarterService *service.RewardQuarterService) *RewardQuarterHandler {
	return &RewardQuarterHandler{quarterService: quarterService}
}

// List handles GET /api/rewards/quarters?workspace_id=xxx.
func (h *RewardQuarterHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	quarters, err := h.quarterService.List(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if quarters == nil {
		quarters = []model.RewardQuarter{}
	}

	writeJSON(w, http.StatusOK, quarters)
}

// Create handles POST /api/rewards/quarters.
func (h *RewardQuarterHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.CreateRewardQuarterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	q, err := h.quarterService.Create(r.Context(), req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, q)
}

// Get handles GET /api/rewards/quarters/{id}.
func (h *RewardQuarterHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	q, err := h.quarterService.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, q)
}

// UpdateStatus handles PATCH /api/rewards/quarters/{id}/status.
func (h *RewardQuarterHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req model.UpdateRewardQuarterStatusRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	q, err := h.quarterService.UpdateStatus(r.Context(), id, req.Status)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, q)
}
