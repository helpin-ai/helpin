package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// PMLabelHandler handles PM label HTTP endpoints.
type PMLabelHandler struct {
	labelService *service.PMLabelService
}

// NewPMLabelHandler creates a new PMLabelHandler.
func NewPMLabelHandler(labelService *service.PMLabelService) *PMLabelHandler {
	return &PMLabelHandler{labelService: labelService}
}

// List handles GET /api/pm/labels.
func (h *PMLabelHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	includeShared, err := queryBoolDefault(r, "include_shared", true)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid include_shared query param")
		return
	}
	labels, err := h.labelService.ListByWorkspace(r.Context(), workspaceID, queryStringPtr(r, "team_id"), includeShared)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if labels == nil {
		labels = []model.PMLabel{}
	}
	writeJSON(w, http.StatusOK, labels)
}

// ListWithStats handles GET /api/pm/labels/stats.
func (h *PMLabelHandler) ListWithStats(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	archived, err := queryBoolPtr(r, "archived")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid archived query param")
		return
	}
	includeShared, err := queryBoolDefault(r, "include_shared", true)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid include_shared query param")
		return
	}
	results, err := h.labelService.ListWithStats(r.Context(), workspaceID, queryStringPtr(r, "team_id"), includeShared, archived)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// Create handles POST /api/pm/labels.
func (h *PMLabelHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateLabelRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	label, err := h.labelService.Create(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, label)
}

// Update handles PUT /api/pm/labels/{id}.
func (h *PMLabelHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req model.UpdateLabelRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	label, err := h.labelService.Update(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, label)
}

// Delete handles DELETE /api/pm/labels/{id}.
func (h *PMLabelHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.labelService.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "label deleted"})
}

func queryBoolDefault(r *http.Request, key string, fallback bool) (bool, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback, nil
	}
	return strconv.ParseBool(value)
}
