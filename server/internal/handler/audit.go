package handler

import (
	"net/http"

	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/service"
)

// AuditHandler handles bonus audit log HTTP requests.
type AuditHandler struct {
	auditService *service.AuditService
}

// NewAuditHandler creates a new AuditHandler.
func NewAuditHandler(auditService *service.AuditService) *AuditHandler {
	return &AuditHandler{auditService: auditService}
}

// List handles GET /api/audit?workspace_id=xxx&quarter_id=xxx.
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	quarterID := r.URL.Query().Get("quarter_id")
	if workspaceID == "" || quarterID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id and quarter_id are required")
		return
	}

	entries, err := h.auditService.List(r.Context(), workspaceID, quarterID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entries == nil {
		entries = []model.BonusAuditLog{}
	}

	writeJSON(w, http.StatusOK, entries)
}
