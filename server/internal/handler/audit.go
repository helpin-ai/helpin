package handler

import (
	"net/http"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// RewardAuditHandler handles reward audit log HTTP requests.
type RewardAuditHandler struct {
	auditService *service.RewardAuditService
}

// NewRewardAuditHandler creates a new RewardAuditHandler.
func NewRewardAuditHandler(auditService *service.RewardAuditService) *RewardAuditHandler {
	return &RewardAuditHandler{auditService: auditService}
}

// List handles GET /api/rewards/audit?workspace_id=xxx&quarter_id=xxx.
func (h *RewardAuditHandler) List(w http.ResponseWriter, r *http.Request) {
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
		entries = []model.RewardAuditLog{}
	}

	writeJSON(w, http.StatusOK, entries)
}
