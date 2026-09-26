package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// CustomerPortalHandler adapts the public slug-based portal contract to the
// workspace-scoped magic link service. The cookie is never exposed to scripts.
type CustomerPortalHandler struct{ auth *service.PortalAuthService }

func NewCustomerPortalHandler(auth *service.PortalAuthService) *CustomerPortalHandler {
	return &CustomerPortalHandler{auth: auth}
}

const portalCookie = "helpin_portal_session"

func (h *CustomerPortalHandler) workspace(w http.ResponseWriter, r *http.Request) string {
	workspaceID, err := h.auth.WorkspaceID(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		writeError(w, http.StatusNotFound, "portal unavailable")
		return ""
	}
	return workspaceID
}

func (h *CustomerPortalHandler) Config(w http.ResponseWriter, r *http.Request) {
	id := h.workspace(w, r)
	if id == "" {
		return
	}
	config, err := h.auth.Configuration(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "portal unavailable")
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (h *CustomerPortalHandler) RequestLink(w http.ResponseWriter, r *http.Request) {
	var body portalAuthBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	// Do not disclose whether the slug, address, or workspace exists.
	if id, err := h.auth.WorkspaceID(r.Context(), chi.URLParam(r, "slug")); err == nil {
		h.auth.RequestLink(r.Context(), id, body.Email)
	}
	w.WriteHeader(http.StatusNoContent)
}

func portalSessionCookie(r *http.Request) string {
	cookie, err := r.Cookie(portalCookie)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func setPortalCookie(w http.ResponseWriter, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: portalCookie, Value: token, Path: "/api/public/portal/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: maxAge})
}

func portalCustomer(identity *model.SupportPortalIdentity) map[string]any {
	return map[string]any{"customer": map[string]any{"id": identity.ID, "email": identity.Email, "name": identity.DisplayName}}
}

func (h *CustomerPortalHandler) Exchange(w http.ResponseWriter, r *http.Request) {
	id := h.workspace(w, r)
	if id == "" {
		return
	}
	var body portalAuthBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	token, identity, err := h.auth.Exchange(r.Context(), id, body.Token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired link")
		return
	}
	setPortalCookie(w, token, 7*24*60*60)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, portalCustomer(identity))
}

func (h *CustomerPortalHandler) authorized(w http.ResponseWriter, r *http.Request) (string, *model.SupportPortalIdentity) {
	id := h.workspace(w, r)
	if id == "" {
		return "", nil
	}
	identity, err := h.auth.Validate(r.Context(), id, portalSessionCookie(r))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid portal session")
		return "", nil
	}
	return id, identity
}

func (h *CustomerPortalHandler) Session(w http.ResponseWriter, r *http.Request) {
	_, identity := h.authorized(w, r)
	if identity == nil {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, portalCustomer(identity))
}

func (h *CustomerPortalHandler) CreateRequest(w http.ResponseWriter, r *http.Request) {
	id, identity := h.authorized(w, r)
	if identity == nil {
		return
	}
	var body struct {
		Subject string `json:"subject"`
		Message string `json:"message"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	request, err := h.auth.CreateRequest(r.Context(), id, identity, body.Subject, body.Message)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPortalRequestInvalid):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrPortalIntakeDisabled):
			writeError(w, http.StatusForbidden, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "unable to create request")
		}
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, request)
}

func (h *CustomerPortalHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if id, err := h.auth.WorkspaceID(r.Context(), chi.URLParam(r, "slug")); err == nil {
		h.auth.Logout(r.Context(), id, portalSessionCookie(r))
	}
	setPortalCookie(w, "", -1)
	w.WriteHeader(http.StatusNoContent)
}

func (h *CustomerPortalHandler) Requests(w http.ResponseWriter, r *http.Request) {
	id, identity := h.authorized(w, r)
	if identity == nil {
		return
	}
	status := r.URL.Query().Get("status")
	if status != "" && status != "active" && status != model.SupportConversationStatusWaitingOnCustomer && status != model.SupportConversationStatusResolved {
		writeError(w, http.StatusBadRequest, "invalid request status")
		return
	}
	requests, err := h.auth.Requests(r.Context(), id, identity.ID, status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "unable to load requests")
		return
	}
	response := make([]map[string]any, 0, len(requests))
	for _, request := range requests {
		response = append(response, map[string]any{"reference": request.Reference, "subject": request.Subject, "status": request.Status, "last_activity_at": request.LastActivityAt})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, response)
}
