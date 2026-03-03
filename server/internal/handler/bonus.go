package handler

import (
	"net/http"

	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/service"
)

// BonusHandler handles bonus calculation HTTP requests.
type BonusHandler struct {
	bonusService *service.BonusService
}

// NewBonusHandler creates a new BonusHandler.
func NewBonusHandler(bonusService *service.BonusService) *BonusHandler {
	return &BonusHandler{bonusService: bonusService}
}

// GetCalculations handles GET /api/bonus/calculations?workspace_id=xxx&quarter_id=xxx.
func (h *BonusHandler) GetCalculations(w http.ResponseWriter, r *http.Request) {
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
		calcs = []model.BonusCalculation{}
	}

	writeJSON(w, http.StatusOK, calcs)
}

// SaveCalculations handles POST /api/bonus/calculations.
func (h *BonusHandler) SaveCalculations(w http.ResponseWriter, r *http.Request) {
	var req model.SaveCalculationsRequest
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

// Lock handles POST /api/bonus/lock.
func (h *BonusHandler) Lock(w http.ResponseWriter, r *http.Request) {
	var req model.LockBonusRequest
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

// Unlock handles POST /api/bonus/unlock.
func (h *BonusHandler) Unlock(w http.ResponseWriter, r *http.Request) {
	var req model.LockBonusRequest
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

// GetTeamSprintData handles GET /api/bonus/team-sprint-data?sprint_id=xxx.
func (h *BonusHandler) GetTeamSprintData(w http.ResponseWriter, r *http.Request) {
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
		data = []model.IndividualCheck{}
	}

	writeJSON(w, http.StatusOK, data)
}
