package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/service"
)

// SupportTeamReplyHandler serves the public page behind the "Ask our team to
// reply" link in AI-written emails. The link token is the only credential.
type SupportTeamReplyHandler struct {
	svc *service.SupportTeamReplyService
}

// NewSupportTeamReplyHandler creates a SupportTeamReplyHandler.
func NewSupportTeamReplyHandler(svc *service.SupportTeamReplyService) *SupportTeamReplyHandler {
	return &SupportTeamReplyHandler{svc: svc}
}

// Page returns the link's conversation and state. It never changes anything:
// mail scanners open links before the customer does.
func (h *SupportTeamReplyHandler) Page(w http.ResponseWriter, r *http.Request) {
	page, err := h.svc.Page(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		writeSupportTeamReplyError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

// Request hands the conversation to the team after the customer confirms.
func (h *SupportTeamReplyHandler) Request(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Note string `json:"note"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	page, err := h.svc.Request(r.Context(), chi.URLParam(r, "token"), body.Note)
	if err != nil {
		writeSupportTeamReplyError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func writeSupportTeamReplyError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, service.ErrSupportTeamReplyLinkInvalid) {
		writeError(w, http.StatusNotFound, "This link is not valid.")
		return
	}
	slog.ErrorContext(r.Context(), "support team reply failed", "error", err)
	writeError(w, http.StatusInternalServerError, "Something went wrong. Please reply to the email instead.")
}
