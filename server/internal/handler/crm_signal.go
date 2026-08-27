package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/querybuilder"
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

// DismissSignal handles POST /api/crm/signals/{id}/dismiss.
func (h *CRMSignalHandler) DismissSignal(w http.ResponseWriter, r *http.Request) {
	actor := authorization.GetActor(r.Context())
	if actor == nil || actor.WorkspaceMemberID == "" {
		writeError(w, http.StatusForbidden, "workspace membership is required")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "a dismissal reason is required")
		return
	}
	if err := h.signalService.DismissSignal(
		r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), actor.WorkspaceMemberID, req.Reason,
	); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "signal dismissed"})
}

// ReviewSignal records detection-to-review time without changing visibility.
func (h *CRMSignalHandler) ReviewSignal(w http.ResponseWriter, r *http.Request) {
	h.recordFeedback(w, r, model.CRMSignalFeedbackReviewed)
}

// ActOnSignal records detection-to-action time.
func (h *CRMSignalHandler) ActOnSignal(w http.ResponseWriter, r *http.Request) {
	h.recordFeedback(w, r, model.CRMSignalFeedbackActed)
}

func (h *CRMSignalHandler) recordFeedback(w http.ResponseWriter, r *http.Request, action string) {
	actor := authorization.GetActor(r.Context())
	if actor == nil || actor.WorkspaceMemberID == "" {
		writeError(w, http.StatusForbidden, "workspace membership is required")
		return
	}
	if err := h.signalService.RecordSignalFeedback(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), actor.WorkspaceMemberID, action, ""); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "signal feedback recorded"})
}

func (h *CRMSignalHandler) PrecisionReport(w http.ResponseWriter, r *http.Request) {
	rows, err := h.signalService.SignalPrecisionReport(r.Context(), getWorkspaceID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": rows})
}

func (h *CRMSignalHandler) ListRuleConfigs(w http.ResponseWriter, r *http.Request) {
	rows, err := h.signalService.ListRuleConfigs(r.Context(), getWorkspaceID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": rows})
}

func (h *CRMSignalHandler) GetRoutingPolicy(w http.ResponseWriter, r *http.Request) {
	policy, err := h.signalService.GetRoutingPolicy(r.Context(), getWorkspaceID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, policy)
}

func (h *CRMSignalHandler) CreateRoutingPolicy(w http.ResponseWriter, r *http.Request) {
	actor := authorization.GetActor(r.Context())
	if actor == nil || actor.WorkspaceMemberID == "" {
		writeError(w, http.StatusForbidden, "workspace membership is required")
		return
	}
	var req model.CreateCRMSignalRoutingPolicyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	policy, err := h.signalService.CreateRoutingPolicy(r.Context(), getWorkspaceID(r), actor.WorkspaceMemberID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, policy)
}

func (h *CRMSignalHandler) ActivateRoutingPolicy(w http.ResponseWriter, r *http.Request) {
	version, err := strconv.Atoi(chi.URLParam(r, "version"))
	if err != nil || h.signalService.ActivateRoutingPolicyVersion(r.Context(), getWorkspaceID(r), version) != nil {
		writeError(w, http.StatusBadRequest, "routing policy version not found")
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "routing policy activated"})
}

func (h *CRMSignalHandler) ActivateRuleVersion(w http.ResponseWriter, r *http.Request) {
	version, err := strconv.Atoi(chi.URLParam(r, "version"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid version")
		return
	}
	if err := h.signalService.ActivateRuleVersion(r.Context(), getWorkspaceID(r), chi.URLParam(r, "ruleKey"), version); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "rule version activated"})
}

