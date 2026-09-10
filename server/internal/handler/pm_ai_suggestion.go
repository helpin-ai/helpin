package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type PMAISuggestionHandler struct {
	service *service.PMAISuggestionService
}

func NewPMAISuggestionHandler(s *service.PMAISuggestionService) *PMAISuggestionHandler {
	return &PMAISuggestionHandler{service: s}
}

func (h *PMAISuggestionHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	if page > 1000000 {
		page = 1000000
	}
	items, total, err := h.service.List(r.Context(), getWorkspaceID(r), page)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	if items == nil {
		items = []model.PMAISuggestionItem{}
	}
	writeJSON(w, http.StatusOK, model.PaginatedResponse{Data: items, Total: int(total), Page: page, PerPage: 25, TotalPages: int((total + 24) / 25)})
}
func (h *PMAISuggestionHandler) Get(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.Get(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	if item == nil {
		writeError(w, http.StatusNotFound, "Suggestion not found")
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (h *PMAISuggestionHandler) Decide(w http.ResponseWriter, r *http.Request) {
	var req model.PMAISuggestionDecision
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid review request")
		return
	}
	result, err := h.service.Decide(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), chi.URLParam(r, "decision"), req)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	if result == nil {
		writeError(w, http.StatusNotFound, "Suggestion not found")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *PMAISuggestionHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrPMAISuggestionForbidden):
		writeError(w, http.StatusForbidden, "You do not have access to these suggestions")
	case errors.Is(err, service.ErrPMAISuggestionInput):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, repository.ErrPMAISuggestionStale), errors.Is(err, repository.ErrCRMSituationStale):
		writeError(w, http.StatusConflict, "This suggestion has changed. Refresh before reviewing it.")
	default:
		slog.ErrorContext(r.Context(), "PM AI suggestion request failed", "workspace_id", getWorkspaceID(r), "error", err)
		writeError(w, http.StatusInternalServerError, "Unable to load or update suggestion")
	}
}
