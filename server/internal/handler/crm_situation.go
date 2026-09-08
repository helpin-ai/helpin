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

// CRMSituationHandler exposes additive customer-work APIs without changing legacy routes.
type CRMSituationHandler struct{ service *service.CRMSituationService }

// NewCRMSituationHandler creates the customer-work API boundary.
func NewCRMSituationHandler(svc *service.CRMSituationService) *CRMSituationHandler {
	return &CRMSituationHandler{service: svc}
}

// List handles GET /api/crm/situations, including server-side category facets.
func (h *CRMSituationHandler) List(w http.ResponseWriter, r *http.Request) {
	filters := model.CRMSituationListFilters{
		Scope: r.URL.Query().Get("scope"), State: r.URL.Query().Get("state"),
		Category: r.URL.Query().Get("category"), Search: r.URL.Query().Get("q"),
	}
	var err error
	if filters.Query, err = queryFilterGroup(r, "filter"); err != nil {
		writeError(w, http.StatusBadRequest, "Check the filters and try again.")
		return
	}
	for _, param := range []struct {
		name string
		dest *int
	}{{name: "page", dest: &filters.Page}, {name: "page_size", dest: &filters.PageSize}} {
		if raw := r.URL.Query().Get(param.name); raw != "" {
			*param.dest, err = strconv.Atoi(raw)
			if err != nil || *param.dest < 1 {
				writeError(w, http.StatusBadRequest, "Invalid page number or page size.")
				return
			}
		}
	}
	result, err := h.service.List(r.Context(), getWorkspaceID(r), filters)
	if err != nil {
		writeSituationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Get handles GET /api/crm/situations/{id} without mutating evidence or actions.
func (h *CRMSituationHandler) Get(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.GetByID(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"))
	if err != nil {
		writeSituationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// Create handles POST /api/crm/situations. It never starts a playbook or executor.
func (h *CRMSituationHandler) Create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	defer r.Body.Close()
	var req model.CreateCRMSituationRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Check the details and try again.")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, http.StatusBadRequest, "Check the details and try again.")
		return
	}
	item, created, err := h.service.Create(r.Context(), getWorkspaceID(r), req)
	if err != nil {
		writeSituationError(w, r, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, item)
}

func writeSituationError(w http.ResponseWriter, r *http.Request, err error) {
	var entitlementError *service.EntitlementError
	if errors.As(err, &entitlementError) {
		writeBillingAwareError(w, http.StatusPaymentRequired, err)
		return
	}
	var filterError *querybuilder.ValidationError
	switch {
	case errors.Is(err, repository.ErrCRMPlaybookExecutionBlocked), errors.Is(err, repository.ErrCRMPlaybookConflict):
		writeError(w, http.StatusConflict, "This action changed or is no longer authorized. Refresh it before continuing.")
	case errors.Is(err, service.ErrCRMPlaybookForbidden):
		writeError(w, http.StatusForbidden, "Only the responsible reviewer with current access can approve this action.")
	case errors.Is(err, service.ErrCRMPlaybookInput):
		writeError(w, http.StatusBadRequest, "Check the proposed action, its recipient and destination.")
	case errors.Is(err, repository.ErrCRMSituationStale):
		writeError(w, http.StatusConflict, "This item has changed. Reload it before saving.")
	case errors.Is(err, repository.ErrCRMSituationCommandConflict):
		writeError(w, http.StatusConflict, "This request was already used for a different change. Reload and try again.")
	case errors.Is(err, repository.ErrCRMSituationExecutionPending):
		writeError(w, http.StatusConflict, "An action is still running or its result is unknown. Check its status before closing this item.")
	case errors.Is(err, service.ErrCRMSituationTransition):
		writeError(w, http.StatusConflict, "This change is no longer available. Reload the item to see its current status.")
	case errors.Is(err, service.ErrCRMSuggestionStale):
		writeError(w, http.StatusConflict, "This action has changed. Reload it before deciding.")
	case errors.Is(err, service.ErrCRMSituationForbidden):
		writeError(w, http.StatusForbidden, "You don't have access to this item.")
	case errors.Is(err, service.ErrCRMSituationNotFound):
		writeError(w, http.StatusNotFound, "This item could not be found.")
	case errors.Is(err, service.ErrCRMSituationInput), errors.As(err, &filterError):
		writeError(w, http.StatusBadRequest, "Check the details and filters, then try again.")
	case errors.Is(err, repository.ErrCRMSituationInvalidReference):
		writeError(w, http.StatusBadRequest, "A record or owner is unavailable in this workspace.")
	case errors.Is(err, repository.ErrCRMSituationConflict):
		writeError(w, http.StatusConflict, "This request was already used to create another item. Reload and try again.")
	default:
		slog.ErrorContext(r.Context(), "customer work request failed", "error", err, "workspace_id", getWorkspaceID(r))
		writeError(w, http.StatusInternalServerError, "This item could not be loaded or saved. Try again.")
	}
}
