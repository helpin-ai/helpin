package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// CRMSequenceHandler handles CRM sequence HTTP endpoints.
type CRMSequenceHandler struct {
	sequenceService *service.CRMSequenceService
}

// NewCRMSequenceHandler creates a new CRMSequenceHandler.
func NewCRMSequenceHandler(sequenceService *service.CRMSequenceService) *CRMSequenceHandler {
	return &CRMSequenceHandler{sequenceService: sequenceService}
}

// List handles GET /api/crm/sequences.
func (h *CRMSequenceHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	filters := model.CRMSequenceListFilters{
		Status: queryStringPtr(r, "status"),
		Search: queryStringPtr(r, "search"),
	}
	pagination := queryPagination(r)

	sequences, total, err := h.sequenceService.List(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if sequences == nil {
		sequences = []model.CRMSequence{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  sequences,
		"total": total,
		"page":  pagination.Page,
	})
}

// Get handles GET /api/crm/sequences/{id}.
func (h *CRMSequenceHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	seq, err := h.sequenceService.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, seq)
}

// Create handles POST /api/crm/sequences.
func (h *CRMSequenceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCRMSequenceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	seq, err := h.sequenceService.Create(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, seq)
}

// Update handles PUT /api/crm/sequences/{id}.
func (h *CRMSequenceHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req model.UpdateCRMSequenceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	seq, err := h.sequenceService.Update(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, seq)
}

// Delete handles DELETE /api/crm/sequences/{id}.
func (h *CRMSequenceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.sequenceService.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "sequence deleted"})
}

// ListEnrollments handles GET /api/crm/sequences/{id}/enrollments.
func (h *CRMSequenceHandler) ListEnrollments(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	seqID := chi.URLParam(r, "id")
	pagination := queryPagination(r)

	filters := model.CRMSequenceEnrollmentListFilters{
		SequenceID: &seqID,
		Status:     queryStringPtr(r, "status"),
	}
	enrollments, total, err := h.sequenceService.ListEnrollments(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if enrollments == nil {
		enrollments = []model.CRMSequenceEnrollment{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  enrollments,
		"total": total,
		"page":  pagination.Page,
	})
}

// CreateEnrollment handles POST /api/crm/sequences/{id}/enrollments.
func (h *CRMSequenceHandler) CreateEnrollment(w http.ResponseWriter, r *http.Request) {
	seqID := chi.URLParam(r, "id")
	var req model.CreateCRMSequenceEnrollmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	req.SequenceID = seqID

	enrollment, err := h.sequenceService.CreateEnrollment(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, enrollment)
}

// UpdateEnrollment handles PUT /api/crm/enrollments/{id}.
func (h *CRMSequenceHandler) UpdateEnrollment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req model.UpdateCRMSequenceEnrollmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	enrollment, err := h.sequenceService.UpdateEnrollment(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, enrollment)
}

// DeleteEnrollment handles DELETE /api/crm/enrollments/{id}.
func (h *CRMSequenceHandler) DeleteEnrollment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.sequenceService.DeleteEnrollment(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "enrollment deleted"})
}
