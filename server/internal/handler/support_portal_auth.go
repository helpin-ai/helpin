package handler

import (
	"net/http"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/service"
)

type PortalAuthHandler struct{ service *service.PortalAuthService }

func NewPortalAuthHandler(s *service.PortalAuthService) *PortalAuthHandler {
	return &PortalAuthHandler{service: s}
}

type portalAuthBody struct {
	WorkspaceID string `json:"workspace_id"`
	Email       string `json:"email"`
	Token       string `json:"token"`
}

func (h *PortalAuthHandler) RequestLink(w http.ResponseWriter, r *http.Request) {
	var body portalAuthBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	h.service.RequestLink(r.Context(), body.WorkspaceID, body.Email)
	writeJSON(w, http.StatusAccepted, map[string]string{"message": "If eligible, a sign-in link will be sent"})
}
func (h *PortalAuthHandler) Exchange(w http.ResponseWriter, r *http.Request) {
	var body portalAuthBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	token, identity, err := h.service.Exchange(r.Context(), body.WorkspaceID, body.Token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired link")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"session_token": token, "identity": identity})
}
func portalBearer(r *http.Request) string {
	parts := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
func (h *PortalAuthHandler) Session(w http.ResponseWriter, r *http.Request) {
	identity, err := h.service.Validate(r.Context(), r.URL.Query().Get("workspace_id"), portalBearer(r))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid portal session")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, identity)
}
func (h *PortalAuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var body portalAuthBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	h.service.Logout(r.Context(), body.WorkspaceID, portalBearer(r))
	w.WriteHeader(http.StatusNoContent)
}
