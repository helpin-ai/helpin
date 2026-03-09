package handler

import (
	"net/http"

	"github.com/helpin-ai/helpin/server/internal/service"
)

// CRMSearchHandler handles CRM search HTTP endpoints.
type CRMSearchHandler struct {
	searchService *service.CRMSearchService
}

// NewCRMSearchHandler creates a new CRMSearchHandler.
func NewCRMSearchHandler(searchService *service.CRMSearchService) *CRMSearchHandler {
	return &CRMSearchHandler{searchService: searchService}
}

// Search handles GET /api/crm/search?q=term.
func (h *CRMSearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, http.StatusOK, []service.CRMSearchResult{})
		return
	}

	results, err := h.searchService.Search(r.Context(), workspaceID, q)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if results == nil {
		results = []service.CRMSearchResult{}
	}
	writeJSON(w, http.StatusOK, results)
}
