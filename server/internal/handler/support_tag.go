package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type SupportTagHandler struct {
	tagService *service.SupportTagService
}

func NewSupportTagHandler(tagService *service.SupportTagService) *SupportTagHandler {
	return &SupportTagHandler{tagService: tagService}
}

func (h *SupportTagHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	tags, err := h.tagService.List(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tags == nil {
		tags = []model.SupportTag{}
	}
	writeJSON(w, http.StatusOK, tags)
}

func (h *SupportTagHandler) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	var req model.CreateSupportTagRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	tag, err := h.tagService.Create(r.Context(), workspaceID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, tag)
}

func (h *SupportTagHandler) Update(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	tagID := chi.URLParam(r, "tagId")
	var req model.UpdateSupportTagRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	tag, err := h.tagService.Update(r.Context(), workspaceID, tagID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tag)
}

func (h *SupportTagHandler) Delete(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	tagID := chi.URLParam(r, "tagId")
	if err := h.tagService.Delete(r.Context(), workspaceID, tagID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "tag deleted"})
}

func (h *SupportTagHandler) AddConversationTag(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	conversationID := chi.URLParam(r, "id")
	tagID := chi.URLParam(r, "tagId")
	actorID := middleware.GetUserID(r.Context())
	if err := h.tagService.AddConversationTag(r.Context(), workspaceID, conversationID, tagID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "tag added"})
}

func (h *SupportTagHandler) RemoveConversationTag(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	conversationID := chi.URLParam(r, "id")
	tagID := chi.URLParam(r, "tagId")
	actorID := middleware.GetUserID(r.Context())
	if err := h.tagService.RemoveConversationTag(r.Context(), workspaceID, conversationID, tagID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "tag removed"})
}
