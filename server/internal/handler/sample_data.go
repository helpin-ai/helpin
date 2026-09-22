package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// SampleDataHandler serves the workspace sample data endpoints.
type SampleDataHandler struct {
	service *service.SampleDataService
	modules func(r *http.Request) (map[model.ModuleID]bool, error)
}

// NewSampleDataHandler creates a SampleDataHandler. Sample data is only seeded
// for modules the caller can access on this deployment.
func NewSampleDataHandler(sampleData *service.SampleDataService, authz *authorization.AuthzService) *SampleDataHandler {
	return &SampleDataHandler{
		service: sampleData,
		modules: func(r *http.Request) (map[model.ModuleID]bool, error) {
			modules, err := authz.AccessibleModules(r.Context(), authorization.GetActor(r.Context()))
			if err != nil {
				return nil, err
			}
			enabled := make(map[model.ModuleID]bool, len(modules))
			for _, module := range modules {
				enabled[module] = true
			}
			return enabled, nil
		},
	}
}

// Get returns whether sample data is loaded and how many records it holds.
func (h *SampleDataHandler) Get(w http.ResponseWriter, r *http.Request) {
	enabled, ok := h.enabledModules(w, r)
	if !ok {
		return
	}
	status, err := h.service.Status(r.Context(), chi.URLParam(r, "id"), enabled)
	if err != nil {
		slog.ErrorContext(r.Context(), "get sample data status", "error", err, "workspace_id", chi.URLParam(r, "id"))
		writeError(w, http.StatusInternalServerError, "failed to read sample data")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// Load seeds sample data. It responds 409 when sample data is already loaded.
func (h *SampleDataHandler) Load(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	enabled, ok := h.enabledModules(w, r)
	if !ok {
		return
	}
	status, err := h.service.Load(r.Context(), chi.URLParam(r, "id"), userID, enabled)
	var forbidden *model.ErrForbidden
	switch {
	case err == nil:
		writeJSON(w, http.StatusCreated, status)
	case errors.Is(err, model.ErrSampleDataAlreadyLoaded):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, model.ErrSampleDataNoModules):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.As(err, &forbidden):
		writeError(w, http.StatusForbidden, forbidden.Message)
	default:
		writeError(w, http.StatusInternalServerError, "failed to load sample data; nothing was changed")
	}
}

// Remove deletes the tracked sample records.
func (h *SampleDataHandler) Remove(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	enabled, ok := h.enabledModules(w, r)
	if !ok {
		return
	}
	status, err := h.service.Remove(r.Context(), chi.URLParam(r, "id"), userID, enabled)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove sample data; nothing was changed")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *SampleDataHandler) enabledModules(w http.ResponseWriter, r *http.Request) (map[model.ModuleID]bool, bool) {
	enabled, err := h.modules(r)
	if err != nil {
		slog.ErrorContext(r.Context(), "resolve sample data modules", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to resolve workspace modules")
		return nil, false
	}
	return enabled, true
}
