package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/d4interactive/teampulse/server/internal/middleware"
	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/service"
)

// PMCommentHandler handles PM comment HTTP endpoints.
type PMCommentHandler struct {
	commentService *service.PMCommentService
}

// NewPMCommentHandler creates a new PMCommentHandler.
func NewPMCommentHandler(commentService *service.PMCommentService) *PMCommentHandler {
	return &PMCommentHandler{commentService: commentService}
}

// List handles GET /api/pm/comments?entity_type=...&entity_id=...
func (h *PMCommentHandler) List(w http.ResponseWriter, r *http.Request) {
	entityType := r.URL.Query().Get("entity_type")
	entityID := r.URL.Query().Get("entity_id")
	if entityType == "" || entityID == "" {
		writeError(w, http.StatusBadRequest, "entity_type and entity_id are required")
		return
	}
	comments, err := h.commentService.List(r.Context(), entityType, entityID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if comments == nil {
		comments = []model.CommentWithAuthor{}
	}
	writeJSON(w, http.StatusOK, comments)
}

// Create handles POST /api/pm/comments.
func (h *PMCommentHandler) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	userID := middleware.GetUserID(r.Context())
	var req model.CreateCommentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	comment, err := h.commentService.Create(r.Context(), req, userID, workspaceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, comment)
}

// Update handles PUT /api/pm/comments/{id}.
func (h *PMCommentHandler) Update(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())
	var req model.UpdateCommentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	comment, err := h.commentService.Update(r.Context(), id, req, userID, false, workspaceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, comment)
}

// Delete handles DELETE /api/pm/comments/{id}.
func (h *PMCommentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())
	if err := h.commentService.Delete(r.Context(), id, userID, false, workspaceID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "comment deleted"})
}
