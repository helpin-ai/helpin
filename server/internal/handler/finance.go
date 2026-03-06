package handler

import (
	"net/http"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// RewardFinanceHandler handles quarterly finance settings HTTP requests.
type RewardFinanceHandler struct {
	bonusService *service.RewardBonusService
}

// NewRewardFinanceHandler creates a new RewardFinanceHandler.
func NewRewardFinanceHandler(bonusService *service.RewardBonusService) *RewardFinanceHandler {
	return &RewardFinanceHandler{bonusService: bonusService}
}

// Get handles GET /api/rewards/finance?workspace_id=xxx&quarter_id=xxx.
func (h *RewardFinanceHandler) Get(w http.ResponseWriter, r *http.Request) {
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

// Upsert handles POST /api/rewards/finance.
func (h *RewardFinanceHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	var req model.UpsertRewardFinanceRequest
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
