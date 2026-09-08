package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func (h *CRMPlaybookHandler) AgentUsage(w http.ResponseWriter, r *http.Request) {
	if h.execution == nil {
		writePlaybookError(w, r, service.ErrCRMPlaybookRuntimeUnavailable)
		return
	}
	result, err := h.execution.AgentUsage(r.Context(), getWorkspaceID(r), chi.URLParam(r, "agentID"))
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *CRMPlaybookHandler) AutomationActivity(w http.ResponseWriter, r *http.Request) {
	if h.execution == nil {
		writePlaybookError(w, r, service.ErrCRMPlaybookRuntimeUnavailable)
		return
	}
	page := 1
	if raw := r.URL.Query().Get("page"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writePlaybookError(w, r, service.ErrCRMPlaybookInput)
			return
		}
		page = parsed
	}
	result, err := h.execution.AutomationActivity(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), page)
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *CRMPlaybookHandler) ExecutionOverview(w http.ResponseWriter, r *http.Request) {
	if h.execution == nil {
		writePlaybookError(w, r, service.ErrCRMPlaybookRuntimeUnavailable)
		return
	}
	result, err := h.execution.Overview(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"))
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *CRMPlaybookHandler) PrepareSetup(w http.ResponseWriter, r *http.Request) {
	if h.setup == nil {
		writePlaybookError(w, r, service.ErrCRMPlaybookRuntimeUnavailable)
		return
	}
	var req model.PrepareCRMPlaybookSetupRequest
	if !decodePlaybookRequest(w, r, &req) {
		return
	}
	result, err := h.setup.Prepare(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), req)
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *CRMPlaybookHandler) ConfigureExecution(w http.ResponseWriter, r *http.Request) {
	if h.execution == nil {
		writePlaybookError(w, r, service.ErrCRMPlaybookRuntimeUnavailable)
		return
	}
	var req model.CRMPlaybookAutomationCommand
	if !decodePlaybookRequest(w, r, &req) {
		return
	}
	result, err := h.execution.Configure(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), req)
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *CRMPlaybookHandler) SignalExecution(w http.ResponseWriter, r *http.Request) {
	if h.execution == nil {
		writePlaybookError(w, r, service.ErrCRMPlaybookRuntimeUnavailable)
		return
	}
	result, err := h.execution.Binding(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"))
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *CRMPlaybookHandler) AdoptExecution(w http.ResponseWriter, r *http.Request) {
	if h.execution == nil {
		writePlaybookError(w, r, service.ErrCRMPlaybookRuntimeUnavailable)
		return
	}
	var req model.CRMPlaybookAutomationAdoption
	if !decodePlaybookRequest(w, r, &req) {
		return
	}
	result, err := h.execution.Adopt(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), req)
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *CRMPlaybookHandler) ActionDetails(w http.ResponseWriter, r *http.Request) {
	if h.actions == nil {
		writePlaybookError(w, r, service.ErrCRMPlaybookRuntimeUnavailable)
		return
	}
	result, err := h.actions.Details(r.Context(), getWorkspaceID(r), chi.URLParam(r, "action_id"))
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *CRMPlaybookHandler) ReconcileAction(w http.ResponseWriter, r *http.Request) {
	if h.actions == nil {
		writePlaybookError(w, r, service.ErrCRMPlaybookRuntimeUnavailable)
		return
	}
	result, err := h.actions.Reconcile(r.Context(), getWorkspaceID(r), chi.URLParam(r, "action_id"))
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *CRMPlaybookHandler) InspectAction(w http.ResponseWriter, r *http.Request) {
	if h.actions == nil {
		writePlaybookError(w, r, service.ErrCRMPlaybookRuntimeUnavailable)
		return
	}
	var req model.CRMPlaybookActionInspection
	if !decodePlaybookRequest(w, r, &req) {
		return
	}
	result, err := h.actions.Inspect(r.Context(), getWorkspaceID(r), chi.URLParam(r, "action_id"), req)
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
