package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// CRMListHandler handles CRM list HTTP endpoints.
type CRMListHandler struct {
	listService *service.CRMListService
}

// NewCRMListHandler creates a new CRMListHandler.
func NewCRMListHandler(listService *service.CRMListService) *CRMListHandler {
	return &CRMListHandler{listService: listService}
}

// List handles GET /api/crm/lists.
func (h *CRMListHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	filters := model.CRMListFilters{
		ObjectType: queryStringPtr(r, "object_type"),
		ListType:   queryStringPtr(r, "list_type"),
		Search:     queryStringPtr(r, "search"),
	}
	pagination := queryPagination(r)

	lists, total, err := h.listService.List(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if lists == nil {
		lists = []model.CRMList{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  lists,
		"total": total,
		"page":  pagination.Page,
	})
}

// Get handles GET /api/crm/lists/{id}.
func (h *CRMListHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	list, err := h.listService.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// Create handles POST /api/crm/lists.
func (h *CRMListHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCRMListRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	list, err := h.listService.Create(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, list)
}

// Update handles PUT /api/crm/lists/{id}.
func (h *CRMListHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req model.UpdateCRMListRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	list, err := h.listService.Update(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// Delete handles DELETE /api/crm/lists/{id}.
func (h *CRMListHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.listService.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "list deleted"})
}

// ListMembers handles GET /api/crm/lists/{id}/members.
func (h *CRMListHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	pagination := queryPagination(r)

	members, total, err := h.listService.ListMembers(r.Context(), id, pagination)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if members == nil {
		members = []model.CRMListMember{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  members,
		"total": total,
		"page":  pagination.Page,
	})
}

// AddMember handles POST /api/crm/lists/{id}/members.
func (h *CRMListHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req model.AddCRMListMemberRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	member, err := h.listService.AddMember(r.Context(), id, req.ObjectID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, member)
}

// RemoveMember handles DELETE /api/crm/lists/{id}/members/{objectId}.
func (h *CRMListHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	objectID := chi.URLParam(r, "objectId")
	if err := h.listService.RemoveMember(r.Context(), id, objectID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "member removed"})
}
