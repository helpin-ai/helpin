package handler

import (
	"net/http"

	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/service"
)

// FinanceHandler handles quarterly finance settings HTTP requests.
type FinanceHandler struct {
	bonusService *service.BonusService
}

// NewFinanceHandler creates a new FinanceHandler.
func NewFinanceHandler(bonusService *service.BonusService) *FinanceHandler {
	return &FinanceHandler{bonusService: bonusService}
}

// Get handles GET /api/finance?workspace_id=xxx&quarter_id=xxx.
func (h *FinanceHandler) Get(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	quarterID := r.URL.Query().Get("quarter_id")
	if workspaceID == "" || quarterID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id and quarter_id are required")
		return
	}

	fs, err := h.bonusService.GetFinance(r.Context(), workspaceID, quarterID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if fs == nil {
		writeJSON(w, http.StatusOK, nil)
		return
	}

	writeJSON(w, http.StatusOK, fs)
}

// Upsert handles POST /api/finance.
func (h *FinanceHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	var req model.UpsertFinanceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	fs, err := h.bonusService.UpsertFinance(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, fs)
}
