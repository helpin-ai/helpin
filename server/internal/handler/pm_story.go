package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/d4interactive/teampulse/server/internal/middleware"
	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/service"
)

// PMStoryHandler handles PM story HTTP endpoints.
type PMStoryHandler struct {
	storyService *service.PMStoryService
}

// NewPMStoryHandler creates a new PMStoryHandler.
func NewPMStoryHandler(storyService *service.PMStoryService) *PMStoryHandler {
	return &PMStoryHandler{storyService: storyService}
}

// List handles GET /api/pm/stories.
func (h *PMStoryHandler) List(w http.ResponseWriter, r *http.Request) {
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
	filters := model.PMStoryFilters{
		TeamID:          queryStringPtr(r, "team_id"),
		EpicID:          queryStringPtr(r, "epic_id"),
		IterationID:     queryStringPtr(r, "iteration_id"),
		WorkflowID:      queryStringPtr(r, "workflow_id"),
		WorkflowStateID: queryStringPtr(r, "state_id"),
		StoryType:       queryStringPtr(r, "story_type"),
		OwnerID:         queryStringPtr(r, "owner_id"),
		LabelID:         queryStringPtr(r, "label_id"),
		Priority:        queryStringPtr(r, "priority"),
		Archived:        archived,
	}
	pagination := queryPagination(r)

	stories, total, err := h.storyService.List(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if stories == nil {
		stories = []model.PMStory{}
	}

	totalPages := 0
	if pagination.PerPage > 0 {
		totalPages = int((total + int64(pagination.PerPage) - 1) / int64(pagination.PerPage))
	}
	writeJSON(w, http.StatusOK, model.PaginatedResponse{
		Data:       stories,
		Total:      int(total),
		Page:       pagination.Page,
		PerPage:    pagination.PerPage,
		TotalPages: totalPages,
	})
}

// ListBoard handles GET /api/pm/stories/board?workflow_id=...
func (h *PMStoryHandler) ListBoard(w http.ResponseWriter, r *http.Request) {
	workflowID := r.URL.Query().Get("workflow_id")
	if workflowID == "" {
		writeError(w, http.StatusBadRequest, "workflow_id is required")
		return
	}
	columns, err := h.storyService.ListByWorkflowState(r.Context(), workflowID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if columns == nil {
		columns = []model.StoryStateColumn{}
	}
	writeJSON(w, http.StatusOK, columns)
}

// CountByState handles GET /api/pm/stories/counts?workflow_id=...
func (h *PMStoryHandler) CountByState(w http.ResponseWriter, r *http.Request) {
	workflowID := r.URL.Query().Get("workflow_id")
	if workflowID == "" {
		writeError(w, http.StatusBadRequest, "workflow_id is required")
		return
	}
	counts, err := h.storyService.CountByState(r.Context(), workflowID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if counts == nil {
		counts = []model.StoryStateCount{}
	}
	writeJSON(w, http.StatusOK, counts)
}

// Create handles POST /api/pm/stories.
func (h *PMStoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var req model.CreateStoryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	story, err := h.storyService.Create(r.Context(), req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, story)
}

// Get handles GET /api/pm/stories/{id}.
func (h *PMStoryHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	story, err := h.storyService.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, story)
}

// GetByDisplayID handles GET /api/pm/stories/display/{displayID}.
func (h *PMStoryHandler) GetByDisplayID(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	rawDisplayID := chi.URLParam(r, "displayID")
	displayID, err := strconv.Atoi(rawDisplayID)
	if err != nil || displayID <= 0 {
		writeError(w, http.StatusBadRequest, "displayID must be a positive integer")
		return
	}
	story, err := h.storyService.GetByDisplayID(r.Context(), workspaceID, displayID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, story)
}

// Update handles PUT /api/pm/stories/{id}.
func (h *PMStoryHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	var req model.UpdateStoryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	story, err := h.storyService.Update(r.Context(), id, req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, story)
}

// Delete handles DELETE /api/pm/stories/{id}.
func (h *PMStoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.storyService.Delete(r.Context(), id, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "story archived"})
}

// Move handles PUT /api/pm/stories/{id}/move.
func (h *PMStoryHandler) Move(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	var req model.MoveStoryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	story, err := h.storyService.MoveToState(r.Context(), id, req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, story)
}

// Reorder handles PUT /api/pm/stories/{id}/reorder.
func (h *PMStoryHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	var req model.ReorderStoryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.storyService.Reorder(r.Context(), id, req, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "story reordered"})
}

// AddOwner handles POST /api/pm/stories/{id}/owners.
func (h *PMStoryHandler) AddOwner(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	var req model.StoryUserLinkRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.storyService.AddOwner(r.Context(), id, req.UserID, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, model.MessageResponse{Message: "owner added"})
}

// RemoveOwner handles DELETE /api/pm/stories/{id}/owners/{userId}.
func (h *PMStoryHandler) RemoveOwner(w http.ResponseWriter, r *http.Request) {
	actorID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	userID := chi.URLParam(r, "userId")
	if err := h.storyService.RemoveOwner(r.Context(), id, userID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "owner removed"})
}

// AddFollower handles POST /api/pm/stories/{id}/followers.
func (h *PMStoryHandler) AddFollower(w http.ResponseWriter, r *http.Request) {
	actorID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	var req model.StoryUserLinkRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.storyService.AddFollower(r.Context(), id, req.UserID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, model.MessageResponse{Message: "follower added"})
}

// RemoveFollower handles DELETE /api/pm/stories/{id}/followers.
func (h *PMStoryHandler) RemoveFollower(w http.ResponseWriter, r *http.Request) {
	actorID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = actorID
	}
	if err := h.storyService.RemoveFollower(r.Context(), id, userID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "follower removed"})
}

// AddLabel handles POST /api/pm/stories/{id}/labels.
func (h *PMStoryHandler) AddLabel(w http.ResponseWriter, r *http.Request) {
	actorID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	var req model.StoryLabelLinkRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.storyService.AddLabel(r.Context(), id, req.LabelID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, model.MessageResponse{Message: "label added"})
}

// RemoveLabel handles DELETE /api/pm/stories/{id}/labels/{labelId}.
func (h *PMStoryHandler) RemoveLabel(w http.ResponseWriter, r *http.Request) {
	actorID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	labelID := chi.URLParam(r, "labelId")
	if err := h.storyService.RemoveLabel(r.Context(), id, labelID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "label removed"})
}

// ListActivity handles GET /api/pm/stories/{id}/activity.
func (h *PMStoryHandler) ListActivity(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	pagination := queryPagination(r)
	entries, total, err := h.storyService.ListActivity(r.Context(), id, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entries == nil {
		entries = []model.ActivityLogEntry{}
	}
	totalPages := 0
	if pagination.PerPage > 0 {
		totalPages = int((total + int64(pagination.PerPage) - 1) / int64(pagination.PerPage))
	}
	writeJSON(w, http.StatusOK, model.PaginatedResponse{
		Data:       entries,
		Total:      int(total),
		Page:       pagination.Page,
		PerPage:    pagination.PerPage,
		TotalPages: totalPages,
	})
}
