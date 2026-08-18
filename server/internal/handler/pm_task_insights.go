package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type PMTaskInsightsHandler struct {
	service *service.PMTaskInsightsService
}

func NewPMTaskInsightsHandler(insightsService *service.PMTaskInsightsService) *PMTaskInsightsHandler {
	return &PMTaskInsightsHandler{service: insightsService}
}

func (h *PMTaskInsightsHandler) ListUpdates(w http.ResponseWriter, r *http.Request) {
	response, err := h.service.ListUpdates(
		r.Context(),
		getWorkspaceID(r),
		chi.URLParam(r, "id"),
		middleware.GetUserID(r.Context()),
		r.URL.Query().Get("filter"),
		r.URL.Query().Get("cursor"),
		queryInt(r, "limit", 50),
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *PMTaskInsightsHandler) UpdateReadState(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateTaskReadStateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	state, err := h.service.UpdateReadState(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), middleware.GetUserID(r.Context()), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (h *PMTaskInsightsHandler) GetStandingBrief(w http.ResponseWriter, r *http.Request) {
	brief, err := h.service.GetStandingBrief(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, brief)
}

func (h *PMTaskInsightsHandler) RefreshStandingBrief(w http.ResponseWriter, r *http.Request) {
	brief, err := h.service.RefreshStandingBrief(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, brief)
}

func (h *PMTaskInsightsHandler) DismissStandingBriefSuggestion(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(chi.URLParam(r, "key"))
	if err := h.service.DismissStandingBriefSuggestion(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), middleware.GetUserID(r.Context()), key); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
