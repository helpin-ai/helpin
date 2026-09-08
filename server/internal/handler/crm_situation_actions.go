package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// DecideAction adapts a situation decision to its canonical CRM suggestion.
func (h *CRMSituationHandler) DecideAction(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	defer r.Body.Close()
	var req struct {
		Revision string                 `json:"revision"`
		Reason   string                 `json:"reason"`
		Edits    map[string]interface{} `json:"edits"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid customer action request")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid customer action request")
		return
	}
	result, err := h.service.DecideAction(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"),
		chi.URLParam(r, "action_id"), req.Revision, chi.URLParam(r, "decision"), req.Reason, req.Edits)
	if err != nil {
		writeSituationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
