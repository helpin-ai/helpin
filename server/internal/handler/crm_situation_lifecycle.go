package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Command handles POST /api/crm/situations/{id}/commands with explicit concurrency controls.
func (h *CRMSituationHandler) Command(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	defer r.Body.Close()
	var req model.CRMSituationCommandRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Check the change and try again.")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, http.StatusBadRequest, "Check the change and try again.")
		return
	}
	result, err := h.service.Command(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), req)
	if err != nil {
		writeSituationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// History handles bounded, keyset-paginated GET /api/crm/situations/{id}/history.
func (h *CRMSituationHandler) History(w http.ResponseWriter, r *http.Request) {
	var before int64
	limit := 50
	var err error
	if raw := r.URL.Query().Get("before_revision"); raw != "" {
		before, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || before < 1 {
			writeError(w, http.StatusBadRequest, "This history link is invalid. Reload the item and try again.")
			return
		}
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			writeError(w, http.StatusBadRequest, "Choose between 1 and 100 history entries.")
			return
		}
	}
	result, err := h.service.History(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), before, limit)
	if err != nil {
		writeSituationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