func (h *CRMSignalHandler) SignalBrief(w http.ResponseWriter, r *http.Request) {
	filters, err := signalListFiltersFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid filters query")
		return
	}
	brief, err := h.signalService.GetSignalBrief(r.Context(), getWorkspaceID(r), filters)
	if err != nil {
		var validationErr *querybuilder.ValidationError
		if errors.As(err, &validationErr) {
			writeError(w, http.StatusBadRequest, validationErr.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, brief)
}

func (h *CRMSignalHandler) MeetingSignalBrief(w http.ResponseWriter, r *http.Request) {
	brief, err := h.signalService.GetMeetingSignalBrief(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, brief)
}

// ListSignals handles GET /api/crm/signals.
func (h *CRMSignalHandler) ListSignals(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	filters, err := signalListFiltersFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid filters query")
		return
	}
	pagination := queryPagination(r)

	signals, total, err := h.signalService.ListSignals(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		var validationErr *querybuilder.ValidationError
		if errors.As(err, &validationErr) {
			writeError(w, http.StatusBadRequest, validationErr.Error())
			return
		}
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

// ListWorkspaceFeed handles GET /api/crm/signals/feed.
func (h *CRMSignalHandler) ListWorkspaceFeed(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	filters, err := signalListFiltersFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid filters query")
		return
	}
	feed, err := h.signalService.ListWorkspaceSignalFeed(r.Context(), workspaceID, filters, queryPagination(r))
	if err != nil {
		var validationErr *querybuilder.ValidationError
		if errors.As(err, &validationErr) {
			writeError(w, http.StatusBadRequest, validationErr.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, feed)
}

func signalListFiltersFromRequest(r *http.Request) (model.CRMBuyerSignalListFilters, error) {
	queryFilters, err := queryFilterGroup(r, "filters")
	if err != nil {
		return model.CRMBuyerSignalListFilters{}, err
	}
	filters := model.CRMBuyerSignalListFilters{
		ContactID: queryStringPtr(r, "contact_id"), DealID: queryStringPtr(r, "deal_id"),
		CompanyID: queryStringPtr(r, "account_id"), SignalType: queryStringPtr(r, "signal_type"),
		SourceType: queryStringPtr(r, "source_type"), OwnerMemberID: queryStringPtr(r, "owner_member_id"),
		SignalDomain: queryStringPtr(r, "domain"), Polarity: queryStringPtr(r, "polarity"),
		EvidenceIdentityTrust: queryStringPtr(r, "trust"), Status: queryStringPtr(r, "status"),
		Severity: queryStringPtr(r, "severity"),
		Query:    queryFilters,
	}
	if filters.CompanyID == nil {
		filters.CompanyID = queryStringPtr(r, "company_id")
	}
	if value := r.URL.Query().Get("max_age_days"); value != "" {
		if days, err := strconv.Atoi(value); err == nil && days > 0 && days <= 366 {
			filters.MaxAgeDays = &days
		}
	}
	return filters, nil
}

// CreateSignal handles POST /api/crm/signals.
func (h *CRMSignalHandler) CreateSignal(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCRMBuyerSignalRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	workspaceID := getWorkspaceID(r)
	if req.WorkspaceID != "" && req.WorkspaceID != workspaceID {
		writeError(w, http.StatusBadRequest, "workspace does not match request scope")
		return
	}
	req.WorkspaceID = workspaceID
	signal, err := h.signalService.CreateSignal(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, signal)
}

// IngestExternalEvidence accepts normalized evidence from configured providers.
func (h *CRMSignalHandler) IngestExternalEvidence(w http.ResponseWriter, r *http.Request) {
	var req model.IngestCRMSignalExternalEvidenceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	workspaceID := getWorkspaceID(r)
	if req.WorkspaceID != "" && req.WorkspaceID != workspaceID {
		writeError(w, http.StatusBadRequest, "workspace does not match request scope")
		return
	}
	req.WorkspaceID = workspaceID
	evidence, created, err := h.signalService.IngestExternalEvidence(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, evidence)
}

// DeleteSignal handles DELETE /api/crm/signals/{id}.
func (h *CRMSignalHandler) DeleteSignal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.signalService.DeleteSignal(r.Context(), getWorkspaceID(r), id); err != nil {
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
	score, err := h.signalService.GetLatestHealthScore(r.Context(), getWorkspaceID(r), dealID)
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

// ListByCompany handles GET /api/crm/companies/{id}/signals.
func (h *CRMSignalHandler) ListByCompany(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	companyID := chi.URLParam(r, "id")
	pagination := queryPagination(r)
	signals, total, err := h.signalService.ListCompanySignals(r.Context(), workspaceID, companyID, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "buyer signals could not be loaded")
		return
	}
	if signals == nil {
		signals = []model.CRMBuyerSignal{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": signals, "total": total, "page": pagination.Page})
}
