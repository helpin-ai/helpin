package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func objectiveErrorStatus(err error) int {
	var forbidden *model.ErrForbidden
	if errors.As(err, &forbidden) {
		return http.StatusForbidden
	}
	return http.StatusBadRequest
}

// PMObjectiveHandler handles PM objective HTTP endpoints.
type PMObjectiveHandler struct {
	objectiveService *service.PMObjectiveService
}

// NewPMObjectiveHandler creates a new PMObjectiveHandler.
func NewPMObjectiveHandler(objectiveService *service.PMObjectiveService) *PMObjectiveHandler {
	return &PMObjectiveHandler{objectiveService: objectiveService}
}

// List handles GET /api/pm/objectives.
func (h *PMObjectiveHandler) List(w http.ResponseWriter, r *http.Request) {
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
	filters := model.PMObjectiveListFilters{
		TeamID:        queryStringPtr(r, "team_id"),
		LabelID:       queryStringPtr(r, "label_id"),
		ObjectiveType: queryStringPtr(r, "objective_type"),
		State:         queryStringPtr(r, "state"),
		Archived:      archived,
	}

	objectives, err := h.objectiveService.List(r.Context(), workspaceID, filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if objectives == nil {
		objectives = []model.ObjectiveWithDetails{}
	}
	writeJSON(w, http.StatusOK, objectives)
}

// Create handles POST /api/pm/objectives.
func (h *PMObjectiveHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var req model.CreateObjectiveRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	obj, err := h.objectiveService.Create(r.Context(), req, userID)
	if err != nil {
		writeError(w, objectiveErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, obj)
}

// Get handles GET /api/pm/objectives/{id}.
func (h *PMObjectiveHandler) Get(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	id := chi.URLParam(r, "id")
	obj, err := h.objectiveService.GetByID(r.Context(), id, workspaceID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, obj)
}

// Update handles PUT /api/pm/objectives/{id}.
func (h *PMObjectiveHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	var req model.UpdateObjectiveRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	obj, err := h.objectiveService.Update(r.Context(), id, req, userID)
	if err != nil {
		writeError(w, objectiveErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, obj)
}

// Delete handles DELETE /api/pm/objectives/{id}.
func (h *PMObjectiveHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.objectiveService.Delete(r.Context(), id, userID); err != nil {
		writeError(w, objectiveErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "objective archived"})
}

// AddTeam handles POST /api/pm/objectives/{id}/teams.
func (h *PMObjectiveHandler) AddTeam(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	var body struct {
		TeamID string `json:"team_id"`
	}
	if err := decodeJSON(r, &body); err != nil || body.TeamID == "" {
		writeError(w, http.StatusBadRequest, "team_id is required")
		return
	}
	if err := h.objectiveService.AddTeam(r.Context(), id, body.TeamID, userID); err != nil {
		writeError(w, objectiveErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "team added"})
}

// RemoveTeam handles DELETE /api/pm/objectives/{id}/teams/{teamId}.
func (h *PMObjectiveHandler) RemoveTeam(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	teamID := chi.URLParam(r, "teamId")
	if err := h.objectiveService.RemoveTeam(r.Context(), id, teamID, userID); err != nil {
		writeError(w, objectiveErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "team removed"})
}

// AddOwner handles POST /api/pm/objectives/{id}/owners.
func (h *PMObjectiveHandler) AddOwner(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	var body struct {
		UserID            string `json:"user_id"`
		WorkspaceMemberID string `json:"workspace_member_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ownerRef := body.WorkspaceMemberID
	if ownerRef == "" {
		ownerRef = body.UserID
	}
	if ownerRef == "" {
		writeError(w, http.StatusBadRequest, "workspace_member_id or user_id is required")
		return
	}
	if err := h.objectiveService.AddOwner(r.Context(), id, ownerRef, userID); err != nil {
		writeError(w, objectiveErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "owner added"})
}

// RemoveOwner handles DELETE /api/pm/objectives/{id}/owners/{ownerRef}.
func (h *PMObjectiveHandler) RemoveOwner(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	ownerRef := chi.URLParam(r, "userId")
	if err := h.objectiveService.RemoveOwner(r.Context(), id, ownerRef, userID); err != nil {
		writeError(w, objectiveErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "owner removed"})
}

// AddEpic handles POST /api/pm/objectives/{id}/epics.
func (h *PMObjectiveHandler) AddEpic(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	var body struct {
		EpicID string `json:"epic_id"`
	}
	if err := decodeJSON(r, &body); err != nil || body.EpicID == "" {
		writeError(w, http.StatusBadRequest, "epic_id is required")
		return
	}
	if err := h.objectiveService.AddEpic(r.Context(), id, body.EpicID, userID); err != nil {
		writeError(w, objectiveErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "epic added"})
}

// RemoveEpic handles DELETE /api/pm/objectives/{id}/epics/{epicId}.
func (h *PMObjectiveHandler) RemoveEpic(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	epicID := chi.URLParam(r, "epicId")
	if err := h.objectiveService.RemoveEpic(r.Context(), id, epicID, userID); err != nil {
		writeError(w, objectiveErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "epic removed"})
}

// CreateKeyResult handles POST /api/pm/objectives/{id}/key-results.
func (h *PMObjectiveHandler) CreateKeyResult(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	var req model.CreateKeyResultRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	kr, err := h.objectiveService.CreateKeyResult(r.Context(), id, req, userID)
	if err != nil {
		writeError(w, objectiveErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, kr)
}

// UpdateKeyResult handles PUT /api/pm/key-results/{id}.
func (h *PMObjectiveHandler) UpdateKeyResult(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	var req model.UpdateKeyResultRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	kr, err := h.objectiveService.UpdateKeyResult(r.Context(), id, req, userID)
	if err != nil {
		writeError(w, objectiveErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, kr)
}

// DeleteKeyResult handles DELETE /api/pm/key-results/{id}.
func (h *PMObjectiveHandler) DeleteKeyResult(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.objectiveService.DeleteKeyResult(r.Context(), id, userID); err != nil {
		writeError(w, objectiveErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "key result deleted"})
}

// ListKeyResultActivity handles GET /api/pm/key-results/{id}/activity.
func (h *PMObjectiveHandler) ListKeyResultActivity(w http.ResponseWriter, r *http.Request) {
	pagination := queryPagination(r)
	pagination.Page = max(1, pagination.Page)
	if pagination.PerPage <= 0 {
		pagination.PerPage = 20
	}
	pagination.PerPage = min(100, pagination.PerPage)
	entries, total, err := h.objectiveService.ListKeyResultActivity(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), pagination)
	if err != nil {
		writeError(w, objectiveErrorStatus(err), err.Error())
		return
	}
	if entries == nil {
		entries = []model.ActivityLogEntry{}
	}
	totalPages := int((total + int64(pagination.PerPage) - 1) / int64(pagination.PerPage))
	writeJSON(w, http.StatusOK, model.PaginatedResponse{Data: entries, Total: int(total), Page: pagination.Page, PerPage: pagination.PerPage, TotalPages: totalPages})
}
