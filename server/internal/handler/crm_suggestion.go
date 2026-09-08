package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// CRMSuggestionHandler handles CRM suggestion HTTP endpoints.
type CRMSuggestionHandler struct {
	suggestionService *service.CRMSuggestionService
}

// NewCRMSuggestionHandler creates a new CRMSuggestionHandler.
func NewCRMSuggestionHandler(suggestionService *service.CRMSuggestionService) *CRMSuggestionHandler {
	return &CRMSuggestionHandler{suggestionService: suggestionService}
}

// List handles GET /api/crm/suggestions.
func (h *CRMSuggestionHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	filters := model.CRMSuggestionListFilters{
		UserID:         queryStringPtr(r, "user_id"),
		SuggestionType: queryStringPtr(r, "suggestion_type"),
		ObjectType:     queryStringPtr(r, "object_type"),
		ObjectID:       queryStringPtr(r, "object_id"),
		Status:         queryStringPtr(r, "status"),
	}
	pagination := queryPagination(r)

	suggestions, total, err := h.suggestionService.List(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if suggestions == nil {
		suggestions = []model.CRMSuggestion{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  suggestions,
		"total": total,
		"page":  pagination.Page,
	})
}

// Get handles GET /api/crm/suggestions/{id}.
func (h *CRMSuggestionHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	suggestion, err := h.suggestionService.GetByID(r.Context(), getWorkspaceID(r), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, suggestion)
}

// Create handles POST /api/crm/suggestions.
func (h *CRMSuggestionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCRMSuggestionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	workspaceID := getWorkspaceID(r)
	if req.WorkspaceID != "" && req.WorkspaceID != workspaceID {
		writeError(w, http.StatusBadRequest, "workspace does not match request scope")
		return
	}
	req.WorkspaceID = workspaceID
	suggestion, err := h.suggestionService.Create(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, suggestion)
}

// Update handles PUT /api/crm/suggestions/{id}.
func (h *CRMSuggestionHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req model.UpdateCRMSuggestionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	suggestion, err := h.suggestionService.Update(r.Context(), getWorkspaceID(r), id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, suggestion)
}

// Delete handles DELETE /api/crm/suggestions/{id}.
func (h *CRMSuggestionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.suggestionService.Delete(r.Context(), getWorkspaceID(r), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "suggestion deleted"})
}

// Accept handles POST /api/crm/suggestions/{id}/accept.
func (h *CRMSuggestionHandler) Accept(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var edits map[string]interface{}
	// An absent body is valid; malformed edits must not execute the old payload.
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&edits); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid suggestion edits")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid suggestion edits")
		return
	}

	suggestion, err := h.suggestionService.AcceptSuggestion(r.Context(), getWorkspaceID(r), id, edits)
	if err != nil {
		if errors.Is(err, service.ErrCRMSuggestionStale) {
			writeError(w, http.StatusConflict, "this suggestion changed or is no longer pending")
			return
		}
		slog.ErrorContext(r.Context(), "suggestion acceptance failed", "error", err, "workspace_id", getWorkspaceID(r), "suggestion_id", id)
		writeError(w, http.StatusBadRequest, "the suggestion could not be accepted; reload it and check its context")
		return
	}
	writeJSON(w, http.StatusOK, suggestion)
}

// Dismiss handles POST /api/crm/suggestions/{id}/dismiss.
func (h *CRMSuggestionHandler) Dismiss(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "a dismissal reason is required")
		return
	}
	suggestion, err := h.suggestionService.DismissSuggestion(r.Context(), getWorkspaceID(r), id, req.Reason)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, suggestion)
}
