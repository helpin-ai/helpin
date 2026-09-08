package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/service"
)

// AutomationPreview returns product-owned job selection without reading or editing saved Agents/Flows.
func (h *CRMPlaybookHandler) AutomationPreview(w http.ResponseWriter, r *http.Request) {
	revision, err := strconv.ParseInt(r.URL.Query().Get("revision"), 10, 64)
	if err != nil || revision < 1 {
		writePlaybookError(w, r, service.ErrCRMPlaybookInput)
		return
	}
	result, err := h.service.AutomationPreview(r.Context(), getWorkspaceID(r),
		chi.URLParam(r, "id"), r.URL.Query().Get("version_id"), revision)
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
