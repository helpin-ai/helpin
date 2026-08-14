package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// CRMEnrichmentHandler handles CRM enrichment HTTP endpoints.
type CRMEnrichmentHandler struct {
	enrichmentService *service.CRMEnrichmentService
}

// ApplySuggestion handles POST /api/crm/enrichments/{id}/apply-suggestion.
func (h *CRMEnrichmentHandler) ApplySuggestion(w http.ResponseWriter, r *http.Request) {
	var req model.ApplyCRMEnrichmentSuggestionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.ActorUserID = middleware.GetUserID(r.Context())
	result, err := h.enrichmentService.ApplySuggestion(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// NewCRMEnrichmentHandler creates a new CRMEnrichmentHandler.
func NewCRMEnrichmentHandler(enrichmentService *service.CRMEnrichmentService) *CRMEnrichmentHandler {
	return &CRMEnrichmentHandler{enrichmentService: enrichmentService}
}

// List handles GET /api/crm/enrichments.
func (h *CRMEnrichmentHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	filters := model.CRMEnrichmentListFilters{
		ObjectType: queryStringPtr(r, "object_type"),
		ObjectID:   queryStringPtr(r, "object_id"),
		Source:     queryStringPtr(r, "source"),
	}
	pagination := queryPagination(r)

	results, total, err := h.enrichmentService.List(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if results == nil {
		results = []model.CRMEnrichmentResult{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  results,
		"total": total,
		"page":  pagination.Page,
	})
}

// Create handles POST /api/crm/enrichments.
func (h *CRMEnrichmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCRMEnrichmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	result, err := h.enrichmentService.Create(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, result)
}
