package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// RewardDraftHandler handles goal draft HTTP requests.
type RewardDraftHandler struct {
	draftService *service.RewardDraftService
}

// NewRewardDraftHandler creates a new RewardDraftHandler.
func NewRewardDraftHandler(draftService *service.RewardDraftService) *RewardDraftHandler {
	return &RewardDraftHandler{draftService: draftService}
}

// Create handles POST /api/rewards/drafts.
func (h *RewardDraftHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.CreateRewardDraftRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	draft, err := h.draftService.Create(r.Context(), req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, draft)
}

// List handles GET /api/rewards/drafts?workspace_id=xxx&quarter_id=xxx.
func (h *RewardDraftHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	quarterID := r.URL.Query().Get("quarter_id")
	if workspaceID == "" || quarterID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id and quarter_id are required")
		return
	}

	drafts, err := h.draftService.List(r.Context(), workspaceID, quarterID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if drafts == nil {
		drafts = []model.RewardGoalDraft{}
	}

	writeJSON(w, http.StatusOK, drafts)
}

// Get handles GET /api/rewards/drafts/{id}.
func (h *RewardDraftHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	draft, err := h.draftService.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, draft)
}

// Update handles PUT /api/rewards/drafts/{id}.
func (h *RewardDraftHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req model.UpdateRewardDraftRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	draft, err := h.draftService.Update(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, draft)
}

// Delete handles DELETE /api/rewards/drafts/{id}.
func (h *RewardDraftHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.draftService.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "draft deleted"})
}
