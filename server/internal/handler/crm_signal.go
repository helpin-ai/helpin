package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// CRMSignalHandler handles CRM signal and health score HTTP endpoints.
type CRMSignalHandler struct {
	signalService *service.CRMSignalService
}

// NewCRMSignalHandler creates a new CRMSignalHandler.
func NewCRMSignalHandler(signalService *service.CRMSignalService) *CRMSignalHandler {
	return &CRMSignalHandler{signalService: signalService}
}

// ListSignals handles GET /api/crm/signals.
func (h *CRMSignalHandler) ListSignals(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	filters := model.CRMBuyerSignalListFilters{
		ContactID:  queryStringPtr(r, "contact_id"),
		DealID:     queryStringPtr(r, "deal_id"),
		SignalType: queryStringPtr(r, "signal_type"),
		SourceType: queryStringPtr(r, "source_type"),
	}
	pagination := queryPagination(r)

	signals, total, err := h.signalService.ListSignals(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if signals == nil {
		signals = []model.CRMBuyerSignal{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  signals,
		"total": total,
		"page":  pagination.Page,
	})
}

// CreateSignal handles POST /api/crm/signals.
func (h *CRMSignalHandler) CreateSignal(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCRMBuyerSignalRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	signal, err := h.signalService.CreateSignal(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, signal)
}

// DeleteSignal handles DELETE /api/crm/signals/{id}.
func (h *CRMSignalHandler) DeleteSignal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.signalService.DeleteSignal(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "signal deleted"})
}

// ListHealthScores handles GET /api/crm/health-scores.
func (h *CRMSignalHandler) ListHealthScores(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	pagination := queryPagination(r)

	scores, total, err := h.signalService.ListHealthScores(r.Context(), workspaceID, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if scores == nil {
		scores = []model.CRMDealHealthScore{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  scores,
		"total": total,
		"page":  pagination.Page,
	})
}

// GetDealHealthScore handles GET /api/crm/deals/{id}/health-score.
func (h *CRMSignalHandler) GetDealHealthScore(w http.ResponseWriter, r *http.Request) {
	dealID := chi.URLParam(r, "id")
	score, err := h.signalService.GetLatestHealthScore(r.Context(), dealID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, score)
}

// CreateHealthScore handles POST /api/crm/health-scores.
func (h *CRMSignalHandler) CreateHealthScore(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCRMDealHealthScoreRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	score, err := h.signalService.CreateHealthScore(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, score)
}

// ListByContact handles GET /api/crm/contacts/{id}/signals.
func (h *CRMSignalHandler) ListByContact(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	contactID := chi.URLParam(r, "id")
	pagination := queryPagination(r)

	filters := model.CRMBuyerSignalListFilters{
		ContactID: &contactID,
	}
	signals, total, err := h.signalService.ListSignals(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if signals == nil {
		signals = []model.CRMBuyerSignal{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  signals,
		"total": total,
		"page":  pagination.Page,
	})
}

// ListByDeal handles GET /api/crm/deals/{id}/signals.
func (h *CRMSignalHandler) ListByDeal(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	dealID := chi.URLParam(r, "id")
	pagination := queryPagination(r)

	filters := model.CRMBuyerSignalListFilters{
		DealID: &dealID,
	}
	signals, total, err := h.signalService.ListSignals(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if signals == nil {
		signals = []model.CRMBuyerSignal{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  signals,
		"total": total,
		"page":  pagination.Page,
	})
}
