package handler

// Public help assistant endpoints for the help center and the customer
// portal. A question starts an agent run; the client polls the session for
// the answer. Responses are never cached or indexed.

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type helpAssistantAskBody struct {
	VisitorID string `json:"visitor_id"`
	service.HelpAssistantQuestion
}

type helpAssistantFeedbackBody struct {
	VisitorID string `json:"visitor_id"`
	SessionID string `json:"session_id"`
	Helpful   bool   `json:"helpful"`
}

// SetHelpAssistant wires the help assistant into the public help center.
func (h *DocsHandler) SetHelpAssistant(assistant *service.HelpAssistantService) {
	h.helpAssistant = assistant
}

// SetHelpAssistant wires the help assistant into the customer portal.
func (h *CustomerPortalHandler) SetHelpAssistant(assistant *service.HelpAssistantService) {
	h.assistant = assistant
}

// PublicAssistantAsk handles POST /hc/{subdomain}/assistant/messages.
func (h *DocsHandler) PublicAssistantAsk(w http.ResponseWriter, r *http.Request) {
	cfg := h.resolveSubdomain(w, r)
	if cfg == nil {
		return
	}
	var body helpAssistantAskBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	asker := service.HelpAssistantAsker{WorkspaceID: cfg.WorkspaceID, Surface: model.HelpAssistantSurfaceHelpcenter, VisitorID: body.VisitorID}
	turn, err := h.helpAssistant.Ask(r.Context(), asker, body.HelpAssistantQuestion)
	writeHelpAssistant(w, r, http.StatusAccepted, turn, err)
}

// PublicAssistantSession handles GET /hc/{subdomain}/assistant/sessions/{sessionId}.
func (h *DocsHandler) PublicAssistantSession(w http.ResponseWriter, r *http.Request) {
	cfg := h.resolveSubdomain(w, r)
	if cfg == nil {
		return
	}
	asker := service.HelpAssistantAsker{WorkspaceID: cfg.WorkspaceID, Surface: model.HelpAssistantSurfaceHelpcenter, VisitorID: r.URL.Query().Get("visitor_id")}
	view, err := h.helpAssistant.Session(r.Context(), asker, chi.URLParam(r, "sessionId"))
	writeHelpAssistant(w, r, http.StatusOK, view, err)
}

// PublicAssistantFeedback handles POST /hc/{subdomain}/assistant/messages/{messageId}/feedback.
func (h *DocsHandler) PublicAssistantFeedback(w http.ResponseWriter, r *http.Request) {
	cfg := h.resolveSubdomain(w, r)
	if cfg == nil {
		return
	}
	var body helpAssistantFeedbackBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	asker := service.HelpAssistantAsker{WorkspaceID: cfg.WorkspaceID, Surface: model.HelpAssistantSurfaceHelpcenter, VisitorID: body.VisitorID}
	err := h.helpAssistant.Feedback(r.Context(), asker, body.SessionID, chi.URLParam(r, "messageId"), body.Helpful)
	writeHelpAssistant(w, r, http.StatusNoContent, nil, err)
}

// AssistantAsk handles POST /public/portal/{slug}/assistant/messages. A valid
// portal session adds the help spaces for signed-in customers.
func (h *CustomerPortalHandler) AssistantAsk(w http.ResponseWriter, r *http.Request) {
	ws := h.workspace(w, r)
	if ws == nil {
		return
	}
	var body helpAssistantAskBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	turn, err := h.assistant.Ask(r.Context(), h.assistantAsker(r, ws, body.VisitorID), body.HelpAssistantQuestion)
	writeHelpAssistant(w, r, http.StatusAccepted, turn, err)
}

// AssistantSession handles GET /public/portal/{slug}/assistant/sessions/{sessionId}.
func (h *CustomerPortalHandler) AssistantSession(w http.ResponseWriter, r *http.Request) {
	ws := h.workspace(w, r)
	if ws == nil {
		return
	}
	view, err := h.assistant.Session(r.Context(), h.assistantAsker(r, ws, r.URL.Query().Get("visitor_id")), chi.URLParam(r, "sessionId"))
	writeHelpAssistant(w, r, http.StatusOK, view, err)
}

// AssistantFeedback handles POST /public/portal/{slug}/assistant/messages/{messageId}/feedback.
func (h *CustomerPortalHandler) AssistantFeedback(w http.ResponseWriter, r *http.Request) {
	ws := h.workspace(w, r)
	if ws == nil {
		return
	}
	var body helpAssistantFeedbackBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	err := h.assistant.Feedback(r.Context(), h.assistantAsker(r, ws, body.VisitorID), body.SessionID, chi.URLParam(r, "messageId"), body.Helpful)
	writeHelpAssistant(w, r, http.StatusNoContent, nil, err)
}

// assistantAsker identifies a portal asker. Only a valid portal session makes
// them a signed-in customer.
func (h *CustomerPortalHandler) assistantAsker(r *http.Request, ws *service.PortalWorkspace, visitorID string) service.HelpAssistantAsker {
	asker := service.HelpAssistantAsker{WorkspaceID: ws.ID, Surface: model.HelpAssistantSurfacePortal, VisitorID: visitorID}
	if secret := portalSessionCookie(r); secret != "" {
		if access, err := h.portal.Authenticate(r.Context(), ws, secret); err == nil && access != nil {
			asker.PortalIdentityID = access.Identity.ID
		}
	}
	return asker
}

func writeHelpAssistant(w http.ResponseWriter, r *http.Request, status int, body any, err error) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Robots-Tag", "noindex")
	switch {
	case err == nil:
		if body == nil {
			w.WriteHeader(status)
			return
		}
		writeJSON(w, status, body)
	case errors.Is(err, service.ErrHelpAssistantInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrHelpAssistantNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrHelpAssistantPending), errors.Is(err, service.ErrHelpAssistantSessionFull):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrHelpAssistantLimit):
		w.Header().Set("Retry-After", "3600")
		writeError(w, http.StatusTooManyRequests, err.Error())
	case errors.Is(err, service.ErrHelpAssistantBusy):
		w.Header().Set("Retry-After", "10")
		writeError(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, service.ErrHelpAssistantUnavailable):
		writeError(w, http.StatusServiceUnavailable, service.ErrHelpAssistantUnavailable.Error())
	default:
		logHelpAssistantError(r, err)
		writeError(w, http.StatusInternalServerError, "the help assistant could not answer")
	}
}

func logHelpAssistantError(r *http.Request, err error) {
	slog.ErrorContext(r.Context(), "help assistant request failed", "path", r.URL.Path, "error", err)
}
