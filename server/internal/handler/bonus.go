package handler

import (
	"net/http"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// RewardBonusHandler handles bonus calculation HTTP requests.
type RewardBonusHandler struct {
	bonusService *service.RewardBonusService
}

// NewRewardBonusHandler creates a new RewardBonusHandler.
func NewRewardBonusHandler(bonusService *service.RewardBonusService) *RewardBonusHandler {
	return &RewardBonusHandler{bonusService: bonusService}
}

// GetCalculations handles GET /api/rewards/bonus/calculations?workspace_id=xxx&quarter_id=xxx.
func (h *RewardBonusHandler) GetCalculations(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	quarterID := r.URL.Query().Get("quarter_id")
	if workspaceID == "" || quarterID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id and quarter_id are required")
		return
	}

	calcs, err := h.bonusService.GetCalculations(r.Context(), workspaceID, quarterID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if calcs == nil {
		calcs = []model.RewardBonusCalculation{}
	}

	writeJSON(w, http.StatusOK, calcs)
}

// SaveCalculations handles POST /api/rewards/bonus/calculations.
func (h *RewardBonusHandler) SaveCalculations(w http.ResponseWriter, r *http.Request) {
	var req model.SaveRewardCalculationsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.bonusService.SaveCalculations(r.Context(), req.Calculations); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "calculations saved"})
}

// Lock handles POST /api/rewards/bonus/lock.
func (h *RewardBonusHandler) Lock(w http.ResponseWriter, r *http.Request) {
	var req model.LockRewardBonusRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.bonusService.Lock(r.Context(), req.WorkspaceID, req.QuarterID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "bonus calculations locked"})
}

// Unlock handles POST /api/rewards/bonus/unlock.
func (h *RewardBonusHandler) Unlock(w http.ResponseWriter, r *http.Request) {
	var req model.LockRewardBonusRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.bonusService.Unlock(r.Context(), req.WorkspaceID, req.QuarterID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "bonus calculations unlocked"})
}

// GetTeamSprintData handles GET /api/rewards/bonus/team-sprint-data?sprint_id=xxx.
func (h *RewardBonusHandler) GetTeamSprintData(w http.ResponseWriter, r *http.Request) {
	sprintID := r.URL.Query().Get("sprint_id")
	if sprintID == "" {
		writeError(w, http.StatusBadRequest, "sprint_id is required")
		return
	}

	data, err := h.bonusService.GetTeamSprintData(r.Context(), sprintID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if data == nil {
		data = []model.RewardIndividualCheck{}
	}

	writeJSON(w, http.StatusOK, data)
}
