package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Inbox lists canonical work and unlinked recommendations without running a backfill.
func (h *CRMSituationHandler) Inbox(w http.ResponseWriter, r *http.Request) {
	filters := model.CRMSignalInboxFilters{Sort: r.URL.Query().Get("sort"), Navigation: model.CRMSituationListFilters{Scope: r.URL.Query().Get("scope"), State: r.URL.Query().Get("state"), Category: r.URL.Query().Get("category"), Search: r.URL.Query().Get("q")}}
	var err error
	filters.Navigation.Query, err = queryFilterGroup(r, "filter")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Check the filters and try again.")
		return
	}
	for _, field := range []struct {
		key    string
		target *int
	}{{key: "page", target: &filters.Navigation.Page}, {key: "page_size", target: &filters.Navigation.PageSize}} {
		if value := r.URL.Query().Get(field.key); value != "" {
			*field.target, err = strconv.Atoi(value)
			if err != nil || *field.target < 1 {
				writeError(w, http.StatusBadRequest, "Invalid page number or page size.")
				return
			}
		}
	}
	result, err := h.service.ListInbox(r.Context(), getWorkspaceID(r), filters)
	if err != nil {
		writeSituationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// InboxRecommendation preserves direct links to canonical recommendations.
func (h *CRMSituationHandler) InboxRecommendation(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.InboxRecommendation(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"))
	if err != nil {
		writeSituationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// DecideInboxRecommendation requires the displayed revision even on legacy recommendations.
func (h *CRMSituationHandler) DecideInboxRecommendation(w http.ResponseWriter, r *http.Request) {
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
		writeError(w, http.StatusBadRequest, "Check the recommendation decision.")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, http.StatusBadRequest, "Check the recommendation decision.")
		return
	}
	result, err := h.service.DecideInboxRecommendation(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), req.Revision, chi.URLParam(r, "decision"), req.Reason, req.Edits)
	if err != nil {
		writeSituationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
