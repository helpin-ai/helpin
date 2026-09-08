package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/querybuilder"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// CRMPlaybookHandler exposes CRM policy and explicit participation without execution controls.
type CRMPlaybookHandler struct {
	service   *service.CRMPlaybookService
	execution *service.CRMPlaybookExecutionService
	setup     *service.CRMPlaybookSetupService
	actions   *service.CRMPlaybookActionService
}

// NewCRMPlaybookHandler binds the CRM policy API.
func NewCRMPlaybookHandler(svc *service.CRMPlaybookService) *CRMPlaybookHandler {
	return &CRMPlaybookHandler{service: svc}
}

// SetExecution connects the guided setup and Signal-owned action controls.
func (h *CRMPlaybookHandler) SetExecution(execution *service.CRMPlaybookExecutionService, setup *service.CRMPlaybookSetupService, actions *service.CRMPlaybookActionService) *CRMPlaybookHandler {
	h.execution, h.setup, h.actions = execution, setup, actions
	return h
}

// Templates returns draft defaults that have not been installed or activated.
func (h *CRMPlaybookHandler) Templates(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Templates(r.Context(), getWorkspaceID(r))
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// List handles GET /api/crm/playbooks with complete participant totals.
func (h *CRMPlaybookHandler) List(w http.ResponseWriter, r *http.Request) {
	page, size, ok := playbookPagination(w, r)
	if !ok {
		return
	}
	result, err := h.service.List(r.Context(), getWorkspaceID(r), model.CRMPlaybookListFilters{Search: r.URL.Query().Get("q"), State: r.URL.Query().Get("state"), Page: page, PageSize: size})
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Get reads a Playbook and its current published definition.
func (h *CRMPlaybookHandler) Get(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Get(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"))
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Create saves a draft, with customer enrollment disabled.
func (h *CRMPlaybookHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCRMPlaybookRequest
	if !decodePlaybookRequest(w, r, &req) {
		return
	}
	result, created, err := h.service.Create(r.Context(), getWorkspaceID(r), req)
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, result)
}

// Command changes a draft, publishes a snapshot, or controls new enrollments only.
func (h *CRMPlaybookHandler) Command(w http.ResponseWriter, r *http.Request) {
	var req model.CRMPlaybookCommandRequest
	if !decodePlaybookRequest(w, r, &req) {
		return
	}
	result, err := h.service.Command(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), req)
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// History returns auditable, cursor-paginated definition changes.
func (h *CRMPlaybookHandler) History(w http.ResponseWriter, r *http.Request) {
	before, limit, ok := playbookCursor(w, r)
	if !ok {
		return
	}
	result, err := h.service.History(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), before, limit)
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Versions returns immutable published definitions, including superseded versions.
func (h *CRMPlaybookHandler) Versions(w http.ResponseWriter, r *http.Request) {
	before, limit, ok := playbookCursor(w, r)
	if !ok {
		return
	}
	result, err := h.service.Versions(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), before, limit)
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Preview evaluates the exact draft revision without performing writes or starting execution.
func (h *CRMPlaybookHandler) Preview(w http.ResponseWriter, r *http.Request) {
	page, size, ok := playbookPagination(w, r)
	if !ok {
		return
	}
	revision, err := strconv.ParseInt(r.URL.Query().Get("revision"), 10, 64)
	if err != nil || revision < 1 {
		writePlaybookError(w, r, service.ErrCRMPlaybookInput)
		return
	}
	result, err := h.service.Preview(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), r.URL.Query().Get("version_id"), revision, page, size)
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Participants reads the canonical Signal list within this Playbook.
func (h *CRMPlaybookHandler) Participants(w http.ResponseWriter, r *http.Request) {
	page, size, ok := playbookPagination(w, r)
	if !ok {
		return
	}
	query, err := queryFilterGroup(r, "filter")
	if err != nil {
		writePlaybookError(w, r, service.ErrCRMPlaybookInput)
		return
	}
	result, err := h.service.Participants(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), model.CRMSituationListFilters{
		Scope: r.URL.Query().Get("scope"), State: r.URL.Query().Get("state"), Category: r.URL.Query().Get("category"),
		Search: r.URL.Query().Get("q"), Query: query, Page: page, PageSize: size})
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Apply explicitly attaches a published Playbook to an existing Signal.
func (h *CRMPlaybookHandler) Apply(w http.ResponseWriter, r *http.Request) {
	var req model.ApplyCRMPlaybookRequest
	if !decodePlaybookRequest(w, r, &req) {
		return
	}
	result, err := h.service.Apply(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), req)
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// AssessMilestone records an attributed human assessment without inferring a customer outcome.
func (h *CRMPlaybookHandler) AssessMilestone(w http.ResponseWriter, r *http.Request) {
	var req model.CRMPlaybookMilestoneRequest
	if !decodePlaybookRequest(w, r, &req) {
		return
	}
	result, err := h.service.AssessMilestone(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), chi.URLParam(r, "situation_id"), req)
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func decodePlaybookRequest(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writePlaybookError(w, r, service.ErrCRMPlaybookInput)
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writePlaybookError(w, r, service.ErrCRMPlaybookInput)
		return false
	}
	return true
}

func playbookPagination(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	values := []int{1, 25}
	for i, name := range []string{"page", "page_size"} {
		if raw := r.URL.Query().Get(name); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 {
				writePlaybookError(w, r, service.ErrCRMPlaybookInput)
				return 0, 0, false
			}
			values[i] = value
		}
	}
	return values[0], values[1], true
}

func playbookCursor(w http.ResponseWriter, r *http.Request) (int64, int, bool) {
	_, limit, ok := playbookPagination(w, r)
	if !ok {
		return 0, 0, false
	}
	var before int64
	if raw := r.URL.Query().Get("before"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 1 {
			writePlaybookError(w, r, service.ErrCRMPlaybookInput)
			return 0, 0, false
		}
		before = value
	}
	return before, limit, true
}

func writePlaybookError(w http.ResponseWriter, r *http.Request, err error) {
	var entitlementError *service.EntitlementError
	if errors.As(err, &entitlementError) {
		writeBillingAwareError(w, http.StatusPaymentRequired, err)
		return
	}
	var filterError *querybuilder.ValidationError
	switch {
	case errors.Is(err, service.ErrCRMPlaybookRuntimeUnavailable):
		writeError(w, http.StatusServiceUnavailable, "Automation is unavailable. Your setup is saved; nothing has been started.")
	case errors.Is(err, repository.ErrCRMPlaybookExecutionBlocked):
		writeError(w, http.StatusConflict, "This work changed or is paused. Refresh the Signal and review its owner and automation settings.")
	case errors.Is(err, service.ErrCRMPlaybookConnectionUnsupported):
		writeError(w, http.StatusConflict, "Choose a disabled Flow that uses Beacon with a CRM target. These settings cannot be published for this Playbook yet.")
	case errors.Is(err, service.ErrCRMPlaybookInput), errors.As(err, &filterError):
		writeError(w, http.StatusBadRequest, "Check the Playbook settings and try again.")
	case errors.Is(err, service.ErrCRMPlaybookForbidden):
		writeError(w, http.StatusForbidden, "You don't have permission to do this.")
	case errors.Is(err, service.ErrCRMPlaybookNotFound):
		writeError(w, http.StatusNotFound, "This Playbook could not be found.")
	case errors.Is(err, repository.ErrCRMPlaybookStale):
		writeError(w, http.StatusConflict, "This Playbook has changed. Reload it before continuing.")
	case errors.Is(err, repository.ErrCRMPlaybookConflict):
		writeError(w, http.StatusConflict, "This request conflicts with existing work. Reload before trying again.")
	case errors.Is(err, repository.ErrCRMPlaybookUnavailable):
		writeError(w, http.StatusConflict, "This Playbook cannot be applied or changed in its current state. Check its settings and the Signal's status.")
	default:
		// Preserve the established Signal/filter/reference error boundary.
		if errors.Is(err, service.ErrCRMSituationNotFound) || errors.Is(err, service.ErrCRMSituationInput) || errors.Is(err, service.ErrCRMSituationForbidden) || errors.Is(err, repository.ErrCRMSituationStale) || errors.Is(err, repository.ErrCRMSituationInvalidReference) {
			writeSituationError(w, r, err)
			return
		}
		slog.ErrorContext(r.Context(), "CRM Playbook request failed", "error", err, "workspace_id", getWorkspaceID(r))
		writeError(w, http.StatusInternalServerError, "The Playbook could not be loaded or saved. Try again.")
	}
}
